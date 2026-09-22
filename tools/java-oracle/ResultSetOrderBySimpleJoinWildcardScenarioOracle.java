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
 * Direct Esper 9.0.0 oracle for ResultSetOrderBySimple ordinals 10-14.
 * Ordinal 10 (ResultSetMultipleKeysJoin) replays three multi-key order-by
 * variants over the SupportMarketDataBean/SupportBeanString join; ordinal 11
 * (ResultSetSimple) replays six single-key variants over a length(5) market
 * window; ordinal 12 (ResultSetSimpleJoin) replays the same six variants over
 * the join; ordinal 13 (ResultSetWildcard) replays two wildcard variants whose
 * rows expose the full bean property set including null id and feed; ordinal
 * 14 (ResultSetWildcardJoin) replays two join-wildcard variants whose rows
 * carry the bean-valued stream properties one and two. Every case sends the
 * shared six market events (IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5); join
 * cases then send the five SupportBeanString seeds (CAT, IBM, CMU, KGB, DOG)
 * whose matches generate the six join rows. Each case records exactly one
 * six-row new-only listener batch.
 */
public final class ResultSetOrderBySimpleJoinWildcardScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-simple-join-wildcard";
    private static final String DESCRIPTION =
            "ResultSetOrderBySimple ordinals 10-14: multi-key, simple, and wildcard "
                    + "order-by over length windows with output every 6 events; ordinals "
                    + "10, 12, and 14 join SupportMarketDataBean to SupportBeanString and "
                    + "send five string seeds after the shared six market events, ordinal "
                    + "13 selects the wildcard bean including null id and feed properties, "
                    + "and ordinal 14 selects the join wildcard as event-valued one/two "
                    + "stream properties.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-6c1c5e581115d83acac7",
            "java-runtime-2524d06789dd35e36c8e",
            "java-runtime-d530652f60616c332431",
            "java-runtime-9455a5a72c84baa7bdc2",
            "java-runtime-4aaec5e95dbea839ece3"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetMultipleKeysJoin",
            "ResultSetSimple",
            "ResultSetSimpleJoin",
            "ResultSetWildcard",
            "ResultSetWildcardJoin"
    };
    private static final String[] STATIC_IDS = {
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6"
    };
    private static final String[] CASES = {
            "multiple-keys-join-v1",
            "multiple-keys-join-v2",
            "multiple-keys-join-v3",
            "simple-v1",
            "simple-v2",
            "simple-v3",
            "simple-v4",
            "simple-v5",
            "simple-v6",
            "simple-join-v1",
            "simple-join-v2",
            "simple-join-v3",
            "simple-join-v4",
            "simple-join-v5",
            "simple-join-v6",
            "wildcard-v1",
            "wildcard-v2",
            "wildcard-join-v1",
            "wildcard-join-v2"
    };
    private static final int[] ORDINALS = {
            10, 10, 10,
            11, 11, 11, 11, 11, 11,
            12, 12, 12, 12, 12, 12,
            13, 13,
            14, 14
    };
    private static final String[] EPLS = {
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by symbol, price",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price, symbol, volume",
            "@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price, volume",
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol",
            "@name('s0') select price from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume*2 from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select symbol, volume from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by symbol",
            "@name('s0') select price from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by symbol, price",
            "@name('s0') select * from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price",
            "@name('s0') select * from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol",
            "@name('s0') select * from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select * from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by symbol, price"
    };
    private static final String[] OBSERVATIONS = {
            "listener; ordinal 10 variant 1 orders the six-row join output batch by "
                    + "symbol, price",
            "listener; ordinal 10 variant 2 orders the six-row join output batch by "
                    + "price, symbol, volume so the price-6 tie resolves CAT, CAT, IBM",
            "listener; ordinal 10 variant 3 selects symbol, volume*2 and orders the "
                    + "six-row join output batch by price, volume; the constant zero "
                    + "volume keeps join-generation order at the price-6 tie",
            "listener; ordinal 11 variant 1 orders the six-row output batch by price",
            "listener; ordinal 11 variant 2 selects symbol, price and orders the "
                    + "six-row output batch by price",
            "listener; ordinal 11 variant 3 selects symbol, volume and orders the "
                    + "six-row output batch by price",
            "listener; ordinal 11 variant 4 selects symbol, volume*2 and orders the "
                    + "six-row output batch by price",
            "listener; ordinal 11 variant 5 selects symbol, volume and orders the "
                    + "six-row output batch by symbol",
            "listener; ordinal 11 variant 6 selects price and orders the six-row "
                    + "output batch by symbol",
            "listener; ordinal 12 variant 1 orders the six-row join output batch by price",
            "listener; ordinal 12 variant 2 selects symbol, price and orders the "
                    + "six-row join output batch by price",
            "listener; ordinal 12 variant 3 selects symbol, volume and orders the "
                    + "six-row join output batch by price",
            "listener; ordinal 12 variant 4 selects symbol, volume*2 and orders the "
                    + "six-row join output batch by price",
            "listener; ordinal 12 variant 5 selects symbol, volume and orders the "
                    + "six-row join output batch by symbol",
            "listener; ordinal 12 variant 6 selects price and orders the six-row "
                    + "join output batch by symbol, price",
            "listener; ordinal 13 variant 1 selects the wildcard bean and orders the "
                    + "six-row output batch by price; rows expose the full property set "
                    + "including null id and feed",
            "listener; ordinal 13 variant 2 selects the wildcard bean and orders the "
                    + "six-row output batch by symbol; rows expose the full property set "
                    + "including null id and feed",
            "listener; ordinal 14 variant 1 selects the join wildcard and orders the "
                    + "six-row join output batch by price; rows carry the event-valued "
                    + "stream properties one and two",
            "listener; ordinal 14 variant 2 selects the join wildcard and orders the "
                    + "six-row join output batch by symbol, price; rows carry the "
                    + "event-valued stream properties one and two"
    };

    private static final String[] SEND_SYMBOLS = {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"};
    private static final double[] SEND_PRICES = {2d, 1d, 3d, 6d, 6d, 5d};
    private static final String[] JOIN_STRINGS = {"CAT", "IBM", "CMU", "KGB", "DOG"};

    private static final String[][] EXPECTED_FIELDS = {
            {"symbol"},
            {"symbol"},
            {"symbol", "volume*2"},
            {"symbol"},
            {"price", "symbol"},
            {"symbol", "volume"},
            {"symbol", "volume*2"},
            {"symbol", "volume"},
            {"price"},
            {"symbol"},
            {"price", "symbol"},
            {"symbol", "volume"},
            {"symbol", "volume*2"},
            {"symbol", "volume"},
            {"price"},
            {"feed", "id", "price", "symbol", "volume"},
            {"feed", "id", "price", "symbol", "volume"},
            {"one", "two"},
            {"one", "two"}
    };
    private static final String[][] EXPECTED_SYMBOLS = {
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            null,
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            null,
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"}
    };
    private static final double[][] EXPECTED_PRICES = {
            null,
            null,
            null,
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null,
            {6d, 5d, 3d, 2d, 6d, 1d},
            null,
            {1d, 2d, 3d, 5d, 6d, 6d},
            null,
            null,
            null,
            {5d, 6d, 3d, 2d, 6d, 1d},
            {1d, 2d, 3d, 5d, 6d, 6d},
            {6d, 5d, 3d, 2d, 6d, 1d},
            null,
            null
    };
    private static final String[] EXPECTED_EXTRA_NAMES = {
            null, null, "volume*2",
            null, null, "volume", "volume*2", "volume", null,
            null, null, "volume", "volume*2", "volume", null,
            "volume", "volume",
            null, null
    };
    private static final long[][] EXPECTED_EXTRA = {
            null, null, {0L, 0L, 0L, 0L, 0L, 0L},
            null, null, {0L, 0L, 0L, 0L, 0L, 0L}, {0L, 0L, 0L, 0L, 0L, 0L},
            {0L, 0L, 0L, 0L, 0L, 0L}, null,
            null, null, {0L, 0L, 0L, 0L, 0L, 0L}, {0L, 0L, 0L, 0L, 0L, 0L},
            {0L, 0L, 0L, 0L, 0L, 0L}, null,
            {0L, 0L, 0L, 0L, 0L, 0L}, {0L, 0L, 0L, 0L, 0L, 0L},
            null, null
    };
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderBySimpleJoinWildcardScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderBySimpleJoinWildcardScenarioOracle <scenario.json>");
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
        if (records.size() != 19) {
            throw new IllegalStateException("expected nineteen trace records, got "
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
                "ResultSetOrderBySimpleJoinWildcardScenarioOracle-" + caseName, configuration);
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
            if (listener.sequence != 1) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected 1");
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
            throw new IllegalArgumentException("scenario must contain exactly nineteen cases");
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
        if (steps.size() != 188) {
            throw new IllegalArgumentException("scenario must contain exactly 188 steps");
        }
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                validateMarketStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                        SEND_PRICES[eventIndex]);
            }
            if (ORDINALS[caseIndex] == 10 || ORDINALS[caseIndex] == 12 || ORDINALS[caseIndex] == 14) {
                for (String expected : JOIN_STRINGS) {
                    validateStringStep(steps.get(offset++), expected);
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String caseRuntimeId(String caseName) {
        return RUNTIME_IDS[caseOrdinal(caseName) - 10];
    }

    private static String caseExecutionName(String caseName) {
        return EXECUTION_NAMES[caseOrdinal(caseName) - 10];
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

    private static void assertNull(Object actual, String field, int rowIndex, String caseName) {
        if (actual != null) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected null, got " + actual);
        }
    }

    private static boolean wildcardJoinCase(int caseIndex) {
        return ORDINALS[caseIndex] == 14;
    }

    private static boolean wildcardCase(int caseIndex) {
        return ORDINALS[caseIndex] == 13;
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
            int next = sequence + 1;
            if (next > 1) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than one listener callback");
            }
            if (newEvents == null || newEvents.length != SEND_SYMBOLS.length) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain six new rows");
            }
            if (oldEvents != null && oldEvents.length != 0) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must not carry old rows");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            validateRows(newEvents);

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

        private void validateRows(EventBean[] events) {
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, EXPECTED_FIELDS[caseIndex])) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                if (wildcardJoinCase(caseIndex)) {
                    // assertSymbolsJoinWildCard: the wildcard join row carries the
                    // bean-valued stream properties one and two; the Java test
                    // asserts the one-side symbol sequence.
                    Object one = event.get("one");
                    if (!(one instanceof SupportMarketDataBean)) {
                        throw new IllegalStateException("case " + CASES[caseIndex] + " row "
                                + rowIndex + " one is not a SupportMarketDataBean: " + one);
                    }
                    assertString(((SupportMarketDataBean) one).getSymbol(),
                            EXPECTED_SYMBOLS[caseIndex][rowIndex], "one.symbol", rowIndex,
                            CASES[caseIndex]);
                    Object two = event.get("two");
                    if (!(two instanceof SupportBeanString)) {
                        throw new IllegalStateException("case " + CASES[caseIndex] + " row "
                                + rowIndex + " two is not a SupportBeanString: " + two);
                    }
                    assertString(((SupportBeanString) two).getTheString(),
                            EXPECTED_SYMBOLS[caseIndex][rowIndex], "two.theString", rowIndex,
                            CASES[caseIndex]);
                    continue;
                }
                if (EXPECTED_SYMBOLS[caseIndex] != null) {
                    assertString(event.get("symbol"), EXPECTED_SYMBOLS[caseIndex][rowIndex],
                            "symbol", rowIndex, CASES[caseIndex]);
                }
                if (EXPECTED_PRICES[caseIndex] != null) {
                    assertDouble(event.get("price"), EXPECTED_PRICES[caseIndex][rowIndex],
                            "price", rowIndex, CASES[caseIndex]);
                }
                if (EXPECTED_EXTRA_NAMES[caseIndex] != null) {
                    assertLong(event.get(EXPECTED_EXTRA_NAMES[caseIndex]),
                            EXPECTED_EXTRA[caseIndex][rowIndex],
                            EXPECTED_EXTRA_NAMES[caseIndex], rowIndex, CASES[caseIndex]);
                }
                if (wildcardCase(caseIndex)) {
                    assertNull(event.get("id"), "id", rowIndex, CASES[caseIndex]);
                    assertNull(event.get("feed"), "feed", rowIndex, CASES[caseIndex]);
                }
            }
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
        if (value instanceof SupportMarketDataBean) {
            // Wildcard join rows carry the underlying bean as the stream
            // property value; render it as the same nested row the Go side
            // emits for an event-valued column.
            SupportMarketDataBean bean = (SupportMarketDataBean) value;
            JsonObject fields = new JsonObject();
            fields.add("feed", normalize(bean.getFeed()));
            fields.add("id", normalize(bean.getId()));
            fields.add("price", normalize(bean.getPrice()));
            fields.add("symbol", normalize(bean.getSymbol()));
            fields.add("volume", normalize(bean.getVolume()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBeanString) {
            SupportBeanString bean = (SupportBeanString) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString()));
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
