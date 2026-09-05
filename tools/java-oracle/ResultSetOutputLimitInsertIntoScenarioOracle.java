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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the ResultSetOutputLimitInsertFirst and
 * ResultSetOutputLimitInsertSnapshot executions of ResultSetOutputLimitInsertInto:
 * output-limited insert-into producers observed through both the producing
 * statement s0 and the routed target stream consumer s1.  Part one replays the
 * insert-first execution: the first event of each one-second output interval is
 * delivered immediately to both listeners, the middle event of the interval
 * stays silent, and the first event after the rollover is delivered again.
 * Part two replays the insert-snapshot execution over a keepall window:
 * timer-driven snapshots deliver [E1] at time 1000 and [E1, E2] at time 2000
 * to both listeners, while the event sent between snapshots stays silent.  The
 * engine queues routed events into the thread work queue and processes them
 * after the producer's listener dispatch, so each emission records s0 before
 * s1.
 */
public final class ResultSetOutputLimitInsertIntoScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-insert-into";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitInsertInto: output-limited insert-into producers observed through both the producing statement and the routed target stream.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitInsertInto.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-e57a7555b3a0303c0303",
            "java-runtime-cd524991d69bc898c061"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOutputLimitInsertFirst",
            "ResultSetOutputLimitInsertSnapshot"
    };
    private static final String[] STATIC_IDS = {
            "java-00bb626878ef4282b5d2",
            "java-460f4166b6f5e1099aae"
    };
    private static final String[] CASES = {
            "insert-first",
            "insert-snapshot"
    };
    private static final int[] ORDINALS = {0, 1};
    private static final String[] EPLS = {
            "@name('s0') insert into MyStream select * from SupportBean output first every 1 second;"
                    + "@name('s1') select * from MyStream",
            "@name('s0') insert into MyStream select * from SupportBean#keepall output snapshot every 1 second;"
                    + "@name('s1') select * from MyStream"
    };

    private static final String[][] EXPECTED_ROW_STRINGS = {
            {"E1", "E2"},
            {"E1", "E1", "E2"}
    };
    private static final long[][] EXPECTED_TIMES = {
            {0L, 1000L},
            {1000L, 2000L}
    };
    // insert-snapshot routes each snapshot row individually: the routed
    // consumer receives three deliveries [E1]@1s, [E1]@2s, [E2]@2s.
    private static final long[][] EXPECTED_TIMES_S1 = {
            {0L, 1000L},
            {1000L, 2000L, 2000L}
    };
    private static final int[][] EXPECTED_CALLBACKS = {{2, 2}, {2, 3}};
    private static final String[] EXPECTED_FIELDS = {"intPrimitive", "theString"};
    private static final int EXPECTED_INT_PRIMITIVE = 0;
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOutputLimitInsertIntoScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitInsertIntoScenarioOracle <scenario.json>");
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
        for (int index = 0; index < CASES.length; index++) {
            runCase(index, allSteps, records);
        }
        if (records.size() != 9) {
            throw new IllegalStateException("expected nine listener records, got "
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

    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOutputLimitInsertIntoScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Module module = new Module();
            List<ModuleItem> items = new ArrayList<>();
            for (String part : EPLS[caseIndex].split(";")) {
                items.add(new ModuleItem(part.trim()));
            }
            module.setItems(items);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement s0 = findStatement(deployment, caseName, "s0");
            EPStatement s1 = findStatement(deployment, caseName, "s1");
            int expectedCallbacksS0 = EXPECTED_CALLBACKS[caseIndex][0];
            int expectedCallbacksS1 = EXPECTED_CALLBACKS[caseIndex][1];
            RecordingListener listenerS0 = new RecordingListener(records, caseIndex, s0, runtime,
                    expectedCallbacksS0);
            RecordingListener listenerS1 = new RecordingListener(records, caseIndex, s1, runtime,
                    expectedCallbacksS1);
            s0.addListener(listenerS0);
            s1.addListener(listenerS1);
            replay(allSteps, caseName, runtime);
            if (listenerS0.sequence != expectedCallbacksS0 || listenerS1.sequence != expectedCallbacksS1) {
                throw new IllegalStateException("case " + caseName + " produced listener records ("
                        + listenerS0.sequence + ", " + listenerS1.sequence + "), expected ("
                        + expectedCallbacksS0 + ", " + expectedCallbacksS1 + ")");
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName,
                                             String statementName) {
        EPStatement result = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if (!statementName.equals(candidate.getName())) {
                continue;
            }
            if (result != null) {
                throw new IllegalStateException("case " + caseName + " deployed multiple "
                        + statementName + " statements");
            }
            result = candidate;
        }
        if (result == null) {
            throw new IllegalStateException("case " + caseName + " did not deploy statement "
                    + statementName);
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
            if ("advance-time".equals(operation)) {
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
            throw new IllegalArgumentException("scenario must contain exactly two cases");
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
        if (steps.size() != 10) {
            throw new IllegalArgumentException("scenario must contain exactly ten steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        validateBeanStep(steps.get(offset++), "E1", 0);
        validateBeanStep(steps.get(offset++), "E2", 0);
        validateAdvanceTimeStep(steps.get(offset++), "1970-01-01T00:00:01Z");
        validateBeanStep(steps.get(offset++), "E2", 0);
        validateCaseMarker(steps.get(offset++), CASES[1]);
        validateBeanStep(steps.get(offset++), "E1", 0);
        validateAdvanceTimeStep(steps.get(offset++), "1970-01-01T00:00:01Z");
        validateBeanStep(steps.get(offset++), "E2", 0);
        validateAdvanceTimeStep(steps.get(offset++), "1970-01-01T00:00:02Z");
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
        private int rowCursor;

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
                        + " produced more than " + expectedCallbacks + " listener callbacks on "
                        + statement.getName());
            }
            if (newEvents == null || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " listener on "
                        + statement.getName() + " must deliver new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            long[] expectedTimes = statement.getName().equals("s1")
                    ? EXPECTED_TIMES_S1[caseIndex] : EXPECTED_TIMES[caseIndex];
            if (now != expectedTimes[next - 1]) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " listener callback "
                        + next + " on " + statement.getName() + " at unexpected time " + now);
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

        private void validateRows(EventBean[] events, int callbackIndex) {
            String[] caseStrings = EXPECTED_ROW_STRINGS[caseIndex];
            if (rowCursor + events.length > caseStrings.length) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " listener callback "
                        + callbackIndex + " on " + statement.getName() + " exceeds the pinned row sequence");
            }
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, EXPECTED_FIELDS)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex] + " row " + rowIndex);
                }
                Object intPrimitive = event.get("intPrimitive");
                if (!Integer.valueOf(EXPECTED_INT_PRIMITIVE).equals(intPrimitive)) {
                    throw new IllegalStateException("case " + CASES[caseIndex] + " row " + rowIndex
                            + " intPrimitive expected " + EXPECTED_INT_PRIMITIVE + ", got "
                            + intPrimitive);
                }
                assertString(event.get("theString"), caseStrings[rowCursor + rowIndex], "theString",
                        rowIndex, CASES[caseIndex]);
            }
            rowCursor += events.length;
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
