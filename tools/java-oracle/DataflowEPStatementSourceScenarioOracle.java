import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventSender;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationException;
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
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowOpEPStatementSource;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
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
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowOpEPStatementSource differential
 * chain (work unit 4.362): EPStatementSource registry/attachment lifecycle
 * observed through DefaultSupportCaptureOp.
 *
 * all-types (EPLDataflowAllTypes) replays runAssertionStatementNameExists as
 * four sub-runs, one per event representation (POJO MyDefaultSupportGraphEvent,
 * Map MyMapEvent, ObjectArray MyOAEvent, XML MyXMLEvent over the
 * regression/threeProperties.xsd classloader resource). Each sub-run deploys
 * the paired @Name('MyStatement') select * statement, interpolates its
 * deployment id into the byte-exact MyDataFlowOne graph, sends
 * {myDouble=1.1,myInt=1,myString=one} then {myDouble=2.2,myInt=2,myString=two}
 * through the representation's EventSender, and records both captured rows with
 * lossless JSON decimal rendering of myDouble (double-preserving normalize, not
 * the long-collapsing one).
 *
 * stmt-name-dynamic (EPLDataflowStmtNameDynamic) replays the fixed
 * statementDeploymentId 'MyDeploymentId' + statementName 'MyStatement' lifecycle
 * byte-exact: empty read while the statement does not exist, attach on deploy,
 * detach on undeploy, reattach on redeploying the same compiled unit, detach
 * again, and attach of a redefined statement whose projection differs.
 *
 * statement-filter (EPLDataflowStatementFilter) replays the pass-everything
 * MyFilter parameterProvider lifecycle: the pre-existing unnamed SupportBean_B
 * statement attaches at open, dynamically deployed statements attach on
 * deployment, undeploy detaches, redeploy reattaches, previously attached
 * statements remain attached, and df.cancel() silences the source.
 *
 * invalid (EPLDataflowInvalid) is covered by the invalidity policy: the oracle
 * asserts the three Java message prefixes in-process (instantiate-time missing
 * statementName/statementFilter parameter; compile-time zero output stream;
 * compile-time statementDeploymentId/statementName pairing) and emits no trace
 * rows.
 *
 * Record protocol: every capture read is one record
 * {case, operation:"capture", statement:"flow:DefaultSupportCaptureOp",
 * sequence, time, new:[rows]} with rows shaped
 * {"kind":"row","fields":{...}}; empty reads are recorded explicitly with
 * new:[]. Session configuration per case mirrors TestSuiteEPLDataflow.configure
 * restricted to these executions: SupportBean/SupportBean_A/SupportBean_B types
 * plus the DefaultSupportGraphEventUtil representations, the dataflow-util
 * package import, the SupportBean import, internal timer off, and epoch
 * initialization; each case gets a fresh runtime destroyed in finally.
 */
public final class DataflowEPStatementSourceScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-epstatement-source";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEPStatementSource.java";
    private static final String CASE_ALL_TYPES = "all-types";
    private static final String CASE_STMT_NAME_DYNAMIC = "stmt-name-dynamic";
    private static final String CASE_STATEMENT_FILTER = "statement-filter";
    private static final String CASE_INVALID = "invalid";
    private static final String[] CASES = {CASE_ALL_TYPES, CASE_STMT_NAME_DYNAMIC, CASE_STATEMENT_FILTER, CASE_INVALID};
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-2d7b6229c7a3b2bee40b",
            "java-runtime-950696d7aa6356acb9d4",
            "java-runtime-8c8f6f03b11605a16d11",
            "java-runtime-ef085a37ed45eb35a13f"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowAllTypes",
            "EPLDataflowStmtNameDynamic",
            "EPLDataflowStatementFilter",
            "EPLDataflowInvalid"};
    private static final int[] RECORD_COUNTS = {4, 6, 7, 0};
    private static final int TOTAL_RECORDS = 17;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME = "MyDataFlowOne";
    private static final String STATEMENT_NAME = "MyStatement";
    private static final String STATEMENT_DEPLOYMENT_ID = "MyStatementDeployment";
    private static final String STATEMENT_DEPLOYMENT_ID_PINNED = "MyDeploymentId";
    private static final String CAPTURE_STATEMENT = "flow:DefaultSupportCaptureOp";
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact graphs and statements from EPLDataflowOpEPStatementSource.
    private static final String[] ALL_TYPES_TYPE_NAMES = {
            DefaultSupportGraphEventUtil.EVENTTYPENAME, "MyMapEvent", "MyOAEvent", "MyXMLEvent"};

    private static final String STMT_NAME_DYNAMIC_FLOW =
            "@name('flow') create dataflow MyDataFlowOne " +
            "create map schema SingleProp (id string), " +
            "EPStatementSource -> thedata<SingleProp> {" +
            "  statementDeploymentId : 'MyDeploymentId'," +
            "  statementName : 'MyStatement'," +
            "} " +
            "DefaultSupportCaptureOp(thedata) {}";

    private static final String STATEMENT_FILTER_FLOW =
            "@name('flow') create dataflow MyDataFlowOne " +
            "create schema AllObjects as java.lang.Object," +
            "EPStatementSource -> thedata<AllObjects> {} " +
            "DefaultSupportCaptureOp(thedata) {}";

    private static final String INVALID_INSTANTIATE_FLOW =
            "create dataflow DF1 " +
            "create schema AllObjects as java.lang.Object," +
            "EPStatementSource -> thedata<AllObjects> {} " +
            "DefaultSupportCaptureOp(thedata) {}";

    private static final String INVALID_NO_OUTPUT_STREAM = "create dataflow DF1 EPStatementSource { statementName : 'abc' }";
    private static final String INVALID_PAIRING = "create dataflow DF1 EPStatementSource ->abc { statementName : 'abc' }";

    private static final String MSG_INSTANTIATE_MISSING_PARAMETER =
            "Failed to instantiate data flow 'DF1': Failed to obtain operator instance for 'EPStatementSource': "
                    + "Failed to find required 'statementName' or 'statementFilter' parameter";
    private static final String MSG_NO_OUTPUT_STREAM =
            "Failed to obtain operator 'EPStatementSource': EPStatementSource operator requires one output stream but produces 0 streams";
    private static final String MSG_PAIRING =
            "Failed to obtain operator 'EPStatementSource': Both 'statementDeploymentId' and 'statementName' are required when either of these are specified";

    private DataflowEPStatementSourceScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowEPStatementSourceScenarioOracle <scenario.json>");
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

        // Session configuration per TestSuiteEPLDataflow.configure restricted to
        // these executions: the SupportBean/SupportBean_A/SupportBean_B types,
        // the DefaultSupportGraphEventUtil representations (including the XML
        // type backed by the regression/threeProperties.xsd classloader
        // resource), the dataflow-util package import that resolves the graph
        // operator simple names, and the SupportBean import.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        // Harness base configuration (SupportConfigFactory.getConfiguration)
        // enables XML-XSD; the MyXMLEvent representation requires it to map the
        // regression/threeProperties.xsd schema.
        configuration.getCommon().getEventMeta().setEnableXMLXSD(true);
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_A.class.getSimpleName(), SupportBean_A.class);
        configuration.getCommon().addEventType(SupportBean_B.class.getSimpleName(), SupportBean_B.class);
        DefaultSupportGraphEventUtil.addTypeConfiguration(configuration);
        configuration.getCommon().addImport(DefaultSupportCaptureOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-dataflow-epstatement-source-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            if (CASE_ALL_TYPES.equals(caseName)) {
                runAllTypes(configuration, runtime, writer);
            } else if (CASE_STMT_NAME_DYNAMIC.equals(caseName)) {
                runStmtNameDynamic(configuration, runtime, writer);
            } else if (CASE_STATEMENT_FILTER.equals(caseName)) {
                runStatementFilter(configuration, runtime, writer);
            } else if (CASE_INVALID.equals(caseName)) {
                runInvalid(configuration, runtime);
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
     * EPLDataflowAllTypes.runAssertionStatementNameExists: four sub-runs, one
     * per event representation; each deploys the paired statement, interpolates
     * its deployment id into the byte-exact graph, sends the two representation
     * events and records the two captured rows.
     */
    private static void runAllTypes(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        Object[][] eventSets = {
                DefaultSupportGraphEventUtil.getPOJOEvents(),
                DefaultSupportGraphEventUtil.getMapEvents(),
                DefaultSupportGraphEventUtil.getOAEvents(),
                DefaultSupportGraphEventUtil.getXMLEvents()};
        JsonObject[] expectedRows = {
                new JsonObject().add("myDouble", 1.1).add("myInt", 1).add("myString", "one"),
                new JsonObject().add("myDouble", 2.2).add("myInt", 2).add("myString", "two")};

        for (int subRun = 0; subRun < ALL_TYPES_TYPE_NAMES.length; subRun++) {
            String typeName = ALL_TYPES_TYPE_NAMES[subRun];
            Object[] events = eventSets[subRun];

            EPCompiled statement = compile(configuration, runtime, "@Name('MyStatement') select * from " + typeName);
            deploy(runtime, statement, STATEMENT_DEPLOYMENT_ID);

            // Deployment id interpolated into the byte-exact graph, mirroring
            // env.deploymentId("MyStatement") in the suite.
            EPCompiled flow = compile(configuration, runtime, allTypesFlowEpl(STATEMENT_DEPLOYMENT_ID));
            deploy(runtime, flow, FLOW_DEPLOYMENT_ID);

            DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>(2);
            EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                    .operatorProvider(new DefaultSupportGraphOpProvider(capture));
            EPDataFlowInstance df = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
            df.start();

            EventSender sender = runtime.getEventService().getEventSender(typeName);
            for (Object event : events) {
                sender.sendEvent(event);
            }

            Object[] rows = capture.get(1, TimeUnit.SECONDS);
            if (rows.length != 2) {
                throw new IllegalStateException("sub-run " + typeName + " capture held " + rows.length + " rows, expected 2");
            }
            JsonArray captured = new JsonArray();
            for (int i = 0; i < 2; i++) {
                // assertEqualsExactOrder(events, captureOp.getCurrent()): the
                // wildcard select passes the same underlying instance through.
                if (rows[i] != events[i]) {
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
            runtime.getDeploymentService().undeployAll();
        }
    }

    private static String allTypesFlowEpl(String statementDeploymentId) {
        return "@name('flow') create dataflow MyDataFlowOne " +
                "create schema AllObject java.lang.Object," +
                "EPStatementSource -> thedata<AllObject> {" +
                "  statementDeploymentId : '" + statementDeploymentId + "'," +
                "  statementName : 'MyStatement'," +
                "} " +
                "DefaultSupportCaptureOp(thedata) {}";
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

    /**
     * EPLDataflowStmtNameDynamic: fixed deployment id 'MyDeploymentId' +
     * statement name 'MyStatement' lifecycle; six reads over the attach,
     * detach, redeploy-reattach, detach and redefinition schedule.
     */
    private static void runStmtNameDynamic(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        EPCompiled flow = compile(configuration, runtime, STMT_NAME_DYNAMIC_FLOW);
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(capture));
        EPDataFlowInstance df = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        if (df.getUserObject() != null) {
            throw new IllegalStateException("instance userObject was " + df.getUserObject() + ", expected null");
        }
        if (df.getInstanceId() != null) {
            throw new IllegalStateException("instance id was " + df.getInstanceId() + ", expected null");
        }
        df.start();

        // 1. statement does not exist yet: E1 is dropped.
        runtime.getEventService().sendEventBean(new SupportBean("E1", 1), "SupportBean");
        assertCurrentEmpty(capture, "before MyStatement exists");
        writer.addCapture(new JsonArray());

        // 2. deploy the named statement: E2 projects id=E2.
        EPCompiled compiled = compile(configuration, runtime,
                "@Name('MyStatement') select theString as id from SupportBean");
        deploy(runtime, compiled, STATEMENT_DEPLOYMENT_ID_PINNED);
        runtime.getEventService().sendEventBean(new SupportBean("E2", 2), "SupportBean");
        capture.waitForInvocation(100, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "E2"));

        // 3. undeploy detaches: E3 is dropped.
        undeployModuleContaining(runtime, STATEMENT_NAME);
        runtime.getEventService().sendEventBean(new SupportBean("E3", 3), "SupportBean");
        assertCurrentEmpty(capture, "after MyStatement undeploy");
        writer.addCapture(new JsonArray());

        // 4. redeploy the same compiled unit: E4 projects id=E4.
        deploy(runtime, compiled, STATEMENT_DEPLOYMENT_ID_PINNED);
        runtime.getEventService().sendEventBean(new SupportBean("E4", 4), "SupportBean");
        capture.waitForInvocation(100, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "E4"));

        // 5. undeploy detaches again: E5 is dropped.
        undeployModuleContaining(runtime, STATEMENT_NAME);
        runtime.getEventService().sendEventBean(new SupportBean("E5", 5), "SupportBean");
        assertCurrentEmpty(capture, "after MyStatement second undeploy");
        writer.addCapture(new JsonArray());

        // 6. redefined statement changes the projection: E6 projects id=XE6X.
        compiled = compile(configuration, runtime,
                "@Name('MyStatement') select 'X'||theString||'X' as id from SupportBean");
        deploy(runtime, compiled, STATEMENT_DEPLOYMENT_ID_PINNED);
        runtime.getEventService().sendEventBean(new SupportBean("E6", 6), "SupportBean");
        capture.waitForInvocation(100, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "XE6X"));

        df.cancel();
    }

    private static JsonArray singleIdRow(Object[] rows, String expectedId) {
        if (rows.length != 1) {
            throw new IllegalStateException("capture held " + rows.length + " rows, expected 1");
        }
        // The thedata<SingleProp> map-schema port wraps the projected row into
        // the SingleProp map {id=...}; assertProps reads the "id" property.
        Object row = rows[0];
        String id;
        if (row instanceof Map) {
            Object value = ((Map<?, ?>) row).get("id");
            id = value == null ? null : String.valueOf(value);
        } else if (row instanceof String) {
            id = (String) row;
        } else {
            throw new IllegalStateException("capture row was " + row + ", expected SingleProp id=" + expectedId);
        }
        if (!expectedId.equals(id)) {
            throw new IllegalStateException("capture row id was " + id + ", expected " + expectedId);
        }
        return new JsonArray().add(new JsonObject().add("kind", "row")
                .add("fields", new JsonObject().add("id", expectedId)));
    }

    /**
     * EPLDataflowStatementFilter: pass-everything statementFilter attaches
     * matching statements at open and on deployment; seven reads over the
     * pre-existing, dynamic, detach, reattach, persist and post-cancel
     * schedule.
     */
    private static void runStatementFilter(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // one statement exists before the data flow
        EPCompiled preExisting = compile(configuration, runtime, "select id from SupportBean_B");
        deploy(runtime, preExisting, "stmt-b-pre");

        EPCompiled flow = compile(configuration, runtime, STATEMENT_FILTER_FLOW);
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);

        DefaultSupportCaptureOp<Object> capture = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions();
        EPLDataflowOpEPStatementSource.MyFilter myFilter = new EPLDataflowOpEPStatementSource.MyFilter();
        options.parameterProvider(new DefaultSupportGraphParamProvider(
                Collections.<String, Object>singletonMap("statementFilter", myFilter)));
        options.operatorProvider(new DefaultSupportGraphOpProvider(capture));
        EPDataFlowInstance df = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        df.start();

        // 1. pre-existing SupportBean_B statement attached at open.
        runtime.getEventService().sendEventBean(new SupportBean_B("B1"), "SupportBean_B");
        capture.waitForInvocation(200, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "B1"));

        // 2. dynamically deployed unnamed statement attaches on deployment.
        deploy(runtime, compile(configuration, runtime, "select theString, intPrimitive from SupportBean"), "stmt-e");
        runtime.getEventService().sendEventBean(new SupportBean("E1", 1), "SupportBean");
        capture.waitForInvocation(200, 1);
        Object[] rows = capture.getCurrentAndReset();
        if (rows.length != 1 || !(rows[0] instanceof Map)) {
            throw new IllegalStateException("capture held " + rows.length + " rows, expected one wrapped map row");
        }
        Map<?, ?> projection = (Map<?, ?>) rows[0];
        Object theStringValue = projection.get("theString");
        Object intPrimitiveValue = projection.get("intPrimitive");
        if (!"E1".equals(theStringValue) || !Integer.valueOf(1).equals(intPrimitiveValue)) {
            throw new IllegalStateException("capture row was " + projection + ", expected {theString=E1, intPrimitive=1}");
        }
        writer.addCapture(new JsonArray().add(new JsonObject().add("kind", "row").add("fields",
                new JsonObject().add("intPrimitive", 1).add("theString", "E1"))));

        // 3. named s2 statement attaches on deployment.
        deploy(runtime, compile(configuration, runtime, "@name('s2') select id from SupportBean_A"), "s2-first");
        runtime.getEventService().sendEventBean(new SupportBean_A("A1"), "SupportBean_A");
        capture.waitForInvocation(200, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "A1"));

        // 4. undeploy detaches s2: A2 is dropped.
        undeployModuleContaining(runtime, "s2");
        runtime.getEventService().sendEventBean(new SupportBean_A("A2"), "SupportBean_A");
        Thread.sleep(50);
        assertCurrentEmpty(capture, "after s2 undeploy");
        writer.addCapture(new JsonArray());

        // 5. redeployed s2 reattaches.
        deploy(runtime, compile(configuration, runtime, "@name('s2') select id from SupportBean_A"), "s2-second");
        runtime.getEventService().sendEventBean(new SupportBean_A("A3"), "SupportBean_A");
        capture.waitForInvocation(200, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "A3"));

        // 6. the first B statement remains attached.
        runtime.getEventService().sendEventBean(new SupportBean_B("B2"), "SupportBean_B");
        capture.waitForInvocation(200, 1);
        writer.addCapture(singleIdRow(capture.getCurrentAndReset(), "B2"));

        // 7. cancel silences the source.
        df.cancel();
        runtime.getEventService().sendEventBean(new SupportBean("E1", 1), "SupportBean");
        runtime.getEventService().sendEventBean(new SupportBean_A("A1"), "SupportBean_A");
        runtime.getEventService().sendEventBean(new SupportBean_B("B3"), "SupportBean_B");
        assertCurrentEmpty(capture, "after cancel");
        writer.addCapture(new JsonArray());
    }

    /**
     * EPLDataflowInvalid: compile/instantiate rejections only, asserted as Java
     * message prefixes in-process; no trace rows (invalidity policy).
     */
    private static void runInvalid(Configuration configuration, EPRuntime runtime) throws Exception {
        // test no statement name or statement filter provided
        EPCompiled flow = compile(configuration, runtime, "@name('flow') " + INVALID_INSTANTIATE_FLOW);
        deploy(runtime, flow, FLOW_DEPLOYMENT_ID);
        try {
            runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, "DF1");
            throw new IllegalStateException("expected EPDataFlowInstantiationException for DF1");
        } catch (EPDataFlowInstantiationException ex) {
            assertPrefix(MSG_INSTANTIATE_MISSING_PARAMETER, ex.getMessage());
        } finally {
            runtime.getDeploymentService().undeployAll();
        }

        // invalid: no output stream
        try {
            compile(configuration, runtime, INVALID_NO_OUTPUT_STREAM);
            throw new IllegalStateException("expected EPCompileException for zero output stream flow");
        } catch (EPCompileException ex) {
            assertPrefix(MSG_NO_OUTPUT_STREAM, ex.getMessage());
        }

        // invalid: no statement deployment id
        try {
            compile(configuration, runtime, INVALID_PAIRING);
            throw new IllegalStateException("expected EPCompileException for unpaired statement parameters");
        } catch (EPCompileException ex) {
            assertPrefix(MSG_PAIRING, ex.getMessage());
        }
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
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "capture")
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
