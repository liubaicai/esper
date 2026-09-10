import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphEventUtil;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowOpEventBusSink;
import com.espertech.esper.regressionlib.support.dataflow.MyObjectArrayGraphSource;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpEventBusSink differential
 * chain (work unit 4.370): the EventBusSink operator observed through
 * statement listeners on plain s0/s1 select-stream statements.
 *
 * eventbus-sink-all-types (EPLDataflowOpEventBusSink EPLDataflowAllTypes)
 * replays runAssertionAllTypes as four representation sub-runs in the suite
 * order MyXMLEvent (over the regression/threeProperties.xsd classloader
 * schema), MyOAEvent, MyMapEvent and MyDefaultSupportGraphEvent. Each sub-run
 * deploys the byte-exact one-line MyGraph graph
 * DefaultSupportSourceOp -> instream&lt;T&gt;{}EventBusSink(instream) {} plus
 * the s0 select * from &lt;T&gt; statement with a listener, supplies the
 * DefaultSupportSourceOp events {1.1,1,one} and {2.2,2,two} through
 * DefaultSupportGraphOpProvider, and drives the flow with the synchronous
 * instance.run(): both events reach s0 before run() returns, so each sub-run
 * records the two listener rows {myDouble:1.1,myInt:1,myString:one} then
 * {myDouble:2.2,myInt:2,myString:two} with lossless doubles. The compile-time
 * invalid graph 'create dataflow DF1 EventBusSink -> s1 {}' (no output stream)
 * is asserted in-process by message prefix with no trace rows. The doc sample
 * path-deploys '@public create schema SampleSchema(tagId string, locX double,
 * locY double)' plus the byte-exact multi-line MyDataFlow graph (BeaconSource
 * with the '// produces sample stream to' and '//demonstrate below' comment
 * lines, the plain EventBusSink(instream) {} sink and the second
 * EventBusSink(instream) with the MyTransformToEventBus collector block
 * spelled 'class : ' with spaces); the flow statement is named s0, it is
 * instantiated with no options, never started, and contributes no rows.
 *
 * eventbus-sink-beacon (EPLDataflowOpEventBusSink EPLDataflowBeacon)
 * path-deploys the @public objectarray schema MyEventBeacon(p0 string,
 * p1 long), the s0 select * from MyEventBeacon statement with a listener and
 * the byte-exact single-line MyDataFlowOne graph with the BeaconSource
 * parameter block carrying two leading spaces inside the braces and the
 * trailing comma after 'p1 : 1'. instantiate + start runs the asynchronous
 * source thread, producing exactly three listener deliveries; the harness
 * polls until three deliveries arrive within the suite's 3000 ms bound, then
 * settles and asserts that no fourth delivery arrives (the iterations : 3
 * bound) and that every row is {p0:abc,p1:1}: the graph pins the p1 constant
 * 1, so the Java '0 &lt; val &lt; 10' assertion is trivially satisfied and the
 * exact value is pinned here.
 *
 * eventbus-sink-dynamic-type (EPLDataflowOpEventBusSink
 * EPLDataflowSendEventDynamicType) path-deploys the two @buseventtype @public
 * objectarray schemas MyEventOne(type string, p0 int, p1 string) and
 * MyEventTwo(type string, f0 string, f1 int) plus s0/s1 select * statements
 * with listeners, and deploys the byte-exact single-line MyDataFlow graph
 * MyObjectArrayGraphSource -> OutStream&lt;?&gt;{}EventBusSink(OutStream) with
 * the MyTransformToEventBus collector block spelled 'class:' with no space
 * before the colon (unlike the doc sample). The source submits
 * {type1,100,abc} then {type2,GE,-1}; the collector forwards the raw
 * Object[] to MyEventOne or MyEventTwo by eventObj[0] and the final marker
 * completes the flow. The harness polls until both listeners fired within
 * 3000 ms, settles, and records s0 {type:type1,p0:100,p1:abc} then s1
 * {type:type2,f0:GE,f1:-1}: the full typed projection including the type
 * property, stronger than and consistent with the Java p0/p1 and f0/f1
 * subset asserts.
 *
 * Record protocol: every listener callback is one record
 * {case, operation:"listener", statement, sequence, time, new:[rows]} with
 * rows shaped {"kind":"row","fields":{...}}; the projections are typed per
 * case (lossless decimal doubles for myDouble), each delivery carries exactly
 * one row, sequences restart at 1 per case, the fixed epoch time
 * 1970-01-01T00:00:00Z is used because the internal timer is off and the
 * runtime is epoch-initialized, and no old-stream delivery occurs in this
 * chain. Statement names are the plain s0/s1 names, never flow labels.
 * Session configuration per case mirrors TestSuiteEPLDataflow.configure
 * restricted to these executions: the DefaultSupportGraphEventUtil
 * representations (including the XML type backed by the
 * regression/threeProperties.xsd classloader resource), the dataflow-util
 * package import that resolves DefaultSupportSourceOp, and for the dynamic
 * type case the support.dataflow package import that resolves
 * MyObjectArrayGraphSource at compile time; XML-XSD is enabled because the
 * harness base configuration enables it and the MyXMLEvent representation
 * requires it; each case gets a fresh runtime destroyed in finally. The
 * collector class is referenced by FQN string inside the graph EPL and
 * reflectively loaded at instantiate, so the oracle only resolves it at
 * compile time to build the byte-exact string.
 */
