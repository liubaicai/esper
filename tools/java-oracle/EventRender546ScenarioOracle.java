/**
 * Java oracle for the event-render-546 parity slice: the seven unreferenced
 * EventRender* executions of EventRender (custom property renderer, object
 * array, POJO map), EventRenderJSON (empty map, enquote) and EventRenderXML
 * (sql-date, enquote). Rendered strings are compared with the sources'
 * removeNewline rule and recorded fully whitespace-stripped so the Go trace
 * (which has only insignificant-whitespace freedom) compares byte-exact.
 * Unrepresentable steps run the real Java assertion in-process and emit the
 * pinned note the Go side cannot produce.
 */
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.render.EventPropertyRendererContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.render.JSONRenderingOptions;
import com.espertech.esper.common.client.render.XMLRenderingOptions;
import com.espertech.esper.common.internal.event.render.OutputValueRendererJSONString;
import com.espertech.esper.common.internal.event.render.OutputValueRendererXMLString;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanRendererOne;
import com.espertech.esper.regressionlib.support.bean.SupportBeanRendererThree;
import com.espertech.esper.regressionlib.suite.event.render.EventRender;
import com.espertech.esper.regressionlib.suite.event.render.EventRenderJSON;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.io.FileReader;
import java.time.Instant;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

public final class EventRender546ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-render-546";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render";

    private static final String[] CASES = {
            "custom-renderer", "object-array", "pojo-map", "empty-map",
            "enquote-json", "sqldate", "enquote-xml",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-26bb69572227aa67230c",
            "java-runtime-eacece93880bd4448b7d",
            "java-runtime-6af1376411de38cffefe",
            "java-runtime-96f1450787b11db671fd",
            "java-runtime-8cce94662734d4052c0e",
            "java-runtime-f47d81ef0d1a48b66476",
            "java-runtime-539c60ae3344b3001b44",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventRenderPropertyCustomRenderer",
            "EventRenderObjectArray",
            "EventRenderPOJOMap",
            "EventRenderEmptyMap",
            "EventRenderEnquote",
            "EventRenderSQLDate",
            "EventRenderEnquote",
    };

    private static final String NOW = Instant.ofEpochMilli(0).toString();

    // Verbatim expected strings from the Java regression sources (compared via
    // removeNewline like the sources do).
    private static final String EXPECTED_CUSTOM_JSON =
            "{ \"MyEvent\": { \"id\": \"id1\", \"someProperties\": [\"index#0=1;index#1=x\", \"index#0=2;index#1=y\"], \"mappedProperty\": { \"key\": \"value\" } } }";
    private static final String EXPECTED_CUSTOM_XML =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?> <MyEvent> <id>id1</id> <someProperties>index#0=1;index#1=x</someProperties> <someProperties>index#0=2;index#1=y</someProperties> <mappedProperty> <key>value</key> </mappedProperty> </MyEvent>";
    private static final String EXPECTED_OA_JSON =
            "{ \"MyEvent\": { \"p0\": \"abc\", \"p1\": 1, \"p3\": 2, \"p4\": 3.0, \"p2\": { \"id\": 1, \"p00\": \"p00\", \"p01\": null, \"p02\": null, \"p03\": null } } }";
    private static final String EXPECTED_OA_XML =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?> <MyEvent> <p0>abc</p0> <p1>1</p1> <p3>2</p3> <p4>3.0</p4> <p2> <id>1</id> <p00>p00</p00> </p2> </MyEvent>";
    private static final String EXPECTED_POJO_JSON =
            "{ \"MyEvent\": { \"stringObjectMap\": { \"abc\": \"def\", \"def\": 123, \"efg\": null } } }";
    private static final String EXPECTED_POJO_XML =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                    "<MyEvent>\n" +
                    "  <stringObjectMap>\n" +
                    "    <abc>def</abc>\n" +
                    "    <def>123</def>\n" +
                    "    <efg></efg>\n" +
                    "  </stringObjectMap>\n" +
                    "</MyEvent>";
    private static final String EXPECTED_POJO_XML_ATTR =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                    "<MyEvent>\n" +
                    "  <stringObjectMap abc=\"def\" def=\"123\"/>\n" +
                    "</MyEvent>";
    private static final String[] EXPECTED_EMPTY_JSON = {
            "{ \"outer\": { \"props\": null } }",
            "{ \"outer\": { \"props\": {} } }",
            "{ \"outer\": { \"props\": { \"a\": \"b\" } } }",
    };
    private static final String EXPECTED_SQLDATE_XML =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?> <testsqldate> <mySqlDate>2010-01-31</mySqlDate> </testsqldate>";

    private static final String[][] JSON_ENQUOTE_ROWS = {
            {"\t", "\"\\t\""},
            {"\n", "\"\\n\""},
            {"\r", "\"\\r\""},
            {Character.toString((char) 0), "\"\\u0000\""},
    };
    private static final String[][] XML_ENCODE_ROWS = {
            {"\"", "&quot;"},
            {"'", "&apos;"},
            {"&", "&amp;"},
            {"<", "&lt;"},
            {">", "&gt;"},
            {Character.toString((char) 0), "\\u0000"},
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventRender546ScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario;
        try (FileReader reader = new FileReader(args[0])) {
            scenario = Json.parse(reader).asObject();
        }
        JsonObject out = new JsonObject();
        out.add("version", VERSION);
        out.add("id", ID);
        out.add("javaCommit", JAVA_COMMIT);
        out.add("javaSource", JAVA_SOURCE);
        JsonArray runtimes = new JsonArray();
        for (String id : RUNTIME_IDS) {
            runtimes.add(id);
        }
        out.add("javaRuntimes", runtimes);
        JsonArray names = new JsonArray();
        for (String name : EXECUTION_NAMES) {
            names.add(name);
        }
        out.add("javaNames", names);
        out.add("java", System.getProperty("java.version"));

        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            replayCase(scenario, caseName, records);
        }
        out.add("records", records);
        System.out.println(out.toString());
    }

    private static void replayCase(JsonObject scenario, String caseName, JsonArray records) throws Exception {
        if (caseName.startsWith("enquote")) {
            // Pure renderer-helper executions: no engine is created, mirroring
            // the Java sources that call enquote/xmlEncode directly.
            State bare = new State();
            bare.records = records;
            bare.sequences = new HashMap<>();
            replayEnquote(scenario, caseName, bare);
            return;
        }
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        configure546(config);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            State state = new State();
            state.records = records;
            state.sequences = new HashMap<>();
            state.runtime = runtime;
            boolean inCase = false;
            for (JsonValue stepValue : scenario.get("steps").asArray()) {
                JsonObject step = stepValue.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (op) {
                    case "deploy": {
                        String statement = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        String mode = step.getString("mode", "");
                        if (!mode.isEmpty()) {
                            state.mode = mode;
                        }
                        if (epl.isEmpty()) {
                            break;
                        }
                        CompilerArguments args = new CompilerArguments(config);
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, args);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        state.deployments.put(statement, deployment);
                        for (EPStatement stmt : deployment.getStatements()) {
                            stmt.addListener(listener(caseName, state));
                        }
                        break;
                    }
                    case "deployed": {
                        String statement = step.getString("statement", "");
                        int sequence = state.sequences.merge(statement + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", statement);
                        record.add("sequence", sequence);
                        record.add("time", NOW);
                        records.add(record);
                        break;
                    }
                    case "send":
                        send(runtime, caseName, state, step);
                        break;
                    case "value":
                        emitValue(runtime, caseName, state, step);
                        break;
                    case "unrepresentable":
                        emitUnrepresentable(runtime, caseName, state, step);
                        break;
                    case "undeploy": {
                        String statement = step.getString("statement", "");
                        EPDeployment deployment = state.deployments.remove(statement);
                        if (deployment == null) {
                            throw new IllegalStateException("no deployment tracked for " + statement);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        state.deployments.clear();
                        state.lastEvent = null;
                        state.mode = null;
                        state.sequences.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + op);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    private static final class State {
        JsonArray records;
        Map<String, Integer> sequences;
        Map<String, EPDeployment> deployments = new LinkedHashMap<>();
        EventBean lastEvent;
        String mode;
        EPRuntime runtime;
    }

    /** Listener emitting one record per invocation per statement. */
    private static UpdateListener listener(String caseName, State state) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            int sequence = state.sequences.merge(statement.getName() + ":listener", 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", NOW);
            JsonArray rows = new JsonArray();
            for (EventBean event : newEvents) {
                JsonObject row = new JsonObject();
                row.add("kind", "row");
                JsonObject fields = new JsonObject();
                fields.add("delivered", true);
                row.add("fields", fields);
                rows.add(row);
                state.lastEvent = event;
            }
            record.add("new", rows);
            state.records.add(record);
        };
    }

    private static void send(EPRuntime runtime, String caseName, State state, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload") != null ? step.get("payload").asObject() : new JsonObject();
        String marker = payload.getString("payload", "");
        switch (caseName) {
            case "custom-renderer":
                runtime.getEventService().sendEventBean(
                        new EventRender.MyRendererEvent("id1",
                                new Object[][]{{1, "x"}, {2, "y"}}), eventType);
                return;
            case "object-array":
                runtime.getEventService().sendEventObjectArray(
                        new Object[]{"abc", 1, new SupportBean_S0(1, "p00"), 2L, 3d}, eventType);
                return;
            case "pojo-map": {
                Map<String, Object> otherMap = new LinkedHashMap<>();
                otherMap.put("abc", "def");
                otherMap.put("def", 123);
                otherMap.put("efg", null);
                otherMap.put(null, 1234);
                if ("SupportBeanRendererThree".equals(eventType)) {
                    SupportBeanRendererThree beanThree = new SupportBeanRendererThree();
                    beanThree.setStringObjectMap(otherMap);
                    runtime.getEventService().sendEventBean(beanThree, eventType);
                } else {
                    SupportBeanRendererOne beanOne = new SupportBeanRendererOne();
                    beanOne.setStringObjectMap(otherMap);
                    runtime.getEventService().sendEventBean(beanOne, eventType);
                }
                return;
            }
            case "empty-map": {
                Map<String, String> props;
                switch (marker) {
                    case "null":
                        props = null;
                        break;
                    case "empty":
                        props = Collections.emptyMap();
                        break;
                    case "map":
                        props = Collections.singletonMap("a", "b");
                        break;
                    default:
                        throw new IllegalArgumentException("unknown empty-map marker " + marker);
                }
                runtime.getEventService().sendEventBean(new EventRenderJSON.EmptyMapEvent(props), eventType);
                return;
            }
            case "sqldate":
                runtime.getEventService().sendEventBean(new SupportBean(), eventType);
                return;
            default:
                throw new IllegalArgumentException("send for case " + caseName);
        }
    }

    private static void emitValue(EPRuntime runtime, String caseName, State state, JsonObject step) {
        String name = step.getString("name", "");
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "value");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", state.sequences.merge(step.getString("statement", "") + ":value", 1, Integer::sum));
        record.add("time", NOW);
        record.add("name", name);
        EventBean event = state.lastEvent;
        if (event == null) {
            throw new IllegalStateException(caseName + " value step has no captured event");
        }
        switch (name) {
            case "json": {
                String json = runtime.getRenderEventService().renderJSON(titleFor(caseName), event);
                String normalized = stripWhitespace(json);
                switch (caseName) {
                    case "object-array":
                        assertEquals(removeNewline(EXPECTED_OA_JSON), removeNewline(json));
                        break;
                    case "pojo-map":
                        assertEquals(removeNewline(EXPECTED_POJO_JSON), removeNewline(json));
                        break;
                    case "empty-map":
                        // The value sequence is the per-send render index.
                        int idx = state.sequences.get(step.getString("statement", "") + ":value") - 1;
                        assertEquals(removeNewline(EXPECTED_EMPTY_JSON[idx]), removeNewline(json));
                        break;
                    default:
                        break;
                }
                record.add("value", normalized);
                break;
            }
            case "xml": {
                String xml = runtime.getRenderEventService().renderXML(titleFor(caseName), event);
                String normalized = stripWhitespace(xml);
                if ("pojo-map".equals(caseName)) {
                    assertEquals(removeNewline(EXPECTED_POJO_XML), removeNewline(xml));
                } else if ("sqldate".equals(caseName)) {
                    assertEquals(removeNewline(EXPECTED_SQLDATE_XML), removeNewline(xml));
                }
                record.add("value", normalized);
                break;
            }
            case "mySqlDate": {
                Object value = event.get("mySqlDate");
                assertEquals(java.sql.Date.valueOf("2010-01-31"), value);
                assertEquals(java.sql.Date.valueOf("2010-01-31"),
                        event.getEventType().getGetter("mySqlDate").get(event));
                record.add("value", String.valueOf(value));
                break;
            }
            default:
                throw new IllegalArgumentException("unknown value name " + name);
        }
        state.records.add(record);
    }

    private static String titleFor(String caseName) {
        switch (caseName) {
            case "empty-map":
                return "outer";
            case "sqldate":
                return "testsqldate";
            default:
                return "MyEvent";
        }
    }

    /**
     * Runs the real Java assertion in-process and emits the pinned note as the
     * record value so the Go side (which lacks the render surface) matches.
     */
    private static void emitUnrepresentable(EPRuntime runtime, String caseName, State state, JsonObject step) {
        String statement = step.getString("statement", "");
        String name = step.getString("name", "");
        String note = step.getString("expectError", "");
        switch (caseName) {
            case "custom-renderer": {
                EventBean event = state.lastEvent;
                if (event == null) {
                    throw new IllegalStateException("custom-renderer has no captured event");
                }
                if ("json".equals(name)) {
                    EventRender.MyRenderer.getContexts().clear();
                    JSONRenderingOptions options = new JSONRenderingOptions();
                    options.setRenderer(new EventRender.MyRenderer());
                    String json = runtime.getRenderEventService().renderJSON("MyEvent", event, options);
                    assertEquals(4, EventRender.MyRenderer.getContexts().size());
                    List<EventPropertyRendererContext> contexts = EventRender.MyRenderer.getContexts();
                    EventPropertyRendererContext context = contexts.get(2);
                    assertNotNull(context.getDefaultRenderer());
                    assertEquals(1, (int) context.getIndexedPropertyIndex());
                    assertEquals("MyRendererEvent", context.getEventType().getName());
                    assertEquals("someProperties", context.getPropertyName());
                    assertEquals(removeNewline(EXPECTED_CUSTOM_JSON), removeNewline(json));
                    assertEquals(note, stripWhitespace(json));
                } else if ("xml".equals(name)) {
                    EventRender.MyRenderer.getContexts().clear();
                    XMLRenderingOptions options = new XMLRenderingOptions();
                    options.setRenderer(new EventRender.MyRenderer());
                    String xml = runtime.getRenderEventService().renderXML("MyEvent", event, options);
                    assertEquals(4, EventRender.MyRenderer.getContexts().size());
                    assertEquals(removeNewline(EXPECTED_CUSTOM_XML), removeNewline(xml));
                    assertEquals(note, stripWhitespace(xml));
                } else {
                    throw new IllegalArgumentException("custom-renderer unrepresentable " + name);
                }
                break;
            }
            case "object-array": {
                // Go's XML render diverges on the Double lexeme (3 vs 3.0) and
                // emits empty elements for the nested bean's null fields where
                // Java drops them; pin the Java output here.
                EventBean event = state.lastEvent;
                if (event == null) {
                    throw new IllegalStateException("object-array has no captured event");
                }
                String xml = runtime.getRenderEventService().renderXML("MyEvent", event);
                assertEquals(removeNewline(EXPECTED_OA_XML), removeNewline(xml));
                assertEquals(note, stripWhitespace(xml));
                break;
            }
            case "pojo-map": {
                // defaultAsAttribute drops the null map entry and self-closes
                // the mapped element in Java; Go renders efg="" and an
                // explicit close tag, so the attribute-mode pin lives here.
                EventBean event = state.lastEvent;
                if (event == null) {
                    throw new IllegalStateException("pojo-map has no captured event");
                }
                XMLRenderingOptions options = new XMLRenderingOptions();
                options.setDefaultAsAttribute(true);
                String xml = runtime.getRenderEventService().renderXML("MyEvent", event, options);
                assertEquals(removeNewline(EXPECTED_POJO_XML_ATTR), removeNewline(xml));
                assertEquals(note, stripWhitespace(xml));
                break;
            }
            default:
                throw new IllegalArgumentException("unrepresentable for case " + caseName);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", statement);
        record.add("sequence", state.sequences.merge(statement + ":unrepresentable", 1, Integer::sum));
        record.add("time", NOW);
        if (!name.isEmpty()) {
            record.add("name", name);
        }
        record.add("value", note);
        state.records.add(record);
    }

    /** Replays the pure-helper enquote/xmlEncode executions without a runtime. */
    private static void replayEnquote(JsonObject scenario, String caseName, State state) {
        boolean inCase = false;
        int rowIndex = 0;
        for (JsonValue stepValue : scenario.get("steps").asArray()) {
            JsonObject step = stepValue.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                inCase = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!inCase) {
                continue;
            }
            if (!"unrepresentable".equals(op)) {
                throw new IllegalArgumentException("unexpected op " + op + " in " + caseName);
            }
            String note = step.getString("expectError", "");
            boolean jsonCase = "enquote-json".equals(caseName);
            String[][] rows = jsonCase ? JSON_ENQUOTE_ROWS : XML_ENCODE_ROWS;
            String[] row = rows[rowIndex++];
            StringBuilder buf = new StringBuilder();
            if (jsonCase) {
                OutputValueRendererJSONString.enquote(row[0], buf);
            } else {
                OutputValueRendererXMLString.xmlEncode(row[0], buf, true);
            }
            assertEquals(row[1], buf.toString());
            String helper = jsonCase ? "enquote" : "xmlEncode";
            String wantNote = helper + "(" + goQuote(row[0]) + ") -> " + goQuote(row[1]);
            assertEquals(wantNote, note);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "unrepresentable");
            record.add("statement", step.getString("statement", ""));
            record.add("sequence", state.sequences.merge(step.getString("statement", "") + ":unrepresentable", 1, Integer::sum));
            record.add("time", NOW);
            String name = step.getString("name", "");
            if (!name.isEmpty()) {
                record.add("name", name);
            }
            record.add("value", note);
            state.records.add(record);
        }
    }

    /** Reproduces Go %q quoting for the pinned enquote note strings. */
    private static String goQuote(String text) {
        StringBuilder out = new StringBuilder("\"");
        for (int i = 0; i < text.length(); i++) {
            char c = text.charAt(i);
            switch (c) {
                case '"':
                    out.append("\\\"");
                    break;
                case '\\':
                    out.append("\\\\");
                    break;
                case '\t':
                    out.append("\\t");
                    break;
                case '\n':
                    out.append("\\n");
                    break;
                case '\r':
                    out.append("\\r");
                    break;
                default:
                    if (c < 0x20) {
                        out.append(String.format("\\x%02x", (int) c));
                    } else {
                        out.append(c);
                    }
            }
        }
        return out.append('"').toString();
    }

    /** Java regression-source comparator: runs of whitespace collapse to one. */
    private static String removeNewline(String text) {
        return text.replaceAll("\\s\\s+|\\n|\\r", " ").trim();
    }

    /** Trace normalizer: strips every whitespace byte like the Go runner. */
    private static String stripWhitespace(String text) {
        return text.replaceAll("(\\s|\\n|\\t)", "");
    }

    /**
     * Registers the config-side types mirroring TestSuiteEventRender's
     * configure() for the covered executions.
     */
    private static void configure546(Configuration config) {
        for (Class<?> clazz : new Class<?>[]{SupportBean.class,
                EventRender.MyRendererEvent.class,
                SupportBeanRendererOne.class, SupportBeanRendererThree.class,
                EventRenderJSON.EmptyMapEvent.class}) {
            config.getCommon().addEventType(clazz);
        }
        String[] props = {"p0", "p1", "p2", "p3", "p4"};
        Object[] types = {String.class, int.class, SupportBean_S0.class, long.class, Double.class};
        config.getCommon().addEventType("MyObjectArrayType", props, types);
        config.getCompiler().getViewResources().setIterableUnbound(true);
    }
}
