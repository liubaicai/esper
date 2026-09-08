import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
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
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ViewGroup grouped-window scenarios (merge-view family).
 *
 * Covers six pinned executions of ViewGroup, each replayed as its own case
 * against a fresh runtime with the pinned module text transcribed verbatim
 * (only the byte-exact EPL text; no other annotations are added):
 *
 * merge-view-union-aggregate replays ViewGroupObjectArrayEvent (execution
 * ordinal 0, runtime java-runtime-a3b6bef89e22a122cc7a):
 * select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2),
 * sending A/10, B/11, A/12, A/13 and pinning the per-group Long sum
 * evolution 10, 21, 33, 36 through istream-only listener records.
 *
 * length-win-groups replays ViewGroupLengthWin (ordinal 14, runtime
 * java-runtime-639bc9b69621f3b6a417): select irstream theString as c0,
 * intPrimitive as c1 from SupportBean#groupwin(theString)#length(3),
 * sending E1/1, E2/20, E1/2, E2/21, E2/22, E1/3 (istream-only while the
 * per-group windows are not full), then E2/23 evicting E2/20 and E1/-1
 * evicting E1/1 as irstream pairs. The suite's iterator assertions here
 * use assertPropsPerRowIteratorAnyOrder, i.e. Java itself leaves
 * cross-group iteration order unpinned, so no snapshot steps are recorded
 * for this case.
 *
 * stats-four-views replays ViewGroupStats (ordinal 1, runtime
 * java-runtime-03ed11fd1e5c3a3d11c2) as a multi-module case: the suite
 * makes four separate compileDeploy calls, replayed here as four modules
 * deployed in order before any send — @name('priceLast3Stats') and
 * @name('volumeLast3Stats') over #groupwin(symbol)#length(3)#uni(...),
 * @name('priceAllStats') and @name('volumeAllStats') over
 * #groupwin(symbol)#uni(...), all select * with "order by symbol asc"
 * and the @name(...) annotation directly adjacent to the select keyword
 * as in the suite concatenation. The 14 sends use the suite helper's
 * SupportMarketDataBean(symbol, price, volume, "") shape. The oracle
 * attaches one listener per named statement (sequence numbering restarts
 * per statement) and records every delivered batch; because each send
 * updates exactly one symbol group and #uni posts istream-only, every
 * invocation carries exactly one new row, so each record IS the value the
 * suite pins with assertLastNewRow (its listener.getLastNewData() last
 * event) at sends 5, 7, 9, and 14, and for the other sends the record is
 * the same single-row observable the suite skips asserting. After send 14
 * two snapshot steps pin the suite's ordered assertPropsPerRowIterator
 * state of priceAllStats and priceLast3Stats (rows {symbol, average}
 * among the full select-* properties).
 *
 * correl-groups replays ViewGroupCorrel (ordinal 4, runtime
 * java-runtime-74e476ad16bf62d4a9e0): select * from
 * SupportMarketDataBean#groupwin(symbol)#length(1000000)#correl(price,
 * volume, feed), sending (ABC,10.0,1000,f1), (DEF,1.0,2,f2),
 * (DEF,2.0,4,f3), (ABC,20.0,2000,f4). Each record carries the full
 * select-* row; the Go-pinned surface is symbol/correlation/feed among
 * all fields, with correlation NaN for the two single-datapoint groups
 * rendered through the NaN marker convention.
 *
 * linest-groups replays ViewGroupLinest (ordinal 5, runtime
 * java-runtime-942d359f7bb1302684bb): select * from
 * SupportMarketDataBean#groupwin(symbol)#length(1000000)#linest(price,
 * volume, feed), sending (ABC,10.0,50000,f1), (DEF,1.0,1,f2),
 * (DEF,2.0,2,f3), (ABC,11.0,50100,f4). Full select-* rows again; the
 * Go-pinned surface is symbol/slope/YIntercept/feed among all fields,
 * with the two single-datapoint slope/YIntercept NaNs marked.
 *
 * multi-property-uni replays ViewGroupMultiProperty (ordinal 6, runtime
 * java-runtime-47877af7a1d114850723): select irstream datapoints as size,
 * symbol, feed, volume from SupportMarketDataBean#groupwin(symbol, feed,
 * volume)#uni(price) order by symbol, feed, volume. The suite helper
 * sendEvent(symbol, feed, volume) constructs SupportMarketDataBean with
 * price 0; the replayed sends are (GE,INFO,1) then (GE,INFO,1),
 * (GE,INFO,2), (GE,INFO,1) — where the suite calls listenerReset and
 * asserts nothing, the oracle records every delivered batch as the
 * established assertPropsNew convention — then (GE,REU,99) and
 * (MSFT,INFO,100), each delivering one irstream pair (new current
 * datapoints, old prior datapoints). The suite's final ordered
 * assertPropsPerRowIterator is pinned with a snapshot step.
 *
 * Suite milestones are harness ordering markers with no observable engine
 * output and record nothing. The OAEventStringInt event type mirrors the
 * pinned regression-run registration as an object-array type with property
 * names {p1, p2} and types {String, int}, sent through
 * sendEventObjectArray exactly like env.sendEventObjectArray.
 * SupportMarketDataBean is mirrored as a local bean class because the
 * fixed run script classpath excludes regression-lib; the mirror keeps the
 * pinned four-arg constructor (symbol, price, volume, feed), field types
 * (String, double, Long, String) and getters byte-equivalent, and the
 * unused id property does not participate in these scenarios. SupportBean
 * is the common-module class already on the classpath and is constructed
 * with the suite's two-arg (theString, intPrimitive) constructor.
 *
 * NaN convention: no prior oracle renders NaN, so normalize renders
 * Double.NaN and Float.NaN as the JSON object {"state":"nan"}, by
 * symmetry with the existing null marker {"state":"null"}; plain JSON,
 * accepted by jq and every JSON parser.
 *
 * Cross-statement record order: the Java runtime dispatches one event to
 * the stats-four-views statements in reverse deployment order
 * (volumeAllStats, priceAllStats, volumeLast3Stats, priceLast3Stats),
 * while the suite only ever asserts per-listener and never pins
 * cross-statement order. Listener records are therefore buffered per
 * send step and flushed in deployment order (the LinkedHashMap order)
 * at the send boundary, so the trace order is canonical; per-statement
 * sequences and all row values are unchanged. Snapshot steps flush
 * immediately as before.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch (not per row), sequence numbering per statement from 1, time
 * rendered from the current engine time, and new/old row arrays rendered
 * with the scalar normalization rules; a batch is skipped only when the
 * engine delivers neither new nor old rows. Snapshot records carry no
 * time or sequence: step {op:"snapshot", statement:"s0"} iterates the
 * named deployed statement and emits its current window contents.
 */
