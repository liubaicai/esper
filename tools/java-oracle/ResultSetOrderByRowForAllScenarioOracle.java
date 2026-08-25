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
 * Java oracle for ResultSetOrderByRowForAll: ungrouped row-for-all
 * aggregation combined with order-by, replaying all three executions of
 * regression-lib/.../orderby/ResultSetOrderByRowForAll.java in registration
 * order.
 *
 * Cases (one per execution). no-output-rate-join joins
 * SupportMarketDataBean#length(10) with SupportBeanString#length(100) on
 * symbol = theString under "select sum(price) as sumPrice ... order by
 * price"; the default (no output-rate clause) stream is istream-only with a
 * new row per matching market event, and the pinned suite asserts it solely
 * through the pull API, so the case declares observation "iterator": the
 * listener stays attached (mirroring env.addListener) but is never recorded,
 * and fifo snapshot steps read [{sumPrice:214.0}] after
 * CAT@50/IBM@49/CAT@15/IBM@100 and [{sumPrice:289.0}] after KGB@75.
 * output-default-no-join and output-default-join replay A1,E1(10),E2(11),E3
 * (12) against "irstream sum(intPrimitive) as c0, last(theString) as c1 from
 * SupportBean#length(2) [,SupportBean_A#keepall] output every 3 events order
 * by sum(intPrimitive) desc"; they declare observation "listener" because
 * the single IR batch is the observable: the SupportBean_A seed never
 * reaches the filtered stream (and in the join variant sits in #keepall
 * without advancing the event-count boundary), so no callback fires through
 * E2 and E3 delivers new=[{23,E3},{21,E2},{10,E1}] paired with the pre-state
 * chain old=[{21,E2},{10,E1},{null,null}].
 *
 * Protocol notes. Internal timer is disabled and each case pins
 * advanceTime(0), so every record carries epoch-0 time
 * ("1970-01-01T00:00:00Z"). Each case owns one sequence counter shared by
 * listener and snapshot records, starting at 1 in emission order. Listener
 * records appear only when either stream is non-empty and carry "new"/"old"
 * row arrays; snapshot records always carry the iterator rows under "new".
 * Scenario snapshot steps declare {statement,label,mode} where mode is
 * "fifo" (positional row comparison) or "any" (order-insensitive); the mode
 * governs downstream differential comparison only and is intentionally not
 * echoed into trace records. Runtime IDs follow
 * testdata/compat/java-execution-inventory.jsonl, whose name() column binds
 * ordinal 1 to ResultSetOutputDefault{join=false} (7642ad83057714f39501) and
 * ordinal 2 to ResultSetOutputDefault{join=true} (046ed3b9a90cf000c5c7).
 *
 * SupportMarketDataBean, SupportBeanString and SupportBean_A are local
 * mirrors of the pinned regression beans keeping the classpath
 * self-contained (only common/compiler/runtime target classes plus their
 * dependencies are on it); SupportBean comes from esper-common.
 */
public class ResultSetOrderByRowForAllScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetOrderByRowForAllScenarioOracle <scenario.json>");
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
        String observation = caseDef.getString("observation", "");
        if (!"iterator".equals(observation) && !"listener".equals(observation)) {
            throw new IllegalStateException("case " + caseName + " needs observation iterator|listener, got " + observation);
        }
        boolean recordListener = "listener".equals(observation);

        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        config.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        config.getCommon().addEventType("SupportBean_A", SupportBeanA.class);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetOrderByRowForAllScenarioOracle-" + caseName, config);
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
                if ((!hasNew && !hasOld) || !recordListener) {
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
                if ("send".equals(op)) {
                    sendEvent(runtime, step, caseName);
                } else if ("snapshot".equals(op)) {
                    if (recordListener) {
                        throw new IllegalStateException("snapshot step is not allowed in listener-observed case " + caseName);
                    }
                    String statementName = step.getString("statement", "");
                    if (!"s0".equals(statementName)) {
                        throw new IllegalStateException("unknown snapshot statement " + statementName + " in case " + caseName);
                    }
                    if (step.getString("label", "").isEmpty()) {
                        throw new IllegalStateException("snapshot step needs a label in case " + caseName);
                    }
                    String mode = step.getString("mode", "");
                    if (!"fifo".equals(mode) && !"any".equals(mode)) {
                        throw new IllegalStateException("snapshot step needs mode fifo|any in case " + caseName);
                    }
                    List<EventBean> snapshotRows = new ArrayList<>();
                    for (java.util.Iterator<EventBean> it = s0.iterator(); it.hasNext(); ) {
                        snapshotRows.add(it.next());
                    }
                    seq[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statementName);
                    record.add("sequence", seq[0]);
                    record.add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                    record.add("new", rows(snapshotRows.toArray(new EventBean[0])));
                    records.add(record);
                } else {
                    throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
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
                String feed = null;
                JsonValue feedVal = payload.get("feed");
                if (feedVal instanceof JsonString) {
                    feed = ((JsonString) feedVal).asString();
                }
                SupportMarketDataBean event = new SupportMarketDataBean(
                    payload.getString("symbol", null),
                    ((JsonNumber) priceVal).asDouble(),
                    volume,
                    feed);
                runtime.getEventService().sendEventBean(event, "SupportMarketDataBean");
            }
            case "SupportBeanString" -> {
                SupportBeanString event = new SupportBeanString(payload.getString("theString", null));
                runtime.getEventService().sendEventBean(event, "SupportBeanString");
            }
            case "SupportBean_A" -> {
                SupportBeanA event = new SupportBeanA(payload.getString("id", null));
                runtime.getEventService().sendEventBean(event, "SupportBean_A");
            }
            case "SupportBean" -> {
                SupportBean event = new SupportBean();
                JsonValue theStringVal = payload.get("theString");
                if (theStringVal instanceof JsonString) {
                    event.setTheString(((JsonString) theStringVal).asString());
                }
                JsonValue intPrimitiveVal = payload.get("intPrimitive");
                if (intPrimitiveVal instanceof JsonNumber) {
                    event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                }
                runtime.getEventService().sendEventBean(event, "SupportBean");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type + " in case " + caseName);
        }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "no-output-rate-join" ->
                "@name('s0')select sum(price) as sumPrice from " +
                    "SupportMarketDataBean#length(10) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "order by price";
            case "output-default-no-join" ->
                "@name('s0') select irstream sum(intPrimitive) as c0, last(theString) as c1 from " +
                    "SupportBean#length(2) " +
                    "output every 3 events order by sum(intPrimitive) desc";
            case "output-default-join" ->
                "@name('s0') select irstream sum(intPrimitive) as c0, last(theString) as c1 from " +
                    "SupportBean#length(2) ,SupportBean_A#keepall " +
                    "output every 3 events order by sum(intPrimitive) desc";
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
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
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
            // The pinned suite never sets id (its sendEvent helper leaves the
            // regression bean's id at its default null).
            this.id = null;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() {
            return symbol;
        }

        public String getId() {
            return id;
        }

        public double getPrice() {
            return price;
        }

        public Long getVolume() {
            return volume;
        }

        public String getFeed() {
            return feed;
        }
    }

    /** Local mirror of the pinned SupportBeanString regression bean. */
    public static class SupportBeanString {
        private String theString;

        public SupportBeanString() {
        }

        public SupportBeanString(String theString) {
            this.theString = theString;
        }

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }
    }

    /** Local mirror of the pinned SupportBean_A regression bean. */
    public static class SupportBeanA {
        private final String id;

        public SupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }
}
