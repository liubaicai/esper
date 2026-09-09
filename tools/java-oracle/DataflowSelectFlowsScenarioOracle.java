import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowEmitterOperator;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstanceCaptive;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.util.EPDataFlowSignalFinalMarker;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProviderByOpName;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
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
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpSelect differential chain
 * (work unit 4.359): dataflow Select iterate/join flows observed through
 * captive-emitter submissions into DefaultSupportCaptureOp.
 *
 * select-iterate-final-marker (EPLDataflowIterateFinalMarker) replays the
 * iterate:true Select over a grouped/ordered aggregate: five SupportBean
 * submissions leave the capture empty (pre-marker emptiness is the observable
 * iterate semantics), the final marker releases one grouped snapshot ordered
 * by theString asc (recorded with the distinct operation "iterate").
 *
 * select-join-order (EPLDataflowFromClauseJoinOrder) replays the three
 * from-clause orderings of the same three-input inner join as three sub-runs
 * in one case; each sub-run observes the inner-join wait-for-all-inputs
 * emptiness after S0/S1, the joined row when S2 arrives, and post-cancel
 * emptiness after instance.cancel(). The capture sequence is continuous
 * across the sub-runs.
 *
 * select-outer-join-multirow (EPLDataflowOuterJoinMultirow) replays the
 * keepall full outer join: one unmatched S0 row projects immediately with a
 * null p10. The suite asserts "p00,p11" positionally (a quirk of the map
 * lookup in the assertion helper); the projected columns are p00,p10 and the
 * trace pins those names.
 *
 * Record protocol: every capture read is one record
 * {case, operation, statement:"flow:DefaultSupportCaptureOp", sequence,
 * time, new:[rows]} with rows shaped {"kind":"row","fields":{...}}; empty
 * reads are recorded explicitly with new:[]. Session configuration per case
 * mirrors TestSuiteEPLDataflow.configure restricted to these executions:
 * SupportBean + SupportBean_S0/S1/S2 event types, the dataflow-util package
 * import, the SupportBean import, internal timer off, and epoch
 * initialization; each case gets a fresh runtime destroyed in finally.
 */
