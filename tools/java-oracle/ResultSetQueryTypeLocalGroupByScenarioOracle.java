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
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinal 13,
 * ResultSetLocalGroupedMultiLevelAccess.  The listener assertion deliberately
 * checks the retained EventBean arrays against the exact SupportBean objects
 * sent by the replay, not merely their equal property values.
 */
public final class ResultSetQueryTypeLocalGroupByScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-by";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinal 13: grouped multi-level window access with local group_by dimensions.";
    private static final String RUNTIME_ID = "java-runtime-87b3bcfdf164bc939fe5";
    private static final String EXECUTION_NAME = "ResultSetLocalGroupedMultiLevelAccess";
    private static final String STATIC_ID = "java-b0b3444c9ac5a1297cd0";
    private static final String CASE = "multi-level-access";
    private static final String EPL =
            "@name('s0') select   theString, intPrimitive,   window(*, group_by:(intPrimitive, theString)) as c0,   window(*) as c1,   window(*, group_by:theString) as c2,   window(*, group_by:intPrimitive) as c3,   window(*, group_by:()) as c4 from SupportBean#keepall group by theString, intPrimitive output snapshot every 10 seconds order by theString, intPrimitive";
    private static final String[] TOP_FIELDS = {"c0", "c1", "c2", "c3", "c4", "intPrimitive", "theString"};
    private static final String[] BEAN_FIELDS = {
            "bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive",
            "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed",
            "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed",
            "shortPrimitive", "theString"
    };

    private ResultSetQueryTypeLocalGroupByScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupByScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        String runtimeURI = "parity-" + ID + "-" + RUNTIME_ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0L);
        JsonArray records = new JsonArray();
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            SupportBean[] sent = new SupportBean[5];
            TraceWriter writer = new TraceWriter(records, statement, runtime, sent);
            statement.addListener(writer);
            runtime.getEventService().advanceTime(0L);
            replay(scenario.get("steps").asArray(), runtime, sent);
            if (writer.sequence != 1) {
                throw new IllegalStateException("expected exactly one listener record, got " + writer.sequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static void replay(JsonArray steps, EPRuntime runtime, SupportBean[] sent) {
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                continue;
            }
            if ("advance-time".equals(operation)) {
                requireFields(step, "op", "at");
                String at = string(step, "at");
                if ("1970-01-01T00:00:00Z".equals(at)) {
                    runtime.getEventService().advanceTime(0L);
                } else if ("1970-01-01T00:00:10Z".equals(at)) {
                    runtime.getEventService().advanceTime(10000L);
                } else {
                    throw new IllegalArgumentException("unexpected timer step at " + at);
                }
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation at step " + index);
            }
            requireFields(step, "op", "eventType", "payload");
            if (!"SupportBean".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("unexpected event type at step " + index);
            }
            JsonObject payload = object(step.get("payload"), "payload step " + index);
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
            bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
            int eventIndex = index - 2;
            if (eventIndex < 0 || eventIndex >= sent.length) {
                throw new IllegalArgumentException("unexpected SupportBean send at step " + index);
            }
            sent[eventIndex] = bean;
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        }
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME_ID}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{EXECUTION_NAME}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(string(definition, "case"))
                || integer(definition, "ordinal") != 13
                || !RUNTIME_ID.equals(string(definition, "runtimeId"))
                || !EXECUTION_NAME.equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 8) {
            throw new IllegalArgumentException("scenario must contain exactly eight steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }
        JsonObject initialTime = object(steps.get(1), "initial time");
        requireFields(initialTime, "op", "at");
        if (!"advance-time".equals(string(initialTime, "op"))
                || !"1970-01-01T00:00:00Z".equals(string(initialTime, "at"))) {
            throw new IllegalArgumentException("initial timer step is not pinned");
        }
        String[] strings = {"E1", "E1", "E2", "E1", "E2"};
        int[] ints = {10, 20, 10, 10, 10};
        long[] longs = {100L, 202L, 303L, 404L, 505L};
        for (int index = 0; index < strings.length; index++) {
            validateSend(steps.get(index + 2), "send step " + (index + 2), strings[index], ints[index], longs[index]);
        }
        JsonObject finalTime = object(steps.get(7), "final time");
        requireFields(finalTime, "op", "at");
        if (!"advance-time".equals(string(finalTime, "op"))
                || !"1970-01-01T00:00:10Z".equals(string(finalTime, "at"))) {
            throw new IllegalArgumentException("final timer step is not pinned");
        }
    }

    private static void validateSend(JsonValue value, String label, String expectedString,
                                     int expectedInt, long expectedLong) {
        JsonObject step = object(value, label);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(label + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), label + " payload");
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException(label + " payload is not pinned");
        }
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final SupportBean[] sent;
        private int sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime, SupportBean[] sent) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.sent = sent;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next != 1) {
                throw new IllegalStateException("unexpected listener callback after one record");
            }
            if (newEvents == null || newEvents.length != 3
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("listener callback must contain three new rows only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 10000L || !"1970-01-01T00:00:10Z".equals(Instant.ofEpochMilli(now).toString())) {
                throw new IllegalStateException("unexpected callback time");
            }
            try {
                assertRows(newEvents);
            } catch (RuntimeException ex) {
                ex.printStackTrace(System.err);
                throw ex;
            }
            sequence = next;
            records.add(new JsonObject().add("case", CASE).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents)));
        }

        private void assertRows(EventBean[] rows) {
            for (EventBean row : rows) {
                String[] names = row.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, TOP_FIELDS)) {
                    throw new IllegalStateException("result field metadata is not pinned: " + Arrays.toString(names));
                }
            }
            assertRow(rows[0], "E1", 10, new SupportBean[]{sent[0], sent[3]},
                    new SupportBean[]{sent[0], sent[3]}, new SupportBean[]{sent[0], sent[1], sent[3]},
                    new SupportBean[]{sent[0], sent[2], sent[3], sent[4]}, sent);
            assertRow(rows[1], "E1", 20, new SupportBean[]{sent[1]},
                    new SupportBean[]{sent[1]}, new SupportBean[]{sent[0], sent[1], sent[3]},
                    new SupportBean[]{sent[1]}, sent);
            assertRow(rows[2], "E2", 10, new SupportBean[]{sent[2], sent[4]},
                    new SupportBean[]{sent[2], sent[4]}, new SupportBean[]{sent[2], sent[4]},
                    new SupportBean[]{sent[0], sent[2], sent[3], sent[4]}, sent);
        }

        private void assertRow(EventBean row, String expectedString, int expectedInt,
                               SupportBean[] c0, SupportBean[] c1, SupportBean[] c2,
                               SupportBean[] c3, SupportBean[] c4) {
            if (!expectedString.equals(row.get("theString"))
                    || !(row.get("intPrimitive") instanceof Integer)
                    || ((Integer) row.get("intPrimitive")) != expectedInt) {
                throw new IllegalStateException("unexpected grouped row key");
            }
            assertWindow(row.get("c0"), "c0", c0);
            assertWindow(row.get("c1"), "c1", c1);
            assertWindow(row.get("c2"), "c2", c2);
            assertWindow(row.get("c3"), "c3", c3);
            assertWindow(row.get("c4"), "c4", c4);
        }

        private void assertWindow(Object value, String label, SupportBean[] expected) {
            Object[] actual;
            if (value instanceof EventBean[] events) {
                actual = events;
            } else if (value instanceof Object[] objects) {
                actual = objects;
            } else {
                throw new IllegalStateException(label + " must be an event array");
            }
            if (actual.length != expected.length) {
                throw new IllegalStateException(label + " length = " + actual.length + ", want " + expected.length);
            }
            for (int index = 0; index < expected.length; index++) {
                Object item = actual[index];
                if (item instanceof EventBean event) {
                    if (event.getUnderlying() != expected[index]) {
                        throw new IllegalStateException(label + " lost SupportBean identity/order at " + index);
                    }
                    assertSupportBeanEvent(event, expected[index]);
                } else if (item != expected[index]) {
                    throw new IllegalStateException(label + " lost SupportBean identity/order at " + index);
                }
            }
        }

        private void assertSupportBeanEvent(EventBean event, SupportBean expected) {
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, BEAN_FIELDS)) {
                throw new IllegalStateException("SupportBean field metadata is not pinned: " + Arrays.toString(names));
            }
            if (event.getUnderlying() != expected
                    || !expected.getTheString().equals(event.get("theString"))
                    || !Integer.valueOf(expected.getIntPrimitive()).equals(event.get("intPrimitive"))
                    || !Long.valueOf(expected.getLongPrimitive()).equals(event.get("longPrimitive"))
                    || !Boolean.FALSE.equals(event.get("boolPrimitive"))
                    || !Character.valueOf('\u0000').equals(event.get("charPrimitive"))
                    || !Short.valueOf((short) 0).equals(event.get("shortPrimitive"))
                    || !Byte.valueOf((byte) 0).equals(event.get("bytePrimitive"))
                    || !Float.valueOf(0.0f).equals(event.get("floatPrimitive"))
                    || !Double.valueOf(0.0d).equals(event.get("doublePrimitive"))
                    || event.get("boolBoxed") != null || event.get("intBoxed") != null
                    || event.get("longBoxed") != null || event.get("charBoxed") != null
                    || event.get("shortBoxed") != null || event.get("byteBoxed") != null
                    || event.get("floatBoxed") != null || event.get("doubleBoxed") != null
                    || event.get("bigDecimal") != null || event.get("bigInteger") != null
                    || event.get("enumValue") != null) {
                throw new IllegalStateException("SupportBean event fields are not pinned");
            }
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
    }
}
