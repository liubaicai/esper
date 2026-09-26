import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.scopetest.EPAssertionUtil;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanCombinedProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the four EventMapNested executions (ordinals
 * 0-3, all SERDEREQUIRED), replayed as one differential chain under the
 * TestSuiteEventMap preconfigured NestedMap type:
 *
 * insert-into (ord 0, EventMapNestedInsertInto): the @public
 * "insert into MyStream select map.mapOne as val1 from NestedMap#length(5)"
 * statement feeds the "select val1 as a from MyStream" consumer; deployments
 * accumulate into the compiler path exactly like RegressionPath so MyStream
 * resolves for the second module. One full-payload send asserts a == the
 * level-two map fragment.
 *
 * event-type (ord 1, EventMapNestedEventType): compiles
 * "select * from NestedMap" and asserts the statement event type — declared
 * property names {simple,object,nodefmap,map} in any order, types
 * String/Map/Map/SupportBean_A, and getPropertyType("map.mapOne.simpleOne")
 * returning null because Esper resolves declared root names only — then
 * emits the deployed marker.
 *
 * nested-pojo (ord 2, EventMapNestedNestedPojo): the verbatim 22-column
 * projection over NestedMap#length(5) — '?' optional paths, indexed[1],
 * nested bean chains and the mapped('1ma') accessor — replayed against
 * getTestData() then getTestDataThree() (L1 drops nodefmapOne with null
 * simpleOne/objectOne, L2 drops simpleTwo, L3 drops objectThree). The
 * simpleThree long survives via the {"_long":N} payload marker.
 *
 * is-exists (ord 3, EventMapNestedIsExists): seven exists() columns over
 * '?'-paths for both payloads ({t,f,t,t,t,t,t} then {t,f,f,t,t,t,f});
 * exists() reports false only for missing values, never for present-null.
 *
 * Send payloads rebuild the pinned maps structurally from the scenario JSON:
 * {"_bean":...} constructs the pinned bean underlyings and {"_long":N} pins
 * a Java Long, mirroring the objectarray-nested oracle. Records follow the
 * standard protocol: s0 listener records with per-case sequences starting
 * at one, the deployed marker at sequence one, and time frozen at epoch
 * zero because the internal timer is disabled.
 */
