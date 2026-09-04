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
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.GroupByClause;
import com.espertech.esper.common.client.soda.OrderByClause;
import com.espertech.esper.common.client.soda.OutputLimitClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.View;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOrderByAggregateGrouped ordinals
 * zero through four.  Each execution is replayed in a fresh runtime and
 * emits the one listener callback produced by the six-event output boundary.
 * The compile-only execution replays EPL-to-model compilation, while the SODA
 * execution builds, serializes, validates, annotates, and compiles its model.
 */
public final class ResultSetOrderByAggregateGroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-aggregate-grouped";
    private static final String DESCRIPTION =
            "ResultSetOrderByAggregateGrouped ordinals 0-4: grouped aggregate order-by aliases and row-per-event switch with listener output.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByAggregateGrouped.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-46cf1731d511ce733720",
            "java-runtime-99e2349823d5d0643cb7",
            "java-runtime-85ba512296ed6b671ece",
            "java-runtime-8295fe09d8727195fb80",
            "java-runtime-08747f9055632cef41c6"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAliasesAggregationCompile",
            "ResultSetAliasesAggregationOM",
            "ResultSetAliases",
            "ResultSetGroupBySwitch",
            "ResultSetGroupBySwitchJoin"
    };
    private static final String[] STATIC_IDS = {
            "java-a8a71b78b77eec10ee62",
            "java-015649f9c0449597e55",
            "java-e002cbfd72133e0c59f0",
            "java-610fffded9e2415c805e",
            "java-b312c9571904401d3fdf"
    };
    private static final String[] CASES = {
            "aliases-aggregation-compile",
            "aliases-aggregation-om",
            "aliases",
            "group-by-switch",
            "group-by-switch-join"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4};
    private static final String[] EPLS = {
            "@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
            "select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
            "@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by mySum, symbol",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol, volume",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol output every 6 events order by sum(price), symbol, volume"
    };

    private static final String[] MARKET_SYMBOLS = {"IBM", "IBM", "CMU", "CMU", "CAT", "CAT"};
    private static final long[] MARKET_VOLUMES = {110L, 120L, 130L, 140L, 150L, 160L};
    private static final double[] MARKET_PRICES = {3d, 4d, 1d, 2d, 5d, 6d};
    private static final String[] EXPECTED_SYMBOLS = {"CMU", "CMU", "IBM", "CAT", "IBM", "CAT"};
    private static final long[] EXPECTED_VOLUMES = {130L, 140L, 110L, 150L, 120L, 160L};
    private static final double[] EXPECTED_SUMS = {1d, 3d, 3d, 5d, 7d, 11d};
    private static final String[] JOIN_SEEDS = {"CAT", "IBM", "CMU", "KGB", "DOG"};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderByAggregateGroupedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByAggregateGroupedScenarioOracle <scenario.json>");
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
        if (records.size() != CASES.length) {
            throw new IllegalStateException("expected five listener records, got " + records.size());
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
                "ResultSetOrderByAggregateGroupedScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = compileCase(caseIndex, configuration, runtime);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime);
            statement.addListener(listener);
            replay(allSteps, caseName, runtime);
            if (listener.sequence != 1) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected one");
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

    private static EPCompiled compileCase(int caseIndex, Configuration configuration,
                                          EPRuntime runtime) throws Exception {
        if (caseIndex == 0) {
            EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(EPLS[caseIndex], configuration);
            model = SerializableObjectCopier.copyMayFail(model);
            if (!EPLS[caseIndex].trim().equals(model.toEPL())) {
                throw new IllegalStateException("aliases-aggregation-compile model toEPL mismatch: " + model.toEPL());
            }
            return compileModel(model, runtime);
        }
        if (caseIndex == 1) {
            EPStatementObjectModel model = new EPStatementObjectModel();
            model.setSelectClause(SelectClause.create("symbol", "volume")
                    .add(Expressions.sum("price"), "mySum"));
            model.setFromClause(FromClause.create(
                    FilterStream.create(SupportMarketDataBean.class.getSimpleName())
                            .addView(View.create("length", Expressions.constant(20)))));
            model.setGroupByClause(GroupByClause.create("symbol"));
            model.setOutputLimitClause(OutputLimitClause.create(6));
            model.setOrderByClause(OrderByClause.create(Expressions.sum("price"))
                    .add("symbol", false));
            model = SerializableObjectCopier.copyMayFail(model);
            if (!EPLS[1].equals(model.toEPL())) {
                throw new IllegalStateException("aliases-aggregation-om model toEPL mismatch: " + model.toEPL());
            }
            model.setAnnotations(java.util.Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
            return compileModel(model, runtime);
        }
        return EPCompilerProvider.getCompiler().compile(
                EPLS[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
    }

    private static EPCompiled compileModel(EPStatementObjectModel model, EPRuntime runtime) throws Exception {
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        return EPCompilerProvider.getCompiler().compile(module,
                new CompilerArguments(runtime.getRuntimePath()));
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
            throw new IllegalArgumentException("scenario must contain exactly five cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 40) {
            throw new IllegalArgumentException("scenario must contain exactly forty steps");
        }
        int offset = 0;
        for (int caseIndex = 0; caseIndex < 4; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            for (int eventIndex = 0; eventIndex < MARKET_SYMBOLS.length; eventIndex++) {
                validateMarketStep(steps.get(offset++), MARKET_SYMBOLS[eventIndex],
                        MARKET_VOLUMES[eventIndex], MARKET_PRICES[eventIndex]);
            }
        }
        validateCaseMarker(steps.get(offset++), CASES[4]);
        for (String seed : JOIN_SEEDS) {
            validateStringStep(steps.get(offset++), seed);
        }
        for (int eventIndex = 0; eventIndex < MARKET_SYMBOLS.length; eventIndex++) {
            validateMarketStep(steps.get(offset++), MARKET_SYMBOLS[eventIndex],
                    MARKET_VOLUMES[eventIndex], MARKET_PRICES[eventIndex]);
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
            if (newEvents == null || newEvents.length != EXPECTED_SYMBOLS.length
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain six new rows only");
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
            String[] expectedFields = caseIndex < 3
                    ? new String[]{"mySum", "symbol", "volume"}
                    : new String[]{"sum(price)", "symbol"};
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                String[] sortedExpected = expectedFields.clone();
                Arrays.sort(sortedExpected);
                if (!Arrays.equals(fields, sortedExpected)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                assertString(event.get("symbol"), EXPECTED_SYMBOLS[rowIndex], "symbol", rowIndex);
                assertNumber(event.get(caseIndex < 3 ? "mySum" : "sum(price)"),
                        EXPECTED_SUMS[rowIndex], caseIndex < 3 ? "mySum" : "sum(price)", rowIndex);
                if (caseIndex < 3) {
                    assertNumber(event.get("volume"), EXPECTED_VOLUMES[rowIndex],
                            "volume", rowIndex);
                }
            }
        }

        private void assertString(Object actual, String expected, String field, int rowIndex) {
            if (!expected.equals(actual)) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " row " + rowIndex
                        + " " + field + " expected " + expected + ", got " + actual);
            }
        }

        private void assertNumber(Object actual, double expected, String field, int rowIndex) {
            if (!(actual instanceof Number)
                    || Double.compare(((Number) actual).doubleValue(), expected) != 0) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " row " + rowIndex
                        + " " + field + " expected " + expected + ", got " + actual);
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
