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
import com.espertech.esper.common.client.variable.VariableNotFoundException;
import com.espertech.esper.common.client.variable.VariableValueException;
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
 * Direct Esper 9.0.0 oracle for the ResultSetOutputLimitAfter event-count
 * after-gate executions.  Part one replays ResultSetDirectNumberOfEvents: the
 * keepall window with "output after 3 events" arms on E1 through E3 without
 * any listener callback, then E4 and E5 each deliver exactly one new-only row
 * carrying theString.  Part two replays ResultSetOutputWhenThen: three boolean
 * variables and the s0 statement deploy as one module, E1 through E3 stay
 * silent while myvar0 is false, setting myvar0 through the variable service
 * arms the gate, E4 delivers exactly one new-only row projecting the a.*
 * underlying event, and the when-then clause must have flipped myvar1 and
 * myvar2 to true; the two read-variable steps then record both values.
 */
public final class ResultSetOutputLimitAfterEventsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "output-after-events";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitAfter event-count after-gate: direct after-3-events delivery and the "
                    + "when-then variable side-effect form.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/"
                    + "ResultSetOutputLimitAfter.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-34bfe3c4c56f505d50cd",
            "java-runtime-499038c2c0fca09551c1"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetDirectNumberOfEvents",
            "ResultSetOutputWhenThen"
    };
    private static final String[] STATIC_IDS = {
            "java-aacc84310d1fab722e9f",
            "java-310737339ed16858a1fc"
    };
    private static final String[] CASES = {
            "after-3-events",
            "after-3-events-when-then"
    };
    private static final int[] ORDINALS = {3, 6};
    private static final String[] SCENARIO_EPLS = {
            "@name('s0') select theString from SupportBean#keepall output after 3 events",
            "@Name('s0') select a.* from SupportBean#time(10) a output after 3 events when myvar0=true "
                    + "then set myvar1=true, myvar2=true"
    };
    private static final String[] DEPLOY_MODULES = {
            SCENARIO_EPLS[0],
            "create variable boolean myvar0 = false;\n"
                    + "create variable boolean myvar1 = false;\n"
                    + "create variable boolean myvar2 = false;\n"
                    + "@Name('s0')\n"
                    + "select a.* from SupportBean#time(10) a output after 3 events when myvar0=true "
                    + "then set myvar1=true, myvar2=true"
    };

    private static final String[][] CASE_SEND_STRINGS = {
            {"E1", "E2", "E3", "E4", "E5"},
            {"E1", "E2", "E3", "E4"}
    };
    private static final String[][] EXPECTED_FIELDS = {
            {"theString"},
            {"intPrimitive", "theString"}
    };
    private static final Object[][][] EXPECTED_NEW_ROWS = {
            {{"E4"}, {"E5"}},
            {{0, "E4"}}
    };
    private static final int[] EXPECTED_CALLBACKS = {2, 1};
    private static final long[] EXPECTED_TIMES = {0L, 0L};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOutputLimitAfterEventsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitAfterEventsScenarioOracle <scenario.json>");
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
        if (records.size() != 5) {
            throw new IllegalStateException("expected five records, got " + records.size());
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
                "ResultSetOutputLimitAfterEventsScenarioOracle-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    DEPLOY_MODULES[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseName));
            EPStatement statement = findStatement(deployment, caseName);
            int expectedCallbacks = expectedListenerCallbacks(caseIndex);
            RecordingListener listener = new RecordingListener(records, caseIndex, statement, runtime,
                    expectedCallbacks);
            statement.addListener(listener);
            String deploymentId = deployment.getDeploymentId();
            replay(allSteps, caseName, deploymentId, runtime, records);
            if (caseIndex == 1) {
                assertVariableTrue(caseName, deploymentId, runtime, "myvar1");
                assertVariableTrue(caseName, deploymentId, runtime, "myvar2");
            }
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

    private static void replay(JsonArray allSteps, String caseName, String deploymentId,
                               EPRuntime runtime, JsonArray records) throws Exception {
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
            if ("send".equals(operation)) {
                sendEvent(runtime, step, caseName);
                continue;
            }
            if ("set-variable".equals(operation)) {
                setVariable(step, caseName, deploymentId, runtime);
                continue;
            }
            if ("read-variable".equals(operation)) {
                readVariable(step, caseName, deploymentId, runtime, records);
                continue;
            }
            throw new IllegalArgumentException("unsupported operation " + operation
                    + " in case " + caseName);
        }
    }

    private static void setVariable(JsonObject step, String caseName, String deploymentId,
                                    EPRuntime runtime)
            throws VariableValueException, VariableNotFoundException {
        requireFields(step, "op", "name", "payload");
        JsonValue payload = step.get("payload");
        if (!"myvar0".equals(string(step, "name")) || payload == null || !payload.isTrue()) {
            throw new IllegalArgumentException("set-variable step in case " + caseName
                    + " must set myvar0 to JSON true");
        }
        runtime.getVariableService().setVariableValue(deploymentId, "myvar0", Boolean.TRUE);
    }

    private static void readVariable(JsonObject step, String caseName, String deploymentId,
                                     EPRuntime runtime, JsonArray records)
            throws VariableNotFoundException {
        requireFields(step, "op", "name");
        String name = string(step, "name");
        if (!"myvar1".equals(name) && !"myvar2".equals(name)) {
            throw new IllegalArgumentException("read-variable step in case " + caseName
                    + " must read myvar1 or myvar2");
        }
        Object value = runtime.getVariableService().getVariableValue(deploymentId, name);
        if (!Boolean.TRUE.equals(value)) {
            throw new IllegalStateException("case " + caseName + " read variable " + name
                    + " expected true, got " + value);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "variable");
        record.add("name", name);
        record.add("value", Json.TRUE);
        records.add(record);
    }

    private static void assertVariableTrue(String caseName, String deploymentId, EPRuntime runtime,
                                           String name) throws VariableNotFoundException {
        Object value = runtime.getVariableService().getVariableValue(deploymentId, name);
        if (!Boolean.TRUE.equals(value)) {
            throw new IllegalStateException("case " + caseName + " variable " + name
                    + " expected true, got " + value);
        }
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
                    || !SCENARIO_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 14) {
            throw new IllegalArgumentException("scenario must contain exactly fourteen steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        for (int eventIndex = 0; eventIndex < CASE_SEND_STRINGS[0].length; eventIndex++) {
            validateBeanStep(steps.get(offset++), CASE_SEND_STRINGS[0][eventIndex]);
        }
        validateCaseMarker(steps.get(offset++), CASES[1]);
        for (int eventIndex = 0; eventIndex < CASE_SEND_STRINGS[1].length - 1; eventIndex++) {
            validateBeanStep(steps.get(offset++), CASE_SEND_STRINGS[1][eventIndex]);
        }
        validateSetVariableStep(steps.get(offset++));
        validateBeanStep(steps.get(offset++), CASE_SEND_STRINGS[1][CASE_SEND_STRINGS[1].length - 1]);
        validateReadVariableStep(steps.get(offset++), "myvar1");
        validateReadVariableStep(steps.get(offset++), "myvar2");
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

    private static void validateBeanStep(JsonValue value, String expectedString) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload, "intPrimitive") != 0) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateSetVariableStep(JsonValue value) {
        JsonObject step = object(value, "set-variable step");
        requireFields(step, "op", "name", "payload");
        JsonValue payload = step.get("payload");
        if (!"set-variable".equals(string(step, "op"))
                || !"myvar0".equals(string(step, "name"))
                || payload == null || !payload.isTrue()) {
            throw new IllegalArgumentException("set-variable step is not pinned");
        }
    }

    private static void validateReadVariableStep(JsonValue value, String expectedName) {
        JsonObject step = object(value, "read-variable step");
        requireFields(step, "op", "name");
        if (!"read-variable".equals(string(step, "op"))
                || !expectedName.equals(string(step, "name"))) {
            throw new IllegalArgumentException("read-variable step is not pinned");
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

    private static void assertValue(Object actual, Object expected, String field, int rowIndex,
                                    String caseName) {
        boolean equal;
        if (expected instanceof Number) {
            equal = actual instanceof Number
                    && ((Number) actual).longValue() == ((Number) expected).longValue();
        } else {
            equal = expected.equals(actual);
        }
        if (!equal) {
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
            int expectedRows = 1;
            if (newEvents == null || newEvents.length != expectedRows
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback must contain " + expectedRows + " new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != EXPECTED_TIMES[caseIndex]) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            validateRows(newEvents, next - 1);

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
            String[] expectedFields = EXPECTED_FIELDS[caseIndex];
            Object[] expectedValues = EXPECTED_NEW_ROWS[caseIndex][callbackIndex];
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, expectedFields)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                for (int fieldIndex = 0; fieldIndex < expectedFields.length; fieldIndex++) {
                    assertValue(event.get(expectedFields[fieldIndex]), expectedValues[fieldIndex],
                            expectedFields[fieldIndex], rowIndex, CASES[caseIndex]);
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
