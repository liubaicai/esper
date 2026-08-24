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
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the ViewUnique parity scenarios (pinned Esper 9.0.0 commit
 * 9e1b9f1cc9117fea4bf33ab043762c045d73839c). Replays all five executions of
 * regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/
 * ViewUnique.java, one scenario case per execution against its own runtime,
 * mirroring the replacement semantics of #unique(expr) and
 * #unique(expr1, expr2) views with irstream selection: a fresh key delivers a
 * new-only row pair (no old data) and a duplicate key delivers new+old where
 * old is the previous row for that key.
 *
 * Cases (verbatim module transcriptions):
 * scene-one replays ViewLastUniqueSceneOne: select irstream symbol, price
 * from SupportMarketDataBean#unique(symbol) order by symbol; S1/100 and S2/5
 * arrive new-only, S1/101 and S1/102 evict the previous S1 row, the ordered
 * iterator holds [{102.0},{5.0}] after the fourth send, and S2/6 evicts
 * {S2,5}. scene-two replays ViewLastUniqueSceneTwo with the two-key window
 * #unique(symbol, feed) ordered by symbol, feed: (S1,F1) prices 100 then 101
 * and (S2,F1) prices 5 then 102 form eviction pairs, (S1,F2,6) is a fresh
 * second key delivered new-only, and iterators hold {101.0,102.0} then
 * {101.0,6.0,102.0}. annotation-prefix replays
 * ViewLastUniqueWithAnnotationPrefix exactly as registered by executions()
 * (null annotation prefix): c0/c1 projection of SupportBean#unique(theString)
 * with empty-window iterator at milestone 1, new-only deliveries for E1/1,
 * E2/20 and E3/30, eviction pairs E1/2 vs E1/1, E2/21 vs E2/20, E2/22 vs
 * E2/21 and E1/3 vs E1/2, plus per-milestone window snapshots. Milestones
 * carry no observable output, so they appear only as step ordering between
 * sends and snapshots. expression-parameter replays
 * ViewUniqueExpressionParameter: the unique key is the evaluated expression
 * Math.abs(intPrimitive), so E1/10 is evicted by E2/-10 and E3/-5 by E4/5,
 * leaving the wildcard window holding {E2,E4}. two-windows replays
 * ViewUniqueTwoWindows: s0 (#unique(intBoxed), irstream *) receives E1
 * new-only before s1 (same view) is deployed mid-case by a deploy step, and
 * E2 then reaches both statements - s0 as new+old (E2/E1, same null intBoxed
 * key) and s1 as new-only - replacing the pinned underlying-identity
 * assertions with structurally normalized rows.
 *
 * Event payloads follow the absent-member-default convention: market events
 * are map events carrying symbol/price/volume/feed (absent price 0.0d,
 * absent volume 0L, feed supplied explicitly to mirror makeMarketDataEvent),
 * and bean events are LocalSupportBean mirrors of the pinned SupportBean
 * members in use (theString/intPrimitive/intBoxed, boxed members null when
 * absent).
 *
 * Listener records follow the standard protocol: one record per delivery
 * with per-statement sequence numbering from 1, time rendered from current
 * engine time, and new/old arrays rendered with the scalar normalization
 * rules. Snapshot records ({op:"snapshot", statement:"s0"}) iterate the
 * statement and emit its current window contents; unique-view iteration
 * order is not contractual (the pinned suite asserts AnyOrder for these
 * windows), so snapshot rows render in canonical sorted-field order in both
 * hosts.
 */
public class ViewUniqueScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewUniqueScenarioOracle <scenario.json>");
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
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewUniqueScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(caseName + "-s0"));
            EPStatement s0 = findStatement(deployment, "s0");
            s0.addListener(new TraceWriter(records, caseName, s0, runtime));

            boolean secondDeployed = false;
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
                if ("snapshot".equals(op)) {
                    String statementName = step.getString("statement", "");
                    if (!"s0".equals(statementName)) {
                        throw new IllegalStateException("unsupported snapshot statement " + statementName);
                    }
                    appendSnapshot(caseName, s0, records);
                    continue;
                }
                if ("deploy".equals(op)) {
                    if (!"two-windows".equals(caseName) || !"s1".equals(step.getString("statement", "")) || secondDeployed) {
                        throw new IllegalStateException("invalid mid-case deployment for case " + caseName);
                    }
                    // Pinned milestone-1 deployment: the second independent
                    // #unique(intBoxed) window starts empty after E1 arrived.
                    EPCompiled secondCompiled = EPCompilerProvider.getCompiler().compile(
                            "@name('s1') select irstream * from SupportBean#unique(intBoxed)",
                            new CompilerArguments(config));
                    EPDeployment secondDeployment = runtime.getDeploymentService().deploy(secondCompiled,
                            new DeploymentOptions().setDeploymentId(caseName + "-s1"));
                    EPStatement s1 = findStatement(secondDeployment, "s1");
                    s1.addListener(new TraceWriter(records, caseName, s1, runtime));
                    secondDeployed = true;
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }
            if (("two-windows".equals(caseName)) != secondDeployed) {
                throw new IllegalStateException("case " + caseName + " deploy steps do not match its plan");
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /** Verbatim transcriptions of the pinned ViewUnique modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "scene-one" ->
                "@name('s0') select irstream symbol, price from SupportMarketDataBean#unique(symbol) order by symbol";
            case "scene-two" ->
                "@name('s0') select irstream symbol, feed, price from  SupportMarketDataBean#unique(symbol, feed) order by symbol, feed";
            case "annotation-prefix" ->
                "@Name('s0') select irstream theString as c0, intPrimitive as c1 from SupportBean#unique(theString)";
            case "expression-parameter" ->
                "@name('s0') select * from SupportBean#unique(Math.abs(intPrimitive))";
            case "two-windows" ->
                "@name('s0') select irstream * from SupportBean#unique(intBoxed)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
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
            JsonValue intBoxedVal = payload.get("intBoxed");
            if (intBoxedVal instanceof JsonNumber) {
                event.setIntBoxed(((JsonNumber) intBoxedVal).asInt());
            }
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    /**
     * Iterates the statement window and appends a snapshot record. Rows sort
     * canonically by serialized fields because unique-view iteration order is
     * not contractual (AnyOrder assertions in the pinned suite).
     */
    private static void appendSnapshot(String caseName, EPStatement statement,
                                       List<JsonObject> records) {
        List<JsonObject> fieldRows = new ArrayList<>();
        Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            fieldRows.add(fields(iterator.next()));
        }
        fieldRows.sort(Comparator.comparing(JsonObject::toString));
        JsonArray rows = new JsonArray();
        for (JsonObject fieldRow : fieldRows) {
            rows.add(new JsonObject().add("kind", "row").add("fields", fieldRow));
        }
        JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "snapshot")
                .add("statement", statement.getName())
                .add("new", rows);
        records.add(record);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            array.add(new JsonObject().add("kind", "row").add("fields", fields(event)));
        }
        return array;
    }

    private static JsonObject fields(EventBean event) {
        JsonObject item = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            item.add(prop, normalize(event.get(prop)));
        }
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
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    private static final class TraceWriter implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(List<JsonObject> records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            sequence++;
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newEvents));
            }
            if (hasOld) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }
    }

    /** Local mirror of the pinned SupportBean regression bean members in use. */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private Integer intBoxed;

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

        public Integer getIntBoxed() {
            return intBoxed;
        }

        public void setIntBoxed(Integer intBoxed) {
            this.intBoxed = intBoxed;
        }
    }
}
