import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.util.JsonEventObject;
import com.espertech.esper.common.client.render.JSONEventRenderer;
import com.espertech.esper.common.client.util.DateTime;
import com.espertech.esper.common.internal.support.EventRepresentationChoice;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.event.json.EventJsonAdapter.LocalEvent;
import com.espertech.esper.regressionlib.suite.event.json.EventJsonAdapter.MyDateJSONParser;
import com.espertech.esper.regressionlib.support.json.SupportJsonFieldAdapterStringDate;
import com.espertech.esper.regressionlib.support.json.SupportJsonFieldAdapterStringPoint;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.awt.Point;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Date;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for the EventJson adapter parity scenario.
 *
 * Covers the three registered executions of the event/json adapter
 * subdomain. Adapter-insert-into replays EventJsonAdapterInsertInto: one
 * compileDeploy carries the byte-exact five-line block
 * "@public @buseventtype create schema LocalEvent as ...LocalEvent;\n" plus
 * the two @JsonSchemaField annotations (mydate with
 * SupportJsonFieldAdapterStringDate, point with
 * SupportJsonFieldAdapterStringPoint) on the @EventRepresentation('json')
 * insert into JsonEvent from LocalEvent, plus s0 (select point,mydate) and
 * s1 (select *); one LocalEvent bean with Point(7,14) and the default-parsed
 * 2002-05-01T08:00:01.999 date is sent, the adapters write the JSON strings
 * "7,14" and "2002-05-01T08:00:01.999", s0 yields the adapter-written
 * strings and s1 yields the JsonEventObject underlying whose minimaljson
 * toString is byte-equal to the filled JSON. Adapter-create-schema replays
 * EventJsonAdapterCreateSchemaWStringTransform: the byte-exact create json
 * schema JsonEvent(point java.awt.Point, mydate Date) with the two adapters
 * plus s0/s1 deploys, the filled payload repeats the first case's rows and
 * the null payload {"point":null,"mydate":null} yields null-marker s0
 * fields and the byte-equal s1 toString. Adapter-doc-sample replays
 * EventJsonAdapterDocSample: the byte-exact create json schema
 * JsonEvent(myDate Date) with the MyDateJSONParser adapter (dd-MM-yyyy
 * parse/write) plus s0 deploys, the exact payload bytes
 * {"myDate" : "22-09-2018"} (one space around the colon) are sent, and the
 * runtime JSON renderer pins renderer.render("hello", event) to
 * {"hello":{"myDate":"22-09-2018"}}.
 *
 * Records follow the standard protocol and the trace is the standard
 * esper-parity/v1 wrapper object around the eight records: automatic s0/s1
 * listener records with case-local
 * sequences starting at one and incremented per emitted record, one render
 * record for the doc sample, time frozen at epoch zero because the internal
 * timer is disabled and time is advanced to zero only, per-case fresh
 * runtimes, and undeployAll at the end of each case.
 */
