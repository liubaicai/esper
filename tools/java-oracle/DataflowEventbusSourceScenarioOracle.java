import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstanceCaptive;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphEventUtil;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphEventUtil.MyDefaultSupportGraphEvent;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphParamProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.common.internal.event.core.EventServiceSendEventCommon;
import com.espertech.esper.common.internal.event.core.SendableEvent;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowOpEventBusSource;
import com.espertech.esper.regressionlib.support.dataflow.DefaultSupportCaptureOpStatic;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPDeploymentService;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.EPUndeployException;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import org.w3c.dom.Node;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpEventBusSource and
 * EPLDataflowOpFilter differential chain (work unit 4.367):
 * EventBusSource and Filter operators observed through
 * DefaultSupportCaptureOp / DefaultSupportCaptureOpStatic.
 *
 * eventbus-all-types (EPLDataflowOpEventBusSource EPLDataflowAllTypes) replays
 * runAssertionAllTypes as four sub-runs, one per event representation (POJO
 * MyDefaultSupportGraphEvent, Map MyMapEvent, XML MyXMLEvent over the
 * regression/threeProperties.xsd classloader resource, ObjectArray MyOAEvent)
 * sent through the representation's SendableEvent. Each sub-run deploys the
 * byte-exact MyDataFlowOne graph, sends events[0] before instantiate and again
 * before start (both asserted empty in-process, the pre-start drop recorded),
 * starts, sends {myDouble=1.1,myInt=1,myString=one} then
 * {myDouble=2.2,myInt=2,myString=two}, records both captured rows in send
 * order with lossless JSON decimal rendering of myDouble, cancels, records the
 * post-cancel drop, and undeploys. The two compile-time invalid graphs (zero
 * output streams; missing declared output event type, preserving the genuine
 * 'declated' typo) and the doc-sample MyDataFlow graph (SampleSchema path
 * deployment plus the three EventBusSource streams including the
 * tagId='001' filter and the MyDummyCollector collector block with trailing
 * comma) are asserted/deployed in-process with no trace rows.
 *
 * eventbus-schema-objectarray (EPLDataflowOpEventBusSource
 * EPLDataflowSchemaObjectArray) path-deploys the @public @buseventtype
 * objectarray schema MyEventOA(p0 string, p1 long) and replays the three
 * sub-runs: the EventBean&lt;MyEventOA&gt; envelope projection records
 * {"p0":"abc","p1":100}; the raw MyEventOA underlying records the Object[]
 * positionally as {"0":"abc","1":100} (raw object-array underlyings serialize
 * with ordinal-string keys, values as-is); the filter p0 like 'A%' sub-run
 * with the MyCollector parameter records the filtered-out 'B' send as an
 * empty read and the collector-resubmitted 'A' event as
 * {"p0":"A","p1":101} with in-process collector asserts (emitter non-null,
 * event type name MyEventOA, isSubmitEventBean false).
 *
 * filter-all-types (EPLDataflowOpFilter EPLDataflowAllTypes) replays
 * runAssertionAllTypes as four sub-runs in the Filter order (POJO, XML,
 * ObjectArray, Map) over the byte-exact MySelect graph
 * DefaultSupportSourceOp -> Filter(myString = 'two') -> DefaultSupportCaptureOp
 * instantiated with userObject 'myuserobject' and instanceId 'myinstanceid';
 * instance.run() is synchronous and each sub-run records the single surviving
 * 'two' row. The doc-sample MyDataFlow graph (inline SampleSchema with the
 * tab-prefixed comment, BeaconSource, single-stream and two-stream Filter)
 * and the captive two-streams MyFilter graph with the two
 * DefaultSupportCaptureOpStatic operators are deployed in-process; the
 * captive run submits SupportBean 'x' then 'y' through the 'e1' emitter and
 * records one row per static capture instance.
 *
 * filter-invalid (EPLDataflowOpFilter EPLDataflowInvalid) is covered by the
 * invalidity policy: the five compile-time Java message prefixes are asserted
 * in-process (missing filter parameter; three and zero output streams;
 * Integer-to-String implicit conversion; prev() not supported) and no trace
 * rows are emitted.
 *
 * Record protocol: every capture read is one record
 * {case, operation:"capture", statement, sequence, time, new:[rows]} with
 * rows shaped {"kind":"row","fields":{...}}; empty reads are recorded
 * explicitly with new:[]. Filter sub-runs use statement
 * "flow:DefaultSupportCaptureOp"; the two-streams captive reads use
 * "flow:DefaultSupportCaptureOpStatic" because those rows come from the
 * static capture instances. Sequence restarts at 1 per case. Session
 * configuration per case mirrors TestSuiteEPLDataflow.configure restricted to
 * these executions: the DefaultSupportGraphEventUtil representations
 * (including the XML type backed by the regression/threeProperties.xsd
 * classloader resource), the dataflow-util package import that resolves the
 * graph operator simple names, and for the Filter cases the SupportBean type
 * plus the support.dataflow package import for DefaultSupportCaptureOpStatic;
 * internal timer off, epoch initialization, and each case gets a fresh
 * runtime destroyed in finally.
 */
