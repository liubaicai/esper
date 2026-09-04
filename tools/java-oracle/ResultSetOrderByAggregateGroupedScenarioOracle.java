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
import java.util.Collections;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOrderByAggregateGrouped ordinals
 * zero through seven.  Each execution is replayed in a fresh runtime and the
 * observation is pinned per execution: ordinals zero through four emit one
 * six-row listener callback at the six-event output boundary, ordinals five
 * and seven (output last every 6 events) emit two three-row listener batches,
 * and ordinal six is a continuous grouped join observed through two statement
 * iterator snapshots while its attached listener never records.  The
 * compile-only execution replays EPL-to-model compilation, while the SODA
 * execution builds, serializes, validates, annotates, and compiles its model.
 */
public final class ResultSetOrderByAggregateGroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-aggregate-grouped";
    private static final String DESCRIPTION =
            "ResultSetOrderByAggregateGrouped ordinals 0-7: grouped aggregate order-by aliases, row-per-event switch, output-last batches, and grouped-join iterator snapshots.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByAggregateGrouped.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-46cf1731d511ce733720",
            "java-runtime-99e2349823d5d0643cb7",
            "java-runtime-85ba512296ed6b671ece",
            "java-runtime-8295fe09d8727195fb80",
            "java-runtime-08747f9055632cef41c6",
            "java-runtime-12d689fc781af934b76d",
            "java-runtime-680e7b6016501fed42cb",
            "java-runtime-18eacf385e5ac00e8a80"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAliasesAggregationCompile",
            "ResultSetAliasesAggregationOM",
            "ResultSetAliases",
            "ResultSetGroupBySwitch",
            "ResultSetGroupBySwitchJoin",
            "ResultSetLastJoin",
            "ResultSetIterator",
            "ResultSetLast"
    };
    private static final String[] STATIC_IDS = {
            "java-a8a71b78b77eec10ee62",
            "java-015649f9c0449597e55a",
            "java-e002cbfd72133e0c59f0",
            "java-610fffded9e2415c805e",
            "java-b312c9571904401d3fdf",
            "java-663c2d183bbd4dc98fa2",
            "java-0de948d5133afc56e9fe",
            "java-726632af45ffc7989e27"
    };
    private static final String[] CASES = {
            "aliases-aggregation-compile",
            "aliases-aggregation-om",
            "aliases",
            "group-by-switch",
            "group-by-switch-join",
            "last-join",
            "iterator",
            "last"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5, 6, 7};
    private static final String[] EPLS = {
            "@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
            "select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol",
            "@name('s0') select symbol, volume, sum(price) as mySum from SupportMarketDataBean#length(20) group by symbol output every 6 events order by mySum, symbol",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) group by symbol output every 6 events order by sum(price), symbol, volume",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(20) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol output every 6 events order by sum(price), symbol, volume",
            "@name('s0') select symbol, volume, sum(price) from SupportMarketDataBean#length(20) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol output last every 6 events order by sum(price)",
            "@name('s0') select symbol, theString, sum(price) as sumPrice from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString group by symbol order by symbol",
            "@name('s0') select symbol, volume, sum(price) from SupportMarketDataBean#length(20) group by symbol output last every 6 events order by sum(price)"
    };

    private static final String[] MARKET_SYMBOLS = {"IBM", "IBM", "CMU", "CMU", "CAT", "CAT"};
    private static final long[] MARKET_VOLUMES = {110L, 120L, 130L, 140L, 150L, 160L};
    private static final double[] MARKET_PRICES = {3d, 4d, 1d, 2d, 5d, 6d};
    private static final String[] EXPECTED_SYMBOLS = {"CMU", "CMU", "IBM", "CAT", "IBM", "CAT"};
    private static final long[] EXPECTED_VOLUMES = {130L, 140L, 110L, 150L, 120L, 160L};
    private static final double[] EXPECTED_SUMS = {1d, 3d, 3d, 5d, 7d, 11d};
    private static final String[] JOIN_SEEDS = {"CAT", "IBM", "CMU", "KGB", "DOG"};
    private static final String[] LAST_BATCH_MARKET_SYMBOLS =
            {"IBM", "IBM", "CMU", "CMU", "CAT", "CAT", "IBM", "IBM", "CMU", "CMU", "DOG", "DOG"};
    private static final long[] LAST_BATCH_MARKET_VOLUMES =
            {101L, 102L, 103L, 104L, 105L, 106L, 201L, 202L, 203L, 204L, 205L, 206L};
    private static final double[] LAST_BATCH_MARKET_PRICES =
            {3d, 4d, 1d, 2d, 5d, 6d, 3d, 4d, 5d, 5d, 0d, 1d};
    private static final String[] LAST_BATCH_SYMBOLS = {"CMU", "IBM", "CAT", "DOG", "CMU", "IBM"};
    private static final long[] LAST_BATCH_VOLUMES = {104L, 102L, 106L, 206L, 204L, 202L};
    private static final double[] LAST_BATCH_SUMS = {3d, 7d, 11d, 1d, 13d, 14d};
    private static final String[] ITERATOR_MARKET_SYMBOLS = {"CAT", "IBM", "CAT", "IBM"};
    private static final long[] ITERATOR_MARKET_VOLUMES = {0L, 0L, 0L, 0L};
    private static final double[] ITERATOR_MARKET_PRICES = {50d, 49d, 15d, 100d};
    private static final String[] ITERATOR_FIELDS = {"sumPrice", "symbol", "theString"};
    private static final String[] ITERATOR_SNAPSHOT_ONE_SYMBOLS = {"CAT", "CAT", "IBM", "IBM"};
    private static final double[] ITERATOR_SNAPSHOT_ONE_SUMS = {65d, 65d, 149d, 149d};
    private static final String[] ITERATOR_SNAPSHOT_TWO_SYMBOLS = {"CAT", "CAT", "IBM", "IBM", "KGB"};
    private static final double[] ITERATOR_SNAPSHOT_TWO_SUMS = {65d, 65d, 149d, 149d, 75d};
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
        if (records.size() != 11) {
            throw new IllegalStateException("expected eleven listener and snapshot records, got "
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
                "ResultSetOrderByAggregateGroupedScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = compileCase(caseIndex, configuration, runtime);
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
        if (caseIndex == 6) {
            return 0;
        }
        return caseIndex < 5 ? 1 : 2;
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
                ? ITERATOR_SNAPSHOT_ONE_SYMBOLS
                : ITERATOR_SNAPSHOT_TWO_SYMBOLS;
        double[] expectedSums = snapshotIndex == 0
                ? ITERATOR_SNAPSHOT_ONE_SUMS
                : ITERATOR_SNAPSHOT_TWO_SUMS;
        if (drained.size() != expectedSymbols.length) {
            throw new IllegalStateException("case " + caseName + " iterator snapshot "
                    + (snapshotIndex + 1) + " must contain " + expectedSymbols.length
                    + " rows, got " + drained.size());
        }
        List<String> actualKeys = new ArrayList<>();
        for (EventBean event : drained) {
            String[] fields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(fields);
            if (!Arrays.equals(fields, ITERATOR_FIELDS)) {
                throw new IllegalStateException("case " + caseName + " iterator snapshot fields are "
                        + Arrays.toString(fields) + ", expected " + Arrays.toString(ITERATOR_FIELDS));
            }
            Object sumPrice = event.get("sumPrice");
            if (!(sumPrice instanceof Number)) {
                throw new IllegalStateException("case " + caseName + " iterator snapshot row "
                        + actualKeys.size() + " sumPrice must be numeric, got " + sumPrice);
            }
            actualKeys.add(rowKey(event.get("symbol"), event.get("theString"),
                    ((Number) sumPrice).doubleValue()));
        }
        List<String> expectedKeys = new ArrayList<>();
        for (int index = 0; index < expectedSymbols.length; index++) {
            expectedKeys.add(rowKey(expectedSymbols[index], expectedSymbols[index], expectedSums[index]));
        }
        Collections.sort(actualKeys);
        Collections.sort(expectedKeys);
        if (!actualKeys.equals(expectedKeys)) {
            throw new IllegalStateException("case " + caseName + " iterator snapshot "
                    + (snapshotIndex + 1) + " rows are not pinned: " + actualKeys);
        }
    }

    private static String rowKey(Object symbol, Object theString, double sumPrice) {
        return symbol + "|" + theString + "|" + sumPrice;
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
            throw new IllegalArgumentException("scenario must contain exactly eight cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            String expectedObservation = index == 6 ? "iterator" : "listener";
            int expectedSnapshots = index == 6 ? 2 : 0;
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !expectedObservation.equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != expectedSnapshots
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 84) {
            throw new IllegalArgumentException("scenario must contain exactly eighty-four steps");
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
        validateCaseMarker(steps.get(offset++), CASES[5]);
        for (String seed : JOIN_SEEDS) {
            validateStringStep(steps.get(offset++), seed);
        }
        for (int eventIndex = 0; eventIndex < LAST_BATCH_MARKET_SYMBOLS.length; eventIndex++) {
            validateMarketStep(steps.get(offset++), LAST_BATCH_MARKET_SYMBOLS[eventIndex],
                    LAST_BATCH_MARKET_VOLUMES[eventIndex], LAST_BATCH_MARKET_PRICES[eventIndex]);
        }
        validateCaseMarker(steps.get(offset++), CASES[6]);
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
        validateCaseMarker(steps.get(offset++), CASES[7]);
        for (int eventIndex = 0; eventIndex < LAST_BATCH_MARKET_SYMBOLS.length; eventIndex++) {
            validateMarketStep(steps.get(offset++), LAST_BATCH_MARKET_SYMBOLS[eventIndex],
                    LAST_BATCH_MARKET_VOLUMES[eventIndex], LAST_BATCH_MARKET_PRICES[eventIndex]);
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
            int expectedRows = caseIndex < 5 ? EXPECTED_SYMBOLS.length : 3;
            if (newEvents == null || newEvents.length != expectedRows
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + expectedRows + " new rows only");
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
            sequence = next;
            records.add(record);
        }

        private void validateRows(EventBean[] events, int callbackSequence) {
            String[] expectedFields;
            if (caseIndex < 3) {
                expectedFields = new String[]{"mySum", "symbol", "volume"};
            } else if (caseIndex < 5) {
                expectedFields = new String[]{"sum(price)", "symbol"};
            } else {
                expectedFields = new String[]{"sum(price)", "symbol", "volume"};
            }
            String sumField = caseIndex < 3 ? "mySum" : "sum(price)";
            int offset = (callbackSequence - 1) * 3;
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                int expectedIndex = caseIndex < 5 ? rowIndex : offset + rowIndex;
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                String[] sortedExpected = expectedFields.clone();
                Arrays.sort(sortedExpected);
                if (!Arrays.equals(fields, sortedExpected)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                assertString(event.get("symbol"),
                        caseIndex < 5 ? EXPECTED_SYMBOLS[expectedIndex] : LAST_BATCH_SYMBOLS[expectedIndex],
                        "symbol", expectedIndex);
                assertNumber(event.get(sumField),
                        caseIndex < 5 ? EXPECTED_SUMS[expectedIndex] : LAST_BATCH_SUMS[expectedIndex],
                        sumField, expectedIndex);
                if (caseIndex < 3) {
                    assertNumber(event.get("volume"), EXPECTED_VOLUMES[expectedIndex],
                            "volume", expectedIndex);
                } else if (caseIndex >= 5) {
                    assertNumber(event.get("volume"), LAST_BATCH_VOLUMES[expectedIndex],
                            "volume", expectedIndex);
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
