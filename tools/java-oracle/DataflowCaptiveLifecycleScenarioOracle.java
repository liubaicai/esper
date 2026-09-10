import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstanceCaptive;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.dataflow.util.EPDataFlowSignalFinalMarker;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowAPIStartCaptive differential
 * chain (work unit 4.371, dataflow-captive-lifecycle): the captive
 * start/submit/signal/cancel lifecycle of one dataflow instance plus the
 * instantiate-only doc sample.
 *
 * captive-emitter (the whole class is ONE execution, the 'direct' variant)
 * replays the run() body in source order. Flow A deploys the byte-exact
 * one-line MyDataFlow graph Emitter({name:'src1'}) -> outstream<MyOAEventType>
 * -> DefaultSupportCaptureOp under the pinned deployment id "flow",
 * instantiates with a fresh unlatched DefaultSupportCaptureOp supplied through
 * DefaultSupportGraphOpProvider, and drives the instance through
 * startCaptive(): the captive handle holds zero runnables and exactly one
 * emitter keyed by the Emitter NAME parameter 'src1', the instance state is
 * RUNNING and stays RUNNING after the final-marker signal (no transition to
 * complete), submissions of {"E1",10} {"E2",20} accumulate in the capture
 * current batch, the signal splits the batch (current read empty, the
 * received batch [E1,E2] surfaced by getAndReset().get(0)), the post-signal
 * {"E3",30} starts a fresh current batch, and cancel() moves the state to
 * CANCELLED. Captured rows are the raw Object[] underlyings serialized
 * positionally as p0/p1 (no envelope, no coercion). Flow B then undeploys and
 * deploys the byte-exact multi-line doc sample HelloWorldDataFlow (inline
 * SampleSchema with the literal tab-comment whitespace, Emitter
 * {name:'myemitter'} -> LogSink), instantiates with NO options — never
 * started, never cancelled, LogSink never runs — and records the
 * INSTANTIATED state before undeployAll.
 *
 * Record protocol (create-start-stop-destroy state/count shapes plus the
 * capture shape of the eventbus-source chain): every capture read is one
 * record {case, operation:"capture", statement, sequence, time, new:[rows]}
 * with statement "flow:DefaultSupportCaptureOp" and rows shaped
 * {"kind":"row","fields":{"p0":...,"p1":...}}; empty reads are recorded
 * explicitly with new:[]. State observables are {case, operation:"state",
 * statement:"flow", sequence, time, name:"instance.state",
 * value:"INSTANTIATED"|"RUNNING"|"CANCELLED"} records and size observables
 * are {case, operation:"count", statement:"flow", sequence, time, name,
 * count} records, both carrying the create-start-stop-destroy statement
 * label "flow". Sequence is case-local and restarts at 1; the time is the
 * fixed epoch 1970-01-01T00:00:00Z. The suite's assertEquals calls
 * (runnables size, emitters size, the 'src1' emitter key, the state reads,
 * and every row projection) are replayed as in-process asserts feeding the
 * records. Session configuration mirrors TestSuiteEPLDataflow.configure
 * restricted to this execution: the MyOAEventType object-array event type
 * with String p0 / int p1 and the dataflow-util package import that resolves
 * DefaultSupportCaptureOp in EPL text; Emitter and LogSink resolve through
 * the built-in ConfigurationCommon.DATAFLOWOPERATOR_IMPORT. Internal timer
 * off, epoch initialization, one fresh runtime destroyed in finally.
 */
public final class DataflowCaptiveLifecycleScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-captive-lifecycle";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIStartCaptive.java";
    private static final String CASE_CAPTIVE_EMITTER = "captive-emitter";
    private static final String[] CASES = {CASE_CAPTIVE_EMITTER};
    private static final int[] ORDINALS = {0};
    private static final String[] RUNTIME_IDS = {"java-runtime-407e42478546e709d774"};
    private static final String[] EXECUTION_NAMES = {"EPLDataflowAPIStartCaptive"};
    private static final int[] RECORD_COUNTS = {11};
    private static final int TOTAL_RECORDS = 11;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME_A = "MyDataFlow";
    private static final String FLOW_NAME_B = "HelloWorldDataFlow";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs from EPLDataflowAPIStartCaptive (lines 32-34 and
    // 70-74): flow A is one line with NO space between {name:'src1'} and
    // DefaultSupportCaptureOp; flow B concatenates the multi-line doc sample
    // including the literal tabs and the tab-prefixed comment line.
    private static final String FLOW_A_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_A + " " +
            "Emitter -> outstream<MyOAEventType> {name:'src1'}" +
            "DefaultSupportCaptureOp(outstream) {}";
    private static final String FLOW_B_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_B + "\n" +
            "  create schema SampleSchema(text string),\t// sample type\t\t\n" +
            "\t\n" +
            "  Emitter -> helloworld.stream<SampleSchema> { name: 'myemitter' }\n" +
            "  LogSink(helloworld.stream) {}";

    private DataflowCaptiveLifecycleScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowCaptiveLifecycleScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the one selected case");
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

        // Session configuration per TestSuiteEPLDataflow.configure restricted
        // to this execution: the MyOAEventType object-array type (suite
        // configure line 144) and the dataflow-util package import (suite
        // configure lines 154-155) that resolves DefaultSupportCaptureOp in
        // EPL text; Emitter and LogSink resolve through the built-in
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT. Internal timer off,
        // epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("MyOAEventType", "p0,p1".split(","), new Object[]{String.class, int.class});
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-captive-lifecycle-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            runCaptiveEmitter(configuration, runtime, writer);
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
     * EPLDataflowAPIStartCaptive.run (suite lines 28-78): the captive
     * start/submit/signal/cancel lifecycle of MyDataFlow followed by the
     * instantiate-only HelloWorldDataFlow doc sample.
     */
    private static void runCaptiveEmitter(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 32-34: byte-exact one-line graph deployed under the
        // pinned deployment id; the deployment carries exactly the 'flow'
        // statement.
        deploy(runtime, compile(configuration, runtime, FLOW_A_EPL), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 36-38: fresh unlatched capture supplied through the
        // operator provider.
        DefaultSupportCaptureOp<Object> captureOp = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
        options.operatorProvider(new DefaultSupportGraphOpProvider(captureOp));

        // suite lines 40-45: startCaptive; zero runnables, exactly one
        // emitter keyed by the Emitter NAME parameter 'src1', state RUNNING.
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_A, options);
        EPDataFlowInstanceCaptive captiveStart = instance.startCaptive();
        if (captiveStart.getRunnables().size() != 0) {
            throw new IllegalStateException("captive held " + captiveStart.getRunnables().size()
                    + " runnables, expected 0");
        }
        writer.addCount("runnables", captiveStart.getRunnables().size());
        if (captiveStart.getEmitters().size() != 1) {
            throw new IllegalStateException("captive held " + captiveStart.getEmitters().size()
                    + " emitters, expected 1");
        }
        if (captiveStart.getEmitters().get("src1") == null) {
            throw new IllegalStateException("captive did not expose the emitter keyed by 'src1'");
        }
        writer.addCount("emitters", captiveStart.getEmitters().size());
        assertState(instance, EPDataFlowState.RUNNING);
        writer.addState(instance.getState());

        // suite lines 47-48: {"E1",10} lands in the current batch.
        captiveStart.getEmitters().get("src1").submit(new Object[]{"E1", 10});
        writer.addCapture(captureRowArray(captureOp.getCurrent(), new JsonObject()
                .add("p0", "E1").add("p1", 10)));

        // suite lines 50-51: {"E2",20} accumulates after {"E1",10}.
        captiveStart.getEmitters().get("src1").submit(new Object[]{"E2", 20});
        writer.addCapture(captureRowArray(captureOp.getCurrent(), new JsonObject()
                        .add("p0", "E1").add("p1", 10),
                new JsonObject().add("p0", "E2").add("p1", 20)));

        // suite lines 53-56: the final-marker signal splits the batch — the
        // current read is empty and the received batch holds [E1,E2]. The
        // marker identity is not observable and not recorded.
        captiveStart.getEmitters().get("src1").submitSignal(new EPDataFlowSignalFinalMarker() {
        });
        if (captureOp.getCurrent().length != 0) {
            throw new IllegalStateException("capture held " + captureOp.getCurrent().length
                    + " rows after the final-marker signal, expected 0");
        }
        writer.addCapture(new JsonArray());
        List<List<Object>> batches = captureOp.getAndReset();
        if (batches.isEmpty()) {
            throw new IllegalStateException("getAndReset returned no batches after the final-marker signal");
        }
        writer.addCapture(captureRowArray(batches.get(0).toArray(), new JsonObject()
                        .add("p0", "E1").add("p1", 10),
                new JsonObject().add("p0", "E2").add("p1", 20)));

        // suite lines 58-59: {"E3",30} starts a fresh current batch.
        captiveStart.getEmitters().get("src1").submit(new Object[]{"E3", 30});
        writer.addCapture(captureRowArray(captureOp.getCurrent(), new JsonObject()
                .add("p0", "E3").add("p1", 30)));

        // suite lines 61-65: stays running until cancelled (no transition to
        // complete); cancel() moves the state to CANCELLED.
        assertState(instance, EPDataFlowState.RUNNING);
        writer.addState(instance.getState());
        instance.cancel();
        assertState(instance, EPDataFlowState.CANCELLED);
        writer.addState(instance.getState());

        // suite line 67: undeploy before the doc sample.
        runtime.getDeploymentService().undeployAll();

        // suite lines 69-76: the byte-exact multi-line doc sample graph,
        // instantiated with NO options — never started, never cancelled, so
        // LogSink never runs.
        deploy(runtime, compile(configuration, runtime, FLOW_B_EPL), FLOW_DEPLOYMENT_ID);
        flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_B);
        writer.addState(EPDataFlowState.INSTANTIATED);

        // suite line 78: undeployAll.
        runtime.getDeploymentService().undeployAll();
    }

    private static void assertState(EPDataFlowInstance instance, EPDataFlowState expected) {
        EPDataFlowState actual = instance.getState();
        if (actual != expected) {
            throw new IllegalStateException("instance state was " + actual + ", expected " + expected);
        }
    }

    /**
     * Asserts the captured rows are raw Object[] underlyings (no envelope, no
     * coercion) projecting positionally to the expected p0/p1 fields, and
     * renders them as {"kind":"row","fields":{...}} rows.
     */
    private static JsonArray captureRowArray(Object[] rows, JsonObject... expected) {
        if (rows.length != expected.length) {
            throw new IllegalStateException("capture held " + rows.length + " rows, expected " + expected.length);
        }
        JsonArray captured = new JsonArray();
        for (int i = 0; i < rows.length; i++) {
            if (!(rows[i] instanceof Object[])) {
                throw new IllegalStateException("row " + i + " was "
                        + (rows[i] == null ? "null" : rows[i].getClass().getName()) + ", expected a raw Object[]");
            }
            Object[] array = (Object[]) rows[i];
            if (array.length != 2 || !(array[0] instanceof String) || !(array[1] instanceof Integer)) {
                throw new IllegalStateException("row " + i + " did not project positionally to String p0 / int p1");
            }
            JsonObject fields = new JsonObject().add("p0", (String) array[0]).add("p1", (Integer) array[1]);
            if (!fields.equals(expected[i])) {
                throw new IllegalStateException("row " + i + " was " + fields + ", expected " + expected[i]);
            }
            captured.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return captured;
    }

    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled, String deploymentId) throws Exception {
        return runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId));
    }

    /**
     * RegressionEnvironmentBase.deploymentId analog: the deployment id of the
     * deployment containing the named statement.
     */
    private static String deploymentId(EPRuntime runtime, String statementName) throws Exception {
        for (String deployment : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deployment);
            for (EPStatement stmt : info.getStatements()) {
                if (stmt.getName().equals(statementName)) {
                    return stmt.getDeploymentId();
                }
            }
        }
        throw new IllegalStateException("statement not found: " + statementName);
    }

    /**
     * Emits {case, operation, statement, sequence, time, new:[...]} records
     * for every capture read, {case, operation:"state", statement:"flow",
     * sequence, time, name:"instance.state", value} records for the instance
     * state reads and {case, operation:"count", statement:"flow", sequence,
     * time, name, count} records for the captive sizes; the sequence is
     * case-local.
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

        private void addState(EPDataFlowState state) {
            add("state", "flow", "instance.state", Json.value(state.name()));
        }

        private void addCount(String name, int count) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "count")
                    .add("statement", "flow")
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("count", count);
            records.add(record);
        }

        private void add(String operation, String statement, String name, JsonValue value) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("value", value);
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }
}