public class EventJsonAdapterScenarioOracle {
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 0) {
            System.err.println("usage: EventJsonAdapterScenarioOracle");
            System.exit(2);
        }
        List<JsonObject> records = new ArrayList<>();
        runCase("adapter-insert-into", records);
        runCase("adapter-create-schema-w-string-transform", records);
        runCase("adapter-doc-sample", records);
        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", "event-json-adapter");
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(String caseName, List<JsonObject> records) throws Exception {
        Configuration config = configure();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EventJsonAdapterScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            ListenerRecorder listener = new ListenerRecorder(caseName, runtime, records);
            deploy(config, runtime, caseName, listener);
            switch (caseName) {
                case "adapter-insert-into" -> {
                    runtime.getEventService().sendEventBean(
                        new LocalEvent(new Point(7, 14), DateTime.parseDefaultDate("2002-05-01T08:00:01.999")),
                        "LocalEvent");
                    assertS0InsertInto(listener);
                    checkS1Json(listener, "{\"point\":\"7,14\",\"mydate\":\"2002-05-01T08:00:01.999\"}");
                }
                case "adapter-create-schema-w-string-transform" -> {
                    runtime.getEventService().sendEventJson(
                        "{\"point\":\"7,14\",\"mydate\":\"2002-05-01T08:00:01.999\"}", "JsonEvent");
                    assertS0CreateSchemaFilled(listener);
                    checkS1Json(listener, "{\"point\":\"7,14\",\"mydate\":\"2002-05-01T08:00:01.999\"}");
                    runtime.getEventService().sendEventJson("{\"point\":null,\"mydate\":null}", "JsonEvent");
                    assertS0CreateSchemaNulled(listener);
                    checkS1Json(listener, "{\"point\":null,\"mydate\":null}");
                }
                case "adapter-doc-sample" -> {
                    runtime.getEventService().sendEventJson("{\"myDate\" : \"22-09-2018\"}", "JsonEvent");
                    Date date;
                    try {
                        date = new SimpleDateFormat("dd-M-yyyy").parse("22-09-2018");
                    } catch (ParseException e) {
                        throw new RuntimeException(e);
                    }
                    EventBean s0 = listener.lastEvent("s0");
                    check(date.equals(s0.get("myDate")), "s0 myDate must equal the parsed doc-sample date");
                    JSONEventRenderer renderer = runtime.getRenderEventService()
                        .getJSONRenderer(runtime.getEventTypeService().getBusEventType("JsonEvent"));
                    String rendered = renderer.render("hello", s0);
                    check("{\"hello\":{\"myDate\":\"22-09-2018\"}}".equals(rendered),
                        "renderer must output {\"hello\":{\"myDate\":\"22-09-2018\"}} but was " + rendered);
                    listener.emitRenderRecord(rendered);
                }
                default -> throw new IllegalStateException("unknown case " + caseName);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Mirrors the pinned runner defaults relevant here plus the
     * TestSuiteEventJson.configure imports: no preconfigured event types (the
     * schemas are created by the deployed EPL), internal timer off,
     * statement exceptions rethrow on the sending thread, and the two support
     * adapter classes imported so their simple names resolve inside
     * @JsonSchemaField exactly like the Java suite.
     */
    private static Configuration configure() {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        config.getCommon().getImports().add(SupportJsonFieldAdapterStringDate.class.getName());
        config.getCommon().getImports().add(SupportJsonFieldAdapterStringPoint.class.getName());
        return config;
    }

    /** Case-name to byte-exact pinned EPL from the Java execution bodies. */
    private static String eplFor(String caseName) {
        return switch (caseName) {
            case "adapter-insert-into" ->
                "@public @buseventtype create schema LocalEvent as " + LocalEvent.class.getName() + ";\n" +
                    "@JsonSchemaField(name=mydate, adapter=" +
                    SupportJsonFieldAdapterStringDate.class.getSimpleName() + ") " +
                    "@JsonSchemaField(name=point, adapter=" +
                    SupportJsonFieldAdapterStringPoint.class.getSimpleName() + ") " +
                    EventRepresentationChoice.JSON.getAnnotationText() +
                    " insert into JsonEvent select point, mydate from LocalEvent;\n" +
                    "@name('s0') select point, mydate from JsonEvent;\n" +
                    "@name('s1') select * from JsonEvent;\n";
            case "adapter-create-schema-w-string-transform" ->
                "@public @buseventtype " +
                    "@JsonSchemaField(name=point, adapter=" +
                    SupportJsonFieldAdapterStringPoint.class.getSimpleName() + ") " +
                    "@JsonSchemaField(name=mydate, adapter=" +
                    SupportJsonFieldAdapterStringDate.class.getSimpleName() + ") " +
                    "create json schema JsonEvent(point java.awt.Point, mydate Date);\n" +
                    "@name('s0') select point, mydate from JsonEvent;\n" +
                    "@name('s1') select * from JsonEvent;\n";
            case "adapter-doc-sample" ->
                "@public @buseventtype @JsonSchemaField(name=myDate, adapter='" +
                    MyDateJSONParser.class.getName() + "')\n" +
                    "create json schema JsonEvent(myDate Date);\n" +
                    "@name('s0') select * from JsonEvent;\n";
            default -> throw new IllegalStateException("case " + caseName + " deploys no statements");
        };
    }

    private static void deploy(Configuration config, EPRuntime runtime, String caseName,
                               ListenerRecorder listener) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler()
            .compile(eplFor(caseName), new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement added : deployment.getStatements()) {
            if ("s0".equals(added.getName()) || "s1".equals(added.getName())) {
                added.addListener(listener);
            }
        }
    }

    /**
     * Mirrors the EventJsonAdapterInsertInto s0 assertion: exactly one new
     * s0 event whose point is Point(7,14) and whose mydate equals the
     * default-parsed instant.
     */
    private static void assertS0InsertInto(ListenerRecorder listener) {
        EventBean s0 = listener.lastEvent("s0");
        Point point = (Point) s0.get("point");
        check(point != null && point.x == 7 && point.y == 14, "s0 point must be Point(7,14)");
        Date mydate = (Date) s0.get("mydate");
        check(mydate != null && mydate.equals(DateTime.parseDefaultDate("2002-05-1T08:00:01.999")),
            "s0 mydate must equal the default-parsed date");
    }

    /**
     * Mirrors the filled-send half of the EventJsonAdapterCreateSchemaWStringTransform
     * s0 assertion: the adapter-parsed Point(7,14) and default-parsed date.
     */
    private static void assertS0CreateSchemaFilled(ListenerRecorder listener) {
        EventBean s0 = listener.lastEvent("s0");
        Point point = (Point) s0.get("point");
        check(point != null && point.x == 7 && point.y == 14, "s0 point must be Point(7,14)");
        Date mydate = (Date) s0.get("mydate");
        check(mydate != null && mydate.equals(DateTime.parseDefaultDate("2002-05-1T08:00:01.999")),
            "s0 mydate must equal the default-parsed date");
    }

    /**
     * Mirrors the nulled-send half of the
     * EventJsonAdapterCreateSchemaWStringTransform s0 assertion: both
     * adapter properties arrive as null.
     */
    private static void assertS0CreateSchemaNulled(ListenerRecorder listener) {
        EventBean s0 = listener.lastEvent("s0");
        check(s0.get("point") == null, "s0 point must be null");
        check(s0.get("mydate") == null, "s0 mydate must be null");
    }

    /**
     * Mirrors the shared doAssert of the insert-into and create-schema
     * executions: the s1 select-star row must carry the JsonEventObject
     * underlying whose minimaljson toString is byte-equal to the JSON
     * payload that produced the event.
     */
    private static void checkS1Json(ListenerRecorder listener, String expectedJson) {
        JsonEventObject underlying = (JsonEventObject) listener.lastEvent("s1").getUnderlying();
        check(expectedJson.equals(underlying.toString()),
            "s1 underlying must render as " + expectedJson + " but was " + underlying);
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    /**
     * Pinned projection order per case, mirroring the select clauses: point
     * then mydate for the two JsonEvent cases, myDate for the doc sample.
     */
    private static List<String> selectedProperties(String caseName) {
        return switch (caseName) {
            case "adapter-insert-into", "adapter-create-schema-w-string-transform" -> List.of("point", "mydate");
            case "adapter-doc-sample" -> List.of("myDate");
            default -> throw new IllegalStateException("case " + caseName + " has no pinned property order");
        };
    }

    /**
     * Applies the adapter write logic to the received values exactly like the
     * JsonFieldAdapterString implementations: Point projects as "x,y"
     * (SupportJsonFieldAdapterStringPoint.write), Date projects through
     * DateTime.print (SupportJsonFieldAdapterStringDate.write) or through the
     * dd-MM-yyyy format for the MyDateJSONParser doc-sample case, and null
     * values use the standard null-marker object.
     */
    private static JsonValue normalize(String caseName, Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Point point) {
            return Json.value(point.x + "," + point.y);
        }
        if (value instanceof Date date) {
            return Json.value(writeDate(caseName, date));
        }
        if (value instanceof String string) {
            return Json.value(string);
        }
        return Json.value(String.valueOf(value));
    }

    private static String writeDate(String caseName, Date date) {
        return switch (caseName) {
            case "adapter-doc-sample" -> new SimpleDateFormat("dd-MM-yyyy").format(date);
            default -> DateTime.print(date);
        };
    }

    private static JsonObject newRow(JsonObject fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    /**
     * Automatic s0/s1 listener mirroring env.addListener("s0") and
     * env.addListener("s1"): every update emits one listener record per new
     * event whose sequence increments per case across both statements. The
     * s1 select-star rows project the JsonEventObject underlying toString as
     * the single "json" field; s0 rows project the pinned select-clause
     * properties through the adapter write logic. Also emits the doc-sample
     * render record after the main flow pins the renderer output.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private final Map<String, EventBean[]> lastNewByStatement = new HashMap<>();
        private long sequence;

        private ListenerRecorder(String caseName, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
        }

        private EventBean lastEvent(String statementName) {
            EventBean[] events = lastNewByStatement.get(statementName);
            check(events != null && events.length == 1,
                "expected exactly one new " + statementName + " event for case " + caseName +
                    ", got " + (events == null ? "none" : events.length));
            return events[0];
        }

        private void emitRenderRecord(String rendered) {
            JsonObject fields = new JsonObject();
            fields.add("json", rendered);
            records.add(newRecord("render", "s0", newRow(fields)));
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            String statementName = statement.getName();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    JsonObject fields = new JsonObject();
                    if ("s1".equals(statementName)) {
                        JsonEventObject underlying = (JsonEventObject) event.getUnderlying();
                        fields.add("json", underlying.toString());
                    } else {
                        for (String property : selectedProperties(caseName)) {
                            fields.add(property, normalize(caseName, event.get(property)));
                        }
                    }
                    records.add(newRecord("listener", statementName, newRow(fields)));
                }
                lastNewByStatement.put(statementName, newEvents);
            }
        }

        private JsonObject newRecord(String operation, String statementName, JsonObject row) {
            sequence++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", operation);
            record.add("statement", statementName);
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = new JsonArray();
            newArray.add(row);
            record.add("new", newArray);
            return record;
        }
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread instead of being
     * absorbed by the default handler.
     */
    public static class RethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return new ExceptionHandler() {
                @Override
                public void handle(ExceptionHandlerContext context) {
                    throw new RuntimeException("Unexpected exception in statement '" + context.getStatementName() +
                        "': " + context.getThrowable().getMessage(), context.getThrowable());
                }
            };
        }
    }
}
