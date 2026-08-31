import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetQueryTypeRowForAll ordinal 10. */
public final class ResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-row-for-all-select-avg-std-group-by-uni";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeRowForAll ordinal 10: average with standard group-by and unique price view.";
    private static final String RUNTIME_ID = "java-runtime-de3a01d12a22bb33ee43";
    private static final String EXECUTION_NAME = "ResultSetQueryTypeRowForAllSelectAvgStdGroupByUni";
    private static final String STATIC_ID = "java-7ba8b4893d30071e2fa5";
    private static final String CASE = "avg-std-group-by-uni";
    private static final String EPL =
            "@name('s0') select istream average as aprice from SupportMarketDataBean#groupwin(symbol)#length(2)#uni(price)";
    private static final String[] SORTED_FIELDS = {"aprice"};
    private static final String[] SYMBOLS = {"A", "B", "A", "A", "A"};
    private static final double[] PRICES = {1.0d, 3.0d, 3.0d, 10.0d, 20.0d};
    private static final double[] CALLBACK_AVERAGES = {1.0d, 3.0d, 2.0d, 6.5d, 15.0d};
    private static final int EXPECTED_TRACE_RECORDS = 4;

    private ResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        String runtimeURI = "parity-" + ID + "-" + RUNTIME_ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        JsonArray records = new JsonArray();
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime);
            statement.addListener(writer);
            replay(scenario.get("steps").asArray(), runtime);
            if (writer.callbackCount != CALLBACK_AVERAGES.length) {
                throw new IllegalStateException("expected five listener callbacks, got " + writer.callbackCount);
            }
            if (writer.sequence != EXPECTED_TRACE_RECORDS) {
                throw new IllegalStateException("expected four listener records, got " + writer.sequence);
            }
        } finally {
            runtime.destroy();
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static void replay(JsonArray steps, EPRuntime runtime) {
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation at step " + index);
            }
            String eventType = string(step, "eventType");
            if (!"SupportMarketDataBean".equals(eventType)) {
                throw new IllegalArgumentException("unexpected event type at step " + index);
            }
            JsonObject payload = object(step.get("payload"), "payload step " + index);
            requireFields(payload, "symbol", "id", "price", "volume", "feed");
            SupportMarketDataBean event = new SupportMarketDataBean(
                    string(payload, "symbol"), null, doubleNumber(payload, "price"), null, null);
            runtime.getEventService().sendEventBean(event, eventType);
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME_ID}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{EXECUTION_NAME}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(string(definition, "case"))
                || integer(definition, "ordinal") != 10
                || !RUNTIME_ID.equals(string(definition, "runtimeId"))
                || !EXECUTION_NAME.equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != SYMBOLS.length + 1) {
            throw new IllegalArgumentException("scenario must contain exactly six steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }
        for (int index = 0; index < SYMBOLS.length; index++) {
            validateMarketSend(steps, index + 1, SYMBOLS[index], PRICES[index]);
        }
    }

    private static void validateMarketSend(JsonArray steps, int index, String expectedSymbol,
                                           double expectedPrice) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step " + index + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "symbol", "id", "price", "volume", "feed");
        if (!expectedSymbol.equals(string(payload, "symbol"))) {
            throw new IllegalArgumentException("send step " + index + " symbol is not pinned");
        }
        requireNull(payload, "id");
        double actualPrice = doubleNumber(payload, "price");
        if (Double.compare(actualPrice, expectedPrice) != 0) {
            throw new IllegalArgumentException("send step " + index + " price is not pinned");
        }
        requireNull(payload, "volume");
        requireNull(payload, "feed");
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return value.asLong();
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static double doubleNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be a finite JSON number");
        }
        try {
            double result = value.asDouble();
            if (!Double.isFinite(result)) {
                throw new IllegalArgumentException(name + " must be a finite JSON number");
            }
            return result;
        } catch (RuntimeException ex) {
            if (ex instanceof IllegalArgumentException
                    && ex.getMessage() != null && ex.getMessage().startsWith(name + " must be")) {
                throw ex;
            }
            throw new IllegalArgumentException(name + " must be a finite JSON number", ex);
        }
    }

    private static void requireNull(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNull()) {
            throw new IllegalArgumentException(name + " must be JSON null");
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int callbackCount;
        private int sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int callback = callbackCount + 1;
            if (callback > CALLBACK_AVERAGES.length) {
                throw new IllegalStateException("unexpected listener callback after five callbacks");
            }
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || newEvents.length != 1 || hasOld) {
                throw new IllegalStateException("listener callback must contain one new row only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L || !"1970-01-01T00:00:00Z".equals(Instant.ofEpochMilli(now).toString())) {
                throw new IllegalStateException("unexpected callback time at callback " + callback);
            }
            double actual = average(newEvents);
            if (Double.compare(actual, CALLBACK_AVERAGES[callback - 1]) != 0) {
                throw new IllegalStateException("unexpected callback value at callback " + callback);
            }
            callbackCount = callback;
            // The Java regression calls listenerReset immediately after A/10.
            // That callback is validated but deliberately absent from the trace.
            if (callback == 4) {
                return;
            }
            int emitted = sequence + 1;
            sequence = emitted;
            JsonObject record = new JsonObject().add("case", CASE).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents));
            records.add(record);
        }

        private double average(EventBean[] events) {
            if (events.length != 1) {
                throw new IllegalStateException("expected one aggregate row");
            }
            String[] names = events[0].getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, SORTED_FIELDS)) {
                throw new IllegalStateException("aggregate field metadata is not pinned");
            }
            Object value = events[0].get("aprice");
            if (!(value instanceof Number)) {
                throw new IllegalStateException("aprice is not numeric");
            }
            return ((Number) value).doubleValue();
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray rows = new JsonArray();
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, SORTED_FIELDS)) {
                    throw new IllegalStateException("aggregate field metadata is not pinned");
                }
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                rows.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return rows;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Double || value instanceof Float) {
                double result = ((Number) value).doubleValue();
                if (!Double.isFinite(result)) {
                    throw new IllegalStateException("non-finite numeric result");
                }
                return Json.value(result);
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).doubleValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }

    /** Local mirror of the pinned SupportMarketDataBean regression bean. */
    public static final class SupportMarketDataBean {
        private final String symbol;
        private final String id;
        private final double price;
        private final Long volume;
        private final String feed;

        public SupportMarketDataBean(String symbol, String id, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.id = id;
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
}
