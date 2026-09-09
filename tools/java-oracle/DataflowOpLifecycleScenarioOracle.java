import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.annotation.Audit;
import com.espertech.esper.common.client.annotation.Name;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.interfaces.DataFlowOpCloseContext;
import com.espertech.esper.common.internal.epl.dataflow.interfaces.DataFlowOpFactoryInitializeContext;
import com.espertech.esper.common.internal.epl.dataflow.interfaces.DataFlowOpForgeInitializeContext;
import com.espertech.esper.common.internal.epl.dataflow.interfaces.DataFlowOpInitializeContext;
import com.espertech.esper.common.internal.epl.dataflow.interfaces.DataFlowOpOpenContext;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowAPIOpLifecycle;
import com.espertech.esper.regressionlib.support.dataflow.MyLineFeedSource;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowAPIOpLifecycle differential
 * chain (work unit 4.357): dataflow operator lifecycle stages — instantiated,
 * configured property, operator context (dataflow name, instance id, user
 * object, operator number/name), open, source next markers, onInput
 * deliveries, and close — ending in the terminal COMPLETE state.
 *
 * Replay-shape adaptations frozen with the scouts (both sides implement the
 * same shape):
 *
 * 1. Forge-stage observables (compile-time initializeForge context:
 *    annotations, port maps) and deploy-time factory-initialize observables
 *    (statementContext) are DROPPED — the Go engine has no two-phase
 *    forge/factory split. This oracle still drains the forge list after
 *    compile and the factory list after deploy and asserts their shape
 *    (mirroring the original suite assertions) before discarding the items.
 * 2. The source-only graph of the second execution gains a
 *    DefaultSupportCaptureOp sink on outstream (Go's submit requires a
 *    connected edge); the captured rows are pinned in the trace as one
 *    capture record.
 * 3. The third execution's post-compileDeploy empty-drain assertion (line
 *    135 of the suite) is a static-list mechanism artifact and is dropped:
 *    the oracle drains and asserts emptiness but records nothing.
 *
 * Operator-provider note: the provider of the second execution carries the
 * DefaultSupportCaptureOp instance only; SupportGraphSource is realized
 * through SupportGraphSourceFactory.operator(ctx). Supplying a
 * SupportGraphSource instance through the provider would bypass the factory
 * (DataflowInstantiator.instantiateOperator early-return) and erase the
 * operator-context observables behind records 2-7; the Go sourceFactory
 * realizes the source runtime the same way.
 */
