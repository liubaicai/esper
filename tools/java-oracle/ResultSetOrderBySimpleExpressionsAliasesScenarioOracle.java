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
 * Direct Esper 9.0.0 oracle for ResultSetOrderBySimple ordinals 5-9.
 * Ordinal 5 (ResultSetExpressions) replays four expression order-by
 * variants over a length(10) market window; ordinal 6
 * (ResultSetAliasesSimple) replays four alias order-by variants mixing
 * length(5) and length(10) windows; ordinal 7 (ResultSetExpressionsJoin)
 * replays the same four expression variants over the
 * SupportMarketDataBean/SupportBeanString join; ordinal 8
 * (ResultSetMultipleKeys) replays three multi-key variants; ordinal 9
 * (ResultSetAliases) replays the four ordinal-6 statements plus a fifth
 * without an output clause that delivers one single-row batch per send.
 * Every output-every-6-events case sends the shared six market events
 * (IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5) and records exactly one
 * six-row new-only batch; join cases then send the five SupportBeanString
 * seeds (CAT, IBM, CMU, KGB, DOG) whose matches generate the six join
 * rows; the no-output-clause case sends FOX@10 after the shared sequence
 * and records seven one-row batches (Esper's order-by without an output
 * clause delivers insert rows only, so no remove-stream rows appear).
 */
