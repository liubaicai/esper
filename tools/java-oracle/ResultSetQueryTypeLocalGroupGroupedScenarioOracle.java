import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.List;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals
 * 10/11/14, the grouped local-group executions.
 * ResultSetLocalGroupedSimple replays five SupportBean sends through a
 * length(4) window over "group by theString, intPrimitive" and pins one
 * new-only row per send; the Java execution asserts exactly one new row and no
 * old data through assertPropsNew.  ResultSetLocalGroupedMultiLevelMethod and
 * ResultSetLocalGroupedMultiLevelNoDefaultLvl mirror the sendTime(env, 0) /
 * 10000 / 20000 protocol and pin the three-row snapshot-every batches.
 * Java delivers those snapshot batches in HashMap group order, which is not a
 * contract, so each batch is matched as a multiset and the trace records it in
 * the canonical (theString, intPrimitive) order.  The internal timer is
 * disabled, every case owns its own runtime, deployment and listener, and the
 * case-zero listener additionally reports (on stderr, never inside the trace)
 * any old rows the runtime delivers for the five sends.
 */
public final class ResultSetQueryTypeLocalGroupGroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-grouped";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 10/11/14: grouped local-group aggregates over a length"
                    + " window and under snapshot-every output.";
    private static final String[] CASES = {
            "grouped-simple", "grouped-multi-level-method", "grouped-multi-level-no-default-lvl"
    };
    private static final int[] ORDINALS = {10, 11, 14};
    private static final String[] RUNTIMES = {
            "java-runtime-49be8402f85fb067edc3", "java-runtime-9a3385eedb2df5f1627c",
            "java-runtime-6f77d02f924ecaa4c3ca"
    };
    private static final String[] NAMES = {
            "ResultSetLocalGroupedSimple", "ResultSetLocalGroupedMultiLevelMethod",
            "ResultSetLocalGroupedMultiLevelNoDefaultLvl"
    };
    private static final String[] OBSERVATIONS = {"listener", "listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0, 0};
    private static final int[] CASE_RECORDS = {5, 2, 2};
    private static final int[] CASE_SENDS = {5, 6, 6};
    private static final int[] CASE_ADVANCES = {0, 3, 3};
    private static final String[] STATIC_IDS = {
            "java-c2618f1757daf7867b21", "java-c4b89d0fa19bf787a80e", "java-59de6fa8771761db231b"
    };
    private static final String EPL_SIMPLE = "@Name('s0') select sum(longPrimitive, group_by:theString) as c0,"
            + "count(*, group_by:theString) as c1,window(*, group_by:theString) as c2,"
            + "sum(longPrimitive, group_by:intPrimitive) as c3,count(*, group_by:intPrimitive) as c4,"
            + "window(*, group_by:intPrimitive) as c5,sum(longPrimitive, group_by:()) as c6,"
            + "count(*, group_by:()) as c7,window(*, group_by:()) as c8,sum(longPrimitive) as c9"
            + " from SupportBean#length(4)group by theString, intPrimitive";
    private static final String EPL_METHOD = "@name('s0') select theString, intPrimitive,"
            + " sum(longPrimitive, group_by:(intPrimitive, theString)) as c0, sum(longPrimitive) as c1,"
            + " sum(longPrimitive, group_by:(theString)) as c2, sum(longPrimitive, group_by:(intPrimitive)) as c3,"
            + " sum(longPrimitive, group_by:()) as c4 from SupportBean group by theString, intPrimitive"
            + " output snapshot every 10 seconds";
    private static final String EPL_NODEFAULT = "@name('s0') select theString, intPrimitive,"
            + " sum(longPrimitive, group_by:(theString)) as c0, sum(longPrimitive, group_by:(intPrimitive)) as c1,"
            + " sum(longPrimitive, group_by:()) as c2 from SupportBean group by theString, intPrimitive"
            + " output snapshot every 10 seconds";
    private static final String[] STEP_OPS = {
            "case", "send", "send", "send", "send", "send",
            "case", "advance-time", "send", "send", "send", "send", "send", "advance-time", "send", "advance-time",
            "case", "advance-time", "send", "send", "send", "send", "send", "advance-time", "send", "advance-time"
    };
    private static final String[] CASE_MARKERS = {
            "grouped-simple", "grouped-multi-level-method", "grouped-multi-level-no-default-lvl"
    };
    private static final String[] ADVANCE_ATS = {
            "1970-01-01T00:00:00Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:20Z",
            "1970-01-01T00:00:00Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:20Z"
    };
    private static final String[] SEND_STRINGS = {
            "E1", "E2", "E1", "E1", "E1",
            "E1", "E1", "E2", "E1", "E2", "E1",
            "E1", "E1", "E2", "E1", "E2", "E1"
    };
    private static final int[] SEND_INTS = {10, 10, 20, 10, 10, 10, 20, 10, 10, 10, 10, 10, 20, 10, 10, 10, 10};
    private static final long[] SEND_LONGS = {
            100, 101, 102, 103, 104,
            100, 202, 303, 404, 505, 1,
            100, 202, 303, 404, 505, 1
    };
    private static final String[] SIMPLE_FIELDS = {"c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9"};
    // c0, c1, c3, c4, c6, c7 and c9 of each grouped-simple row, in that order,
    // exactly as the Java execution's assertPropsNew pins them.
    private static final String[] SIMPLE_SCALAR_FIELDS = {"c0", "c1", "c3", "c4", "c6", "c7", "c9"};
    private static final long[][] SIMPLE_SCALARS = {
            {100L, 1L, 100L, 1L, 100L, 1L, 100L},
            {101L, 1L, 201L, 2L, 201L, 2L, 101L},
            {202L, 2L, 102L, 1L, 303L, 3L, 102L},
            {305L, 3L, 304L, 3L, 406L, 4L, 203L},
            {309L, 3L, 308L, 3L, 410L, 4L, 207L},
    };
    // c2, c5 and c8 of each grouped-simple row as indexes into the sent
    // SupportBeans, in the order the Java execution asserts them.
    private static final int[][][] SIMPLE_WINDOWS = {
            {{0}, {0}, {0}},
            {{1}, {0, 1}, {0, 1}},
            {{0, 2}, {2}, {0, 1, 2}},
            {{0, 2, 3}, {0, 1, 3}, {0, 1, 2, 3}},
            {{2, 3, 4}, {1, 3, 4}, {1, 2, 3, 4}},
    };
    private static final String[] METHOD_FIELDS = {"theString", "intPrimitive", "c0", "c1", "c2", "c3", "c4"};
    private static final String[] SORTED_METHOD_FIELDS =
            {"c0", "c1", "c2", "c3", "c4", "intPrimitive", "theString"};
    private static final Object[][][] METHOD_ROWS = {
            {
                    {"E1", 10, 504L, 504L, 706L, 1312L, 1514L},
                    {"E1", 20, 202L, 202L, 706L, 202L, 1514L},
                    {"E2", 10, 808L, 808L, 808L, 1312L, 1514L},
            },
            {
                    {"E1", 10, 505L, 505L, 707L, 1313L, 1515L},
                    {"E1", 20, 202L, 202L, 707L, 202L, 1515L},
                    {"E2", 10, 808L, 808L, 808L, 1313L, 1515L},
            },
    };
    private static final String[] NODEFAULT_FIELDS = {"theString", "intPrimitive", "c0", "c1", "c2"};
    private static final String[] SORTED_NODEFAULT_FIELDS = {"c0", "c1", "c2", "intPrimitive", "theString"};
    private static final Object[][][] NODEFAULT_ROWS = {
            {
                    {"E1", 10, 706L, 1312L, 1514L},
                    {"E1", 20, 706L, 202L, 1514L},
                    {"E2", 10, 808L, 1312L, 1514L},
            },
            {
                    {"E1", 10, 707L, 1313L, 1515L},
                    {"E1", 20, 707L, 202L, 1515L},
                    {"E2", 10, 808L, 1313L, 1515L},
            },
    };
    private static final String[] BEAN_FIELDS = {
            "bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive",
            "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed",
            "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed",
            "shortPrimitive", "theString"
    };

    private ResultSetQueryTypeLocalGroupGroupedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupGroupedScenarioOracle <scenario.json>");
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
        List<String> observations = new ArrayList<>();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, records, observations, index);
        }
        if (records.size() != 9) {
            throw new IllegalStateException("expected nine trace records, got " + records.size());
        }
        for (String observation : observations) {
            System.err.println(observation);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, JsonArray records, List<String> observations, int index)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPStatement statement = deploy(runtime, configuration, index);
            SupportBean[] sent = new SupportBean[CASE_SENDS[index]];
            ListenerWriter writer = new ListenerWriter(records, statement, runtime, sent, observations, index);
            statement.addListener(writer);
            boolean active = false;
            int markers = 0;
            int sends = 0;
            int advances = 0;
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
                if ("send".equals(operation)) {
                    if (sends >= sent.length) {
                        throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                    }
                    SupportBean bean = bean(step, stepIndex);
                    sent[sends++] = bean;
                    runtime.getEventService().sendEventBean(bean, "SupportBean");
                } else if ("advance-time".equals(operation)) {
                    advances++;
                    advanceTime(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
            }
            if (sends != CASE_SENDS[index] || advances != CASE_ADVANCES[index]) {
                throw new IllegalStateException("expected " + CASE_SENDS[index] + " sends and "
                        + CASE_ADVANCES[index] + " advance-time steps for " + CASES[index] + ", got " + sends
                        + " and " + advances);
            }
            if (writer.sequence != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " listener records for "
                        + CASES[index] + ", got " + writer.sequence);
            }
            if (index == 0) {
                observations.add("old-row-summary case=" + CASES[0] + " callbacks=" + writer.sequence
                        + " oldRows=" + writer.oldRows);
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
        if (index == 0) {
            return EPL_SIMPLE;
        }
        return index == 1 ? EPL_METHOD : EPL_NODEFAULT;
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
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
        return bean;
    }

    private static void advanceTime(EPRuntime runtime, JsonObject step) {
        requireFields(step, "op", "at");
        Instant instant = Instant.parse(string(step, "at"));
        runtime.getEventService().advanceTime(instant.toEpochMilli());
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
            throw new IllegalArgumentException("expected three cases");
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
            throw new IllegalArgumentException("scenario must contain exactly twenty-six steps");
        }
        int sendIndex = 0;
        int advanceIndex = 0;
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
            } else if ("advance-time".equals(operation)) {
                requireFields(step, "op", "at");
                if (advanceIndex >= ADVANCE_ATS.length || !ADVANCE_ATS[advanceIndex].equals(string(step, "at"))) {
                    throw new IllegalArgumentException("advance-time step is not pinned at step " + index);
                }
                advanceIndex++;
            } else {
                validateSend(step, index, SEND_STRINGS[sendIndex], SEND_INTS[sendIndex], SEND_LONGS[sendIndex]);
                sendIndex++;
            }
        }
        if (markerIndex != CASE_MARKERS.length || sendIndex != SEND_STRINGS.length
                || advanceIndex != ADVANCE_ATS.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, String expectedString,
                                     int expectedInt, long expectedLong) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
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
        private final List<String> observations;
        private final int caseIndex;
        private int sequence;
        private int oldRows;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, SupportBean[] sent,
                               List<String> observations, int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.sent = sent;
            this.observations = observations;
            this.caseIndex = caseIndex;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > CASE_RECORDS[caseIndex]) {
                throw new IllegalStateException("unexpected listener callback after " + CASE_RECORDS[caseIndex]
                        + " records for " + CASES[caseIndex]);
            }
            try {
                if (newEvents == null || newEvents.length == 0) {
                    throw new IllegalStateException("listener callback must carry new rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                EventBean[] ordered = newEvents;
                if (caseIndex == 0) {
                    if (newEvents.length != 1 || now != 0L) {
                        throw new IllegalStateException("grouped-simple callback " + next
                                + " is not a single new row at time zero");
                    }
                    assertSimpleRow(newEvents[0], next);
                    noteOldRows(next, oldEvents);
                } else {
                    long expectedTime = next == 1 ? 10000L : 20000L;
                    if (newEvents.length != 3 || now != expectedTime) {
                        throw new IllegalStateException(CASES[caseIndex] + " callback " + next
                                + " is not a three-row batch at " + expectedTime);
                    }
                    if (oldEvents != null && oldEvents.length != 0) {
                        throw new IllegalStateException(CASES[caseIndex] + " callback " + next
                                + " delivered " + oldEvents.length + " old rows");
                    }
                    assertSnapshotBatch(newEvents, next);
                    ordered = canonical(newEvents);
                }
                sequence = next;
                records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(ordered)));
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        /**
         * Case zero only: the Java execution pins new-only through
         * assertPropsNew, so the observation line is reported outside the
         * trace together with the send that produced it, and any old row is
         * rejected exactly like the ords 11/14 batches.
         */
        private void noteOldRows(int send, EventBean[] oldEvents) {
            int count = oldEvents == null ? 0 : oldEvents.length;
            oldRows += count;
            JsonArray oldRowRecords = count == 0 ? null : rows(oldEvents);
            StringBuilder line = new StringBuilder("old-row-observation case=").append(CASES[caseIndex])
                    .append(" send=").append(send).append(" oldRows=").append(count);
            if (count != 0) {
                line.append(" rows=").append(oldRowRecords);
            }
            observations.add(line.toString());
            if (count != 0) {
                throw new IllegalStateException(CASES[caseIndex] + " send " + send + " delivered " + count
                        + " old rows: " + oldRowRecords);
            }
        }

        private void assertSimpleRow(EventBean row, int sequence) {
            assertResultFields(row, SIMPLE_FIELDS);
            long[] scalars = SIMPLE_SCALARS[sequence - 1];
            for (int index = 0; index < SIMPLE_SCALAR_FIELDS.length; index++) {
                assertLong(row, SIMPLE_SCALAR_FIELDS[index], scalars[index]);
            }
            int[][] windows = SIMPLE_WINDOWS[sequence - 1];
            assertWindow(row, "c2", windows[0]);
            assertWindow(row, "c5", windows[1]);
            assertWindow(row, "c8", windows[2]);
        }

        private void assertSnapshotBatch(EventBean[] batch, int snapshot) {
            String[] fields = caseIndex == 1 ? METHOD_FIELDS : NODEFAULT_FIELDS;
            String[] metadata = caseIndex == 1 ? SORTED_METHOD_FIELDS : SORTED_NODEFAULT_FIELDS;
            Object[][] expected = caseIndex == 1 ? METHOD_ROWS[snapshot - 1] : NODEFAULT_ROWS[snapshot - 1];
            if (batch.length != expected.length) {
                throw new IllegalStateException(CASES[caseIndex] + " snapshot " + snapshot + " has "
                        + batch.length + " rows, want " + expected.length);
            }
            for (EventBean row : batch) {
                assertResultFields(row, metadata);
            }
            // Java's assertPropsPerRowLastNewAnyOrder matches the batch in any
            // order; the batch is compared as a multiset and the trace keeps
            // the canonical order.
            boolean[] matched = new boolean[batch.length];
            for (Object[] want : expected) {
                boolean found = false;
                for (int index = 0; index < batch.length; index++) {
                    if (!matched[index] && matchesSnapshotRow(batch[index], fields, want)) {
                        matched[index] = true;
                        found = true;
                        break;
                    }
                }
                if (!found) {
                    throw new IllegalStateException(CASES[caseIndex] + " snapshot " + snapshot
                            + " has no row " + Arrays.deepToString(want));
                }
            }
        }

        private static boolean matchesSnapshotRow(EventBean row, String[] fields, Object[] expected) {
            for (int index = 0; index < fields.length; index++) {
                Object value = row.get(fields[index]);
                Object want = expected[index];
                if (want instanceof String) {
                    if (!want.equals(value)) {
                        return false;
                    }
                } else if (want instanceof Integer) {
                    if (!(value instanceof Integer) || !want.equals(value)) {
                        return false;
                    }
                } else if (!(value instanceof Long) || !want.equals(value)) {
                    return false;
                }
            }
            return true;
        }

        private static EventBean[] canonical(EventBean[] batch) {
            if (batch.length < 2) {
                return batch.clone();
            }
            EventBean[] ordered = batch.clone();
            Arrays.sort(ordered, (left, right) -> {
                int byString = ((String) left.get("theString")).compareTo((String) right.get("theString"));
                return byString != 0 ? byString
                        : Integer.compare((Integer) left.get("intPrimitive"), (Integer) right.get("intPrimitive"));
            });
            return ordered;
        }

        private static void assertResultFields(EventBean row, String[] expected) {
            String[] names = row.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, expected)) {
                throw new IllegalStateException("result field metadata is not pinned: " + Arrays.toString(names));
            }
        }

        private static void assertLong(EventBean row, String name, long expected) {
            Object value = row.get(name);
            if (!(value instanceof Long) || ((Long) value).longValue() != expected) {
                throw new IllegalStateException("grouped row field " + name + " = " + value + ", want " + expected);
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
