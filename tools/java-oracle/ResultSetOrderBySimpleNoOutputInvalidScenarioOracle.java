import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOrderBySimple ordinals 15-17.
 * Ordinal 15 (ResultSetNoOutputClauseView) replays two single-stream
 * order-by variants without an output clause: a length(5) window that emits
 * one sorted single-row batch per market event, and a time_batch(1 sec)
 * window that emits one six-row batch sorted by price at the one-second
 * boundary. Ordinal 16 (ResultSetNoOutputClauseJoin) replays the same two
 * shapes over the SupportMarketDataBean/SupportBeanString join. Ordinal 17
 * (ResultSetInvalid) compiles six statements whose order-by aggregate does
 * not occur identically in the select expression and pins the resulting
 * compile diagnostic. Every replayable case sends the shared six market
 * events (IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5); join cases then send
 * the five SupportBeanString seeds (CAT, IBM, CMU, KGB, DOG).
 */
public final class ResultSetOrderBySimpleNoOutputInvalidScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-simple-no-output-invalid";
    private static final String DESCRIPTION =
            "ResultSetOrderBySimple ordinals 15-17: order-by without an output "
                    + "clause over a length window and a time batch, the same two "
                    + "shapes over the SupportMarketDataBean/SupportBeanString join, "
                    + "and six build-error probes pinning the aggregate order-by "
                    + "compile diagnostic.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";
    private static final String INVALID_MESSAGE =
            "Aggregate functions in the order-by clause must also occur in the select expression";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-6de2b14776f97a0a69b2",
            "java-runtime-091c77bae73759b7cb3e",
            "java-runtime-bbc3ab446f26d0f99488"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetNoOutputClauseView",
            "ResultSetNoOutputClauseJoin",
            "ResultSetInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6"
    };
    private static final String[] CASES = {
            "no-output-view-v1",
            "no-output-view-v2",
            "no-output-join-v1",
            "no-output-join-v2",
            "invalid"
    };
    private static final int[] ORDINALS = {15, 15, 16, 16, 17};
    private static final String[] EPLS = {
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "order by price",
            "@name('s0') select symbol from SupportMarketDataBean#time_batch(1 sec) "
                    + "order by price",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "order by price",
            "@name('s0') select symbol from SupportMarketDataBean#time_batch(1) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "order by price, symbol",
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by sum(price)"
    };
    private static final String[] OBSERVATIONS = {
            "listener; ordinal 15 variant 1 emits one sorted single-row batch per "
                    + "market event over length(5) with no output clause",
            "listener; ordinal 15 variant 2 emits one six-row batch sorted by price "
                    + "when the time_batch(1 sec) window flushes at the one-second boundary",
            "listener; ordinal 16 variant 1 emits one sorted single-row batch per "
                    + "matching join row over length(10) with no output clause",
            "listener; ordinal 16 variant 2 emits one six-row join batch sorted by "
                    + "price, symbol when the time_batch(1) window flushes at the "
                    + "one-second boundary",
            "compile-only; ordinal 17 rejects six statements whose order-by aggregate "
                    + "does not occur identically in the select expression"
    };

    private static final String[] SEND_SYMBOLS = {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"};
    private static final double[] SEND_PRICES = {2d, 1d, 3d, 6d, 6d, 5d};
    private static final String[] JOIN_STRINGS = {"CAT", "IBM", "CMU", "KGB", "DOG"};

    private static final String[] PROBE_STATEMENTS = {
            "single-sum-missing",
            "single-sum-different",
            "single-sum-input-different",
            "join-sum-missing",
            "join-sum-different",
            "join-sum-input-different"
    };
    private static final String[] PROBE_EPLS = {
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by sum(price)",
            "@name('s0') select sum(price) from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by sum(price + 6)",
            "@name('s0') select sum(price + 6) from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by sum(price)",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by sum(price)",
            "@name('s0') select sum(price) from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by sum(price + 6)",
            "@name('s0') select sum(price + 6) from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by sum(price)"
    };

    // EXPECTED_SYMBOLS pins the per-record symbol sequences the Java test
    // observes through assertValues on the listener's last new data.
    private static final String[][][] EXPECTED_SYMBOLS = {
            {{"IBM"}, {"KGB"}, {"CMU"}, {"IBM"}, {"CAT"}, {"CAT"}, {"FOX"}},
            {{"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"}},
            {{"CAT", "CAT"}, {"IBM", "IBM"}, {"CMU"}, {"KGB"}, {"DOG"}},
            {{"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"}},
            {}
    };
    private static final long[] EXPECTED_TIMES = {
            0L, 1000L, 0L, 1000L, 0L
    };
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderBySimpleNoOutputInvalidScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderBySimpleNoOutputInvalidScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray allSteps = scenario.get("steps").asArray();
        JsonArray caseDefinitions = scenario.get("cases").asArray();
        for (int index = 0; index < CASES.length; index++) {
            if (ORDINALS[index] == 17) {
                runInvalidCase(caseDefinitions.get(index).asObject(), allSteps, records);
            } else {
                runReplayableCase(index, caseDefinitions.get(index).asObject(), allSteps, records);
            }
        }
        if (records.size() != 20) {
            throw new IllegalStateException("expected twenty trace records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    private static void runReplayableCase(int caseIndex, JsonObject caseDefinition,
                                          JsonArray allSteps, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        configuration.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderBySimpleNoOutputInvalidScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    EPLS[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime);
            statement.addListener(listener);
            replay(allSteps, caseName, runtime);
            if (listener.sequence != EXPECTED_SYMBOLS[caseIndex].length) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected "
                        + EXPECTED_SYMBOLS[caseIndex].length);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static void runInvalidCase(JsonObject caseDefinition, JsonArray allSteps,
                                       JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        configuration.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderBySimpleNoOutputInvalidScenarioOracle-invalid", configuration);
        runtime.getEventService().advanceTime(0);
        try {
            boolean inCase = false;
            int sequence = 0;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = object(stepValue, "step");
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = "invalid".equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if (!"build-error".equals(operation)) {
                    throw new IllegalArgumentException("unsupported operation " + operation
                            + " in case invalid");
                }
                String label = string(step, "statement");
                int probeIndex = indexOf(PROBE_STATEMENTS, label);
                if (probeIndex < 0 || !PROBE_EPLS[probeIndex].equals(string(step, "epl"))) {
                    throw new IllegalArgumentException("build-error probe is not pinned: " + label);
                }
                String expected = string(step, "expectError");
                if (!expected.equals(INVALID_MESSAGE + " [" + PROBE_EPLS[probeIndex] + "]")) {
                    throw new IllegalArgumentException("expected diagnostic is not pinned: " + label);
                }
                String actual;
                try {
                    EPCompilerProvider.getCompiler().compile(step.getString("epl", ""),
                            new CompilerArguments(configuration));
                    actual = "<no-error>";
                } catch (EPCompileException ex) {
                    actual = ex.getMessage();
                }
                if (!expected.equals(actual)) {
                    throw new IllegalStateException("compile diagnostic drift for " + label
                            + ": expected [" + expected + "] got [" + actual + "]");
                }
                records.add(new JsonObject().add("case", "invalid")
                        .add("operation", "compile-rejected")
                        .add("statement", label).add("sequence", ++sequence)
                        .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                        .add("value", actual));
            }
            if (sequence != PROBE_STATEMENTS.length) {
                throw new IllegalStateException("expected exactly six build-error probes, got " + sequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        EPStatement result = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if (!"s0".equals(candidate.getName())) {
                continue;
            }
            if (result != null) {
                throw new IllegalStateException("case " + caseName + " deployed multiple s0 statements");
            }
            result = candidate;
        }
        if (result == null) {
            throw new IllegalStateException("case " + caseName + " did not deploy statement s0");
        }
        return result;
    }

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = object(stepValue, "step");
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (operation) {
                case "send" -> sendEvent(runtime, step, caseName);
                case "advance-time" -> {
                    String at = string(step, "at");
                    long millis = Instant.parse(at).toEpochMilli();
                    runtime.getEventService().advanceTime(millis);
                }
                default -> throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "event payload");
        switch (eventType) {
            case "SupportMarketDataBean" -> {
                requireFields(payload, "symbol", "volume", "price");
                String symbol = string(payload, "symbol");
                long volume = longInteger(payload, "volume");
                double price = number(payload, "price");
                runtime.getEventService().sendEventBean(
                        new SupportMarketDataBean(symbol, price, volume, null),
                        "SupportMarketDataBean");
            }
            case "SupportBeanString" -> {
                requireFields(payload, "theString");
                runtime.getEventService().sendEventBean(
                        new SupportBeanString(string(payload, "theString")), "SupportBeanString");
            }
            default -> throw new IllegalArgumentException("unknown event type " + eventType
                    + " in case " + caseName);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly five cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !caseRuntimeId(CASES[index]).equals(string(definition, "runtimeId"))
                    || !caseExecutionName(CASES[index]).equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 51) {
            throw new IllegalArgumentException("scenario must contain exactly 51 steps");
        }
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            switch (CASES[caseIndex]) {
                case "no-output-view-v1" -> {
                    for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                        validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                                SEND_PRICES[eventIndex]);
                    }
                    validateMarketStep(steps.get(offset++), "FOX", 10d);
                }
                case "no-output-view-v2" -> {
                    validateAdvanceStep(steps.get(offset++), "1970-01-01T00:00:00.000Z");
                    for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                        validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                                SEND_PRICES[eventIndex]);
                    }
                    validateAdvanceStep(steps.get(offset++), "1970-01-01T00:00:01.000Z");
                }
                case "no-output-join-v1" -> {
                    for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                        validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                                SEND_PRICES[eventIndex]);
                    }
                    for (String expected : JOIN_STRINGS) {
                        validateStringStep(steps.get(offset++), expected);
                    }
                    validateMarketStep(steps.get(offset++), "DOG", 10d);
                }
                case "no-output-join-v2" -> {
                    validateAdvanceStep(steps.get(offset++), "1970-01-01T00:00:00.000Z");
                    for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                        validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                                SEND_PRICES[eventIndex]);
                    }
                    for (String expected : JOIN_STRINGS) {
                        validateStringStep(steps.get(offset++), expected);
                    }
                    validateAdvanceStep(steps.get(offset++), "1970-01-01T00:00:01.000Z");
                }
                case "invalid" -> {
                    for (int probeIndex = 0; probeIndex < PROBE_STATEMENTS.length; probeIndex++) {
                        validateProbeStep(steps.get(offset++), PROBE_STATEMENTS[probeIndex],
                                PROBE_EPLS[probeIndex]);
                    }
                }
                default -> throw new IllegalArgumentException("unknown case " + CASES[caseIndex]);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String caseRuntimeId(String caseName) {
        return RUNTIME_IDS[caseOrdinal(caseName) - 15];
    }

    private static String caseExecutionName(String caseName) {
        return EXECUTION_NAMES[caseOrdinal(caseName) - 15];
    }

    private static int caseOrdinal(String caseName) {
        for (int index = 0; index < CASES.length; index++) {
            if (CASES[index].equals(caseName)) {
                return ORDINALS[index];
            }
        }
        throw new IllegalArgumentException("unknown case " + caseName);
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateMarketStep(JsonValue value, String expectedSymbol,
                                           double expectedPrice) {
        JsonObject step = object(value, "market step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("market step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "market payload");
        requireFields(payload, "symbol", "volume", "price");
        if (!expectedSymbol.equals(string(payload, "symbol"))
                || longInteger(payload, "volume") != 0L
                || Double.compare(number(payload, "price"), expectedPrice) != 0) {
            throw new IllegalArgumentException("market payload is not pinned");
        }
    }

    private static void validateStringStep(JsonValue value, String expectedString) {
        JsonObject step = object(value, "string step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBeanString".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("string step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "string payload");
        requireFields(payload, "theString");
        if (!expectedString.equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("string payload is not pinned");
        }
    }

    private static void validateAdvanceStep(JsonValue value, String expectedAt) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op")) || !expectedAt.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step is not pinned");
        }
    }

    private static void validateProbeStep(JsonValue value, String expectedStatement,
                                          String expectedEpl) {
        JsonObject step = object(value, "build-error step");
        requireFields(step, "op", "statement", "epl", "expectError");
        if (!"build-error".equals(string(step, "op"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !(INVALID_MESSAGE + " [" + expectedEpl + "]").equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("build-error step is not pinned for " + expectedStatement);
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        if (!INTEGER_SYNTAX.matcher(text).matches()) {
            throw new IllegalArgumentException(name + " must use integer JSON syntax");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static double number(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON number");
        }
        return ((JsonNumber) value).asDouble();
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    private static int indexOf(String[] values, String wanted) {
        for (int index = 0; index < values.length; index++) {
            if (values[index].equals(wanted)) {
                return index;
            }
        }
        return -1;
    }

    private static void assertString(Object actual, String expected, String field, int rowIndex,
                                     String caseName) {
        if (!expected.equals(actual)) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static final class RecordingListener implements UpdateListener {
        private final JsonArray records;
        private final int caseIndex;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int sequence;

        private RecordingListener(JsonArray records, int caseIndex, EPStatement statement,
                                  EPRuntime runtime) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            if (newEvents == null || newEvents.length == 0) {
                if (oldEvents == null || oldEvents.length == 0) {
                    return;
                }
            }
            int next = sequence + 1;
            String[][] expected = EXPECTED_SYMBOLS[caseIndex];
            if (next > expected.length) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than " + expected.length + " listener callbacks");
            }
            if (oldEvents != null && oldEvents.length != 0) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must not carry old rows");
            }
            String[] expectedSymbols = expected[next - 1];
            if (newEvents == null || newEvents.length != expectedSymbols.length) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback " + next + " must contain "
                        + expectedSymbols.length + " new rows");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != EXPECTED_TIMES[caseIndex]) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " unexpected callback time " + now);
            }
            for (int rowIndex = 0; rowIndex < newEvents.length; rowIndex++) {
                EventBean event = newEvents[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, new String[]{"symbol"})) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                assertString(event.get("symbol"), expectedSymbols[rowIndex],
                        "symbol", rowIndex, CASES[caseIndex]);
            }

            JsonObject record = new JsonObject();
            record.add("case", CASES[caseIndex]);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", next);
            record.add("time", Instant.ofEpochMilli(now).toString());
            record.add("new", rows(newEvents));
            sequence = next;
            records.add(record);
        }
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray result = new JsonArray();
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            for (String property : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(property, normalize(event.get(property)));
            }
            result.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return result;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
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

    /** Local mirror of the pinned SupportMarketDataBean regression bean. */
    public static final class SupportMarketDataBean {
        private final String symbol;
        private final String id;
        private final double price;
        private final Long volume;
        private final String feed;

        public SupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
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
    public static final class SupportBeanString {
        private final String theString;

        public SupportBeanString(String theString) {
            this.theString = theString;
        }

        public String getTheString() {
            return theString;
        }
    }
}
