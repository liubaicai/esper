import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
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

/**
 * Java oracle for ViewTimeBatch time-batch view scenarios.
 *
 * Covers the eight implemented ViewTimeBatch executions replayed as eight
 * scenario cases, each case running against its own fresh runtime and
 * replaying its pinned module verbatim (only the @name('s0') annotations are
 * sanctioned additions): scene-one replays ViewTimeBatchSceneOne:
 * time_batch(1 sec) over SupportMarketDataBean flushing E1+E2 at the 2500
 * tick, new E3-E5 / old E1-E2 at 3500, old-only at 4500, and the E6/E7
 * single-row cadence at 6500/7500/8500. ten-sec replays ViewTimeBatch10Sec:
 * time_batch(10 sec) over SupportBean pushing E1+E2 at the 11000 tick, IR
 * pairs and old-only batches on the 21000/31000 cadence, quiet at 41000.
 * start-eager-force-update-scene-two replays
 * ViewTimeBatchStartEagerForceUpdateSceneTwo: time_batch(1 sec, "START_EAGER,
 * FORCE_UPDATE") over SupportMarketDataBean with empty FORCE_UPDATE flushes
 * at 1000/2000/5000, the E1+E2 batch at 3000 and its old counterpart at
 * 4000. month-scoped replays ViewTimeBatchMonthScoped: time_batch(1 month)
 * anchored at 2002-02-01 with each bean flushed exactly at its month
 * deadline (2002-03-01/04-01/05-01) and quiet at deadline-minus-1ms.
 * start-eager-force-update replays ViewTimeBatchStartEagerForceUpdate:
 * time_batch(1, "START_EAGER,FORCE_UPDATE") over SupportBean starting at
 * 1000 with empty flushes at 2000/3000/6000/7000, E1 new at 4000 and old at
 * 5000. multirow replays ViewTimeBatchMultirow: a four-bean first batch at
 * 11000 then the IR/old-only cadence. multi-batch replays
 * ViewTimeBatchMultiBatch: successive two-bean batches with IR pairs at
 * 11000/21000/31000/41000, old-only at 51000, quiet at 61000. no-ref-point
 * replays ViewTimeBatchNoRefPoint: a bare SupportBean (theString null) in
 * time_batch(10 minutes) flushing at exactly 600000. The FORCE_UPDATE
 * variants invoke the listener with empty batches at every flush; the record
 * protocol skips those invocations (new and old both empty), so they stay
 * silent in the trace.
 *
 * The ViewTimeBatchLonger execution is excluded from the chain because its
 * send schedule is driven by an unseeded java.util.Random, and the
 * ViewTimeBatchRefPoint execution is excluded because its reference-point
 * argument (time_batch(10 minutes, 10L)) is out of chain scope.
 *
 * Configuration follows the pinned regression schema: SupportMarketDataBean
 * is a map type {symbol string, price double, volume long, feed string}
 * (sends set symbol, price defaults 0.0, volume 0L, feed null, mirroring
 * makeMarketDataEvent); SupportBean is mirrored locally as {theString
 * string, intPrimitive int, doubleBoxed Double} because the pinned bean
 * lives outside the oracle classpath, theString being set unconditionally
 * from the payload including explicit JSON null for the bare bean of the
 * no-ref-point case. The internal timer is disabled and advanceTime(0) is
 * not issued at runtime creation because every case begins with its own
 * advance-time step.
 *
 * The single deployed step compiles buildEPL, deploys it plainly, fetches
 * the named statement and attaches the standard listener.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch containing rows, one sequence counter per case starting at 1, time
 * rendered from the current engine time, and new/old row arrays rendered
 * with the scalar normalization rules, where map values render as bare
 * JsonObject objects with TreeSet-sorted String.valueOf keys and recursively
 * normalized values. Snapshot records carry no time or sequence: step
 * {op:"snapshot", statement:"s0"} iterates the named statement and emits its
 * current window contents.
 */
public class ViewTimeBatchScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewTimeBatchScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently against its own runtime,
        // matching one runtime ID per execution.
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewTimeBatchScenarioOracle-" + caseName, config);
        try {
            EPStatement s0 = null;
            int[] seq = new int[] {0};

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
                if ("deployed".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                    s0 = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), statementName);
                    if (s0 == null) {
                        throw new IllegalStateException("statement " + statementName + " was not deployed for case " + caseName);
                    }
                    addListener(runtime, caseName, seq, records, s0);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    if (s0 == null) {
                        throw new IllegalStateException("snapshot requires statement s0 for case " + caseName);
                    }
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

    private static void addListener(EPRuntime runtime, String caseName, int[] seq, List<JsonObject> records,
                                    EPStatement statement) {
        statement.addListener((newData, oldData, stmt, rt) -> {
            boolean hasNew = newData != null && newData.length > 0;
            boolean hasOld = oldData != null && oldData.length > 0;
            if (!hasNew && !hasOld) {
                // FORCE_UPDATE flushes deliver empty batches; the record
                // protocol skips them so they stay silent in the trace
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
    }

    /** Verbatim transcriptions of the pinned ViewTimeBatch modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "scene-one" ->
                "@name('s0') select irstream * from SupportMarketDataBean#time_batch(1 sec)";
            case "ten-sec" ->
                "@Name('s0') select irstream * from SupportBean#time_batch(10 sec)";
            case "start-eager-force-update-scene-two" ->
                "@name('s0') select irstream symbol from SupportMarketDataBean#time_batch(1 sec, \"START_EAGER, FORCE_UPDATE\")";
            case "month-scoped" ->
                "@name('s0') select * from SupportBean#time_batch(1 month)";
            case "start-eager-force-update" ->
                "@name('s0') select irstream * from SupportBean#time_batch(1, \"START_EAGER,FORCE_UPDATE\")";
            case "multirow" ->
                "@Name('s0') select irstream * from SupportBean#time_batch(10 sec)";
            case "multi-batch" ->
                "@Name('s0') select irstream * from SupportBean#time_batch(10 sec)";
            case "no-ref-point" ->
                "@name('s0') select * from SupportBean#time_batch(10 minutes)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            JsonValue priceVal = payload.get("price");
            event.put("price", priceVal instanceof JsonNumber ? ((JsonNumber) priceVal).asDouble() : 0.0d);
            JsonValue volumeVal = payload.get("volume");
            event.put("volume", volumeVal instanceof JsonNumber ? ((JsonNumber) volumeVal).asLong() : 0L);
            event.put("feed", payload.getString("feed", null));
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            LocalSupportBean event = new LocalSupportBean();
            // theString is set unconditionally from the payload, including
            // explicit JSON null for the bare bean of the no-ref-point case
            JsonValue theStringVal = payload.get("theString");
            event.setTheString(theStringVal instanceof JsonString ? ((JsonString) theStringVal).asString() : null);
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            if (intPrimitiveVal instanceof JsonNumber) {
                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
            }
            JsonValue doubleBoxedVal = payload.get("doubleBoxed");
            if (doubleBoxedVal instanceof JsonNumber) {
                event.setDoubleBoxed(((JsonNumber) doubleBoxedVal).asDouble());
            }
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
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
            // covers map projections, which surface as bare JSON objects
            // rather than scalars
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
            // window-array projection values surface as arrays rather than
            // scalars
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the pinned SupportBean regression bean members in use. */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private Double doubleBoxed;

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }

        public Double getDoubleBoxed() {
            return doubleBoxed;
        }

        public void setDoubleBoxed(Double doubleBoxed) {
            this.doubleBoxed = doubleBoxed;
        }
    }
}
