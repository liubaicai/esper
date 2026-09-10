import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProviderByOpName;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowInputOutputVariations;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowInputOutputVariations
 * differential chain (work unit 4.372, dataflow-ports-feedback): typed
 * multi-port and feedback dataflow graphs observed through latched
 * DefaultSupportCaptureOp reads.
 *
 * fan-in-out (EPLDataflowFanInOut) deploys the byte-exact multi-line
 * MultiInMultiOutGraph graph (the 'MultiInMultiOutGraph ' line keeps its
 * trailing space, the empty second source literal contributes nothing, and
 * there is no space between 'OutTwo<SchemaOne>' and '{}'), instantiates with
 * four latched DefaultSupportCaptureOp<Object>(2) captures supplied through
 * DefaultSupportGraphOpProviderByOpName keyed by the name parameters
 * SupportOpCountFutureOneA/OneB/TwoA/TwoB, starts, and performs the four
 * get(3, TimeUnit.SECONDS) reads in suite order. MyCustomOp receives S0
 * events (p1 'A1'/'A2') on the (InOne, InTwo) port pair and emits
 * 'S0-' + value through submitPort(1), and S1 events (10/20) on the
 * (InThree, InFour) port pair and emits 'S1-' + value through submitPort(0);
 * the numeric submitPort calls map to the declared port order
 * OutOne=0/OutTwo=1 with no type check against the declared stream types, so
 * the S1-int strings land in the SchemaTwo-typed OutOne and the S0-strings in
 * the SchemaOne-typed OutTwo as raw Object[] passthrough payloads projected
 * positionally as p0 (no envelope, no coercion). Every capture read's rows
 * are sorted at emission by the canonical-fields JSON comparator (each row's
 * fields map serialized to JSON, rows stably ordered by that string),
 * freezing the suite's assertEqualsAnyOrder deterministically; the Go runner
 * emits the identical canonical order.
 *
 * factorial (EPLDataflowFactorial) deploys the byte-exact multi-line
 * FactorialGraph graph (same trailing-space and '{}' concatenation rules),
 * instantiates with one latched DefaultSupportCaptureOp<Object>(1) through
 * the type-based DefaultSupportGraphOpProvider, starts and reads
 * get(3, TimeUnit.SECONDS); MyFactorialOp feeds the (current, temp) pair back
 * through submitPort(0) to TempResult (declared port 0) until current reaches
 * 1 and submits the final {temp} through submitPort(1) to FinalResult
 * (declared port 1), producing 120L == 5*4*3*2. The instance reaches COMPLETE
 * on its own — the suite never cancels it — and the protocol stays
 * capture-only (no state records).
 *
 * large-num-ops (EPLDataflowLargeNumOpsDataFlow) deploys the byte-exact
 * 17-stage Select chain MyGraph graph (the Select stages out_1..out_17
 * chained through the select: subquery parameter, transcribed exactly as the
 * Java source concatenates them; the env.isHA() guard is a runner-variant
 * gate and the direct oracle runtime is not HA), instantiates with one
 * latched DefaultSupportCaptureOp<Object>(1) through
 * DefaultSupportGraphOpProviderByOpName keyed by the operator simple name
 * 'DefaultSupportCaptureOp' (no name parameter), starts and reads
 * get(3, TimeUnit.SECONDS), recording the single 'A1' row. The Go runner
 * adapts Java's select: subquery parameter with an identity custom operator
 * forwarding p1 unchanged — a contract-pinned equivalence.
 *
 * No case flushes: BeaconSource markers die at the source channel and the
 * latch reads observe exactly the current-batch rows the capture ops
 * collected.
 *
 * Record protocol: every capture read is one record
 * {case, operation:"capture", statement, sequence, time, new:[rows]} with
 * rows shaped {"kind":"row","fields":{"p0":...}}. fan-in-out uses the
 * deployment-prefixed distinct name parameters
 * 'flow:SupportOpCountFutureOneA'/'flow:SupportOpCountFutureOneB'/
 * 'flow:SupportOpCountFutureTwoA'/'flow:SupportOpCountFutureTwoB' as
 * statement labels because the four captures are disambiguated by name;
 * the unnamed captures of factorial and large-num-ops use
 * 'flow:DefaultSupportCaptureOp'. Sequence is case-local and restarts at 1;
 * the time is the fixed epoch 1970-01-01T00:00:00Z. Session configuration
 * mirrors TestSuiteEPLDataflow.configure restricted to these executions: the
 * com.espertech.esper.common.internal.epl.dataflow.util.* package import
 * that resolves DefaultSupportCaptureOp in EPL text (BeaconSource and Select
 * resolve through the built-in ConfigurationCommon.DATAFLOWOPERATOR_IMPORT)
 * plus addImport of the regression-lib nested classes
 * EPLDataflowInputOutputVariations.MyCustomOp and
 * EPLDataflowInputOutputVariations.MyFactorialOp; internal timer off, epoch
 * initialization, one fresh runtime per case destroyed in finally, the
 * deployment id pinned 'flow', undeployAll at case end.
 */
public final class DataflowPortsFeedbackScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-ports-feedback";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/"
                    + "EPLDataflowInputOutputVariations.java";
    private static final String CASE_FAN_IN_OUT = "fan-in-out";
    private static final String CASE_FACTORIAL = "factorial";
    private static final String CASE_LARGE_NUM_OPS = "large-num-ops";
    private static final String[] CASES = {CASE_FAN_IN_OUT, CASE_FACTORIAL, CASE_LARGE_NUM_OPS};
    private static final int[] ORDINALS = {1, 2, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-6309a6b7e0f0ba981a97",
            "java-runtime-98c5bdc6afa84705b9db",
            "java-runtime-bddc8c72bd9d2a2c8869"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowFanInOut", "EPLDataflowFactorial", "EPLDataflowLargeNumOpsDataFlow"};
    private static final int[] RECORD_COUNTS = {4, 1, 1};
    private static final int TOTAL_RECORDS = 6;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME_FAN_IN_OUT = "MultiInMultiOutGraph";
    private static final String FLOW_NAME_FACTORIAL = "FactorialGraph";
    private static final String FLOW_NAME_LARGE_NUM_OPS = "MyGraph";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String CAPTURE_ONE_A = "flow:SupportOpCountFutureOneA";
    private static final String CAPTURE_ONE_B = "flow:SupportOpCountFutureOneB";
    private static final String CAPTURE_TWO_A = "flow:SupportOpCountFutureTwoA";
    private static final String CAPTURE_TWO_B = "flow:SupportOpCountFutureTwoB";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs from EPLDataflowInputOutputVariations (fan-in-out
    // lines 105-120, factorial lines 159-169, large-num-ops lines 51-74).
    // The 'MultiInMultiOutGraph '/'FactorialGraph '/'MyGraph ' lines keep
    // their trailing space before the newline; each graph's second Java
    // source literal is the empty string and contributes nothing; there is
    // NO space between the last declared output stream and '{}'.
    private static final String FLOW_FAN_IN_OUT_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_FAN_IN_OUT + " \n" +
            "" +
            "create objectarray schema SchemaOne (p1 string),\n" +
            "create objectarray schema SchemaTwo (p2 int),\n" +
            "\n" +
            "BeaconSource -> InOne<SchemaOne> {p1:'A1', iterations:1}\n" +
            "BeaconSource -> InTwo<SchemaOne> {p1:'A2', iterations:1}\n" +
            "\n" +
            "BeaconSource -> InThree<SchemaTwo> {p2:10, iterations:1}\n" +
            "BeaconSource -> InFour<SchemaTwo> {p2:20, iterations:1}\n" +
            "MyCustomOp((InOne, InTwo) as S0, (InThree, InFour) as S1) -> OutOne<SchemaTwo>, "
                    + "OutTwo<SchemaOne>{}\n" +
            "\n" +
            "DefaultSupportCaptureOp(OutOne) { name : 'SupportOpCountFutureOneA' }\n" +
            "DefaultSupportCaptureOp(OutOne) { name : 'SupportOpCountFutureOneB' }\n" +
            "DefaultSupportCaptureOp(OutTwo) { name : 'SupportOpCountFutureTwoA' }\n" +
            "DefaultSupportCaptureOp(OutTwo) { name : 'SupportOpCountFutureTwoB' }\n";

    private static final String FLOW_FACTORIAL_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_FACTORIAL + " \n" +
            "" +
            "create objectarray schema InputSchema (number int),\n" +
            "create objectarray schema TempSchema (current int, temp long),\n" +
            "create objectarray schema FinalSchema (result long),\n" +
            "\n" +
            "BeaconSource -> InputData<InputSchema> {number:5, iterations:1}\n" +
            "\n" +
            "MyFactorialOp(InputData as Input, TempResult as Temp) -> TempResult<TempSchema>, "
                    + "FinalResult<FinalSchema>{}\n" +
            "\n" +
            "DefaultSupportCaptureOp(FinalResult) {}\n";

    private static final String FLOW_LARGE_NUM_OPS_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_LARGE_NUM_OPS + " \n" +
            "" +
            "create objectarray schema SchemaOne (p1 string),\n" +
            "\n" +
            "BeaconSource -> InStream<SchemaOne> {p1:'A1', iterations:1}\n" +
            "Select(InStream) -> out_1 { select: (select p1 from InStream) }\n" +
            "Select(out_1) -> out_2 { select: (select p1 from out_1) }\n" +
            "Select(out_2) -> out_3 { select: (select p1 from out_2) }\n" +
            "Select(out_3) -> out_4 { select: (select p1 from out_3) }\n" +
            "Select(out_4) -> out_5 { select: (select p1 from out_4) }\n" +
            "Select(out_5) -> out_6 { select: (select p1 from out_5) }\n" +
            "Select(out_6) -> out_7 { select: (select p1 from out_6) }\n" +
            "Select(out_7) -> out_8 { select: (select p1 from out_7) }\n" +
            "Select(out_8) -> out_9 { select: (select p1 from out_8) }\n" +
            "Select(out_9) -> out_10 { select: (select p1 from out_9) }\n" +
            "Select(out_10) -> out_11 { select: (select p1 from out_10) }\n" +
            "Select(out_11) -> out_12 { select: (select p1 from out_11) }\n" +
            "Select(out_12) -> out_13 { select: (select p1 from out_12) }\n" +
            "Select(out_13) -> out_14 { select: (select p1 from out_13) }\n" +
            "Select(out_14) -> out_15 { select: (select p1 from out_14) }\n" +
            "Select(out_15) -> out_16 { select: (select p1 from out_15) }\n" +
            "Select(out_16) -> out_17 { select: (select p1 from out_16) }\n" +
            "\n" +
            "DefaultSupportCaptureOp(out_17) {}\n";

    private DataflowPortsFeedbackScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowPortsFeedbackScenarioOracle <scenario.json>");
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

        // Session configuration per TestSuiteEPLDataflow.configure restricted
        // to these executions: the dataflow-util package import that resolves
        // DefaultSupportCaptureOp in EPL text (suite configure line 155) and
        // the regression-lib nested operator class imports (suite configure
        // lines 165-166); BeaconSource and Select resolve through the
        // built-in ConfigurationCommon.DATAFLOWOPERATOR_IMPORT. Internal
        // timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(EPLDataflowInputOutputVariations.MyFactorialOp.class);
        configuration.getCommon().addImport(EPLDataflowInputOutputVariations.MyCustomOp.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-ports-feedback-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_FAN_IN_OUT.equals(caseName)) {
                runFanInOut(configuration, runtime, writer);
            } else if (CASE_FACTORIAL.equals(caseName)) {
                runFactorial(configuration, runtime, writer);
            } else if (CASE_LARGE_NUM_OPS.equals(caseName)) {
                runLargeNumOps(configuration, runtime, writer);
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
     * EPLDataflowInputOutputVariations.EPLDataflowFanInOut: the four
     * name-keyed latched captures over the multi-in/multi-out graph, read in
     * suite order OneA, OneB, TwoA, TwoB.
     */
    private static void runFanInOut(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 105-121: byte-exact multi-line graph deployed under the
        // pinned deployment id.
        deploy(runtime, compile(configuration, runtime, FLOW_FAN_IN_OUT_EPL), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 123-135: four latched captures keyed by the name
        // parameters through DefaultSupportGraphOpProviderByOpName.
        DefaultSupportCaptureOp<Object> futureOneA = new DefaultSupportCaptureOp<Object>(2);
        DefaultSupportCaptureOp<Object> futureOneB = new DefaultSupportCaptureOp<Object>(2);
        DefaultSupportCaptureOp<Object> futureTwoA = new DefaultSupportCaptureOp<Object>(2);
        DefaultSupportCaptureOp<Object> futureTwoB = new DefaultSupportCaptureOp<Object>(2);
        Map<String, Object> operators = new HashMap<String, Object>();
        operators.put("SupportOpCountFutureOneA", futureOneA);
        operators.put("SupportOpCountFutureOneB", futureOneB);
        operators.put("SupportOpCountFutureTwoA", futureTwoA);
        operators.put("SupportOpCountFutureTwoB", futureTwoB);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));

        // suite line 137: instantiate + start.
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_FAN_IN_OUT, options).start();

        // suite lines 139-146: the four latch reads in suite order; the S1
        // strings land on OutOne (submitPort(0)) and the S0 strings on
        // OutTwo (submitPort(1)), numeric submitPort mapped to the declared
        // port order without type coercion. Rows are canonical-sorted at
        // emission.
        JsonObject[] oneExpected = p0Fields("S1-10", "S1-20");
        JsonObject[] twoExpected = p0Fields("S0-A1", "S0-A2");
        writer.addCapture(CAPTURE_ONE_A, captureRows(futureOneA.get(3, TimeUnit.SECONDS), oneExpected));
        writer.addCapture(CAPTURE_ONE_B, captureRows(futureOneB.get(3, TimeUnit.SECONDS), oneExpected));
        writer.addCapture(CAPTURE_TWO_A, captureRows(futureTwoA.get(3, TimeUnit.SECONDS), twoExpected));
        writer.addCapture(CAPTURE_TWO_B, captureRows(futureTwoB.get(3, TimeUnit.SECONDS), twoExpected));

        // suite line 148: undeployAll.
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * EPLDataflowInputOutputVariations.EPLDataflowFactorial: the single
     * type-keyed latched capture over the feedback graph; the instance
     * reaches COMPLETE on its own (never cancelled).
     */
    private static void runFactorial(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 159-170: byte-exact multi-line graph deployed under the
        // pinned deployment id.
        deploy(runtime, compile(configuration, runtime, FLOW_FACTORIAL_EPL), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 172-174: single latched capture through the type-based
        // provider.
        DefaultSupportCaptureOp<Object> future = new DefaultSupportCaptureOp<Object>(1);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(future));

        // suite line 176: instantiate + start; no cancel — the instance
        // completes on its own.
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_FACTORIAL, options).start();

        // suite lines 178-185: get(3, TimeUnit.SECONDS); result.length == 1
        // and result[0][0] == 120L == 5*4*3*2; TempResult is declared port 0
        // (the feedback pair) and FinalResult declared port 1 (the final
        // {temp}).
        Object[] result = future.get(3, TimeUnit.SECONDS);
        writer.addCapture(CAPTURE_STATEMENT, captureRows(result, new JsonObject().add("p0", 120L)));

        // suite line 187: undeployAll.
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * EPLDataflowInputOutputVariations.EPLDataflowLargeNumOpsDataFlow: the
     * single op-name-keyed latched capture over the 17-stage Select chain.
     */
    private static void runLargeNumOps(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite line 48: the env.isHA() guard is a runner-variant gate; the
        // direct oracle runtime is not HA, so the execution proceeds.
        // suite lines 51-75: byte-exact 17-stage Select chain deployed under
        // the pinned deployment id.
        deploy(runtime, compile(configuration, runtime, FLOW_LARGE_NUM_OPS_EPL), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 77-82: one latched capture keyed by the operator simple
        // name 'DefaultSupportCaptureOp' (no name parameter).
        DefaultSupportCaptureOp<Object> futureOneA = new DefaultSupportCaptureOp<Object>(1);
        Map<String, Object> operators = new HashMap<String, Object>();
        operators.put("DefaultSupportCaptureOp", futureOneA);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));

        // suite line 84: instantiate + start.
        runtime.getDataFlowService().instantiate(flowDeploymentId, FLOW_NAME_LARGE_NUM_OPS, options).start();

        // suite lines 86-92: get(3, TimeUnit.SECONDS); the single 'A1' row
        // survives the 17 identity-select stages.
        Object[] result = futureOneA.get(3, TimeUnit.SECONDS);
        writer.addCapture(CAPTURE_STATEMENT, captureRows(result, new JsonObject().add("p0", "A1")));

        // suite line 94: undeployAll.
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Asserts the captured rows are raw Object[] passthrough payloads
     * projecting positionally to the single p0 field and match the expected
     * fields exactly once each (the deterministic freeze of the suite's
     * assertEqualsAnyOrder), then renders them as
     * {"kind":"row","fields":{...}} rows in canonical emission order.
     */
    private static JsonArray captureRows(Object[] rows, JsonObject... expectedFields) {
        if (rows.length != expectedFields.length) {
            throw new IllegalStateException("capture held " + rows.length + " rows, expected "
                    + expectedFields.length);
        }
        boolean[] matched = new boolean[expectedFields.length];
        JsonArray captured = new JsonArray();
        for (Object row : rows) {
            if (!(row instanceof Object[])) {
                throw new IllegalStateException("row was "
                        + (row == null ? "null" : row.getClass().getName()) + ", expected a raw Object[]");
            }
            Object[] array = (Object[]) row;
            if (array.length != 1) {
                throw new IllegalStateException("row held " + array.length + " columns, expected 1");
            }
            JsonObject fields;
            if (array[0] instanceof String) {
                fields = new JsonObject().add("p0", (String) array[0]);
            } else if (array[0] instanceof Long) {
                fields = new JsonObject().add("p0", (Long) array[0]);
            } else {
                throw new IllegalStateException("row p0 was "
                        + (array[0] == null ? "null" : array[0].getClass().getName())
                        + ", expected String or Long");
            }
            boolean found = false;
            for (int i = 0; i < expectedFields.length; i++) {
                if (!matched[i] && fields.equals(expectedFields[i])) {
                    matched[i] = true;
                    found = true;
                    break;
                }
            }
            if (!found) {
                throw new IllegalStateException("row " + fields + " did not match any expected fields");
            }
            captured.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        for (int i = 0; i < matched.length; i++) {
            if (!matched[i]) {
                throw new IllegalStateException("expected row " + expectedFields[i] + " was not captured");
            }
        }
        return canonicalSort(captured);
    }

    /**
     * Canonical-fields JSON comparator: each row's fields map is serialized
     * to JSON and rows are stably ordered by that string, freezing the
     * suite's assertEqualsAnyOrder deterministically; the Go runner emits the
     * identical canonical order.
     */
    private static JsonArray canonicalSort(JsonArray rows) {
        JsonObject[] array = new JsonObject[rows.size()];
        for (int i = 0; i < array.length; i++) {
            array[i] = rows.get(i).asObject();
        }
        for (int i = 1; i < array.length; i++) {
            JsonObject current = array[i];
            String currentJson = current.get("fields").toString();
            int j = i - 1;
            while (j >= 0 && currentJson.compareTo(array[j].get("fields").toString()) < 0) {
                array[j + 1] = array[j];
                j--;
            }
            array[j + 1] = current;
        }
        JsonArray sorted = new JsonArray();
        for (JsonObject row : array) {
            sorted.add(row);
        }
        return sorted;
    }

    private static JsonObject[] p0Fields(String... values) {
        JsonObject[] fields = new JsonObject[values.length];
        for (int i = 0; i < values.length; i++) {
            fields[i] = new JsonObject().add("p0", values[i]);
        }
        return fields;
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
     * Emits {case, operation:"capture", statement, sequence, time, new:[...]}
     * records for every capture read; the sequence is case-local.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        private void addCapture(String statement, JsonArray rows) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "capture")
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
