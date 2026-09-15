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
 * Java oracle for ViewFirstTime first-time-window view scenarios.
 *
 * Covers all three ViewFirstTime executions replayed as three scenario cases,
 * each case running against its own fresh runtime and replaying its pinned
 * module verbatim (only the @name('s0') annotations are sanctioned
 * additions): first-time-simple replays ViewFirstTimeSimple: the
 * firsttime(1 month) window anchored by the calendar-month advance to
 * 2002-02-01T09:00:00.000 admits E1 at 2002-02-15 and E2 at the
 * deadline-minus-1ms tick (2002-03-01T09:00:00.000 minus 1 ms), while E3 sent
 * at the deadline tick itself is never admitted and the iterator still shows
 * E1 and E2. first-time-scene-one replays ViewFirstTimeSceneOne:
 * firsttime(10 sec) over SupportBean projecting c0/c1 admits E1 and E2 during
 * the window, the deadline expiry at the 10000 tick stays silent (no old
 * stream rows), and E3 at the deadline tick plus E4 afterwards are never
 * admitted, the iterator keeping E1 and E2. first-time-scene-two replays
 * ViewFirstTimeSceneTwo: firsttime(1 sec) over SupportMarketDataBean admits
 * E1 at 500 ms and E2 at 600 ms, the 1500 ms deadline expiry is silent as
 * pinned by assertListenerNotInvoked, and E3 at 1600 ms and E4 at 2000 ms
 * after the deadline are never admitted.
 *
 * Configuration follows the pinned regression schema: SupportMarketDataBean
 * is a map type {symbol string, price double, volume long, feed string}
 * (sends set symbol, price defaults 0.0, volume 0L, feed null, mirroring
 * makeMarketDataEvent); SupportBean is mirrored locally as {theString
 * string, intPrimitive int, doubleBoxed Double} because the pinned bean
 * lives outside the oracle classpath. The internal timer is disabled and
 * advanceTime(0) is not issued at runtime creation because every case begins
 * with its own advance-time step.
 *
 * Each case deploys exactly one statement: the deployed step compiles
 * buildEPL(caseName), deploys it plainly, fetches the named statement and
 * attaches the standard listener.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch containing rows, one sequence counter per case starting at 1, time
 * rendered from the current engine time, and new/old row arrays rendered
 * with the scalar normalization rules. Snapshot records carry no time or
 * sequence: step {op:"snapshot", statement:"s0"} iterates the named
 * statement and emits its current window contents.
 */
public class ViewFirstTimeScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewFirstTimeScenarioOracle <scenario.json>");
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
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewFirstTimeScenarioOracle-" + caseName, config);
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

    /** Verbatim transcriptions of the pinned ViewFirstTime modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "first-time-simple" ->
                "@name('s0') select * from SupportBean#firsttime(1 month)";
            case "first-time-scene-one" ->
                "@Name('s0') select irstream theString as c0, intPrimitive as c1 from SupportBean#firsttime(10 sec)";
            case "first-time-scene-two" ->
                "@name('s0') select irstream * from SupportMarketDataBean#firsttime(1 sec)";
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
            JsonValue theStringVal = payload.get("theString");
            if (theStringVal instanceof JsonString) {
                event.setTheString(((JsonString) theStringVal).asString());
            }
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
