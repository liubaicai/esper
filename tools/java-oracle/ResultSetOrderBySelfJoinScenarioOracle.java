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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the ResultSetOrderBySelfJoinSimple execution of
 * ResultSetOrderBySelfJoin, ordinal zero: a three-way self-join over
 * SupportHierarchyEvent#lastevent and two grouped #lastevent streams with an
 * ungrouped cast(count(*), int) and order by c2.priority asc.  Row-per-event
 * delivery is pinned: the first event delivers exactly one new-only row, and
 * each subsequent event delivers exactly one new-only two-row batch ordered by
 * c2.priority ascending.  The retained join set is additionally observed
 * through one statement iterator snapshot of the two ordered rows.
 */
public final class ResultSetOrderBySelfJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-orderby-self-join";
    private static final String DESCRIPTION =
            "ResultSetOrderBySelfJoin ordinals 0: three-way self-join with ungrouped count and order-by on a join field.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySelfJoin.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-74b0c1ce48febfe83007"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOrderBySelfJoinSimple"
    };
    private static final String[] STATIC_IDS = {
            "java-7cbb50aac0764e25b3bc"
    };
    private static final String[] CASES = {
            "selfjoin"
    };
    private static final int[] ORDINALS = {0};
    private static final String[] EPLS = {
            "@name('s0') select c1.event_criteria_id as ecid, "
                    + "c1.priority as priority, "
                    + "c2.priority as prio, cast(count(*), int) as cnt from "
                    + "SupportHierarchyEvent#lastevent as c1, "
                    + "SupportHierarchyEvent#groupwin(event_criteria_id)#lastevent as c2, "
                    + "SupportHierarchyEvent#groupwin(event_criteria_id)#lastevent as p "
                    + "where c2.event_criteria_id in (c1.event_criteria_id,2,1) "
                    + "and p.event_criteria_id in (c1.parent_event_criteria_id, c1.event_criteria_id) "
                    + "order by c2.priority asc"
    };

    private static final int[] SEND_EVENT_CRITERIA_IDS = {1, 3, 3};
    private static final int[] SEND_PRIORITIES = {1, 2, 2};
    private static final Integer[] SEND_PARENTS = {null, 2, 2};

    /** Row tuples follow the sorted EXPECTED_FIELDS order: cnt, ecid, prio, priority. */
    private static final String[] EXPECTED_FIELDS = {"cnt", "ecid", "prio", "priority"};
    private static final int[][] CALLBACK_ONE_ROWS = {{1, 1, 1, 1}};
    private static final int[][] CALLBACK_TWO_ROWS = {{2, 3, 1, 2}, {2, 3, 2, 2}};
    private static final int[][][] EXPECTED_CALLBACK_ROWS = {
            CALLBACK_ONE_ROWS,
            CALLBACK_TWO_ROWS,
            CALLBACK_TWO_ROWS
    };
    private static final int[][] SNAPSHOT_ROWS = CALLBACK_TWO_ROWS;
    private static final int[] EXPECTED_CALLBACKS = {3};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOrderBySelfJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderBySelfJoinScenarioOracle <scenario.json>");
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
        if (records.size() != 4) {
            throw new IllegalStateException("expected four listener and snapshot records, got "
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
        configuration.getCommon().addEventType("SupportHierarchyEvent", SupportHierarchyEvent.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetOrderBySelfJoinScenarioOracle-" + caseName, configuration);
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
        int snapshots = 0;
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
            if ("snapshot".equals(operation)) {
                if (!"selfjoin".equals(caseName)) {
                    throw new IllegalArgumentException("snapshot step in case " + caseName
                            + " is not supported; only case selfjoin carries snapshots");
                }
                emitSnapshot(step, statement, caseName, runtime, records, snapshots++);
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation
                        + " in case " + caseName);
            }
            sendEvent(runtime, step, caseName);
        }
    }

    private static void emitSnapshot(JsonObject step, EPStatement statement, String caseName,
                                     EPRuntime runtime, JsonArray records, int snapshotIndex) {
        validateSnapshotStep(step);
        List<EventBean> drained = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            drained.add(iterator.next());
        }
        validateIteratorSnapshot(caseName, snapshotIndex, drained);
        long now = runtime.getEventService().getCurrentTime();
        if (now != 0L) {
            throw new IllegalStateException("unexpected snapshot time " + now);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(now).toString());
        record.add("new", rows(drained.toArray(new EventBean[0])));
        records.add(record);
    }

    private static void validateIteratorSnapshot(String caseName, int snapshotIndex,
                                                 List<EventBean> drained) {
        if (drained.size() != SNAPSHOT_ROWS.length) {
            throw new IllegalStateException("case " + caseName + " iterator snapshot "
                    + (snapshotIndex + 1) + " must contain " + SNAPSHOT_ROWS.length
                    + " rows, got " + drained.size());
        }
        for (int rowIndex = 0; rowIndex < drained.size(); rowIndex++) {
            EventBean event = drained.get(rowIndex);
            String[] fields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(fields);
            if (!Arrays.equals(fields, EXPECTED_FIELDS)) {
                throw new IllegalStateException("case " + caseName + " iterator snapshot fields are "
                        + Arrays.toString(fields) + ", expected " + Arrays.toString(EXPECTED_FIELDS));
            }
            for (int fieldIndex = 0; fieldIndex < EXPECTED_FIELDS.length; fieldIndex++) {
                assertInteger(event.get(EXPECTED_FIELDS[fieldIndex]),
                        SNAPSHOT_ROWS[rowIndex][fieldIndex], EXPECTED_FIELDS[fieldIndex],
                        rowIndex, caseName);
            }
        }
    }

    private static void assertInteger(Object actual, long expected, String field, int rowIndex,
                                      String caseName) {
        if (!(actual instanceof Integer || actual instanceof Long || actual instanceof Short
                || actual instanceof Byte) || ((Number) actual).longValue() != expected) {
            throw new IllegalStateException("case " + caseName + " row " + rowIndex + " " + field
                    + " expected " + expected + ", got " + actual);
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = string(step, "eventType");
        if (!"SupportHierarchyEvent".equals(eventType)) {
            throw new IllegalArgumentException("unknown event type " + eventType
                    + " in case " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "event payload");
        requireFields(payload, "event_criteria_id", "priority", "parent_event_criteria_id");
        int eventCriteriaId = integer(payload, "event_criteria_id");
        int priority = integer(payload, "priority");
        JsonValue parentValue = payload.get("parent_event_criteria_id");
        Integer parent = null;
        if (parentValue instanceof JsonNumber) {
            parent = integer(payload, "parent_event_criteria_id");
        } else if (parentValue != Json.NULL) {
            throw new IllegalArgumentException("SupportHierarchyEvent parent_event_criteria_id in case "
                    + caseName + " must be an integer or null");
        }
        runtime.getEventService().sendEventBean(
                new SupportHierarchyEvent(eventCriteriaId, priority, parent), "SupportHierarchyEvent");
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
                    || integer(definition, "iteratorSnapshots") != 1
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 5) {
            throw new IllegalArgumentException("scenario must contain exactly five steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        for (int eventIndex = 0; eventIndex < SEND_EVENT_CRITERIA_IDS.length; eventIndex++) {
            validateHierarchyStep(steps.get(offset++), SEND_EVENT_CRITERIA_IDS[eventIndex],
                    SEND_PRIORITIES[eventIndex], SEND_PARENTS[eventIndex]);
        }
        validateSnapshotStep(steps.get(offset++));
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

    private static void validateHierarchyStep(JsonValue value, int expectedEventCriteriaId,
                                              int expectedPriority, Integer expectedParent) {
        JsonObject step = object(value, "SupportHierarchyEvent step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportHierarchyEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportHierarchyEvent step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportHierarchyEvent payload");
        requireFields(payload, "event_criteria_id", "priority", "parent_event_criteria_id");
        if (integer(payload, "event_criteria_id") != expectedEventCriteriaId
                || integer(payload, "priority") != expectedPriority) {
            throw new IllegalArgumentException("SupportHierarchyEvent payload is not pinned");
        }
        JsonValue parentValue = payload.get("parent_event_criteria_id");
        if (expectedParent == null) {
            if (parentValue != Json.NULL) {
                throw new IllegalArgumentException("SupportHierarchyEvent parent payload must be null");
            }
        } else if (!(parentValue instanceof JsonNumber)
                || longInteger(payload, "parent_event_criteria_id") != expectedParent) {
            throw new IllegalArgumentException("SupportHierarchyEvent parent payload is not pinned");
        }
    }

    private static void validateSnapshotStep(JsonValue value) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "statement");
        if (!"snapshot".equals(string(step, "op"))
                || !"s0".equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned");
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
            int next = sequence + 1;
            if (next > expectedCallbacks) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " produced more than " + expectedCallbacks + " listener callbacks");
            }
            int expectedRows = EXPECTED_CALLBACK_ROWS[next - 1].length;
            if (newEvents == null || newEvents.length != expectedRows
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " listener callback " + next + " must contain " + expectedRows
                        + " new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            validateRows(newEvents, EXPECTED_CALLBACK_ROWS[next - 1]);

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

        private void validateRows(EventBean[] events, int[][] expectedRows) {
            for (int rowIndex = 0; rowIndex < events.length; rowIndex++) {
                EventBean event = events[rowIndex];
                String[] fields = event.getEventType().getPropertyNames().clone();
                Arrays.sort(fields);
                if (!Arrays.equals(fields, EXPECTED_FIELDS)) {
                    throw new IllegalStateException("result field metadata is not pinned for case "
                            + CASES[caseIndex]);
                }
                for (int fieldIndex = 0; fieldIndex < EXPECTED_FIELDS.length; fieldIndex++) {
                    assertInteger(event.get(EXPECTED_FIELDS[fieldIndex]),
                            expectedRows[rowIndex][fieldIndex], EXPECTED_FIELDS[fieldIndex],
                            rowIndex, CASES[caseIndex]);
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

    /** Local mirror of the pinned SupportHierarchyEvent regression bean. */
    public static final class SupportHierarchyEvent {
        private final Integer event_criteria_id;
        private final Integer priority;
        private final Integer parent_event_criteria_id;

        public SupportHierarchyEvent(Integer event_criteria_id, Integer priority,
                                     Integer parent_event_criteria_id) {
            this.event_criteria_id = event_criteria_id;
            this.priority = priority;
            this.parent_event_criteria_id = parent_event_criteria_id;
        }

        public Integer getEvent_criteria_id() {
            return event_criteria_id;
        }

        public Integer getPriority() {
            return priority;
        }

        public Integer getParent_event_criteria_id() {
            return parent_event_criteria_id;
        }
    }
}
