import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.EPCompileException;
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
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ViewLengthBatch length-batch view scenarios.
 *
 * Covers all ten executions of ViewLengthBatch across ten scenario cases,
 * each replaying its pinned module verbatim (only the @name('s0')
 * annotations are sanctioned additions):
 *
 * scene-one replays ViewLengthBatchSceneOne: select irstream * over
 * SupportMarketDataBean#length_batch(3) flushed by sends E1..E10 at E3/E6/E9
 * (new current triple, old prior triple, null first), with snapshot steps
 * after E1/E4/E5 observing partial-window iterator state. size-two replays
 * ViewLengthBatchSize2: length_batch(2) over six bare SupportBeans flushing
 * at even positions, snapshots after odd sends verify the single-row partial
 * window. size-one replays ViewLengthBatchSize1: length_batch(1) where every
 * send flushes immediately. size-three replays ViewLengthBatchSize3:
 * length_batch(3) over six bare SupportBeans with snapshots between flushes.
 * invalid replays ViewLengthBatchInvalid as a compile-only case: the pinned
 * module select * from SupportMarketDataBean#length_batch(0) must fail
 * compilation, recorded as a "deployed"/"invalid" marker of successful
 * rejection. prev replays ViewLengthBatchPrev: batch-level accessor columns
 * prev(1)/prevtail(0)/prevtail(1)/prevcount/prevwindow evaluated once per
 * delivery and shared by all three new rows of the single flush. delete
 * replays ViewLengthBatchDelete: named window ABCWin#length_batch(3) fed by
 * insert-into, emptied through on SupportBean_A deletes that stay quiet
 * under the batch view, observed via snapshots between flushes.
 * normal-view/normal-namedwindow/normal-groupwin replay the three
 * ViewLengthBatchNormal variants: the prevString projection, the
 * create-window ABCWin twin, and groupwin(doubleBoxed)#length_batch(3),
 * all sharing the identical nine-send schedule.
 *
 * Event types follow the pinned regression schema: SupportMarketDataBean is
 * a map type {symbol string, price double, volume long, feed string} whose
 * sends set symbol only (price 0, volume 0L, feed null, mirroring
 * makeMarketDataEvent); SupportBean is mirrored locally as
 * {theString string, intPrimitive int, doubleBoxed Double} because the
 * pinned bean lives outside the oracle classpath, bare sends keeping
 * theString null (get10Events) while normal-family sends carry
 * theString Ei with intPrimitive 0; SupportBean_A carries id only for the
 * delete-case triggers.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch containing rows, sequence numbering per case from 1, time rendered
 * from the current engine time, and new/old row arrays rendered with the
 * scalar normalization rules. Zero-selection queries deliver event-backed
 * rows enumerating ALL properties per row. Snapshot records carry no time
 * or sequence: step {op:"snapshot", statement:"s0"} iterates the deployed
 * s0 statement and emits its current window contents.
 */
public class ViewLengthBatchScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewLengthBatchScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType("SupportBean_A", LocalSupportBeanA.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewLengthBatchScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            boolean invalidCase = "invalid".equals(caseName);

            // The invalid case is compile-only: the pinned module must be
            // rejected, so it deploys nothing and owns no s0 statement.
            EPStatement s0 = null;
            if (!invalidCase) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                s0 = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), "s0");
                if (s0 == null) {
                    throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
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
                if ("deployed".equals(op)) {
                    if (!invalidCase || !"invalid".equals(step.getString("statement", ""))) {
                        throw new IllegalStateException("unexpected deployed op for case " + caseName);
                    }
                    try {
                        EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                    } catch (EPCompileException rejected) {
                        // pinned behavior: LengthBatch(0) fails validation
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", "invalid");
                        records.add(record);
                        continue;
                    }
                    throw new IllegalStateException("compile unexpectedly succeeded for case " + caseName);
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /** Verbatim transcriptions of the pinned ViewLengthBatch modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "scene-one" ->
                "@name('s0') select irstream * from SupportMarketDataBean#length_batch(3)";
            case "size-two" ->
                "@name('s0') select irstream * from SupportBean#length_batch(2)";
            case "size-one" ->
                "@name('s0') select irstream * from SupportBean#length_batch(1)";
            case "size-three" ->
                "@name('s0') select irstream * from SupportBean#length_batch(3)";
            case "invalid" ->
                "select * from SupportMarketDataBean#length_batch(0)";
            case "prev" ->
                "@name('s0') select irstream *, " +
                    "prev(1, symbol) as prev1, " +
                    "prevtail(0, symbol) as prevTail0, " +
                    "prevtail(1, symbol) as prevTail1, " +
                    "prevcount(symbol) as prevCountSym, " +
                    "prevwindow(symbol) as prevWindowSym " +
                    "from SupportMarketDataBean#length_batch(3)";
            case "delete" ->
                "create window ABCWin#length_batch(3) as SupportBean;\n" +
                    "insert into ABCWin select * from SupportBean;\n" +
                    "on SupportBean_A delete from ABCWin where theString = id;\n" +
                    "@Name('s0') select irstream * from ABCWin;\n";
            case "normal-view" ->
                "@Name('s0') select irstream theString, prev(1, theString) as prevString " +
                    "from SupportBean#length_batch(3)";
            case "normal-namedwindow" ->
                "create window ABCWin#length_batch(3) as SupportBean;\n" +
                    "insert into ABCWin select * from SupportBean;\n" +
                    "@Name('s0') select irstream * from ABCWin;\n";
            case "normal-groupwin" ->
                "@Name('s0') select irstream * from SupportBean#groupwin(doubleBoxed)#length_batch(3)";
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
        if ("SupportBean_A".equals(eventType)) {
            LocalSupportBeanA event = new LocalSupportBeanA(payload.getString("id", null));
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
            // covers prevwindow(symbol), which surfaces as an array of
            // window values rather than a scalar
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

    /** Local mirror of the pinned SupportBean_A regression bean. */
    public static class LocalSupportBeanA {
        private final String id;

        public LocalSupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }
}
