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
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals 20/21,
 * the named-window row-removal executions:
 * ResultSetLocalUngroupedRowRemove (ungrouped sum over a keepall window) and
 * ResultSetLocalGroupedRowRemove (the same selection grouped by theString,
 * intPrimitive).
 *
 * Each case compiles and deploys the five statements of its scenario {@code epl}
 * field in one compileDeploy, exactly like the Java execution, subscribes one
 * listener to the {@code @name('s0')} query and replays the nine sends
 * (SupportBean plus the SupportBean_S0 row-delete and SupportBean_S1 delete-all
 * triggers).  The ungrouped query answers only the six inserting sends: the two
 * row-delete sends and the delete-all send deliver no callback at all, which the
 * per-send callback census pins.  The grouped query answers every send because
 * the group rows are projected as new rows of the removal, the delete-all
 * arriving as a single callback whose new rows are the three group rows the
 * delete removed.
 *
 * The Java grouped execution discards that delete-all delivery with
 * {@code env.listenerReset("s0")} (a reset without any assertion), so the oracle
 * records what the runtime actually delivers there.  The rows of that multi-row
 * delivery arrive in the group map's iteration order; this oracle verifies them
 * as a set and writes them ascending by (theString, intPrimitive) so the trace
 * row order is deterministic.
 *
 * Every callback must be new-only (the queries are istream-only selects over a
 * window), carry no old rows, arrive synchronously during its pinned send and be
 * stamped at 1970-01-01T00:00:00Z: each case owns its runtime and deployment,
 * the internal timer is disabled and the clock stays at zero.  Rows render as
 * the shared result row shape and null aggregates as the shared
 * {"state":"null"} marker.
 */