public final class DataflowEventbusSinkScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-eventbus-sink";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEventBusSink.java";
    private static final String CASE_ALL_TYPES = "eventbus-sink-all-types";
    private static final String CASE_BEACON = "eventbus-sink-beacon";
    private static final String CASE_DYNAMIC_TYPE = "eventbus-sink-dynamic-type";
    private static final String[] CASES = {CASE_ALL_TYPES, CASE_BEACON, CASE_DYNAMIC_TYPE};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-e6b4bb618f70b384cf03",
            "java-runtime-1ecb769e10b4a818772c",
            "java-runtime-11a7b321e6901dbad740"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowAllTypes",
            "EPLDataflowBeacon",
            "EPLDataflowSendEventDynamicType"};
    private static final int[] RECORD_COUNTS = {8, 3, 2};
    private static final int TOTAL_RECORDS = 13;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String SCHEMA_DEPLOYMENT_ID = "schema";
    private static final String S0_DEPLOYMENT_ID = "s0";
    private static final String S1_DEPLOYMENT_ID = "s1";
    private static final String FLOW_NAME_GRAPH = "MyGraph";
    private static final String FLOW_NAME_BEACON = "MyDataFlowOne";
    private static final String FLOW_NAME_DYNAMIC = "MyDataFlow";
    private static final String FLOW_NAME_DOC = "MyDataFlow";
    private static final String COLLECTOR_CLASS =
            EPLDataflowOpEventBusSink.MyTransformToEventBus.class.getName();
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final long LISTENER_WAIT_MILLIS = 3000;
    private static final long SETTLE_MILLIS = 200;

    // Byte-exact graphs, schema modules and message from
    // EPLDataflowOpEventBusSink.
    private static final String SAMPLE_SCHEMA_EPL =
            "@public create schema SampleSchema(tagId string, locX double, locY double)";

    private static final String INVALID_OUTPUT_STREAM = "create dataflow DF1 EventBusSink -> s1 {}";

    private static final String MSG_OUTPUT_STREAM =
            "Failed to obtain operator 'EventBusSink': EventBusSink operator does not provide an output stream";

    private static final String BEACON_SCHEMA_EPL =
            "@public create objectarray schema MyEventBeacon(p0 string, p1 long)";

    private static final String BEACON_GRAPH_EPL =
            "@name('flow') create dataflow MyDataFlowOne "
                    + ""
                    + "BeaconSource -> BeaconStream<MyEventBeacon> {"
                    + "  iterations : 3,"
                    + "  p0 : 'abc',"
                    + "  p1 : 1,"
                    + "}"
                    + "EventBusSink(BeaconStream) {}";

    private static final String DYNAMIC_SCHEMA_EPL =
            "@buseventtype @public create objectarray schema MyEventOne(type string, p0 int, p1 string);\n"
                    + "@buseventtype @public create objectarray schema MyEventTwo(type string, f0 string, f1 int);\n";

    private static final String DYNAMIC_GRAPH_EPL =
            "@name('flow') create dataflow MyDataFlow "
                    + "MyObjectArrayGraphSource -> OutStream<?> {}"
                    + "EventBusSink(OutStream) {"
                    + "  collector : {"
                    + "    class: '" + COLLECTOR_CLASS + "'"
                    + "  }"
                    + "}";

    private DataflowEventbusSinkScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowEventbusSinkScenarioOracle <scenario.json>");
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
        // these executions: the DefaultSupportGraphEventUtil representations
        // (including the XML type backed by the regression/threeProperties.xsd
        // classloader resource), the dataflow-util package import that resolves
        // DefaultSupportSourceOp, and for the dynamic type case the
        // support.dataflow package import for MyObjectArrayGraphSource.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        // Harness base configuration (SupportConfigFactory.getConfiguration)
        // enables XML-XSD; the MyXMLEvent representation requires it to map the
        // regression/threeProperties.xsd schema.
        configuration.getCommon().getEventMeta().setEnableXMLXSD(true);
        DefaultSupportGraphEventUtil.addTypeConfiguration(configuration);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");
        if (CASE_DYNAMIC_TYPE.equals(caseName)) {
            configuration.getCommon().addImport(MyObjectArrayGraphSource.class.getPackage().getName() + ".*");
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-eventbus-sink-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, rendererFor(caseName));
            if (CASE_ALL_TYPES.equals(caseName)) {
                runAllTypes(configuration, runtime, writer);
            } else if (CASE_BEACON.equals(caseName)) {
                runBeacon(configuration, runtime, writer);
            } else if (CASE_DYNAMIC_TYPE.equals(caseName)) {
                runDynamicType(configuration, runtime, writer);
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

    private static RowRenderer rendererFor(String caseName) {
        if (CASE_ALL_TYPES.equals(caseName)) {
            return DataflowEventbusSinkScenarioOracle::renderAllTypes;
        }
        if (CASE_BEACON.equals(caseName)) {
            return DataflowEventbusSinkScenarioOracle::renderBeacon;
        }
        if (CASE_DYNAMIC_TYPE.equals(caseName)) {
            return DataflowEventbusSinkScenarioOracle::renderDynamic;
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    /**
     * EPLDataflowOpEventBusSink.EPLDataflowAllTypes: four representation
     * sub-runs in the suite order, each synchronous through instance.run(),
     * then the invalid no-output-stream compile and the doc-sample flow that
     * is instantiated and never started.
     */
    private static void runAllTypes(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        String[] typeNames = {
                "MyXMLEvent", "MyOAEvent", "MyMapEvent", DefaultSupportGraphEventUtil.EVENTTYPENAME};
        Object[][] eventSets = {
                DefaultSupportGraphEventUtil.getXMLEvents(),
                DefaultSupportGraphEventUtil.getOAEvents(),
                DefaultSupportGraphEventUtil.getMapEvents(),
                DefaultSupportGraphEventUtil.getPOJOEvents()};
        JsonObject[] expectedRows = {
                new JsonObject().add("myDouble", 1.1).add("myInt", 1).add("myString", "one"),
                new JsonObject().add("myDouble", 2.2).add("myInt", 2).add("myString", "two")};

        for (int subRun = 0; subRun < typeNames.length; subRun++) {
            String typeName = typeNames[subRun];
            String graph = "@name('flow') create dataflow " + FLOW_NAME_GRAPH + " " +
                    "DefaultSupportSourceOp -> instream<" + typeName + ">{}" +
                    "EventBusSink(instream) {}";
            deploy(runtime, compile(configuration, runtime, graph), FLOW_DEPLOYMENT_ID);
            EPDeployment s0 = deploy(runtime, compile(configuration, runtime,
                    "@name('s0') select * from " + typeName), S0_DEPLOYMENT_ID);
            statementOf(s0, "s0").addListener(writer);

            DefaultSupportSourceOp source = new DefaultSupportSourceOp(eventSets[subRun]);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                    .operatorProvider(new DefaultSupportGraphOpProvider(source));
            EPDataFlowInstance instance = runtime.getDataFlowService()
                    .instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_GRAPH, options);
            int fromSequence = (int) writer.count();
            instance.run();

            // run() is synchronous: both events reached s0 before it returned.
            writer.assertRows(fromSequence, new String[]{"s0", "s0"}, expectedRows);

            runtime.getDeploymentService().undeployAll();
        }

        // invalid: output stream
        tryInvalidCompile(configuration, runtime, INVALID_OUTPUT_STREAM, MSG_OUTPUT_STREAM);

        // test doc sample on a fresh path: the flow statement itself is named
        // s0; instantiated with no options and never started, so no rows.
        deploy(runtime, compile(configuration, runtime, SAMPLE_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        deploy(runtime, compile(configuration, runtime, docGraphEpl()), S0_DEPLOYMENT_ID);
        runtime.getDataFlowService().instantiate(S0_DEPLOYMENT_ID, FLOW_NAME_DOC);
    }

    /**
     * Byte-exact doc-sample graph from EPLDataflowOpEventBusSink
     * EPLDataflowAllTypes, collector FQN substituted.
     */
    private static String docGraphEpl() {
        return "@name('s0') create dataflow " + FLOW_NAME_DOC + "\n" +
                "BeaconSource -> instream<SampleSchema> {} // produces sample stream to\n" +
                "//demonstrate below\n" +
                "// Send SampleSchema events produced by beacon to the event bus.\n" +
                "EventBusSink(instream) {}\n" +
                "\n" +
                "// Send SampleSchema events produced by beacon to the event bus.\n" +
                "// With collector that performs transformation.\n" +
                "EventBusSink(instream) {\n" +
                "collector : {\n" +
                "class : '" + COLLECTOR_CLASS + "'\n" +
                "}\n" +
                "}";
    }

    /**
     * EPLDataflowOpEventBusSink.EPLDataflowBeacon: the asynchronous beacon
     * source produces exactly three deliveries of the pinned {abc,1} row.
     */
    private static void runBeacon(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        deploy(runtime, compile(configuration, runtime, BEACON_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        EPDeployment s0 = deploy(runtime, compile(configuration, runtime,
                "@name('s0') select * from MyEventBeacon"), S0_DEPLOYMENT_ID);
        statementOf(s0, "s0").addListener(writer);
        deploy(runtime, compile(configuration, runtime, BEACON_GRAPH_EPL), FLOW_DEPLOYMENT_ID);

        int fromSequence = (int) writer.count();
        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_BEACON).start();

        // Adaptation of env.assertListener waitForInvocation(3000, 3): poll
        // until the third delivery, then settle and require that the
        // iterations : 3 bound holds - no fourth delivery may arrive.
        long deadline = System.currentTimeMillis() + LISTENER_WAIT_MILLIS;
        while (writer.count() - fromSequence < 3 && System.currentTimeMillis() < deadline) {
            Thread.sleep(10);
        }
        if (writer.count() - fromSequence < 3) {
            throw new IllegalStateException("s0 received " + (writer.count() - fromSequence)
                    + " deliveries within " + LISTENER_WAIT_MILLIS + "ms, expected 3");
        }
        Thread.sleep(SETTLE_MILLIS);
        if (writer.count() - fromSequence != 3) {
            throw new IllegalStateException("s0 received " + (writer.count() - fromSequence)
                    + " deliveries after settle, expected exactly 3");
        }
        // The graph pins p1 to the constant 1, so the Java '0 < val < 10'
        // assertion is trivially satisfied and the exact value is pinned.
        JsonObject row = new JsonObject().add("p0", "abc").add("p1", 1L);
        writer.assertRows(fromSequence, new String[]{"s0", "s0", "s0"}, new JsonObject[]{row, row, row});
    }

    /**
     * EPLDataflowOpEventBusSink.EPLDataflowSendEventDynamicType: the source
     * submits two raw Object[] rows; the collector routes them by eventObj[0]
     * onto MyEventOne and MyEventTwo.
     */
    private static void runDynamicType(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        deploy(runtime, compile(configuration, runtime, DYNAMIC_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        EPDeployment s0 = deploy(runtime, compile(configuration, runtime,
                "@name('s0') select * from MyEventOne"), S0_DEPLOYMENT_ID);
        statementOf(s0, "s0").addListener(writer);
        EPDeployment s1 = deploy(runtime, compile(configuration, runtime,
                "@name('s1') select * from MyEventTwo"), S1_DEPLOYMENT_ID);
        statementOf(s1, "s1").addListener(writer);
        deploy(runtime, compile(configuration, runtime, DYNAMIC_GRAPH_EPL), FLOW_DEPLOYMENT_ID);

        MyObjectArrayGraphSource source = new MyObjectArrayGraphSource(Arrays.asList(
                new Object[]{"type1", 100, "abc"},
                new Object[]{"type2", "GE", -1}
        ).iterator());
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source));

        int fromSequence = (int) writer.count();
        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_DYNAMIC, options).start();

        // Adaptation of env.assertThat with waitForInvocation(3000, 1) on both
        // s0 and s1: poll until both deliveries, then settle. The source sends
        // type1 before type2 on one thread, so s0 fires before s1.
        long deadline = System.currentTimeMillis() + LISTENER_WAIT_MILLIS;
        while (writer.count() - fromSequence < 2 && System.currentTimeMillis() < deadline) {
            Thread.sleep(10);
        }
        if (writer.count() - fromSequence < 2) {
            throw new IllegalStateException("s0/s1 received " + (writer.count() - fromSequence)
                    + " deliveries within " + LISTENER_WAIT_MILLIS + "ms, expected 2");
        }
        Thread.sleep(SETTLE_MILLIS);
        if (writer.count() - fromSequence != 2) {
            throw new IllegalStateException("s0/s1 received " + (writer.count() - fromSequence)
                    + " deliveries after settle, expected exactly 2");
        }
        writer.assertRows(fromSequence, new String[]{"s0", "s1"}, new JsonObject[]{
                new JsonObject().add("type", "type1").add("p0", 100).add("p1", "abc"),
                new JsonObject().add("type", "type2").add("f0", "GE").add("f1", -1)});
    }

    /**
     * Renders one all-types listener row: POJO bean, Map, Object[] or XML
     * node, all projected to the typed myDouble/myInt/myString fields with a
     * lossless double.
     */
    private static JsonObject renderAllTypes(EventBean event) {
        return new JsonObject()
                .add("myDouble", ((Number) event.get("myDouble")).doubleValue())
                .add("myInt", ((Number) event.get("myInt")).intValue())
                .add("myString", String.valueOf(event.get("myString")));
    }

    private static JsonObject renderBeacon(EventBean event) {
        return new JsonObject()
                .add("p0", String.valueOf(event.get("p0")))
                .add("p1", ((Number) event.get("p1")).longValue());
    }

    /**
     * Renders the full dynamic-type projection including the type property:
     * the collector sends the raw Object[] unchanged, so every declared
     * property is pinned.
     */
    private static JsonObject renderDynamic(EventBean event) {
        String typeName = event.getEventType().getName();
        if ("MyEventOne".equals(typeName)) {
            return new JsonObject()
                    .add("type", String.valueOf(event.get("type")))
                    .add("p0", ((Number) event.get("p0")).intValue())
                    .add("p1", String.valueOf(event.get("p1")));
        }
        if ("MyEventTwo".equals(typeName)) {
            return new JsonObject()
                    .add("type", String.valueOf(event.get("type")))
                    .add("f0", String.valueOf(event.get("f0")))
                    .add("f1", ((Number) event.get("f1")).intValue());
        }
        throw new IllegalStateException("unexpected event type " + typeName);
    }

    private static void tryInvalidCompile(Configuration configuration, EPRuntime runtime, String graph, String message)
            throws Exception {
        try {
            compile(configuration, runtime, graph);
            throw new IllegalStateException("expected EPCompileException for graph:\n" + graph);
        } catch (EPCompileException ex) {
            assertPrefix(message, ex.getMessage());
        }
    }

    private static void assertPrefix(String expected, String message) {
        if (message == null || !message.startsWith(expected)) {
            throw new IllegalStateException("Expected prefix:\n" + expected + "\nbut received:\n" + message);
        }
    }

    private static EPStatement statementOf(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement '" + name + "' not found in deployment " + deployment.getDeploymentId());
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
     * Listener trace writer: one record per listener callback with the
     * {case, operation:"listener", statement, sequence, time, new:[rows]}
     * shape; case-local sequences restart at 1 and the time is the fixed
     * epoch because the internal timer is off and the runtime was
     * epoch-initialized.
     */
    private interface RowRenderer {
        JsonObject render(EventBean event);
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final RowRenderer renderer;
        private long sequence;
        private String error;

        private TraceWriter(JsonArray records, String caseName, RowRenderer renderer) {
            this.records = records;
            this.caseName = caseName;
            this.renderer = renderer;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime ignoredRuntime) {
            if (error != null) {
                return;
            }
            if (oldEvents != null && oldEvents.length > 0) {
                error = "unexpected old-stream delivery of " + oldEvents.length
                        + " events on " + statement.getName();
                return;
            }
            JsonArray rows = new JsonArray();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    JsonObject fields;
                    try {
                        fields = renderer.render(event);
                    } catch (RuntimeException ex) {
                        error = "failed to render event on " + statement.getName() + ": " + ex;
                        return;
                    }
                    rows.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("new", rows);
            records.add(record);
        }

        private long count() {
            return sequence;
        }

        /**
         * Asserts that exactly the awaited deliveries happened from the given
         * starting sequence, that each is for the expected statement and
         * carries exactly one row with the expected fields.
         */
        private void assertRows(int fromSequence, String[] statements, JsonObject[] rowsPerRecord) {
            if (error != null) {
                throw new IllegalStateException(error);
            }
            if (sequence - fromSequence != statements.length) {
                throw new IllegalStateException("case " + caseName + " received " + (sequence - fromSequence)
                        + " listener deliveries from sequence " + (fromSequence + 1) + ", expected "
                        + statements.length);
            }
            for (int i = 0; i < statements.length; i++) {
                JsonObject record = records.get(records.size() - statements.length + i).asObject();
                if (!statements[i].equals(record.getString("statement", ""))) {
                    throw new IllegalStateException("record " + (fromSequence + i + 1) + " was for statement "
                            + record.getString("statement", "") + ", expected " + statements[i]);
                }
                JsonArray delivered = record.get("new").asArray();
                if (delivered.size() != 1) {
                    throw new IllegalStateException("record " + (fromSequence + i + 1) + " carried "
                            + delivered.size() + " rows, expected 1");
                }
                JsonObject fields = delivered.get(0).asObject().get("fields").asObject();
                if (!fields.equals(rowsPerRecord[i])) {
                    throw new IllegalStateException("record " + (fromSequence + i + 1) + " carried " + fields
                            + ", expected " + rowsPerRecord[i]);
                }
            }
        }
    }
}
