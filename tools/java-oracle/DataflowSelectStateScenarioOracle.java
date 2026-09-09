import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowEmitterOperator;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstanceCaptive;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProviderByOpName;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.CollectionUtil;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpSelect differential chain
 * (work unit 4.360): dataflow Select stateful executions observed through
 * captive-emitter submissions into DefaultSupportCaptureOp under an external
 * virtual clock.
 *
 * select-output-rate-limit (EPLDataflowOutputRateLimit) replays the
 * "output snapshot every 1 minute" Select: captive submissions alone emit
 * nothing (snapshot-only output), the snapshot releases on virtual-time ticks
 * with the cumulative unbounded sum (14 at the t=60s boundary observed at
 * t=65s, 23 at the t=120s boundary observed at t=125s), and after
 * instance.cancel() a suppressed post-cancel submission plus the advance to
 * t=245s reads empty.
 *
 * select-time-window-triggered (EPLDataflowTimeWindowTriggered) replays the
 * instream_s0#time(1 minute) Select: rows project at insert time (sumInt=2
 * for E1 at t=5s, cumulative sumInt=7 when E2 arrives at t=10s with E1 still
 * inside the 60s window), and boundary-inclusive expiry drops E1 exactly at
 * 5000+60000=65000 leaving the cumulative sumInt=5.
 *
 * Record protocol: every capture read is one record
 * {case, operation:"capture", statement:"flow:DefaultSupportCaptureOp",
 * sequence, time, new:[rows]} with rows shaped
 * {"kind":"row","fields":{...}}; empty reads are recorded explicitly with
 * new:[]. Unlike the static-epoch chains, record time pins the LIVE virtual
 * clock at each read (Instant.ofEpochMilli(runtime.getEventService()
 * .getCurrentTime()), precedent TimeWindowScenarioOracle), so each record
 * carries the phase boundary the read observed. Session configuration per
 * case mirrors TestSuiteEPLDataflow.configure restricted to these
 * executions: the SupportBean event type, the dataflow-util package import
 * that resolves DefaultSupportCaptureOp in the graph text, the SupportBean
 * import, internal timer off (mandatory for external clocking), and epoch
 * initialization via initialize(0L) (the harness realization of the suite's
 * advanceTime(0) before compileDeploy); each case gets a fresh runtime with
 * a distinct URI destroyed in finally.
 */