public final class EventMapNested547ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "event-map-nested-547";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapNested.java";

    private static final String DESCRIPTION =
            "EventMapNested nested-map event slice (all 4 executions, SERDEREQUIRED): insert-into routes map.mapOne as val1 through the public MyStream into the s0 consumer over NestedMap#length(5); event-type introspects the deployed select-* NestedMap statement (declared property names, String/Map/Map/SupportBean_A types, null dotted-property type); nested-pojo projects the verbatim 22-column graph incl. '?' optional paths, indexed[1], nested bean chains and the mapped('1ma') accessor over the full then partial payload; is-exists checks seven exists() columns over '?'-paths for both payloads.";

    private static final String[] CASES = {
            "insert-into", "event-type", "nested-pojo", "is-exists"};
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7eef27d1a8de16f61ff7",
            "java-runtime-17f60eacc6ec6d23d967",
            "java-runtime-cf3abd23bf728441a001",
            "java-runtime-68a5f852752eba589f5b"
    };
    private static final String[] EXECUTIONS = {
            "EventMapNestedInsertInto",
            "EventMapNestedEventType",
            "EventMapNestedNestedPojo",
            "EventMapNestedIsExists"
    };
    private static final String[] STATIC_IDS = {
            "java-26557b255ea0dcbd9d2a",
            "java-26557b255ea0dcbd9d2a",
            "java-26557b255ea0dcbd9d2a",
            "java-26557b255ea0dcbd9d2a"
    };
    private static final String[] OBSERVATIONS = {
            "listener", "deployed", "listener", "listener"};

    // Verbatim transcriptions of the pinned statement texts, keeping the
    // missing space after "as c2," and the double space before "from" in
    // the exists statement exactly as the Java concatenations produce them.
    private static final String EPL_INSERT =
            "@public insert into MyStream select map.mapOne as val1 from NestedMap#length(5)";
    private static final String EPL_CONSUMER =
            "@name('s0') select val1 as a from MyStream";
    private static final String EPL_EVENT_TYPE =
            "@name('s0') select * from NestedMap";
    private static final String EPL_NESTED_POJO =
            "@name('s0') select " +
            "simple, object, nodefmap, map, " +
            "object.id as a1, nodefmap.key1? as a2, nodefmap.key2? as a3, nodefmap.key3?.key4 as a4, " +
            "map.objectOne as b1, map.simpleOne as b2, map.nodefmapOne.key2? as b3, map.mapOne.simpleTwo? as b4, " +
            "map.objectOne.indexed[1] as c1, map.objectOne.nested.nestedValue as c2," +
            "map.mapOne.simpleTwo as d1, map.mapOne.objectTwo as d2, map.mapOne.nodefmapTwo as d3, " +
            "map.mapOne.mapTwo as e1, map.mapOne.mapTwo.simpleThree as e2, map.mapOne.mapTwo.objectThree as e3, " +
            "map.mapOne.objectTwo.array[1].mapped('1ma').value as f1, map.mapOne.mapTwo.objectThree.id as f2" +
            " from NestedMap#length(5)";
    private static final String EPL_IS_EXISTS =
            "@name('s0') select " +
            "exists(map.mapOne?) as a," +
            "exists(map.mapOne?.simpleOne) as b," +
            "exists(map.mapOne?.simpleTwo) as c," +
            "exists(map.mapOne?.mapTwo) as d," +
            "exists(map.mapOne.mapTwo?) as e," +
            "exists(map.mapOne.mapTwo.simpleThree?) as f," +
            "exists(map.mapOne.mapTwo.objectThree?) as g " +
            " from NestedMap#length(5)";

    private static final String[][] DEPLOY_EPLS = {
            {EPL_INSERT, EPL_CONSUMER},
            {EPL_EVENT_TYPE},
            {EPL_NESTED_POJO},
            {EPL_IS_EXISTS}
    };
    private static final String[] CASE_EPLS = {
            EPL_INSERT + "\n" + EPL_CONSUMER,
            EPL_EVENT_TYPE,
            EPL_NESTED_POJO,
            EPL_IS_EXISTS
    };

    private static final String[][] STEP_OPS = {
            {"deploy", "deploy", "send", "undeploy-all"},
            {"deploy", "deployed", "undeploy-all"},
            {"deploy", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "undeploy-all"}
    };
    private static final String[][] STEP_STATEMENTS = {
            {"insert-into", "s0"},
            {"s0"},
            {"s0"},
            {"s0"}
    };
    private static final int[][] STEP_SENDS = {
            {1},
            {},
            {1, 1},
            {1, 1}
    };

    private static final String NOW = Instant.ofEpochMilli(0).toString();
    private static final int EXPECTED_STEPS = 19;
    private static final int EXPECTED_RECORDS = 6;

    private EventMapNested547ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EventMapNested547ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        registerEventTypes(configuration);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            Map<String, EPStatement> statements = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
            boolean active = false;
            int deployIndex = 0;
            int sendIndex = 0;
            for (JsonValue stepValue : steps) {
                JsonObject step = stepValue.asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (operation) {
                    case "deploy" -> {
                        if (deployIndex >= DEPLOY_EPLS[caseIndex].length) {
                            throw new IllegalStateException("unexpected deploy index " + deployIndex
                                    + " for case " + caseName);
                        }
                        String epl = DEPLOY_EPLS[caseIndex][deployIndex];
                        if (!epl.equals(step.getString("epl", ""))) {
                            throw new IllegalArgumentException("deploy epl is not pinned for case "
                                    + caseName + " deploy " + deployIndex);
                        }
                        deployIndex++;
                        // RegressionPath: earlier compiled modules join the
                        // path so @public MyStream resolves in the consumer.
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        for (EPCompiled deployed : deployedModules) {
                            compilerArgs.getPath().add(deployed);
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions());
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
                            if ("s0".equals(statement.getName()) && !"event-type".equals(caseName)) {
                                statement.addListener(writer);
                            }
                        }
                        if ("event-type".equals(caseName)) {
                            assertEventType(findStatement(statements, "s0"));
                        }
                    }
                    case "deployed" -> {
                        String statement = step.getString("statement", "");
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", statement);
                        record.add("sequence",
                                sequences.merge(statement + ":deployed", 1, Integer::sum));
                        record.add("time", NOW);
                        records.add(record);
                    }
                    case "send" -> {
                        if (sendIndex >= STEP_SENDS[caseIndex].length) {
                            throw new IllegalStateException("unexpected send index " + sendIndex
                                    + " for case " + caseName);
                        }
                        sendIndex++;
                        sendEvent(runtime, step);
                    }
                    case "undeploy-all" -> {
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deployedModules.clear();
                    }
                    default -> throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Registers the NestedMap chain mirroring TestSuiteEventMap.configure:
     * NestedMap = {simple:String, object:SupportBean_A, nodefmap:Map,
     * map:L1{simpleOne:Integer, objectOne:SupportBeanComplexProps,
     * nodefmapOne:Map, mapOne:L2{simpleTwo:Integer,
     * objectTwo:SupportBeanCombinedProps, nodefmapTwo:Map,
     * mapTwo:L3{simpleThree:Long, objectThree:SupportBean_B}}}}.
     */
    private static void registerEventTypes(Configuration configuration) {
        Map<String, Object> levelThree = new HashMap<>();
        levelThree.put("simpleThree", Long.class);
        levelThree.put("objectThree", SupportBean_B.class);
        Map<String, Object> levelTwo = new HashMap<>();
        levelTwo.put("simpleTwo", Integer.class);
        levelTwo.put("objectTwo", SupportBeanCombinedProps.class);
        levelTwo.put("nodefmapTwo", Map.class);
        levelTwo.put("mapTwo", levelThree);
        Map<String, Object> levelOne = new HashMap<>();
        levelOne.put("simpleOne", Integer.class);
        levelOne.put("objectOne", SupportBeanComplexProps.class);
        levelOne.put("nodefmapOne", Map.class);
        levelOne.put("mapOne", levelTwo);
        Map<String, Object> levelZero = new HashMap<>();
        levelZero.put("simple", String.class);
        levelZero.put("object", SupportBean_A.class);
        levelZero.put("nodefmap", Map.class);
        levelZero.put("map", levelOne);
        configuration.getCommon().addEventType("NestedMap", levelZero);
    }

    private static EPStatement findStatement(Map<String, EPStatement> statements, String name) {
        EPStatement statement = statements.get(name);
        if (statement == null) {
            throw new IllegalStateException("statement " + name + " was not deployed");
        }
        return statement;
    }

    /**
     * Mirrors EventMapNestedEventType's assertStatement: the deployed s0
     * statement's event type carries the four declared properties in any
     * order with the pinned root types; the nested dotted lookup stays null.
     */
    private static void assertEventType(EPStatement statement) {
        EventType eventType = statement.getEventType();
        String[] propertiesReceived = eventType.getPropertyNames();
        String[] propertiesExpected = new String[]{"simple", "object", "nodefmap", "map"};
        EPAssertionUtil.assertEqualsAnyOrder(propertiesReceived, propertiesExpected);
        check(String.class.equals(eventType.getPropertyType("simple")),
                "simple type " + eventType.getPropertyType("simple"));
        check(Map.class.equals(eventType.getPropertyType("map")),
                "map type " + eventType.getPropertyType("map"));
        check(Map.class.equals(eventType.getPropertyType("nodefmap")),
                "nodefmap type " + eventType.getPropertyType("nodefmap"));
        check(SupportBean_A.class.equals(eventType.getPropertyType("object")),
                "object type " + eventType.getPropertyType("object"));
        check(eventType.getPropertyType("map.mapOne.simpleOne") == null,
                "map.mapOne.simpleOne type unexpectedly " + eventType.getPropertyType("map.mapOne.simpleOne"));
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"NestedMap".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        runtime.getEventService().sendEventMap(toMap(step.get("payload").asObject()), eventType);
    }

    private static Map<String, Object> toMap(JsonObject object) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (Member member : object) {
            map.put(member.getName(), toJavaValue(member.getValue()));
        }
        return map;
    }

    /**
     * Converts a JSON payload value to the Java value Esper expects:
     * {"_bean":...} constructs the pinned bean, {"_long":N} pins a Java
     * Long, plain objects become Maps, arrays become Object[] and numbers
     * become Integer/Long/Double.
     */
    private static Object toJavaValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isObject()) {
            JsonObject object = value.asObject();
            JsonValue beanTag = object.get("_bean");
            if (beanTag != null && beanTag.isString()) {
                return toBean(beanTag.asString(), object);
            }
            JsonValue longTag = object.get("_long");
            if (longTag != null && longTag.isNumber()) {
                return longTag.asLong();
            }
            return toMap(object);
        }
        if (value.isArray()) {
            JsonArray items = value.asArray();
            Object[] out = new Object[items.size()];
            for (int index = 0; index < items.size(); index++) {
                out[index] = toJavaValue(items.get(index));
            }
            return out;
        }
        if (value.isNumber()) {
            String text = value.toString();
            if (text.indexOf('.') >= 0 || text.indexOf('e') >= 0 || text.indexOf('E') >= 0) {
                return value.asDouble();
            }
            long number = value.asLong();
            if (number >= Integer.MIN_VALUE && number <= Integer.MAX_VALUE) {
                return (int) number;
            }
            return number;
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        return value.asString();
    }

    private static Object toBean(String bean, JsonObject object) {
        switch (bean) {
            case "SupportBean_A" -> {
                return new SupportBean_A(object.getString("id", null));
            }
            case "SupportBean_B" -> {
                return new SupportBean_B(object.getString("id", null));
            }
            case "SupportBeanComplexProps" -> {
                return SupportBeanComplexProps.makeDefaultBean();
            }
            case "SupportBeanCombinedProps" -> {
                return SupportBeanCombinedProps.makeDefaultBean();
            }
            default -> throw new IllegalArgumentException("unsupported bean tag: " + bean);
        }
    }

    private static JsonObject eventRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonObject row(JsonObject fields) {
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    /**
     * Trace normalizer matching the Go runner's renderer: maps become
     * sorted-field rows, the pinned beans render their declared field sets,
     * collections and arrays become JSON arrays, integral numbers keep
     * their long lexeme, null becomes {state:null}.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof EventBean event) {
            return eventRow(event);
        }
        if (value instanceof EventBean[] events) {
            JsonArray array = new JsonArray();
            for (EventBean event : events) {
                array.add(event == null ? normalize(null) : eventRow(event));
            }
            return array;
        }
        if (value instanceof Map<?, ?> mapValue) {
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            return row(fields);
        }
        if (value instanceof List<?> list) {
            JsonArray array = new JsonArray();
            for (Object item : list) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof SupportBean_A bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            return row(fields);
        }
        if (value instanceof SupportBean_B bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            return row(fields);
        }
        if (value instanceof SupportBeanComplexProps bean) {
            JsonObject fields = new JsonObject();
            // The bean exposes indexed only through getIndexed(int); the
            // pinned makeDefaultBean carries exactly slots 0-1.
            fields.add("arrayProperty", normalize(bean.getArrayProperty()));
            fields.add("indexed", normalize(new int[]{bean.getIndexed(0), bean.getIndexed(1)}));
            fields.add("mapProperty", normalize(bean.getMapProperty()));
            fields.add("nested", normalize(bean.getNested()));
            fields.add("simpleProperty", normalize(bean.getSimpleProperty()));
            return row(fields);
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNested", normalize(nested.getNestedNested()));
            fields.add("nestedValue", normalize(nested.getNestedValue()));
            return row(fields);
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNestedNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNestedValue", normalize(nested.getNestedNestedValue()));
            return row(fields);
        }
        if (value instanceof SupportBeanCombinedProps bean) {
            JsonObject fields = new JsonObject();
            fields.add("array", normalize(bean.getArray()));
            fields.add("indexed", normalize(bean.getArray()));
            return row(fields);
        }
        if (value instanceof SupportBeanCombinedProps.NestedLevOne nested) {
            JsonObject fields = new JsonObject();
            fields.add("mapprop", normalize(nested.getMapprop()));
            fields.add("nestLevOneVal", normalize(nested.getNestLevOneVal()));
            return row(fields);
        }
        if (value instanceof SupportBeanCombinedProps.NestedLevTwo nested) {
            JsonObject fields = new JsonObject();
            fields.add("value", normalize(nested.getValue()));
            return row(fields);
        }
        if (value instanceof Object[] objects) {
            JsonArray array = new JsonArray();
            for (Object object : objects) {
                array.add(normalize(object));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            int length = Array.getLength(value);
            JsonArray array = new JsonArray();
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            double number = ((Number) value).doubleValue();
            if (number == Math.rint(number) && !Double.isInfinite(number)) {
                return Json.value((long) number);
            }
            return Json.value(number);
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[]{"SERDEREQUIRED"}, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly " + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl", "flags");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
            validateStringArray(definition.get("flags"), new String[]{"SERDEREQUIRED"},
                    "case " + index + " flags");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned deploy/deployed/send/undeploy-all steps. Deploy steps
     * carry the verbatim EPL per statement; send steps carry the pinned
     * event type. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int deployIndex = 0;
            int sendIndex = 0;
            int deployedIndex = 0;
            for (String operation : STEP_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!STEP_STATEMENTS[caseIndex][deployIndex].equals(string(step, "statement"))
                                || !DEPLOY_EPLS[caseIndex][deployIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deployIndex++;
                        break;
                    case "deployed":
                        requireFields(step, "op", "case", "statement");
                        if (!STEP_STATEMENTS[caseIndex][deployedIndex].equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deployed step " + cursor + " is not pinned");
                        }
                        deployedIndex++;
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!"NestedMap".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        sendIndex++;
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (deployIndex != DEPLOY_EPLS[caseIndex].length || sendIndex != STEP_SENDS[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " step sequence is not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per case. Rows
     * render sorted-field maps through the pinned normalize() shapes.
     */
    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", NOW);
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(eventRow(event));
            }
            return output;
        }
    }
}
