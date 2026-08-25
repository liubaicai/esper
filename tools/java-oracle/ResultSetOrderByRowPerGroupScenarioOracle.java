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
 * Java oracle for ResultSetOrderByRowPerGroup: grouped aggregates with
 * order-by over row-per-group output, replaying all nine executions of
 * regression-lib/.../orderby/ResultSetOrderByRowPerGroup.java.
 *
 * Cases (one per execution): no-having-no-join and having-no-join
 * (#length(20) group by symbol, output every 6 events, order by sum(price),
 * symbol; the having variant filters both streams), no-having-join,
 * having-join and having-join-alias (the same pair joined with
 * SupportBeanString#length(100) on symbol = theString, the alias variant
 * ordering by the mysum alias instead of the aggregate expression), last and
 * last-join (output last every 6 events, old stream carrying the previous
 * per-group sums with nulls on first emission), iterator-row-per-group
 * (continuous join over #length(10) observed through iterator snapshots) and
 * order-by-last (SupportBean#length_batch(5) group by theString ordered desc
 * by last(intPrimitive), flushed as one new-only batch).
 *
 * Protocol notes. Pinned env.milestone/milestoneInc calls are no-ops in the
 * pinned harness (RegressionEnvironmentEsper.milestone returns this without
 * touching statements), so scenario steps carry case/send/snapshot markers
 * only and milestone positions collapse away. Listener records keep the
 * WithOldStream protocol: one record per callback where either stream is
 * non-empty, "new"/"old" arrays emitted only for non-empty streams, sequence
 * numbered from 1 in callback order. The iterator execution uses snapshot
 * records (operation "snapshot"); rows appear in statement-iterator order,
 * which for this EPL applies the order-by symbol clause (symbol-ascending).
 * Join cases seed SupportBeanString rows via send steps exactly as the
 * pinned executions do. Market sends mirror the
 * pinned helper: volume 0L and feed null when a payload omits them.
 *
 * SupportMarketDataBean and SupportBeanString are local mirrors of the pinned
 * regression beans keeping the classpath self-contained (only common/compiler/
 * runtime target classes plus their dependencies are on it); SupportBean comes
 * from esper-common.
 */
public class ResultSetOrderByRowPerGroupScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetOrderByRowPerGroupScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        config.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetOrderByRowPerGroupScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            StringBuilder moduleText = new StringBuilder();
            for (int i = 0; i < epls.length; i++) {
                if (i > 0) {
                    moduleText.append(';');
                }
                moduleText.append(epls[i]);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(moduleText.toString(), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            List<EPStatement> s0Statements = new ArrayList<>();
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0Statements.add(candidate);
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
                stmt.addListener((newData, oldData, statement, rt) -> {
                    boolean hasNew = newData != null && newData.length > 0;
                    boolean hasOld = oldData != null && oldData.length > 0;
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
                        record.add("new", rows(newData));
                    }
                    if (hasOld) {
                        record.add("old", rows(oldData));
                    }
                    records.add(record);
                });
            }

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
                    String type = step.getString("eventType", "");
                    JsonObject payload = step.get("payload").asObject();
                    switch (type) {
                        case "SupportMarketDataBean" -> {
                            JsonValue priceVal = payload.get("price");
                            if (!(priceVal instanceof JsonNumber)) {
                                throw new IllegalStateException("SupportMarketDataBean payload needs numeric price");
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
                        default -> throw new IllegalStateException("unknown eventType: " + type);
                    }
                } else if ("snapshot".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPStatement target = null;
                    for (EPStatement candidate : s0Statements) {
                        if (candidate.getName().equals(statementName)) {
                            target = candidate;
                            break;
                        }
                    }
                    if (target == null) {
                        throw new IllegalStateException("unknown snapshot statement " + statementName + " in case " + caseName);
                    }
                    List<EventBean> snapshotRows = new ArrayList<>();
                    for (java.util.Iterator<EventBean> it = target.iterator(); it.hasNext(); ) {
                        snapshotRows.add(it.next());
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statementName);
                    record.add("sequence", 0);
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

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "no-having-no-join" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) " +
                    "group by symbol " +
                    "output every 6 events " +
                    "order by sum(price), symbol"
            };
            case "having-no-join" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) " +
                    "group by symbol " +
                    "having sum(price) > 0 " +
                    "output every 6 events " +
                    "order by sum(price), symbol"
            };
            case "no-having-join" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "group by symbol " +
                    "output every 6 events " +
                    "order by sum(price), symbol"
            };
            case "having-join" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "group by symbol " +
                    "having sum(price) > 0 " +
                    "output every 6 events " +
                    "order by sum(price), symbol"
            };
            case "having-join-alias" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "group by symbol " +
                    "having sum(price) > 0 " +
                    "output every 6 events " +
                    "order by mysum, symbol"
            };
            case "last" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) " +
                    "group by symbol " +
                    "output last every 6 events " +
                    "order by sum(price), symbol"
            };
            case "last-join" -> new String[]{
                "@name('s0') select irstream symbol, sum(price) as mysum from " +
                    "SupportMarketDataBean#length(20) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "group by symbol " +
                    "output last every 6 events " +
                    "order by sum(price), symbol"
            };
            case "iterator-row-per-group" -> new String[]{
                "@name('s0') select symbol, sum(price) as sumPrice from " +
                    "SupportMarketDataBean#length(10) as one, " +
                    "SupportBeanString#length(100) as two " +
                    "where one.symbol = two.theString " +
                    "group by symbol " +
                    "order by symbol"
            };
            case "order-by-last" -> new String[]{
                "@name('s0') select last(intPrimitive) as c0, theString as c1  " +
                    "from SupportBean#length_batch(5) group by theString order by last(intPrimitive) desc"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
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
}