public final class DataflowOpLifecycleScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-op-lifecycle";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIOpLifecycle.java";
    private static final String TYPE_EVENT = "type-event";
    private static final String FLOW_GRAPH_SOURCE = "flow-graph-source";
    private static final String FLOW_GRAPH_OPERATOR = "flow-graph-operator";
    private static final String[] CASES = {TYPE_EVENT, FLOW_GRAPH_SOURCE, FLOW_GRAPH_OPERATOR};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-ff077635ccc5082a06a1",
            "java-runtime-12a29443119ff32fd057",
            "java-runtime-4bf013f93c9464b8e890"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowTypeEvent",
            "EPLDataflowFlowGraphSource",
            "EPLDataflowFlowGraphOperator"};
    private static final int[] RECORD_COUNTS = {1, 14, 6};
    private static final int TOTAL_RECORDS = 21;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String SOURCE_STATEMENT = "flow:SupportGraphSource";
    private static final String OPERATOR_STATEMENT = "flow:SupportOperator";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String SOURCE_EPL =
            "@name('flow') create dataflow MyDataFlow @Name('Goodie') @Audit SupportGraphSource "
                    + "-> outstream<SupportBean> {propOne:'abc'} DefaultSupportCaptureOp(outstream) {}";
    private static final String OPERATOR_EPL =
            "@name('flow') create dataflow MyDataFlow MyLineFeedSource -> outstream {} "
                    + "SupportOperator(outstream) {propOne:'abc'}";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    private DataflowOpLifecycleScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowOpLifecycleScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the three selected cases");
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

        // Session configuration per TestSuiteEPLDataflow.configure: the SupportBean
        // type plus the imports that resolve the dataflow operator simple names in
        // EPL text (the dataflow-util package import resolves DefaultSupportCaptureOp).
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(EPLDataflowAPIOpLifecycle.SupportGraphSourceForge.class);
        configuration.getCommon().addImport(EPLDataflowAPIOpLifecycle.SupportOperatorForge.class);
        configuration.getCommon().addImport(EPLDataflowAPIOpLifecycle.MyCaptureOutputPortOpForge.class);
        configuration.getCommon().addImport(MyLineFeedSource.class);
        // resolves MyLineFeedSourceForge (the "+Forge" operator-name convention)
        configuration.getCommon().addImport(MyLineFeedSource.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-op-lifecycle-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (TYPE_EVENT.equals(caseName)) {
                runTypeEvent(configuration, runtime, writer);
            } else if (FLOW_GRAPH_SOURCE.equals(caseName)) {
                runFlowGraphSource(configuration, runtime, writer);
            } else if (FLOW_GRAPH_OPERATOR.equals(caseName)) {
                runFlowGraphOperator(configuration, runtime, writer);
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
     * EPLDataflowTypeEvent: compile-only (no deploy) two-statement compilation;
     * the compile-time forge stores the declared output port and the observed
     * value is the declared event type NAME.
     */
    private static void runTypeEvent(Configuration configuration, EPRuntime runtime, TraceWriter writer) throws Exception {
        EPCompiled compiled = compile(configuration, runtime,
                "create schema MySchema(key string, value int);\n"
                        + "@name('flow') create dataflow MyDataFlowOne MyCaptureOutputPortOp -> outstream<EventBean<MySchema>> {}");
        String portTypeName = EPLDataflowAPIOpLifecycle.MyCaptureOutputPortOpForge.getPort()
                .getOptionalDeclaredType().getEventType().getName();
        if (!"MySchema".equals(portTypeName)) {
            throw new IllegalStateException("declared output port type name was " + portTypeName + ", expected MySchema");
        }
        writer.add("port-type", "flow:MyCaptureOutputPortOp", "outstream", Json.value(portTypeName));
    }

    /**
     * EPLDataflowFlowGraphSource with the capture-sink adaptation; records the
     * operator-context observables after instantiate and the open/next/close
     * stages plus the captured rows after the blocking run.
     */
    private static void runFlowGraphSource(Configuration configuration, EPRuntime runtime, TraceWriter writer) throws Exception {
        EPLDataflowAPIOpLifecycle.SupportGraphSourceForge.getAndResetLifecycle();
        EPLDataflowAPIOpLifecycle.SupportGraphSourceFactory.getAndResetLifecycle();
        EPLDataflowAPIOpLifecycle.SupportGraphSource.getAndResetLifecycle();

        EPCompiled compiled = compile(configuration, runtime, SOURCE_EPL);

        // Forge-stage drain (dropped observable): shape-checked, then discarded.
        List<Object> forgeEvents = EPLDataflowAPIOpLifecycle.SupportGraphSourceForge.getAndResetLifecycle();
        if (forgeEvents.size() != 3
                || !"instantiated".equals(forgeEvents.get(0))
                || !"setPropOne=abc".equals(forgeEvents.get(1))
                || !(forgeEvents.get(2) instanceof DataFlowOpForgeInitializeContext)) {
            throw new IllegalStateException("forge lifecycle after compile was " + forgeEvents);
        }
        DataFlowOpForgeInitializeContext forgeCtx = (DataFlowOpForgeInitializeContext) forgeEvents.get(2);
        if (forgeCtx.getInputPorts().size() != 0
                || forgeCtx.getOutputPorts().size() != 1
                || !"outstream".equals(forgeCtx.getOutputPorts().get(0).getStreamName())
                || !"SupportBean".equals(forgeCtx.getOutputPorts().get(0).getOptionalDeclaredType().getEventType().getName())
                || forgeCtx.getOperatorAnnotations().length != 2
                || !"Goodie".equals(((Name) forgeCtx.getOperatorAnnotations()[0]).value())
                || !(forgeCtx.getOperatorAnnotations()[1] instanceof Audit)
                || !"MyDataFlow".equals(forgeCtx.getDataflowName())
                || forgeCtx.getOperatorNumber() != 0) {
            throw new IllegalStateException("forge initialize context did not match the suite contract");
        }

        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        // Factory-initialize drain (dropped observable): shape-checked, then discarded.
        List<Object> factoryEvents = EPLDataflowAPIOpLifecycle.SupportGraphSourceFactory.getAndResetLifecycle();
        if (factoryEvents.size() != 3
                || !"instantiated".equals(factoryEvents.get(0))
                || !"setPropOne=abc".equals(factoryEvents.get(1))
                || !(factoryEvents.get(2) instanceof DataFlowOpFactoryInitializeContext)) {
            throw new IllegalStateException("factory lifecycle after deploy was " + factoryEvents);
        }
        DataFlowOpFactoryInitializeContext deployedFactoryCtx = (DataFlowOpFactoryInitializeContext) factoryEvents.get(2);
        if (!"MyDataFlow".equals(deployedFactoryCtx.getDataFlowName())
                || deployedFactoryCtx.getOperatorNumber() != 0
                || deployedFactoryCtx.getStatementContext() == null) {
            throw new IllegalStateException("factory initialize context did not match the suite contract");
        }

        // Instantiate: the provider carries the capture sink; SupportGraphSource is
        // factory-realized so the operator-context observables exist.
        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .dataFlowInstanceId("id1")
                .dataFlowInstanceUserObject("myobject")
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, "MyDataFlow", options);

        List<Object> operatorCtxEvents = EPLDataflowAPIOpLifecycle.SupportGraphSourceFactory.getAndResetLifecycle();
        if (operatorCtxEvents.size() != 1 || !(operatorCtxEvents.get(0) instanceof DataFlowOpInitializeContext)) {
            throw new IllegalStateException("factory lifecycle after instantiate was " + operatorCtxEvents);
        }
        DataFlowOpInitializeContext opCtx = (DataFlowOpInitializeContext) operatorCtxEvents.get(0);
        if (!"MyDataFlow".equals(opCtx.getDataFlowName())
                || !"id1".equals(opCtx.getDataFlowInstanceId())
                || opCtx.getAgentInstanceContext() == null
                || !"myobject".equals(opCtx.getDataflowInstanceUserObject())
                || opCtx.getOperatorNumber() != 0
                || !"SupportGraphSource".equals(opCtx.getOperatorName())
                || !(opCtx.getDataFlowOperatorFactory() instanceof EPLDataflowAPIOpLifecycle.SupportGraphSourceFactory)) {
            throw new IllegalStateException("operator initialize context did not match the suite contract");
        }
        String propOne = ((EPLDataflowAPIOpLifecycle.SupportGraphSourceFactory) opCtx.getDataFlowOperatorFactory()).getPropOne();

        List<Object> sourceInstantiateEvents = EPLDataflowAPIOpLifecycle.SupportGraphSource.getAndResetLifecycle();
        if (sourceInstantiateEvents.size() != 1 || !"instantiated".equals(sourceInstantiateEvents.get(0))) {
            throw new IllegalStateException("source lifecycle after instantiate was " + sourceInstantiateEvents);
        }

        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("instantiated"));
        writer.add("lifecycle", SOURCE_STATEMENT, "propOne", Json.value(propOne));
        writer.add("lifecycle", SOURCE_STATEMENT, "dataflowName", Json.value(opCtx.getDataFlowName()));
        writer.add("lifecycle", SOURCE_STATEMENT, "instanceId", Json.value(opCtx.getDataFlowInstanceId()));
        writer.add("lifecycle", SOURCE_STATEMENT, "userObject", Json.value(String.valueOf(opCtx.getDataflowInstanceUserObject())));
        writer.add("lifecycle", SOURCE_STATEMENT, "operatorNumber", Json.value(opCtx.getOperatorNumber()));
        writer.add("lifecycle", SOURCE_STATEMENT, "operatorName", Json.value(opCtx.getOperatorName()));

        // Run: blocking; the source opens, polls three times, and closes.
        instance.run();

        List<Object> sourceRunEvents = EPLDataflowAPIOpLifecycle.SupportGraphSource.getAndResetLifecycle();
        if (sourceRunEvents.size() != 5
                || !(sourceRunEvents.get(0) instanceof DataFlowOpOpenContext)
                || !"next(numrows=0)".equals(sourceRunEvents.get(1))
                || !"next(numrows=1)".equals(sourceRunEvents.get(2))
                || !"next(numrows=2)".equals(sourceRunEvents.get(3))
                || !(sourceRunEvents.get(4) instanceof DataFlowOpCloseContext)) {
            throw new IllegalStateException("source lifecycle after run was " + sourceRunEvents);
        }
        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("open"));
        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("next(numrows=0)"));
        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("next(numrows=1)"));
        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("next(numrows=2)"));
        writer.add("lifecycle", SOURCE_STATEMENT, "stage", Json.value("close"));

        // Capture: the source submitted the raw strings "E1"/"E2" then the final
        // marker. SupportGraphSourceForge carries no @DataFlowOpProvideSignal, so
        // the final marker is not forwarded to the sink's onSignal and the rows
        // stay in the sink's current batch; the Go capture records every
        // delivered row without batch semantics, so read the current rows.
        Object[] capturedRows = capture.getCurrentAndReset();
        if (capturedRows.length != 2) {
            throw new IllegalStateException("capture held " + capturedRows.length + " rows, expected 2");
        }
        JsonArray rows = new JsonArray();
        for (Object row : capturedRows) {
            if (!(row instanceof String)) {
                throw new IllegalStateException("captured row was not a String: " + row);
            }
            rows.add(new JsonObject().add("kind", "row").add("fields", new JsonObject().add("item", (String) row)));
        }
        writer.addCapture("capture", CAPTURE_STATEMENT, rows);

        emitState(writer, instance);
    }

    /**
     * EPLDataflowFlowGraphOperator: MyLineFeedSource is injected through the
     * operator provider; SupportOperator is factory-realized and records the
     * instantiated/open/onInput/close stages.
     */
    private static void runFlowGraphOperator(Configuration configuration, EPRuntime runtime, TraceWriter writer) throws Exception {
        EPLDataflowAPIOpLifecycle.SupportOperator.getAndResetLifecycle();

        EPCompiled compiled = compile(configuration, runtime, OPERATOR_EPL);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        // CompileDeploy-time drain: empty per suite line 135; static-list
        // mechanism artifact, dropped from the trace.
        List<Object> preInstantiation = EPLDataflowAPIOpLifecycle.SupportOperator.getAndResetLifecycle();
        if (!preInstantiation.isEmpty()) {
            throw new IllegalStateException("operator lifecycle before instantiate was " + preInstantiation);
        }

        MyLineFeedSource source = new MyLineFeedSource(Arrays.asList("abc", "def").iterator());
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, "MyDataFlow", options);

        List<Object> instantiateEvents = EPLDataflowAPIOpLifecycle.SupportOperator.getAndResetLifecycle();
        if (instantiateEvents.size() != 1 || !"instantiated".equals(instantiateEvents.get(0))) {
            throw new IllegalStateException("operator lifecycle after instantiate was " + instantiateEvents);
        }
        writer.add("lifecycle", OPERATOR_STATEMENT, "stage", Json.value("instantiated"));

        // Run: blocking; onInput receives the single-element Object[] payloads.
        instance.run();

        List<Object> runEvents = EPLDataflowAPIOpLifecycle.SupportOperator.getAndResetLifecycle();
        if (runEvents.size() != 4
                || !(runEvents.get(0) instanceof DataFlowOpOpenContext)
                || !isPayload(runEvents.get(1), "abc")
                || !isPayload(runEvents.get(2), "def")
                || !(runEvents.get(3) instanceof DataFlowOpCloseContext)) {
            throw new IllegalStateException("operator lifecycle after run was " + runEvents);
        }
        writer.add("lifecycle", OPERATOR_STATEMENT, "stage", Json.value("open"));
        writer.add("lifecycle", OPERATOR_STATEMENT, "onInput", Json.value("abc"));
        writer.add("lifecycle", OPERATOR_STATEMENT, "onInput", Json.value("def"));
        writer.add("lifecycle", OPERATOR_STATEMENT, "stage", Json.value("close"));

        emitState(writer, instance);
    }

    private static boolean isPayload(Object event, String expected) {
        return event instanceof Object[] && ((Object[]) event).length == 1
                && expected.equals(((Object[]) event)[0]);
    }

    private static void emitState(TraceWriter writer, EPDataFlowInstance instance) {
        if (instance.getState() != EPDataFlowState.COMPLETE) {
            throw new IllegalStateException("dataflow instance ended in state " + instance.getState() + ", expected COMPLETE");
        }
        writer.add("state", "flow", "state", Json.value("complete"));
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
     * Emits {case, operation, statement, sequence, time, name, value} records
     * for lifecycle/port-type/state observables and {case, operation,
     * statement, sequence, time, new:[...]} records for captures.
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

        private void addCapture(String operation, String statement, JsonArray rows) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement)
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