public final class DataflowEventbusSourceScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-eventbus-source";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEventBusSource.java";
    private static final String CASE_EVENTBUS_ALL_TYPES = "eventbus-all-types";
    private static final String CASE_EVENTBUS_SCHEMA_OA = "eventbus-schema-objectarray";
    private static final String CASE_FILTER_ALL_TYPES = "filter-all-types";
    private static final String CASE_FILTER_INVALID = "filter-invalid";
    private static final String[] CASES = {
            CASE_EVENTBUS_ALL_TYPES, CASE_EVENTBUS_SCHEMA_OA, CASE_FILTER_ALL_TYPES, CASE_FILTER_INVALID};
    private static final int[] ORDINALS = {0, 1, 1, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-dfb59d3bd4798d57cc0c",
            "java-runtime-37aed9aedcbbb25c4826",
            "java-runtime-73c4f6808b38087c5a59",
            "java-runtime-0bc68f1db4dd07d89a66"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowAllTypes",
            "EPLDataflowSchemaObjectArray",
            "EPLDataflowAllTypes",
            "EPLDataflowInvalid"};
    private static final int[] RECORD_COUNTS = {12, 4, 6, 0};
    private static final int TOTAL_RECORDS = 22;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String SCHEMA_DEPLOYMENT_ID = "schema";
    private static final String FLOW_NAME_ONE = "MyDataFlowOne";
    private static final String FLOW_NAME_DOC = "MyDataFlow";
    private static final String FLOW_NAME_SELECT = "MySelect";
    private static final String FLOW_NAME_FILTER = "MyFilter";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String CAPTURE_STATIC_STATEMENT = "flow:DefaultSupportCaptureOpStatic";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs and messages from EPLDataflowOpEventBusSource and
    // EPLDataflowOpFilter.
    private static final String SAMPLE_SCHEMA_EPL =
            "@public create schema SampleSchema(tagId string, locX double, locY double)";

    private static final String INVALID_NO_OUTPUT_STREAM = "create dataflow DF1 EventBusSource {}";
    private static final String INVALID_NO_TYPE = "create dataflow DF1 EventBusSource -> ABC {}";

    private static final String MSG_NO_OUTPUT_STREAM =
            "Failed to obtain operator 'EventBusSource': EventBusSource operator requires one output stream but produces 0 streams";
    private static final String MSG_NO_TYPE =
            "Failed to obtain operator 'EventBusSource': EventBusSource operator requires an event type declated for the output stream";

    private static final String INVALID_FILTER_MISSING =
            "create dataflow DF1 BeaconSource -> instream<SupportBean> {} Filter(instream) -> abc {}";
    private static final String INVALID_FILTER_THREE_STREAMS =
            "create dataflow DF1 BeaconSource -> instream<SupportBean> {} Filter(instream) -> abc,def,efg { filter : true }";
    private static final String INVALID_FILTER_ZERO_STREAMS =
            "create dataflow DF1 BeaconSource -> instream<SupportBean> {} Filter(instream) { filter : true }";

    private static final String MSG_FILTER_MISSING =
            "Failed to obtain operator 'Filter': Required parameter 'filter' providing the filter expression is not provided";
    private static final String MSG_FILTER_THREE_STREAMS =
            "Failed to obtain operator 'Filter': Filter operator requires one or two output stream(s) but produces 3 streams";
    private static final String MSG_FILTER_ZERO_STREAMS =
            "Failed to obtain operator 'Filter': Filter operator requires one or two output stream(s) but produces 0 streams";
    private static final String MSG_FILTER_CONVERSION =
            "Failed to obtain operator 'Filter': Failed to validate filter dataflow operator expression 'theString=1': "
                    + "Implicit conversion from datatype 'Integer' to 'String' is not allowed";
    private static final String MSG_FILTER_PREV =
            "Failed to obtain operator 'Filter': Invalid filter dataflow operator expression 'prev(theString,1)=\"abc\"': "
                    + "Aggregation, sub-select, previous or prior functions are not supported in this context";

    private DataflowEventbusSourceScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowEventbusSourceScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the four selected cases");
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
        boolean filterCase = CASE_FILTER_ALL_TYPES.equals(caseName) || CASE_FILTER_INVALID.equals(caseName);

        // Session configuration per TestSuiteEPLDataflow.configure restricted to
        // these executions: the DefaultSupportGraphEventUtil representations
        // (including the XML type backed by the regression/threeProperties.xsd
        // classloader resource), the dataflow-util package import that resolves
        // the graph operator simple names, and for the Filter cases the
        // SupportBean type/import plus the support.dataflow package import for
        // DefaultSupportCaptureOpStatic.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        // Harness base configuration (SupportConfigFactory.getConfiguration)
        // enables XML-XSD; the MyXMLEvent representation requires it to map the
        // regression/threeProperties.xsd schema.
        configuration.getCommon().getEventMeta().setEnableXMLXSD(true);
        DefaultSupportGraphEventUtil.addTypeConfiguration(configuration);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        if (filterCase) {
            configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
            configuration.getCommon().addImport(SupportBean.class);
            configuration.getCommon().addImport(DefaultSupportCaptureOpStatic.class.getPackage().getName() + ".*");
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-eventbus-source-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_EVENTBUS_ALL_TYPES.equals(caseName)) {
                runEventbusAllTypes(configuration, runtime, writer);
            } else if (CASE_EVENTBUS_SCHEMA_OA.equals(caseName)) {
                runEventbusSchemaObjectArray(configuration, runtime, writer);
            } else if (CASE_FILTER_ALL_TYPES.equals(caseName)) {
                runFilterAllTypes(configuration, runtime, writer);
            } else if (CASE_FILTER_INVALID.equals(caseName)) {
                runFilterInvalid(configuration, runtime);
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
     * EPLDataflowOpEventBusSource.EPLDataflowAllTypes: four sub-runs, one per
     * event representation in the suite order; each records the pre-start
     * drop, the two started rows in send order and the post-cancel drop.
     */
    private static void runEventbusAllTypes(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        String[] typeNames = {
                DefaultSupportGraphEventUtil.EVENTTYPENAME, "MyMapEvent", "MyXMLEvent", "MyOAEvent"};
        SendableEvent[][] eventSets = {
                DefaultSupportGraphEventUtil.getPOJOEventsSendable(),
                DefaultSupportGraphEventUtil.getMapEventsSendable(),
                DefaultSupportGraphEventUtil.getXMLEventsSendable(),
                DefaultSupportGraphEventUtil.getOAEventsSendable()};
        JsonObject[] expectedRows = {
                new JsonObject().add("myDouble", 1.1).add("myInt", 1).add("myString", "one"),
                new JsonObject().add("myDouble", 2.2).add("myInt", 2).add("myString", "two")};
        EventServiceSendEventCommon eventService = (EventServiceSendEventCommon) runtime.getEventService();

        for (int subRun = 0; subRun < typeNames.length; subRun++) {
            String typeName = typeNames[subRun];
            SendableEvent[] events = eventSets[subRun];

            EPCompiled flow = compile(configuration, runtime, eventbusFlowEpl(typeName));
            deploy(runtime, flow, FLOW_DEPLOYMENT_ID);

            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                    .operatorProvider(new DefaultSupportGraphOpProvider(capture));

            // sent before instantiate: no capture wiring exists yet.
            events[0].send(eventService);
            assertCurrentEmpty(capture, "sub-run " + typeName + " before instantiate");

            EPDataFlowInstance df = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);

            // sent after instantiate but before start: the source drops it.
            events[0].send(eventService);
            assertCurrentEmpty(capture, "sub-run " + typeName + " before start");
            writer.addCapture(new JsonArray());

            df.start();

            // send events
            for (SendableEvent event : events) {
                event.send(eventService);
            }
            capture.waitForInvocation(200, events.length);
            Object[] rows = capture.getCurrentAndReset();
            if (rows.length != events.length) {
                throw new IllegalStateException("sub-run " + typeName + " capture held " + rows.length
                        + " rows, expected " + events.length);
            }
            JsonArray captured = new JsonArray();
            for (int i = 0; i < events.length; i++) {
                // assertSame(events[i].getUnderlying(), rows[i]): the source
                // passes the same underlying instance through.
                if (rows[i] != events[i].getUnderlying()) {
                    throw new IllegalStateException("sub-run " + typeName + " row " + i + " was " + rows[i]
                            + ", expected the sent " + typeName + " underlying instance");
                }
                JsonObject fields = allTypesRowFields(rows[i]);
                if (!fields.equals(expectedRows[i])) {
                    throw new IllegalStateException("sub-run " + typeName + " row " + i + " was " + fields
                            + ", expected " + expectedRows[i]);
                }
                captured.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            writer.addCapture(captured);

            df.cancel();

            events[0].send(eventService);
            Thread.sleep(50);
            assertCurrentEmpty(capture, "sub-run " + typeName + " after cancel");
            writer.addCapture(new JsonArray());

            runtime.getDeploymentService().undeployAll();
        }

        // invalid: no output stream
        try {
            compile(configuration, runtime, INVALID_NO_OUTPUT_STREAM);
            throw new IllegalStateException("expected EPCompileException for zero output stream flow");
        } catch (EPCompileException ex) {
            assertPrefix(MSG_NO_OUTPUT_STREAM, ex.getMessage());
        }

        // invalid: type not found
        try {
            compile(configuration, runtime, INVALID_NO_TYPE);
            throw new IllegalStateException("expected EPCompileException for undeclared output type flow");
        } catch (EPCompileException ex) {
            assertPrefix(MSG_NO_TYPE, ex.getMessage());
        }

        // test doc samples
        deploy(runtime, compile(configuration, runtime, SAMPLE_SCHEMA_EPL), SCHEMA_DEPLOYMENT_ID);
        String epl = "@name('flow') create dataflow " + FLOW_NAME_DOC + "\n" +
                "\n" +
                "  // Receive all SampleSchema events from the event bus.\n" +
                "  // No transformation.\n" +
                "  EventBusSource -> stream.one<SampleSchema> {}\n" +
                "  \n" +
                "  // Receive all SampleSchema events with tag id '001' from the event bus.\n" +
                "  // No transformation.\n" +
                "  EventBusSource -> stream.one<SampleSchema> {\n" +
                "    filter : tagId = '001'\n" +
                "  }\n" +
                "\n" +
                "  // Receive all SampleSchema events from the event bus.\n" +
                "  // With collector that performs transformation.\n" +
                "  EventBusSource -> stream.two<SampleSchema> {\n" +
                "    collector : {\n" +
                "      class : '" + EPLDataflowOpEventBusSource.MyDummyCollector.class.getName() + "'\n" +
                "    },\n" +
                "  }";
        deploy(runtime, compile(configuration, runtime, epl), FLOW_DEPLOYMENT_ID);
        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_DOC);
    }

    private static String eventbusFlowEpl(String typeName) {
        return "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
                "EventBusSource -> ReceivedStream<" + typeName + "> {} " +
                "DefaultSupportCaptureOp(ReceivedStream) {}";
    }

    /**
     * EPLDataflowOpEventBusSource.EPLDataflowSchemaObjectArray: envelope,
     * raw-underlying and filter+collector sub-runs over the @buseventtype
     * objectarray schema.
     */
    private static void runEventbusSchemaObjectArray(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled schema = compile(configuration, runtime,
                "@public @buseventtype create objectarray schema MyEventOA(p0 string, p1 long)");
        deploy(runtime, schema, SCHEMA_DEPLOYMENT_ID);

        // envelope: EventBean<MyEventOA> output port, projected properties.
        EPCompiled flow = compile(configuration, runtime, eventbusFlowEpl("EventBean<MyEventOA>"));
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);
        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(1);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);
        instance.start();
        runtime.getEventService().sendEventObjectArray(new Object[]{"abc", 100L}, "MyEventOA");
        Object[] rows = capture.get(1, TimeUnit.SECONDS);
        if (rows.length != 1 || !(rows[0] instanceof EventBean)) {
            throw new IllegalStateException("envelope capture held " + rows.length + " rows, expected one EventBean");
        }
        EventBean event = (EventBean) rows[0];
        if (!"abc".equals(event.get("p0")) || !Long.valueOf(100L).equals(event.get("p1"))) {
            throw new IllegalStateException("envelope row was " + event.get("p0") + "/" + event.get("p1")
                    + ", expected abc/100");
        }
        writer.addCapture(singleRow(new JsonObject().add("p0", "abc").add("p1", 100L)));
        instance.cancel();
        undeployModuleContaining(runtime, FLOW_DEPLOYMENT_ID);

        // underlying: raw Object[] output port, asserted exactly, serialized
        // positionally with ordinal-string keys.
        flow = compile(configuration, runtime, eventbusFlowEpl("MyEventOA"));
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);
        capture = new DefaultSupportCaptureOp<Object>(1);
        options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);
        instance.start();
        runtime.getEventService().sendEventObjectArray(new Object[]{"abc", 100L}, "MyEventOA");
        rows = capture.get(1, TimeUnit.SECONDS);
        if (rows.length != 1 || !(rows[0] instanceof Object[])) {
            throw new IllegalStateException("underlying capture held " + rows.length + " rows, expected one Object[]");
        }
        Object[] underlying = (Object[]) rows[0];
        if (underlying.length != 2 || !"abc".equals(underlying[0]) || !Long.valueOf(100L).equals(underlying[1])) {
            throw new IllegalStateException("underlying row was " + underlying.length + " columns, expected abc/100");
        }
        writer.addCapture(singleRow(new JsonObject().add("0", "abc").add("1", 100L)));
        instance.cancel();
        undeployModuleContaining(runtime, FLOW_DEPLOYMENT_ID);

        // filter + collector: 'B' is filtered out, 'A' is resubmitted by the
        // collector and captured.
        flow = compile(configuration, runtime, "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
                "EventBusSource -> ReceivedStream<MyEventOA> {filter: p0 like 'A%'} " +
                "DefaultSupportCaptureOp(ReceivedStream) {}");
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);
        EPLDataflowOpEventBusSource.MyCollector collector = new EPLDataflowOpEventBusSource.MyCollector();
        capture = new DefaultSupportCaptureOp<Object>();
        options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture))
                .parameterProvider(new DefaultSupportGraphParamProvider(
                        Collections.<String, Object>singletonMap("collector", collector)));
        instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_ONE, options);
        instance.start();

        runtime.getEventService().sendEventObjectArray(new Object[]{"B", 100L}, "MyEventOA");
        Thread.sleep(50);
        if (collector.getLast() != null) {
            throw new IllegalStateException("collector saw the filtered-out 'B' event");
        }
        assertCurrentEmpty(capture, "filter+collector after 'B'");
        writer.addCapture(new JsonArray());

        runtime.getEventService().sendEventObjectArray(new Object[]{"A", 101L}, "MyEventOA");
        capture.waitForInvocation(100, 1);
        if (collector.getLast() == null || collector.getLast().getEmitter() == null) {
            throw new IllegalStateException("collector context emitter was null");
        }
        if (!"MyEventOA".equals(collector.getLast().getEvent().getEventType().getName())) {
            throw new IllegalStateException("collector event type was "
                    + collector.getLast().getEvent().getEventType().getName() + ", expected MyEventOA");
        }
        if (collector.getLast().isSubmitEventBean()) {
            throw new IllegalStateException("collector isSubmitEventBean was true, expected false");
        }
        rows = capture.getCurrentAndReset();
        if (rows.length != 1) {
            throw new IllegalStateException("filter+collector capture held " + rows.length + " rows, expected 1");
        }
        JsonObject fields = oaRowFields(rows[0]);
        JsonObject expected = new JsonObject().add("p0", "A").add("p1", 101L);
        if (!fields.equals(expected)) {
            throw new IllegalStateException("filter+collector row was " + fields + ", expected " + expected);
        }
        writer.addCapture(singleRow(fields));
        instance.cancel();

        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Renders one MyEventOA captured row: EventBean envelope (property
     * access) or raw Object[] underlying, serialized positionally with
     * ordinal-string keys.
     */
    private static JsonObject oaRowFields(Object row) {
        if (row instanceof EventBean) {
            EventBean event = (EventBean) row;
            return new JsonObject().add("p0", String.valueOf(event.get("p0")))
                    .add("p1", ((Number) event.get("p1")).longValue());
        }
        if (row instanceof Object[]) {
            Object[] array = (Object[]) row;
            if (array.length != 2) {
                throw new IllegalStateException("object-array row held " + array.length + " columns, expected 2");
            }
            return new JsonObject().add("0", String.valueOf(array[0]))
                    .add("1", ((Number) array[1]).longValue());
        }
        throw new IllegalStateException("unexpected MyEventOA row of " + row.getClass().getName() + ": " + row);
    }

    /**
     * EPLDataflowOpFilter.EPLDataflowAllTypes: four sub-runs in the Filter
     * order (POJO, XML, ObjectArray, Map) over the synchronous
     * DefaultSupportSourceOp -> Filter -> DefaultSupportCaptureOp graph, then
     * the doc-sample flow and the captive two-streams static-capture run.
     */
    private static void runFilterAllTypes(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        String[] typeNames = {
                DefaultSupportGraphEventUtil.EVENTTYPENAME, "MyXMLEvent", "MyOAEvent", "MyMapEvent"};
        Object[][] eventSets = {
                DefaultSupportGraphEventUtil.getPOJOEvents(),
                DefaultSupportGraphEventUtil.getXMLEvents(),
                DefaultSupportGraphEventUtil.getOAEvents(),
                DefaultSupportGraphEventUtil.getMapEvents()};
        JsonObject expectedRow = new JsonObject().add("myDouble", 2.2).add("myInt", 2).add("myString", "two");

        for (int subRun = 0; subRun < typeNames.length; subRun++) {
            String typeName = typeNames[subRun];
            Object[] events = eventSets[subRun];
            String graph = "@name('flow') create dataflow " + FLOW_NAME_SELECT + "\n" +
                    "DefaultSupportSourceOp -> instream.with.dot<" + typeName + ">{}\n" +
                    "Filter(instream.with.dot) -> outstream.dot {filter: myString = 'two'}\n" +
                    "DefaultSupportCaptureOp(outstream.dot) {}";
            deploy(runtime, compile(configuration, runtime, graph), FLOW_DEPLOYMENT_ID);

            DefaultSupportSourceOp source = new DefaultSupportSourceOp(events);
            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(2);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
            options.setDataFlowInstanceUserObject("myuserobject");
            options.setDataFlowInstanceId("myinstanceid");
            options.operatorProvider(new DefaultSupportGraphOpProvider(source, capture));
            EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_SELECT, options);
            if (!"myuserobject".equals(instance.getUserObject())) {
                throw new IllegalStateException("instance userObject was " + instance.getUserObject());
            }
            if (!"myinstanceid".equals(instance.getInstanceId())) {
                throw new IllegalStateException("instance id was " + instance.getInstanceId());
            }

            instance.run();

            List<List<Object>> batches = capture.getAndReset();
            if (batches.isEmpty() || batches.get(0).size() != 1) {
                throw new IllegalStateException("sub-run " + typeName + " produced " + batches.size()
                        + " batches, expected one single-row batch");
            }
            Object[] result = batches.get(0).toArray();
            if (result.length != 1 || result[0] != events[1]) {
                throw new IllegalStateException("sub-run " + typeName + " row was " + (result.length == 0 ? null : result[0])
                        + ", expected the sent 'two' underlying instance");
            }
            JsonObject fields = allTypesRowFields(result[0]);
            if (!fields.equals(expectedRow)) {
                throw new IllegalStateException("sub-run " + typeName + " row was " + fields + ", expected " + expectedRow);
            }
            writer.addCapture(singleRow(fields));

            instance.cancel();

            runtime.getDeploymentService().undeployAll();
        }

        // test doc sample
        String epl = "@name('flow') create dataflow " + FLOW_NAME_DOC + "\n" +
                "  create schema SampleSchema(tagId string, locX double),\t// sample type\n" +
                "  BeaconSource -> samplestream<SampleSchema> {}\n" +
                "  \n" +
                "  // Filter all events that have a tag id of '001'\n" +
                "  Filter(samplestream) -> tags_001 {\n" +
                "    filter : tagId = '001' \n" +
                "  }\n" +
                "  \n" +
                "  // Filter all events that have a tag id of '001', putting all other tags into the second stream\n" +
                "  Filter(samplestream) -> tags_001, tags_other {\n" +
                "    filter : tagId = '001' \n" +
                "  }";
        deploy(runtime, compile(configuration, runtime, epl), FLOW_DEPLOYMENT_ID);
        runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_DOC);
        runtime.getDeploymentService().undeployAll();

        // test two streams
        DefaultSupportCaptureOpStatic.getInstances().clear();
        String graph = "@name('flow') create dataflow " + FLOW_NAME_FILTER + "\n" +
                "Emitter -> sb<SupportBean> {name : 'e1'}\n" +
                "Filter(sb) -> out.ok, out.fail {filter: theString = 'x'}\n" +
                "DefaultSupportCaptureOpStatic(out.ok) {}" +
                "DefaultSupportCaptureOpStatic(out.fail) {}";
        deploy(runtime, compile(configuration, runtime, graph), FLOW_DEPLOYMENT_ID);

        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME_FILTER);
        EPDataFlowInstanceCaptive captive = instance.startCaptive();

        captive.getEmitters().get("e1").submit(new SupportBean("x", 10));
        List<DefaultSupportCaptureOpStatic> staticInstances = DefaultSupportCaptureOpStatic.getInstances();
        if (staticInstances.size() != 2) {
            throw new IllegalStateException("held " + staticInstances.size() + " static captures, expected 2");
        }
        writer.addCapture(CAPTURE_STATIC_STATEMENT, supportBeanRow(staticInstances.get(0), "x", 10));
        captive.getEmitters().get("e1").submit(new SupportBean("y", 11));
        writer.addCapture(CAPTURE_STATIC_STATEMENT, supportBeanRow(staticInstances.get(1), "y", 11));
        DefaultSupportCaptureOpStatic.getInstances().clear();

        instance.cancel();

        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Reads one DefaultSupportCaptureOpStatic instance, asserts it holds
     * exactly the submitted SupportBean and renders its two fields.
     */
    private static JsonArray supportBeanRow(DefaultSupportCaptureOpStatic capture, String theString, int intPrimitive) {
        List<Object> current = capture.getCurrent();
        if (current.size() != 1 || !(current.get(0) instanceof SupportBean)) {
            throw new IllegalStateException("static capture held " + current.size()
                    + " rows, expected one SupportBean");
        }
        SupportBean bean = (SupportBean) current.get(0);
        if (!theString.equals(bean.getTheString()) || bean.getIntPrimitive() != intPrimitive) {
            throw new IllegalStateException("static capture row was " + bean.getTheString() + "/"
                    + bean.getIntPrimitive() + ", expected " + theString + "/" + intPrimitive);
        }
        JsonObject fields = new JsonObject().add("theString", bean.getTheString())
                .add("intPrimitive", bean.getIntPrimitive());
        return new JsonArray().add(new JsonObject().add("kind", "row").add("fields", fields));
    }

    /**
     * EPLDataflowOpFilter.EPLDataflowInvalid: compile rejections only,
     * asserted as Java message prefixes in-process; no trace rows (invalidity
     * policy).
     */
    private static void runFilterInvalid(Configuration configuration, EPRuntime runtime) throws Exception {
        // invalid: no filter
        tryInvalidCompile(configuration, runtime, INVALID_FILTER_MISSING, MSG_FILTER_MISSING);

        // invalid: too many output streams
        tryInvalidCompile(configuration, runtime, INVALID_FILTER_THREE_STREAMS, MSG_FILTER_THREE_STREAMS);

        // invalid: too few output streams
        tryInvalidCompile(configuration, runtime, INVALID_FILTER_ZERO_STREAMS, MSG_FILTER_ZERO_STREAMS);

        // invalid filter expressions
        tryInvalidFilter(configuration, runtime, "theString = 1", MSG_FILTER_CONVERSION);
        tryInvalidFilter(configuration, runtime, "prev(theString, 1) = 'abc'", MSG_FILTER_PREV);
    }

    private static void tryInvalidFilter(Configuration configuration, EPRuntime runtime, String filter, String message)
            throws Exception {
        String graph = "@name('flow') create dataflow " + FLOW_NAME_SELECT + "\n" +
                "DefaultSupportSourceOp -> instream<SupportBean>{}\n" +
                "Filter(instream as ME) -> outstream {filter: " + filter + "}\n" +
                "DefaultSupportCaptureOp(outstream) {}";
        tryInvalidCompile(configuration, runtime, graph, message);
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

    /**
     * Renders one all-types captured row: POJO bean, Map, Object[] (positional
     * myDouble,myInt,myString) or XML Node (attributes), all to the same typed
     * fields with a lossless double.
     */
    private static JsonObject allTypesRowFields(Object row) {
        if (row instanceof MyDefaultSupportGraphEvent) {
            MyDefaultSupportGraphEvent bean = (MyDefaultSupportGraphEvent) row;
            return new JsonObject().add("myDouble", bean.getMyDouble())
                    .add("myInt", bean.getMyInt()).add("myString", bean.getMyString());
        }
        if (row instanceof Map) {
            Map<?, ?> map = (Map<?, ?>) row;
            if (map.size() != 3 || !map.containsKey("myDouble") || !map.containsKey("myInt")
                    || !map.containsKey("myString")) {
                throw new IllegalStateException("map row " + map + " does not hold myDouble/myInt/myString");
            }
            return new JsonObject().add("myDouble", ((Number) map.get("myDouble")).doubleValue())
                    .add("myInt", ((Number) map.get("myInt")).intValue())
                    .add("myString", String.valueOf(map.get("myString")));
        }
        if (row instanceof Object[]) {
            Object[] array = (Object[]) row;
            if (array.length != 3) {
                throw new IllegalStateException("object-array row held " + array.length + " columns, expected 3");
            }
            return new JsonObject().add("myDouble", ((Number) array[0]).doubleValue())
                    .add("myInt", ((Number) array[1]).intValue())
                    .add("myString", String.valueOf(array[2]));
        }
        if (row instanceof Node) {
            Node node = (Node) row;
            return new JsonObject().add("myDouble", Double.parseDouble(attribute(node, "myDouble")))
                    .add("myInt", Integer.parseInt(attribute(node, "myInt")))
                    .add("myString", attribute(node, "myString"));
        }
        throw new IllegalStateException("unexpected all-types row of " + row.getClass().getName() + ": " + row);
    }

    private static String attribute(Node node, String name) {
        Node item = node.getAttributes().getNamedItem(name);
        if (item == null) {
            throw new IllegalStateException("node " + node.getNodeName() + " has no attribute " + name);
        }
        return item.getNodeValue();
    }

    private static JsonArray singleRow(JsonObject fields) {
        return new JsonArray().add(new JsonObject().add("kind", "row").add("fields", fields));
    }

    private static void assertPrefix(String expected, String message) {
        if (message == null || !message.startsWith(expected)) {
            throw new IllegalStateException("Expected prefix:\n" + expected + "\nbut received:\n" + message);
        }
    }

    private static void assertCurrentEmpty(DefaultSupportCaptureOp<Object> capture, String phase) {
        Object[] current = capture.getCurrent();
        if (current.length != 0) {
            throw new IllegalStateException("capture held " + current.length + " rows " + phase + ", expected 0");
        }
    }

    private static void undeployModuleContaining(EPRuntime runtime, String statementName) throws EPUndeployException {
        EPDeploymentService deployments = runtime.getDeploymentService();
        for (String deploymentId : deployments.getDeployments()) {
            EPDeployment deployment = deployments.getDeployment(deploymentId);
            if (deployment == null) {
                continue;
            }
            for (EPStatement statement : deployment.getStatements()) {
                if (statementName.equals(statement.getName())) {
                    deployments.undeploy(deploymentId);
                    return;
                }
            }
        }
        throw new IllegalStateException("no deployment containing statement '" + statementName + "'");
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

        private void addCapture(JsonArray rows) {
            addCapture(CAPTURE_STATEMENT, rows);
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
