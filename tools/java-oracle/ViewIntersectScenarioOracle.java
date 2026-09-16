import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

import com.espertech.esper.common.client.EventBean;

/**
 * Scenario oracle for ViewIntersect ordinals 6/14/15/16/17/18.
 *
 * Each case deploys the pinned EPL verbatim (pattern producer+consumer,
 * grouped time+unique+sort, subselect membership, firstunique+firstlength
 * named window with on-delete, and the two named-window time+unique variants)
 * and replays send/advance-time/snapshot steps. Listener records follow the
 * standard protocol: one record per delivered batch, sequence counter per
 * case starting at 1, engine time rendered as Instant, and rows normalized
 * with the scalar rules. Snapshot records iterate the named statement.
 *
 * Event types are map-backed so the rendered row fields match the Go runner's
 * registered beans exactly: SupportBean {theString,intPrimitive,intBoxed},
 * SupportBean_S0 {id,p00}, SupportBean_S1 {id,p10}, SupportSensorEvent
 * {id,type,device,measurement,confidence}.
 */
public class ViewIntersectScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewIntersectScenarioOracle <scenario.json>");
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
        root.add("scenario", scenario.getString("description", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("intBoxed", Integer.class);
        config.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        config.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        config.getCommon().addEventType("SupportBean_S1", s1Type);
        Map<String, Object> sensorType = new HashMap<>();
        sensorType.put("id", Integer.class);
        sensorType.put("type", String.class);
        sensorType.put("device", String.class);
        sensorType.put("measurement", Double.class);
        sensorType.put("confidence", Double.class);
        config.getCommon().addEventType("SupportSensorEvent", sensorType);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewIntersectScenarioOracle-" + caseName, config);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            EPStatement s0 = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), "s0");
            if (s0 == null) {
                throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
            }
            int[] seq = new int[]{0};
            s0.addListener((newData, oldData, stmt, rt) -> {
                boolean hasNew = newData != null && newData.length > 0;
                boolean hasOld = oldData != null && oldData.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", stmt.getName());
                record.add("sequence", seq[0]);
                record.add("time", Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                if (hasNew) {
                    record.add("new", rows(newData));
                }
                if (hasOld) {
                    record.add("old", rows(oldData));
                }
                records.add(record);
            });

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
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                    continue;
                }
                if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    continue;
                }
                if ("snapshot".equals(op)) {
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", step.getString("statement", "s0"));
                    record.add("new", rows(s0.iterator()));
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }
        } finally {
            runtime.destroy();
        }
    }

    /** Verbatim transcriptions of the pinned ViewIntersect statements. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "pattern" ->
                "@name('s0') select irstream a.p00||b.p10 as theString from pattern [every a=SupportBean_S0 -> b=SupportBean_S1]#unique(a.id)#unique(b.id) retain-intersection";
            case "group-time-unique" ->
                "@name('s0') SELECT irstream * FROM SupportSensorEvent#groupwin(type)#time(1 hour)#unique(device)#sort(1, measurement desc) as high order by measurement asc";
            case "subselect" ->
                "@name('s0') select * from SupportBean_S0 where p00 in (select theString from SupportBean#length(2)#unique(intPrimitive) retain-intersection)";
            case "firstunique-length-ondelete" ->
                "create window MyWindowOne#firstunique(theString)#firstlength(3) as SupportBean;\n" +
                        "insert into MyWindowOne select * from SupportBean;\n" +
                        "on SupportBean_S0 delete from MyWindowOne where theString = p00;\n" +
                        "@name('s0') select irstream * from MyWindowOne";
            case "timewin-namedwindow" ->
                "@name('s0') create window MyWindowTwo#time(10 sec)#unique(intPrimitive) retain-intersection as select * from SupportBean;\n" +
                        "insert into MyWindowTwo select * from SupportBean;\n" +
                        "on SupportBean_S0 delete from MyWindowTwo where intBoxed = id;\n";
            case "timewin-namedwindow-delete" ->
                "@name('s0') create window MyWindowThree#time(10 sec)#unique(intPrimitive) retain-intersection as select * from SupportBean;\n" +
                        "insert into MyWindowThree select * from SupportBean\n;" +
                        "on SupportBean_S0 delete from MyWindowThree where intBoxed = id;\n";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        switch (eventType) {
            case "SupportBean" -> {
                event.put("theString", payload.getString("theString", null));
                JsonValue intVal = payload.get("intPrimitive");
                if (intVal instanceof JsonNumber) {
                    event.put("intPrimitive", ((JsonNumber) intVal).asInt());
                }
                JsonValue boxedVal = payload.get("intBoxed");
                if (boxedVal instanceof JsonNumber) {
                    event.put("intBoxed", ((JsonNumber) boxedVal).asInt());
                }
            }
            case "SupportBean_S0" -> {
                JsonValue idVal = payload.get("id");
                if (idVal instanceof JsonNumber) {
                    event.put("id", ((JsonNumber) idVal).asInt());
                }
                event.put("p00", payload.getString("p00", null));
            }
            case "SupportBean_S1" -> {
                JsonValue idVal = payload.get("id");
                if (idVal instanceof JsonNumber) {
                    event.put("id", ((JsonNumber) idVal).asInt());
                }
                event.put("p10", payload.getString("p10", null));
            }
            case "SupportSensorEvent" -> {
                JsonValue idVal = payload.get("id");
                if (idVal instanceof JsonNumber) {
                    event.put("id", ((JsonNumber) idVal).asInt());
                }
                event.put("type", payload.getString("type", null));
                event.put("device", payload.getString("device", null));
                JsonValue measurement = payload.get("measurement");
                if (measurement instanceof JsonNumber) {
                    event.put("measurement", ((JsonNumber) measurement).asDouble());
                }
                JsonValue confidence = payload.get("confidence");
                if (confidence instanceof JsonNumber) {
                    event.put("confidence", ((JsonNumber) confidence).asDouble());
                }
            }
            default -> throw new IllegalStateException("unknown eventType: " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonArray rows(Iterator<EventBean> iterator) {
        JsonArray array = new JsonArray();
        while (iterator.hasNext()) {
            array.add(row(iterator.next()));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
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
            JsonObject object = new JsonObject();
            for (String key : keys) {
                object.add(key, normalize(mapValue.get(key)));
            }
            return object;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }
}
