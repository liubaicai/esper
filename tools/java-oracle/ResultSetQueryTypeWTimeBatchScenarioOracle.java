import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.ArrayList;
import java.util.List;
import java.util.TreeSet;

/**
 * Java oracle for ResultSetQueryTypeWTimeBatch: the eight time-batch query
 * executions of regression-lib/.../querytype/ResultSetQueryTypeWTimeBatch.java,
 * replayed in registration (ordinal) order. Each case owns one fresh runtime
 * with the internal timer disabled and an absolute advance-time driver
 * (advance 0 -> events -> 1000 -> event -> 2000). Batch-1 events are
 * DELL@10, IBM@15, DELL@20; batch-2 is IBM@20 volume 600.
 *
 * Executions: row-for-all nojoin/join (sum only, old=[] on flush1, old=[45]
 * on flush2), row-per-event nojoin/join (three new rows sharing the running
 * aggregate at flush1; three old rows carrying the post-flush aggregate at
 * flush2), row-per-group nojoin (+order by symbol asc; empty DELL group is
 * re-emitted with a null sum at flush2) / join (same values, set semantics),
 * and aggregate-grouped nojoin/join projecting the per-event volume whose
 * old rows preserve original volumes while sums may be null.
 *
 * Protocol: esper-parity/v1 records; doubles render as raw JSON numbers
 * via Double doubleValue; long volumes render as integers; null renders as
 * {"state":"null"}; listener records omit empty "new"/"old" arrays. The
 * runtime IDs come from testdata/compat/java-execution-inventory.jsonl rows
 * binding ordinals 0..7 to the eight ids listed in the scenario cases array.
 */
public class ResultSetQueryTypeWTimeBatchScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeWTimeBatchScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            runCase(caseVal.asObject(), allSteps, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonObject caseDef, JsonArray allSteps, List<JsonObject> records) throws Exception {
        String caseName = caseDef.getString("case", "");
        String runtimeId = caseDef.getString("runtimeId", "");
        if (runtimeId.isEmpty()) {
            throw new IllegalStateException("case " + caseName + " has no runtimeId");
        }

        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetQueryTypeWTimeBatchScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement s0 = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0 = candidate;
                    break;
                }
            }
            if (s0 == null) {
                throw new IllegalStateException("statement 's0' not found in case " + caseName);
            }
            int[] seq = new int[]{0};
            s0.addListener((newEvents, oldEvents, statement, rt) -> {
                boolean hasNew = newEvents != null && newEvents.length > 0;
                boolean hasOld = oldEvents != null && oldEvents.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("sequence", seq[0]);
                record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                if (hasNew) {
                    record.add("new", rows(newEvents));
                }
                if (hasOld) {
                    record.add("old", rows(oldEvents));
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
                switch (op) {
                    case "send" -> sendEvent(runtime, step, caseName);
                    case "advance-time" -> {
                        String at = step.getString("at", "");
                        if (at.isEmpty()) {
                            throw new IllegalStateException("advance-time needs 'at' in case " + caseName);
                        }
                        long millis = java.time.Instant.parse(at).toEpochMilli();
                        runtime.getEventService().advanceTime(millis);
                    }
                    default -> throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
                }
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportMarketDataBean" -> {
                JsonValue priceVal = payload.get("price");
                if (!(priceVal instanceof JsonNumber)) {
                    throw new IllegalStateException("SupportMarketDataBean payload needs numeric price in case " + caseName);
                }
                long volume = 0L;
                JsonValue volumeVal = payload.get("volume");
                if (volumeVal instanceof JsonNumber) {
                    volume = ((JsonNumber) volumeVal).asLong();
                }
                SupportMarketDataBean event = new SupportMarketDataBean(
                    payload.getString("symbol", null),
                    ((JsonNumber) priceVal).asDouble(),
                    volume,
                    null);
                runtime.getEventService().sendEventBean(event, "SupportMarketDataBean");
            }
            case "SupportBean" -> {
                SupportBean event = new SupportBean(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", -1));
                runtime.getEventService().sendEventBean(event, "SupportBean");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type + " in case " + caseName);
        }
    }

    /** Local mirror of the pinned SupportMarketDataBean regression bean. */
    public static class SupportMarketDataBean {
        private final String symbol;
        private final String id;
        private final double price;
        private final Long volume;
        private final String feed;

        public SupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.id = null;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() { return symbol; }
        public String getId() { return id; }
        public double getPrice() { return price; }
        public Long getVolume() { return volume; }
        public String getFeed() { return feed; }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "time-batch-row-for-all-nojoin" ->
                "@name('s0') select irstream sum(price) as sumPrice from SupportMarketDataBean#time_batch(1 sec)";
            case "time-batch-row-for-all-join" ->
                "@name('s0') select irstream sum(price) as sumPrice from " +
                    "SupportMarketDataBean#time_batch(1 sec) as s0, " +
                    "SupportBean#keepall as s1 where s0.symbol = s1.theString";
            case "time-batch-row-per-event-nojoin" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice from SupportMarketDataBean#time_batch(1 sec)";
            case "time-batch-row-per-event-join" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice from " +
                    "SupportMarketDataBean#time_batch(1 sec) as s0, " +
                    "SupportBean#keepall as s1 where s0.symbol = s1.theString";
            case "time-batch-row-per-group-nojoin" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice from " +
                    "SupportMarketDataBean#time_batch(1 sec) group by symbol order by symbol asc";
            case "time-batch-row-per-group-join" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice from " +
                    "SupportMarketDataBean#time_batch(1 sec) as s0, " +
                    "SupportBean#keepall as s1 where s0.symbol = s1.theString group by symbol";
            case "time-batch-aggr-grouped-nojoin" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice, volume from SupportMarketDataBean#time_batch(1 sec) group by symbol";
            case "time-batch-aggr-grouped-join" ->
                "@name('s0') select irstream symbol, sum(price) as sumPrice, volume from " +
                    "SupportMarketDataBean#time_batch(1 sec) as s0, " +
                    "SupportBean#keepall as s1 where s0.symbol = s1.theString group by symbol";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(event.get(prop)));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
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
        return Json.value(String.valueOf(value));
    }
}
