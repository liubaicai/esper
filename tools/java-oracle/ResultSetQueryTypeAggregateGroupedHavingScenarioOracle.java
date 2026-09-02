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
 * Java oracle for ResultSetQueryTypeAggregateGroupedHaving: grouped aggregates
 * with a having clause, replaying all four executions of
 * regression-lib/.../querytype/ResultSetQueryTypeAggregateGroupedHaving.java.
 *
 * Cases (one per execution): groupby-having-nojoin (select * from
 * SupportBean#length_batch(3) group by theString having count(*) &gt; 1 — the
 * flush emits one row per event of a passing group: [E2/20, E2/21]; the E1
 * group fails and is suppressed), groupby-having-join (select theString,
 * intPrimitive from SupportBean_S0#lastevent, SupportBean#length_batch(3)
 * group by theString having count(*) &gt; 1 — the S0 row only enables the
 * join; the same flush semantics), sum-one-view (select irstream symbol,
 * volume, sum(price) as mySum over SupportMarketDataBean#length(3) group by
 * symbol having sum(price) &gt;= 50 — pre-subtraction remove-stream old rows)
 * and sum-join (the same joined with SupportBeanString#length(100)).
 *
 * Protocol notes. Pinned env.milestone calls are no-ops in the pinned harness,
 * so scenario steps carry case/send markers only. Listener records follow the
 * shared protocol: one record per callback where either stream is non-empty,
 * "new"/"old" arrays emitted only for non-empty streams, sequence numbered
 * from 1 per case. SupportMarketDataBean mirrors the pinned regression bean
 * (volume Long, feed null); SupportBeanString is a local mirror.
 */
public class ResultSetQueryTypeAggregateGroupedHavingScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeAggregateGroupedHavingScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType("SupportBean_S0", SupportBeanS0.class);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetQueryTypeAggregateGroupedHavingScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement s0 = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0 = candidate;
                    break;
                }
            }
            if (s0 == null) {
                throw new IllegalStateException("statement s0 missing for case " + caseName);
            }

            int[] seq = new int[] {0};
            s0.addListener((newData, oldData, statement, rt) -> {
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
                            Long volume = null;
                            JsonValue volumeVal = payload.get("volume");
                            if (volumeVal instanceof JsonNumber) {
                                volume = ((JsonNumber) volumeVal).asLong();
                            }
                            String feed = null;
                            JsonValue feedVal = payload.get("feed");
                            if (feedVal instanceof JsonString) {
                                feed = ((JsonString) feedVal).asString();
                            }
                            String symbol = null;
                            JsonValue symbolVal = payload.get("symbol");
                            if (symbolVal instanceof JsonString) {
                                symbol = ((JsonString) symbolVal).asString();
                            }
                            SupportMarketDataBean event = new SupportMarketDataBean(
                                symbol,
                                ((JsonNumber) priceVal).asDouble(),
                                volume,
                                feed);
                            runtime.getEventService().sendEventBean(event, "SupportMarketDataBean");
                        }
                        case "SupportBeanString" -> {
                            SupportBeanString event = new SupportBeanString(payload.getString("theString", null));
                            runtime.getEventService().sendEventBean(event, "SupportBeanString");
                        }
                        case "SupportBean_S0" -> {
                            SupportBeanS0 event = new SupportBeanS0(payload.getInt("id", 0), null, null);
                            runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                        }
                        case "SupportBean" -> {
                            SupportBean event = new SupportBean();
                            event.setTheString(payload.getString("theString", null));
                            JsonValue intPrimitiveVal = payload.get("intPrimitive");
                            if (intPrimitiveVal instanceof JsonNumber) {
                                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean");
                        }
                        default -> throw new IllegalStateException("unknown eventType: " + type);
                    }
                } else {
                    throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
                }
            }
        } finally {
            runtime.destroy();
        }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "groupby-having-nojoin" -> "@name('s0') select * from SupportBean#length_batch(3) group by theString having count(*) > 1";
            case "groupby-having-join" -> "@name('s0') select theString, intPrimitive from SupportBean_S0#lastevent, SupportBean#length_batch(3) group by theString having count(*) > 1";
            case "sum-one-view" -> "@name('s0') select irstream symbol, volume, sum(price) as mySum " +
                "from SupportMarketDataBean#length(3) " +
                "where symbol='DELL' or symbol='IBM' or symbol='GE' " +
                "group by symbol " +
                "having sum(price) >= 50";
            case "sum-join" -> "@name('s0') select irstream symbol, volume, sum(price) as mySum " +
                "from SupportBeanString#length(100) as one, " +
                "SupportMarketDataBean#length(3) as two " +
                "where (symbol='DELL' or symbol='IBM' or symbol='GE') " +
                "  and one.theString = two.symbol " +
                "group by symbol " +
                "having sum(price) >= 50";
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
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        if (value instanceof EventBean[] eventArray) {
            JsonArray array = new JsonArray();
            for (EventBean inner : eventArray) {
                array.add(normalize(inner));
            }
            return array;
        }
        if (value != null && value.getClass().isArray() && !value.getClass().getComponentType().isPrimitive()) {
            Object[] objects = (Object[]) value;
            JsonArray array = new JsonArray();
            for (Object element : objects) {
                array.add(normalizeBean(element));
            }
            return array;
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

    private static JsonValue normalizeBean(Object bean) {
        if (bean == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        JsonObject fields = new JsonObject();
        for (java.lang.reflect.Method method : bean.getClass().getMethods()) {
            if (!method.getName().startsWith("get") && !method.getName().startsWith("is")) {
                continue;
            }
            if (method.getParameterCount() != 0 || method.getDeclaringClass() == Object.class) {
                continue;
            }
            String name = method.getName().startsWith("is")
                ? Character.toLowerCase(method.getName().charAt(2)) + method.getName().substring(3)
                : Character.toLowerCase(method.getName().charAt(3)) + method.getName().substring(4);
            try {
                fields.add(name, normalize(method.invoke(bean)));
            } catch (ReflectiveOperationException ex) {
                throw new IllegalStateException("cannot render property " + name, ex);
            }
        }
        JsonObject rowObj = new JsonObject();
        rowObj.add("kind", "row");
        rowObj.add("fields", fields);
        return rowObj;
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

    /** Local mirror of the pinned SupportBean_S0 regression bean. */
    public static class SupportBeanS0 {
        private final int id;
        private final String p00;
        private final String p01;

        public SupportBeanS0(int id, String p00, String p01) {
            this.id = id;
            this.p00 = p00;
            this.p01 = p01;
        }

        public int getId() {
            return id;
        }

        public String getP00() {
            return p00;
        }

        public String getP01() {
            return p01;
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
