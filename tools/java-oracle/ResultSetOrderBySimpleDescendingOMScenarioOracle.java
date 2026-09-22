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
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.OrderByClause;
import com.espertech.esper.common.client.soda.OutputLimitClause;
import com.espertech.esper.common.client.soda.SelectClause;
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
import java.util.Arrays;
import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the ResultSetDescendingOM (ordinal 3) and
 * ResultSetDescending (ordinal 4) executions of ResultSetOrderBySimple.
 * Ordinal 3 builds the statement through the SODA object model: the
 * SerializableObjectCopier round-trip and the model.toEPL() text assertion
 * are Java-API assertions with no typed Go boundary, so the scenario pins
 * them as "unrepresentable" records that this oracle verifies before
 * emitting; the deployed statement's runtime flow is identical to ordinal
 * 4 variant 1 (select symbol over a length(5) window, output every 6
 * events, order by price desc).  Ordinal 4 replays variants 2-6 of the
 * six-statement matrix: each variant deploys, sends the same six market
 * events (IBM@2, KGB@1, CMU@3, IBM@6, CAT@6, CAT@5), delivers exactly one
 * new-only six-row batch sorted by the pinned order-by keys, and
 * undeploys.
 */
public final class ResultSetOrderBySimpleDescendingOMScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-simple-descending-om";
    private static final String DESCRIPTION =
            "ResultSetOrderBySimple ordinals 3-4: descending order-by over a length(5) window "
                    + "with output every 6 events; ordinal 3's SODA object-model assertions "
                    + "(toEPL text, serialization) are pinned as unrepresentable records while "
                    + "its runtime flow replays identically to ordinal 4 variant 1, and ordinal 4 "
                    + "variants 2-6 cover price desc+symbol asc, price asc, symbol desc, "
                    + "symbol desc+price desc, and symbol+price.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-df8ea61ff025609ce309",
            "java-runtime-9668909b2b2f00769dab"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetDescendingOM",
            "ResultSetDescending"
    };
    private static final String[] STATIC_IDS = {
            "java-042c1b302e7183feb8a6",
            "java-042c1b302e7183feb8a6"
    };
    private static final String[] CASES = {
            "descending-om",
            "descending-v2",
            "descending-v3",
            "descending-v4",
            "descending-v5",
            "descending-v6"
    };
    private static final int[] ORDINALS = {3, 4, 4, 4, 4, 4};
    private static final String[] EPLS = {
            "select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price desc",
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price desc, symbol asc",
            "@name('s0') select symbol from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by price asc",
            "@name('s0') select symbol, volume from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol desc",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol desc, price desc",
            "@name('s0') select symbol, price from SupportMarketDataBean#length(5) "
                    + "output every 6 events order by symbol, price"
    };
    private static final String[] OBSERVATIONS = {
            "unrepresentable+listener; the SODA object-model assertions (serialization round-trip, "
                    + "toEPL text) have no typed Go boundary so they pin unrepresentable records; "
                    + "the deployed statement is annotated @name('s0') and its runtime flow is "
                    + "identical to ordinal 4 variant 1, delivering one six-row batch ordered by "
                    + "price desc",
            "listener; ordinal 4 variant 2 orders the six-row output batch by price desc, "
                    + "symbol asc so the price-6 tie resolves CAT before IBM",
            "listener; ordinal 4 variant 3 orders the six-row output batch by price asc",
            "listener; ordinal 4 variant 4 selects symbol, volume and orders the six-row output "
                    + "batch by symbol desc",
            "listener; ordinal 4 variant 5 selects symbol, price and orders the six-row output "
                    + "batch by symbol desc, price desc",
            "listener; ordinal 4 variant 6 selects symbol, price and orders the six-row output "
                    + "batch by symbol, price"
    };

    private static final String[] UNREPRESENTABLE_STATEMENTS = {
            "object-model-serialization",
            "object-model-to-epl"
    };
    private static final String[] UNREPRESENTABLE_NOTES = {
            "SerializableObjectCopier.copyMayFail round-trips the EPStatementObjectModel through "
                    + "Java serialization; no Go object-model serialization boundary exists",
            "EPStatementObjectModel.toEPL() renders 'select symbol from "
                    + "SupportMarketDataBean#length(5) output every 6 events order by price desc'; "
                    + "EPL text is not the typed Go entry point"
    };

    private static final String[] SEND_SYMBOLS = {"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"};
    private static final double[] SEND_PRICES = {2d, 1d, 3d, 6d, 6d, 5d};
    private static final String[][] EXPECTED_FIELDS = {
            {"symbol"},
            {"symbol"},
            {"symbol"},
            {"symbol", "volume"},
            {"price", "symbol"},
            {"price", "symbol"}
    };
    private static final String[][] EXPECTED_SYMBOLS = {
            {"IBM", "CAT", "CAT", "CMU", "IBM", "KGB"},
            {"CAT", "IBM", "CAT", "CMU", "IBM", "KGB"},
            {"KGB", "IBM", "CMU", "CAT", "IBM", "CAT"},
            {"KGB", "IBM", "IBM", "CMU", "CAT", "CAT"},
            {"KGB", "IBM", "IBM", "CMU", "CAT", "CAT"},
            {"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"}
    };
    private static final double[][] EXPECTED_PRICES = {
            null,
            null,
            null,
            null,
            {1d, 6d, 2d, 3d, 6d, 5d},
            {5d, 6d, 3d, 2d, 6d, 1d}
    };
    private static final long[] EXPECTED_VOLUMES = {0L, 0L, 0L, 0L, 0L, 0L};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderBySimpleDescendingOMScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderBySimpleDescendingOMScenarioOracle <scenario.json>");
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
        if (records.size() != 8) {
            throw new IllegalStateException("expected eight trace records, got "
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
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderBySimpleDescendingOMScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = compileCase(caseIndex, runtime);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime);
            statement.addListener(listener);
            replay(allSteps, caseName, runtime, records);
            if (listener.sequence != 1) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected 1");
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Ordinal 3 compiles the SODA object model exactly like the Java
     * execution: the SerializableObjectCopier round-trip and the toEPL
     * text assertion run before the s0 annotation is added and the model
     * is wrapped in a module for the compiler.  Ordinals 4 variants
     * compile the pinned EPL text.
     */
    private static EPCompiled compileCase(int caseIndex, EPRuntime runtime) throws Exception {
        if (caseIndex == 0) {
            EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(buildObjectModel());
            if (!EPLS[0].equals(model.toEPL())) {
                throw new IllegalStateException(
                        "object model toEPL drifted from the pinned stmtText: " + model.toEPL());
            }
            model.setAnnotations(Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
            Module module = new Module();
            module.getItems().add(new ModuleItem(model));
            module.setModuleText(model.toEPL());
            return EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(runtime.getRuntimePath()));
        }
        return EPCompilerProvider.getCompiler().compile(EPLS[caseIndex],
                new CompilerArguments(runtime.getRuntimePath()));
    }

    private static EPStatementObjectModel buildObjectModel() {
        EPStatementObjectModel model = new EPStatementObjectModel();
        model.setSelectClause(SelectClause.create("symbol"));
        model.setFromClause(FromClause.create(
                FilterStream.create("SupportMarketDataBean")
                        .addView("length", Expressions.constant(5))));
        model.setOutputLimitClause(OutputLimitClause.create(6));
        model.setOrderByClause(OrderByClause.create().add("price", true));
        return model;
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

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime,
                               JsonArray records) throws Exception {
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
            if ("unrepresentable".equals(operation)) {
                if (!"descending-om".equals(caseName)) {
                    throw new IllegalArgumentException("unrepresentable step in case " + caseName
                            + " is not supported; only case descending-om pins object-model assertions");
                }
                unrepresentableStep(caseName, step, records);
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
            sendEvent(runtime, step, caseName);
        }
    }

    /**
     * Emits the pinned "unrepresentable" record for the ordinal-3
     * object-model assertions after re-running the Java assertion the
     * record documents: the serialization round-trip rebuilds and copies
     * the model, and the toEPL step re-renders the pinned stmtText.
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            JsonArray records) throws Exception {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        int pinned = -1;
        for (int index = 0; index < UNREPRESENTABLE_STATEMENTS.length; index++) {
            if (UNREPRESENTABLE_STATEMENTS[index].equals(label)) {
                pinned = index;
                break;
            }
        }
        if (pinned < 0 || !UNREPRESENTABLE_NOTES[pinned].equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " carries an unpinned note");
        }
        if ("object-model-serialization".equals(label)) {
            EPStatementObjectModel copy = SerializableObjectCopier.copyMayFail(buildObjectModel());
            if (copy == null || !EPLS[0].equals(copy.toEPL())) {
                throw new IllegalStateException(
                        "object-model serialization round-trip drifted from the pinned stmtText");
            }
        } else if (!EPLS[0].equals(buildObjectModel().toEPL())) {
            throw new IllegalStateException(
                    "object-model toEPL drifted from the pinned stmtText");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = string(step, "eventType");
        if (!"SupportMarketDataBean".equals(eventType)) {
            throw new IllegalArgumentException("unknown event type " + eventType
                    + " in case " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "event payload");
        requireFields(payload, "symbol", "volume", "price");
        String symbol = string(payload, "symbol");
        long volume = longInteger(payload, "volume");
        double price = number(payload, "price");
        runtime.getEventService().sendEventBean(
                new SupportMarketDataBean(symbol, price, volume, null), "SupportMarketDataBean");
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
            throw new IllegalArgumentException("scenario must contain exactly six cases");
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
        if (steps.size() != 44) {
            throw new IllegalArgumentException("scenario must contain exactly forty-four steps");
        }
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            validateCaseMarker(steps.get(offset++), CASES[caseIndex]);
            if (caseIndex == 0) {
                for (int index = 0; index < UNREPRESENTABLE_STATEMENTS.length; index++) {
                    validateUnrepresentableStep(steps.get(offset++), CASES[0],
                            UNREPRESENTABLE_STATEMENTS[index], UNREPRESENTABLE_NOTES[index]);
                }
            }
            for (int eventIndex = 0; eventIndex < SEND_SYMBOLS.length; eventIndex++) {
                validateBeanStep(steps.get(offset++), SEND_SYMBOLS[eventIndex],
                        SEND_PRICES[eventIndex]);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String caseRuntimeId(String caseName) {
        return "descending-om".equals(caseName) ? RUNTIME_IDS[0] : RUNTIME_IDS[1];
    }

    private static String caseExecutionName(String caseName) {
        return "descending-om".equals(caseName) ? EXECUTION_NAMES[0] : EXECUTION_NAMES[1];
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateUnrepresentableStep(JsonValue value, String expectedCase,
                                                    String expectedStatement, String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !expectedCase.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + expectedStatement);
        }
    }

    private static void validateBeanStep(JsonValue value, String expectedSymbol,
                                         double expectedPrice) {
        JsonObject step = object(value, "SupportMarketDataBean step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportMarketDataBean step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload");
        requireFields(payload, "symbol", "volume", "price");
        if (!expectedSymbol.equals(string(payload, "symbol"))
                || longInteger(payload, "volume") != 0L
                || number(payload, "price") != expectedPrice) {
            throw new IllegalArgumentException("SupportMarketDataBean payload is not pinned");
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
        return value.asDouble();
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
        if (!(actual instanceof Number) || ((Number) actual).doubleValue() != expected) {
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
            int next = sequence + 1;
            if (next > 1) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than one listener callback");
            }
            if (newEvents == null || newEvents.length != SEND_SYMBOLS.length
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
            String[] expectedFields = EXPECTED_FIELDS[caseIndex];
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, expectedFields)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                assertString(event.get("symbol"), EXPECTED_SYMBOLS[caseIndex][rowIndex],
                        "symbol", rowIndex, CASES[caseIndex]);
                if (caseIndex == 3) {
                    assertLong(event.get("volume"), EXPECTED_VOLUMES[rowIndex],
                            "volume", rowIndex, CASES[caseIndex]);
                }
                if (EXPECTED_PRICES[caseIndex] != null) {
                    assertDouble(event.get("price"), EXPECTED_PRICES[caseIndex][rowIndex],
                            "price", rowIndex, CASES[caseIndex]);
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
        private String symbol;
        private String id;
        private double price;
        private Long volume;
        private String feed;

        public SupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() {
            return symbol;
        }

        public void setSymbol(String symbol) {
            this.symbol = symbol;
        }

        public String getId() {
            return id;
        }

        public void setId(String id) {
            this.id = id;
        }

        public double getPrice() {
            return price;
        }

        public void setPrice(double price) {
            this.price = price;
        }

        public Long getVolume() {
            return volume;
        }

        public void setVolume(Long volume) {
            this.volume = volume;
        }

        public String getFeed() {
            return feed;
        }

        public void setFeed(String feed) {
            this.feed = feed;
        }
    }
}
