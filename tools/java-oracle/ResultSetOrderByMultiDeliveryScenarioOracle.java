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
import java.time.format.DateTimeParseException;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the ResultSetOrderByMultiDelivery execution of
 * ResultSetOrderBySimple, the ESPER-409 / QWY-933597 order-by delivery batching
 * contract: order-by sorts each delivered batch, and delivery granularity is
 * governed by the statement shape instead of one callback per event.  Part one
 * replays the plain pattern execution; the arming A-events never notify, and
 * the single B-event delivers exactly one new-only two-row batch ordered by
 * a.theString descending.  Part two replays the pattern with output-every-3-
 * events; four events deliver exactly one new-only three-row batch ordered by
 * a.theString descending.  Part three replays the grouped time window under
 * externally advanced time; events enter at second one and the eleven-second
 * advance expires both groups, delivering exactly one new-only two-row rstream
 * batch ordered by theString descending at time 11000.
 */
public final class ResultSetOrderByMultiDeliveryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-multi-delivery";
    private static final String DESCRIPTION =
            "ResultSetOrderByMultiDelivery: pattern and grouped-time delivery batching with order-by over each delivered batch.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-1c3d57ae9e8d4bca739f"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOrderByMultiDelivery"
    };
    private static final String[] STATIC_IDS = {
            "java-5151c7da40772952b49a"
    };
    private static final String[] CASES = {
            "multi-delivery-pattern",
            "multi-delivery-pattern-output-limit",
            "multi-delivery-groupwin-time"
    };
    private static final int[] ORDINALS = {0, 0, 0};
    private static final String[] EPLS = {
            "@name('s0') select a.theString from pattern [every a=SupportBean(theString like 'A%') "
                    + "-> b=SupportBean(theString like 'B%')] order by a.theString desc",
            "@name('s0') select a.theString from pattern [every a=SupportBean(theString like 'A%') "
                    + "-> b=SupportBean(theString like 'B%')] output every 3 events order by a.theString desc",
            "@name('s0') select rstream theString from SupportBean#groupwin(theString)#time(10) order by theString desc"
    };

    private static final String[] PATTERN_SEND_STRINGS = {"A1", "A2", "B"};
    private static final int[] PATTERN_SEND_INTS = {1, 2, 3};
    private static final String[] OUTPUT_LIMIT_SEND_STRINGS = {"A1", "A2", "A3", "B"};
    private static final int[] OUTPUT_LIMIT_SEND_INTS = {1, 2, 3, 3};
    private static final String[] GROUPWIN_SEND_STRINGS = {"A1", "A2"};
    private static final int[] GROUPWIN_SEND_INTS = {1, 1};
    private static final String[][] EXPECTED_FIELDS = {
            {"a.theString"},
            {"a.theString"},
            {"theString"}
    };
    private static final String[][] EXPECTED_VALUES = {
            {"A2", "A1"},
            {"A3", "A2", "A1"},
            {"A2", "A1"}
    };
    private static final int[] EXPECTED_CALLBACKS = {1, 1, 1};
    private static final long[] EXPECTED_TIMES = {0L, 0L, 11000L};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderByMultiDeliveryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByMultiDeliveryScenarioOracle <scenario.json>");
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
        if (records.size() != 3) {
            throw new IllegalStateException("expected three listener records, got "
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
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderByMultiDeliveryScenarioOracle-" + caseName, configuration);
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
        return EXPECTED_CALLBACKS[caseIndex];
    }

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime,
                               EPStatement statement, JsonArray records) {
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
            if ("advance-time".equals(operation)) {
                if (!"multi-delivery-groupwin-time".equals(caseName)) {
                    throw new IllegalArgumentException("advance-time step in case " + caseName
                            + " is not supported; only case multi-delivery-groupwin-time advances time");
                }
                advanceTime(step, runtime);
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
            sendEvent(runtime, step, caseName);
        }
    }

    private static void advanceTime(JsonObject step, EPRuntime runtime) {
        requireFields(step, "op", "at");
        Instant instant;
        try {
            instant = Instant.parse(string(step, "at"));
        } catch (DateTimeParseException ex) {
            throw new IllegalArgumentException("advance-time value is not an instant", ex);
        }
        runtime.getEventService().advanceTime(instant.toEpochMilli());
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = string(step, "eventType");
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalArgumentException("unknown event type " + eventType
                    + " in case " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "event payload");
        requireFields(payload, "theString", "intPrimitive");
        String theString = string(payload, "theString");
        int intPrimitive = integer(payload, "intPrimitive");
        runtime.getEventService().sendEventBean(new SupportBean(theString, intPrimitive),
                "SupportBean");
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
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[0].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[0].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 14) {
            throw new IllegalArgumentException("scenario must contain exactly fourteen steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        for (int eventIndex = 0; eventIndex < PATTERN_SEND_STRINGS.length; eventIndex++) {
            validateBeanStep(steps.get(offset++), PATTERN_SEND_STRINGS[eventIndex],
                    PATTERN_SEND_INTS[eventIndex]);
        }
        validateCaseMarker(steps.get(offset++), CASES[1]);
        for (int eventIndex = 0; eventIndex < OUTPUT_LIMIT_SEND_STRINGS.length; eventIndex++) {
            validateBeanStep(steps.get(offset++), OUTPUT_LIMIT_SEND_STRINGS[eventIndex],
                    OUTPUT_LIMIT_SEND_INTS[eventIndex]);
        }
        validateCaseMarker(steps.get(offset++), CASES[2]);
        validateAdvanceTimeStep(steps.get(offset++), "1970-01-01T00:00:01Z");
        for (int eventIndex = 0; eventIndex < GROUPWIN_SEND_STRINGS.length; eventIndex++) {
            validateBeanStep(steps.get(offset++), GROUPWIN_SEND_STRINGS[eventIndex],
                    GROUPWIN_SEND_INTS[eventIndex]);
        }
        validateAdvanceTimeStep(steps.get(offset++), "1970-01-01T00:00:11Z");
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

    private static void validateBeanStep(JsonValue value, String expectedString,
                                         int expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload, "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateAdvanceTimeStep(JsonValue value, String expectedInstant) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op"))) {
            throw new IllegalArgumentException("advance-time step is not pinned");
        }
        if (!expectedInstant.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time value is not pinned");
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
            int expectedRows = EXPECTED_VALUES[caseIndex].length;
            if (newEvents == null || newEvents.length != expectedRows
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + expectedRows + " new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != EXPECTED_TIMES[caseIndex]) {
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
            String[] expectedValues = EXPECTED_VALUES[caseIndex];
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, expectedFields)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                assertString(event.get(expectedFields[0]), expectedValues[rowIndex],
                        expectedFields[0], rowIndex, CASES[caseIndex]);
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

    /** Local mirror of the pinned SupportBean regression bean. */
    public static final class SupportBean {
        private String theString;
        private int intPrimitive;

        public SupportBean(String theString, int intPrimitive) {
            this.theString = theString;
            this.intPrimitive = intPrimitive;
        }

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }
    }
}
