import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
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

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for all six RowRecogAfter executions (the
 * AFTER MATCH SKIP family) replayed as one differential chain:
 *
 * after-current-row (ord 0, RowRecogAfterCurrentRow): skip-to-current-row
 * over pattern (A B*) with tag-indexed measures B[0]/B[1]; the B1 send
 * extends the same match. The Java execution runs the assertion twice
 * (compileDeploy then eplToModelCompileDeploy); the scenario mirrors that
 * as undeploy-all plus a second verbatim deploy cycle.
 *
 * after-next-row (ord 1, RowRecogAfterNextRow): the same measures and
 * pattern under AFTER MATCH SKIP TO NEXT ROW. The B1 send produces no
 * listener callback (incremental mode suppresses it) while the statement
 * iterator still reports the extended match, so the scenario places the
 * snapshot after the listener-suppressed send.
 *
 * skip-to-next-row (ord 2, RowRecogSkipToNextRow): all-matches (A B) with
 * B.value > A.value and order-by; consecutive pairs E2-E3, E4-E5 and the
 * overlapping E5-E6 all match.
 *
 * variable-more-then-once (ord 3, RowRecogVariableMoreThenOnce): variable
 * A appears twice in pattern ( A B A ) with tag-indexed measures
 * A[0]/A[1]; matches E5-E6-E7 and E7-E8-E9.
 *
 * skip-to-next-row-partitioned (ord 4,
 * RowRecogSkipToNextRowPartitioned): the same all-matches (A B) shape
 * partitioned by theString across a 22-event S1..S4 interleave.
 *
 * skip-past-last (ord 5, RowRecogAfterSkipPastLast): same sends as
 * skip-to-next-row but skip-past-last forbids the overlapping E5-E6
 * match, so E6 produces no listener callback.
 *
 * Mirroring SupportEvalRunner, each case deploys "@name('s0') <case EPL>"
 * once per cycle, sends every assertion event, snapshots the iterator
 * where Java asserts it, then undeploys. The pinned case EPL is the
 * contract text verbatim including its irregular whitespace and ord0's
 * double-quoted like literals. SupportRecogBean is declared as a map
 * event type (theString string, value int) because the regression-lib
 * jar is not on the oracle classpath. The TraceWriter skips null/null
 * listener callbacks.
 */
