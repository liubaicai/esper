import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowAlreadyExistsException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowExecutionException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstanceOperatorStat;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowNotFoundException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowOperatorParameterProviderContext;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowOperatorProviderContext;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowSavedConfiguration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowService;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.client.util.StatementType;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.framework.RegressionPath;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowAPIInstantiationOptions;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowAPIRunStartCancelJoin;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the dataflow service/instance lifecycle core
 * differential chain (work unit 4.374-a, dataflow-lifecycle-core): saved
 * configuration and saved instance service CRUD, operator statistics,
 * instantiation option injection callbacks and the invalid join/run plus
 * blocking-exception state machines.
 *
 * config-and-instance (EPLDataflowAPIConfigAndInstance, the 'direct' variant)
 * replays the run() body in source order: the empty saved-config list, the
 * null getSavedConfiguration recorded as the normalized token "absent" (the
 * null assert is in-process), the false removeSavedConfiguration, the
 * instantiateSavedConfiguration miss (message asserted in-process), the
 * saveConfiguration against the missing dataflow 'MyDataflow', the byte-exact
 * two-statement graph (the @public objectarray schema MyEvent plus the
 * BeaconSource iterations:1 EventBusSink flow deployed under the pinned
 * deployment id "flow") saved as 'MyFirstFlow', the duplicate-save rejection,
 * the true remove (the second false remove and the emptied list replay as
 * in-process asserts only), the re-save, instantiateSavedConfiguration with
 * the s0 MyEvent listener, the blocking run() whose EventBusSink delivery
 * invokes the listener, the strengthened COMPLETE state, and the saved
 * instance CRUD around name 'F1' (the deployment-id equality assert is
 * in-process — deployment ids are pinned but never recorded).
 *
 * statistics (EPLDataflowAPIStatistics, 'direct') deploys the byte-exact
 * one-line MyGraph graph (note the exact space before
 * DefaultSupportCaptureOp), asserts the statement properties
 * STATEMENTTYPE=CREATE_DATAFLOW and CREATEOBJECTNAME=MyGraph as oracle-internal
 * in-process asserts (the Go engine exposes no statement-type/creator
 * introspection surface, so they are never recorded), instantiates with
 * operatorStatistics(true) cpuStatistics(true), runs, and records the
 * two-operator statistics: the source name/number plus the NORMALIZED bare-port
 * pretty print "DefaultSupportSourceOp#0() -> outstream" (Java's full
 * "...<SupportBean>" string is asserted in-process), the overall submission
 * count 2 and the port-0 per-port count 2; the capture name/number, the native
 * bare pretty print "DefaultSupportCaptureOp#1(outstream)" and the overall
 * submission count 0. Time magnitudes are excluded from records: timeOverall>0
 * and timeOverall==timePerPort[0] for the source and timeOverall==0 plus the
 * per-port array length for the capture stay in-process, and the capture
 * per-port length is NOT recorded because the Go engine structurally sizes
 * SubmittedByPort by declared input ports (length 1 with value 0 for a
 * terminal), so the two sides cannot record the same length honestly.
 *
 * parameter-injection-callback (EPLDataflowAPIInstantiationOptions ordinal 0)
 * deploys the byte-exact SomeType schema plus the MyDataFlowOne MyOp graph and
 * instantiates with a parameter provider supplying propTwo:'def'. The MyOp
 * INSTANCES static registry is cleared before the case and harvested through
 * getAndClearInstances() after instantiate (exactly one instance). The
 * provider's private context map is harvested through an oracle subclass that
 * mirrors the Java puts; it holds three contexts recorded in sorted parameter
 * order propOne/propThree/propTwo (String natural order), the shared context
 * observables (operatorName MyOp, operatorNum 0, dataFlowName MyDataFlowOne)
 * recorded once each from the fully-asserted propTwo context, the
 * factory-identity observables for propTwo and propThree (assertSame replayed),
 * and the resolved parameter values abc/def/xyz read back from the MyOp
 * instance.
 *
 * operator-injection-callback (EPLDataflowAPIInstantiationOptions ordinal 1)
 * deploys the same byte-exact graph shape, instantiates with an operator
 * provider subclass mirroring the Java puts (the substitute MyOp registration
 * in the static INSTANCES registry is harvested and cleared in-process), and
 * records the single provider context (operatorName MyOp, dataFlowName
 * MyDataFlowOne).
 *
 * invalid-join-run (EPLDataflowAPIRunStartCancelJoin ordinal 6) deploys the
 * byte-exact BeaconSource iterations graph, records the INSTANTIATED entry
 * state (strengthened — the suite asserts no initial state here), replays the
 * join-before-execution IllegalStateException (message asserted in-process),
 * cancel (CANCELLED), the run-after-cancel and start-after-cancel
 * IllegalStateExceptions, and the idempotent second cancel (state stays
 * CANCELLED).
 *
 * blocking-exception (EPLDataflowAPIRunStartCancelJoin ordinal 3) deploys the
 * byte-exact two-clause graph whose capture clause is concatenated with NO
 * separator after the source clause's trailing '{}' plus the SomeType schema
 * preamble, drives the blocking run() that throws EPDataFlowExecutionException
 * (cause chain ending in MyRuntimeException("TestException") and the wrapped
 * source message asserted in-process), then records the COMPLETE state and the
 * empty capture read.
 *
 * Record protocol (create-start-stop-destroy conventions): count records are
 * {case, operation:"count", statement:"flow", sequence, time, name, count},
 * state records {case, operation:"state", statement:"flow", sequence, time,
 * name:"instance.state", value:UPPERCASE enum name} and value/lifecycle
 * records {case, operation:"lifecycle", statement:"flow", sequence, time,
 * name, value}. Error observables are recorded as error-class TOKENS only —
 * the byte-exact Java message texts are asserted in-process with equals and
 * never recorded (matching the invalidity policy). Sequence is case-local and
 * restarts at 1; the time is the fixed epoch 1970-01-01T00:00:00Z. Each case
 * runs on its own fresh runtime (internal timer off, epoch initialization)
 * that is undeployAll'd after the case body and destroyed in finally;
 * deployment ids are pinned ("flow" for the dataflow deployments) but never
 * recorded. Session configuration mirrors TestSuiteEPLDataflow.configure
 * restricted to these executions: the SupportBean event type plus the
 * dataflow-util package import for the DefaultSupportSourceOp /
 * DefaultSupportCaptureOp cases, the EPLDataflowAPIInstantiationOptions.
 * MyOpForge class import that resolves the EPL operator name MyOp by the
 * Forge-suffix convention, and the built-in
 * ConfigurationCommon.DATAFLOWOPERATOR_IMPORT for BeaconSource and
 * EventBusSink; the SomeType and MyEvent types come from the deployed byte-exact
 * EPL preambles over the RegressionPath.
 */
public final class DataflowLifecycleCoreScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-lifecycle-core";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIConfigAndInstance.java";
    private static final String CASE_CONFIG_INSTANCE = "config-and-instance";
    private static final String CASE_STATISTICS = "statistics";
    private static final String CASE_PARAMETER_INJECTION = "parameter-injection-callback";
    private static final String CASE_OPERATOR_INJECTION = "operator-injection-callback";
    private static final String CASE_INVALID_JOIN_RUN = "invalid-join-run";
    private static final String CASE_BLOCKING_EXCEPTION = "blocking-exception";
    private static final String[] CASES = {
            CASE_CONFIG_INSTANCE, CASE_STATISTICS, CASE_PARAMETER_INJECTION,
            CASE_OPERATOR_INJECTION, CASE_INVALID_JOIN_RUN, CASE_BLOCKING_EXCEPTION};
    private static final int[] ORDINALS = {0, 0, 0, 1, 6, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-67b54e408ca6f86b4ca5",
            "java-runtime-c8cbd0bc1aeb3eba6006",
            "java-runtime-1dd9223830c5409fc252",
            "java-runtime-1092d7e9660b84bc6e6d",
            "java-runtime-7e40b511fb6b6db584fd",
            "java-runtime-3bdb22d6cdef39f0336c"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowAPIConfigAndInstance",
            "EPLDataflowAPIStatistics",
            "EPLDataflowParameterInjectionCallback",
            "EPLDataflowOperatorInjectionCallback",
            "EPLDataflowInvalidJoinRun",
            "EPLDataflowBlockingException"};
    private static final int[] RECORD_COUNTS = {15, 10, 12, 3, 6, 3};
    private static final int TOTAL_RECORDS = 49;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME_CONFIG = "MyDataflow";
    private static final String FLOW_NAME_GRAPH = "MyGraph";
    private static final String FLOW_NAME_ONE = "MyDataFlowOne";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs and preambles from EPLDataflowAPIConfigAndInstance
    // (lines 47-52), EPLDataflowAPIStatistics (lines 36-38),
    // EPLDataflowAPIInstantiationOptions (lines 47-48/86-87) and
    // EPLDataflowAPIRunStartCancelJoin (lines 165-168/278-279): the config
    // graph concatenates the schema line with \n and ends with ";\n"; the
    // statistics graph carries an exact space before DefaultSupportCaptureOp;
    // the blocking graph concatenates the capture clause with NO separator
    // after the source clause's trailing '{}'.
    private static final String CONFIG_INSTANCE_EPL =
            "@public create objectarray schema MyEvent ();\n" +
            "@name('df') create dataflow " + FLOW_NAME_CONFIG + " " +
            "BeaconSource -> outdata<MyEvent> {" +
            "  iterations:1" +
            "}" +
            "EventBusSink(outdata) {};\n";
    private static final String S0_EPL = "@name('s0') select * from MyEvent";
    private static final String STATISTICS_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_GRAPH + " " +
            "DefaultSupportSourceOp -> outstream<SupportBean> {} " +
            "DefaultSupportCaptureOp(outstream) {}";
    private static final String SOME_TYPE_EPL = "@public create schema SomeType ()";
    private static final String INJECTION_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "MyOp -> outstream<SomeType> {propOne:'abc', propThree:'xyz'}";
    private static final String INVALID_JOIN_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "BeaconSource -> BeaconStream {iterations : 1}";
    private static final String BLOCKING_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "DefaultSupportSourceOp -> outstream<SomeType> {}" +
            "DefaultSupportCaptureOp(outstream) {}";

    // Byte-exact Java message texts asserted in-process, never recorded.
    private static final String MSG_INSTANTIATION_NOT_FOUND =
            "Dataflow saved configuration 'MyFirstFlow' could not be found";
    private static final String MSG_SAVE_NOT_FOUND = "Failed to locate data flow 'MyDataflow'";
    private static final String MSG_ALREADY_EXISTS =
            "Data flow saved configuration by name 'MyFirstFlow' already exists";
    private static final String MSG_INSTANCE_ALREADY_EXISTS = "Data flow instance name 'F1' already saved";
    private static final String MSG_JOIN_NOT_EXECUTED =
            "Data flow 'MyDataFlowOne' instance has not been executed, please use join after start or run";
    private static final String MSG_AFTER_CANCEL =
            "Data flow 'MyDataFlowOne' instance has been cancelled and cannot be run or started";
    private static final String MSG_SOURCE_GENERATED = "Support-graph-source generated exception: TestException";
    private static final String PRETTY_SOURCE_FULL = "DefaultSupportSourceOp#0() -> outstream<SupportBean>";
    private static final String PRETTY_SOURCE_BARE = "DefaultSupportSourceOp#0() -> outstream";
    private static final String PRETTY_CAPTURE = "DefaultSupportCaptureOp#1(outstream)";

    private DataflowLifecycleCoreScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowLifecycleCoreScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the six selected cases");
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
        // to this execution. The statistics and blocking-exception cases need
        // the SupportBean event type and the dataflow-util package import that
        // resolves DefaultSupportSourceOp / DefaultSupportCaptureOp in EPL
        // text; the injection cases need the MyOpForge class import that
        // resolves the EPL operator name MyOp by the Forge-suffix convention;
        // BeaconSource and EventBusSink resolve through the built-in
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT; the SomeType and MyEvent
        // types are declared by the byte-exact EPL preambles over the
        // RegressionPath. Internal timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        if (caseIndex == 1 || caseIndex == 5) {
            configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
            configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");
        } else if (caseIndex == 2 || caseIndex == 3) {
            configuration.getCommon().addImport(EPLDataflowAPIInstantiationOptions.MyOpForge.class);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-lifecycle-core-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            switch (caseIndex) {
                case 0:
                    runConfigAndInstance(configuration, runtime, writer);
                    break;
                case 1:
                    runStatistics(configuration, runtime, writer);
                    break;
                case 2:
                    runParameterInjection(configuration, runtime, writer);
                    break;
                case 3:
                    runOperatorInjection(configuration, runtime, writer);
                    break;
                case 4:
                    runInvalidJoinRun(configuration, runtime, writer);
                    break;
                default:
                    runBlockingException(configuration, runtime, writer);
                    break;
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
     * EPLDataflowAPIConfigAndInstance.run: saved-config and saved-instance
     * service CRUD around the byte-exact BeaconSource/EventBusSink graph.
     */
    private static void runConfigAndInstance(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPDataFlowService dataFlowRuntime = runtime.getDataFlowService();

        // suite line 29: the saved-config list starts empty.
        int savedConfigCount = dataFlowRuntime.getSavedConfigurations().length;
        if (savedConfigCount != 0) {
            throw new IllegalStateException("saved configurations held " + savedConfigCount + " entries, expected 0");
        }
        writer.addCount("saved-configs", savedConfigCount);

        // suite line 30: the named lookup misses — the null assert is
        // in-process and the record carries the normalized token "absent"
        // (mirrors the error-class token convention; the Go value surface
        // cannot emit a JSON null).
        if (dataFlowRuntime.getSavedConfiguration("MyFirstFlow") != null) {
            throw new IllegalStateException("getSavedConfiguration returned a value, expected null");
        }
        writer.add("lifecycle", "flow", "saved-config", Json.value("absent"));

        // suite line 31: removing a missing configuration reports false.
        if (dataFlowRuntime.removeSavedConfiguration("MyFirstFlow")) {
            throw new IllegalStateException("removeSavedConfiguration returned true, expected false");
        }
        writer.add("lifecycle", "flow", "removed", Json.value(false));

        // suite lines 32-37: instantiating a missing saved configuration
        // fails with the byte-exact message (asserted in-process).
        try {
            dataFlowRuntime.instantiateSavedConfiguration("MyFirstFlow");
            throw new IllegalStateException("expected EPDataFlowInstantiationException");
        } catch (EPDataFlowInstantiationException ex) {
            if (!MSG_INSTANTIATION_NOT_FOUND.equals(ex.getMessage())) {
                throw new IllegalStateException("instantiation message was " + ex.getMessage()
                        + ", expected " + MSG_INSTANTIATION_NOT_FOUND);
            }
        }
        writer.add("lifecycle", "flow", "instantiation.error-class", Json.value("instantiation-not-found"));

        // suite lines 38-43: saving against the missing dataflow 'MyDataflow'
        // fails with the byte-exact message (asserted in-process).
        try {
            dataFlowRuntime.saveConfiguration("MyFirstFlow", "x", FLOW_NAME_CONFIG, null);
            throw new IllegalStateException("expected EPDataFlowNotFoundException");
        } catch (EPDataFlowNotFoundException ex) {
            if (!MSG_SAVE_NOT_FOUND.equals(ex.getMessage())) {
                throw new IllegalStateException("save message was " + ex.getMessage()
                        + ", expected " + MSG_SAVE_NOT_FOUND);
            }
        }
        writer.add("lifecycle", "flow", "save.error-class", Json.value("save-not-found"));

        // suite lines 46-53: the byte-exact two-statement graph deployed under
        // the pinned deployment id (never recorded); the deployment carries
        // the 'df' dataflow statement.
        RegressionPath path = new RegressionPath();
        EPCompiled compiled = compile(configuration, runtime, CONFIG_INSTANCE_EPL, path);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);
        path.add(compiled);
        String deploymentId = deploymentId(runtime, "df");
        if (!FLOW_DEPLOYMENT_ID.equals(deploymentId)) {
            throw new IllegalStateException("deployment id for statement 'df' was " + deploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 57-61: the save succeeds and the stored configuration
        // carries the saved name and the dataflow name.
        dataFlowRuntime.saveConfiguration("MyFirstFlow", deploymentId, FLOW_NAME_CONFIG, null);
        if (dataFlowRuntime.getSavedConfigurations().length != 1) {
            throw new IllegalStateException("saved configurations held "
                    + dataFlowRuntime.getSavedConfigurations().length + " entries, expected 1");
        }
        writer.addCount("saved-configs", 1);
        EPDataFlowSavedConfiguration savedConfiguration =
                dataFlowRuntime.getSavedConfiguration(dataFlowRuntime.getSavedConfigurations()[0]);
        if (!"MyFirstFlow".equals(savedConfiguration.getSavedConfigurationName())) {
            throw new IllegalStateException("saved configuration name was "
                    + savedConfiguration.getSavedConfigurationName() + ", expected MyFirstFlow");
        }
        writer.add("lifecycle", "flow", "saved-config.name",
                Json.value(savedConfiguration.getSavedConfigurationName()));
        if (!FLOW_NAME_CONFIG.equals(savedConfiguration.getDataflowName())) {
            throw new IllegalStateException("saved dataflow name was " + savedConfiguration.getDataflowName()
                    + ", expected " + FLOW_NAME_CONFIG);
        }
        writer.add("lifecycle", "flow", "saved-config.dataflow-name", Json.value(savedConfiguration.getDataflowName()));

        // suite lines 62-67: the duplicate save is rejected (message asserted
        // in-process).
        try {
            dataFlowRuntime.saveConfiguration("MyFirstFlow", deploymentId, FLOW_NAME_CONFIG, null);
            throw new IllegalStateException("expected EPDataFlowAlreadyExistsException");
        } catch (EPDataFlowAlreadyExistsException ex) {
            if (!MSG_ALREADY_EXISTS.equals(ex.getMessage())) {
                throw new IllegalStateException("duplicate save message was " + ex.getMessage()
                        + ", expected " + MSG_ALREADY_EXISTS);
            }
        }
        writer.add("lifecycle", "flow", "save.error-class", Json.value("already-exists"));

        // suite lines 70-73: the remove reports true (recorded); the second
        // remove reports false and the list is empty again with a null lookup
        // (in-process asserts only, per the frozen record set).
        if (!dataFlowRuntime.removeSavedConfiguration("MyFirstFlow")) {
            throw new IllegalStateException("removeSavedConfiguration returned false, expected true");
        }
        writer.add("lifecycle", "flow", "removed", Json.value(true));
        if (dataFlowRuntime.removeSavedConfiguration("MyFirstFlow")) {
            throw new IllegalStateException("second removeSavedConfiguration returned true, expected false");
        }
        if (dataFlowRuntime.getSavedConfigurations().length != 0) {
            throw new IllegalStateException("saved configurations not empty after removal");
        }
        if (dataFlowRuntime.getSavedConfiguration("MyFirstFlow") != null) {
            throw new IllegalStateException("getSavedConfiguration returned a value after removal");
        }

        // suite lines 76-79: re-save, instantiate from the saved configuration
        // and attach the s0 listener to the MyEvent select.
        dataFlowRuntime.saveConfiguration("MyFirstFlow", deploymentId, FLOW_NAME_CONFIG, null);
        EPDataFlowInstance instance = dataFlowRuntime.instantiateSavedConfiguration("MyFirstFlow");
        EPCompiled s0Compiled = compile(configuration, runtime, S0_EPL, path);
        EPDeployment s0Deployment = runtime.getDeploymentService().deploy(s0Compiled);
        path.add(s0Compiled);
        EPStatement s0Statement = findStatement(s0Deployment, "s0");
        final boolean[] invoked = {false};
        s0Statement.addListener(new UpdateListener() {
            public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime rt) {
                invoked[0] = true;
            }
        });

        // suite line 79-80: the blocking run delivers the BeaconSource event
        // through EventBusSink to the s0 listener.
        instance.run();
        if (!invoked[0]) {
            throw new IllegalStateException("the s0 listener was not invoked");
        }
        writer.add("lifecycle", "flow", "listener-invoked", Json.value(true));

        // Strengthened (not in the suite): the blocking run completes the
        // single-iteration flow.
        assertState(instance, EPDataFlowState.COMPLETE);
        writer.addState(instance.getState());

        // suite lines 81-82: the saved-config list still carries exactly
        // 'MyFirstFlow' and the named lookup returns it (in-process asserts).
        String[] names = dataFlowRuntime.getSavedConfigurations();
        if (names.length != 1 || !"MyFirstFlow".equals(names[0])) {
            throw new IllegalStateException("saved configurations were "
                    + String.join(",", names) + ", expected [MyFirstFlow]");
        }
        if (dataFlowRuntime.getSavedConfiguration("MyFirstFlow") == null) {
            throw new IllegalStateException("getSavedConfiguration returned null after re-save");
        }

        // suite lines 85-89: the instance is saved under 'F1' (the
        // deployment-id equality assert stays in-process — deployment ids are
        // pinned but never recorded).
        dataFlowRuntime.saveInstance("F1", instance);
        String[] savedInstances = dataFlowRuntime.getSavedInstances();
        if (savedInstances.length != 1 || !"F1".equals(savedInstances[0])) {
            throw new IllegalStateException("saved instances were " + String.join(",", savedInstances)
                    + ", expected [F1]");
        }
        writer.addCount("saved-instances", 1);
        EPDataFlowInstance instanceFromSvc = dataFlowRuntime.getSavedInstance("F1");
        if (!deploymentId.equals(instanceFromSvc.getDataFlowDeploymentId())) {
            throw new IllegalStateException("saved instance deployment id was "
                    + instanceFromSvc.getDataFlowDeploymentId() + ", expected " + deploymentId);
        }
        if (!FLOW_NAME_CONFIG.equals(instanceFromSvc.getDataFlowName())) {
            throw new IllegalStateException("saved instance dataflow name was "
                    + instanceFromSvc.getDataFlowName() + ", expected " + FLOW_NAME_CONFIG);
        }

        // suite lines 90-96: the duplicate saveInstance is rejected (message
        // asserted in-process).
        try {
            dataFlowRuntime.saveInstance("F1", instance);
            throw new IllegalStateException("expected EPDataFlowAlreadyExistsException");
        } catch (EPDataFlowAlreadyExistsException ex) {
            if (!MSG_INSTANCE_ALREADY_EXISTS.equals(ex.getMessage())) {
                throw new IllegalStateException("duplicate saveInstance message was " + ex.getMessage()
                        + ", expected " + MSG_INSTANCE_ALREADY_EXISTS);
            }
        }
        writer.add("lifecycle", "flow", "save-instance.error-class", Json.value("instance-already-exists"));

        // suite lines 97-98: the instance remove reports true (recorded); the
        // second remove reports false (in-process assert only).
        if (!dataFlowRuntime.removeSavedInstance("F1")) {
            throw new IllegalStateException("removeSavedInstance returned false, expected true");
        }
        writer.add("lifecycle", "flow", "instance-removed", Json.value(true));
        if (dataFlowRuntime.removeSavedInstance("F1")) {
            throw new IllegalStateException("second removeSavedInstance returned true, expected false");
        }

        // suite line 100: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIStatistics.run: operator statistics for the source and
     * capture operators. The statement-property asserts are oracle-internal
     * (no Go introspection surface) and the time magnitudes are excluded from
     * records.
     */
    private static void runStatistics(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 36-38: byte-exact one-line graph deployed under the
        // pinned deployment id.
        deploy(runtime, compile(configuration, runtime, STATISTICS_EPL, null), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 39-40: statement-property asserts are oracle-internal
        // (the Go engine exposes no statement-type/creator introspection
        // surface) and are never recorded.
        EPStatement flowStatement = findStatement(runtime, "flow");
        Object statementType = flowStatement.getProperty(StatementProperty.STATEMENTTYPE);
        if (statementType != StatementType.CREATE_DATAFLOW) {
            throw new IllegalStateException("STATEMENTTYPE was " + statementType
                    + ", expected CREATE_DATAFLOW");
        }
        Object createObjectName = flowStatement.getProperty(StatementProperty.CREATEOBJECTNAME);
        if (!FLOW_NAME_GRAPH.equals(createObjectName)) {
            throw new IllegalStateException("CREATEOBJECTNAME was " + createObjectName
                    + ", expected " + FLOW_NAME_GRAPH);
        }

        // suite lines 42-49: the source submits SupportBean E1/E2, the capture
        // is fresh, and statistics are enabled.
        DefaultSupportSourceOp source =
                new DefaultSupportSourceOp(new Object[]{new SupportBean("E1", 1), new SupportBean("E2", 2)});
        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source, capture))
                .operatorStatistics(true)
                .cpuStatistics(true);
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_GRAPH, options);

        // suite line 51: the blocking run submits both events.
        instance.run();

        // suite lines 53-54: exactly two operator statistics.
        List<EPDataFlowInstanceOperatorStat> stats = instance.getStatistics().getOperatorStatistics();
        if (stats.size() != 2) {
            throw new IllegalStateException("operator statistics held " + stats.size() + " entries, expected 2");
        }
        writer.addCount("operator-stats-size", stats.size());

        // suite lines 56-63: the source statistics — the name and number are
        // recorded, the pretty print is asserted in-process with the full
        // <SupportBean> label and recorded in the NORMALIZED bare-port form,
        // the submission counts are recorded and the time magnitudes stay
        // in-process (timeOverall>0 and timeOverall==timePerPort[0]).
        EPDataFlowInstanceOperatorStat sourceStat = stats.get(0);
        if (!"DefaultSupportSourceOp".equals(sourceStat.getOperatorName())) {
            throw new IllegalStateException("source operator name was " + sourceStat.getOperatorName()
                    + ", expected DefaultSupportSourceOp");
        }
        writer.add("lifecycle", "flow", "operator-source.name", Json.value(sourceStat.getOperatorName()));
        if (sourceStat.getOperatorNumber() != 0) {
            throw new IllegalStateException("source operator number was " + sourceStat.getOperatorNumber()
                    + ", expected 0");
        }
        writer.add("lifecycle", "flow", "operator-source.number", Json.value(sourceStat.getOperatorNumber()));
        if (!PRETTY_SOURCE_FULL.equals(sourceStat.getOperatorPrettyPrint())) {
            throw new IllegalStateException("source pretty print was " + sourceStat.getOperatorPrettyPrint()
                    + ", expected " + PRETTY_SOURCE_FULL);
        }
        String barePort = sourceStat.getOperatorPrettyPrint()
                .substring(0, sourceStat.getOperatorPrettyPrint().indexOf('<'));
        if (!PRETTY_SOURCE_BARE.equals(barePort)) {
            throw new IllegalStateException("normalized bare-port form was " + barePort
                    + ", expected " + PRETTY_SOURCE_BARE);
        }
        writer.add("lifecycle", "flow", "operator-source.pretty-print", Json.value(barePort));
        if (sourceStat.getSubmittedOverallCount() != 2) {
            throw new IllegalStateException("source submitted overall count was "
                    + sourceStat.getSubmittedOverallCount() + ", expected 2");
        }
        writer.addCount("operator-source.submitted-overall", (int) sourceStat.getSubmittedOverallCount());
        long[] sourcePerPort = sourceStat.getSubmittedPerPortCount();
        if (sourcePerPort.length != 1 || sourcePerPort[0] != 2L) {
            throw new IllegalStateException("source per-port counts were not exactly [2]");
        }
        writer.addCount("operator-source.submitted-per-port", (int) sourcePerPort[0]);
        if (sourceStat.getTimeOverall() <= 0) {
            throw new IllegalStateException("source timeOverall was " + sourceStat.getTimeOverall()
                    + ", expected > 0");
        }
        if (sourceStat.getTimePerPort().length != 1 || sourceStat.getTimeOverall() != sourceStat.getTimePerPort()[0]) {
            throw new IllegalStateException("source timeOverall did not equal timePerPort[0]");
        }

        // suite lines 65-72: the capture statistics — the name, number, native
        // bare pretty print and the zero overall submission count are
        // recorded; the time magnitudes stay in-process and the per-port array
        // length is NOT recorded because the Go engine structurally sizes
        // SubmittedByPort by declared input ports (length 1 with value 0 for a
        // terminal), so the two sides cannot record the same length honestly.
        EPDataFlowInstanceOperatorStat destStat = stats.get(1);
        if (!"DefaultSupportCaptureOp".equals(destStat.getOperatorName())) {
            throw new IllegalStateException("capture operator name was " + destStat.getOperatorName()
                    + ", expected DefaultSupportCaptureOp");
        }
        writer.add("lifecycle", "flow", "operator-capture.name", Json.value(destStat.getOperatorName()));
        if (destStat.getOperatorNumber() != 1) {
            throw new IllegalStateException("capture operator number was " + destStat.getOperatorNumber()
                    + ", expected 1");
        }
        writer.add("lifecycle", "flow", "operator-capture.number", Json.value(destStat.getOperatorNumber()));
        if (!PRETTY_CAPTURE.equals(destStat.getOperatorPrettyPrint())) {
            throw new IllegalStateException("capture pretty print was " + destStat.getOperatorPrettyPrint()
                    + ", expected " + PRETTY_CAPTURE);
        }
        writer.add("lifecycle", "flow", "operator-capture.pretty-print", Json.value(destStat.getOperatorPrettyPrint()));
        if (destStat.getSubmittedOverallCount() != 0) {
            throw new IllegalStateException("capture submitted overall count was "
                    + destStat.getSubmittedOverallCount() + ", expected 0");
        }
        writer.addCount("operator-capture.submitted-overall", (int) destStat.getSubmittedOverallCount());
        if (destStat.getSubmittedPerPortCount().length != 0) {
            throw new IllegalStateException("capture per-port counts were not empty");
        }
        if (destStat.getTimeOverall() != 0) {
            throw new IllegalStateException("capture timeOverall was " + destStat.getTimeOverall() + ", expected 0");
        }
        if (destStat.getTimePerPort().length != 0) {
            throw new IllegalStateException("capture timePerPort was not empty");
        }

        // suite line 74: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIInstantiationOptions EPLDataflowParameterInjectionCallback
     * (ordinal 0): the parameter provider receives one context per declared
     * operator parameter and the resolved values land on the MyOp instance.
     * The static MyOp INSTANCES registry is cleared before the case and
     * harvested through getAndClearInstances() (mirroring the suite).
     */
    private static void runParameterInjection(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // Fresh static MyOp registry.
        EPLDataflowAPIInstantiationOptions.MyOp.getAndClearInstances();

        // suite lines 46-48: the byte-exact SomeType preamble and the
        // MyDataFlowOne MyOp graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, INJECTION_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 50-54: the parameter provider supplies propTwo:'def'
        // through the oracle harvesting subclass that mirrors the Java puts.
        HarvestingParameterProvider myParameterProvider = new HarvestingParameterProvider(
                Collections.<String, Object>singletonMap("propTwo", "def"));
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
        options.parameterProvider(myParameterProvider);
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_ONE, options);

        // suite line 55: harvest and clear the static registry — exactly one
        // MyOp was constructed at instantiation time.
        List<EPLDataflowAPIInstantiationOptions.MyOp> instances =
                EPLDataflowAPIInstantiationOptions.MyOp.getAndClearInstances();
        if (instances.size() != 1) {
            throw new IllegalStateException("MyOp registry held " + instances.size() + " instances, expected 1");
        }
        EPLDataflowAPIInstantiationOptions.MyOp myOp = instances.get(0);

        // suite lines 56-57: the resolved propOne comes from the graph and
        // propTwo from the parameter provider.
        if (!"abc".equals(myOp.getPropOne())) {
            throw new IllegalStateException("MyOp propOne was " + myOp.getPropOne() + ", expected abc");
        }
        if (!"def".equals(myOp.getPropTwo())) {
            throw new IllegalStateException("MyOp propTwo was " + myOp.getPropTwo() + ", expected def");
        }

        // suite lines 59-60: three provider contexts, one per declared
        // parameter, with propOne present.
        Map<String, EPDataFlowOperatorParameterProviderContext> contextMap = myParameterProvider.getContexts();
        if (contextMap.size() != 3) {
            throw new IllegalStateException("provider context map held " + contextMap.size()
                    + " entries, expected 3");
        }
        writer.addCount("provider-contexts", contextMap.size());
        if (contextMap.get("propOne") == null) {
            throw new IllegalStateException("provider context map is missing the propOne context");
        }
        List<String> parameterNames = new ArrayList<String>(contextMap.keySet());
        Collections.sort(parameterNames);
        if (parameterNames.size() != 3 || !"propOne".equals(parameterNames.get(0))
                || !"propThree".equals(parameterNames.get(1)) || !"propTwo".equals(parameterNames.get(2))) {
            throw new IllegalStateException("provider context parameters were " + parameterNames
                    + ", expected [propOne, propThree, propTwo] in sorted order");
        }
        for (String parameterName : parameterNames) {
            writer.add("lifecycle", "flow", "parameter-name", Json.value(parameterName));
        }

        // suite lines 62-67: the propTwo context carries the operator name,
        // the same factory instance as the constructed MyOp, the operator
        // number and the dataflow name (assertSame replayed as identity
        // equality).
        EPDataFlowOperatorParameterProviderContext contextPropTwo = contextMap.get("propTwo");
        if (!"propTwo".equals(contextPropTwo.getParameterName())) {
            throw new IllegalStateException("propTwo context parameter name was "
                    + contextPropTwo.getParameterName() + ", expected propTwo");
        }
        if (!"MyOp".equals(contextPropTwo.getOperatorName())) {
            throw new IllegalStateException("propTwo context operator name was "
                    + contextPropTwo.getOperatorName() + ", expected MyOp");
        }
        if (contextPropTwo.getFactory() != myOp.getFactory()) {
            throw new IllegalStateException("propTwo context factory is not the MyOp factory");
        }
        if (contextPropTwo.getOperatorNum() != 0) {
            throw new IllegalStateException("propTwo context operator num was "
                    + contextPropTwo.getOperatorNum() + ", expected 0");
        }
        if (!FLOW_NAME_ONE.equals(contextPropTwo.getDataFlowName())) {
            throw new IllegalStateException("propTwo context dataflow name was "
                    + contextPropTwo.getDataFlowName() + ", expected " + FLOW_NAME_ONE);
        }
        writer.add("lifecycle", "flow", "context.operator-name", Json.value(contextPropTwo.getOperatorName()));
        writer.add("lifecycle", "flow", "context.operator-num", Json.value(contextPropTwo.getOperatorNum()));
        writer.add("lifecycle", "flow", "context.dataflow-name", Json.value(contextPropTwo.getDataFlowName()));
        writer.add("lifecycle", "flow", "factory-identity.propTwo", Json.value(true));

        // suite lines 69-73: the propThree context carries the same operator
        // name, factory identity and operator number.
        EPDataFlowOperatorParameterProviderContext contextPropThree = contextMap.get("propThree");
        if (!"propThree".equals(contextPropThree.getParameterName())) {
            throw new IllegalStateException("propThree context parameter name was "
                    + contextPropThree.getParameterName() + ", expected propThree");
        }
        if (!"MyOp".equals(contextPropThree.getOperatorName())) {
            throw new IllegalStateException("propThree context operator name was "
                    + contextPropThree.getOperatorName() + ", expected MyOp");
        }
        if (contextPropThree.getFactory() != myOp.getFactory()) {
            throw new IllegalStateException("propThree context factory is not the MyOp factory");
        }
        if (contextPropThree.getOperatorNum() != 0) {
            throw new IllegalStateException("propThree context operator num was "
                    + contextPropThree.getOperatorNum() + ", expected 0");
        }
        writer.add("lifecycle", "flow", "factory-identity.propThree", Json.value(true));

        // Resolved parameter values read back from the MyOp instance: propOne
        // and propTwo are asserted by the suite, propThree (declared 'xyz',
        // provider returns null) is a strengthened assert.
        if (!"xyz".equals(myOp.getPropThree())) {
            throw new IllegalStateException("MyOp propThree was " + myOp.getPropThree() + ", expected xyz");
        }
        writer.add("lifecycle", "flow", "resolved.propOne", Json.value(myOp.getPropOne()));
        writer.add("lifecycle", "flow", "resolved.propTwo", Json.value(myOp.getPropTwo()));
        writer.add("lifecycle", "flow", "resolved.propThree", Json.value(myOp.getPropThree()));

        // suite line 75: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIInstantiationOptions EPLDataflowOperatorInjectionCallback
     * (ordinal 1): the operator provider receives one context for MyOp and
     * supplies the substitute operator; the MyOp instance its provide()
     * constructs is harvested and cleared from the static registry.
     */
    private static void runOperatorInjection(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // Fresh static MyOp registry.
        EPLDataflowAPIInstantiationOptions.MyOp.getAndClearInstances();

        // suite lines 85-87: the byte-exact SomeType preamble and the
        // MyDataFlowOne MyOp graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, INJECTION_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 89-93: the operator provider through the oracle
        // harvesting subclass that mirrors the Java puts.
        HarvestingOperatorProvider myOperatorProvider = new HarvestingOperatorProvider();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
        options.operatorProvider(myOperatorProvider);
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_ONE, options);

        // Strengthened (not asserted by the suite): the substitute MyOp the
        // provider constructed is the only registry entry; clear it.
        List<EPLDataflowAPIInstantiationOptions.MyOp> instances =
                EPLDataflowAPIInstantiationOptions.MyOp.getAndClearInstances();
        if (instances.size() != 1) {
            throw new IllegalStateException("MyOp registry held " + instances.size() + " instances, expected 1");
        }

        // suite lines 95-98: exactly one provider context naming MyOp and the
        // dataflow.
        Map<String, EPDataFlowOperatorProviderContext> contextMap = myOperatorProvider.getContexts();
        if (contextMap.size() != 1) {
            throw new IllegalStateException("operator provider context map held " + contextMap.size()
                    + " entries, expected 1");
        }
        writer.addCount("provider-contexts", contextMap.size());
        EPDataFlowOperatorProviderContext context = contextMap.get("MyOp");
        if (context == null) {
            throw new IllegalStateException("operator provider context map is missing the MyOp context");
        }
        if (!"MyOp".equals(context.getOperatorName())) {
            throw new IllegalStateException("operator context name was " + context.getOperatorName()
                    + ", expected MyOp");
        }
        writer.add("lifecycle", "flow", "context.operator-name", Json.value(context.getOperatorName()));
        if (!FLOW_NAME_ONE.equals(context.getDataFlowName())) {
            throw new IllegalStateException("operator context dataflow name was " + context.getDataFlowName()
                    + ", expected " + FLOW_NAME_ONE);
        }
        writer.add("lifecycle", "flow", "context.dataflow-name", Json.value(context.getDataFlowName()));

        // suite line 100: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowInvalidJoinRun (ordinal 6):
     * the invalid join/run/start state machine around the BeaconSource graph.
     */
    private static void runInvalidJoinRun(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite line 278: byte-exact BeaconSource graph deployed under the
        // pinned deployment id.
        deploy(runtime, compile(configuration, runtime, INVALID_JOIN_FLOW_EPL, null), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 281-283: the source instruction 5000 is never consumed —
        // the instance is never executed.
        DefaultSupportSourceOp source = new DefaultSupportSourceOp(new Object[]{5000});
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // Strengthened (the suite asserts no initial state here): the entry
        // state is INSTANTIATED, recorded before the first error.
        assertState(dfOne, EPDataFlowState.INSTANTIATED);
        writer.addState(dfOne.getState());

        // suite lines 286-293: join before execution fails with the byte-exact
        // IllegalStateException (asserted in-process).
        try {
            dfOne.join();
            throw new IllegalStateException("expected IllegalStateException from join");
        } catch (IllegalStateException ex) {
            if (!MSG_JOIN_NOT_EXECUTED.equals(ex.getMessage())) {
                throw new IllegalStateException("join message was " + ex.getMessage()
                        + ", expected " + MSG_JOIN_NOT_EXECUTED);
            }
        } catch (InterruptedException ex) {
            throw new RuntimeException(ex);
        }
        writer.add("lifecycle", "flow", "join.error-class", Json.value("join-not-executed"));

        // suite line 297: cancel moves the instance to CANCELLED (strengthened
        // state assert).
        dfOne.cancel();
        assertState(dfOne, EPDataFlowState.CANCELLED);
        writer.addState(dfOne.getState());

        // suite lines 300-312: run and start after cancel fail with the
        // byte-exact IllegalStateException (asserted in-process).
        try {
            dfOne.run();
            throw new IllegalStateException("expected IllegalStateException from run");
        } catch (IllegalStateException ex) {
            if (!MSG_AFTER_CANCEL.equals(ex.getMessage())) {
                throw new IllegalStateException("run message was " + ex.getMessage()
                        + ", expected " + MSG_AFTER_CANCEL);
            }
        }
        writer.add("lifecycle", "flow", "run.error-class", Json.value("run-after-cancel"));
        try {
            dfOne.start();
            throw new IllegalStateException("expected IllegalStateException from start");
        } catch (IllegalStateException ex) {
            if (!MSG_AFTER_CANCEL.equals(ex.getMessage())) {
                throw new IllegalStateException("start message was " + ex.getMessage()
                        + ", expected " + MSG_AFTER_CANCEL);
            }
        }
        writer.add("lifecycle", "flow", "start.error-class", Json.value("start-after-cancel"));

        // suite line 315: the second cancel is silently idempotent — the state
        // stays CANCELLED.
        dfOne.cancel();
        assertState(dfOne, EPDataFlowState.CANCELLED);
        writer.add("lifecycle", "flow", "cancel-idempotent", Json.value(true));

        // suite line 316: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowBlockingException (ordinal
     * 3): the blocking run rethrows the source throwable wrapped in
     * EPDataFlowExecutionException while the instance completes.
     */
    private static void runBlockingException(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 164-168: the byte-exact SomeType preamble and the
        // two-clause graph (capture clause concatenated with NO separator)
        // over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, BLOCKING_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 170-173: the source is instructed to throw
        // MyRuntimeException("TestException").
        DefaultSupportSourceOp src = new DefaultSupportSourceOp(
                new Object[]{new EPLDataflowAPIRunStartCancelJoin.MyRuntimeException("TestException")});
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(src, output));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 175-181: the blocking run throws
        // EPDataFlowExecutionException whose cause chain ends in the
        // MyRuntimeException and whose cause message is the wrapped source
        // text (asserted in-process, never recorded).
        try {
            dfOne.run();
            throw new IllegalStateException("expected EPDataFlowExecutionException from run");
        } catch (EPDataFlowExecutionException ex) {
            if (!(ex.getCause().getCause() instanceof EPLDataflowAPIRunStartCancelJoin.MyRuntimeException)) {
                throw new IllegalStateException("run exception root cause was "
                        + (ex.getCause().getCause() == null ? "null" : ex.getCause().getCause().getClass().getName())
                        + ", expected MyRuntimeException");
            }
            if (!MSG_SOURCE_GENERATED.equals(ex.getCause().getMessage())) {
                throw new IllegalStateException("run exception cause message was " + ex.getCause().getMessage()
                        + ", expected " + MSG_SOURCE_GENERATED);
            }
        }
        writer.add("lifecycle", "flow", "run.error-class", Json.value("execution-exception"));

        // suite line 183: the instance still completes.
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite line 184: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 185: undeployAll happens at the case boundary.
    }

    private static void assertState(EPDataFlowInstance instance, EPDataFlowState expected) {
        EPDataFlowState actual = instance.getState();
        if (actual != expected) {
            throw new IllegalStateException("instance state was " + actual + ", expected " + expected);
        }
    }

    /**
     * Compiles the EPL: with a RegressionPath the compileds collected on the
     * path are exported (RegressionEnvironmentBase.getArgsWithExportToPath
     * analog), without one the runtime path is exported.
     */
    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl, RegressionPath path)
            throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        if (path != null) {
            compilerArgs.getPath().getCompileds().addAll(path.getCompileds());
        } else {
            compilerArgs.getPath().add(runtime.getRuntimePath());
        }
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

    private static EPStatement findStatement(EPDeployment deployment, String statementName) {
        for (EPStatement stmt : deployment.getStatements()) {
            if (stmt.getName().equals(statementName)) {
                return stmt;
            }
        }
        throw new IllegalStateException("statement not found in deployment: " + statementName);
    }

    private static EPStatement findStatement(EPRuntime runtime, String statementName) throws Exception {
        for (String deployment : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deployment);
            EPStatement found = findStatement(info, statementName);
            if (found != null) {
                return found;
            }
        }
        throw new IllegalStateException("statement not found: " + statementName);
    }

    /**
     * Mirrors EPLDataflowAPIInstantiationOptions.MyParameterProvider (its
     * private context map is inaccessible from the oracle) — every provide()
     * call is recorded before delegating to the suite provider.
     */
    private static final class HarvestingParameterProvider
            extends EPLDataflowAPIInstantiationOptions.MyParameterProvider {
        private final Map<String, EPDataFlowOperatorParameterProviderContext> contexts =
                new LinkedHashMap<String, EPDataFlowOperatorParameterProviderContext>();

        private HarvestingParameterProvider(Map<String, Object> values) {
            super(values);
        }

        private Map<String, EPDataFlowOperatorParameterProviderContext> getContexts() {
            return contexts;
        }

        @Override
        public Object provide(EPDataFlowOperatorParameterProviderContext context) {
            contexts.put(context.getParameterName(), context);
            return super.provide(context);
        }
    }

    /**
     * Mirrors EPLDataflowAPIInstantiationOptions.MyOperatorProvider (its
     * private context map is inaccessible from the oracle) — every provide()
     * call is recorded before delegating to the suite provider.
     */
    private static final class HarvestingOperatorProvider
            extends EPLDataflowAPIInstantiationOptions.MyOperatorProvider {
        private final Map<String, EPDataFlowOperatorProviderContext> contexts =
                new LinkedHashMap<String, EPDataFlowOperatorProviderContext>();

        private Map<String, EPDataFlowOperatorProviderContext> getContexts() {
            return contexts;
        }

        @Override
        public Object provide(EPDataFlowOperatorProviderContext context) {
            contexts.put(context.getOperatorName(), context);
            return super.provide(context);
        }
    }

    /**
     * Emits {case, operation:"count", statement:"flow", sequence, time, name,
     * count} records for size observables, {case, operation:"state",
     * statement:"flow", sequence, time, name:"instance.state", value} records
     * for the instance-state reads and {case, operation, statement, sequence,
     * time, name, value} records for the lifecycle observables; the sequence
     * is case-local.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
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

        private void addState(EPDataFlowState state) {
            add("state", "flow", "instance.state", Json.value(state.name()));
        }

        private long count() {
            return sequence;
        }
    }
}