public final class ResultSetOrderBySimpleExpressionsAliasesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-simple-expressions-aliases";
    private static final String DESCRIPTION =
            "ResultSetOrderBySimple ordinals 5-9: expression, alias, join, and multi-key "
                    + "order-by over length windows with output every 6 events; ordinal 9 "
                    + "variant 5 drops the output clause so each of its seven sends delivers "
                    + "a one-row batch.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-0a7a2ff59087a96fe0e1",
            "java-runtime-fc69777a3bedc3ad6c52",
            "java-runtime-b0f39fb300a33fa7ddbf",
            "java-runtime-92e69b0657b10a656fdc",
            "java-runtime-178a1d0412982958a3cc"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetExpressions",
            "ResultSetAliasesSimple",
            "ResultSetExpressionsJoin",
            "ResultSetMultipleKeys",
            "ResultSetAliases"
    };
    private static final String[] STATIC_IDS = {
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6"
    };
    private static final String[] CASES = {
            "expressions-v1",
            "expressions-v2",
            "expressions-v3",
            "expressions-v4",
            "aliases-simple-v1",
            "aliases-simple-v2",
            "aliases-simple-v3",
            "aliases-simple-v4",
            "expressions-join-v1",
            "expressions-join-v2",
            "expressions-join-v3",
            "expressions-join-v4",
            "multiple-keys-v1",
            "multiple-keys-v2",
            "multiple-keys-v3",
            "aliases-v1",
            "aliases-v2",
            "aliases-v3",
            "aliases-v4",
            "aliases-v5"
    };
    private static final int[] ORDINALS = {
            5, 5, 5, 5,
            6, 6, 6, 6,
            7, 7, 7, 7,
            8, 8, 8,
            9, 9, 9, 9, 9
    };
    private static final String[] EPLS = {
            "@name('s0') select symbol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (price * 6) + 5",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (price * 6) + 5, price",
            "@name('s0') select symbol, 1+volume*23 from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (price * 6) + 5, price, volume",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by volume*price, symbol",
            "@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by mySymbol",
            "@name('s0') select symbol as mySymbol, price as myPrice from "
                    + "SupportMarketDataBean#length(5) output every 6 events order by myPrice",
            "@name('s0') select symbol, price as myPrice from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (myPrice * 6) + 5, price",
            "@name('s0') select symbol, 1+volume*23 as myVol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (price * 6) + 5, price, myVol",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by (price * 6) + 5",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by (price * 6) + 5, price",
            "@name('s0') select symbol, 1+volume*23 from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by (price * 6) + 5, price, volume",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by volume*price, symbol",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by symbol, price",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by price, symbol, volume",
            "@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by price, volume",
            "@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by mySymbol",
            "@name('s0') select symbol as mySymbol, price as myPrice from "
                    + "SupportMarketDataBean#length(5) output every 6 events order by myPrice",
            "@name('s0') select symbol, price as myPrice from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (myPrice * 6) + 5, price",
            "@name('s0') select symbol, 1+volume*23 as myVol from SupportMarketDataBean#length(10) "
                    + "output every 6 events order by (price * 6) + 5, price, myVol",
            "@name('s0') select symbol as mySymbol from SupportMarketDataBean#length(5) "
                    + "order by price, mySymbol"
    };
    private static final String[] OBSERVATIONS = {
            "listener; ordinal 5 variant 1 orders the six-row output batch by (price * 6) + 5",
            "listener; ordinal 5 variant 2 selects symbol, price and orders the six-row output "
                    + "batch by (price * 6) + 5, price",
            "listener; ordinal 5 variant 3 selects symbol and the computed column 1+volume*23, "
                    + "ordering the six-row output batch by (price * 6) + 5, price, volume",
            "listener; ordinal 5 variant 4 orders the six-row output batch by volume*price, "
                    + "symbol; the constant zero volume makes the symbol key decide",
            "listener; ordinal 6 variant 1 selects symbol as mySymbol over a length(5) window "
                    + "and orders the six-row output batch by the mySymbol alias",
            "listener; ordinal 6 variant 2 selects symbol as mySymbol, price as myPrice and "
                    + "orders the six-row output batch by the myPrice alias",
            "listener; ordinal 6 variant 3 selects symbol, price as myPrice and orders the "
                    + "six-row output batch by (myPrice * 6) + 5, price",
            "listener; ordinal 6 variant 4 selects symbol, 1+volume*23 as myVol and orders the "
                    + "six-row output batch by (price * 6) + 5, price, myVol",
            "listener; ordinal 7 variant 1 orders the six-row join output batch by "
                    + "(price * 6) + 5; the price-6 tie keeps join-generation order so CAT "
                    + "precedes IBM",
            "listener; ordinal 7 variant 2 selects symbol, price and orders the six-row join "
                    + "output batch by (price * 6) + 5, price",
            "listener; ordinal 7 variant 3 selects symbol and the computed column 1+volume*23, "
                    + "ordering the six-row join output batch by (price * 6) + 5, price, volume",
            "listener; ordinal 7 variant 4 orders the six-row join output batch by "
                    + "volume*price, symbol; the constant zero volume makes the symbol key decide",
            "listener; ordinal 8 variant 1 orders the six-row output batch by symbol, price",
            "listener; ordinal 8 variant 2 orders the six-row output batch by price, symbol, "
                    + "volume so the price-6 tie resolves CAT, CAT, IBM",
            "listener; ordinal 8 variant 3 selects symbol, volume*2 and orders the six-row "
                    + "output batch by price, volume; the constant zero volume keeps insertion "
                    + "order at the price-6 tie",
            "listener; ordinal 9 variant 1 replays the ordinal 6 variant 1 statement, ordering "
                    + "the six-row output batch by the mySymbol alias",
            "listener; ordinal 9 variant 2 replays the ordinal 6 variant 2 statement, ordering "
                    + "the six-row output batch by the myPrice alias",
            "listener; ordinal 9 variant 3 replays the ordinal 6 variant 3 statement, ordering "
                    + "the six-row output batch by (myPrice * 6) + 5, price",
            "listener; ordinal 9 variant 4 replays the ordinal 6 variant 4 statement, ordering "
                    + "the six-row output batch by (price * 6) + 5, price, myVol",
            "listener; ordinal 9 variant 5 has no output clause so each of the seven sends "
                    + "delivers its own one-row batch ordered by price, mySymbol"
    };

    private static final String[] SEND_SYMBOLS = {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"};
    private static final double[] SEND_PRICES = {2d, 1d, 3d, 6d, 6d, 5d};
    private static final String[] JOIN_STRINGS = {"CAT", "IBM", "CMU", "KGB", "DOG"};
    private static final String EXTRA_SYMBOL = "FOX";
    private static final double EXTRA_PRICE = 10d;

    private static final String[][] EXPECTED_FIELDS = {
            {"symbol"},
            {"price", "symbol"},
            {"1+volume*23", "symbol"},
            {"symbol"},
            {"mySymbol"},
            {"myPrice", "mySymbol"},
            {"myPrice", "symbol"},
            {"myVol", "symbol"},
            {"symbol"},
            {"price", "symbol"},
            {"1+volume*23", "symbol"},
            {"symbol"},
            {"symbol"},
            {"symbol"},
            {"symbol", "volume*2"},
            {"mySymbol"},
            {"myPrice", "mySymbol"},
            {"myPrice", "symbol"},
            {"myVol", "symbol"},
            {"mySymbol"}
    };
    private static final String[][] EXPECTED_SYMBOLS = {
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT", "FOX"}
    };
    private static final double[][] EXPECTED_PRICES = {
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null,
            null,
            null,
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null
    };
    private static final String[] EXPECTED_COMPUTED_NAMES = {
            null, null, "1+volume*23", null,
            null, null, null, "myVol",
            null, null, "1+volume*23", null,
            null, null, "volume*2",
            null, null, null, "myVol", null
    };
    private static final long[][] EXPECTED_COMPUTED = {
            null, null, {1L, 1L, 1L, 1L, 1L, 1L}, null,
            null, null, null, {1L, 1L, 1L, 1L, 1L, 1L},
            null, null, {1L, 1L, 1L, 1L, 1L, 1L}, null,
            null, null, {0L, 0L, 0L, 0L, 0L, 0L},
            null, null, null, {1L, 1L, 1L, 1L, 1L, 1L}, null
    };
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderBySimpleExpressionsAliasesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderBySimpleExpressionsAliasesScenarioOracle <scenario.json>");
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
            runCase(index, caseDefinitions.get(index).asObject(), allSteps, records);
        }
        if (records.size() != 26) {
            throw new IllegalStateException("expected twenty-six trace records, got "
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

    private static void runCase(int caseIndex, JsonObject caseDefinition,
                                JsonArray allSteps, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        configuration.getCommon().addEventType("SupportBeanString", SupportBeanString.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderBySimpleExpressionsAliasesScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    EPLS[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            int expectedCallbacks = expectedListenerCallbacks(caseIndex);
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime,
                    expectedCallbacks);
            statement.addListener(listener);
            replay(allSteps, caseName, runtime);
            if (listener.sequence != expectedCallbacks) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected " + expectedCallbacks);
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

    private static int expectedListenerCallbacks(int caseIndex) {
        return "aliases-v5".equals(CASES[caseIndex]) ? 7 : 1;
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
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
            sendEvent(runtime, step, caseName);
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
            throw new IllegalArgumentException("scenario must contain exactly twenty cases");
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
        if (steps.size() != 161) {
            throw new IllegalArgumentException("scenario must contain exactly 161 steps");
        }
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                        SEND_PRICES[eventIndex]);
            }
            if (ORDINALS[caseIndex] == 7) {
                for (String expected : JOIN_STRINGS) {
                    validateStringStep(steps.get(offset++), expected);
                }
            }
            if ("aliases-v5".equals(CASES[caseIndex])) {
                validateMarketStep(steps.get(offset++), EXTRA_SYMBOL, EXTRA_PRICE);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String caseRuntimeId(String caseName) {
        return RUNTIME_IDS[caseOrdinal(caseName) - 5];
    }

    private static String caseExecutionName(String caseName) {
        return EXECUTION_NAMES[caseOrdinal(caseName) - 5];
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

    private static void assertString(Object actual, String expected, String field, int rowIndex,
                                     String caseName) {
        if (!expected.equals(actual)) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static void assertLong(Object actual, long expected, String field, int rowIndex,
                                   String caseName) {
        if (!(actual instanceof Number) || ((Number) actual).longValue() != expected) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static void assertDouble(Object actual, double expected, String field, int rowIndex,
                                     String caseName) {
        if (!(actual instanceof Number)
                || Double.compare(((Number) actual).doubleValue(), expected) != 0) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static final class RecordingListener implements UpdateListener {
        private final JsonArray records;
        private final int caseIndex;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int expectedCallbacks;
        private int sequence;

        private RecordingListener(JsonArray records, int caseIndex, EPStatement statement,
                                  EPRuntime runtime, int expectedCallbacks) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.statement = statement;
            this.runtime = runtime;
            this.expectedCallbacks = expectedCallbacks;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > expectedCallbacks) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than " + expectedCallbacks + " listener callbacks");
            }
            boolean perEvent = expectedCallbacks > 1;
            int expectedRows = perEvent ? 1 : SEND_SYMBOLS.length;
            if (newEvents == null || newEvents.length != expectedRows) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + expectedRows + " new rows");
            }
            if (oldEvents != null && oldEvents.length != 0) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must not carry old rows");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            validateRows(newEvents, next);

            JsonObject record = new JsonObject();
            record.add("case", CASES[caseIndex]);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", next);
            record.add("time", Instant.ofEpochMilli(now).toString());
            record.add("new", rows(newEvents));
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            sequence = next;
            records.add(record);
        }

        private void validateRows(EventBean[] events, int callback) {
            String symbolField = symbolField();
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, EXPECTED_FIELDS[caseIndex])) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                int expectedIndex = expectedCallbacks > 1 ? callback - 1 : rowIndex;
                assertString(event.get(symbolField), EXPECTED_SYMBOLS[caseIndex][expectedIndex],
                        symbolField, rowIndex, CASES[caseIndex]);
                if (EXPECTED_PRICES[caseIndex] != null) {
                    String priceField = hasField(EXPECTED_FIELDS[caseIndex], "myPrice")
                            ? "myPrice" : "price";
                    assertDouble(event.get(priceField), EXPECTED_PRICES[caseIndex][expectedIndex],
                            priceField, rowIndex, CASES[caseIndex]);
                }
                if (EXPECTED_COMPUTED_NAMES[caseIndex] != null) {
                    assertLong(event.get(EXPECTED_COMPUTED_NAMES[caseIndex]),
                            EXPECTED_COMPUTED[caseIndex][expectedIndex],
                            EXPECTED_COMPUTED_NAMES[caseIndex], rowIndex, CASES[caseIndex]);
                }
            }
        }

        private String symbolField() {
            return hasField(EXPECTED_FIELDS[caseIndex], "mySymbol") ? "mySymbol" : "symbol";
        }

        private boolean hasField(String[] fields, String name) {
            for (String field : fields) {
                if (field.equals(name)) {
                    return true;
                }
            }
            return false;
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
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String property : new TreeSet<>(Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(property, normalize(inner.get(property)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
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
