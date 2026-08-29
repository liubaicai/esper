import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.GroupByClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.StreamSelector;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetAggregateMedianAndDeviation ordinals 0-2. */
public final class ResultSetAggregateMedianAndDeviationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-median-and-deviation";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMedianAndDeviation.java";
    private static final String DESCRIPTION =
            "ResultSetAggregateMedianAndDeviation ordinals 0-2: grouped and join median, distinct median, sample stddev, and avedev over the finite DELL price sequence 10,20,20,90,5,90,30. The source NaN sensitivity phase is covered by a focused Go semantic test and is not represented in JSON.";

    private static final String STMT_MAIN = "stmt-main";
    private static final String JOIN_OM = "join-om";
    private static final String JOIN_EPL = "join-epl";
    private static final String[] CASES = {STMT_MAIN, JOIN_OM, JOIN_EPL};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIMES = {
            "java-runtime-101db664ea346721e986",
            "java-runtime-2f533a8a1bc0d93ae649",
            "java-runtime-2667a7eadeb7a458d7ce"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateStmt", "ResultSetAggregateStmtJoinOM", "ResultSetAggregateStmtJoin"
    };
    private static final String[] STATIC_IDS = {
            "java-b11b233b0ea7da05217f", "java-66a516ce6e14456627b2", "java-a5c65b307ecdb2d0355b"
    };
    private static final int[] SEND_COUNTS = {7, 10, 10};
    private static final int[] RECORD_COUNTS = {7, 7, 7};
    private static final double[] PRICES = {10d, 20d, 20d, 90d, 5d, 90d, 30d};
    private static final String[] SEED_SYMBOLS = {"DELL", "IBM", "AAA"};

    private ResultSetAggregateMedianAndDeviationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateMedianAndDeviationScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be an object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }
        if (records.size() != 21) {
            throw new IllegalStateException("expected 21 listener records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
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

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version")) || !ID.equals(string(scenario, "id"))) {
            throw new IllegalArgumentException("unsupported scenario identity");
        }
        if (!DESCRIPTION.equals(string(scenario, "description"))) {
            throw new IllegalArgumentException("scenario description is not pinned");
        }
        if (!JAVA_COMMIT.equals(string(scenario, "javaCommit"))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        JsonArray definitions = caseValue.asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject definition = object(definitions.get(caseIndex), "case definition " + caseIndex);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation", "epl");
            if (!CASES[caseIndex].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[caseIndex]
                    || !RUNTIMES[caseIndex].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[caseIndex].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || !eplFor(CASES[caseIndex]).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + caseIndex);
            }
        }

        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray() || stepsValue.asArray().size() != 30) {
            throw new IllegalArgumentException("scenario must contain exactly 30 steps");
        }
        JsonArray steps = stepsValue.asArray();
        int stepIndex = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(stepIndex), "case marker " + caseIndex);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            stepIndex++;
            if (caseIndex > 0) {
                for (String seed : SEED_SYMBOLS) {
                    validateStringSend(object(steps.get(stepIndex), "string send " + stepIndex), seed, stepIndex);
                    stepIndex++;
                }
            }
            for (int eventIndex = 0; eventIndex < PRICES.length; eventIndex++) {
                validateMarketSend(object(steps.get(stepIndex), "market send " + stepIndex),
                        PRICES[eventIndex], stepIndex);
                stepIndex++;
            }
        }
        if (stepIndex != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            double number = value.asDouble();
            long integral = value.asLong();
            if (!Double.isFinite(number) || number != integral || integral < Integer.MIN_VALUE
                    || integral > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            }
            return (int) integral;
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = values.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static void validateStringSend(JsonObject step, String expected, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBeanString".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send SupportBeanString");
        }
        JsonObject payload = object(step.get("payload"), "string payload " + stepIndex);
        requireFields(payload, "theString");
        if (!expected.equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("step " + stepIndex + " string payload mismatch");
        }
    }

    private static void validateMarketSend(JsonObject step, double expectedPrice, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send SupportMarketDataBean");
        }
        JsonObject payload = object(step.get("payload"), "market payload " + stepIndex);
        requireFields(payload, "symbol", "price", "volume");
        if (!"DELL".equals(string(payload, "symbol"))) {
            throw new IllegalArgumentException("step " + stepIndex + " symbol must be DELL");
        }
        JsonValue price = payload.get("price");
        if (price == null || !price.isNumber() || !Double.isFinite(price.asDouble())
                || Double.compare(price.asDouble(), expectedPrice) != 0) {
            throw new IllegalArgumentException("step " + stepIndex + " price mismatch");
        }
        if (integer(payload, "volume") != 0) {
            throw new IllegalArgumentException("step " + stepIndex + " volume must be integer zero");
        }
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        String runtimeURI = "parity-resultset-aggregate-median-and-deviation-" + caseName + "-" + RUNTIMES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        marketType.put("volume", Long.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        if (JOIN_OM.equals(caseName) || JOIN_EPL.equals(caseName)) {
            Map<String, Object> stringType = new HashMap<>();
            stringType.put("theString", String.class);
            configuration.getCommon().addEventType("SupportBeanString", stringType);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = JOIN_OM.equals(caseName)
                    ? compileJoinOM(runtime)
                    : EPCompilerProvider.getCompiler().compile(eplFor(caseName),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment, caseName);
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replay(steps, caseIndex, runtime);
            if (writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.sequence
                        + " listener records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPCompiled compileJoinOM(EPRuntime runtime) throws Exception {
        EPStatementObjectModel model = new EPStatementObjectModel();
        model.setSelectClause(SelectClause.create("symbol")
                .add(Expressions.median("price"), "myMedian")
                .add(Expressions.medianDistinct("price"), "myDistMedian")
                .add(Expressions.stddev("price"), "myStdev")
                .add(Expressions.avedev("price"), "myAvedev")
                .streamSelector(StreamSelector.RSTREAM_ISTREAM_BOTH));
        model.setFromClause(FromClause.create(
                FilterStream.create("SupportBeanString", "one")
                        .addView(View.create("length", Expressions.constant(100))),
                FilterStream.create("SupportMarketDataBean", "two")
                        .addView(View.create("length", Expressions.constant(5)))));
        model.setWhereClause(Expressions.and().add(
                Expressions.or()
                        .add(Expressions.eq("symbol", "DELL"))
                        .add(Expressions.eq("symbol", "IBM"))
                        .add(Expressions.eq("symbol", "GE")))
                .add(Expressions.eqProperty("one.theString", "two.symbol")));
        model.setGroupByClause(GroupByClause.create("symbol"));
        model = SerializableObjectCopier.copyMayFail(model);
        String expected = eplFor(JOIN_OM);
        if (!expected.equals(model.toEPL())) {
            throw new IllegalStateException("join-om model toEPL mismatch: " + model.toEPL());
        }
        model.setAnnotations(Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        return EPCompilerProvider.getCompiler().compile(module,
                new CompilerArguments(runtime.getRuntimePath()));
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
    }

    private static String eplFor(String caseName) {
        switch (caseName) {
            case STMT_MAIN:
                return "@name('s0') select irstream symbol,median(all price) as myMedian,"
                        + "median(distinct price) as myDistMedian,stddev(all price) as myStdev,"
                        + "avedev(all price) as myAvedev from SupportMarketDataBean#length(5) "
                        + "where symbol='DELL' or symbol='IBM' or symbol='GE' group by symbol";
            case JOIN_OM:
                return "select irstream symbol, median(price) as myMedian, "
                        + "median(distinct price) as myDistMedian, stddev(price) as myStdev, "
                        + "avedev(price) as myAvedev from SupportBeanString#length(100) as one, "
                        + "SupportMarketDataBean#length(5) as two where (symbol=\"DELL\" or symbol=\"IBM\" or symbol=\"GE\") "
                        + "and one.theString=two.symbol group by symbol";
            case JOIN_EPL:
                return "@name('s0') select irstream symbol,median(price) as myMedian,"
                        + "median(distinct price) as myDistMedian,stddev(price) as myStdev,"
                        + "avedev(price) as myAvedev from SupportBeanString#length(100) as one, "
                        + "SupportMarketDataBean#length(5) as two where (symbol='DELL' or symbol='IBM' or symbol='GE') "
                        + "       and one.theString = two.symbol group by symbol";
            default:
                throw new IllegalArgumentException("unsupported case " + caseName);
        }
    }

    private static void replay(JsonArray steps, int caseIndex, EPRuntime runtime) {
        String caseName = CASES[caseIndex];
        boolean active = false;
        int sends = 0;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                active = caseName.equals(string(step, "case"));
                continue;
            }
            if (!active) {
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
            }
            String eventType = string(step, "eventType");
            JsonObject payload = object(step.get("payload"), "payload " + index);
            if ("SupportBeanString".equals(eventType)) {
                Map<String, Object> event = new HashMap<>();
                event.put("theString", string(payload, "theString"));
                runtime.getEventService().sendEventMap(event, eventType);
            } else if ("SupportMarketDataBean".equals(eventType)) {
                Map<String, Object> event = new HashMap<>();
                event.put("symbol", string(payload, "symbol"));
                event.put("price", payload.get("price").asDouble());
                event.put("volume", (long) integer(payload, "volume"));
                runtime.getEventService().sendEventMap(event, eventType);
            } else {
                throw new IllegalArgumentException("unsupported event type in case " + caseName + ": " + eventType);
            }
            sends++;
        }
        if (sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalStateException("case " + caseName + " replayed " + sends
                    + " sends, expected " + SEND_COUNTS[caseIndex]);
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
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
            if (!hasNew || !hasOld || newEvents.length != 1 || oldEvents.length != 1
                    || sequence >= RECORD_COUNTS[0]) {
                throw new IllegalStateException("aggregate callback must contain one old and one new row");
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("new", rows(newEvents))
                    .add("old", rows(oldEvents));
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
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
                if (!Double.isFinite(number)) {
                    throw new IllegalStateException("non-finite value cannot be serialized");
                }
                if (number == Math.rint(number)) {
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