public final class DataflowSelectFlowsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-select-flows";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpSelect.java";
    private static final String CASE_ITERATE = "select-iterate-final-marker";
    private static final String CASE_JOIN_ORDER = "select-join-order";
    private static final String CASE_OUTER_JOIN = "select-outer-join-multirow";
    private static final String[] CASES = {CASE_ITERATE, CASE_JOIN_ORDER, CASE_OUTER_JOIN};
    private static final int[] ORDINALS = {3, 6, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-59ce9d5f4b9ca475c272",
            "java-runtime-323dd1ec14f5ed5c0586",
            "java-runtime-21dcd981de8124bf387c"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowIterateFinalMarker",
            "EPLDataflowFromClauseJoinOrder",
            "EPLDataflowOuterJoinMultirow"};
    private static final int[] RECORD_COUNTS = {2, 9, 1};
    private static final int TOTAL_RECORDS = 12;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME = "MySelect";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final String[] JOIN_FROM_CLAUSES = {
            "from S2#lastevent as s2, S1#lastevent as s1, S0#lastevent as s0",
            "from S0#lastevent as s0, S1#lastevent as s1, S2#lastevent as s2",
            "from S1#lastevent as s1, S2#lastevent as s2, S0#lastevent as s0"};
    private static final String JOIN_GRAPH_TEMPLATE =
            "@name('flow') create dataflow MySelect\n" +
            "Emitter -> instream_s0<SupportBean_S0>{name: 'emitterS0'}\n" +
            "Emitter -> instream_s1<SupportBean_S1>{name: 'emitterS1'}\n" +
            "Emitter -> instream_s2<SupportBean_S2>{name: 'emitterS2'}\n" +
            "Select(instream_s0 as S0, instream_s1 as S1, instream_s2 as S2) -> outstream {\n" +
            "  select: (select s0.id as s0id, s1.id as s1id, s2.id as s2id <fromClause>)\n" +
            "}\n" +
            "DefaultSupportCaptureOp(outstream) {}\n";
    private static final String ITERATE_GRAPH =
            "@name('flow') create dataflow MySelect\n" +
            "Emitter -> instream_s0<SupportBean>{name: 'emitterS0'}\n" +
            "@Audit Select(instream_s0 as ALIAS) -> outstream {\n" +
            "  select: (select theString, sum(intPrimitive) as sumInt from ALIAS group by theString order by theString asc),\n" +
            "  iterate: true" +
            "}\n" +
            "DefaultSupportCaptureOp(outstream) {}\n";
    private static final String OUTER_JOIN_GRAPH =
            "@name('flow') create dataflow MySelect\n" +
            "Emitter -> instream_s0<SupportBean_S0>{name: 'emitterS0'}\n" +
            "Emitter -> instream_s1<SupportBean_S1>{name: 'emitterS1'}\n" +
            "Select(instream_s0 as S0, instream_s1 as S1) -> outstream {\n" +
            "  select: (select p00, p10 from S0#keepall full outer join S1#keepall)\n" +
            "}\n" +
            "DefaultSupportCaptureOp(outstream) {}\n";

    private DataflowSelectFlowsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowSelectFlowsScenarioOracle <scenario.json>");
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

        // Session configuration per TestSuiteEPLDataflow.configure restricted to
        // these executions: the SupportBean/S0/S1/S2 types, the dataflow-util
        // package import that resolves DefaultSupportCaptureOp in the graph
        // text, and the SupportBean import.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class.getSimpleName(), SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class.getSimpleName(), SupportBean_S1.class);
        configuration.getCommon().addEventType(SupportBean_S2.class.getSimpleName(), SupportBean_S2.class);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-select-flows-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_ITERATE.equals(caseName)) {
                runIterateFinalMarker(configuration, runtime, writer);
            } else if (CASE_JOIN_ORDER.equals(caseName)) {
                runFromClauseJoinOrder(configuration, runtime, writer);
            } else if (CASE_OUTER_JOIN.equals(caseName)) {
                runOuterJoinMultirow(configuration, runtime, writer);
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
     * EPLDataflowIterateFinalMarker: five captive submissions accumulate while
     * the capture stays empty (iterate semantics), then the final marker
     * releases the grouped snapshot ordered by theString asc.
     */
    private static void runIterateFinalMarker(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled compiled = compile(configuration, runtime, ITERATE_GRAPH);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        Map<String, Object> operators = CollectionUtil.populateNameValueMap("DefaultSupportCaptureOp", capture);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        EPDataFlowInstanceCaptive captive = instance.startCaptive();

        EPDataFlowEmitterOperator emitter = captive.getEmitters().get("emitterS0");
        emitter.submit(new SupportBean("E3", 4));
        emitter.submit(new SupportBean("E2", 3));
        emitter.submit(new SupportBean("E1", 1));
        emitter.submit(new SupportBean("E2", 2));
        emitter.submit(new SupportBean("E1", 5));

        // Pre-marker emptiness is the observable iterate semantics.
        Object[] preMarker = capture.getCurrent();
        if (preMarker.length != 0) {
            throw new IllegalStateException("pre-marker capture held " + preMarker.length + " rows, expected 0");
        }
        writer.addCapture("capture", new JsonArray());

        emitter.submitSignal(new EPDataFlowSignalFinalMarker() {
        });

        Object[] rows = capture.getCurrent();
        if (rows.length != 3) {
            throw new IllegalStateException("post-marker capture held " + rows.length + " rows, expected 3");
        }
        String[] expectedKeys = {"E1", "E2", "E3"};
        int[] expectedSums = {6, 5, 4};
        JsonArray snapshot = new JsonArray();
        for (int i = 0; i < rows.length; i++) {
            if (!(rows[i] instanceof Object[])) {
                throw new IllegalStateException("iterate row " + i + " was not an Object[]: " + rows[i]);
            }
            Object[] row = (Object[]) rows[i];
            if (row.length != 2) {
                throw new IllegalStateException("iterate row " + i + " held " + row.length + " columns, expected 2");
            }
            if (!expectedKeys[i].equals(row[0]) || !Integer.valueOf(expectedSums[i]).equals(row[1])) {
                throw new IllegalStateException("iterate row " + i + " was [theString=" + row[0]
                        + ", sumInt=" + row[1] + "], expected [theString=" + expectedKeys[i]
                        + ", sumInt=" + expectedSums[i] + "]");
            }
            snapshot.add(new JsonObject().add("kind", "row").add("fields",
                    new JsonObject().add("theString", expectedKeys[i]).add("sumInt", expectedSums[i])));
        }
        writer.addCapture("iterate", snapshot);

        instance.cancel();
    }

    /**
     * EPLDataflowFromClauseJoinOrder: three sub-runs, one per from-clause
     * ordering, each with its own deploy/instantiate/captive/cancel/undeploy
     * cycle and a continuous capture sequence. Every sub-run reads the capture
     * three times: empty while only S0/S1 have arrived (inner-join
     * wait-for-all-inputs), the joined row once S2 arrives, and empty again
     * after instance.cancel() plus a post-cancel submission.
     */
    private static void runFromClauseJoinOrder(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        for (int subRun = 0; subRun < JOIN_FROM_CLAUSES.length; subRun++) {
            String graph = JOIN_GRAPH_TEMPLATE.replace("<fromClause>", JOIN_FROM_CLAUSES[subRun]);
            EPCompiled compiled = compile(configuration, runtime, graph);
            deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
            Map<String, Object> operators = CollectionUtil.populateNameValueMap("DefaultSupportCaptureOp", capture);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                    .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));
            EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);

            EPDataFlowInstanceCaptive captive = instance.startCaptive();
            captive.getEmitters().get("emitterS0").submit(new SupportBean_S0(1));
            captive.getEmitters().get("emitterS1").submit(new SupportBean_S1(10));

            Object[] waiting = capture.getCurrent();
            if (waiting.length != 0) {
                throw new IllegalStateException("sub-run " + subRun + " capture held " + waiting.length
                        + " rows before S2, expected 0");
            }
            writer.addCapture("capture", new JsonArray());

            captive.getEmitters().get("emitterS2").submit(new SupportBean_S2(100));
            Object[] joined = capture.getCurrent();
            if (joined.length != 1) {
                throw new IllegalStateException("sub-run " + subRun + " capture held " + joined.length
                        + " rows after S2, expected 1");
            }
            if (!(joined[0] instanceof Object[])) {
                throw new IllegalStateException("sub-run " + subRun + " joined row was not an Object[]: " + joined[0]);
            }
            Object[] row = (Object[]) joined[0];
            if (row.length != 3) {
                throw new IllegalStateException("sub-run " + subRun + " joined row held " + row.length
                        + " columns, expected 3");
            }
            if (!Integer.valueOf(1).equals(row[0]) || !Integer.valueOf(10).equals(row[1])
                    || !Integer.valueOf(100).equals(row[2])) {
                throw new IllegalStateException("sub-run " + subRun + " joined row was [s0id=" + row[0]
                        + ", s1id=" + row[1] + ", s2id=" + row[2] + "], expected [s0id=1, s1id=10, s2id=100]");
            }
            capture.getCurrentAndReset();
            writer.addCapture("capture", new JsonArray().add(new JsonObject().add("kind", "row").add("fields",
                    new JsonObject().add("s0id", 1).add("s1id", 10).add("s2id", 100))));

            instance.cancel();

            captive.getEmitters().get("emitterS2").submit(new SupportBean_S2(101));
            Object[] postCancel = capture.getCurrent();
            if (postCancel.length != 0) {
                throw new IllegalStateException("sub-run " + subRun + " capture held " + postCancel.length
                        + " rows after cancel, expected 0");
            }
            writer.addCapture("capture", new JsonArray());

            runtime.getDeploymentService().undeployAll();
        }
    }

    /**
     * EPLDataflowOuterJoinMultirow: the unmatched S0 row projects immediately
     * over the keepall full outer join with a null p10. The suite asserts
     * "p00,p11" positionally; the projected columns are p00,p10 and the trace
     * pins those names.
     */
    private static void runOuterJoinMultirow(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled compiled = compile(configuration, runtime, OUTER_JOIN_GRAPH);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        Map<String, Object> operators = CollectionUtil.populateNameValueMap("DefaultSupportCaptureOp", capture);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(operators));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);

        EPDataFlowInstanceCaptive captive = instance.startCaptive();
        captive.getEmitters().get("emitterS0").submit(new SupportBean_S0(1, "S0_1"));

        Object[] rows = capture.getCurrentAndReset();
        if (rows.length != 1) {
            throw new IllegalStateException("capture held " + rows.length + " rows, expected 1");
        }
        if (!(rows[0] instanceof Object[])) {
            throw new IllegalStateException("outer-join row was not an Object[]: " + rows[0]);
        }
        Object[] row = (Object[]) rows[0];
        if (row.length != 2) {
            throw new IllegalStateException("outer-join row held " + row.length + " columns, expected 2");
        }
        Object p00 = row[0];
        Object p10 = row[1];
        if (!"S0_1".equals(p00) || p10 != null) {
            throw new IllegalStateException("outer-join row was [p00=" + p00 + ", p10=" + p10
                    + "], expected [p00=S0_1, p10=null]");
        }
        writer.addCapture("capture", new JsonArray().add(new JsonObject().add("kind", "row").add("fields",
                new JsonObject().add("p00", "S0_1").add("p10", new JsonObject().add("state", "null")))));

        instance.cancel();
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
     * for every capture read; empty reads carry an empty new array.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        private void addCapture(String operation, JsonArray rows) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
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
