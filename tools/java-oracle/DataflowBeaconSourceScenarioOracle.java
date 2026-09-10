import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeBean;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowOpBeaconSource;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpBeaconSource differential
 * chain (work unit 4.365): BeaconSource operator configurations observed
 * through latched DefaultSupportCaptureOp reads.
 *
 * beacon-with-beans (EPLDataflowBeaconWithBeans) replays both bean-typed
 * sub-runs of runAssertionBeans: each sub-run is a standalone
 * deploy/instantiate/start/undeploy cycle over BeaconStream&lt;MyLegacyEvent&gt;
 * and BeaconStream&lt;MyEventNoDefaultCtor&gt; with "myfield : 'abc',
 * iterations : 1"; the latched capture releases on the single emitted row,
 * the underlying is the configured bean instance with myfield "abc" (the
 * no-default-ctor type is populated through constructor selection plus the
 * setMyfield setter), and the trace pins {"myfield":"abc"} for both.
 *
 * beacon-variable (EPLDataflowBeaconVariable) replays the path-deployed
 * "@public create Schema SomeEvent()" plus "@public create variable int
 * var_iterations=3" pair; the BeaconSource resolves iterations from the
 * variable at instantiation and emits three rows whose underlyings are empty
 * Map instances for the empty declared Map schema.
 *
 * beacon-no-type-iterations (EPLDataflowBeaconNoType sub-run 2) replays the
 * undeclared-type graph with "iterations: 5": the operator forges a transient
 * object-array type, the manufacturer stays absent, and each iteration
 * submits an empty Object[0] per BeaconSourceOp.
 *
 * beacon-no-type-instantiate-only (EPLDataflowBeaconNoType sub-run 5) replays
 * the "@public create objectarray schema MyTestOAType(p1 string)" path graph
 * with interval 0.5 and p1 'abc': the dataflow is instantiated with no
 * options and never started, so the observable is the absence of records.
 *
 * The remaining executions stay out of the deterministic chain on purpose:
 * EPLDataflowBeaconNoType sub-runs 1 (unbounded source plus cancel),
 * 3 (initialDelay with a wall-clock delta assertion) and 4 (interval source
 * plus cancel) as well as every sub-run inside EPLDataflowBeaconFields
 * (random Math.random values, EventBusSink polling) are timing- or
 * randomness-sensitive.
 *
 * Record protocol: every capture read is one record
 * {case, operation:"capture", statement:"flow:DefaultSupportCaptureOp",
 * sequence, time, new:[rows]} with rows shaped
 * {"kind":"row","fields":{...}}; both the empty Map underlying and the empty
 * Object[0] underlying serialize to {"kind":"row","fields":{}}. Session
 * configuration per case mirrors TestSuiteEPLDataflow.configure restricted to
 * these executions: the two bean types (MyLegacyEvent through the legacy
 * ConfigurationCommonEventTypeBean, MyEventNoDefaultCtor as a plain class
 * type) and the dataflow-util package import that resolves
 * DefaultSupportCaptureOp in the graph text; internal timer off, epoch
 * initialization, and each case gets a fresh runtime destroyed in finally.
 */