public final class DataflowSelectStateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-select-state";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java";
    private static final String CASE_RATE_LIMIT = "select-output-rate-limit";
    private static final String CASE_TIME_WINDOW = "select-time-window-triggered";
    private static final String[] CASES = {CASE_RATE_LIMIT, CASE_TIME_WINDOW};
    private static final int[] ORDINALS = {4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-64f78048eb114d0e9cf9",
            "java-runtime-553841d9498fc8d6d1c7"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowOutputRateLimit",
            "EPLDataflowTimeWindowTriggered"};
    private static final int[] RECORD_COUNTS = {5, 3};
    private static final int TOTAL_RECORDS = 8;
    private static final String CAPTURE_OPERATION = "capture";
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME = "MySelect";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String[][] PHASE_INSTANTS = {
            {"1970-01-01T00:00:00Z", "1970-01-01T00:00:05Z", "1970-01-01T00:01:05Z",
                    "1970-01-01T00:02:05Z", "1970-01-01T00:04:05Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z",
                    "1970-01-01T00:01:05Z"}};
    private static final String RATE_LIMIT_GRAPH =
            "@name('flow') create dataflow MySelect\n" +
            "Emitter -> instream_s0<SupportBean>{name: 'emitterS0'}\n" +
            "Select(instream_s0) -> outstream {\n" +
            "  select: (select sum(intPrimitive) as sumInt from instream_s0 output snapshot every 1 minute)\n" +
            "}\n" +
            "DefaultSupportCaptureOp(outstream) {}\n";
    private static final String TIME_WINDOW_GRAPH =
            "@name('flow') create dataflow MySelect\n" +
            "Emitter -> instream_s0<SupportBean>{name: 'emitterS0'}\n" +
            "Select(instream_s0) -> outstream {\n" +
            "  select: (select sum(intPrimitive) as sumInt from instream_s0#time(1 minute))\n" +
            "}\n" +
            "DefaultSupportCaptureOp(outstream) {}\n";

    private DataflowSelectStateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowSelectStateScenarioOracle <scenario.json>");
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

        int expectedSteps = 0;
        for (String[] phases : PHASE_INSTANTS) {
            expectedSteps += 1 + phases.length;
        }
        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != expectedSteps) {
            throw new IllegalArgumentException("scenario must contain exactly " + expectedSteps + " steps");
        }
        JsonArray steps = stepValue.asArray();
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(offset), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            offset++;
            for (String phase : PHASE_INSTANTS[caseIndex]) {
                JsonObject advance = object(steps.get(offset), "advance-time at offset " + offset);
                if (!"advance-time".equals(advance.getString("op", ""))
                        || !phase.equals(advance.getString("at", ""))) {
                    throw new IllegalArgumentException("case " + CASES[caseIndex]
                            + " must pin the phase boundary " + phase);
                }
                offset++;
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
        // these executions: the SupportBean type, the dataflow-util package
        // import that resolves DefaultSupportCaptureOp in the graph text, and
        // the SupportBean import. Internal timer stays off: the clock is
        // driven externally through advanceTime at the pinned phase
        // boundaries.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-select-state-" + RUNTIME_IDS[caseIndex], configuration);
        // Epoch initialization is the harness realization of the suite's
        // advanceTime(0) before compileDeploy.
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            if (CASE_RATE_LIMIT.equals(caseName)) {
                runOutputRateLimit(configuration, runtime, writer);
            } else if (CASE_TIME_WINDOW.equals(caseName)) {
                runTimeWindowTriggered(configuration, runtime, writer);
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
     * EPLDataflowOutputRateLimit: submissions accumulate silently under
     * "output snapshot every 1 minute" (snapshot-only output), the snapshot
     * releases the cumulative unbounded sum on virtual-time ticks, and
     * post-cancel activity is suppressed.
     */
    private static void runOutputRateLimit(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled compiled = compile(configuration, runtime, RATE_LIMIT_GRAPH);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        Map<String, Object> operators = CollectionUtil.populateNameValueMap("DefaultSupportCaptureOp", capture);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        EPDataFlowInstanceCaptive captive = instance.startCaptive();
        EPDataFlowEmitterOperator emitter = captive.getEmitters().get("emitterS0");

        // Record 1: submissions alone emit nothing under snapshot-only output.
        runtime.getEventService().advanceTime(5000);
        emitter.submit(new SupportBean("E1", 5));
        emitter.submit(new SupportBean("E2", 3));
        emitter.submit(new SupportBean("E3", 6));
        writer.addCapture(emptyRead(capture, "post-submission t=5000"));

        // Record 2: the t=60000 snapshot fires when the clock crosses to 65000.
        runtime.getEventService().advanceTime(65000);
        writer.addCapture(singleSumIntRead(capture, 14, "t=65000 snapshot"));

        // Record 3: further submissions stay invisible until the next tick.
        emitter.submit(new SupportBean("E4", 3));
        emitter.submit(new SupportBean("E5", 6));
        writer.addCapture(emptyRead(capture, "post-submission t=65000"));

        // Record 4: the t=120000 snapshot releases the cumulative unbounded
        // sum 14+3+6=23 (no window resets between snapshots).
        runtime.getEventService().advanceTime(125000);
        writer.addCapture(singleSumIntRead(capture, 23, "t=125000 snapshot"));

        // Record 5: after cancel a suppressed post-cancel submission plus the
        // advance to t=245000 read empty.
        instance.cancel();
        emitter.submit(new SupportBean("E5", 6));
        runtime.getEventService().advanceTime(245000);
        writer.addCapture(emptyRead(capture, "post-cancel t=245000"));
    }

    /**
     * EPLDataflowTimeWindowTriggered: rows project at insert time over the
     * cumulative #time(1 minute) window, and boundary-inclusive expiry drops
     * E1 exactly at 5000+60000=65000.
     */
    private static void runTimeWindowTriggered(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled compiled = compile(configuration, runtime, TIME_WINDOW_GRAPH);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        Map<String, Object> operators = CollectionUtil.populateNameValueMap("DefaultSupportCaptureOp", capture);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        EPDataFlowInstanceCaptive captive = instance.startCaptive();
        EPDataFlowEmitterOperator emitter = captive.getEmitters().get("emitterS0");

        // Record 1: insert-time output for E1.
        runtime.getEventService().advanceTime(5000);
        emitter.submit(new SupportBean("E1", 2));
        writer.addCapture(singleSumIntRead(capture, 2, "insert-time t=5000"));

        // Record 2: cumulative insert-time output; E1@5s is still inside the
        // 60s window when E2 arrives at t=10s.
        runtime.getEventService().advanceTime(10000);
        emitter.submit(new SupportBean("E2", 5));
        writer.addCapture(singleSumIntRead(capture, 7, "insert-time t=10000"));

        // Record 3: boundary-inclusive expiry — E1 expires exactly at
        // 5000+60000=65000 and only E2 remains.
        runtime.getEventService().advanceTime(65000);
        writer.addCapture(singleSumIntRead(capture, 5, "expiry t=65000"));

        instance.cancel();
    }

    /**
     * Reads the capture with getCurrentAndReset and requires exactly one row
     * carrying the expected sumInt.
     */
    private static JsonArray singleSumIntRead(DefaultSupportCaptureOp<Object> capture, int expectedSumInt, String phase) {
        Object[] rows = capture.getCurrentAndReset();
        if (rows.length != 1) {
            throw new IllegalStateException(phase + ": capture held " + rows.length + " rows, expected 1");
        }
        if (!(rows[0] instanceof Object[])) {
            throw new IllegalStateException(phase + ": capture row was not an Object[]: " + rows[0]);
        }
        Object[] row = (Object[]) rows[0];
        if (row.length != 1) {
            throw new IllegalStateException(phase + ": capture row held " + row.length + " columns, expected 1");
        }
        if (!(row[0] instanceof Integer)) {
            throw new IllegalStateException(phase + ": sumInt was not an Integer: " + row[0]);
        }
        int sumInt = (Integer) row[0];
        if (sumInt != expectedSumInt) {
            throw new IllegalStateException(phase + ": sumInt was " + sumInt + ", expected " + expectedSumInt);
        }
        JsonArray snapshot = new JsonArray();
        snapshot.add(new JsonObject().add("kind", "row").add("fields",
                new JsonObject().add("sumInt", sumInt)));
        return snapshot;
    }

    /**
     * Reads the capture with getCurrentAndReset and requires emptiness; the
     * empty read is recorded explicitly.
     */
    private static JsonArray emptyRead(DefaultSupportCaptureOp<Object> capture, String phase) {
        Object[] rows = capture.getCurrentAndReset();
        if (rows.length != 0) {
            throw new IllegalStateException(phase + ": capture held " + rows.length + " rows, expected 0");
        }
        return new JsonArray();
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
     * for every capture read; time pins the LIVE virtual clock at the read
     * (not a static epoch), so each record carries the phase boundary it
     * observed.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        private void addCapture(JsonArray rows) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", CAPTURE_OPERATION)
                    .add("statement", CAPTURE_STATEMENT)
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", rows);
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }
}
