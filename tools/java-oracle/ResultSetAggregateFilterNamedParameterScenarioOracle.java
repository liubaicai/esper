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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregateFilterNamedParameter
 * ordinals 4-7. Each execution is deployed and replayed in a fresh runtime.
 */
public final class ResultSetAggregateFilterNamedParameterScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-filter-named-parameter";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java";
    private static final String DESCRIPTION =
            "Named filter aggregate methods replaying ResultSetAggregateFilterNamedParameter executions: leaving, nth, virtual-time rate and timestamp rate.";
    private static final String SHARED_STATIC_ID = "java-0c29efb6d43971aba5c4";

    private static final String LEAVING = "leaving";
    private static final String NTH = "nth";
    private static final String RATE_UNBOUND = "rate-unbound";
    private static final String RATE_BOUND = "rate-bound";
    private static final String[] CASES = {LEAVING, NTH, RATE_UNBOUND, RATE_BOUND};
    private static final int[] ORDINALS = {4, 5, 6, 7};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7bc068fcf2ea07b9c1f7",
            "java-runtime-dd319218418ee0418b21",
            "java-runtime-4b24ef27ade0eae24258",
            "java-runtime-3d732054eac8d5b8ba14"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateMethodAggLeaving",
            "ResultSetAggregateMethodAggNth",
            "ResultSetAggregateMethodAggRateUnbound",
            "ResultSetAggregateMethodAggRateBound"
    };
    private static final String[] EPLS = {
            "@name('s0') select leaving(filter:intPrimitive=1) as c0,leaving(filter:intPrimitive=2) as c1 from SupportBean#length(2)",
            "@name('s0') select nth(intPrimitive, 1, filter:theString like 'A%') as c0 from SupportBean",
            "@name('s0') select rate(1, filter:theString like 'A%') as c0 from SupportBean",
            "@name('s0') select rate(longPrimitive, filter:theString like 'A%') as myrate, rate(longPrimitive, intPrimitive, filter:theString like 'A%') as myqtyrate from SupportBean#length(3)"
    };
    private static final int[] SEND_COUNTS = {4, 7, 5, 8};
    private static final int[] RECORD_COUNTS = {4, 7, 5, 8};

    private static final String[] LEAVING_STRINGS = {"E1", "E2", "E3", "E4"};
    private static final int[] LEAVING_INTS = {2, 1, 3, 4};
    private static final String[] NTH_STRINGS = {"X1", "X2", "A3", "A4", "X3", "A5", "X4"};
    private static final int[] NTH_INTS = {0, 0, 1, 2, 0, 3, 0};
    private static final String[] RATE_UNBOUND_STRINGS = {"X1", "A1", "X2", "A2", "A3"};
    private static final String[] RATE_BOUND_STRINGS = {"X1", "X2", "X2", "A1", "A2", "A3", "A4", "A5"};
    private static final long[] RATE_BOUND_LONGS = {1000L, 1200L, 1300L, 1000L, 1200L, 1300L, 1500L, 2000L};
    private static final int[] RATE_BOUND_INTS = {10, 0, 0, 10, 0, 0, 14, 11};

    private ResultSetAggregateFilterNamedParameterScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateFilterNamedParameterScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }
        if (records.size() != 24) {
            throw new IllegalStateException("expected 24 listener records, got " + records.size());
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", ID)
                .add("javaCommit", JAVA_COMMIT)
                .add("java", System.getProperty("java.version"))
                .add("records", records);
        System.out.println(trace);
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
        validateStringArray(scenario.get("javaStaticIds"), new String[]{SHARED_STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray definitions = array(scenario.get("cases"), "cases");
        if (definitions.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(definitions.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("scenario case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 29) {
            throw new IllegalArgumentException("scenario must contain exactly 29 steps");
        }
        int index = 0;
        validateCaseMarker(steps, index++, LEAVING);
        for (int event = 0; event < SEND_COUNTS[0]; event++) {
            validateBeanSend(steps, index++, LEAVING, event);
        }
        validateCaseMarker(steps, index++, NTH);
        for (int event = 0; event < SEND_COUNTS[1]; event++) {
            validateBeanSend(steps, index++, NTH, event);
        }
        validateCaseMarker(steps, index++, RATE_UNBOUND);
        validateUnboundSend(steps, index++, 0);
        validateUnboundSend(steps, index++, 1);
        validateAdvanceTime(steps, index++, "1970-01-01T00:00:01Z");
        validateUnboundSend(steps, index++, 2);
        validateUnboundSend(steps, index++, 3);
        validateUnboundSend(steps, index++, 4);
        validateCaseMarker(steps, index++, RATE_BOUND);
        for (int event = 0; event < SEND_COUNTS[3]; event++) {
            validateBoundSend(steps, index++, event);
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static void validateCaseMarker(JsonArray steps, int index, String expectedCase) {
        JsonObject marker = object(steps.get(index), "case marker " + index);
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker mismatch at step " + index);
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, String caseName, int eventIndex) {
        JsonObject step = sendStep(steps, index);
        JsonObject payload = object(step.get("payload"), "payload " + index);
        String[] expectedStrings;
        int[] expectedInts;
        if (LEAVING.equals(caseName)) {
            expectedStrings = LEAVING_STRINGS;
            expectedInts = LEAVING_INTS;
        } else if (NTH.equals(caseName)) {
            expectedStrings = NTH_STRINGS;
            expectedInts = NTH_INTS;
        } else {
            throw new IllegalArgumentException("unsupported bean case " + caseName);
        }
        requireFields(payload, "theString", "intPrimitive");
        if (eventIndex < 0 || eventIndex >= expectedStrings.length
                || !expectedStrings[eventIndex].equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInts[eventIndex]) {
            throw new IllegalArgumentException("SupportBean payload mismatch at step " + index);
        }
    }

    private static void validateUnboundSend(JsonArray steps, int index, int eventIndex) {
        JsonObject step = sendStep(steps, index);
        JsonObject payload = object(step.get("payload"), "payload " + index);
        requireFields(payload, "theString");
        if (eventIndex < 0 || eventIndex >= RATE_UNBOUND_STRINGS.length
                || !RATE_UNBOUND_STRINGS[eventIndex].equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("rate-unbound payload mismatch at step " + index);
        }
    }

    private static void validateBoundSend(JsonArray steps, int index, int eventIndex) {
        JsonObject step = sendStep(steps, index);
        JsonObject payload = object(step.get("payload"), "payload " + index);
        requireFields(payload, "theString", "longPrimitive", "intPrimitive");
        if (eventIndex < 0 || eventIndex >= RATE_BOUND_STRINGS.length
                || !RATE_BOUND_STRINGS[eventIndex].equals(string(payload, "theString"))
                || longNumber(payload, "longPrimitive") != RATE_BOUND_LONGS[eventIndex]
                || integer(payload, "intPrimitive") != RATE_BOUND_INTS[eventIndex]) {
            throw new IllegalArgumentException("rate-bound payload mismatch at step " + index);
        }
    }

    private static JsonObject sendStep(JsonArray steps, int index) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBean");
        }
        return step;
    }

    private static void validateAdvanceTime(JsonArray steps, int index, String expected) {
        JsonObject step = object(steps.get(index), "advance-time step " + index);
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op")) || !expected.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step " + index + " is not pinned");
        }
    }

    private static void runCase(JsonArray allSteps, int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        if (caseIndex == 0 || caseIndex == 1 || caseIndex == 3) {
            beanType.put("intPrimitive", Integer.class);
        }
        if (caseIndex == 3) {
            beanType.put("longPrimitive", Long.class);
        }
        configuration.getCommon().addEventType("SupportBean", beanType);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    EPLS[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment, caseName);
            TraceWriter writer = new TraceWriter(records, caseIndex, statement, runtime);
            statement.addListener(writer);
            replayCase(allSteps, caseIndex, runtime);
            if (writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.sequence
                        + " listener records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null) {
            throw new IllegalStateException("case " + caseName + " deployed no statements");
        }
        for (EPStatement statement : statements) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
    }

    private static void replayCase(JsonArray steps, int caseIndex, EPRuntime runtime) {
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
            if ("send".equals(operation)) {
                send(runtime, step, caseIndex);
                sends++;
            } else if ("advance-time".equals(operation)) {
                runtime.getEventService().advanceTime(Instant.parse(string(step, "at")).toEpochMilli());
            } else {
                throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
            }
        }
        if (sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalStateException("case " + caseName + " replayed " + sends
                    + " sends, expected " + SEND_COUNTS[caseIndex]);
        }
    }

    private static void send(EPRuntime runtime, JsonObject step, int caseIndex) {
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unsupported event type: " + string(step, "eventType"));
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        Map<String, Object> event = new HashMap<>();
        event.put("theString", string(payload, "theString"));
        if (caseIndex == 0 || caseIndex == 1 || caseIndex == 3) {
            event.put("intPrimitive", integer(payload, "intPrimitive"));
        }
        if (caseIndex == 3) {
            event.put("longPrimitive", longNumber(payload, "longPrimitive"));
        }
        runtime.getEventService().sendEventMap(event, "SupportBean");
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final int caseIndex;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, int caseIndex, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.caseName = CASES[caseIndex];
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + caseName
                        + " listener callback must contain one new-only row");
            }
            if (sequence >= RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced too many callbacks");
            }
            long nextSequence = sequence + 1;
            validateRow(newEvents[0], nextSequence);
            long now = runtime.getEventService().getCurrentTime();
            long expectedTime = caseIndex == 2 && nextSequence >= 3 ? 1000L : 0L;
            if (now != expectedTime) {
                throw new IllegalStateException("case " + caseName + " callback time at sequence "
                        + nextSequence + " was " + now + ", expected " + expectedTime);
            }
            sequence = nextSequence;
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", rows(newEvents));
            records.add(record);
        }

        private void validateRow(EventBean event, long nextSequence) {
            String[] expectedFields;
            if (caseIndex == 0) {
                expectedFields = new String[]{"c0", "c1"};
            } else if (caseIndex == 1 || caseIndex == 2) {
                expectedFields = new String[]{"c0"};
            } else {
                expectedFields = new String[]{"myqtyrate", "myrate"};
            }
            String[] actualFields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(actualFields);
            if (!Arrays.equals(actualFields, expectedFields)) {
                throw new IllegalStateException("case " + caseName + " field metadata mismatch: "
                        + Arrays.toString(actualFields));
            }

            if (caseIndex == 0) {
                boolean[][] expected = {{false, false}, {false, false}, {false, true}, {true, true}};
                if (!same(event.get("c0"), expected[(int) nextSequence - 1][0])
                        || !same(event.get("c1"), expected[(int) nextSequence - 1][1])) {
                    throw new IllegalStateException("case leaving values mismatch at sequence " + nextSequence);
                }
            } else if (caseIndex == 1) {
                Integer[] expected = {null, null, null, 1, 1, 2, 2};
                if (!same(event.get("c0"), expected[(int) nextSequence - 1])) {
                    throw new IllegalStateException("case nth value mismatch at sequence " + nextSequence);
                }
            } else if (caseIndex == 2) {
                Double[] expected = {null, null, null, 1.0, 2.0};
                if (!same(event.get("c0"), expected[(int) nextSequence - 1])) {
                    throw new IllegalStateException("case rate-unbound value mismatch at sequence " + nextSequence);
                }
            } else {
                Double[] expectedRate = {null, null, null, null, null, null, 6.0, 3.75};
                Double[] expectedQuantityRate = {null, null, null, null, null, null, 28.0, 31.25};
                if (!same(event.get("myrate"), expectedRate[(int) nextSequence - 1])
                        || !same(event.get("myqtyrate"), expectedQuantityRate[(int) nextSequence - 1])) {
                    throw new IllegalStateException("case rate-bound values mismatch at sequence " + nextSequence);
                }
            }
        }

        private boolean same(Object actual, Object expected) {
            if (expected == null) {
                return actual == null;
            }
            if (actual instanceof Number && expected instanceof Number) {
                return Double.compare(((Number) actual).doubleValue(), ((Number) expected).doubleValue()) == 0;
            }
            return expected.equals(actual);
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
            double number = value.asDouble();
            long integral = value.asLong();
            if (!Double.isFinite(number) || number != integral) {
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            }
            return integral;
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
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
}
