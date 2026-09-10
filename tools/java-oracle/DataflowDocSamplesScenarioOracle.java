import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowService;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
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

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowDocSamples differential chain
 * (work unit 4.364, dataflow-doc-samples, Pairing A): the two documentation
 * dataflow samples replayed as deployment/instantiation-state lifecycle
 * records.
 *
 * doc-run (ordinal 0) replays EPLDataflowDocSamplesRun: the byte-exact
 * hello-world graph HelloWorldDataFlow (BeaconSource -> helloworldStream with
 * text 'hello world' and iterations 1, drained by LogSink) is deployed under
 * the pinned deployment id "flow", instantiated, and drained by the blocking
 * instance.run(); the instance state moves INSTANTIATED -> COMPLETE.
 * BeaconSource emits exactly one event plus a final marker for iterations=1
 * and LogSink prints it to stdout — a non-asserted side effect that is not
 * captured or recorded.
 *
 * doc-flow (ordinal 1) replays the EPLDataflowDocSamples inner execution of
 * EPLDataflowOpSelect: the byte-exact five-Select graph MyDataFlow (two
 * BeaconSource inputs over an inline SampleSchema with embedded comments and
 * tabs, iterate:true, alias and join configurations) is compiled, deployed
 * under the pinned deployment id "flow", instantiated, and NEVER run — two
 * source operators would make run() throw IllegalStateException — then
 * undeployAll empties the deployment registry.
 *
 * Replay-shape adaptations frozen with the scouts (both sides implement the
 * same shape):
 *
 * 1. The 14 parse-only EPL fragments MyDataFlow..MyDataFlow14 (suite lines
 *    46-97) are asserted internally through parseModule success (mirroring
 *    the suite's tryEpl helper) but emit NO records: there is no Go parser
 *    surface for EPL text (documented adaptation).
 * 2. The record protocol maps the deployment/instantiation observables into
 *    {case, operation, statement, sequence, time, name, value} records with
 *    a sequence that is continuous per case (precedent
 *    DataflowCreateStartStopDestroyScenarioOracle).
 * 3. The deployment id is pinned to "flow" through DeploymentOptions so the
 *    runtime-assigned random UUID never leaks into the trace; the suite's
 *    env.deploymentId("flow") lookup is replayed as the deployment-id analog
 *    and asserted to equal the pinned id.
 * 4. The suite's HA-mode early return of the EPLDataflowOpSelect inner
 *    execution is a dropped non-replayable artifact (this replay is non-HA).
 *    The LogSink console output of doc-run and the discarded instantiate
 *    result of doc-flow are likewise non-recorded side effects.
 */
public final class DataflowDocSamplesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-doc-samples";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowDocSamples.java";
    private static final String CASE_DOC_RUN = "doc-run";
    private static final String CASE_DOC_FLOW = "doc-flow";
    private static final String[] CASES = {CASE_DOC_RUN, CASE_DOC_FLOW};
    private static final int[] ORDINALS = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-b41da4e41f347dcd49a8",
            "java-runtime-aaf36f532374aca5a232"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowDocSamplesRun",
            "EPLDataflowDocSamples"};
    private static final int[] RECORD_COUNTS = {4, 4};
    private static final int TOTAL_RECORDS = 8;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final String DOC_RUN_EPL =
            "@name('flow') create dataflow HelloWorldDataFlow\n" +
            "BeaconSource -> helloworldStream { text: 'hello world', iterations : 1 }\n" +
            "LogSink(helloworldStream) {}";
    private static final String DOC_FLOW_EPL =
            "@name('flow') create dataflow MyDataFlow\n" +
            "  create schema SampleSchema(tagId string, locX double),\t// sample type\t\t\t\n" +
            "  BeaconSource -> instream<SampleSchema> {}  // sample stream\n" +
            "  BeaconSource -> secondstream<SampleSchema> {}  // sample stream\n" +
            "  \n" +
            "  // Simple continuous count of events\n" +
            "  Select(instream) -> outstream {\n" +
            "    select: (select count(*) from instream)\n" +
            "  }\n" +
            "  \n" +
            "  // Demonstrate use of alias\n" +
            "  Select(instream as myalias) -> outstream {\n" +
            "    select: (select count(*) from myalias)\n" +
            "  }\n" +
            "  \n" +
            "  // Output only when the final marker arrives\n" +
            "  Select(instream as myalias) -> outstream {\n" +
            "    select: (select count(*) from myalias),\n" +
            "    iterate: true\n" +
            "  }\n" +
            "\n" +
            "  // Same input port for the two sample streams.\n" +
            "  Select( (instream, secondstream) as myalias) -> outstream {\n" +
            "    select: (select count(*) from myalias)\n" +
            "  }\n" +
            "\n" +
            "  // A join with multiple input streams,\n" +
            "  // joining the last event per stream forming pairs\n" +
            "  Select(instream, secondstream) -> outstream {\n" +
            "    select: (select a.tagId, b.tagId \n" +
            "                 from instream#lastevent as a, secondstream#lastevent as b)\n" +
            "  }\n" +
            "  \n" +
            "  // A join with multiple input streams and using aliases.\n" +
            "  @Audit Select(instream as S1, secondstream as S2) -> outstream {\n" +
            "    select: (select a.tagId, b.tagId \n" +
            "                 from S1#lastevent as a, S2#lastevent as b)\n" +
            "  }";
    private static final String[][] PARSE_ONLY_FRAGMENTS = {
            {"MyDataFlow", "create dataflow MyDataFlow\n" +
                    "MyOperator {}"},
            {"MyDataFlow2", "create dataflow MyDataFlow2\n" +
                    "create schema MyEvent as (id string, price double),\n" +
                    "MyOperator -> myOutStream<MyEvent> {\n" +
                    "myParameter : 10\n" +
                    "}"},
            {"MyDataFlow3", "create dataflow MyDataFlow3\n" +
                    "MyOperator(myInStream as mis) {}"},
            {"MyDataFlow4", "create dataflow MyDataFlow4\n" +
                    "MyOperator(streamOne as one, streamTwo as two) {}"},
            {"MyDataFlow5", "create dataflow MyDataFlow5\n" +
                    "MyOperator( (streamA, streamB) as streamsAB) {}"},
            {"MyDataFlow6", "create dataflow MyDataFlow6\n" +
                    "MyOperator(abc) -> my.out.stream {}"},
            {"MyDataFlow7", "create dataflow MyDataFlow7\n" +
                    "MyOperator -> my.out.one, my.out.two {}"},
            {"MyDataFlow8", "create dataflow MyDataFlow8\n" +
                    "create objectarray schema RFIDSchema (tagId string, locX double, locy double),\n" +
                    "MyOperator -> rfid.stream<RFIDSchema> {}"},
            {"MyDataFlow9", "create dataflow MyDataFlow9\n" +
                    "create objectarray schema RFIDSchema (tagId string, locX double, locy double),\n" +
                    "MyOperator -> rfid.stream<eventbean<RFIDSchema>> {}"},
            {"MyDataFlow10", "create dataflow MyDataFlow10\n" +
                    "MyOperator -> my.stream<eventbean<?>> {}"},
            {"MyDataFlow11", "create dataflow MyDataFlow11\n" +
                    "MyOperator {\n" +
                    "stringParam : 'sample',\n" +
                    "secondString : \"double-quotes are fine\",\n" +
                    "intParam : 10\n" +
                    "}"},
            {"MyDataFlow12", "create dataflow MyDataFlow12\n" +
                    "MyOperator {\n" +
                    "intParam : 24*60^60,\n" +
                    "threshold : var_threshold, // a variable defined in the runtime\n" +
                    "}"},
            {"MyDataFlow13", "create dataflow MyDataFlow13\n" +
                    "MyOperator {\n" +
                    "someSystemProperty : systemProperties('mySystemProperty')\n" +
                    "}"},
            {"MyDataFlow14", "create dataflow MyDataFlow14\n" +
                    "MyOperator {\n" +
                    "  myStringArray: ['a', \"b\",],\n" +
                    "  myMapOrObject: {\n" +
                    "    a : 10,\n" +
                    "    b : 'xyz',\n" +
                    "  },\n" +
                    "  myInstance: {\n" +
                    "    class: 'com.myorg.myapp.MyImplementation',\n" +
                    "    myValue : 'sample'\n" +
                    "  }\n" +
                    "}"},
    };

    private DataflowDocSamplesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowDocSamplesScenarioOracle <scenario.json>");
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

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];

        // Fresh session configuration per case: the BeaconSource, LogSink and
        // Select graph operators resolve through the built-in
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT; the doc-flow SampleSchema
        // is declared inline by the graph itself, so no extra imports or event
        // types are needed. Internal timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-doc-samples-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_DOC_RUN.equals(caseName)) {
                runDocRun(configuration, runtime, writer);
            } else if (CASE_DOC_FLOW.equals(caseName)) {
                runDocFlow(configuration, runtime, writer);
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
     * EPLDataflowDocSamplesRun (suite lines 36-98): the hello-world
     * BeaconSource -> LogSink graph deployed under the pinned deployment id,
     * instantiated, drained by the blocking run() (INSTANTIATED -> COMPLETE),
     * then the 14 parse-only fragments asserted internally (no records).
     */
    private static void runDocRun(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 38-41: byte-exact EPL deployed under the pinned id.
        EPDeployment deployed = deploy(runtime, compile(configuration, runtime, DOC_RUN_EPL), FLOW_DEPLOYMENT_ID);
        recordDeploy(deployed, writer);

        // suite lines 43-44: instantiate then blocking run; the instance state
        // is read from the instance API before and after.
        EPDataFlowService dfruntime = runtime.getDataFlowService();
        String flowDeploymentId = deploymentId(runtime, "flow");
        EPDataFlowInstance instance = dfruntime.instantiate(flowDeploymentId, "HelloWorldDataFlow");
        assertState(instance, EPDataFlowState.INSTANTIATED);
        writer.add("state", "flow:HelloWorldDataFlow", "instance.state", Json.value(instance.getState().name()));
        instance.run();
        assertState(instance, EPDataFlowState.COMPLETE);
        writer.add("state", "flow:HelloWorldDataFlow", "instance.state", Json.value(instance.getState().name()));
        // BeaconSource emitted exactly one event plus the final marker;
        // LogSink printed it — a non-asserted side effect, not captured.

        // suite lines 46-97: the 14 parse-only fragments via the tryEpl analog;
        // parse success is asserted internally, no records are emitted.
        for (String[] fragment : PARSE_ONLY_FRAGMENTS) {
            tryEpl(fragment[0], fragment[1]);
        }
    }

    /**
     * EPLDataflowOpSelect.EPLDataflowDocSamples inner execution (lines 57-105):
     * the five-Select graph is compiled, deployed under the pinned deployment
     * id, instantiated and NEVER run — two source operators would make run()
     * throw IllegalStateException — then undeployAll empties the registry.
     */
    private static void runDocFlow(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 63-101: byte-exact five-Select graph deployed under the
        // pinned id.
        EPDeployment deployed = deploy(runtime, compile(configuration, runtime, DOC_FLOW_EPL), FLOW_DEPLOYMENT_ID);
        recordDeploy(deployed, writer);

        // suite line 102: instantiate with no options; never started.
        EPDataFlowService dfruntime = runtime.getDataFlowService();
        String flowDeploymentId = deploymentId(runtime, "flow");
        EPDataFlowInstance instance = dfruntime.instantiate(flowDeploymentId, "MyDataFlow");
        assertState(instance, EPDataFlowState.INSTANTIATED);
        writer.add("state", "flow:MyDataFlow", "instance.state", Json.value(instance.getState().name()));

        // suite line 103: undeployAll; the registry holds zero deployments.
        runtime.getDeploymentService().undeployAll();
        int deploymentCount = runtime.getDeploymentService().getDeployments().length;
        if (deploymentCount != 0) {
            throw new IllegalStateException("deployment registry held " + deploymentCount
                    + " deployments after undeployAll, expected 0");
        }
        writer.add("undeploy", "flow", "deploymentCount", Json.value(deploymentCount));
    }

    /**
     * Shared deploy observables: the pinned deployment id took effect (the
     * default random UUID must not leak), the deployment carries exactly one
     * statement named 'flow'.
     */
    private static void recordDeploy(EPDeployment deployed, TraceWriter writer) throws Exception {
        if (!FLOW_DEPLOYMENT_ID.equals(deployed.getDeploymentId())) {
            throw new IllegalStateException("deployment id was " + deployed.getDeploymentId()
                    + ", expected the pinned " + FLOW_DEPLOYMENT_ID);
        }
        if (deployed.getStatements().length != 1) {
            throw new IllegalStateException("deployment held " + deployed.getStatements().length
                    + " statements, expected 1");
        }
        writer.add("deploy", "flow", "statementCount", Json.value(deployed.getStatements().length));
        String statementName = deployed.getStatements()[0].getName();
        if (!FLOW_DEPLOYMENT_ID.equals(statementName)) {
            throw new IllegalStateException("statement name was " + statementName + ", expected flow");
        }
        writer.add("deploy", "flow", "statementName", Json.value(statementName));
    }

    private static void assertState(EPDataFlowInstance instance, EPDataFlowState expected) {
        EPDataFlowState actual = instance.getState();
        if (actual != expected) {
            throw new IllegalStateException("instance state was " + actual + ", expected " + expected);
        }
    }

    /**
     * Suite helper tryEpl (EPLDataflowDocSamples lines 132-138): parseModule
     * must succeed; there is no Go parser surface, so nothing is recorded.
     */
    private static void tryEpl(String name, String epl) {
        try {
            if (EPCompilerProvider.getCompiler().parseModule(epl) == null) {
                throw new IllegalStateException("parseModule returned null for " + name);
            }
        } catch (Throwable t) {
            throw new IllegalStateException("parse-only fragment " + name + " failed: " + t.getMessage(), t);
        }
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
     * Emits {case, operation, statement, sequence, time, name, value} records
     * with a sequence that is continuous per case.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
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
