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
import com.espertech.esper.runtime.client.UpdateListener;

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
 * Java oracle for ResultSetQueryTypeAggregateGrouped: grouped aggregation
 * replaying all nine registered executions of regression-lib/.../querytype/
 * ResultSetQueryTypeAggregateGrouped.java in inventory ordinal order
 * (testdata/compat/java-execution-inventory.jsonl, outer class
 * ResultSetQueryTypeAggregateGrouped):
 *
 * Ordinal 0 criteria-by-dot-method (java-runtime-de8f7d7c8aef4f94257f)
 * groups SupportBean#length_batch(2) by sb.getTheString(); the second
 * E1/20/200L send flushes the batch window delivering one new-only callback
 * carrying both group rows in window insertion order
 * [{c0:100,c1:30},{c0:200,c1:30}] with no old data.
 *
 * Ordinal 1 iterate-unbound (java-runtime-f158e09462cf81dcaed6) declares
 * observation "iterator": @IterableUnbound keeps the un-viewed stream
 * iterable while the pinned listener stays attached but is never asserted,
 * so listener recording is suppressed and any-order snapshot steps read the
 * running per-symbol sums [{E1,10}], [{E1,10},{E2,20}],
 * [{E1,21},{E2,20}].
 *
 * Ordinal 2 unaggregated-having (java-runtime-53a0852371cfa557bb19) filters
 * groups through having intPrimitive > 5; E1/3 and E2/5 (=5) stay silent,
 * then E1/6 and E3/7 each deliver a single-row new batch.
 *
 * Ordinal 3 wildcard-min (java-runtime-40a398cbeadf315e6404) projects
 * select *, min(intPrimitive) as minval over SupportBean#length(2) grouped
 * by theString; rows enumerate the full flattened wildcard surface plus the
 * minval column following the sibling oracle rendering conventions.
 *
 * Ordinal 4 aggregation-over-grouped-props (java-runtime-
 * 2b8ffb9e25212f96d12c) replays ESPER-185: irstream
 * volume,symbol,price,count(price) over SupportMarketDataBean#length(5)
 * group by symbol, price. Every send is followed by a fifo snapshot; rows
 * follow window insertion order of the group member events, so the
 * (IBM,5.0) group contributes one row per remaining member while count
 * shows the shared group aggregate.
 *
 * Ordinals 5 sum-one-view (java-runtime-0dd2d8188c6705b7e352) and
 * 6 sum-join (java-runtime-0b86c8778cda88c48804) share tryAssertionSum:
 * irstream symbol, volume, sum(price) over SupportMarketDataBean#length(3)
 * filtered to DELL/IBM/GE, grouped by symbol; the join variant seeds
 * SupportBeanString DELL and IBM first (no output) and takes its initial
 * empty any-order snapshot after the seeds. Evictions produce cross-group
 * old rows (the leaving event's group survives with a reduced sum), e.g.
 * new {IBM,10000,90.0} paired with old {DELL,10000,52.0}.
 *
 * Ordinal 7 insert-into (java-runtime-f5ae7195e04ce1676ec4) starts with s0
 * (avg(price)/sum(volume) over #length(3000)), sends IBM/10/20000, then a
 * mid-case deploy step compiles one module holding s1 (@name('s1') insert
 * into StockAverages ...) together with s2 (select * from StockAverages),
 * mirroring env.compileDeploy(stmt).addListener("s1").addListener("s2"):
 * s1 keeps a real listener attached (mirroring the pinned suite, which never
 * asserts it) but stays unrecorded, s0 and s2 record.
 *
 * Ordinal 8 multikey-w-array (java-runtime-91048f4568185e6225f9) groups
 * sum(value) by the int[] array property using content-equal multikey
 * semantics; every send delivers one new row {id,thesum}.
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
 * echoed into trace records. Cases declare observation "iterator" when the
 * pinned suite asserts exclusively through the pull API (listener attached,
 * never recorded) and "listener" otherwise; listener-observed cases may
 * still interleave snapshot steps.
 *
 * SupportMarketDataBean, SupportBeanString and SupportEventWithIntArray are
 * local mirrors of the pinned regression beans keeping the classpath
 * self-contained (only common/compiler/runtime target classes plus their
 * dependencies are on it); SupportBean comes from esper-common.
 */
public class ResultSetQueryTypeAggregateGroupedScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "resultset-querytype-aggregate-grouped";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String INSERT_INTO_MODULE = "@name('s1') insert into StockAverages select symbol as symbol, avg(price) as average, sum(volume) as sumation " +
            "from SupportMarketDataBean#length(3000);\n" +
            "@name('s2') select * from StockAverages";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeAggregateGroupedScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalStateException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalStateException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            runCase(caseVal.asObject(), allSteps, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", SCENARIO_ID);
        root.add("javaCommit", PINNED_COMMIT);
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
        config.getCommon().addEventType("SupportEventWithIntArray", SupportEventWithIntArray.class);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetQueryTypeAggregateGroupedScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(initialEPL(caseName),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseName));
            Map<String, EPStatement> statements = new HashMap<>();
            for (EPStatement candidate : deployment.getStatements()) {
                statements.put(candidate.getName(), candidate);
            }
            EPStatement s0 = statements.get("s0");
            if (s0 == null) {
                throw new IllegalStateException("statement 's0' not found in case " + caseName);
            }
            int[] seq = new int[]{0};
            s0.addListener(new RecordingListener(records, caseName, s0, runtime, seq, recordListener));

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
                    takeSnapshot(runtime, records, seq, step, caseName, statements);
                } else if ("deploy".equals(op)) {
                    deployModule(runtime, records, seq, step, caseName, statements, recordListener);
                } else {
                    throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
                }
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void takeSnapshot(EPRuntime runtime, List<JsonObject> records, int[] seq, JsonObject step,
                                     String caseName, Map<String, EPStatement> statements) {
        String statementName = step.getString("statement", "");
        EPStatement statement = statements.get(statementName);
        if (statement == null) {
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
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            snapshotRows.add(it.next());
        }
        seq[0]++;
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statementName);
        record.add("sequence", seq[0]);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("new", rows(snapshotRows.toArray(new EventBean[0])));
        records.add(record);
    }

    private static void deployModule(EPRuntime runtime, List<JsonObject> records, int[] seq, JsonObject step,
                                     String caseName, Map<String, EPStatement> statements, boolean recordListener) throws Exception {
        String module = step.getString("statement", "");
        if (!"s1+s2".equals(module)) {
            throw new IllegalStateException("unsupported deploy step " + module + " in case " + caseName);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(INSERT_INTO_MODULE,
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseName + "-mid"));
        for (EPStatement candidate : deployment.getStatements()) {
            statements.put(candidate.getName(), candidate);
            if ("s1".equals(candidate.getName())) {
                // Mirrors env.addListener("s1"): attached in the pinned suite
                // but never asserted, hence intentionally unrecorded.
                candidate.addListener((newEvents, oldEvents, statement, rt) -> { });
            } else if ("s2".equals(candidate.getName())) {
                candidate.addListener(new RecordingListener(records, caseName, candidate, runtime, seq, recordListener));
            }
        }
    }

    private static String initialEPL(String caseName) {
        switch (caseName) {
            case "criteria-by-dot-method":
                return "@name('s0') select sb.getLongPrimitive() as c0, sum(intPrimitive) as c1 " +
                        "from SupportBean#length_batch(2) as sb group by sb.getTheString()";
            case "iterate-unbound":
                return "@name('s0') @IterableUnbound select theString as c0, sum(intPrimitive) as c1 " +
                        "from SupportBean group by theString";
            case "unaggregated-having":
                return "@name('s0') select theString from SupportBean group by theString having intPrimitive > 5";
            case "wildcard-min":
                return "@name('s0') select *, min(intPrimitive) as minval from SupportBean#length(2) group by theString";
            case "aggregation-over-grouped-props":
                return "@name('s0') select irstream volume,symbol,price,count(price) as mycount " +
                        "from SupportMarketDataBean#length(5) group by symbol, price";
            case "sum-one-view":
                return "@name('s0') select irstream symbol, volume, sum(price) as mySum " +
                        "from SupportMarketDataBean#length(3) " +
                        "where symbol='DELL' or symbol='IBM' or symbol='GE' group by symbol";
            case "sum-join":
                return "@name('s0') select irstream symbol, volume, sum(price) as mySum " +
                        "from SupportBeanString#length(100) as one, " +
                        "SupportMarketDataBean#length(3) as two " +
                        "where (symbol='DELL' or symbol='IBM' or symbol='GE') " +
                        "  and one.theString = two.symbol " +
                        "group by symbol";
            case "insert-into":
                return "@name('s0') select symbol as symbol, avg(price) as average, sum(volume) as sumation " +
                        "from SupportMarketDataBean#length(3000)";
            case "multikey-w-array":
                return "@name('s0') select id, sum(value) as thesum from SupportEventWithIntArray group by array";
            default:
                throw new IllegalStateException("unknown case: " + caseName);
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
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
                JsonValue longPrimitiveVal = payload.get("longPrimitive");
                if (longPrimitiveVal instanceof JsonNumber) {
                    event.setLongPrimitive(((JsonNumber) longPrimitiveVal).asLong());
                }
                runtime.getEventService().sendEventBean(event, "SupportBean");
            }
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
            case "SupportEventWithIntArray" -> {
                JsonArray arrayVal = payload.get("array").asArray();
                int[] array = new int[arrayVal.size()];
                for (int i = 0; i < array.length; i++) {
                    array[i] = arrayVal.get(i).asInt();
                }
                SupportEventWithIntArray event = new SupportEventWithIntArray(
                        payload.getString("id", null),
                        array,
                        payload.getInt("value", 0));
                runtime.getEventService().sendEventBean(event, "SupportEventWithIntArray");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type + " in case " + caseName);
        }
    }

    private static final class RecordingListener implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int[] seq;
        private final boolean record;

        private RecordingListener(List<JsonObject> records, String caseName, EPStatement statement,
                                  EPRuntime runtime, int[] seq, boolean record) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.seq = seq;
            this.record = record;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if ((!hasNew && !hasOld) || !record) {
                return;
            }
            seq[0]++;
            JsonObject rec = new JsonObject();
            rec.add("case", caseName);
            rec.add("operation", "listener");
            rec.add("statement", statement.getName());
            rec.add("sequence", seq[0]);
            rec.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                rec.add("new", rows(newEvents));
            }
            if (hasOld) {
                rec.add("old", rows(oldEvents));
            }
            records.add(rec);
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
        if (value instanceof int[]) {
            JsonArray array = new JsonArray();
            for (int item : (int[]) value) {
                array.add(item);
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

    /** Local mirror of the pinned SupportEventWithIntArray regression bean. */
    public static class SupportEventWithIntArray {
        private final String id;
        private final int[] array;
        private final int value;

        public SupportEventWithIntArray(String id, int[] array, int value) {
            this.id = id;
            this.array = array;
            this.value = value;
        }

        public String getId() {
            return id;
        }

        public int[] getArray() {
            return array;
        }

        public int getValue() {
            return value;
        }
    }
}
