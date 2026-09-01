import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/** Direct Esper 9.0.0 oracle for ResultSetAggregationMethodSorted ordinal 11. */
public final class ResultSetAggregationMethodSortedGroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-grouped";
    private static final String DESCRIPTION =
            "ResultSetAggregationMethodSorted ordinal 11: grouped table-backed sorted collection selector with first/last key access.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java";
    private static final String RUNTIME_ID = "java-runtime-3ffe177eb6a90acb699a";
    private static final String EXECUTION_NAME = "ResultSetAggregateSortedGrouped";
    private static final String STATIC_ID = "java-5a1abe9fb7420df7353d";
    private static final String INVENTORY_ID = "java-0ded2b32c81677a9d36f";
    private static final String CASE = "grouped";
    private static final String RUNTIME_URI = "parity-resultset-aggregate-sorted-grouped";
    private static final String EPL =
            "create table MyTable(k0 string primary key, sortcol sorted(intPrimitive) @type('SupportBean'));\n" +
            "into table MyTable select sorted(*) as sortcol from SupportBean group by theString;\n" +
            "@name('s0') select MyTable[p00].sortcol.sorted() as sortcol,MyTable[p00].sortcol.firstKey() as firstkey,MyTable[p00].sortcol.lastKey() as lastkey from SupportBean_S0";
    private static final int STEP_COUNT = 11;
    private static final int RECORD_COUNT = 5;
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");
    private static final Integer[] EXPECTED_FIRST = {null, 10, 10, 10, 100};
    private static final Integer[] EXPECTED_LAST = {null, 20, 21, 21, 100};

    private ResultSetAggregationMethodSortedGroupedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregationMethodSortedGroupedScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S0.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(RUNTIME_URI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        JsonArray records = new JsonArray();
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime);
            statement.addListener(writer);
            replay(scenario.get("steps").asArray(), runtime);
            if (writer.sequence != RECORD_COUNT) {
                throw new IllegalStateException("expected " + RECORD_COUNT
                        + " listener records, got " + writer.sequence);
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
        EPStatement result = null;
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                if (result != null) {
                    throw new IllegalStateException("deployment contains multiple statements named s0");
                }
                result = statement;
            }
        }
        if (result == null) {
            throw new IllegalStateException("statement s0 was not deployed");
        }
        return result;
    }

    private static void replay(JsonArray steps, EPRuntime runtime) {
        for (int index = 1; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String eventType = string(step, "eventType");
            JsonObject payload = object(step.get("payload"), "step " + index + " payload");
            if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive")),
                        "SupportBean");
            } else if ("SupportBean_S0".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(integer(payload, "id"), string(payload, "p00")),
                        "SupportBean_S0");
            } else {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
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
                || integer(definition, "ordinal") != 11
                || !RUNTIME_ID.equals(string(definition, "runtimeId"))
                || !EXECUTION_NAME.equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_COUNT) {
            throw new IllegalArgumentException("scenario must contain exactly " + STEP_COUNT + " steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }

        validateTrigger(steps.get(1), "A");
        validateBean(steps.get(2), "A", 10);
        validateBean(steps.get(3), "A", 20);
        validateTrigger(steps.get(4), "A");
        validateBean(steps.get(5), "A", 10);
        validateBean(steps.get(6), "A", 21);
        validateTrigger(steps.get(7), "A");
        validateBean(steps.get(8), "B", 100);
        validateTrigger(steps.get(9), "A");
        validateTrigger(steps.get(10), "B");
    }

    private static void validateBean(JsonValue value, String expectedString, int expectedInt) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateTrigger(JsonValue value, String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 trigger");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 trigger is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (integer(payload, "id") != -1 || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned");
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
            JsonArray array = value.asArray();
            for (int index = 0; index < array.size(); index++) {
                rejectDuplicateKeys(array.get(index));
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
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        if (!INTEGER_SYNTAX.matcher(text).matches()) {
            throw new IllegalArgumentException(name + " must use integer JSON syntax");
        }
        try {
            return Integer.parseInt(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java int range", ex);
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

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > RECORD_COUNT) {
                throw new IllegalStateException("unexpected listener callback after five records");
            }
            if (newEvents == null || newEvents.length != 1
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("listener callback must contain one new row only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time " + now);
            }
            EventBean event = newEvents[0];
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, new String[]{"firstkey", "lastkey", "sortcol"})) {
                throw new IllegalStateException("result field metadata is not pinned");
            }
            assertKey(event.get("firstkey"), EXPECTED_FIRST[next - 1], "firstkey", next);
            assertKey(event.get("lastkey"), EXPECTED_LAST[next - 1], "lastkey", next);

            JsonObject record = new JsonObject().add("case", CASE).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", next)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", rows(newEvents));
            sequence = next;
            records.add(record);
        }

        private void assertKey(Object actual, Integer expected, String name, int recordNumber) {
            if (expected == null) {
                if (actual != null) {
                    throw new IllegalStateException(name + " must be null in record " + recordNumber);
                }
                return;
            }
            if (!(actual instanceof Number)
                    || ((Number) actual).doubleValue() != expected.doubleValue()) {
                throw new IllegalStateException(name + " must be " + expected + " in record " + recordNumber
                        + ", got " + actual);
            }
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                output.add(eventValue(event));
            }
            return output;
        }

        private JsonObject eventValue(EventBean event) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean) {
                return normalize(((EventBean) value).getUnderlying());
            }
            if (value instanceof SupportBean) {
                SupportBean bean = (SupportBean) value;
                return new JsonObject().add("kind", "row").add("fields",
                        new JsonObject().add("intPrimitive", normalize(bean.getIntPrimitive()))
                                .add("theString", normalize(bean.getTheString())));
            }
            if (value instanceof Map) {
                return mapRow((Map<?, ?>) value);
            }
            if (value instanceof Collection) {
                JsonArray array = new JsonArray();
                for (Object item : (Collection<?>) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Object[] || value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = Array.getLength(value);
                for (int index = 0; index < length; index++) {
                    array.add(normalize(Array.get(value, index)));
                }
                return array;
            }
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
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

        private JsonValue mapRow(Map<?, ?> map) {
            List<Map.Entry<?, ?>> entries = new ArrayList<>(map.entrySet());
            entries.sort(Comparator.comparing(entry -> String.valueOf(entry.getKey())));
            JsonObject fields = new JsonObject();
            for (Map.Entry<?, ?> entry : entries) {
                fields.add(String.valueOf(entry.getKey()), normalize(entry.getValue()));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
    }
}
