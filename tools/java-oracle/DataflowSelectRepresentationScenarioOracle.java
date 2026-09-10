import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.collection.Pair;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collections;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpSelect differential chain
 * (work unit 4.361): the select-star passthrough representation with and
 * without wrapper decoration, observed on the non-captive event bus through
 * DefaultSupportCaptureOp.
 *
 * select-wrapper-no-additional-props (EPLDataflowOpSelectWrapper{
 * wrapperWithAdditionalProps=false}) replays the plain
 * "insert into B select * from A" passthrough: one Map event {value=10} on
 * the @public @buseventtype schema A flows EventBusSource -> TheEvents<B>
 * through Select (select: (select * from TheEvents)) into the capture, which
 * releases exactly one plain Map row {value=10}.
 *
 * select-wrapper-additional-props (EPLDataflowOpSelectWrapper{
 * wrapperWithAdditionalProps=true}) replays the decorated
 * "insert into B select 'a' as hello, * from A": the hello column makes B a
 * wrapper (decorated) event type whose capture surface is the union
 * {value,hello}. The Java oracle asserts the suite's Pair split internally
 * (Pair.first is the underlying Map with value=10, Pair.second is the
 * additional Map with hello='a', mirroring suite lines 416-419), while the
 * trace pins the flat union surface {hello='a', value=10}: the
 * underlying-versus-additional namespace split is representation-only and
 * documented in the manifest difference.
 *
 * Record protocol: the Java test performs exactly one latch read per case
 * (DefaultSupportCaptureOp latch of 1, 1-second get), so every case records
 * exactly one record {case, operation:"capture",
 * statement:"flow:DefaultSupportCaptureOp", sequence, time, new:[rows]} with
 * rows shaped {"kind":"row","fields":{...}} and Integer values rendered as
 * JSON ints; there are no empty reads and no post-cancel reads. Session
 * configuration per case: internal timer off, epoch initialization, the
 * dataflow-util package import that resolves DefaultSupportCaptureOp in the
 * graph text (the only import needed); schemas A and B come from the
 * deployed EPL itself, no SupportBean types. Each case gets a fresh runtime
 * with a distinct URI, destroyed in finally.
 */
