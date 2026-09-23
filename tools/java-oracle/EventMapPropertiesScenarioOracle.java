/*
 ***************************************************************************************
 *  Copyright (C) 2006 EsperTech, Inc. All rights reserved.                            *
 *  http://www.espertech.com/esper                                                     *
 *  http://www.espertech.com                                                           *
 *  ---------------------------------------------------------------------------------- *
 *  The software in this package is published under the terms of the GPL license       *
 *  a copy of which has been included with this distribution in the license.txt file.  *
 ***************************************************************************************
 */

import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.PropertyAccessException;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBeanComplexProps;
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
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Properties;

/**
 * Scenario runner for the EventMapProperties regression suite
 * (regression-lib/.../event/map/EventMapProperties.java): map-event property
 * access over indexed, mapped and named-map properties.
 *
 * <p>The suite registers eight Map event types on the Configuration exactly
 * like TestSuiteEventMap.configure (no create-schema EPL): MyNamedMap{n0:int};
 * MyMapWithAMap{p0:"MyNamedMap",p1:"MyNamedMap[]"}; MyArrayMap{p0:int[],
 * p1:SupportBean[]}; MyArrayMapOuter{outer:<inline MyArrayMap def>};
 * MyMappedPropertyMap{p0:Map}; MyMappedPropertyMapOuter{outer:<inline
 * MyMappedPropertyMap def>}; MyMappedPropertyMapOuterTwo{outerTwo:
 * SupportBeanComplexProps}; MyArrayMapTwo{outer:<inline MyMapWithAMap def>}.
 * Java shares the arrayDef/mappedDef map objects between the inline anonymous
 * definitions and the named types, while MyArrayMapTwo's inline def is a
 * structurally identical fresh map, so the nested fragments are identical.</p>
 *
 * <p>Each case replays one execution on a fresh runtime. Every statement is
 * its own compileDeploy/addListener("s0")/sendEvent/undeploy cycle —
 * undeployModuleContaining("s0") and undeployAll are the same boundary for a
 * single-statement module. The listener records one row per delivered event
 * with a per-statement sequence and the runtime current time; Java's
 * EventType compile-time assertions (Integer/Object/String/Map/Map[]/int[]/
 * bean) carry no trace records.</p>
 *
 * <p>Column rendering: null -> {state:null}; EventBean -> {kind:"row",row}
 * over the event type's property names (never selected here); SupportBean ->
 * {theString,intPrimitive} pinned fields; SupportBeanComplexProps -> the six
 * makeDefaultBean readable properties (never selected here); arrays ->
 * element-wise JSON arrays (int[] -> [1,2,3], Map[]/bean[] -> arrays of
 * objects); Map -> plain JSON object; everything else -> the boxed value.
 * This matches the Go normalizeValue output for the same underlying values
 * (raw struct -> plain JSON object, map -> plain object, slice -> array).</p>
 */