public final class RowRecogAfterScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-after";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogAfter.java";

    private static final String DESCRIPTION =
            "RowRecogAfter ordinals 0-5 (all six executions, the AFTER MATCH SKIP family): after-current-row runs skip-to-current-row twice, mirroring the Java compileDeploy then eplToModelCompileDeploy cycle as undeploy-all plus redeploy; after-next-row shows incremental-mode listener suppression on the B1 send while the iterator still reports the extended match; skip-to-next-row, variable-more-then-once and skip-to-next-row-partitioned cover all-matches tag sequences including a 22-event four-partition interleave; skip-past-last contrasts skip-to-next-row by forbidding the overlapping E5-E6 match. Each case deploys s0 with the verbatim Java EPL, sends SupportRecogBean events and snapshots the statement iterator.";

    private static final String[] CASES = {
            "after-current-row", "after-next-row", "skip-to-next-row",
            "variable-more-then-once", "skip-to-next-row-partitioned", "skip-past-last"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-040bbf4e7c7c45f3cd76",
            "java-runtime-ad336d2b8e1195e4abf6",
            "java-runtime-ebf5f4119f980f7d9258",
            "java-runtime-3ff12edd4f2bb4ecec9b",
            "java-runtime-21259f97372ba54fe074",
            "java-runtime-a960711a30045575a880"
    };
    private static final String[] EXECUTIONS = {
            "RowRecogAfterCurrentRow",
            "RowRecogAfterNextRow",
            "RowRecogSkipToNextRow",
            "RowRecogVariableMoreThenOnce",
            "RowRecogSkipToNextRowPartitioned",
            "RowRecogAfterSkipPastLast"
    };
    private static final String[] STATIC_IDS = {
            "java-41b5fd1eb615163a8c32"
    };
    private static final String[] OBSERVATIONS = {
            "listener+iterator; runs two deploy cycles mirroring the Java compileDeploy then eplToModelCompileDeploy replay",
            "listener+iterator; the B1 send produces no listener callback (incremental skip-to-next-row) while the iterator still reports the extended match",
            "listener+iterator",
            "listener+iterator",
            "listener+iterator",
            "listener+iterator; E6 produces no match since skip-past-last forbids the overlapping E5-E6 match (contrast skip-to-next-row)"
    };
    private static final String[] CASE_EPLS = {
            "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1 after match skip to current row pattern (A B*) define A as A.theString like \"A%\", B as B.theString like \"B%\")",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a, B[0].theString as b0, B[1].theString as b1  AFTER MATCH SKIP TO NEXT ROW   pattern (A B*)   define     A as A.theString like 'A%',    B as B.theString like 'B%')",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   all matches   after match skip to next row   pattern (A B)   define B as B.value > A.value) order by a_string, b_string",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, B.theString as b, A[1].theString as a1   all matches   after match skip to next row   pattern ( A B A )   define     A as (A.value = 1),    B as (B.value = 2))",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by theString  measures A.theString as a_string, A.value as a_value, B.value as b_value   all matches   after match skip to next row   pattern (A B)   define B as (B.value > A.value)) order by a_string",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   all matches   after match skip past last row  pattern (A B)   define B as B.value > A.value) order by a_string, b_string"
    };

    // Pinned op sequences per case (after the case marker). ord0 replays
    // deploy -> sends -> undeploy-all -> deploy -> sends -> undeploy-all.
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "snapshot", "send", "snapshot", "undeploy-all",
                    "deploy", "send", "snapshot", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "snapshot", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "send", "snapshot", "send", "snapshot", "send", "send", "snapshot",
                    "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "snapshot",
                    "send", "snapshot", "send", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "send", "send",
                    "snapshot", "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "send", "send", "send", "send", "snapshot", "send", "snapshot",
                    "send", "snapshot", "send", "send", "send", "send", "send",
                    "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "send", "snapshot", "send", "snapshot", "send", "send", "snapshot",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded "theString:value".
    private static final String[][] CASE_SENDS = {
            {"A1:1", "B1:2", "A1:1", "B1:2"},
            {"A1:1", "B1:2"},
            {"E1:5", "E2:3", "E3:6", "E4:4", "E5:6", "E6:10", "E7:9", "E8:4"},
            {"E1:3", "E2:1", "E3:2", "E4:5", "E5:1", "E6:2", "E7:1", "E8:2", "E9:1"},
            {"S1:5", "S2:6", "S3:3", "S4:4", "S1:5", "S2:5", "S1:4", "S4:-1",
                    "S1:6", "S4:10", "S4:11", "S3:3", "S4:-1", "S3:2", "S1:4",
                    "S1:7", "S4:12", "S4:12", "S1:7", "S2:4", "S1:5", "S2:5"},
            {"E1:5", "E2:3", "E3:6", "E4:4", "E5:6", "E6:10", "E7:9", "E8:4"}
    };

    private static final int EXPECTED_STEPS = 102;
    private static final int EXPECTED_RECORDS = 47;

    private RowRecogAfterScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogAfterScenarioOracle <scenario.json>");
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
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportRecogBean", beanType);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
            int deployCount = 0;
            boolean active = false;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    String epl = step.getString("epl", "");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(
                                    SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                    deployCount++;
                    writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                    writer.statement.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    writer = null;
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("snapshot".equals(operation)) {
                    if (writer == null) {
                        throw new IllegalStateException("snapshot without a deployed statement");
                    }
                    writer.snapshot();
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportRecogBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("value", payload.get("value").asInt());
        runtime.getEventService().sendEventMap(event, "SupportRecogBean");
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned ops — deploy s0 with the verbatim EPL, the assertion
     * sends with pinned payloads, iterator snapshots and undeploy-all.
     * ord0 carries two deploy cycles. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!"SupportRecogBean".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "value");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String actual = string(payload, "theString") + ":" + integer(payload, "value");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    case "snapshot":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " send count is not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
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
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
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
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            append("listener", ++sequence, newEvents, oldEvents);
        }

        private void snapshot() {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append("snapshot", 0, events.toArray(new EventBean[0]), null);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
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
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
            }
            if (value instanceof EventBean[] events) {
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    array.add(normalize(event));
                }
                return array;
            }
            if (value instanceof EventBean event) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
