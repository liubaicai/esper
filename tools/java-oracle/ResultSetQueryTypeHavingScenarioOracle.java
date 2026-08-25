import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ResultSetQueryTypeHaving avg-HAVING family scenarios.
 *
 * Covers the seven currently registered executions of ResultSetQueryTypeHaving (the StatementJoin and NoAggregationJoin having/where twins remain dormant branches pending the Go join-aggregate old/new classification fix; their buildEPL cases are present but not referenced by any fixture case). * (two entries, one semantic scenario: the text-model and object-model
 * runtime twins are behaviorally identical, so the oracle replays the same
 * EPL once per entry exactly like the variables-onset-set onset-array-at-index
 * dual-runtime case) compiles
 *
 *   select irstream symbol, price, avg(price) as avgPrice
 *   from SupportMarketDataBean#length(5)
 *   having price < avg(price)
 *
 * and replays seven DELL market sends exercising new-stream deliveries,
 * suppressed updates under strict less-than, expiry re-evaluation that fails
 * the HAVING predicate, and an old-only delivery when the leaving event
 * still satisfies it. having-statement-join seeds one DELL SupportBeanString
 * into SupportBeanString#length(100) and replays the same seven sends against
 * the two-stream twin joined over where one.theString = two.symbol.
 * having-wildcard-select filters a SupportBean#length_batch(2) window with
 * where intPrimitive&gt;0 and releases one two-row batch through having
 * count(*)=2. having-noagg-join-having and having-noagg-join-where are
 * behavioral twins replaying one SYM1/SYM2 spread sequence whose EPL differs
 * only in the having/where keyword gating Math.max-Math.min &gt;= 1.4.
 * having-substream-insert gates an insert into MyStream select quote.* on
 * avg(intPrimitive)&gt;=3. having-unbounded-sum and having-unbounded-sum-istream
 * aggregate an unbound pattern stream on sum(myEvent.intPrimitive)=2, where
 * only the irstream variant delivers synthesized old rows that pass the
 * post-state gate. having-sum-noagg-prop is compile-only: the mixed
 * aggregated/non-aggregated module (having volume &lt; avg(price)) deploys
 * successfully and emits a single "deployed" marker record with no sends
 * and no listener output.
 *
 * Events follow the pinned regression schema: SupportMarketDataBean is a
 * map type {symbol string, price double, volume long, feed string} whose
 * sends set symbol and price plus an optional volume (default 0, feed null;
 * the spread twins replay the pinned volume=-1 sends); SupportBean is a map
 * type {theString string, intPrimitive int}; SupportBeanString carries the
 * single theString member through a local mirror class because the pinned
 * beans live outside the oracle classpath.
 *
 * Listener records follow the variables_onset protocol: one record per
 * delivered batch containing rows, with sequence numbering per case from 1,
 * time rendered from the current engine time, and new/old row arrays
 * rendered with the scalar normalization rules (strings passthrough,
 * integral numbers as long, other numbers as double, null as
 * {"state":"null"}).
 */
public class ResultSetQueryTypeHavingScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeHavingScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently: a case name may appear once
        // per runtime ID (the dual-ID semantic-twin precedent), so duplicated
        // names replay the identical scenario per entry instead of being
        // deduplicated.
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
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", int.class);
        config.getCommon().addEventType("SupportBean", beanType);
        config.getCommon().addEventType("SupportBeanString", LocalSupportBeanString.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetQueryTypeHavingScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String module = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            int[] seq = new int[] {0};
            boolean observable = !"having-sum-noagg-prop".equals(caseName);
            boolean foundS0 = false;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    foundS0 = true;
                    if (observable) {
                        candidate.addListener((newData, oldData, statement, rt) -> {
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
                            record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                            record.add("sequence", seq[0]);
                            if (hasNew) {
                                record.add("new", rows(newData));
                            }
                            if (hasOld) {
                                record.add("old", rows(oldData));
                            }
                            records.add(record);
                        });
                    }
                }
            }
            if (!foundS0) {
                throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
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
                    continue;
                }
                if ("deployed".equals(op)) {
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "deployed");
                    record.add("statement", step.getString("statement", ""));
                    record.add("sequence", 0);
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            event.put("price", payload.get("price").asDouble());
            event.put("volume", payload.getLong("volume", 0L));
            event.put("feed", null);
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBeanString".equals(eventType)) {
            LocalSupportBeanString event = new LocalSupportBeanString(payload.getString("theString", null));
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    /** Verbatim transcriptions of the pinned ResultSetQueryTypeHaving modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "having-statement" ->
                "@name('s0') select irstream symbol, price, avg(price) as avgPrice " +
                    "from SupportMarketDataBean#length(5) " +
                    "having price < avg(price)";
            case "having-statement-join" ->
                "@name('s0') select irstream symbol, price, avg(price) as avgPrice " +
                    "from SupportBeanString#length(100) as one, " +
                    "SupportMarketDataBean#length(5) as two " +
                    "where one.theString = two.symbol " +
                    "having price < avg(price)";
            case "having-sum-noagg-prop" ->
                "@name('s0') select irstream symbol, price, avg(price) as avgPrice " +
                    "from SupportMarketDataBean#length(5) as two " +
                    "having volume < avg(price)";
            case "having-wildcard-select" ->
                "@name('s0') select * from SupportBean#length_batch(2) " +
                    "where intPrimitive>0 " +
                    "having count(*)=2";
            case "having-noagg-join-having" ->
                "@name('s0') select irstream a.price as aPrice, b.price as bPrice, " +
                    "Math.max(a.price,b.price)-Math.min(a.price,b.price) as spread " +
                    "from SupportMarketDataBean(symbol='SYM1')#length(1) as a, " +
                    "SupportMarketDataBean(symbol='SYM2')#length(1) as b " +
                    "having Math.max(a.price,b.price)-Math.min(a.price,b.price)>=1.4";
            case "having-noagg-join-where" ->
                "@name('s0') select irstream a.price as aPrice, b.price as bPrice, " +
                    "Math.max(a.price,b.price)-Math.min(a.price,b.price) as spread " +
                    "from SupportMarketDataBean(symbol='SYM1')#length(1) as a, " +
                    "SupportMarketDataBean(symbol='SYM2')#length(1) as b " +
                    "where Math.max(a.price,b.price)-Math.min(a.price,b.price)>=1.4";
            case "having-substream-insert" ->
                "@name('s0') insert into MyStream select quote.* " +
                    "from SupportBean#length(14) quote " +
                    "having avg(intPrimitive)>=3";
            case "having-unbounded-sum" ->
                "@name('s0') select irstream sum(myEvent.intPrimitive) as mysum " +
                    "from pattern [every myEvent=SupportBean] " +
                    "having sum(myEvent.intPrimitive)=2";
            case "having-unbounded-sum-istream" ->
                "@name('s0') select istream sum(myEvent.intPrimitive) as mysum " +
                    "from pattern [every myEvent=SupportBean] " +
                    "having sum(myEvent.intPrimitive)=2";
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

    /** Local mirror of the pinned SupportBeanString regression bean. */
    public static class LocalSupportBeanString {
        private String theString;

        public LocalSupportBeanString(String theString) {
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