public class EventMapPropertiesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-map-properties";

    // Byte-exact EPL transcriptions of EventMapProperties.java: stmt1 line 40,
    // stmt2 line 59 (ord 0); lines 82, 94, 105 (ord 1); lines 120, 142 (ord 2);
    // line 158 (ord 3).
    private static final String EPL_ARRAY =
        "@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayMap";
    private static final String EPL_ARRAY_OUTER =
        "@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayMapOuter";
    private static final String EPL_MAPPED =
        "@name('s0') select p0('k1') as a from MyMappedPropertyMap";
    private static final String EPL_MAPPED_OUTER =
        "@name('s0') select outer.p0('k1') as a from MyMappedPropertyMapOuter";
    private static final String EPL_MAPPED_OUTER_TWO =
        "@name('s0') select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo";
    private static final String EPL_NAME_NESTED =
        "@name('s0') select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyArrayMapTwo";
    private static final String EPL_NAME_NESTED_OPT =
        "@name('s0') select outer.p0.n0? as a, outer.p1[0].n0? as b, outer.p1[1]?.n0 as c, outer.p0? as d, outer.p1? as e from MyArrayMapTwo";
    private static final String EPL_MAP_NAME_PROPERTY =
        "@name('s0') select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyMapWithAMap";

    private static final String[] CASES = {
        "array-property",
        "mapped-property",
        "map-name-nested",
        "map-name",
    };

    // Pinned per-case step shape: (deploy EPL, send eventType, send payload)
    // triples; every triple is followed by an undeploy of s0. Payloads compare
    // against JsonObject.toString() (compact, member order preserved).
    private static final String[][][] CASE_STEPS = {
        {
            {EPL_ARRAY, "MyArrayMap",
                "{\"p0\":[1,2,3],\"p1\":[{\"theString\":\"e1\",\"intPrimitive\":5},{\"theString\":\"e2\",\"intPrimitive\":6}]}"},
            {EPL_ARRAY_OUTER, "MyArrayMapOuter",
                "{\"outer\":{\"p0\":[1,2,3],\"p1\":[{\"theString\":\"e1\",\"intPrimitive\":5},{\"theString\":\"e2\",\"intPrimitive\":6}]}}"},
        },
        {
            {EPL_MAPPED, "MyMappedPropertyMap",
                "{\"p0\":{\"k1\":\"v1\"}}"},
            {EPL_MAPPED_OUTER, "MyMappedPropertyMapOuter",
                "{\"outer\":{\"p0\":{\"k1\":\"v1\"}}}"},
            {EPL_MAPPED_OUTER_TWO, "MyMappedPropertyMapOuterTwo",
                "{\"outerTwo\":{\"simpleProperty\":\"simple\",\"mapped\":{\"keyOne\":\"valueOne\",\"keyTwo\":\"valueTwo\"},\"indexed\":[1,2],\"mapProperty\":{\"xOne\":\"yOne\",\"xTwo\":\"yTwo\"},\"arrayProperty\":[10,20,30],\"nested\":{\"nestedValue\":\"nestedValue\",\"nestedNested\":{\"nestedNestedValue\":\"nestedNestedValue\"}}}}"},
        },
        {
            {EPL_NAME_NESTED, "MyArrayMapTwo",
                "{\"outer\":{\"p0\":{\"n0\":1},\"p1\":[{\"n0\":2},{\"n0\":3}]}}"},
            {EPL_NAME_NESTED_OPT, "MyArrayMapTwo",
                "{\"outer\":{\"p0\":{\"n0\":1},\"p1\":[{\"n0\":2},{\"n0\":3}]}}"},
        },
        {
            {EPL_MAP_NAME_PROPERTY, "MyMapWithAMap",
                "{\"p0\":{\"n0\":1},\"p1\":[{\"n0\":2},{\"n0\":3}]}"},
        },
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventMapPropertiesScenarioOracle <scenario>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        validateScenario(scenario);
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        JsonArray outRecords = new JsonArray();
        for (JsonObject record : records) {
            outRecords.add(record);
        }
        root.add("records", outRecords);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records)
        throws Exception {
        Configuration configuration = configuration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
        try {
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(epl, new CompilerArguments(configuration));
                        EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                        for (EPStatement statement : deployment.getStatements()) {
                            statement.addListener(listener(caseName, sequences, records, runtime));
                        }
                        deployments.put(label, deployment);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                        break;
                    case "undeploy": {
                        // undeployModuleContaining: every statement deploys as
                        // its own module, so the statement key selects the
                        // whole deployment.
                        String label = string(step, "statement");
                        EPDeployment deployment = deployments.remove(label);
                        if (deployment == null) {
                            throw new IllegalStateException(
                                "undeploy of unknown statement " + label);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        break;
                    }
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
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

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           List<JsonObject> records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(row(event));
        }
        return rows;
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            Object value;
            try {
                value = event.get(name);
            } catch (PropertyAccessException unreadable) {
                continue;
            }
            fields.add(name, normalize(value));
        }
        item.add("fields", fields);
        return item;
    }


    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject state = new JsonObject();
            state.add("state", "null");
            return state;
        }
        if (value instanceof EventBean) {
            EventBean bean = (EventBean) value;
            JsonObject wrapped = new JsonObject();
            wrapped.add("kind", "row");
            wrapped.add("row", row(bean));
            return wrapped;
        }
        if (value instanceof SupportBean) {
            SupportBean bean = (SupportBean) value;
            JsonObject object = new JsonObject();
            object.add("theString", normalize(bean.getTheString()));
            object.add("intPrimitive", bean.getIntPrimitive());
            return object;
        }
        if (value instanceof SupportBeanComplexProps) {
            // Never selected by this suite; rendered for completeness from the
            // makeDefaultBean readable properties.
            SupportBeanComplexProps bean = (SupportBeanComplexProps) value;
            JsonObject object = new JsonObject();
            object.add("simpleProperty", bean.getSimpleProperty());
            JsonObject mapped = new JsonObject();
            mapped.add("keyOne", bean.getMapped("keyOne"));
            mapped.add("keyTwo", bean.getMapped("keyTwo"));
            object.add("mapped", mapped);
            JsonArray indexed = new JsonArray();
            indexed.add(bean.getIndexed(0));
            indexed.add(bean.getIndexed(1));
            object.add("indexed", indexed);
            object.add("mapProperty", normalize(bean.getMapProperty()));
            object.add("arrayProperty", normalize(bean.getArrayProperty()));
            JsonObject nested = new JsonObject();
            nested.add("nestedValue", bean.getNested().getNestedValue());
            JsonObject nestedNested = new JsonObject();
            nestedNested.add("nestedNestedValue",
                bean.getNested().getNestedNested().getNestedNestedValue());
            nested.add("nestedNested", nestedNested);
            object.add("nested", nested);
            return object;
        }
        if (value instanceof Map<?, ?> map) {
            JsonObject object = new JsonObject();
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                object.add(key, normalize(map.get(key)));
            }
            return object;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int i = 0; i < length; i++) {
                array.add(normalize(Array.get(value, i)));
            }
            return array;
        }
        if (value instanceof Integer || value instanceof Long
            || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }

    private static void sendEvent(EPRuntime runtime, String eventType, JsonObject payload) {
        Map<String, Object> event = new LinkedHashMap<>();
        switch (eventType) {
            case "MyArrayMap":
                event.put("p0", intArray(payload.get("p0").asArray()));
                event.put("p1", beans(payload.get("p1").asArray()));
                break;
            case "MyArrayMapOuter": {
                Map<String, Object> outer = new LinkedHashMap<>();
                JsonObject inner = payload.get("outer").asObject();
                outer.put("p0", intArray(inner.get("p0").asArray()));
                outer.put("p1", beans(inner.get("p1").asArray()));
                event.put("outer", outer);
                break;
            }
            case "MyMappedPropertyMap":
                event.put("p0", mappedEvent(payload.get("p0").asObject()));
                break;
            case "MyMappedPropertyMapOuter": {
                Map<String, Object> outer = new LinkedHashMap<>();
                outer.put("p0", mappedEvent(payload.get("outer").asObject().get("p0").asObject()));
                event.put("outer", outer);
                break;
            }
            case "MyMappedPropertyMapOuterTwo":
                event.put("outerTwo", complexProps(payload.get("outerTwo").asObject()));
                break;
            case "MyArrayMapTwo": {
                Map<String, Object> outer = new LinkedHashMap<>();
                JsonObject inner = payload.get("outer").asObject();
                outer.put("p0", namedMap(inner.get("p0").asObject()));
                outer.put("p1", namedMaps(inner.get("p1").asArray()));
                event.put("outer", outer);
                break;
            }
            case "MyMapWithAMap":
                event.put("p0", namedMap(payload.get("p0").asObject()));
                event.put("p1", namedMaps(payload.get("p1").asArray()));
                break;
            default:
                throw new IllegalStateException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static int[] intArray(JsonArray array) {
        int[] values = new int[array.size()];
        for (int i = 0; i < values.length; i++) {
            values[i] = array.get(i).asInt();
        }
        return values;
    }

    private static SupportBean[] beans(JsonArray array) {
        SupportBean[] beans = new SupportBean[array.size()];
        for (int i = 0; i < beans.length; i++) {
            JsonObject object = array.get(i).asObject();
            String theString = object.get("theString").isNull()
                ? null : object.get("theString").asString();
            beans[i] = new SupportBean(theString, object.get("intPrimitive").asInt());
        }
        return beans;
    }

    private static Map<String, Object> mappedEvent(JsonObject object) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (String name : object.names()) {
            map.put(name, object.get(name).asString());
        }
        return map;
    }

    private static Map<String, Object> namedMap(JsonObject object) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (String name : object.names()) {
            map.put(name, object.get(name).asInt());
        }
        return map;
    }

    private static Map[] namedMaps(JsonArray array) {
        Map[] maps = new Map[array.size()];
        for (int i = 0; i < maps.length; i++) {
            maps[i] = namedMap(array.get(i).asObject());
        }
        return maps;
    }

    // complexProps rebuilds SupportBeanComplexProps.makeDefaultBean() from the
    // pinned payload fields (objectArray stays null exactly like the Java
    // default bean).
    private static SupportBeanComplexProps complexProps(JsonObject object) {
        SupportBeanComplexProps bean = SupportBeanComplexProps.makeDefaultBean();
        bean.setSimpleProperty(object.getString("simpleProperty", null));
        Properties mapped = new Properties();
        for (String name : object.get("mapped").asObject().names()) {
            mapped.put(name, object.get("mapped").asObject().get(name).asString());
        }
        bean.setMappedProps(mapped);
        bean.setIndexedProps(intArray(object.get("indexed").asArray()));
        Map<String, String> mapProperty = new HashMap<>();
        for (String name : object.get("mapProperty").asObject().names()) {
            mapProperty.put(name, object.get("mapProperty").asObject().get(name).asString());
        }
        bean.setMapProperty(mapProperty);
        bean.setArrayProperty(intArray(object.get("arrayProperty").asArray()));
        JsonObject nested = object.get("nested").asObject();
        bean.setNested(new SupportBeanComplexProps.SupportBeanSpecialGetterNested(
            nested.getString("nestedValue", null),
            nested.get("nestedNested").asObject().getString("nestedNestedValue", null)));
        return bean;
    }

    // configuration mirrors the TestSuiteEventMap.configure registrations this
    // suite uses (regression-run/.../event/TestSuiteEventMap.java:71-212):
    // SupportBean class type plus the eight map types. Java registers
    // MyNamedMap/MyMapWithAMap twice with identical definitions; the second
    // registration is a no-op and is mirrored for fidelity.
    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
            RethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
            UndeployRethrowPolicy.RETHROW_FIRST);
        for (Class clazz : new Class[]{SupportBean.class}) {
            configuration.getCommon().addEventType(clazz);
        }

        // MyNamedMap={n0:int}
        Map<String, Object> namedDef = new HashMap<String, Object>();
        namedDef.put("n0", int.class);
        configuration.getCommon().addEventType("MyNamedMap", namedDef);

        // MyMapWithAMap={p0:"MyNamedMap",p1:"MyNamedMap[]"}
        Map<String, Object> eventDef = new HashMap<String, Object>();
        eventDef.put("p0", "MyNamedMap");
        eventDef.put("p1", "MyNamedMap[]");
        configuration.getCommon().addEventType("MyMapWithAMap", eventDef);

        // MyArrayMap={p0:int[],p1:SupportBean[]}
        Map<String, Object> arrayDef = new HashMap<String, Object>();
        arrayDef.put("p0", int[].class);
        arrayDef.put("p1", SupportBean[].class);
        configuration.getCommon().addEventType("MyArrayMap", arrayDef);

        // MyArrayMapOuter={outer:<inline MyArrayMap def>}
        Map<String, Object> arrayDefOuter = new HashMap<String, Object>();
        arrayDefOuter.put("outer", arrayDef);
        configuration.getCommon().addEventType("MyArrayMapOuter", arrayDefOuter);

        // MyMappedPropertyMap={p0:Map}
        Map<String, Object> mappedDef = new HashMap<String, Object>();
        mappedDef.put("p0", Map.class);
        configuration.getCommon().addEventType("MyMappedPropertyMap", mappedDef);

        // MyMappedPropertyMapOuter={outer:<inline MyMappedPropertyMap def>}
        Map<String, Object> mappedDefOuter = new HashMap<String, Object>();
        mappedDefOuter.put("outer", mappedDef);
        configuration.getCommon().addEventType("MyMappedPropertyMapOuter", mappedDefOuter);

        // MyMappedPropertyMapOuterTwo={outerTwo:SupportBeanComplexProps}
        Map<String, Object> mappedDefOuterTwo = new HashMap<String, Object>();
        mappedDefOuterTwo.put("outerTwo", SupportBeanComplexProps.class);
        configuration.getCommon().addEventType("MyMappedPropertyMapOuterTwo", mappedDefOuterTwo);

        // MyArrayMapTwo={outer:<inline MyMapWithAMap def>}
        Map<String, Object> myArrayMapTwo = new HashMap<String, Object>();
        myArrayMapTwo.put("outer", eventDef);
        configuration.getCommon().addEventType("MyArrayMapTwo", myArrayMapTwo);

        // TestSuiteEventMap registers the named-map pair twice identically.
        Map<String, Object> myNamedMap = new HashMap<String, Object>();
        myNamedMap.put("n0", int.class);
        configuration.getCommon().addEventType("MyNamedMap", myNamedMap);
        Map<String, Object> myMapWithAMap = new HashMap<String, Object>();
        myMapWithAMap.put("p0", "MyNamedMap");
        myMapWithAMap.put("p1", "MyNamedMap[]");
        configuration.getCommon().addEventType("MyMapWithAMap", myMapWithAMap);

        return configuration;
    }

    // validateScenario pins the step sequence per case: a case marker, then
    // (deploy, send, undeploy) triples with the byte-exact EPL, event type and
    // compact payload of the contract.
    private static void validateScenario(JsonObject scenario) {
        JsonArray steps = scenario.get("steps").asArray();
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            String caseName = CASES[caseIndex];
            JsonObject marker = stepAt(steps, offset);
            if (!"case".equals(string(marker, "op"))
                || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException(
                    "step " + offset + " is not the " + caseName + " case marker");
            }
            offset++;
            for (String[] pinned : CASE_STEPS[caseIndex]) {
                JsonObject deploy = stepAt(steps, offset);
                if (!"deploy".equals(string(deploy, "op"))
                    || !caseName.equals(string(deploy, "case"))
                    || !"s0".equals(string(deploy, "statement"))
                    || !pinned[0].equals(string(deploy, "epl"))) {
                    throw new IllegalArgumentException(
                        "deploy step " + offset + " is not pinned for " + caseName);
                }
                offset++;
                JsonObject send = stepAt(steps, offset);
                if (!"send".equals(string(send, "op"))
                    || !caseName.equals(string(send, "case"))
                    || !pinned[1].equals(string(send, "eventType"))
                    || !pinned[2].equals(object(send.get("payload"), "payload").toString())) {
                    throw new IllegalArgumentException(
                        "send step " + offset + " is not pinned for " + caseName);
                }
                offset++;
                JsonObject undeploy = stepAt(steps, offset);
                if (!"undeploy".equals(string(undeploy, "op"))
                    || !caseName.equals(string(undeploy, "case"))
                    || !"s0".equals(string(undeploy, "statement"))) {
                    throw new IllegalArgumentException(
                        "undeploy step " + offset + " is not pinned for " + caseName);
                }
                offset++;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException(
                "scenario has " + (steps.size() - offset) + " trailing steps");
        }
    }

    private static JsonObject stepAt(JsonArray steps, int index) {
        if (index >= steps.size()) {
            throw new IllegalArgumentException("scenario truncated at step " + index);
        }
        return steps.get(index).asObject();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a string");
        }
        return value.asString();
    }

    private static JsonObject object(JsonValue value, String name) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(name + " must be an object");
        }
        return value.asObject();
    }

    /**
     * Rethrow handler mirroring the regression environment: statement
     * exceptions rethrow on the sending thread instead of being absorbed by
     * the default handler.
     */
    public static class RethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return (handlerContext) -> {
                throw new RuntimeException(handlerContext.getThrowable());
            };
        }
    }
}
