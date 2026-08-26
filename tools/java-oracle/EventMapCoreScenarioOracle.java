import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.meta.EventTypeApplicationType;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Properties;
import java.util.TreeSet;

/**
 * Java oracle for the EventMapCore map-event parity scenario.
 *
 * Covers all five registered executions under the TestSuiteEventMap
 * preconfiguration: SupportBean as a class type, myMapEvent as a Properties
 * map {myInt:int, myString:string, beanA:SupportBeanComplexProps,
 * myStringArray:string[]} and MyMap as the nested map-of-maps
 * {lev0name:{lev1name:{sb:'SupportBean'}}}.
 *
 * Map-nested-event-type replays EventMapCoreMapNestedEventType: the MyMap
 * preconfigured type must resolve, @name('s0') selects
 * lev0name.lev1name.sb.theString through three map levels down into the
 * SupportBean leaf, one nested-map send yields val='E1', and the object-array
 * sender against the Map-typed name must fail at send time with the pinned
 * "...refers to a java.util.Map event type" message recorded as a send-error.
 * Metadata replays EventMapCoreMetadata: pure introspection of myMapEvent
 * asserting MAP application type, the name, exactly four property descriptors
 * (myInt Integer, myString String, beanA SupportBeanComplexProps fragment,
 * myStringArray String[] indexed with String componentType) with no events,
 * acknowledged by a single deployed marker record. Nested-objects replays
 * EventMapCoreNestedObjects navigating beanA.simpleProperty,
 * beanA.nested.nestedValue, beanA.indexed[1] and
 * beanA.nested.nestedNested.nestedNestedValue over myMapEvent#length(5).
 * Query-fields replays EventMapCoreQueryFields receiving the typed payload
 * then a raw extra-key-free HashMap against the same declared type.
 * Invalid-statement replays EventMapCoreInvalidStatement: three rejected
 * compilations whose full EPCompileException messages ride in build-error
 * records (the pinned expectation is 'skip', so only the failure category is
 * contractual, not the text).
 *
 * Sends rebuild the pinned payloads structurally from the scenario JSON: the
 * myMapEvent payload carries the makeDefaultBean values
 * (simpleProperty="simple", mappedProps{keyOne:valueOne,keyTwo:valueTwo},
 * indexedProps=[1,2], mapProperty{xOne:yOne,xTwo:yTwo},
 * arrayProperty=[10,20,30], nested.nestedValue="nestedValue",
 * nested.nestedNested.nestedNestedValue="nestedNestedValue"); object identity
 * of the static map is not observable because no execution mutates or compares
 * it. Records follow the standard protocol: automatic s0 listener records with
 * per-case sequences starting at one, deployed/build-error/send-error markers
 * at sequence zero, and time frozen at epoch zero because the internal timer
 * is disabled and time is advanced to zero only.
 */
public class EventMapCoreScenarioOracle {
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventMapCoreScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = configure();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EventMapCoreScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            // Pinned pre-execution assertions: MyMap must resolve for the
            // nested-type execution; metadata fully introspects myMapEvent.
            if ("map-nested-event-type".equals(caseName)) {
                check(runtime.getEventTypeService().getEventTypePreconfigured("MyMap") != null,
                    "preconfigured type MyMap not found");
            } else if ("metadata".equals(caseName)) {
                assertMetadata(runtime);
            }

            ListenerRecorder listener = new ListenerRecorder(caseName, runtime, records);
            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (op) {
                    case "deploy" -> deploy(config, runtime, caseName, step.getString("statement", ""), listener);
                    case "send" -> sendEvent(runtime, step);
                    case "send-error" -> sendError(runtime, caseName, step, records);
                    case "build-error" -> buildError(config, runtime, caseName, step, records);
                    case "deployed" -> deployedMarker(caseName, step, records);
                    default -> throw new IllegalStateException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /** TestSuiteEventMap.configure() restricted to the types this suite uses. */
    private static Configuration configure() {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);

        Properties myMapEvent = new Properties();
        myMapEvent.put("myInt", "int");
        myMapEvent.put("myString", "string");
        myMapEvent.put("beanA", SupportBeanComplexProps.class.getName());
        myMapEvent.put("myStringArray", "string[]");
        config.getCommon().addEventType("myMapEvent", myMapEvent);

        Map<String, Object> lev2def = new LinkedHashMap<>();
        lev2def.put("sb", "SupportBean");
        Map<String, Object> lev1def = new LinkedHashMap<>();
        lev1def.put("lev1name", lev2def);
        Map<String, Object> lev0def = new LinkedHashMap<>();
        lev0def.put("lev0name", lev1def);
        config.getCommon().addEventType("MyMap", lev0def);

        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // Mirror the pinned runner: statement exceptions rethrow on the
        // sending thread instead of being absorbed by the default handler.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        return config;
    }

