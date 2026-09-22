import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.FragmentEventType;
import com.espertech.esper.common.client.PropertyAccessException;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.event.arr.ObjectArrayEventBean;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the six EventObjectArrayEventNested
 * executions (ordinals 0-4) plus EventObjectArrayEventNestedPojo ordinal 0,
 * replayed as one differential chain:
 *
 * array-property (ord 0, EventObjectArrayArrayProperty): two deploy cycles
 * projecting p0[0]/p0[1]/p1[0].intPrimitive/p1[1]/p0 over MyArrayOA and the
 * same paths under outer.* over MyArrayOAMapOuter.
 *
 * mapped-property (ord 1, EventObjectArrayMappedProperty): three cycles of
 * mapped-property access p0('k1'), outer.p0('k1') and
 * outerTwo.mapProperty('xOne') over events sent via sendEventMap.
 *
 * map-name-nested (ord 2, EventObjectArrayMapNamePropertyNested): two cycles
 * reading named-map properties inside an objectarray type, the second with
 * the '?' dynamic-property variant.
 *
 * map-name (ord 3, EventObjectArrayMapNameProperty): named-map properties
 * p0.n0/p1[i].n0 plus whole-map selects over MyOAWithAMap.
 *
 * oa-nested (ord 4, EventObjectArrayObjectArrayNested): TypeRoot#lastevent
 * asserted through the statement iterator only (no listener); the snapshot
 * record renders the nested TypeLev0/TypeLev1 fragments as rows.
 *
 * pojo (EventObjectArrayEventNestedPojo ord 0): three cycles — the verbatim
 * 22-column deep projection over NestedObjectArr, a select-* type-check
 * cycle whose listener row is still recorded, and the MyNested
 * bean.insides.anyOf filter cycle.
 *
 * Mirroring the sibling scenario oracles, each case runs in its own runtime
 * (URI event-objectarray-nested-<case>), deploys the pinned EPL per cycle,
 * records listener updates as sorted-field rows and undeploys between
 * cycles. Bean payloads are tagged {"_bean":...} and constructed in the
 * oracle; {"_long":N} pins a Java Long inside map payloads. Nested
 * objectarray fragments render as nested rows via getFragment; indexed-only
 * plain gets render the <unreadable> marker.
 */
