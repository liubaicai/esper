import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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
import java.util.Objects;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetQueryTypeRollupHavingAndOrderBy ordinals 4-7. */
public final class ResultSetQueryTypeRollupOrderByUnidirectionalScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-rollup-orderby-unidirectional";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeRollupHavingAndOrderBy ordinals 4-7: time-batch rollup ordering and unidirectional cube listener semantics.";
    private static final String[] STATIC_IDS = {
            "java-715eae421ccf37a5062a",
            "java-24e47ca92d533e352474",
            "java-ba719758f9ea057c8d79"
    };
    private static final String[] RUNTIMES = {
            "java-runtime-b09e67f84d7426e94834",
            "java-runtime-7859d904aebfc874ac11",
            "java-runtime-7f3911bcedf70526d5df",
            "java-runtime-2ff9039bfd91c9054507"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetQueryTypeOrderByTwoCriteriaAsc{join=false}",
            "ResultSetQueryTypeOrderByTwoCriteriaAsc{join=true}",
            "ResultSetQueryTypeUnidirectional",
            "ResultSetQueryTypeOrderByOneCriteriaDesc"
    };
    private static final String[] CASES = {
            "order-by-two-criteria-no-join",
            "order-by-two-criteria-join",
            "unidirectional-cube",
            "order-by-one-criteria-desc"
    };
    private static final int[] ORDINALS = {4, 5, 6, 7};
    private static final String[] EPL = {
            "@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) group by rollup(theString, intPrimitive) order by theString, intPrimitive",
            "@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) , SupportBean_S0#lastevent group by rollup(theString, intPrimitive) order by theString, intPrimitive",
            "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean_S0 unidirectional, SupportBean#keepall group by cube(theString, intPrimitive)",
            "@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) group by rollup(theString, intPrimitive) order by theString desc;"
    };

    private ResultSetQueryTypeRollupOrderByUnidirectionalScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetQueryTypeRollupOrderByUnidirectionalScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(scenario.get("steps").asArray(), records, index);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version")) || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !COMMIT.equals(string(scenario, "javaCommit"))
                || !SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPL[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 42) {
            throw new IllegalArgumentException("scenario must contain exactly 42 steps");
        }
        int[] markerIndices = {0, 11, 23, 31};
        for (int index = 0; index < markerIndices.length; index++) {
            int stepIndex = markerIndices[index];
            JsonObject step = object(steps.get(stepIndex), "case marker " + stepIndex);
            requireFields(step, "op", "case");
            if (!"case".equals(string(step, "op")) || !CASES[index].equals(string(step, "case"))) {
                throw new IllegalArgumentException("case marker is not pinned at step " + stepIndex);
            }
        }
        validateAdvance(steps, 1, "1970-01-01T00:00:00Z");
        validateAdvance(steps, 6, "1970-01-01T00:00:01Z");
        validateAdvance(steps, 10, "1970-01-01T00:00:02Z");
        validateAdvance(steps, 12, "1970-01-01T00:00:00Z");
        validateAdvance(steps, 18, "1970-01-01T00:00:01Z");
        validateAdvance(steps, 22, "1970-01-01T00:00:02Z");
        validateAdvance(steps, 32, "1970-01-01T00:00:00Z");
        validateAdvance(steps, 37, "1970-01-01T00:00:01Z");
        validateAdvance(steps, 41, "1970-01-01T00:00:02Z");

        validateBean(steps, 2, "E2", 10, 100L);
        validateBean(steps, 3, "E1", 11, 200L);
        validateBean(steps, 4, "E1", 10, 300L);
        validateBean(steps, 5, "E1", 11, 400L);
        validateBean(steps, 7, "E1", 11, 500L);
        validateBean(steps, 8, "E1", 10, 600L);
        validateBean(steps, 9, "E1", 12, 700L);

        validateS0(steps, 13, 1);
        validateBean(steps, 14, "E2", 10, 100L);
        validateBean(steps, 15, "E1", 11, 200L);
        validateBean(steps, 16, "E1", 10, 300L);
        validateBean(steps, 17, "E1", 11, 400L);
        validateBean(steps, 19, "E1", 11, 500L);
        validateBean(steps, 20, "E1", 10, 600L);
        validateBean(steps, 21, "E1", 12, 700L);

        validateBean(steps, 24, "E1", 10, 100L);
        validateBean(steps, 25, "E2", 20, 200L);
        validateBean(steps, 26, "E1", 11, 300L);
        validateBean(steps, 27, "E2", 20, 400L);
        validateS0(steps, 28, 1);
        validateBean(steps, 29, "E1", 10, 1L);
        validateS0(steps, 30, 2);

        validateBean(steps, 33, "E2", 10, 100L);
        validateBean(steps, 34, "E1", 11, 200L);
        validateBean(steps, 35, "E1", 10, 300L);
        validateBean(steps, 36, "E1", 11, 400L);
        validateBean(steps, 38, "E1", 11, 500L);
        validateBean(steps, 39, "E1", 10, 600L);
        validateBean(steps, 40, "E1", 12, 700L);
    }

    private static void validateAdvance(JsonArray steps, int index, String expected) {
        JsonObject step = object(steps.get(index), "advance-time step " + index);
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op")) || !expected.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step " + index + " is not pinned");
        }
    }

    private static void validateBean(JsonArray steps, int index, String expectedString, int expectedInt, long expectedLong) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean send step " + index + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException("SupportBean payload step " + index + " is not pinned");
        }
    }

    private static void validateS0(JsonArray steps, int index, int expectedID) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 send step " + index + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "id");
        if (integer(payload, "id") != expectedID) {
            throw new IllegalArgumentException("SupportBean_S0 payload step " + index + " is not pinned");
        }
    }

    private static void runCase(JsonArray steps, JsonArray records, int caseIndex) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL[caseIndex],
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + CASES[caseIndex]));
            EPStatement statement = findStatement(deployment, caseIndex);
            TraceWriter writer = new TraceWriter(records, statement, runtime, caseIndex);
            statement.addListener(writer);
            replay(steps, runtime, caseIndex);
            if (writer.sequence != 2) {
                throw new IllegalStateException("case " + CASES[caseIndex] + " expected two listener records, got " + writer.sequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, int caseIndex) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0 in case " + CASES[caseIndex]);
        }
        return statements[0];
    }

    private static void replay(JsonArray steps, EPRuntime runtime, int caseIndex) {
        boolean active = false;
        int markers = 0;
        for (JsonValue value : steps) {
            JsonObject step = object(value, "step");
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                if (active) {
                    break;
                }
                active = CASES[caseIndex].equals(string(step, "case"));
                if (active) {
                    markers++;
                }
                continue;
            }
            if (!active) {
                continue;
            }
            if ("send".equals(operation)) {
                send(runtime, step);
            } else if ("advance-time".equals(operation)) {
                runtime.getEventService().advanceTime(Instant.parse(string(step, "at")).toEpochMilli());
            } else {
                throw new IllegalArgumentException("unsupported operation " + operation + " in case " + CASES[caseIndex]);
            }
        }
        if (!active || markers != 1) {
            throw new IllegalStateException("case marker replay mismatch for " + CASES[caseIndex]);
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        if ("SupportBean".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean(
                    string(payload, "theString"), integer(payload, "intPrimitive")) {{
                        setLongPrimitive(longNumber(payload, "longPrimitive"));
                    }}, eventType);
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean_S0(integer(payload, "id")), eventType);
            return;
        }
        throw new IllegalArgumentException("unknown event type " + eventType);
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int caseIndex;
        private int sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime, int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseIndex = caseIndex;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > 2) {
                throw new IllegalStateException("unexpected listener callback after two records in case " + CASES[caseIndex]);
            }
            Object[][] expectedNew = expectedNew(caseIndex, next);
            Object[][] expectedOld = expectedOld(caseIndex, next);
            validateRows(newEvents, expectedNew, "new", next);
            validateRows(oldEvents, expectedOld, "old", next);
            String expectedTime = caseIndex == 2
                    ? "1970-01-01T00:00:00Z"
                    : (next == 1 ? "1970-01-01T00:00:01Z" : "1970-01-01T00:00:02Z");
            String actualTime = Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString();
            if (!expectedTime.equals(actualTime)) {
                throw new IllegalStateException("unexpected callback time in case " + CASES[caseIndex]
                        + ": expected " + expectedTime + ", got " + actualTime);
            }
            sequence = next;
            JsonObject record = new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence).add("time", actualTime);
            if (newEvents != null && newEvents.length > 0) {
                record.add("new", rows(newEvents));
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private static void validateRows(EventBean[] actual, Object[][] expected, String stream, int sequence) {
            int actualLength = actual == null ? 0 : actual.length;
            if (actualLength != expected.length) {
                throw new IllegalStateException("unexpected " + stream + " row count at sequence " + sequence
                        + ": expected " + expected.length + ", got " + actualLength);
            }
            for (int index = 0; index < expected.length; index++) {
                EventBean event = actual[index];
                String[] properties = event.getEventType().getPropertyNames().clone();
                Arrays.sort(properties);
                if (!Arrays.equals(properties, new String[]{"c0", "c1", "c2"})) {
                    throw new IllegalStateException("unexpected result schema at " + stream + " row " + index);
                }
                if (!valueEquals(event.get("c0"), expected[index][0])
                        || !valueEquals(event.get("c1"), expected[index][1])
                        || !valueEquals(event.get("c2"), expected[index][2])) {
                    throw new IllegalStateException("unexpected " + stream + " row " + index
                            + " at sequence " + sequence);
                }
            }
        }
    }

    private static Object[][] expectedNew(int caseIndex, int sequence) {
        if (caseIndex == 2) {
            return sequence == 1 ? new Object[][]{
                    {"E1", 10, 100L}, {"E2", 20, 600L}, {"E1", 11, 300L},
                    {"E1", null, 400L}, {"E2", null, 600L}, {null, 10, 100L},
                    {null, 20, 600L}, {null, 11, 300L}, {null, null, 1000L}
            } : new Object[][]{
                    {"E1", 10, 101L}, {"E2", 20, 600L}, {"E1", 11, 300L},
                    {"E1", null, 401L}, {"E2", null, 600L}, {null, 10, 101L},
                    {null, 20, 600L}, {null, 11, 300L}, {null, null, 1001L}
            };
        }
        if (caseIndex == 3) {
            return sequence == 1 ? new Object[][]{
                    {"E2", 10, 100L}, {"E2", null, 100L}, {"E1", 11, 600L},
                    {"E1", 10, 300L}, {"E1", null, 900L}, {null, null, 1000L}
            } : new Object[][]{
                    {"E2", 10, null}, {"E2", null, null}, {"E1", 11, 500L},
                    {"E1", 10, 600L}, {"E1", 12, 700L}, {"E1", null, 1800L},
                    {null, null, 1800L}
            };
        }
        return sequence == 1 ? new Object[][]{
                {null, null, 1000L}, {"E1", null, 900L}, {"E1", 10, 300L},
                {"E1", 11, 600L}, {"E2", null, 100L}, {"E2", 10, 100L}
        } : new Object[][]{
                {null, null, 1800L}, {"E1", null, 1800L}, {"E1", 10, 600L},
                {"E1", 11, 500L}, {"E1", 12, 700L}, {"E2", null, null},
                {"E2", 10, null}
        };
    }

    private static Object[][] expectedOld(int caseIndex, int sequence) {
        if (caseIndex == 2) {
            return new Object[0][];
        }
        if (caseIndex == 3) {
            return sequence == 1 ? new Object[][]{
                    {"E2", 10, null}, {"E2", null, null}, {"E1", 11, null},
                    {"E1", 10, null}, {"E1", null, null}, {null, null, null}
            } : new Object[][]{
                    {"E2", 10, 100L}, {"E2", null, 100L}, {"E1", 11, 600L},
                    {"E1", 10, 300L}, {"E1", 12, null}, {"E1", null, 900L},
                    {null, null, 1000L}
            };
        }
        return sequence == 1 ? new Object[][]{
                {null, null, null}, {"E1", null, null}, {"E1", 10, null},
                {"E1", 11, null}, {"E2", null, null}, {"E2", 10, null}
        } : new Object[][]{
                {null, null, 1000L}, {"E1", null, 900L}, {"E1", 10, 300L},
                {"E1", 11, 600L}, {"E1", 12, null}, {"E2", null, 100L},
                {"E2", 10, 100L}
        };
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        for (EventBean event : events) {
            String[] properties = event.getEventType().getPropertyNames().clone();
            Arrays.sort(properties);
            JsonObject fields = new JsonObject();
            for (String property : properties) {
                fields.add(property, normalize(event.get(property)));
            }
            rows.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return rows;
    }

    private static boolean valueEquals(Object actual, Object expected) {
        if (actual == null || expected == null) {
            return actual == expected;
        }
        if (actual instanceof Number && expected instanceof Number) {
            return ((Number) actual).doubleValue() == ((Number) expected).doubleValue();
        }
        return Objects.equals(actual, expected);
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
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
}