    /** Statement-name to pinned-EPL mapping; every case deploys at most s0. */
    private static String eplFor(String caseName, String statementName) {
        if (!"s0".equals(statementName)) {
            throw new IllegalStateException("unknown statement " + statementName + " in case " + caseName);
        }
        return switch (caseName) {
            case "map-nested-event-type" ->
                "@name('s0') select lev0name.lev1name.sb.theString as val from MyMap";
            case "nested-objects" ->
                "@name('s0') select beanA.simpleProperty as simple," +
                    "beanA.nested.nestedValue as nested," +
                    "beanA.indexed[1] as indexed," +
                    "beanA.nested.nestedNested.nestedNestedValue as nestednested " +
                    "from myMapEvent#length(5)";
            case "query-fields" ->
                "@name('s0') select myInt as intVal, myString as stringVal from myMapEvent#length(5)";
            default -> throw new IllegalStateException("case " + caseName + " deploys no statements");
        };
    }

    private static void deploy(Configuration config, EPRuntime runtime, String caseName,
                               String statementName, UpdateListener listener) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler()
            .compile(eplFor(caseName, statementName), new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement added : deployment.getStatements()) {
            if ("s0".equals(added.getName())) {
                added.addListener(listener);
            }
        }
    }

    /**
     * Metadata replay of EventMapCoreMetadata: MAP application type, name and
     * exactly the four pinned descriptors including the fragment flag on
     * beanA and the componentType/indexed flags on myStringArray.
     */
    private static void assertMetadata(EPRuntime runtime) {
        EventType type = runtime.getEventTypeService().getEventTypePreconfigured("myMapEvent");
        check(type != null, "preconfigured type myMapEvent not found");
        check(type.getMetadata().getApplicationType() == EventTypeApplicationType.MAP,
            "expected MAP application type, got " + type.getMetadata().getApplicationType());
        check("myMapEvent".equals(type.getMetadata().getName()),
            "expected name myMapEvent, got " + type.getMetadata().getName());

        Map<String, EventPropertyDescriptor> byName = new LinkedHashMap<>();
        for (EventPropertyDescriptor desc : type.getPropertyDescriptors()) {
            check(!byName.containsKey(desc.getPropertyName()),
                "duplicate descriptor " + desc.getPropertyName());
            byName.put(desc.getPropertyName(), desc);
        }
        check(byName.size() == 4, "expected four descriptors, got " + byName.keySet());
        checkNames(byName.keySet(), "myInt", "myString", "beanA", "myStringArray");

        EventPropertyDescriptor myInt = byName.get("myInt");
        check(myInt.getPropertyType() == Integer.class, "myInt type " + myInt.getPropertyType());
        check(!myInt.isFragment() && !myInt.isIndexed() && !myInt.isMapped(),
            "myInt flags fragment=" + myInt.isFragment() + " indexed=" + myInt.isIndexed());

        EventPropertyDescriptor myString = byName.get("myString");
        check(myString.getPropertyType() == String.class, "myString type " + myString.getPropertyType());

        EventPropertyDescriptor beanA = byName.get("beanA");
        check(beanA.getPropertyType() == SupportBeanComplexProps.class,
            "beanA type " + beanA.getPropertyType());
        check(beanA.isFragment(), "beanA must be a fragment");

        EventPropertyDescriptor myStringArray = byName.get("myStringArray");
        check(myStringArray.getPropertyType() == String[].class,
            "myStringArray type " + myStringArray.getPropertyType());
        check(myStringArray.getPropertyComponentType() == String.class,
            "myStringArray componentType " + myStringArray.getPropertyComponentType());
        check(myStringArray.isIndexed(), "myStringArray must be indexed");
    }

    private static void checkNames(java.util.Set<String> actual, String... expected) {
        for (String name : expected) {
            check(actual.contains(name), "missing descriptor " + name);
        }
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    /** Dispatches scenario sends: typed map payloads and the nested MyMap tree. */
    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "myMapEvent" ->
                runtime.getEventService().sendEventMap(mapEventPayload(payload), "myMapEvent");
            case "MyMap" ->
                runtime.getEventService().sendEventMap(nestedMapPayload(payload), "MyMap");
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    /** Builds the myMapEvent payload; beanA reconstructs makeDefaultBean values. */
    private static Map<String, Object> mapEventPayload(JsonObject payload) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (String key : payload.names()) {
            if ("beanA".equals(key)) {
                map.put(key, complexProps(payload.get(key).asObject()));
            } else {
                map.put(key, scalar(payload.get(key)));
            }
        }
        return map;
    }

    /**
     * Rebuilds the three-level MyMap send: lev0name/lev1name stay Maps while
     * the sb leaf becomes the SupportBean("E1", 0) underlying.
     */
    private static Map<String, Object> nestedMapPayload(JsonObject lev0) {
        JsonObject lev1 = lev0.get("lev0name").asObject();
        JsonObject lev2 = lev1.get("lev1name").asObject();
        JsonObject sb = lev2.get("sb").asObject();
        SupportBean supportBean = new SupportBean(
            stringOrNull(sb.get("theString")), sb.getInt("intPrimitive", 0));
        Map<String, Object> lev2map = new LinkedHashMap<>();
        lev2map.put("sb", supportBean);
        Map<String, Object> lev1map = new LinkedHashMap<>();
        lev1map.put("lev1name", lev2map);
        Map<String, Object> lev0map = new LinkedHashMap<>();
        lev0map.put("lev0name", lev1map);
        return lev0map;
    }

    /** SupportBeanComplexProps from the pinned payload shape. */
    private static SupportBeanComplexProps complexProps(JsonObject bean) {
        Properties mappedProps = new Properties();
        JsonObject mapped = bean.get("mappedProps").asObject();
        for (String key : mapped.names()) {
            mappedProps.put(key, mapped.get(key).asString());
        }
        Map<String, String> mapProperty = new LinkedHashMap<>();
        JsonObject mapValues = bean.get("mapProperty").asObject();
        for (String key : mapValues.names()) {
            mapProperty.put(key, mapValues.get(key).asString());
        }
        JsonObject nested = bean.get("nested").asObject();
        return new SupportBeanComplexProps(
            bean.getString("simpleProperty", null),
            mappedProps,
            intArray(bean.get("indexedProps")),
            mapProperty,
            intArray(bean.get("arrayProperty")),
            nested.getString("nestedValue", null),
            nested.get("nestedNested").asObject().getString("nestedNestedValue", null));
    }

    private static int[] intArray(JsonValue value) {
        JsonArray array = value.asArray();
        int[] out = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            out[i] = array.get(i).asInt();
        }
        return out;
    }

    private static Object scalar(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value instanceof JsonNumber number) {
            return number.asInt();
        }
        if (value instanceof JsonString string) {
            return string.asString();
        }
        return String.valueOf(value);
    }

    private static String stringOrNull(JsonValue value) {
        return value == null || value.isNull() ? null
            : ((JsonString) value).asString();
    }

    /**
     * Sends an event expected to fail; records the exact root-cause message
     * or the "&lt;no-error&gt;" drift marker. Sender defaults to the map
     * sender; "object-array" drives sendEventObjectArray against the Map type.
     */
    private static void sendError(EPRuntime runtime, String caseName, JsonObject step, List<JsonObject> records) {
        String eventType = step.getString("eventType", "");
        String sender = step.getString("sender", "map");
        JsonObject errorRecord = new JsonObject();
        errorRecord.add("case", caseName);
        errorRecord.add("operation", "send-error");
        errorRecord.add("statement", eventType);
        errorRecord.add("sequence", 0);
        try {
            if ("object-array".equals(sender)) {
                runtime.getEventService().sendEventObjectArray(objectArray(step.get("payload")), eventType);
            } else if ("map".equals(sender)) {
                sendEvent(runtime, step);
            } else {
                throw new IllegalStateException("unknown sender " + sender);
            }
            errorRecord.add("value", "<no-error>");
        } catch (RuntimeException ex) {
            errorRecord.add("value", rootCauseMessage(ex));
        }
        records.add(errorRecord);
    }

    private static Object[] objectArray(JsonValue value) {
        JsonArray array = value.asArray();
        Object[] out = new Object[array.size()];
        for (int i = 0; i < array.size(); i++) {
            out[i] = scalar(array.get(i));
        }
        return out;
    }

    /** Deepest non-null cause message, the canonical cross-runtime error text. */
    private static String rootCauseMessage(Throwable throwable) {
        Throwable current = throwable;
        while (true) {
            Throwable cause = current.getCause();
            if (cause == null || cause == current) {
                break;
            }
            current = cause;
        }
        return current.getMessage();
    }

    /**
     * Compiles the pinned EPL expected to fail and records the full
     * EPCompileException message, or the "&lt;no-error&gt;" drift marker.
     */
    private static void buildError(Configuration config, EPRuntime runtime, String caseName,
                                   JsonObject step, List<JsonObject> records) {
        String message;
        try {
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), new CompilerArguments(config));
            message = "<no-error>";
        } catch (EPCompileException ex) {
            message = ex.getMessage();
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "build-error");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("value", message);
        records.add(record);
    }

    /** Deployed marker: only the metadata case (which deploys nothing) emits it. */
    private static void deployedMarker(String caseName, JsonObject step, List<JsonObject> records) {
        if (!"metadata".equals(caseName)) {
            throw new IllegalStateException("unexpected deployed op for case " + caseName);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "deployed");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", 0);
        records.add(record);
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof Collection<?>) {
            JsonArray items = new JsonArray();
            for (Object element : (Collection<?>) value) {
                items.add(normalize(element));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per case. This
     * suite has no remove-streams, so only the new stream is recorded.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private long sequence;

        private ListenerRecorder(String caseName, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", ++sequence);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = new JsonArray();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    newArray.add(renderRow(event));
                }
            }
            record.add("new", newArray);
            records.add(record);
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