public final class EventObjectArrayNestedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "event-objectarray-nested";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNested.java";

    private static final String DESCRIPTION =
            "EventObjectArrayEventNested ordinals 0-4 plus EventObjectArrayEventNestedPojo ordinal 0 (regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNestedPojo.java): array-property projects primitive and bean arrays at level one and inside a nested objectarray type; mapped-property resolves p0('k1'), outer.p0('k1') and outerTwo.mapProperty('xOne') over map-sent events; map-name-nested and map-name read named-map properties inside objectarray types including the '?' dynamic variant; oa-nested asserts the TypeRoot#lastevent iterator's nested p0.p0id/p0.p1.p1id paths through a snapshot record; pojo projects a 22-column deep mixed graph, a select-* type check and a MyNested anyOf filter.";

    private static final String[] CASES = {
            "array-property", "mapped-property", "map-name-nested", "map-name",
            "oa-nested", "pojo"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-1704d5718125f4502592",
            "java-runtime-072c48af61cdd8b27856",
            "java-runtime-5a08e705396c651e2159",
            "java-runtime-7d6eb46f34c9d597b7d1",
            "java-runtime-957493fa76a9e36a6091",
            "java-runtime-57d2cf8ae645bedf06e6"
    };
    private static final String[] EXECUTIONS = {
            "EventObjectArrayArrayProperty",
            "EventObjectArrayMappedProperty",
            "EventObjectArrayMapNamePropertyNested",
            "EventObjectArrayMapNameProperty",
            "EventObjectArrayObjectArrayNested",
            "EventObjectArrayEventNestedPojo"
    };
    private static final String[] STATIC_IDS = {
            "java-2b57d1da10ba0e8fcce5",
            "java-2b57d1da10ba0e8fcce5",
            "java-2b57d1da10ba0e8fcce5",
            "java-2b57d1da10ba0e8fcce5",
            "java-2b57d1da10ba0e8fcce5",
            "java-a9545c224ccf899a4d45"
    };
    private static final String[] OBSERVATIONS = {
            "listener", "listener", "listener", "listener", "iterator", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0, 0, 0, 1, 0};

    // Verbatim transcriptions of the pinned statement texts; the pojo
    // projection keeps the missing space after "as c2," from the Java
    // string concatenation.
    private static final String EPL_ARRAY_DIRECT =
            "@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayOA";
    private static final String EPL_ARRAY_OUTER =
            "@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayOAMapOuter";
    private static final String EPL_MAPPED_DIRECT =
            "@name('s0') select p0('k1') as a from MyMappedPropertyMap";
    private static final String EPL_MAPPED_OUTER =
            "@name('s0') select outer.p0('k1') as a from MyMappedPropertyMapOuter";
    private static final String EPL_MAPPED_BEAN =
            "@name('s0') select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo";
    private static final String EPL_MAP_NESTED =
            "@name('s0') select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyObjectArrayMapOuter";
    private static final String EPL_MAP_NESTED_DYNAMIC =
            "@name('s0') select outer.p0.n0? as a, outer.p1[0].n0? as b, outer.p1[1]?.n0 as c, outer.p0? as d, outer.p1? as e from MyObjectArrayMapOuter";
    private static final String EPL_MAP_NAME =
            "@name('s0') select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyOAWithAMap";
    private static final String EPL_OA_NESTED =
            "@name('s0') select * from TypeRoot#lastevent";
    private static final String EPL_POJO_PROJECTION =
            "@name('s0') select simple, object, nodefmap, map, " +
            "object.id as a1, nodefmap.key1? as a2, nodefmap.key2? as a3, nodefmap.key3?.key4 as a4, " +
            "map.objectOne as b1, map.simpleOne as b2, map.nodefmapOne.key2? as b3, map.mapOne.simpleTwo? as b4, " +
            "map.objectOne.indexed[1] as c1, map.objectOne.nested.nestedValue as c2," +
            "map.mapOne.simpleTwo as d1, map.mapOne.objectTwo as d2, map.mapOne.nodefmapTwo as d3, " +
            "map.mapOne.mapTwo as e1, map.mapOne.mapTwo.simpleThree as e2, map.mapOne.mapTwo.objectThree as e3, " +
            "map.mapOne.objectTwo.array[1].mapped('1ma').value as f1, map.mapOne.mapTwo.objectThree.id as f2" +
            " from NestedObjectArr";
    private static final String EPL_POJO_STAR =
            "@name('s0') select * from NestedObjectArr";
    private static final String EPL_POJO_ANYOF =
            "@name('s0') select * from MyNested(bean.insides.anyOf(i=>id = 'A'))";

    private static final String[][] DEPLOY_EPLS = {
            {EPL_ARRAY_DIRECT, EPL_ARRAY_OUTER},
            {EPL_MAPPED_DIRECT, EPL_MAPPED_OUTER, EPL_MAPPED_BEAN},
            {EPL_MAP_NESTED, EPL_MAP_NESTED_DYNAMIC},
            {EPL_MAP_NAME},
            {EPL_OA_NESTED},
            {EPL_POJO_PROJECTION, EPL_POJO_STAR, EPL_POJO_ANYOF}
    };
    private static final String[] CASE_EPLS = {
            EPL_ARRAY_DIRECT, EPL_MAPPED_DIRECT, EPL_MAP_NESTED, EPL_MAP_NAME,
            EPL_OA_NESTED, EPL_POJO_PROJECTION};

    private static final String[][] STEP_OPS = {
            {"deploy", "send", "undeploy-all", "deploy", "send", "undeploy-all"},
            {"deploy", "send", "undeploy-all", "deploy", "send", "undeploy-all",
                    "deploy", "send", "undeploy-all"},
            {"deploy", "send", "undeploy-all", "deploy", "send", "undeploy-all"},
            {"deploy", "send", "undeploy-all"},
            {"deploy", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "undeploy-all", "deploy", "send", "undeploy-all",
                    "deploy", "send", "undeploy-all"}
    };
    private static final String[][] STEP_EVENT_TYPES = {
            {"MyArrayOA", "MyArrayOAMapOuter"},
            {"MyMappedPropertyMap", "MyMappedPropertyMapOuter", "MyMappedPropertyMapOuterTwo"},
            {"MyObjectArrayMapOuter", "MyObjectArrayMapOuter"},
            {"MyOAWithAMap"},
            {"TypeRoot"},
            {"NestedObjectArr", "NestedObjectArr", "MyNested"}
    };

    private static final int EXPECTED_STEPS = 43;
    private static final int EXPECTED_RECORDS = 12;

    private EventObjectArrayNestedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EventObjectArrayNestedScenarioOracle <scenario.json>");
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
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        registerEventTypes(configuration);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            Map<String, EPStatement> statements = new HashMap<>();
            boolean active = false;
            int deployIndex = 0;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("deploy".equals(operation)) {
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
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(
                                    SCENARIO_ID + "-" + caseIndex + "-" + deployIndex));
                    EPStatement s0 = findStatement(deployment);
                    statements.put(s0.getName(), s0);
                    // oa-nested mirrors env.assertIterator: no listener is
                    // attached, so the send produces no listener record.
                    if (!"oa-nested".equals(caseName)) {
                        s0.addListener(writer);
                    }
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("snapshot".equals(operation)) {
                    snapshotStep(runtime, caseName, step, statements, records);
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Registers the event types used by the six executions, mirroring
     * TestSuiteEventObjectArray.configure: objectarray types via the
     * String[]/Object[] addEventType form (named nested types as type-name
     * strings, anonymous nested maps as Map defs), map types via Map defs
     * and bean types via Class.
     */
    private static void registerEventTypes(Configuration configuration) {
        configuration.getCommon().addEventType(SupportBean.class);

        configuration.getCommon().addEventType("MyArrayOA",
                new String[]{"p0", "p1"}, new Object[]{int[].class, SupportBean[].class});
        configuration.getCommon().addEventType("MyArrayOAMapOuter",
                new String[]{"outer"}, new Object[]{"MyArrayOA"});

        Map<String, Object> mappedDef = new HashMap<>();
        mappedDef.put("p0", Map.class);
        configuration.getCommon().addEventType("MyMappedPropertyMap", mappedDef);
        Map<String, Object> mappedDefOuter = new HashMap<>();
        mappedDefOuter.put("outer", mappedDef);
        configuration.getCommon().addEventType("MyMappedPropertyMapOuter", mappedDefOuter);
        Map<String, Object> mappedDefOuterTwo = new HashMap<>();
        mappedDefOuterTwo.put("outerTwo", LocalSupportBeanComplexProps.class);
        configuration.getCommon().addEventType("MyMappedPropertyMapOuterTwo", mappedDefOuterTwo);

        Map<String, Object> namedDef = new HashMap<>();
        namedDef.put("n0", int.class);
        configuration.getCommon().addEventType("MyNamedMap", namedDef);
        Map<String, Object> eventDef = new HashMap<>();
        eventDef.put("p0", "MyNamedMap");
        eventDef.put("p1", "MyNamedMap[]");
        configuration.getCommon().addEventType("MyObjectArrayMapOuter",
                new String[]{"outer"}, new Object[]{eventDef});
        configuration.getCommon().addEventType("MyOAWithAMap",
                new String[]{"p0", "p1"}, new Object[]{"MyNamedMap", "MyNamedMap[]"});

        configuration.getCommon().addEventType("TypeLev1",
                new String[]{"p1id"}, new Object[]{int.class});
        configuration.getCommon().addEventType("TypeLev0",
                new String[]{"p0id", "p1"}, new Object[]{int.class, "TypeLev1"});
        configuration.getCommon().addEventType("TypeRoot",
                new String[]{"rootId", "p0"}, new Object[]{int.class, "TypeLev0"});

        Map<String, Object> levelThree = new HashMap<>();
        levelThree.put("simpleThree", Long.class);
        levelThree.put("objectThree", LocalSupportBeanB.class);
        Map<String, Object> levelTwo = new HashMap<>();
        levelTwo.put("simpleTwo", Integer.class);
        levelTwo.put("objectTwo", LocalSupportBeanCombinedProps.class);
        levelTwo.put("nodefmapTwo", Map.class);
        levelTwo.put("mapTwo", levelThree);
        Map<String, Object> levelOne = new HashMap<>();
        levelOne.put("simpleOne", Integer.class);
        levelOne.put("objectOne", LocalSupportBeanComplexProps.class);
        levelOne.put("nodefmapOne", Map.class);
        levelOne.put("mapOne", levelTwo);
        configuration.getCommon().addEventType("NestedObjectArr",
                new String[]{"simple", "object", "nodefmap", "map"},
                new Object[]{String.class, LocalSupportBeanA.class, Map.class, levelOne});

        configuration.getCommon().addEventType("MyNested",
                new String[]{"bean"}, new Object[]{LocalMyNested.class});
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    /**
     * Emits one {"operation":"snapshot"} record mirroring the Java
     * execution's assertIterator call: the statement's default iterator
     * drained into normalized rows. The TypeRoot row renders the nested
     * objectarray fragments (p0, p0.p1) as rows, proving the
     * rootId/p0.p0id/p0.p1.p1id path resolution.
     */
    private static void snapshotStep(EPRuntime runtime, String caseName, JsonObject step,
                                     Map<String, EPStatement> statements, JsonArray records) {
        String name = step.getString("statement", "");
        EPStatement statement = statements.get(name);
        if (statement == null) {
            throw new IllegalStateException("snapshot references unknown statement " + name);
        }
        List<EventBean> drained = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            drained.add(iterator.next());
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = new JsonArray();
        for (EventBean event : drained) {
            rows.add(eventRow(event));
        }
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonValue payload = step.get("payload");
        switch (eventType) {
            case "MyArrayOA" -> {
                JsonArray items = payload.asArray();
                int[] p0 = toIntArray(items.get(0).asArray());
                JsonArray beanItems = items.get(1).asArray();
                SupportBean[] p1 = new SupportBean[beanItems.size()];
                for (int index = 0; index < p1.length; index++) {
                    p1[index] = (SupportBean) toJavaValue(beanItems.get(index));
                }
                runtime.getEventService().sendEventObjectArray(new Object[]{p0, p1}, eventType);
            }
            case "MyArrayOAMapOuter" -> {
                JsonArray items = payload.asArray();
                JsonArray inner = items.get(0).asArray();
                int[] p0 = toIntArray(inner.get(0).asArray());
                JsonArray beanItems = inner.get(1).asArray();
                SupportBean[] p1 = new SupportBean[beanItems.size()];
                for (int index = 0; index < p1.length; index++) {
                    p1[index] = (SupportBean) toJavaValue(beanItems.get(index));
                }
                runtime.getEventService().sendEventObjectArray(
                        new Object[]{new Object[]{p0, p1}}, eventType);
            }
            case "MyMappedPropertyMap", "MyMappedPropertyMapOuter", "MyMappedPropertyMapOuterTwo" ->
                runtime.getEventService().sendEventMap(toMap(payload.asObject()), eventType);
            case "MyObjectArrayMapOuter" -> {
                JsonArray items = payload.asArray();
                JsonObject outer = items.get(0).asObject();
                Map<String, Object> outerMap = new HashMap<>();
                outerMap.put("p0", toMap(outer.get("p0").asObject()));
                JsonArray p1Items = outer.get("p1").asArray();
                Map[] p1 = new Map[p1Items.size()];
                for (int index = 0; index < p1.length; index++) {
                    p1[index] = toMap(p1Items.get(index).asObject());
                }
                outerMap.put("p1", p1);
                runtime.getEventService().sendEventObjectArray(new Object[]{outerMap}, eventType);
            }
            case "MyOAWithAMap" -> {
                JsonArray items = payload.asArray();
                Map<String, Object> p0 = toMap(items.get(0).asObject());
                JsonArray p1Items = items.get(1).asArray();
                Map[] p1 = new Map[p1Items.size()];
                for (int index = 0; index < p1.length; index++) {
                    p1[index] = toMap(p1Items.get(index).asObject());
                }
                runtime.getEventService().sendEventObjectArray(new Object[]{p0, p1}, eventType);
            }
            case "TypeRoot", "NestedObjectArr", "MyNested" ->
                runtime.getEventService().sendEventObjectArray(toObjectArray(payload.asArray()), eventType);
            default -> throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static int[] toIntArray(JsonArray items) {
        int[] values = new int[items.size()];
        for (int index = 0; index < values.length; index++) {
            values[index] = items.get(index).asInt();
        }
        return values;
    }

    private static Map<String, Object> toMap(JsonObject object) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (Member member : object) {
            map.put(member.getName(), toJavaValue(member.getValue()));
        }
        return map;
    }

    private static Object[] toObjectArray(JsonArray items) {
        Object[] values = new Object[items.size()];
        for (int index = 0; index < values.length; index++) {
            values[index] = toJavaValue(items.get(index));
        }
        return values;
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
            return toObjectArray(value.asArray());
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
            case "SupportBean" -> {
                return new SupportBean(object.getString("theString", null),
                        object.getInt("intPrimitive", 0));
            }
            case "SupportBean_A" -> {
                return new LocalSupportBeanA(object.getString("id", null));
            }
            case "SupportBean_B" -> {
                return new LocalSupportBeanB(object.getString("id", null));
            }
            case "SupportBeanComplexProps" -> {
                return LocalSupportBeanComplexProps.makeDefaultBean();
            }
            case "SupportBeanCombinedProps" -> {
                return LocalSupportBeanCombinedProps.makeDefaultBean();
            }
            case "MyNested" -> {
                JsonArray insides = object.get("insides").asArray();
                List<LocalMyInside> list = new ArrayList<>();
                for (JsonValue item : insides) {
                    list.add(new LocalMyInside(item.asObject().getString("id", null)));
                }
                return new LocalMyNested(list);
            }
            default -> throw new IllegalArgumentException("unsupported bean tag: " + bean);
        }
    }

    /**
     * Reads one output column. Only objectarray fragments resolve through
     * getFragment: nested objectarray properties render as rows (the
     * oa-nested snapshot's p0/p0.p1 path), and indexed fragments of
     * objectarray elements wrap each Object[] in an ObjectArrayEventBean.
     * Bean and map fragments fall through to the plain get so bean columns
     * render through normalize's pinned bean shapes instead of the bean
     * event type's full property set. Plain gets that fail with
     * PropertyAccessException (indexed-only properties) render the
     * <unreadable> marker so the column stays visible in the trace.
     */
    private static Object fragmentAwareValue(EventBean event, String prop) {
        FragmentEventType fragmentType = event.getEventType().getFragmentType(prop);
        if (fragmentType != null
                && Object[].class.equals(fragmentType.getFragmentType().getUnderlyingType())) {
            Object fragment = event.getFragment(prop);
            if (fragment != null) {
                if (fragmentType.isIndexed()) {
                    if (fragment instanceof Object[] array) {
                        EventBean[] events = new EventBean[array.length];
                        for (int index = 0; index < array.length; index++) {
                            if (array[index] instanceof Object[] nested) {
                                events[index] = new ObjectArrayEventBean(nested,
                                        fragmentType.getFragmentType());
                            }
                        }
                        return events;
                    }
                    return fragment;
                }
                if (fragment instanceof Object[] nested) {
                    return new ObjectArrayEventBean(nested, fragmentType.getFragmentType());
                }
                return fragment;
            }
        }
        return readProperty(event, prop);
    }

    private static Object readProperty(EventBean event, String prop) {
        try {
            return event.get(prop);
        } catch (PropertyAccessException unreadable) {
            return UNREADABLE;
        }
    }

    private static final Object UNREADABLE = new Object() {
        @Override
        public String toString() {
            return "<unreadable>";
        }
    };

    private static JsonObject eventRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(fragmentAwareValue(event, prop)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonObject row(JsonObject fields) {
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value == UNREADABLE) {
            return Json.value("<unreadable>");
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
        if (value instanceof SupportBean bean) {
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanA bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanB bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanComplexProps bean) {
            JsonObject fields = new JsonObject();
            fields.add("arrayProperty", normalize(bean.getArrayProperty()));
            fields.add("indexed", normalize(bean.getIndexedProps()));
            fields.add("mapProperty", normalize(bean.getMapProperty()));
            fields.add("nested", normalize(bean.getNested()));
            fields.add("simpleProperty", normalize(bean.getSimpleProperty()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanComplexProps.LocalNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNested", normalize(nested.getNestedNested()));
            fields.add("nestedValue", normalize(nested.getNestedValue()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanComplexProps.LocalNestedNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNestedValue", normalize(nested.getNestedNestedValue()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanCombinedProps bean) {
            JsonObject fields = new JsonObject();
            fields.add("array", normalize(bean.getArray()));
            fields.add("indexed", normalize(bean.getArray()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanCombinedProps.LocalNestedLevOne nested) {
            JsonObject fields = new JsonObject();
            fields.add("mapprop", normalize(nested.getMapprop()));
            fields.add("nestLevOneVal", normalize(nested.getNestLevOneVal()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanCombinedProps.LocalNestedLevTwo nested) {
            JsonObject fields = new JsonObject();
            fields.add("value", normalize(nested.getValue()));
            return row(fields);
        }
        if (value instanceof LocalMyNested bean) {
            JsonObject fields = new JsonObject();
            fields.add("insides", normalize(bean.getInsides()));
            return row(fields);
        }
        if (value instanceof LocalMyInside bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS[index]
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned deploy/send/snapshot/undeploy-all steps. Deploy steps
     * carry the verbatim EPL per cycle; send steps carry the pinned event
     * type. Unknown step fields are rejected.
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
            for (String operation : STEP_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!"s0".equals(string(step, "statement"))
                                || !DEPLOY_EPLS[caseIndex][deployIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deployIndex++;
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!STEP_EVENT_TYPES[caseIndex][sendIndex].equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        sendIndex++;
                        break;
                    case "snapshot":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
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
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
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

    /** Local mirror of the pinned SupportBean_A regression bean. */
    public static class LocalSupportBeanA {
        private final String id;

        public LocalSupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    /** Local mirror of the pinned SupportBean_B regression bean. */
    public static class LocalSupportBeanB {
        private final String id;

        public LocalSupportBeanB(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    /**
     * Local mirror of the pinned SupportBeanComplexProps regression bean:
     * makeDefaultBean carries mapProperty {xOne:yOne,xTwo:yTwo},
     * indexedProps {1,2}, arrayProperty {10,20,30}, simpleProperty "simple"
     * and nested values "nestedValue"/"nestedNestedValue".
     */
    public static class LocalSupportBeanComplexProps {
        private final String simpleProperty;
        private final int[] indexedProps;
        private final Map<String, String> mapProperty;
        private final int[] arrayProperty;
        private final LocalNested nested;

        public static LocalSupportBeanComplexProps makeDefaultBean() {
            Map<String, String> mapProp = new HashMap<>();
            mapProp.put("xOne", "yOne");
            mapProp.put("xTwo", "yTwo");
            return new LocalSupportBeanComplexProps("simple", new int[]{1, 2}, mapProp,
                    new int[]{10, 20, 30}, "nestedValue", "nestedNestedValue");
        }

        public LocalSupportBeanComplexProps(String simpleProperty, int[] indexedProps,
                                            Map<String, String> mapProperty, int[] arrayProperty,
                                            String nestedValue, String nestedNestedValue) {
            this.simpleProperty = simpleProperty;
            this.indexedProps = indexedProps;
            this.mapProperty = mapProperty;
            this.arrayProperty = arrayProperty;
            this.nested = new LocalNested(nestedValue, nestedNestedValue);
        }

        public String getSimpleProperty() {
            return simpleProperty;
        }

        public Map<String, String> getMapProperty() {
            return mapProperty;
        }

        public int getIndexed(int index) {
            return indexedProps[index];
        }

        public int[] getIndexedProps() {
            return indexedProps;
        }

        public int[] getArrayProperty() {
            return arrayProperty;
        }

        public LocalNested getNested() {
            return nested;
        }

        public static class LocalNested {
            private final String nestedValue;
            private final LocalNestedNested nestedNested;

            public LocalNested(String nestedValue, String nestedNestedValue) {
                this.nestedValue = nestedValue;
                this.nestedNested = new LocalNestedNested(nestedNestedValue);
            }

            public String getNestedValue() {
                return nestedValue;
            }

            public LocalNestedNested getNestedNested() {
                return nestedNested;
            }
        }

        public static class LocalNestedNested {
            private final String nestedNestedValue;

            public LocalNestedNested(String nestedNestedValue) {
                this.nestedNestedValue = nestedNestedValue;
            }

            public String getNestedNestedValue() {
                return nestedNestedValue;
            }
        }
    }

    /**
     * Local mirror of the pinned SupportBeanCombinedProps regression bean:
     * makeDefaultBean carries four indexed slots ([3] left empty) whose
     * mapped('1ma').value is "1ma0"; getArray() returns the same array.
     */
    public static class LocalSupportBeanCombinedProps {
        private final LocalNestedLevOne[] indexed;

        public static LocalSupportBeanCombinedProps makeDefaultBean() {
            LocalNestedLevOne[] nested = new LocalNestedLevOne[4];
            nested[0] = new LocalNestedLevOne(new String[][]{{"0ma", "0ma0"}, {"0mb", "0ma1"}});
            nested[1] = new LocalNestedLevOne(new String[][]{{"1ma", "1ma0"}, {"1mb", "1ma1"}});
            nested[2] = new LocalNestedLevOne(new String[][]{{"2ma", "valueOne"}, {"2mb", "2ma1"}});
            return new LocalSupportBeanCombinedProps(nested);
        }

        public LocalSupportBeanCombinedProps(LocalNestedLevOne[] indexed) {
            this.indexed = indexed;
        }

        public LocalNestedLevOne getIndexed(int index) {
            return indexed[index];
        }

        public LocalNestedLevOne[] getArray() {
            return indexed;
        }

        public static class LocalNestedLevOne {
            private final Map<String, LocalNestedLevTwo> map = new HashMap<>();

            public LocalNestedLevOne(String[][] keysAndValues) {
                for (String[] keyAndValue : keysAndValues) {
                    map.put(keyAndValue[0], new LocalNestedLevTwo(keyAndValue[1]));
                }
            }

            public LocalNestedLevTwo getMapped(String key) {
                return map.get(key);
            }

            public Map<String, LocalNestedLevTwo> getMapprop() {
                return map;
            }

            public String getNestLevOneVal() {
                return "abc";
            }
        }

        public static class LocalNestedLevTwo {
            private final String value;

            public LocalNestedLevTwo(String value) {
                this.value = value;
            }

            public String getValue() {
                return value;
            }
        }
    }

    /** Local mirror of the pinned EventObjectArrayEventNestedPojo.MyNested bean. */
    public static class LocalMyNested {
        private final List<LocalMyInside> insides;

        public LocalMyNested(List<LocalMyInside> insides) {
            this.insides = insides;
        }

        public List<LocalMyInside> getInsides() {
            return insides;
        }
    }

    /** Local mirror of the pinned EventObjectArrayEventNestedPojo.MyInside bean. */
    public static class LocalMyInside {
        private final String id;

        public LocalMyInside(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }
}
