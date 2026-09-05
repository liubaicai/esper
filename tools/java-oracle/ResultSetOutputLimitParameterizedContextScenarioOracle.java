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
 * Direct Esper 9.0.0 oracle for the ResultSetOutputLimitParameterizedByContext
 * execution: a context started by SupportScheduleSimpleEvent parameterizes the
 * output-last crontab from the start event (atminute=15, athour=10).  One S0
 * event joins the partition and exactly one new-only single-row count callback
 * fires at the scheduled 10:15 instant; the 10:14:59 probe stays silent.  The
 * when-terminated output on context-partition teardown is outside the scripted
 * observation window, so the listener is detached before undeploy.
 */
public final class ResultSetOutputLimitParameterizedContextScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-parameterized-context";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitParameterizedByContext: a context started by "
                    + "SupportScheduleSimpleEvent parameterizes the output-last crontab from the start "
                    + "event; one S0 event joins the partition and exactly one new-only single-row "
                    + "count callback fires at the scheduled 10:15 instant.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitParameterizedByContext.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-3fc747eca88c34b6ab3a"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOutputLimitParameterizedByContext"
    };
    private static final String[] STATIC_IDS = {
            "java-3bf9eee607100a7666ab"
    };
    private static final String[] CASES = {
            "context-cron"
    };
    private static final int[] ORDINALS = {0};
    private static final String EPL =
            "@name('ctx') create context MyCtx start SupportScheduleSimpleEvent as sse;\n"
                    + "@name('s0') context MyCtx\n"
                    + "select count(*) as c \n"
                    + "from SupportBean_S0\n"
                    + "output last at(context.sse.atminute, context.sse.athour, *, *, *, *) and when terminated\n";

    private static final long START_TIME = Instant.parse("2002-05-01T09:00:00.000Z").toEpochMilli();
    private static final String START_INSTANT = "2002-05-01T09:00:00.000Z";
    private static final int EXPECTED_SSE_ATHOUR = 10;
    private static final int EXPECTED_SSE_ATMINUTE = 15;
    private static final int EXPECTED_S0_ID = 0;
    private static final String[] EXPECTED_FIELDS = {"c"};
    private static final long EXPECTED_COUNT = 1L;
    private static final int EXPECTED_ROWS = 1;
    private static final int EXPECTED_CALLBACKS = 1;
    private static final long EXPECTED_TIME = Instant.parse("2002-05-01T10:15:00.000Z").toEpochMilli();
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOutputLimitParameterizedContextScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitParameterizedContextScenarioOracle <scenario.json>");
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
        if (records.size() != 1) {
            throw new IllegalStateException("expected one listener record, got "
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
        configuration.getCommon().addEventType("SupportScheduleSimpleEvent", SupportScheduleSimpleEvent.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOutputLimitParameterizedContextScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(START_TIME);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    EPL, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            int expectedCallbacks = EXPECTED_CALLBACKS;
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime,
                    expectedCallbacks);
            statement.addListener(listener);
            replay(allSteps, caseName, runtime, records);
            if (listener.sequence != expectedCallbacks) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + listener.sequence + " listener records, expected " + expectedCallbacks);
            }
            // The context partition terminates at undeploy and the when-terminated
            // output is outside the scripted observation window; detach first.
            statement.removeListener(listener);
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

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime,
                               JsonArray records) {
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
        if ("SupportScheduleSimpleEvent".equals(eventType)) {
            JsonObject payload = object(step.get("payload"), "SupportScheduleSimpleEvent payload");
            requireFields(payload, "athour", "atminute");
            int athour = integer(payload, "athour");
            int atminute = integer(payload, "atminute");
            runtime.getEventService().sendEventBean(new SupportScheduleSimpleEvent(athour, atminute),
                    "SupportScheduleSimpleEvent");
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
            requireFields(payload, "id");
            int id = integer(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S0(id), "SupportBean_S0");
            return;
        }
        throw new IllegalArgumentException("unknown event type " + eventType
                + " in case " + caseName);
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
            throw new IllegalArgumentException("scenario must contain exactly one case");
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
                    || !EPL.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 6) {
            throw new IllegalArgumentException("scenario must contain exactly six steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        validateAdvanceTimeStep(steps.get(offset++), START_INSTANT);
        validateScheduleSimpleStep(steps.get(offset++), EXPECTED_SSE_ATHOUR, EXPECTED_SSE_ATMINUTE);
        validateS0Step(steps.get(offset++), EXPECTED_S0_ID);
        validateAdvanceTimeStep(steps.get(offset++), "2002-05-01T10:14:59.000Z");
        validateAdvanceTimeStep(steps.get(offset++), "2002-05-01T10:15:00.000Z");
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

    private static void validateScheduleSimpleStep(JsonValue value, int expectedAthour,
                                                   int expectedAtminute) {
        JsonObject step = object(value, "SupportScheduleSimpleEvent step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportScheduleSimpleEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportScheduleSimpleEvent step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportScheduleSimpleEvent payload");
        requireFields(payload, "athour", "atminute");
        if (longInteger(payload, "athour") != expectedAthour
                || longInteger(payload, "atminute") != expectedAtminute) {
            throw new IllegalArgumentException("SupportScheduleSimpleEvent payload is not pinned");
        }
    }

    private static void validateS0Step(JsonValue value, int expectedId) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (longInteger(payload, "id") != expectedId) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned");
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
            if (newEvents == null || newEvents.length != EXPECTED_ROWS
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + EXPECTED_ROWS + " new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != EXPECTED_TIME) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            validateRow(newEvents[0]);

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

        private void validateRow(EventBean event) {
            String[] fields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(fields);
            if (!Arrays.equals(fields, EXPECTED_FIELDS)) {
                throw new IllegalStateException("result field metadata is not pinned for case "
                        + CASES[caseIndex]);
            }
            Object count = event.get("c");
            if (!(count instanceof Long) || (Long) count != EXPECTED_COUNT) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " c expected "
                        + EXPECTED_COUNT + ", got " + count);
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

    /** Local mirror of the pinned SupportScheduleSimpleEvent regression bean. */
    public static final class SupportScheduleSimpleEvent {
        private int athour;
        private int atminute;

        public SupportScheduleSimpleEvent(int athour, int atminute) {
            this.athour = athour;
            this.atminute = atminute;
        }

        public int getAthour() {
            return athour;
        }

        public int getAtminute() {
            return atminute;
        }
    }

    /** Local mirror of the pinned SupportBean_S0 engine support bean. */
    public static final class SupportBean_S0 {
        private int id;
        private String p00;
        private String p01;
        private String p02;
        private String p03;

        public SupportBean_S0(int id) {
            this.id = id;
        }

        public int getId() {
            return id;
        }

        public void setId(int id) {
            this.id = id;
        }

        public String getP00() {
            return p00;
        }

        public void setP00(String p00) {
            this.p00 = p00;
        }

        public String getP01() {
            return p01;
        }

        public void setP01(String p01) {
            this.p01 = p01;
        }

        public String getP02() {
            return p02;
        }

        public void setP02(String p02) {
            this.p02 = p02;
        }

        public String getP03() {
            return p03;
        }

        public void setP03(String p03) {
            this.p03 = p03;
        }
    }
}