public class ViewGroupMergeViewScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewGroupMergeViewScenarioOracle <scenario.json>");
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
        // Mirrors the pinned regression-run registration of OAEventStringInt
        // as an object-array event type (TestSuiteView): names {p1, p2},
        // types {String, int}.
        config.getCommon().addEventType("OAEventStringInt",
            new String[] {"p1", "p2"}, new Object[] {String.class, int.class});
        // Local mirror class: regression-lib is outside the oracle classpath.
        config.getCommon().addEventType("SupportMarketDataBean", LocalSupportMarketDataBean.class);
        config.getCommon().addEventType("SupportBean", SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewGroupMergeViewScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            // Mirrors the suite's sequential compileDeploy calls: multi-module
            // cases (stats-four-views) deploy one module per suite call in
            // order; single-module cases deploy exactly the pinned module.
            Map<String, EPStatement> statementsByName = new LinkedHashMap<>();
            for (String module : eplModulesFor(caseName)) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement statement : deployment.getStatements()) {
                    statementsByName.put(statement.getName(), statement);
                }
            }
            if (!statementsByName.containsKey("s0") && !statementsByName.containsKey("priceLast3Stats")) {
                throw new IllegalStateException("no observable statement deployed for case " + caseName);
            }
            // One listener per named statement; the suite attaches listeners
            // to every statement it deploys before sending events. Per-
            // statement sequence counters keep each statement's pin unambiguous.
            // Deliveries buffer per statement and flush at the send boundary
            // so records appear in canonical deployment order.
            Map<String, List<JsonObject>> pending = new LinkedHashMap<>();
            for (EPStatement statement : statementsByName.values()) {
                int[] seq = new int[] {0};
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
                    record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                    if (hasNew) {
                        record.add("new", rows(newData));
                    }
                    if (hasOld) {
                        record.add("old", rows(oldData));
                    }
                    pending.computeIfAbsent(stmt.getName(), key -> new ArrayList<>()).add(record);
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
                    sendEvent(runtime, step);
                    // Flush this send's buffered deliveries in deployment
                    // order (the Java runtime dispatches to the four
                    // stats-four-views statements in reverse deployment
                    // order; the suite only asserts per-listener, so
                    // deployment order is the canonical record order).
                    // Single-statement cases are unaffected: one buffer,
                    // one flush, sequences unchanged.
                    flushPending(statementsByName, pending, records);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPStatement snapshotStatement = statementsByName.get(statementName);
                    if (snapshotStatement == null) {
                        throw new IllegalStateException(
                            "snapshot statement " + statementName + " not deployed for case " + caseName);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statementName);
                    record.add("new", rows(snapshotStatement.iterator()));
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /** Appends buffered per-send records in deployment order, draining them. */
    private static void flushPending(Map<String, EPStatement> statementsByName,
        Map<String, List<JsonObject>> pending, List<JsonObject> records) {
        for (String name : statementsByName.keySet()) {
            List<JsonObject> batch = pending.get(name);
            if (batch != null && !batch.isEmpty()) {
                records.addAll(batch);
                batch.clear();
            }
        }
    }

    /** Verbatim transcriptions of the pinned ViewGroup modules. */
    private static List<String> eplModulesFor(String caseName) {
        List<String> modules = new ArrayList<>();
        switch (caseName) {
            case "merge-view-union-aggregate" ->
                modules.add("@name('s0') select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2)");
            case "length-win-groups" ->
                modules.add("@Name('s0') select irstream theString as c0,intPrimitive as c1 from SupportBean#groupwin(theString)#length(3)");
            case "stats-four-views" -> {
                // Four separate compileDeploy calls in the suite; the filter
                // variable concatenates directly after @name('...').
                String filter = "select * from SupportMarketDataBean";
                modules.add("@name('priceLast3Stats')" + filter + "#groupwin(symbol)#length(3)#uni(price) order by symbol asc");
                modules.add("@name('volumeLast3Stats')" + filter + "#groupwin(symbol)#length(3)#uni(volume) order by symbol asc");
                modules.add("@name('priceAllStats')" + filter + "#groupwin(symbol)#uni(price) order by symbol asc");
                modules.add("@name('volumeAllStats')" + filter + "#groupwin(symbol)#uni(volume) order by symbol asc");
            }
            case "correl-groups" ->
                modules.add("@name('s0') select * from SupportMarketDataBean#groupwin(symbol)#length(1000000)#correl(price, volume, feed)");
            case "linest-groups" ->
                modules.add("@name('s0') select * from SupportMarketDataBean#groupwin(symbol)#length(1000000)#linest(price, volume, feed)");
            case "multi-property-uni" ->
                modules.add("@name('s0') select irstream datapoints as size, symbol, feed, volume " +
                    "from SupportMarketDataBean#groupwin(symbol, feed, volume)#uni(price) order by symbol, feed, volume");
            default -> throw new IllegalStateException("unknown case: " + caseName);
        }
        return modules;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("OAEventStringInt".equals(eventType)) {
            // Mirrors env.sendEventObjectArray(new Object[]{p1, p2}, "OAEventStringInt").
            runtime.getEventService().sendEventObjectArray(
                new Object[] {payload.getString("p1", null), payload.getInt("p2", 0)}, eventType);
            return;
        }
        if ("SupportMarketDataBean".equals(eventType)) {
            // Ord-1 sends mirror the suite helper sendEvent(env, symbol,
            // price, volume) with feed ""; ord-4/5 sends mirror the suite's
            // direct four-arg construction with feed f1..f4; ord-6 sends
            // mirror sendEvent(env, symbol, feed, volume) with price 0.
            JsonValue priceVal = payload.get("price");
            double price = priceVal instanceof JsonNumber ? ((JsonNumber) priceVal).asDouble() : 0.0d;
            JsonValue volumeVal = payload.get("volume");
            Long volume = volumeVal instanceof JsonNumber ? ((JsonNumber) volumeVal).asLong() : null;
            runtime.getEventService().sendEventBean(
                new LocalSupportMarketDataBean(payload.getString("symbol", null), price, volume,
                    payload.getString("feed", null)),
                eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            // Mirrors the suite helper sendSupportBean(env, theString, intPrimitive).
            runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
                eventType);
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
        if (value instanceof Double && ((Double) value).isNaN()
            || value instanceof Float && ((Float) value).isNaN()) {
            // NaN marker: {"state":"nan"}, symmetric with the null marker;
            // plain JSON that jq and every parser accept.
            JsonObject nanObj = new JsonObject();
            nanObj.add("state", "nan");
            return nanObj;
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

    /**
     * Local mirror of the pinned SupportMarketDataBean regression bean
     * (com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean),
     * reduced to the four members the ViewGroup scenarios use. Constructor
     * parameter order, field types, and getters match the pinned bean; the
     * unused id property does not participate in these scenarios.
     */
    public static class LocalSupportMarketDataBean {
        private final String symbol;
        private final double price;
        private final Long volume;
        private final String feed;

        public LocalSupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() {
            return symbol;
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
}
