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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the join executions of ResultSetOrderBySimple,
 * ordinals one and two.  Ordinal one (ResultSetIterator) replays the continuous
 * length-window join observed through two ordered statement iterator snapshots
 * while its attached listener never records.  Ordinal two (ResultSetAcrossJoin)
 * replays both output-every-6-events join variants; each variant emits exactly
 * one six-row listener callback, ordered by price for the symbol/theString
 * projection and by theString then price for the symbol-only projection.
 */
public final class ResultSetOrderByJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-join";
    private static final String DESCRIPTION =
            "ResultSetOrderBySimple ordinals 1-2: join order-by with statement-iterator snapshots and output-limit over join deltas.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-53aea47cdd71b80fdbb9",
            "java-runtime-4a0adaff4741914b9ad1"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetIterator",
            "ResultSetAcrossJoin"
    };
    private static final String[] STATIC_IDS = {
            "java-3d5138ecf75168eb575d",
            "java-61243b98a3005c75950a"
    };
    private static final String[] CASES = {
            "iterator",
            "across-join-price",
            "across-join-symbol-price"
    };
    private static final int[] ORDINALS = {1, 2, 2};
    private static final String[] EPLS = {
            "@name('s0') select symbol, theString, price from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString order by price",
            "@name('s0') select symbol, theString from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by price",
            "@name('s0') select symbol from SupportMarketDataBean#length(10) as one, "
                    + "SupportBeanString#length(100) as two where one.symbol = two.theString "
                    + "output every 6 events order by theString, price"
    };

    private static final String[] JOIN_SEEDS = {"CAT", "IBM", "CMU", "KGB", "DOG"};
    private static final String[] ITERATOR_MARKET_SYMBOLS = {"CAT", "IBM", "CAT", "IBM"};
    private static final long[] ITERATOR_MARKET_VOLUMES = {0L, 0L, 0L, 0L};
    private static final double[] ITERATOR_MARKET_PRICES = {50d, 49d, 15d, 100d};
    private static final String[] ACROSS_MARKET_SYMBOLS = {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"};
    private static final long[] ACROSS_MARKET_VOLUMES = {0L, 0L, 0L, 0L, 0L, 0L};
    private static final double[] ACROSS_MARKET_PRICES = {2d, 1d, 3d, 6d, 6d, 5d};
    private static final String[] ITERATOR_FIELDS = {"price", "symbol", "theString"};
    private static final String[] SNAPSHOT_ONE_SYMBOLS = {"CAT", "IBM", "CAT", "IBM"};
    private static final double[] SNAPSHOT_ONE_PRICES = {15d, 49d, 50d, 100d};
    private static final String[] SNAPSHOT_TWO_SYMBOLS = {"CAT", "IBM", "CAT", "KGB", "IBM"};
    private static final double[] SNAPSHOT_TWO_PRICES = {15d, 49d, 50d, 75d, 100d};
    private static final String[] ACROSS_PRICE_SYMBOLS = {"KGB", "IBM", "CMU", "CAT", "CAT", "IBM"};
    private static final String[] ACROSS_SYMBOL_PRICE_SYMBOLS = {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderByJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByJoinScenarioOracle <scenario.json>");
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
        if (records.size() != 4) {
            throw new IllegalStateException("expected four listener and snapshot records, got "
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
                "ResultSetOrderByJoinScenarioOracle-" + caseName, configuration);
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
            replay(allSteps, caseName, runtime, statement, records);
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
        return caseIndex == 0 ? 0 : 1;
    }

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime,
                               EPStatement statement, JsonArray records) {
        boolean inCase = false;
        int snapshots = 0;
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
            if ("snapshot".equals(operation)) {
                if (!"iterator".equals(caseName)) {
                    throw new IllegalArgumentException("snapshot step in case " + caseName
                            + " is not supported; only case iterator carries snapshots");
                }
                emitSnapshot(step, statement, caseName, runtime, records, snapshots++);
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
            sendEvent(runtime, step, caseName);
        }
    }

    private static void emitSnapshot(JsonObject step, EPStatement statement, String caseName,
                                     EPRuntime runtime, JsonArray records, int snapshotIndex) {
        validateSnapshotStep(step);
        List<EventBean> drained = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            drained.add(iterator.next());
        }
        validateIteratorSnapshot(caseName, snapshotIndex, drained);
        long now = runtime.getEventService().getCurrentTime();
        if (now != 0L) {
            throw new IllegalStateException("unexpected snapshot time " + now);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(now).toString());
        record.add("new", rows(drained.toArray(new EventBean[0])));
        records.add(record);
    }

    private static void validateIteratorSnapshot(String caseName, int snapshotIndex,
                                                 List<EventBean> drained) {
        String[] expectedSymbols = snapshotIndex == 0
                ? SNAPSHOT_ONE_SYMBOLS
                : SNAPSHOT_TWO_SYMBOLS;
        double[] expectedPrices = snapshotIndex == 0
                ? SNAPSHOT_ONE_PRICES
                : SNAPSHOT_TWO_PRICES;
        if (drained.size() != expectedSymbols.length) {
            throw new IllegalStateException("case " + caseName + " iterator snapshot "
                    + (snapshotIndex + 1) + " must contain " + expectedSymbols.length
                    + " rows, got " + drained.size());
        }
        for (int rowIndex = 0; rowIndex < drained.size(); rowIndex++) {
            EventBean event = drained.get(rowIndex);
            String[] fields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(fields);
            if (!Arrays.equals(fields, ITERATOR_FIELDS)) {
                throw new IllegalStateException("case " + caseName + " iterator snapshot fields are "
                        + Arrays.toString(fields) + ", expected " + Arrays.toString(ITERATOR_FIELDS));
            }
            assertString(event.get("symbol"), expectedSymbols[rowIndex], "symbol", rowIndex, caseName);
            assertString(event.get("theString"), expectedSymbols[rowIndex], "theString", rowIndex,
                    caseName);
            assertNumber(event.get("price"), expectedPrices[rowIndex], "price", rowIndex, caseName);
        }
    }

    private static void assertString(Object actual, String expected, String field, int rowIndex,
                                     String caseName) {
        if (!expected.equals(actual)) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static void assertNumber(Object actual, double expected, String field, int rowIndex,
                                     String caseName) {
        if (!(actual instanceof Number)
                || Double.compare(((Number) actual).doubleValue(), expected) != 0) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "event payload");
        switch (eventType) {
            case "SupportMarketDataBean" -> {
                double price = number(payload, "price");
                long volume = longInteger(payload, "volume");
                String symbol = string(payload, "symbol");
                runtime.getEventService().sendEventBean(
                        new SupportMarketDataBean(symbol, price, volume, null),
                        "SupportMarketDataBean");
            }
            case "SupportBeanString" -> runtime.getEventService().sendEventBean(
                    new SupportBeanString(string(payload, "theString")), "SupportBeanString");
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
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            String expectedObservation = index == 0 ? "iterator" : "listener";
            int expectedSnapshots = index == 0 ? 2 : 0;
            int executionIndex = index == 0 ? 0 : 1;
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[executionIndex].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[executionIndex].equals(string(definition, "executionName"))
                    || !expectedObservation.equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != expectedSnapshots
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 37) {
            throw new IllegalArgumentException("scenario must contain exactly thirty-seven steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        for (String seed : JOIN_SEEDS) {
            validateStringStep(steps.get(offset++), seed);
        }
        for (int eventIndex = 0; eventIndex < ITERATOR_MARKET_SYMBOLS.length; eventIndex++) {
            validateMarketStep(steps.get(offset++), ITERATOR_MARKET_SYMBOLS[eventIndex],
                    ITERATOR_MARKET_VOLUMES[eventIndex], ITERATOR_MARKET_PRICES[eventIndex]);
        }
        validateSnapshotStep(steps.get(offset++));
        validateMarketStep(steps.get(offset++), "KGB", 0L, 75d);
        validateSnapshotStep(steps.get(offset++));
        for (int caseIndex = 1; caseIndex < CASES.length; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            for (int eventIndex = 0; eventIndex < ACROSS_MARKET_SYMBOLS.length; eventIndex++) {
                validateMarketStep(steps.get(offset++), ACROSS_MARKET_SYMBOLS[eventIndex],
                        ACROSS_MARKET_VOLUMES[eventIndex], ACROSS_MARKET_PRICES[eventIndex]);
            }
            for (String seed : JOIN_SEEDS) {
                validateStringStep(steps.get(offset++), seed);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateMarketStep(JsonValue value, String expectedSymbol,
                                           long expectedVolume, double expectedPrice) {
        JsonObject step = object(value, "market step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("market step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "market payload");
        requireFields(payload, "symbol", "volume", "price");
        if (!expectedSymbol.equals(string(payload, "symbol"))
                || longInteger(payload, "volume") != expectedVolume
                || Double.compare(number(payload, "price"), expectedPrice) != 0) {
            throw new IllegalArgumentException("market payload is not pinned");
        }
    }

    private static void validateSnapshotStep(JsonValue value) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "statement");
        if (!"snapshot".equals(string(step, "op"))
                || !"s0".equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned");
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
            if (expectedCallbacks == 0) {
                return;
            }
            int next = sequence + 1;
            if (next > expectedCallbacks) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than " + expectedCallbacks + " listener callbacks");
            }
            int expectedRows = ACROSS_PRICE_SYMBOLS.length;
            if (newEvents == null || newEvents.length != expectedRows
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + expectedRows + " new rows only");
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
            sequence = next;
            records.add(record);
        }

        private void validateRows(EventBean[] events) {
            boolean priceCase = caseIndex == 1;
            String[] expectedFields = priceCase
                    ? new String[]{"symbol", "theString"}
                    : new String[]{"symbol"};
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, expectedFields)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                String expectedSymbol = priceCase
                        ? ACROSS_PRICE_SYMBOLS[rowIndex]
                        : ACROSS_SYMBOL_PRICE_SYMBOLS[rowIndex];
                assertString(event.get("symbol"), expectedSymbol, "symbol", rowIndex,
                        CASES[caseIndex]);
                if (priceCase) {
                    assertString(event.get("theString"), expectedSymbol, "theString", rowIndex,
                            CASES[caseIndex]);
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