public final class DataflowBeaconSourceScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-beacon-source";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpBeaconSource.java";
    private static final String CASE_BEANS = "beacon-with-beans";
    private static final String CASE_VARIABLE = "beacon-variable";
    private static final String CASE_NO_TYPE_ITERATIONS = "beacon-no-type-iterations";
    private static final String CASE_NO_TYPE_INSTANTIATE_ONLY = "beacon-no-type-instantiate-only";
    private static final String[] CASES = {CASE_BEANS, CASE_VARIABLE, CASE_NO_TYPE_ITERATIONS, CASE_NO_TYPE_INSTANTIATE_ONLY};
    private static final int[] ORDINALS = {0, 1, 3, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-5438f7be9b56ca119b0c",
            "java-runtime-f54de0ae037b90b24551",
            "java-runtime-f1fc7e290467bab36366",
            "java-runtime-f1fc7e290467bab36366"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowBeaconWithBeans",
            "EPLDataflowBeaconVariable",
            "EPLDataflowBeaconNoType",
            "EPLDataflowBeaconNoType"};
    private static final int[] RECORD_COUNTS = {2, 1, 1, 0};
    private static final int TOTAL_RECORDS = 4;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String SCHEMA_DEPLOYMENT_ID = "schema";
    private static final String VARIABLE_DEPLOYMENT_ID = "variable";
    private static final String FLOW_NAME_ONE = "MyDataFlowOne";
    private static final String FLOW_NAME_TWO = "MyDataFlowTwo";
    private static final String FLOW_NAME_FIVE = "MyDataFlowFive";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final String[] BEAN_TYPE_NAMES = {"MyLegacyEvent", "MyEventNoDefaultCtor"};
    private static final String BEANS_GRAPH_PREFIX =
            "@name('flow') create dataflow MyDataFlowOne " +
            "BeaconSource -> BeaconStream<";
    private static final String BEANS_GRAPH_SUFFIX =
            "> {" +
            "  myfield : 'abc', iterations : 1" +
            "}" +
            "DefaultSupportCaptureOp(BeaconStream) {}";
    private static final String VARIABLE_SCHEMA_EPL = "@public create Schema SomeEvent()";
    private static final String VARIABLE_EPL = "@public create variable int var_iterations=3";
    private static final String VARIABLE_FLOW_GRAPH =
            "@name('flow') create dataflow MyDataFlowOne " +
            "BeaconSource -> BeaconStream<SomeEvent> {" +
            "  iterations : var_iterations" +
            "}" +
            "DefaultSupportCaptureOp(BeaconStream) {}";
    private static final String NO_TYPE_ITERATIONS_GRAPH =
            "@name('flow') create dataflow MyDataFlowTwo " +
            "BeaconSource -> BeaconStream {" +
            "  iterations: 5" +
            "}" +
            "DefaultSupportCaptureOp(BeaconStream) {}";
    private static final String OA_SCHEMA_EPL = "@public create objectarray schema MyTestOAType(p1 string)";
    private static final String INSTANTIATE_ONLY_GRAPH =
            "@name('flow') create dataflow MyDataFlowFive " +
            "BeaconSource -> BeaconStream<MyTestOAType> {" +
            "  interval: 0.5," +
            "  p1 : 'abc'" +
            "}";

    private DataflowBeaconSourceScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowBeaconSourceScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the four selected cases");
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

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];

        // Session configuration per TestSuiteEPLDataflow.configure restricted to
        // these executions: the two beacon bean types (MyLegacyEvent through the
        // legacy ConfigurationCommonEventTypeBean, MyEventNoDefaultCtor as a
        // plain class type) and the dataflow-util package import that resolves
        // DefaultSupportCaptureOp in the graph text.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        ConfigurationCommonEventTypeBean legacy = new ConfigurationCommonEventTypeBean();
        configuration.getCommon().addEventType("MyLegacyEvent", EPLDataflowOpBeaconSource.MyLegacyEvent.class.getName(), legacy);
        configuration.getCommon().addEventType("MyEventNoDefaultCtor", EPLDataflowOpBeaconSource.MyEventNoDefaultCtor.class);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-beacon-source-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_BEANS.equals(caseName)) {
                runBeaconWithBeans(configuration, runtime, writer);
            } else if (CASE_VARIABLE.equals(caseName)) {
                runBeaconVariable(configuration, runtime, writer);
            } else if (CASE_NO_TYPE_ITERATIONS.equals(caseName)) {
                runBeaconNoTypeIterations(configuration, runtime, writer);
            } else if (CASE_NO_TYPE_INSTANTIATE_ONLY.equals(caseName)) {
                runBeaconNoTypeInstantiateOnly(configuration, runtime, writer);
            } else {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
            if (writer.count() != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.count()
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * EPLDataflowBeaconWithBeans: two standalone sub-runs, one per bean type.
     * Each sub-run deploys the graph, instantiates with a latched capture
     * (latch 1), starts, awaits the single row, pins {"myfield":"abc"} and
     * undeploys.
     */
    private static void runBeaconWithBeans(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        for (int subRun = 0; subRun < BEAN_TYPE_NAMES.length; subRun++) {
            String typeName = BEAN_TYPE_NAMES[subRun];
            EPCompiled compiled = compile(configuration, runtime, BEANS_GRAPH_PREFIX + typeName + BEANS_GRAPH_SUFFIX);
            deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(1);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                    .operatorProvider(new DefaultSupportGraphOpProvider(capture));
            EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);
            instance.start();

            Object[] rows = capture.get(2, TimeUnit.SECONDS);
            if (rows.length != 1) {
                throw new IllegalStateException("bean sub-run " + subRun + " (" + typeName + ") capture held "
                        + rows.length + " rows, expected 1");
            }
            if (subRun == 0) {
                if (!(rows[0] instanceof EPLDataflowOpBeaconSource.MyLegacyEvent)) {
                    throw new IllegalStateException("bean sub-run " + subRun + " row was not a MyLegacyEvent: " + rows[0]);
                }
                if (!"abc".equals(((EPLDataflowOpBeaconSource.MyLegacyEvent) rows[0]).getMyfield())) {
                    throw new IllegalStateException("bean sub-run " + subRun + " myfield was not 'abc': " + rows[0]);
                }
            } else {
                if (!(rows[0] instanceof EPLDataflowOpBeaconSource.MyEventNoDefaultCtor)) {
                    throw new IllegalStateException("bean sub-run " + subRun + " row was not a MyEventNoDefaultCtor: " + rows[0]);
                }
                if (!"abc".equals(((EPLDataflowOpBeaconSource.MyEventNoDefaultCtor) rows[0]).getMyfield())) {
                    throw new IllegalStateException("bean sub-run " + subRun + " myfield was not 'abc': " + rows[0]);
                }
            }
            writer.addCapture(new JsonArray().add(new JsonObject().add("kind", "row")
                    .add("fields", new JsonObject().add("myfield", "abc"))));

            runtime.getDeploymentService().undeployAll();
        }
    }

    /**
     * EPLDataflowBeaconVariable: path-deployed public SomeEvent schema and
     * var_iterations variable; the flow resolves iterations from the variable
     * at instantiation and the latched capture (latch 3) yields three rows
     * whose underlyings are empty Map instances.
     */
    private static void runBeaconVariable(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        deploy(runtime, compile(configuration, runtime, VARIABLE_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        deploy(runtime, compile(configuration, runtime, VARIABLE_EPL), VARIABLE_DEPLOYMENT_ID);
        deploy(runtime, compile(configuration, runtime, VARIABLE_FLOW_GRAPH), FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(3);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);
        instance.start();

        Object[] rows = capture.get(2, TimeUnit.SECONDS);
        if (rows.length != 3) {
            throw new IllegalStateException("variable capture held " + rows.length + " rows, expected 3");
        }
        JsonArray output = new JsonArray();
        for (int i = 0; i < rows.length; i++) {
            if (!(rows[i] instanceof Map)) {
                throw new IllegalStateException("variable row " + i + " was not a Map underlying: " + rows[i]);
            }
            if (!((Map<?, ?>) rows[i]).isEmpty()) {
                throw new IllegalStateException("variable row " + i + " was not an empty Map: " + rows[i]);
            }
            output.add(new JsonObject().add("kind", "row").add("fields", new JsonObject()));
        }
        writer.addCapture(output);

        runtime.getDeploymentService().undeployAll();
    }

    /**
     * EPLDataflowBeaconNoType sub-run 2: no declared type, iterations 5. The
     * latched capture (latch 5) yields five rows, each the empty Object[0]
     * that BeaconSourceOp submits per iteration when no manufacturer exists.
     */
    private static void runBeaconNoTypeIterations(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        deploy(runtime, compile(configuration, runtime, NO_TYPE_ITERATIONS_GRAPH), FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(5);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_TWO, options).start();

        Object[] rows = capture.get(1, TimeUnit.SECONDS);
        if (rows.length != 5) {
            throw new IllegalStateException("no-type capture held " + rows.length + " rows, expected 5");
        }
        JsonArray output = new JsonArray();
        for (int i = 0; i < rows.length; i++) {
            if (!(rows[i] instanceof Object[]) || ((Object[]) rows[i]).length != 0) {
                throw new IllegalStateException("no-type row " + i + " was not an empty Object[0]: " + rows[i]);
            }
            output.add(new JsonObject().add("kind", "row").add("fields", new JsonObject()));
        }
        writer.addCapture(output);

        runtime.getDeploymentService().undeployAll();
    }

    /**
     * EPLDataflowBeaconNoType sub-run 5: path-deployed MyTestOAType objectarray
     * schema and the interval 0.5 / p1 'abc' flow, instantiated with no
     * options and never started; instantiation succeeding is the observable,
     * so this case contributes no records.
     */
    private static void runBeaconNoTypeInstantiateOnly(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        deploy(runtime, compile(configuration, runtime, OA_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        deploy(runtime, compile(configuration, runtime, INSTANTIATE_ONLY_GRAPH), FLOW_DEPLOYMENT_ID);

        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_FIVE);

        runtime.getDeploymentService().undeployAll();
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
     * Emits {case, operation, statement, sequence, time, new:[...]} records
     * for every capture read; the sequence restarts at 1 for every case.
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