public final class ResultSetQueryTypeLocalGroupRowRemoveScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-row-remove";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 20/21: named-window row removal with ungrouped"
                    + " and grouped local-group aggregates.";
    private static final String[] CASES = {"ungrouped-row-remove", "grouped-row-remove"};
    private static final int[] ORDINALS = {20, 21};
    private static final String[] RUNTIMES = {
            "java-runtime-3a47ac428a21e66d8614", "java-runtime-07d68fdd9a8dffc8c7f4"
    };
    private static final String[] NAMES = {
            "ResultSetLocalUngroupedRowRemove", "ResultSetLocalGroupedRowRemove"
    };
    private static final String[] OBSERVATIONS = {"listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0};
    private static final String[] STATIC_IDS = {
            "java-3da6ea3da5df0d53578a", "java-ce99d7bbb48728947c59"
    };
    private static final String EPL_UNGROUPED = "create window MyWindow#keepall as SupportBean;\n"
            + "insert into MyWindow select * from SupportBean;\n"
            + "on SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\n"
            + "on SupportBean_S1 delete from MyWindow;\n"
            + "@name('s0') select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive,"
            + " group_by:theString) as c1 from MyWindow;\n";
    private static final String EPL_GROUPED = "create window MyWindow#keepall as SupportBean;\n"
            + "insert into MyWindow select * from SupportBean;\n"
            + "on SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\n"
            + "on SupportBean_S1 delete from MyWindow;\n"
            + "@name('s0') select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive,"
            + " group_by:theString) as c1   from MyWindow group by theString, intPrimitive;\n";
    // The five statements each case EPL carries: the four unnamed statements
    // (auto-named by position) plus the @name('s0') query the listener attaches to.
    private static final String[] DEPLOYED_STATEMENT_NAMES = {"s0", "stmt-0", "stmt-1", "stmt-2", "stmt-3"};
    private static final int[] CASE_RECORDS = {6, 9};
    private static final int[] CASE_SENDS = {9, 9};
    private static final String[] STEP_OPS = {
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send"
    };
    private static final String[] CASE_MARKERS = {"ungrouped-row-remove", "grouped-row-remove"};
    private static final String[] SEND_EVENT_TYPES = {
            "SupportBean", "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean",
            "SupportBean_S0", "SupportBean", "SupportBean_S1", "SupportBean"
    };
    // SEND_STRINGS: theString for the SupportBean sends, p00 for the two
    // SupportBean_S0 sends and null for the SupportBean_S1 send, which carries no
    // string property (the Java execution builds it as new SupportBean_S1(-1)).
    private static final String[] SEND_STRINGS = {"E1", "E1", "E1", "E2", "E1", "E1", "E1", null, "E1"};
    // SEND_INTS: intPrimitive for the SupportBean sends, id for the SupportBean_S0
    // and SupportBean_S1 sends.
    private static final int[] SEND_INTS = {10, 10, 20, 30, 40, 40, 50, -1, 60};
    // SEND_LONGS: longPrimitive for the SupportBean sends; the row-delete and
    // delete-all sends do not carry it, so those slots stay at zero.
    private static final long[] SEND_LONGS = {101, 0, 102, 103, 104, 0, 105, 0, 106};
    private static final String[] RESULT_FIELDS = {"theString", "intPrimitive", "c0", "c1"};
    // The six ungrouped-row-remove callbacks in Java assertion order: every
    // inserting send answers one row, both row-delete sends and the delete-all
    // send stay silent.  c0 (the whole-window sum) and c1 (the theString-local
    // sum) stay Long.
    private static final Object[][] UNGROUPED_ROWS = {
            {"E1", 10, 101L, 101L},
            {"E1", 20, 102L, 102L},
            {"E2", 30, 205L, 103L},
            {"E1", 40, 309L, 206L},
            {"E1", 50, 310L, 207L},
            {"E1", 60, 106L, 106L},
    };
    // The eight single-row grouped-row-remove callbacks in Java assertion order;
    // the null slot at index 7 (record 8) is the delete-all delivery, which
    // arrives as one callback carrying the three removed group rows (below).
    private static final Object[][] GROUPED_ROWS = {
            {"E1", 10, 101L, 101L},
            {"E1", 10, null, null},
            {"E1", 20, 102L, 102L},
            {"E2", 30, 103L, 103L},
            {"E1", 40, 104L, 206L},
            {"E1", 40, null, 102L},
            {"E1", 50, 105L, 207L},
            null,
            {"E1", 60, 106L, 106L},
    };
    private static final Object[][][] CASE_ROWS = {UNGROUPED_ROWS, GROUPED_ROWS};
    // The grouped SupportBean_S1(-1) delete-all: Esper answers with one callback
    // whose new rows are the three group rows just removed (E1/20, E1/50, E2/30
    // after the two earlier row deletes), each with the null local-group sums the
    // removal leaves behind.  The oracle verifies them as a set and records them
    // ascending by (theString, intPrimitive), the order the trace pins.
    private static final Object[][] DELETE_ALL_ROWS = {
            {"E1", 20, null, null},
            {"E1", 50, null, null},
            {"E2", 30, null, null},
    };
    // Per record: the 1-based send whose synchronous callbacks the record is.
    private static final int[][] CASE_SEND_ORDINALS = {
            {1, 3, 4, 5, 7, 9},
            {1, 2, 3, 4, 5, 6, 7, 8, 9},
    };
    // Per send: the number of listener callbacks it must deliver.  The ungrouped
    // case is callback-free on both row-delete sends (2 and 6) and on the
    // delete-all send (8); the grouped case answers every send.
    private static final int[][] CASE_CALLBACKS_PER_SEND = {
            {1, 0, 1, 1, 1, 0, 1, 0, 1},
            {1, 1, 1, 1, 1, 1, 1, 1, 1},
    };

    private ResultSetQueryTypeLocalGroupRowRemoveScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupRowRemoveScenarioOracle <scenario.json>");
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
        if (records.size() != 15) {
            throw new IllegalStateException("expected fifteen trace records, got " + records.size());
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
        configuration.getCommon().addEventType(SupportBean_S1.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPStatement statement = deploy(runtime, index);
            ListenerWriter writer = new ListenerWriter(records, statement, runtime, index);
            statement.addListener(writer);
            boolean active = false;
            int markers = 0;
            int sends = 0;
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
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": "
                            + operation);
                }
                if (sends >= CASE_SENDS[index]) {
                    throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                }
                // Any callback this send produces arrives synchronously.
                writer.sendOrdinal = sends + 1;
                send(runtime, step, stepIndex);
                sends++;
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
            }
            if (sends != CASE_SENDS[index]) {
                throw new IllegalStateException("expected " + CASE_SENDS[index] + " sends for " + CASES[index]
                        + ", got " + sends);
            }
            if (!Arrays.equals(writer.callbacksPerSend, CASE_CALLBACKS_PER_SEND[index])) {
                throw new IllegalStateException(CASES[index] + " callback census is " + Arrays.toString(
                        writer.callbacksPerSend) + ", want " + Arrays.toString(CASE_CALLBACKS_PER_SEND[index]));
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

    private static EPStatement deploy(EPRuntime runtime, int index) throws Exception {
        // The scenario pins this EPL verbatim, so the constant and the scenario
        // field are the same five statements.
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != DEPLOYED_STATEMENT_NAMES.length) {
            throw new IllegalStateException("expected the five statements of the " + CASES[index] + " EPL");
        }
        EPStatement wanted = null;
        Set<String> names = new HashSet<>();
        for (EPStatement statement : statements) {
            names.add(statement.getName());
            if ("s0".equals(statement.getName())) {
                wanted = statement;
            }
        }
        if (wanted == null
                || !names.equals(new HashSet<>(Arrays.asList(DEPLOYED_STATEMENT_NAMES)))) {
            throw new IllegalStateException("deployed statement names are not pinned: " + names);
        }
        return wanted;
    }

    private static String epl(int index) {
        return index == 0 ? EPL_UNGROUPED : EPL_GROUPED;
    }

    private static void send(EPRuntime runtime, JsonObject step, int stepIndex) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
            bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
            runtime.getEventService().sendEventBean(bean, eventType);
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            requireFields(payload, "id", "p00");
            runtime.getEventService().sendEventBean(
                    new SupportBean_S0(integer(payload, "id"), string(payload, "p00")), eventType);
            return;
        }
        if ("SupportBean_S1".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S1(integer(payload, "id")), eventType);
            return;
        }
        throw new IllegalArgumentException("unsupported event type at step " + stepIndex + ": " + eventType);
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
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly twenty steps");
        }
        int sendIndex = 0;
        int markerIndex = 0;
        for (int index = 0; index < STEP_OPS.length; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if (!STEP_OPS[index].equals(operation)) {
                throw new IllegalArgumentException("step " + index + " operation is " + operation + ", want "
                        + STEP_OPS[index]);
            }
            if ("case".equals(operation)) {
                requireFields(step, "op", "case");
                if (markerIndex >= CASE_MARKERS.length || !CASE_MARKERS[markerIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("case marker is not pinned at step " + index);
                }
                markerIndex++;
            } else {
                // Both cases replay the same nine sends.
                validateSend(step, index, sendIndex % SEND_EVENT_TYPES.length);
                sendIndex++;
            }
        }
        if (markerIndex != CASE_MARKERS.length || sendIndex != SEND_EVENT_TYPES.length * CASES.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        String eventType = string(step, "eventType");
        if (!SEND_EVENT_TYPES[sendIndex].equals(eventType)) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            if (!SEND_STRINGS[sendIndex].equals(string(payload, "theString"))
                    || integer(payload, "intPrimitive") != SEND_INTS[sendIndex]
                    || longNumber(payload, "longPrimitive") != SEND_LONGS[sendIndex]) {
                throw new IllegalArgumentException("SupportBean payload is not pinned at step " + index);
            }
        } else if ("SupportBean_S0".equals(eventType)) {
            requireFields(payload, "id", "p00");
            if (integer(payload, "id") != SEND_INTS[sendIndex]
                    || !SEND_STRINGS[sendIndex].equals(string(payload, "p00"))) {
                throw new IllegalArgumentException("SupportBean_S0 payload is not pinned at step " + index);
            }
        } else {
            requireFields(payload, "id");
            if (integer(payload, "id") != SEND_INTS[sendIndex]) {
                throw new IllegalArgumentException("SupportBean_S1 payload is not pinned at step " + index);
            }
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

    /**
     * Pins the result metadata of a row as a sorted set of names: the runtime
     * sorts the result event's property names, so the pinned selection order is
     * sorted here instead of being transcribed a second time by hand.
     */
    private static void assertResultFields(EventBean row, String label) {
        String[] names = row.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        String[] wanted = RESULT_FIELDS.clone();
        Arrays.sort(wanted);
        if (!Arrays.equals(names, wanted)) {
            throw new IllegalStateException(label + " result field metadata is not pinned: "
                    + Arrays.toString(names) + ", want " + Arrays.toString(wanted));
        }
    }

    private static void assertRow(EventBean row, Object[] expected, String label) {
        assertResultFields(row, label);
        assertTyped(row, "theString", expected[0], label);
        assertTyped(row, "intPrimitive", expected[1], label);
        assertTyped(row, "c0", expected[2], label);
        assertTyped(row, "c1", expected[3], label);
    }

    private static void assertTyped(EventBean row, String name, Object expected, String label) {
        Object actual = row.get(name);
        if (expected == null) {
            if (actual != null) {
                throw new IllegalStateException(label + " field " + name + " = " + actual + " ("
                        + simpleName(actual) + "), want null");
            }
            return;
        }
        if (!expected.getClass().isInstance(actual) || !expected.equals(actual)) {
            throw new IllegalStateException(label + " field " + name + " = " + actual + " ("
                    + simpleName(actual) + "), want " + expected + " (" + simpleName(expected) + ")");
        }
    }

    private static String simpleName(Object value) {
        return value == null ? "null" : value.getClass().getSimpleName();
    }

    /**
     * The delete-all callback's rows in the trace order the contract pins:
     * ascending by (theString, intPrimitive).
     */
    private static EventBean[] sortedByGroupKey(EventBean[] events) {
        EventBean[] sorted = events.clone();
        Arrays.sort(sorted, (left, right) -> {
            int byString = ((String) left.get("theString")).compareTo((String) right.get("theString"));
            if (byString != 0) {
                return byString;
            }
            return Integer.compare((Integer) left.get("intPrimitive"), (Integer) right.get("intPrimitive"));
        });
        return sorted;
    }

    private static final class ListenerWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int caseIndex;
        private final int[] callbacksPerSend;
        private int sequence;
        private int sendOrdinal;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseIndex = caseIndex;
            this.callbacksPerSend = new int[CASE_SENDS[caseIndex]];
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            try {
                int record = sequence + 1;
                if (record > CASE_RECORDS[caseIndex]) {
                    throw new IllegalStateException("unexpected listener callback after " + CASE_RECORDS[caseIndex]
                            + " records for " + CASES[caseIndex]);
                }
                // The callback must be the synchronous answer of the pinned send:
                // this is what fails the run when a row-delete or delete-all send
                // of the ungrouped case answers at all.
                if (sendOrdinal < 1 || sendOrdinal > CASE_SENDS[caseIndex]) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + record
                            + " arrived outside a send");
                }
                if (CASE_SEND_ORDINALS[caseIndex][record - 1] != sendOrdinal) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + record + " arrived during send "
                            + sendOrdinal + ", want send " + CASE_SEND_ORDINALS[caseIndex][record - 1]);
                }
                // Both queries are istream-only selects over a window, so no
                // callback ever carries old data.
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + record + " delivered "
                            + oldEvents.length + " old rows");
                }
                if (newEvents == null || newEvents.length == 0) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + record
                            + " carried no new rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                if (now != 0L) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + record + " is at time " + now
                            + ", want 1970-01-01T00:00:00Z");
                }
                callbacksPerSend[sendOrdinal - 1]++;
                Object[] expected = CASE_ROWS[caseIndex][record - 1];
                EventBean[] delivered;
                if (expected == null) {
                    delivered = assertDeleteAllRows(newEvents, record);
                } else {
                    if (newEvents.length != 1) {
                        throw new IllegalStateException(CASES[caseIndex] + " callback " + record + " must carry"
                                + " exactly one new row, got " + newEvents.length);
                    }
                    assertRow(newEvents[0], expected, label(record, 0));
                    delivered = newEvents;
                }
                sequence = record;
                records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(delivered)));
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        /**
         * The grouped delete-all delivery: one callback carrying the three group
         * rows the delete removed, verified as a set and recorded ascending by
         * (theString, intPrimitive).
         */
        private EventBean[] assertDeleteAllRows(EventBean[] newEvents, int record) {
            if (newEvents.length != DELETE_ALL_ROWS.length) {
                throw new IllegalStateException(CASES[caseIndex] + " record " + record + " carries "
                        + newEvents.length + " rows, want " + DELETE_ALL_ROWS.length);
            }
            EventBean[] sorted = sortedByGroupKey(newEvents);
            for (int index = 0; index < sorted.length; index++) {
                assertRow(sorted[index], DELETE_ALL_ROWS[index], label(record, index));
            }
            return sorted;
        }

        private String label(int record, int rowIndex) {
            return CASES[caseIndex] + " record " + record + " row " + rowIndex;
        }
    }
}
