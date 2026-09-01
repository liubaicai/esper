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

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregationMethodSorted ordinals 5-6.
 */
public final class ResultSetAggregationMethodSortedFirstLastScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-first-last";
    private static final String DESCRIPTION =
            "ResultSetAggregationMethodSorted ordinals 5-6: table-backed sorted first/last event, bucket, and key access with enumeration projections.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java";
    private static final String[] RUNTIME_IDS = {
            "java-runtime-2ec322fbd681b83590f1",
            "java-runtime-b7e36a11fb9b9c982249",
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateSortedFirstLast",
            "ResultSetAggregateSortedFirstLastEnumerationAndDot",
    };
    private static final String[] STATIC_IDS = {
            "java-0ded2b32c81677a9d36f",
            "java-5eb997e432653280b46b",
    };
    private static final String[] CASES = {"first-last", "first-last-dot"};
    private static final int[] ORDINALS = {5, 6};
    private static final String[] EPLS = {
            "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
                    + "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
                    + "@name('s0') select MyTable.sortcol.firstEvent() as fe,MyTable.sortcol.minBy() as minb,"
                    + "MyTable.sortcol.firstEvents() as fes,MyTable.sortcol.firstKey() as fk,"
                    + "MyTable.sortcol.lastEvent() as le,MyTable.sortcol.maxBy() as maxb,"
                    + "MyTable.sortcol.lastEvents() as les,MyTable.sortcol.lastKey() as lk from SupportBean_S0",
            "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
                    + "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
                    + "@name('s0') select MyTable.sortcol.firstEvent().theString as feid,"
                    + "MyTable.sortcol.firstEvent().firstOf() as fefo,"
                    + "MyTable.sortcol.firstEvents().lastOf() as feslo,"
                    + "MyTable.sortcol.lastEvent().theString() as leid,"
                    + "MyTable.sortcol.lastEvent().firstOf() as lefo,"
                    + "MyTable.sortcol.lastEvents().lastOf as leslo from SupportBean_S0",
    };
    private static final String[][][] SEEDS = {
            {{"E1a", "1"}, {"E1b", "1"}, {"E4b", "4"}, {"E6a", "6"}, {"E6b", "6"}, {"E8", "8"}, {"E9", "9"}},
            {{"E1a", "1"}, {"E1b", "1"}, {"E4b", "4"}, {"E6a", "6"}, {"E6b", "6"}, {"E8", "8"}, {"E9", "9"}},
    };
    private static final int STEP_COUNT = 18;
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetAggregationMethodSortedFirstLastScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregationMethodSortedFirstLastScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            replayCase(scenario, caseIndex, records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void replayCase(JsonObject scenario, int caseIndex, JsonArray records) throws Exception {
        String runtimeURI = "parity-" + ID + "-" + CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPLS[caseIndex],
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + CASES[caseIndex]));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime, caseIndex);
            statement.addListener(writer);
            replaySteps(scenario.get("steps").asArray(), caseIndex, runtime);
            if (writer.sequence != 1) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " expected one listener record, got " + writer.sequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
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

    private static void replaySteps(JsonArray steps, int caseIndex, EPRuntime runtime) {
        int base = caseIndex * 9;
        for (int index = base + 1; index <= base + 8; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String eventType = string(step, "eventType");
            JsonObject payload = object(step.get("payload"), "step " + index + " payload");
            if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive")),
                        "SupportBean");
            } else if ("SupportBean_S0".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(integer(payload, "id")),
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
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata " + index + " is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_COUNT) {
            throw new IllegalArgumentException("scenario must contain exactly " + STEP_COUNT + " steps");
        }
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            int base = caseIndex * 9;
            JsonObject marker = object(steps.get(base), "case marker " + caseIndex);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + caseIndex + " is not pinned");
            }
            for (int seedIndex = 0; seedIndex < 7; seedIndex++) {
                validateBean(steps.get(base + 1 + seedIndex),
                        SEEDS[caseIndex][seedIndex][0], Integer.parseInt(SEEDS[caseIndex][seedIndex][1]));
            }
            validateTrigger(steps.get(base + 8));
        }
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

    private static void validateTrigger(JsonValue value) {
        JsonObject step = object(value, "SupportBean_S0 trigger");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 trigger is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (integer(payload, "id") != -1) {
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
            if (next > 1) {
                throw new IllegalStateException("unexpected listener callback after one record");
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
            if (caseIndex == 0) {
                if (!Arrays.equals(names, new String[]{"fe", "fes", "fk", "le", "les", "lk", "maxb", "minb"})) {
                    throw new IllegalStateException("first-last result field metadata is not pinned");
                }
                assertEvent(event.get("fe"), "E1a", 1, "fe");
                assertEvent(event.get("minb"), "E1a", 1, "minb");
                assertEventArray(event.get("fes"), new String[]{"E1a", "E1b"}, new int[]{1, 1}, "fes");
                assertKey(event.get("fk"), 1, "fk");
                assertEvent(event.get("le"), "E9", 9, "le");
                assertEvent(event.get("maxb"), "E9", 9, "maxb");
                assertEventArray(event.get("les"), new String[]{"E9"}, new int[]{9}, "les");
                assertKey(event.get("lk"), 9, "lk");
            } else {
                if (!Arrays.equals(names, new String[]{"fefo", "feid", "feslo", "lefo", "leid", "leslo"})) {
                    throw new IllegalStateException("first-last-dot result field metadata is not pinned");
                }
                if (!"E1a".equals(event.get("feid")) || !"E9".equals(event.get("leid"))) {
                    throw new IllegalStateException("dot event strings are not pinned");
                }
                assertEvent(event.get("fefo"), "E1a", 1, "fefo");
                assertEvent(event.get("feslo"), "E1b", 1, "feslo");
                assertEvent(event.get("lefo"), "E9", 9, "lefo");
                assertEvent(event.get("leslo"), "E9", 9, "leslo");
            }

            JsonObject record = new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", next)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", rows(newEvents));
            sequence = next;
            records.add(record);
        }

        private void assertEvent(Object actual, String expectedString, int expectedInt, String name) {
            if (!(actual instanceof SupportBean)) {
                throw new IllegalStateException(name + " must be a SupportBean, got "
                        + (actual == null ? "null" : actual.getClass().getName()));
            }
            SupportBean bean = (SupportBean) actual;
            if (!expectedString.equals(bean.getTheString()) || bean.getIntPrimitive() != expectedInt) {
                throw new IllegalStateException(name + " must be " + expectedString + "/" + expectedInt
                        + ", got " + bean.getTheString() + "/" + bean.getIntPrimitive());
            }
        }

        private void assertEventArray(Object actual, String[] expectedStrings, int[] expectedInts, String name) {
            if (!(actual instanceof SupportBean[]) || ((SupportBean[]) actual).length != expectedStrings.length) {
                throw new IllegalStateException(name + " must be a SupportBean array of length "
                        + expectedStrings.length);
            }
            SupportBean[] beans = (SupportBean[]) actual;
            for (int index = 0; index < beans.length; index++) {
                if (!expectedStrings[index].equals(beans[index].getTheString())
                        || beans[index].getIntPrimitive() != expectedInts[index]) {
                    throw new IllegalStateException(name + " element " + index + " is not pinned");
                }
            }
        }

        private void assertKey(Object actual, int expected, String name) {
            if (!(actual instanceof Integer) || ((Integer) actual).intValue() != expected) {
                throw new IllegalStateException(name + " must be " + expected + ", got " + actual);
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
