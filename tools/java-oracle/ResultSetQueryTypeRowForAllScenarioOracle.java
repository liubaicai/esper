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
import com.espertech.esper.regressionlib.support.bean.SupportBeanString;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.regressionlib.support.bean.SupportPriceEvent;
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
import java.util.Arrays;
import java.util.Iterator;
import java.util.List;
import java.util.TreeSet;

/**
 * Direct Esper oracle for ResultSetQueryTypeRowForAll ordinals 4-8 from the
 * pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c checkout.
 *
 * Each descriptor is replayed in a fresh virtual-time runtime. Listener and
 * iterator records are read from Esper's actual EventBean values; this class
 * has no expected-result substitute. The wildcard case verifies that its
 * selected EventBean retains the typed SupportMarketDataBean underlying and
 * records that type in the flat row representation used by parity traces.
 */
public final class ResultSetQueryTypeRowForAllScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "resultset-querytype-row-for-all";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private static final String[] CASES = {
        "sum-one-view",
        "sum-join",
        "avg-per-sym",
        "select-star-std-group-by",
        "select-expr-group-win"
    };
    private static final String[] RUNTIME_IDS = {
        "java-runtime-1d0f29a53fbb381563ec",
        "java-runtime-061f312bd81e3cb1c340",
        "java-runtime-422d65bd284a78278d56",
        "java-runtime-9a7f951d363bb58d506e",
        "java-runtime-780e3dede1e586fde738"
    };
    private static final String[] EXECUTION_NAMES = {
        "ResultSetQueryTypeRowForAllSumOneView",
        "ResultSetQueryTypeRowForAllSumJoin",
        "ResultSetQueryTypeRowForAllAvgPerSym",
        "ResultSetQueryTypeRowForAllSelectStarStdGroupBy",
        "ResultSetQueryTypeRowForAllSelectExprGroupWin"
    };

    private ResultSetQueryTypeRowForAllScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeRowForAllScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(
            Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);

        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();
        for (JsonValue caseValue : scenario.get("cases").asArray()) {
            runCase(caseValue.asObject(), allSteps, records);
        }

        JsonObject root = new JsonObject()
            .add("version", VERSION)
            .add("id", SCENARIO_ID)
            .add("javaCommit", PINNED_COMMIT)
            .add("java", System.getProperty("java.version"));
        JsonArray recordArray = new JsonArray();
        for (JsonObject record : records) {
            recordArray.add(record);
        }
        root.add("records", recordArray);
        System.out.println(root.toString());
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalStateException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalStateException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalStateException("scenario javaCommit is not pinned");
        }

        JsonArray caseDefs = scenario.get("cases").asArray();
        if (caseDefs.size() != CASES.length) {
            throw new IllegalStateException("expected five case descriptors");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonObject caseDef = caseDefs.get(i).asObject();
            String caseName = caseDef.getString("case", "");
            if (!CASES[i].equals(caseName)) {
                throw new IllegalStateException("unexpected case at ordinal " + (i + 4) + ": " + caseName);
            }
            if (caseDef.getInt("ordinal", -1) != i + 4) {
                throw new IllegalStateException("wrong ordinal for case " + caseName);
            }
            if (!RUNTIME_IDS[i].equals(caseDef.getString("runtimeId", ""))) {
                throw new IllegalStateException("wrong runtimeId for case " + caseName);
            }
            if (!EXECUTION_NAMES[i].equals(caseDef.getString("executionName", ""))) {
                throw new IllegalStateException("wrong executionName for case " + caseName);
            }
            if (!"listener".equals(caseDef.getString("observation", ""))) {
                throw new IllegalStateException("case " + caseName + " must be listener-observed");
            }
            if (!eplFor(caseName).equals(caseDef.getString("epl", ""))) {
                throw new IllegalStateException("EPL mismatch for case " + caseName);
            }
        }
    }

    private static void runCase(JsonObject caseDef, JsonArray allSteps, List<JsonObject> records) throws Exception {
        String caseName = caseDef.getString("case", "");
        String runtimeId = caseDef.getString("runtimeId", "");

        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        config.getCommon().addEventType("SupportPriceEvent", SupportPriceEvent.class);
        config.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
            "ResultSetQueryTypeRowForAllScenarioOracle-" + runtimeId, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                eplFor(caseName), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(
                compiled, new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, "s0", caseName);

            int[] sequence = new int[]{0};
            statement.addListener(new RecordingListener(records, caseName, statement, runtime, sequence));

            boolean inCase = false;
            boolean sawCaseMarker = false;
            int snapshots = 0;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    sawCaseMarker |= inCase;
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
                            throw new IllegalStateException("advance-time needs at in case " + caseName);
                        }
                        runtime.getEventService().advanceTime(Instant.parse(at).toEpochMilli());
                    }
                    case "snapshot" -> {
                        takeSnapshot(runtime, statement, records, sequence, step, caseName);
                        snapshots++;
                    }
                    default -> throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
                }
            }
            if (!sawCaseMarker) {
                throw new IllegalStateException("scenario has no case marker for " + caseName);
            }
            int expectedSnapshots = caseDef.getInt("iteratorSnapshots", 0);
            if (snapshots != expectedSnapshots) {
                throw new IllegalStateException("case " + caseName + " snapshot count " + snapshots
                    + " != descriptor " + expectedSnapshots);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String statementName, String caseName) {
        for (EPStatement statement : deployment.getStatements()) {
            if (statementName.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement '" + statementName + "' not found in case " + caseName);
    }

    private static void takeSnapshot(EPRuntime runtime, EPStatement statement, List<JsonObject> records,
                                     int[] sequence, JsonObject step, String caseName) {
        if (!statement.getName().equals(step.getString("statement", ""))) {
            throw new IllegalStateException("snapshot targets unknown statement in case " + caseName);
        }
        if (step.getString("label", "").isEmpty()) {
            throw new IllegalStateException("snapshot needs label in case " + caseName);
        }
        String mode = step.getString("mode", "");
        if (!"any".equals(mode) && !"fifo".equals(mode)) {
            throw new IllegalStateException("snapshot needs mode any|fifo in case " + caseName);
        }

        List<EventBean> values = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            values.add(iterator.next());
        }
        JsonObject record = new JsonObject()
            .add("case", caseName)
            .add("operation", "snapshot")
            .add("statement", statement.getName())
            .add("sequence", ++sequence[0])
            .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
            .add("new", rows(values.toArray(new EventBean[0]), caseName));
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean event = new SupportBean();
                event.setTheString(nullableString(payload, "theString"));
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null && !longBoxed.isNull()) {
                    event.setLongBoxed(longBoxed.asLong());
                } else {
                    event.setLongBoxed(null);
                }
                runtime.getEventService().sendEventBean(event, eventType);
            }
            case "SupportBeanString" -> runtime.getEventService().sendEventBean(
                new SupportBeanString(nullableString(payload, "theString")), eventType);
            case "SupportPriceEvent" -> {
                JsonValue price = payload.get("price");
                if (price == null || price.isNull() || !(price instanceof JsonNumber)) {
                    throw new IllegalStateException("SupportPriceEvent needs numeric price in case " + caseName);
                }
                runtime.getEventService().sendEventBean(
                    new SupportPriceEvent(price.asInt(), nullableString(payload, "sym")), eventType);
            }
            case "SupportMarketDataBean" -> {
                JsonValue price = payload.get("price");
                if (price == null || price.isNull() || !(price instanceof JsonNumber)) {
                    throw new IllegalStateException("SupportMarketDataBean needs numeric price in case " + caseName);
                }
                Long volume = null;
                JsonValue volumeValue = payload.get("volume");
                if (volumeValue != null && !volumeValue.isNull()) {
                    volume = volumeValue.asLong();
                }
                SupportMarketDataBean event = new SupportMarketDataBean(
                    nullableString(payload, "symbol"), price.asDouble(), volume,
                    nullableString(payload, "feed"));
                JsonValue idValue = payload.get("id");
                if (idValue != null && !idValue.isNull()) {
                    event.setId(idValue.asString());
                }
                runtime.getEventService().sendEventBean(event, eventType);
            }
            default -> throw new IllegalStateException("unknown eventType " + eventType + " in case " + caseName);
        }
    }

    private static String nullableString(JsonObject object, String name) {
        JsonValue value = object.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static final class RecordingListener implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int[] sequence;

        private RecordingListener(List<JsonObject> records, String caseName, EPStatement statement,
                                  EPRuntime runtime, int[] sequence) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.sequence = sequence;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "listener")
                .add("statement", statement.getName())
                .add("sequence", ++sequence[0])
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newEvents, caseName));
            }
            if (hasOld) {
                record.add("old", rows(oldEvents, caseName));
            }
            records.add(record);
        }
    }

    private static JsonArray rows(EventBean[] events, String caseName) {
        JsonArray rows = new JsonArray();
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            if ("select-star-std-group-by".equals(caseName)) {
                if (!(event.getUnderlying() instanceof SupportMarketDataBean)) {
                    throw new IllegalStateException("wildcard row lost SupportMarketDataBean underlying");
                }
                fields.add("__type", "SupportMarketDataBean");
            }
            String[] properties = event.getEventType().getPropertyNames().clone();
            Arrays.sort(properties);
            for (String property : properties) {
                fields.add(property, normalize(event.get(property)));
            }
            rows.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return rows;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        if (value instanceof int[]) {
            JsonArray array = new JsonArray();
            for (int item : (int[]) value) {
                array.add(item);
            }
            return array;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof EventBean) {
            EventBean event = (EventBean) value;
            JsonObject fields = new JsonObject();
            String[] properties = event.getEventType().getPropertyNames().clone();
            Arrays.sort(properties);
            for (String property : properties) {
                fields.add(property, normalize(event.get(property)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        return Json.value(String.valueOf(value));
    }

    private static String eplFor(String caseName) {
        return switch (caseName) {
            case "sum-one-view" ->
                "@name('s0') select irstream sum(longBoxed) as mySum from SupportBean#time(10 sec)";
            case "sum-join" ->
                "@name('s0') select irstream sum(longBoxed) as mySum "
                    + "from SupportBeanString#keepall as one, SupportBean#time(10 sec) as two "
                    + "where one.theString = two.theString";
            case "avg-per-sym" ->
                "@name('s0') select irstream avg(price) as avgp, sym "
                    + "from SupportPriceEvent#groupwin(sym)#length(2)";
            case "select-star-std-group-by" ->
                "@name('s0') select istream * from SupportMarketDataBean#groupwin(symbol)#length(2)";
            case "select-expr-group-win" ->
                "@name('s0') select istream price from SupportMarketDataBean#groupwin(symbol)#length(2)";
            default -> throw new IllegalStateException("unknown case " + caseName);
        };
    }
}
