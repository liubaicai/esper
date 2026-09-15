import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
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
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals 8-9:
 * ResultSetLocalUngroupedUnidirectionalJoin (the SupportBean_S0 driver emits one
 * listener record per send, each carrying the whole batch of per-event
 * local-group rows) and ResultSetLocalUngroupedThreeLevelWTop (every
 * SupportBean send emits one listener record with the local group_by
 * aggregates and windows at three levels).  Each case replays in its own
 * runtime with the clock held at zero, mirroring the Java executions where
 * env.milestone is a no-op.  The join execution asserts its rows in any order,
 * so the oracle matches the batch as a multiset while recording the rows in
 * the order the listener emitted them; the window columns of the second case
 * are checked against the exact SupportBean objects sent by the replay.
 */
public final class ResultSetQueryTypeLocalGroupByExtendedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-extended";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 8-9: ungrouped local aggregation over a unidirectional join"
                    + " and over multi-level window access.";
    private static final String[] CASES = {"ungrouped-unidirectional-join", "ungrouped-three-level-wtop"};
    private static final int[] ORDINALS = {8, 9};
    private static final String[] RUNTIMES = {
            "java-runtime-a1b494ebb2cb69d4c90a", "java-runtime-344d65c309eddd141cc5"
    };
    private static final String[] NAMES = {
            "ResultSetLocalUngroupedUnidirectionalJoin", "ResultSetLocalUngroupedThreeLevelWTop"
    };
    private static final String[] OBSERVATIONS = {"listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0};
    private static final int[] CASE_RECORDS = {2, 5};
    private static final String[] STATIC_IDS = {"java-d4c45f0591f6b53f8f74", "java-3e936d8e8328dbd8a2ed"};
    private static final String EPL_JOIN = "@name('s0') select theString, sum(intPrimitive, group_by:theString)"
            + " as c0 from SupportBean#keepall, SupportBean_S0 unidirectional";
    private static final String EPL_WTOP = "@Name('s0') select sum(longPrimitive, group_by:theString) as c0,"
            + "count(*, group_by:theString) as c1,window(*, group_by:theString) as c2,"
            + "sum(longPrimitive, group_by:intPrimitive) as c3,count(*, group_by:intPrimitive) as c4,"
            + "window(*, group_by:intPrimitive) as c5,"
            + "sum(longPrimitive, group_by:(theString, intPrimitive)) as c6,"
            + "count(*, group_by:(theString, intPrimitive)) as c7,"
            + "window(*, group_by:(theString, intPrimitive)) as c8,sum(longPrimitive) as c9"
            + " from SupportBean#length(4)";
    private static final int JOIN_SENDS = 4;
    private static final int WTOP_SENDS = 5;
    private static final String[] JOIN_FIELDS = {"c0", "theString"};
    // Java's assertPropsPerRowLastNewAnyOrder pins each join batch as an
    // unordered set of rows; the oracle matches the batch as a multiset and the
    // trace records the rows in the order the runtime emitted them, which is
    // the retained window order.
    private static final String[][] JOIN_STRINGS = {{"E1", "E2", "E1"}, {"E1", "E2", "E1", "E1"}};
    private static final long[][] JOIN_SUMS = {{40L, 20L, 40L}, {80L, 20L, 80L, 80L}};
    private static final String[] WTOP_FIELDS = {"c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9"};
    // c0, c1, c3, c4, c6, c7 and c9 of each wtop record, in that order.
    private static final long[][] WTOP_SCALARS = {
            {100L, 1L, 100L, 1L, 100L, 1L, 100L},
            {101L, 1L, 201L, 2L, 101L, 1L, 201L},
            {202L, 2L, 102L, 1L, 102L, 1L, 303L},
            {305L, 3L, 304L, 3L, 203L, 2L, 406L},
            {309L, 3L, 308L, 3L, 207L, 2L, 410L},
    };
    // c2, c5 and c8 of each wtop record as indexes into the sent SupportBeans.
    private static final int[][][] WTOP_WINDOWS = {
            {{0}, {0}, {0}},
            {{1}, {0, 1}, {1}},
            {{0, 2}, {2}, {2}},
            {{0, 2, 3}, {0, 1, 3}, {0, 3}},
            {{2, 3, 4}, {1, 3, 4}, {3, 4}},
    };
    private static final String[] BEAN_FIELDS = {
            "bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive",
            "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed",
            "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed",
            "shortPrimitive", "theString"
    };

    private ResultSetQueryTypeLocalGroupByExtendedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupByExtendedScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, records, index);
        }
        if (records.size() != 7) {
            throw new IllegalStateException("expected seven trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, JsonArray records, int index) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPStatement statement = deploy(runtime, configuration, index);
            SupportBean[] sent = new SupportBean[WTOP_SENDS];
            ListenerWriter writer = new ListenerWriter(records, statement, runtime, sent, index);
            statement.addListener(writer);
            boolean active = false;
            int markers = 0;
            int sends = 0;
            int beans = 0;
            for (int stepIndex = 0; stepIndex < steps.size(); stepIndex++) {
                JsonObject step = object(steps.get(stepIndex), "step " + stepIndex);
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    if (CASES[index].equals(string(step, "case"))) {
                        if (++markers != 1) {
                            throw new IllegalStateException("duplicate active case marker");
                        }
                        active = true;
                    } else if (active) {
                        active = false;
                    }
                    continue;
                }
                if (!active) {
                    continue;
                }
                if (!"send".equals(operation)) {
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
                sends++;
                if ("SupportBean".equals(string(step, "eventType"))) {
                    if (beans >= sent.length) {
                        throw new IllegalArgumentException("unexpected SupportBean send at step " + stepIndex);
                    }
                    SupportBean bean = bean(step, stepIndex);
                    sent[beans++] = bean;
                    runtime.getEventService().sendEventBean(bean, "SupportBean");
                } else {
                    sendS0(runtime, step, stepIndex);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
            }
            int expectedSends = index == 0 ? JOIN_SENDS + 2 : WTOP_SENDS;
            if (sends != expectedSends) {
                throw new IllegalStateException("expected " + expectedSends + " sends for " + CASES[index] + ", got "
                        + sends);
            }
            if (index == 0 && beans != JOIN_SENDS) {
                throw new IllegalStateException("expected four SupportBean sends for " + CASES[0] + ", got " + beans);
            }
            if (writer.sequence != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " listener records for "
                        + CASES[index] + ", got " + writer.sequence);
            }
            if (records.size() - before != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " records for " + CASES[index]
                        + ", got " + (records.size() - before));
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement deploy(EPRuntime runtime, Configuration configuration, int index) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
        return findStatement(deployment);
    }

    private static String epl(int index) {
        return index == 0 ? EPL_JOIN : EPL_WTOP;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static SupportBean bean(JsonObject step, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
        return bean;
    }

    private static void sendS0(EPRuntime runtime, JsonObject step, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "id");
        runtime.getEventService().sendEventBean(new SupportBean_S0(integer(payload, "id")), "SupportBean_S0");
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
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("expected two cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(string(definition, "runtimeId"))
                    || !NAMES[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS[index]
                    || !epl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 13) {
            throw new IllegalArgumentException("scenario must contain exactly thirteen steps");
        }
        validateCaseMarker(steps.get(0), 0, CASES[0]);
        validateBeanSend(steps.get(1), 1, "E1", 10, 0L);
        validateBeanSend(steps.get(2), 2, "E2", 20, 0L);
        validateBeanSend(steps.get(3), 3, "E1", 30, 0L);
        validateS0Send(steps.get(4), 4, 1);
        validateBeanSend(steps.get(5), 5, "E1", 40, 0L);
        validateS0Send(steps.get(6), 6, 1);
        validateCaseMarker(steps.get(7), 7, CASES[1]);
        validateBeanSend(steps.get(8), 8, "E1", 10, 100L);
        validateBeanSend(steps.get(9), 9, "E2", 10, 101L);
        validateBeanSend(steps.get(10), 10, "E1", 20, 102L);
        validateBeanSend(steps.get(11), 11, "E1", 10, 103L);
        validateBeanSend(steps.get(12), 12, "E1", 10, 104L);
    }

    private static void validateCaseMarker(JsonValue value, int index, String expectedCase) {
        JsonObject step = object(value, "step " + index);
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !expectedCase.equals(string(step, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned at step " + index);
        }
    }

    private static void validateBeanSend(JsonValue value, int index, String expectedString,
                                         int expectedInt, long expectedLong) {
        JsonObject step = object(value, "step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static void validateS0Send(JsonValue value, int index, int expectedId) {
        JsonObject step = object(value, "step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("S0 send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "id");
        if (integer(payload, "id") != expectedId) {
            throw new IllegalArgumentException("S0 send payload is not pinned at step " + index);
        }
    }

    private static JsonObject row(EventBean event) {
        String[] properties = event.getEventType().getPropertyNames().clone();
        Arrays.sort(properties);
        JsonObject fields = new JsonObject();
        for (String property : properties) {
            fields.add(property, normalize(event.get(property)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray output = new JsonArray();
        for (EventBean event : events) {
            output.add(row(event));
        }
        return output;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
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
        if (value instanceof SupportBean bean) {
            JsonObject fields = new JsonObject();
            fields.add("bigDecimal", normalize(bean.getBigDecimal()));
            fields.add("bigInteger", normalize(bean.getBigInteger()));
            fields.add("boolBoxed", normalize(bean.getBoolBoxed()));
            fields.add("boolPrimitive", normalize(bean.isBoolPrimitive()));
            fields.add("byteBoxed", normalize(bean.getByteBoxed()));
            fields.add("bytePrimitive", normalize(bean.getBytePrimitive()));
            fields.add("charBoxed", normalize(bean.getCharBoxed()));
            fields.add("charPrimitive", normalize(bean.getCharPrimitive()));
            fields.add("doubleBoxed", normalize(bean.getDoubleBoxed()));
            fields.add("doublePrimitive", normalize(bean.getDoublePrimitive()));
            fields.add("enumValue", normalize(bean.getEnumValue()));
            fields.add("floatBoxed", normalize(bean.getFloatBoxed()));
            fields.add("floatPrimitive", normalize(bean.getFloatPrimitive()));
            fields.add("intBoxed", normalize(bean.getIntBoxed()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("longBoxed", normalize(bean.getLongBoxed()));
            fields.add("longPrimitive", normalize(bean.getLongPrimitive()));
            fields.add("shortBoxed", normalize(bean.getShortBoxed()));
            fields.add("shortPrimitive", normalize(bean.getShortPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            return new JsonObject().add("kind", "row").add("fields", fields);
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
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (com.espertech.esper.common.client.json.minimaljson.Member member : value.asObject()) {
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
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
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

    private static final class ListenerWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final SupportBean[] sent;
        private final int caseIndex;
        private int sequence;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime,
                               SupportBean[] sent, int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.sent = sent;
            this.caseIndex = caseIndex;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > CASE_RECORDS[caseIndex]) {
                throw new IllegalStateException("unexpected listener callback after " + CASE_RECORDS[caseIndex]
                        + " records");
            }
            if (newEvents == null || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("listener callback must contain new rows only");
            }
            try {
                if (caseIndex == 0) {
                    assertJoinBatch(newEvents, next);
                } else {
                    assertWtopRow(newEvents, next);
                }
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L || !"1970-01-01T00:00:00Z".equals(Instant.ofEpochMilli(now).toString())) {
                throw new IllegalStateException("unexpected callback time");
            }
            sequence = next;
            records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents)));
        }

        private void assertJoinBatch(EventBean[] newEvents, int next) {
            String[] wantStrings = JOIN_STRINGS[next - 1];
            long[] wantSums = JOIN_SUMS[next - 1];
            if (newEvents.length != wantStrings.length) {
                throw new IllegalStateException("join batch " + next + " has " + newEvents.length
                        + " rows, want " + wantStrings.length);
            }
            for (EventBean row : newEvents) {
                assertResultFields(row, JOIN_FIELDS);
            }
            // Java asserts the batch in any order; match it as a multiset and
            // leave the emitted order for the trace.
            boolean[] matched = new boolean[newEvents.length];
            for (int wantIndex = 0; wantIndex < wantStrings.length; wantIndex++) {
                boolean found = false;
                for (int rowIndex = 0; rowIndex < newEvents.length; rowIndex++) {
                    if (matched[rowIndex]) {
                        continue;
                    }
                    if (matchesJoinRow(newEvents[rowIndex], wantStrings[wantIndex], wantSums[wantIndex])) {
                        matched[rowIndex] = true;
                        found = true;
                        break;
                    }
                }
                if (!found) {
                    throw new IllegalStateException("join batch " + next + " has no row " + wantStrings[wantIndex]
                            + "/" + wantSums[wantIndex]);
                }
            }
        }

        private void assertJoinRowTypes(EventBean row) {
            Object c0 = row.get("c0");
            Object theString = row.get("theString");
            if (!(c0 instanceof Integer) || !(theString instanceof String)) {
                throw new IllegalStateException("join row is not typed Integer/String: [" + theString + ", " + c0 + "]");
            }
        }

        private boolean matchesJoinRow(EventBean row, String expectedString, long expectedSum) {
            assertJoinRowTypes(row);
            return expectedString.equals(row.get("theString"))
                    && ((Integer) row.get("c0")).longValue() == expectedSum;
        }

        private void assertWtopRow(EventBean[] newEvents, int next) {
            if (newEvents.length != 1) {
                throw new IllegalStateException("wtop callback must contain one new row only");
            }
            EventBean row = newEvents[0];
            assertResultFields(row, WTOP_FIELDS);
            long[] scalars = WTOP_SCALARS[next - 1];
            int[][] windows = WTOP_WINDOWS[next - 1];
            assertLong(row, "c0", scalars[0]);
            assertLong(row, "c1", scalars[1]);
            assertWindow(row, "c2", windows[0]);
            assertLong(row, "c3", scalars[2]);
            assertLong(row, "c4", scalars[3]);
            assertWindow(row, "c5", windows[1]);
            assertLong(row, "c6", scalars[4]);
            assertLong(row, "c7", scalars[5]);
            assertWindow(row, "c8", windows[2]);
            assertLong(row, "c9", scalars[6]);
        }

        private void assertResultFields(EventBean row, String[] expected) {
            String[] names = row.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, expected)) {
                throw new IllegalStateException("result field metadata is not pinned: " + Arrays.toString(names));
            }
        }

        private void assertLong(EventBean row, String name, long expected) {
            Object value = row.get(name);
            if (!(value instanceof Long) || ((Long) value).longValue() != expected) {
                throw new IllegalStateException("wtop field " + name + " = " + value + ", want " + expected);
            }
        }

        private void assertWindow(EventBean row, String name, int[] expectedIndexes) {
            Object value = row.get(name);
            Object[] actual;
            if (value instanceof EventBean[] events) {
                actual = events;
            } else if (value instanceof Object[] objects) {
                actual = objects;
            } else {
                throw new IllegalStateException(name + " must be an event array");
            }
            if (actual.length != expectedIndexes.length) {
                throw new IllegalStateException(name + " length = " + actual.length + ", want "
                        + expectedIndexes.length);
            }
            for (int index = 0; index < expectedIndexes.length; index++) {
                SupportBean expected = sent[expectedIndexes[index]];
                Object item = actual[index];
                if (item instanceof EventBean event) {
                    if (event.getUnderlying() != expected) {
                        throw new IllegalStateException(name + " lost SupportBean identity/order at " + index);
                    }
                    assertSupportBeanEvent(event);
                } else if (item != expected) {
                    throw new IllegalStateException(name + " lost SupportBean identity/order at " + index);
                }
            }
        }

        private void assertSupportBeanEvent(EventBean event) {
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, BEAN_FIELDS)) {
                throw new IllegalStateException("SupportBean field metadata is not pinned: " + Arrays.toString(names));
            }
        }
    }
}
