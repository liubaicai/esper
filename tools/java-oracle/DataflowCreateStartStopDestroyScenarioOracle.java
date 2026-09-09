import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowDescriptor;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowService;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.scopetest.EPAssertionUtil;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.DeploymentIdNamePair;
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
 * Direct Esper 9.0.0 oracle for the EPLDataflowAPICreateStartStopDestroy
 * differential chain (work unit 4.358): the dataflow DEFINITION REGISTRY
 * lifecycle — deploy/register, descriptor lookup, instantiate (discarded,
 * never started), undeploy, error probes, destroy removal, re-deploy and
 * re-instantiate. No dataflow instance is ever run on either side and the
 * chain produces zero rows.
 *
 * Replay-shape adaptations frozen with the scouts (both sides implement the
 * same shape):
 *
 * 1. The record protocol maps Java service observables into {name,value}
 *    records; instantiate-failure probes carry only the error class token
 *    "not-defined" (no Go text match). The byte-exact Java messages
 *    ("Data flow by name 'MyGraph' for deployment id 'DEP1' has not been
 *    defined" and the 'DUMMY' variant) are asserted internally with message
 *    equality before each class-token record is emitted.
 * 2. The re-deploy of the first execution (suite line 74) uses the
 *    runtime-assigned random UUID deployment id; it is not pinned and not
 *    recorded (dropped non-replayable artifact) — only the resulting
 *    instantiate is recorded.
 * 3. The post-destroy probe (suite line 71) is asserted internally with its
 *    byte-exact message but is not recorded: it duplicates the earlier
 *    DEP1/MyGraph error record in every recorded field.
 * 4. The second execution's parseModule single-module-item result is
 *    asserted internally (no Go surface) and not recorded, and the suite's
 *    HA-mode early return of that execution is a dropped non-replayable
 *    artifact (this replay is non-HA).
 */
public final class DataflowCreateStartStopDestroyScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-create-start-stop-destroy";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPICreateStartStopDestroy.java";
    private static final String CREATE_START_STOP = "create-start-stop";
    private static final String DEPLOYMENT_ADMIN = "deployment-admin";
    private static final String[] CASES = {CREATE_START_STOP, DEPLOYMENT_ADMIN};
    private static final int[] ORDINALS = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-1a8426a71e3b5252a181",
            "java-runtime-2c3c90d3cdb64596c225"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowCreateStartStop",
            "EPLDataflowDeploymentAdmin"};
    private static final int[] RECORD_COUNTS = {7, 1};
    private static final int TOTAL_RECORDS = 8;
    private static final String CREATE_START_STOP_EPL =
            "@Name('flow') create dataflow MyGraph Emitter -> outstream<?> {}";
    private static final String DEPLOYMENT_ADMIN_EPL =
            "@name('flow') create dataflow TheGraph\n"
                    + "create schema ABC as " + SupportBean.class.getName() + ","
                    + "DefaultSupportSourceOp -> outstream<SupportBean> {}\n"
                    + "Select(outstream) -> selectedData {select: (select theString, intPrimitive from outstream) }\n"
                    + "DefaultSupportCaptureOp(selectedData) {};";
    private static final String NOT_DEFINED_MYGRAPH =
            "Data flow by name 'MyGraph' for deployment id 'DEP1' has not been defined";
    private static final String NOT_DEFINED_DUMMY =
            "Data flow by name 'DUMMY' for deployment id 'DEP1' has not been defined";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    private DataflowCreateStartStopDestroyScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowCreateStartStopDestroyScenarioOracle <scenario.json>");
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

        // Session configuration per TestSuiteEPLDataflow.configure subset: the
        // SupportBean type plus the dataflow-util package imports that resolve
        // DefaultSupportSourceOp and DefaultSupportCaptureOp in EPL text (the
        // suite's lines 154-155; the core dataflow ops package containing
        // Emitter and Select is built-in through
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT).
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-create-start-stop-destroy-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CREATE_START_STOP.equals(caseName)) {
                runCreateStartStop(configuration, runtime, writer);
            } else if (DEPLOYMENT_ADMIN.equals(caseName)) {
                runDeploymentAdmin(configuration, runtime, writer);
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
     * EPLDataflowCreateStartStop (suite lines 45-77): the definition registry
     * lifecycle of MyGraph under the pinned deployment id DEP1 — register,
     * instantiate (discarded), undeploy, error probes, destroy removal,
     * re-deploy and re-instantiate. No instance is ever started.
     */
    private static void runCreateStartStop(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPDataFlowService dfruntime = runtime.getDataFlowService();

        // suite lines 47-53: byte-exact EPL deployed under the pinned id DEP1.
        EPCompiled compiledGraph = compile(configuration, runtime, CREATE_START_STOP_EPL);
        deploy(runtime, compiledGraph, "DEP1");

        // suite lines 55-59: registry listing contains the deployed pair and the
        // descriptor carries dataFlowName=MyGraph and statementName=flow (the
        // descriptor shape is asserted internally, not recorded).
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!"DEP1".equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId + ", expected DEP1");
        }
        EPAssertionUtil.assertEqualsAnyOrder(
                new DeploymentIdNamePair[]{new DeploymentIdNamePair(flowDeploymentId, "MyGraph")},
                dfruntime.getDataFlows());
        EPDataFlowDescriptor desc = dfruntime.getDataFlow("DEP1", "MyGraph");
        if (desc == null
                || !"MyGraph".equals(desc.getDataFlowName())
                || !"flow".equals(desc.getStatementName())) {
            throw new IllegalStateException("descriptor did not match the suite contract: " + desc);
        }
        writer.add("lifecycle", "flow:MyGraph", "registered", Json.value("MyGraph"));

        // suite line 61: instantiate; the instance is discarded, never started.
        dfruntime.instantiate(flowDeploymentId, "MyGraph");
        writer.add("state", "flow", "state", Json.value("instantiated"));

        // suite line 64: stop — undeploy the deployment containing 'flow'; the
        // definition can no longer be instantiated (suite lines 65-66).
        undeployModuleContaining(runtime, "flow");
        tryInstantiate(dfruntime, "DEP1", "MyGraph", NOT_DEFINED_MYGRAPH, writer);
        tryInstantiate(dfruntime, "DEP1", "DUMMY", NOT_DEFINED_DUMMY, writer);

        // suite lines 69-70: destroy — the descriptor is gone and the registry
        // is empty.
        if (dfruntime.getDataFlow("DEP1", "MyGraph") != null) {
            throw new IllegalStateException("descriptor survived undeploy for DEP1/MyGraph");
        }
        writer.add("lifecycle", "flow:MyGraph", "registered", Json.value((String) null));
        int savedConfigurations = dfruntime.getDataFlows().length;
        if (savedConfigurations != 0) {
            throw new IllegalStateException("registry held " + savedConfigurations + " definitions after destroy, expected 0");
        }
        writer.addCount("lifecycle", "flow", "saved-configurations", savedConfigurations);

        // suite line 71: post-destroy probe — asserted internally with the same
        // byte-exact message; not recorded (duplicates the first error record).
        assertInstantiateFails(dfruntime, "DEP1", "MyGraph", NOT_DEFINED_MYGRAPH);

        // suite lines 74-75: re-deploy under the runtime-assigned random UUID
        // deployment id (not pinned, not recorded) and re-instantiate.
        EPCompiled recompiled = compile(configuration, runtime, CREATE_START_STOP_EPL);
        EPDeployment redeployed = deploy(runtime, recompiled);
        if (redeployed.getDeploymentId() == null || "DEP1".equals(redeployed.getDeploymentId())) {
            throw new IllegalStateException("re-deploy did not assign a fresh deployment id");
        }
        String redeploymentId = deploymentId(runtime, "flow");
        dfruntime.instantiate(redeploymentId, "MyGraph");
        writer.add("state", "flow", "state", Json.value("instantiated"));
    }

    /**
     * EPLDataflowDeploymentAdmin (suite lines 85-108): parseModule yields one
     * module item (asserted internally, not recorded), the four-node graph is
     * compiled and deployed, and the discarded instantiate of TheGraph emits
     * the single state record. The suite's HA-mode early return is not
     * applicable to this non-HA replay.
     */
    private static void runDeploymentAdmin(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        Module module = EPCompilerProvider.getCompiler().parseModule(DEPLOYMENT_ADMIN_EPL);
        if (module.getItems().size() != 1) {
            throw new IllegalStateException("module item count was " + module.getItems().size() + ", expected 1");
        }

        EPCompiled compiled = compile(configuration, runtime, DEPLOYMENT_ADMIN_EPL);
        deploy(runtime, compiled);

        String flowDeploymentId = deploymentId(runtime, "flow");
        runtime.getDataFlowService().instantiate(flowDeploymentId, "TheGraph");
        writer.add("state", "flow", "state", Json.value("instantiated"));
    }

    /**
     * Suite helper (lines 115-122): instantiate must fail with the byte-exact
     * message; the record carries only the error class token "not-defined".
     */
    private static void tryInstantiate(EPDataFlowService dfruntime, String deploymentId, String graph,
                                       String message, TraceWriter writer) {
        assertInstantiateFails(dfruntime, deploymentId, graph, message);
        writer.add("lifecycle", "flow", "instantiate-error", Json.value("not-defined"));
    }

    private static void assertInstantiateFails(EPDataFlowService dfruntime, String deploymentId, String graph,
                                               String message) {
        try {
            dfruntime.instantiate(deploymentId, graph);
            throw new IllegalStateException("expected instantiate of '" + graph + "' to fail");
        } catch (EPDataFlowInstantiationException ex) {
            if (!message.equals(ex.getMessage())) {
                throw new IllegalStateException("instantiate error message was '" + ex.getMessage()
                        + "', expected '" + message + "'");
            }
        }
    }

    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled) throws Exception {
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled, String deploymentId) throws Exception {
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(deploymentId));
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
     * RegressionEnvironmentBase.undeployModuleContaining analog.
     */
    private static void undeployModuleContaining(EPRuntime runtime, String statementName) throws Exception {
        for (String deployment : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deployment);
            for (EPStatement stmt : info.getStatements()) {
                if (stmt.getName().equals(statementName)) {
                    runtime.getDeploymentService().undeploy(info.getDeploymentId());
                    return;
                }
            }
        }
        throw new IllegalStateException("Failed to find deployment with statement '" + statementName + "'");
    }

    /**
     * Emits {case, operation, statement, sequence, time, name, value} records
     * for registry/state observables and the count-form {case, operation,
     * statement, sequence, time, name, count} record for registry sizes.
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

        private void addCount(String operation, String statement, String name, int count) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("count", count);
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }
}