public final class DataflowSelectRepresentationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-select-representation";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java";
    private static final String CASE_NO_ADDITIONAL_PROPS = "select-wrapper-no-additional-props";
    private static final String CASE_ADDITIONAL_PROPS = "select-wrapper-additional-props";
    private static final String[] CASES = {CASE_NO_ADDITIONAL_PROPS, CASE_ADDITIONAL_PROPS};
    private static final int[] ORDINALS = {9, 10};
    private static final boolean[] ADDITIONAL_PROPS = {false, true};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9550df89e223e839e414",
            "java-runtime-13cbd82acb5791b30d07"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowOpSelectWrapper{wrapperWithAdditionalProps=false}",
            "EPLDataflowOpSelectWrapper{wrapperWithAdditionalProps=true}"};
    private static final int[] RECORD_COUNTS = {1, 1};
    private static final int TOTAL_RECORDS = 2;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME = "OutputFlow";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact EPL from EPLDataflowOpSelectWrapper (suite lines 393-400).
    private static final String EPL_SCHEMA_A = "@public @buseventtype create schema A(value int);\n";
    private static final String EPL_INSERT_PLAIN = "insert into B select * from A; \n";
    private static final String EPL_INSERT_WRAPPER = "insert into B select 'a' as hello, * from A; \n";
    private static final String EPL_GRAPH =
            "@name('flow') create dataflow OutputFlow\n" +
            "  EventBusSource -> TheEvents<B> {}\n" +
            "  Select(TheEvents) -> outstream {\n" +
            "    select: (select * from TheEvents)\n" +
            "  }\n" +
            "  DefaultSupportCaptureOp(outstream) {}\n";

    private DataflowSelectRepresentationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowSelectRepresentationScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, records);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " records, got " + records.size());
        }
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT).add("java", System.getProperty("java.version")).add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(scenario.getString("javaSource", JAVA_SOURCE))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain the two selected cases");
        }
        JsonArray caseDefinitions = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[i].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != CASES.length * 2) {
            throw new IllegalArgumentException("scenario must contain exactly " + (CASES.length * 2) + " steps");
        }
        JsonArray steps = stepValue.asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(caseIndex * 2), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            JsonObject advance = object(steps.get(caseIndex * 2 + 1), "advance-time " + caseIndex);
            if (!"advance-time".equals(advance.getString("op", ""))
                    || !EPOCH.equals(advance.getString("at", ""))) {
                throw new IllegalArgumentException("each case must pin the epoch advance-time step");
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static String epl(boolean wrapperWithAdditionalProps) {
        return EPL_SCHEMA_A
                + (wrapperWithAdditionalProps ? EPL_INSERT_WRAPPER : EPL_INSERT_PLAIN)
                + EPL_GRAPH;
    }

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        boolean wrapperWithAdditionalProps = ADDITIONAL_PROPS[caseIndex];
        String caseName = CASES[caseIndex];

        // Session configuration restricted to this execution: internal timer
        // off, and the dataflow-util package import that resolves
        // DefaultSupportCaptureOp in the graph text (the only import needed);
        // schemas A and B come from the deployed EPL itself.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-select-representation-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = compile(configuration, runtime, epl(wrapperWithAdditionalProps));
            deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<>(1);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
            options.operatorProvider(new DefaultSupportGraphOpProvider(capture));
            EPDataFlowInstance instance = runtime.getDataFlowService()
                    .instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
            instance.start();

            runtime.getEventService().sendEventMap(Collections.singletonMap("value", 10), "A");

            Object[] result;
            try {
                result = capture.get(1, TimeUnit.SECONDS);
            } catch (Exception e) {
                throw new RuntimeException("Timeout: " + e.getMessage(), e);
            }
            if (result.length != 1) {
                throw new IllegalStateException("case " + caseName + " capture held " + result.length
                        + " rows, expected 1");
            }

            JsonArray rows = new JsonArray();
            JsonObject fields = new JsonObject();
            if (wrapperWithAdditionalProps) {
                // Mirror the suite's Pair split (lines 416-419): the capture
                // surface is a Pair of underlying and additional namespaces.
                if (!(result[0] instanceof Pair)) {
                    throw new IllegalStateException("case " + caseName + " row was not a Pair: " + result[0]);
                }
                Pair<?, ?> pair = (Pair<?, ?>) result[0];
                Object first = pair.getFirst();
                Object second = pair.getSecond();
                if (!(first instanceof Map) || !(second instanceof Map)) {
                    throw new IllegalStateException("case " + caseName + " pair halves were not maps: first="
                            + first + " second=" + second);
                }
                Object value = ((Map<?, ?>) first).get("value");
                Object hello = ((Map<?, ?>) second).get("hello");
                if (!Integer.valueOf(10).equals(value) || !"a".equals(hello)) {
                    throw new IllegalStateException("case " + caseName + " wrapper row was [value=" + value
                            + ", hello=" + hello + "], expected [value=10, hello=a]");
                }
                // Record the flat union surface; the underlying-versus-
                // additional namespace split is representation-only and
                // documented in the manifest difference.
                fields.add("hello", "a").add("value", 10);
            } else {
                // Mirror the suite's assertPropsPerRow over the plain Map row.
                if (!(result[0] instanceof Map)) {
                    throw new IllegalStateException("case " + caseName + " row was not a Map: " + result[0]);
                }
                Object value = ((Map<?, ?>) result[0]).get("value");
                if (!Integer.valueOf(10).equals(value)) {
                    throw new IllegalStateException("case " + caseName + " plain row was [value=" + value
                            + "], expected [value=10]");
                }
                fields.add("value", 10);
            }
            rows.add(new JsonObject().add("kind", "row").add("fields", fields));

            TraceWriter writer = new TraceWriter(records, caseName);
            writer.addCapture(rows);
            if (writer.count() != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.count()
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }

            instance.cancel();
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled, String deploymentId) throws Exception {
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(deploymentId));
    }

    /**
     * Emits {case, operation:"capture", statement, sequence, time, new:[...]}
     * records for the single latch read of the case.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        private void addCapture(JsonArray rows) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "capture")
                    .add("statement", CAPTURE_STATEMENT)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("new", rows);
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }
}
