import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowExceptionContext;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowAPIExceptions;
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
 * Direct Esper 9.0.0 oracle for the EPLDataflowAPIExceptions differential
 * chain (work unit 4.373, dataflow-exceptions): the dataflow
 * exception-propagation chain where one synchronous-throwing graph source and
 * one synchronous-throwing operator each reach the registered
 * EPDataFlowExceptionHandler exactly once while the instance completes.
 *
 * exceptions (the whole class is ONE execution, the 'direct' variant) replays
 * the run() body in source order as two flows hosted by one runtime, with the
 * suite's static handler-context list cleared before flow A and between the
 * flows (suite lines 39/64). Flow A (source-throw) deploys the byte-exact
 * one-line MyDataFlow DefaultSupportSourceOp -> outstream<SupportBean> graph
 * under the pinned deployment id "flow", instantiates with a
 * DefaultSupportSourceOp instructed to throw RuntimeException("My-Exception-
 * Is-Here") supplied through DefaultSupportGraphOpProvider plus the suite's
 * own MyExceptionHandler, and drives start()+join() — deterministic
 * completion; run() is NOT used because it rethrows the execution exception
 * on this path. The source wrapper surfaces the throwable message
 * "Support-graph-source generated exception: My-Exception-Is-Here", the
 * handler context names operator DefaultSupportSourceOp number 0 with the
 * pretty print "DefaultSupportSourceOp#0() -> outstream<SupportBean>", and
 * the instance state is COMPLETE. Flow B (operator-throw) deploys the
 * byte-exact two-clause graph whose operator clause is concatenated with NO
 * separator after the source clause's trailing '{}' (suite lines 67-68),
 * instantiates with a DefaultSupportSourceOp instructed to submit
 * SupportBean{"E1",1}, and drives start()+join(): MyExceptionOp.onInput
 * throws RuntimeException("Operator-thrown-exception"), the engine routes the
 * RAW unwrapped throwable to the handler (which swallows it, so submit
 * returns normally and exactly one context accumulates), the context names
 * operator MyExceptionOp number 1 with the native bare pretty print
 * "MyExceptionOp#1(outstream)", and the instance state is COMPLETE
 * (strengthened assert, not in the suite). Java's cancel-after-COMPLETE is a
 * silent no-op (EPDataFlowInstanceImpl.cancel lines 185-188) while the Go
 * Cancel returns an error there, so NO cancel record is emitted — the frozen
 * disposition.
 *
 * Record protocol (state/count/value records only — NO data rows): each flow
 * emits the same six-record shape after the in-process asserts. The count
 * observable is {case, operation:"count", statement:"flow", sequence, time,
 * name:"handler-contexts", count:1}; the handler-context observables are
 * {case, operation:"lifecycle", statement:"flow", sequence, time, name, value}
 * records for handler.error-class (the class token "source-error" /
 * "operator-throw" — the byte-exact throwable messages are asserted
 * in-process and never recorded, matching the invalidity policy),
 * handler.operator-name, handler.operator-number and
 * handler.operator-pretty-print; the lifecycle observable is {case,
 * operation:"state", statement:"flow", sequence, time, name:"instance.state",
 * value:"COMPLETE"}. Flow A's pretty-print record carries the NORMALIZED
 * bare-port form "DefaultSupportSourceOp#0() -> outstream" with the
 * <SupportBean> type label stripped (Go renders package-free bare ports);
 * the byte-exact Java string is asserted in-process. Flow B's pretty print
 * "MyExceptionOp#1(outstream)" is already in the native bare form. Sequence
 * is case-local and restarts at 1; the time is the fixed epoch
 * 1970-01-01T00:00:00Z. Session configuration mirrors
 * TestSuiteEPLDataflow.configure restricted to this execution: the SupportBean
 * event type, the com.espertech.esper.common.internal.epl.dataflow.util.*
 * package import that resolves DefaultSupportSourceOp in EPL text, and the
 * EPLDataflowAPIExceptions.MyExceptionOpForge class import that resolves the
 * EPL operator name MyExceptionOp by the Forge-suffix convention. Internal
 * timer off, epoch initialization, one fresh runtime destroyed in finally.
 */
public final class DataflowExceptionsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-exceptions";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIExceptions.java";
    private static final String CASE_EXCEPTIONS = "exceptions";
    private static final String[] CASES = {CASE_EXCEPTIONS};
    private static final int[] ORDINALS = {0};
    private static final String[] RUNTIME_IDS = {"java-runtime-c1d106a7e1cc70655006"};
    private static final String[] EXECUTION_NAMES = {"EPLDataflowAPIExceptions"};
    private static final int[] RECORD_COUNTS = {12};
    private static final int TOTAL_RECORDS = 12;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME = "MyDataFlow";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs from EPLDataflowAPIExceptions (lines 42 and 67-68):
    // flow A is one line; flow B concatenates the MyExceptionOp clause with
    // NO separator after the source clause's trailing '{}'.
    private static final String FLOW_A_EPL =
            "@name('flow') create dataflow " + FLOW_NAME + " DefaultSupportSourceOp -> outstream<SupportBean> {}";
    private static final String FLOW_B_EPL =
            "@name('flow') create dataflow " + FLOW_NAME + " DefaultSupportSourceOp -> outstream<SupportBean> {}" +
            "MyExceptionOp(outstream) {}";

    private DataflowExceptionsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowExceptionsScenarioOracle <scenario.json>");
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
        // to this execution: the SupportBean event type (suite configure line
        // 138-141), the dataflow-util package import (suite configure lines
        // 153-154) that resolves DefaultSupportSourceOp in EPL text, and the
        // MyExceptionOpForge class import (suite configure line 160) that
        // resolves the EPL operator name MyExceptionOp by the Forge-suffix
        // convention. Internal timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(EPLDataflowAPIExceptions.MyExceptionOpForge.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-exceptions-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            runExceptions(runtime, configuration, writer);
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
     * EPLDataflowAPIExceptions.run (suite lines 38-87): the source-throw flow
     * followed by the operator-throw flow, hosted by one runtime with the
     * suite's static handler-context list cleared before flow A (suite line
     * 39) and between the flows (suite line 64).
     */
    private static void runExceptions(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite line 39: fresh static handler-context list.
        EPLDataflowAPIExceptions.MyExceptionHandler.getContexts().clear();

        // suite lines 41-49: byte-exact one-line source-throw graph deployed
        // under the pinned deployment id; the source is instructed to throw
        // RuntimeException("My-Exception-Is-Here") and supplied through the
        // operator provider together with the suite's own exception handler.
        deploy(runtime, compile(configuration, runtime, FLOW_A_EPL), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        DefaultSupportSourceOp op = new DefaultSupportSourceOp(new Object[]{new RuntimeException("My-Exception-Is-Here")});
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
        options.operatorProvider(new DefaultSupportGraphOpProvider(op));
        EPLDataflowAPIExceptions.MyExceptionHandler handler = new EPLDataflowAPIExceptions.MyExceptionHandler();
        options.exceptionHandler(handler);
        EPDataFlowInstance df = runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME, options);

        // suite lines 51-53: start()+join() gives deterministic completion;
        // run() is NOT used because it rethrows the execution exception on
        // this source-throw path (the suite sleeps 100ms instead).
        df.start();
        df.join();
        assertState(df, EPDataFlowState.COMPLETE);

        // suite lines 55-61: exactly one handler context carrying the
        // source-throw chain — the wrapped message, the operator coordinates
        // and the byte-exact pretty print including the <SupportBean> type
        // label. These asserts are in-process and never recorded.
        List<EPDataFlowExceptionContext> contexts = EPLDataflowAPIExceptions.MyExceptionHandler.getContexts();
        if (contexts.size() != 1) {
            throw new IllegalStateException("handler held " + contexts.size() + " contexts, expected 1");
        }
        EPDataFlowExceptionContext context = contexts.get(0);
        if (!FLOW_NAME.equals(context.getDataFlowName())) {
            throw new IllegalStateException("context dataFlowName was " + context.getDataFlowName()
                    + ", expected " + FLOW_NAME);
        }
        if (!"DefaultSupportSourceOp".equals(context.getOperatorName())) {
            throw new IllegalStateException("context operatorName was " + context.getOperatorName()
                    + ", expected DefaultSupportSourceOp");
        }
        if (!Integer.valueOf(0).equals(context.getOperatorNumber())) {
            throw new IllegalStateException("context operatorNumber was " + context.getOperatorNumber()
                    + ", expected 0");
        }
        Object prettyPrint = context.getOperatorPrettyPrint();
        if (!"DefaultSupportSourceOp#0() -> outstream<SupportBean>".equals(prettyPrint)) {
            throw new IllegalStateException("context operatorPrettyPrint was " + prettyPrint
                    + ", expected DefaultSupportSourceOp#0() -> outstream<SupportBean>");
        }
        if (!"Support-graph-source generated exception: My-Exception-Is-Here".equals(
                context.getThrowable().getMessage())) {
            throw new IllegalStateException("context throwable message was " + context.getThrowable().getMessage()
                    + ", expected 'Support-graph-source generated exception: My-Exception-Is-Here'");
        }

        // Records 1-6: the handler-context count, the error-class token, the
        // operator coordinates, the NORMALIZED bare-port pretty print (the
        // <SupportBean> type label stripped — Go renders package-free bare
        // ports) and the COMPLETE instance state.
        writer.addCount("handler-contexts", contexts.size());
        writer.add("lifecycle", "flow", "handler.error-class", Json.value("source-error"));
        writer.add("lifecycle", "flow", "handler.operator-name", Json.value(context.getOperatorName()));
        writer.add("lifecycle", "flow", "handler.operator-number",
                Json.value(((Number) context.getOperatorNumber()).intValue()));
        String barePort = ((String) prettyPrint).substring(0, ((String) prettyPrint).indexOf('<'));
        if (!"DefaultSupportSourceOp#0() -> outstream".equals(barePort)) {
            throw new IllegalStateException("normalized bare-port form was " + barePort
                    + ", expected DefaultSupportSourceOp#0() -> outstream");
        }
        writer.add("lifecycle", "flow", "handler.operator-pretty-print", Json.value(barePort));
        writer.addState(df.getState());

        // suite lines 62-64: cancel after COMPLETE (silent no-op in Java,
        // NOT replayed — the frozen disposition), undeploy the flow
        // deployment (undeployAll is equivalent here — it is the only
        // deployment), and clear the static handler-context list.
        runtime.getDeploymentService().undeployAll();
        EPLDataflowAPIExceptions.MyExceptionHandler.getContexts().clear();

        // suite lines 66-75: byte-exact two-clause operator-throw graph — the
        // MyExceptionOp clause is concatenated with NO separator after the
        // source clause's trailing '{}' — deployed under the same pinned
        // deployment id; the source is instructed to submit
        // SupportBean{"E1",1} and supplied through the operator provider
        // together with a fresh handler instance (the same static list).
        deploy(runtime, compile(configuration, runtime, FLOW_B_EPL), FLOW_DEPLOYMENT_ID);
        flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        DefaultSupportSourceOp opTwo = new DefaultSupportSourceOp(new Object[]{new SupportBean("E1", 1)});
        EPDataFlowInstantiationOptions optionsTwo = new EPDataFlowInstantiationOptions();
        optionsTwo.operatorProvider(new DefaultSupportGraphOpProvider(opTwo));
        EPLDataflowAPIExceptions.MyExceptionHandler handlerTwo = new EPLDataflowAPIExceptions.MyExceptionHandler();
        optionsTwo.exceptionHandler(handlerTwo);
        EPDataFlowInstance dfTwo = runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME, optionsTwo);

        // suite lines 77-78: start()+join(); the engine routes the RAW
        // unwrapped operator throwable to the handler which swallows it, so
        // submit returns normally and the source's exhausted instruction list
        // emits the final marker — deterministic COMPLETE (strengthened
        // assert, not in the suite).
        dfTwo.start();
        dfTwo.join();
        assertState(dfTwo, EPDataFlowState.COMPLETE);

        // suite lines 80-86: exactly one handler context (the swallow
        // guarantees it) carrying the raw operator-throw chain.
        List<EPDataFlowExceptionContext> contextsTwo = EPLDataflowAPIExceptions.MyExceptionHandler.getContexts();
        if (contextsTwo.size() != 1) {
            throw new IllegalStateException("handler held " + contextsTwo.size() + " contexts, expected 1");
        }
        EPDataFlowExceptionContext contextTwo = contextsTwo.get(0);
        if (!FLOW_NAME.equals(contextTwo.getDataFlowName())) {
            throw new IllegalStateException("context dataFlowName was " + contextTwo.getDataFlowName()
                    + ", expected " + FLOW_NAME);
        }
        if (!"MyExceptionOp".equals(contextTwo.getOperatorName())) {
            throw new IllegalStateException("context operatorName was " + contextTwo.getOperatorName()
                    + ", expected MyExceptionOp");
        }
        if (!Integer.valueOf(1).equals(contextTwo.getOperatorNumber())) {
            throw new IllegalStateException("context operatorNumber was " + contextTwo.getOperatorNumber()
                    + ", expected 1");
        }
        Object prettyPrintTwo = contextTwo.getOperatorPrettyPrint();
        if (!"MyExceptionOp#1(outstream)".equals(prettyPrintTwo)) {
            throw new IllegalStateException("context operatorPrettyPrint was " + prettyPrintTwo
                    + ", expected MyExceptionOp#1(outstream)");
        }
        if (!"Operator-thrown-exception".equals(contextTwo.getThrowable().getMessage())) {
            throw new IllegalStateException("context throwable message was " + contextTwo.getThrowable().getMessage()
                    + ", expected 'Operator-thrown-exception'");
        }

        // Records 7-12: the same six-record shape with the operator-throw
        // class token, the MyExceptionOp coordinates and the native bare-port
        // pretty print, plus the COMPLETE instance state.
        writer.addCount("handler-contexts", contextsTwo.size());
        writer.add("lifecycle", "flow", "handler.error-class", Json.value("operator-throw"));
        writer.add("lifecycle", "flow", "handler.operator-name", Json.value(contextTwo.getOperatorName()));
        writer.add("lifecycle", "flow", "handler.operator-number",
                Json.value(((Number) contextTwo.getOperatorNumber()).intValue()));
        writer.add("lifecycle", "flow", "handler.operator-pretty-print",
                Json.value((String) prettyPrintTwo));
        writer.addState(dfTwo.getState());

        // end of the run() body; the case-level undeployAll follows in
        // runCase.
    }

    private static void assertState(EPDataFlowInstance instance, EPDataFlowState expected) {
        EPDataFlowState actual = instance.getState();
        if (actual != expected) {
            throw new IllegalStateException("instance state was " + actual + ", expected " + expected);
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
     * Emits {case, operation:"count", statement:"flow", sequence, time, name,
     * count} records for handler-context sizes and {case, operation,
     * statement, sequence, time, name, value} records for the lifecycle
     * handler-context observables and the instance-state reads; the sequence
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
