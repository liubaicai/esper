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
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for two adjacent aggregate executions. */
public final class ResultSetAggregateMaxMinGroupByJoinSelectHavingScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-minmax-groupby-join-select-having";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java";
    private static final String JOIN = "minmax-join";
    private static final String SELECT_HAVING = "min-no-group-select-having";
    private static final String[] CASES = {JOIN, SELECT_HAVING};
    private static final int[] ORDINALS = {3, 5};
    private static final String[] RUNTIMES = {
            "java-runtime-802aec9425dc772e3aa5",
            "java-runtime-060f5af73edf420ef45a"
    };
    private static final String[] NAMES = {
            "ResultSetAggregateMinMaxJoin",
            "ResultSetAggregateMinNoGroupSelectHaving"
    };
    private static final int[] SEND_COUNTS = {14, 6};
    private static final int[] RECORD_COUNTS = {12, 2};
    private static final int[] JOIN_ROW_COUNTS = {1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1};
    private static final String[] JOIN_SYMBOLS = {
            "DELL", "DELL", "DELL", "DELL", "DELL",
            "IBM", "IBM", "IBM", "IBM", "IBM", "IBM", "IBM"
    };
    private static final Long[] JOIN_VOLUMES = {
            50L, 30L, 30L, 90L, 100L,
            20L, 5L, 15L, 18L, null, null, null
    };
    private static final Long[] SELECT_VOLUMES = {100L, 105L, 100L, 131L, 132L, 129L};

    private ResultSetAggregateMaxMinGroupByJoinSelectHavingScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateMaxMinGroupByJoinSelectHavingScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(
                Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }
        if (records.size() != 14) {
            throw new IllegalStateException("expected 14 listener records, got " + records.size());
        }
        System.out.println(new JsonObject()
                .add("version", VERSION)
                .add("id", ID)
                .add("javaCommit", COMMIT)
                .add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))
                || !ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario");
        }
        if (!COMMIT.equals(scenario.getString("javaCommit", ""))
                || !SOURCE.equals(scenario.getString("javaSource", ""))) {
            throw new IllegalArgumentException("scenario Java identity is not pinned");
        }
        validateStrings(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStrings(scenario.get("javaNames"), NAMES, "javaNames");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray()
                || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        JsonArray cases = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(cases.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIMES[i].equals(definition.getString("runtimeId", ""))
                    || !NAMES[i].equals(definition.getString("executionName", ""))
                    || !"listener".equals(definition.getString("observation", ""))
                    || !epl(CASES[i]).equals(definition.getString("epl", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 22) {
            throw new IllegalArgumentException("scenario must contain exactly 22 steps");
        }
        int stepIndex = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(stepIndex++), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", ""))
                    || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            if (caseIndex == 0) {
                validateStringSend(object(steps.get(stepIndex), "DELL seed"), "DELL", stepIndex++);
                validateStringSend(object(steps.get(stepIndex), "IBM seed"), "IBM", stepIndex++);
            }
            int marketCount = caseIndex == 0 ? JOIN_SYMBOLS.length : SELECT_VOLUMES.length;
            for (int eventIndex = 0; eventIndex < marketCount; eventIndex++) {
                String symbol = caseIndex == 0 ? JOIN_SYMBOLS[eventIndex] : "DELL";
                Long volume = caseIndex == 0 ? JOIN_VOLUMES[eventIndex] : SELECT_VOLUMES[eventIndex];
                validateMarketSend(object(steps.get(stepIndex), "market step " + stepIndex),
                        symbol, volume, stepIndex++);
            }
        }
        if (stepIndex != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static String epl(String caseName) {
        if (JOIN.equals(caseName)) {
            return "@name('s0') select irstream symbol, min(volume) as minVol, "
                    + "max(volume) as maxVol, min(distinct volume) as minDistVol, "
                    + "max(distinct volume) as maxDistVol from SupportBeanString#length(100) as one, "
                    + "SupportMarketDataBean#length(3) as two where (symbol='DELL' or symbol='IBM' or symbol='GE') "
                    + "and one.theString = two.symbol group by symbol";
        }
        return "@name('s0') select symbol, min(volume) as mymin "
                + "from SupportMarketDataBean#length(5) having volume > min(volume) * 1.3";
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("volume", Long.class);
        marketType.put("price", Double.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        if (JOIN.equals(caseName)) {
            Map<String, Object> stringType = new HashMap<>();
            stringType.put("theString", String.class);
            configuration.getCommon().addEventType("SupportBeanString", stringType);
        }

        String runtimeURI = "parity-resultset-aggregate-minmax-groupby-join-select-having-"
                + RUNTIMES[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    epl(caseName), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(
                    compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replay(steps, caseIndex, runtime);
            if (writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.sequence
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replay(JsonArray steps, int caseIndex, EPRuntime runtime) {
        String caseName = CASES[caseIndex];
        boolean active = false;
        int sends = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = object(steps.get(i), "step " + i);
            if ("case".equals(step.getString("op", ""))) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if (!"send".equals(step.getString("op", ""))) {
                throw new IllegalArgumentException("unsupported operation in case " + caseName);
            }
            JsonObject payload = object(step.get("payload"), "payload " + i);
            if ("SupportBeanString".equals(step.getString("eventType", ""))) {
                Map<String, Object> event = new HashMap<>();
                event.put("theString", payload.getString("theString", null));
                runtime.getEventService().sendEventMap(event, "SupportBeanString");
            } else {
                Map<String, Object> event = new HashMap<>();
                event.put("symbol", payload.getString("symbol", null));
                JsonValue volume = payload.get("volume");
                event.put("volume", volume == null || volume.isNull() ? null : volume.asLong());
                event.put("price", payload.getDouble("price", 0.0d));
                event.put("feed", null);
                runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
            }
            sends++;
        }
        if (sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalStateException("case " + caseName + " replayed " + sends
                    + " sends, expected " + SEND_COUNTS[caseIndex]);
        }
    }

    private static void validateStringSend(JsonObject step, String expected, int stepIndex) {
        if (!"send".equals(step.getString("op", ""))
                || !"SupportBeanString".equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send SupportBeanString");
        }
        JsonObject payload = object(step.get("payload"), "string payload " + stepIndex);
        if (payload.size() != 1 || !payload.names().contains("theString")
                || !expected.equals(payload.getString("theString", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " string payload mismatch");
        }
    }

    private static void validateMarketSend(JsonObject step, String expectedSymbol,
                                           Long expectedVolume, int stepIndex) {
        if (!"send".equals(step.getString("op", ""))
                || !"SupportMarketDataBean".equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send SupportMarketDataBean");
        }
        JsonObject payload = object(step.get("payload"), "market payload " + stepIndex);
        if (payload.size() != 3 || !payload.names().contains("symbol")
                || !payload.names().contains("volume") || !payload.names().contains("price")) {
            throw new IllegalArgumentException("step " + stepIndex + " market payload fields mismatch");
        }
        if (!expectedSymbol.equals(payload.getString("symbol", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " symbol mismatch");
        }
        JsonValue price = payload.get("price");
        if (price == null || !isIntegralNumber(price) || price.asLong() != 0L) {
            throw new IllegalArgumentException("step " + stepIndex + " price must be integer zero");
        }
        JsonValue volume = payload.get("volume");
        if (expectedVolume == null) {
            if (volume == null || !volume.isNull()) {
                throw new IllegalArgumentException("step " + stepIndex + " volume must be null");
            }
        } else if (volume == null || !isIntegralNumber(volume)
                || volume.asLong() != expectedVolume) {
            throw new IllegalArgumentException("step " + stepIndex + " volume mismatch");
        }
    }

    private static boolean isIntegralNumber(JsonValue value) {
        if (!value.isNumber()) {
            return false;
        }
        try {
            return value.asDouble() == value.asLong();
        } catch (RuntimeException ex) {
            return false;
        }
    }

    private static void validateStrings(JsonValue value, String[] expected, String name) {
        JsonArray values = array(value, name);
        if (values.size() != expected.length) {
            throw new IllegalArgumentException(name + " length mismatch");
        }
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be an array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            if (JOIN.equals(caseName)) {
                if (!hasNew || !hasOld || newEvents.length != oldEvents.length
                        || sequence >= JOIN_ROW_COUNTS.length
                        || newEvents.length != JOIN_ROW_COUNTS[(int) sequence]) {
                    throw new IllegalStateException("join callback row count mismatch");
                }
            } else if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("select/having callback must contain one new row");
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newEvents));
            }
            if (hasOld) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
