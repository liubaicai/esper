import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.HashableMultiKey;
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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for
 * ResultSetAggregationMethodSorted.ResultSetAggregateSortedMultiCriteria.
 *
 * The one fresh runtime mirrors the pinned EPL exactly: a public table stores
 * sorted SupportBean events under the two-field (theString, intPrimitive)
 * lexicographic key, and an on-event query reads the four key navigators. The
 * scenario intentionally contains only the seven population sends and the
 * final SupportBean_S0(-1) trigger from the fixed regression execution.
 */
public final class ResultSetAggregationMethodSortedMultiCriteriaScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-multi-criteria";
    private static final String CASE = "multi-criteria";
    private static final String EXECUTION = "ResultSetAggregateSortedMultiCriteria";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_RUNTIME_ID = "java-runtime-dd79ba5aba4eb4ec0a1a";
    private static final String JAVA_INVENTORY_ID = "java-0ded2b32c81677a9d36f";
    private static final String JAVA_STATIC_ID = "java-427c23ee540c6e2e7cf9";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java";
    private static final String RUNTIME_URI = "parity-resultset-aggregate-sorted-multi-criteria";
    private static final int SEND_COUNT = 7;
    private static final String[] SEND_STRINGS = {"E1a", "E1b", "E4b", "E6a", "E6b", "E8", "E9"};
    private static final int[] SEND_INTS = {1, 1, 4, 6, 6, 8, 9};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");
    private static final String EPL =
            "create table MyTable(sortcol sorted(theString, intPrimitive) @type('SupportBean'));\n" +
            "into table MyTable select sorted(*) as sortcol from SupportBean;\n" +
            "@name('s0') select " +
            "MyTable.sortcol.firstKey() as firstkey," +
            "MyTable.sortcol.lastKey() as lastkey," +
            "MyTable.sortcol.lowerKey(new HashableMultiKey('E4', 1)) as lowerkey," +
            "MyTable.sortcol.higherKey(new HashableMultiKey('E4b', -1)) as higherkey" +
            " from SupportBean_S0";

    private ResultSetAggregationMethodSortedMultiCriteriaScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregationMethodSortedMultiCriteriaScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be an object");
        }
        validateNoDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        requireFields(scenario, "version", "id", "steps");
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id");
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateShape(steps);

        JsonArray records = new JsonArray();
        runCase(steps, records);
        if (records.size() != 1) {
            throw new IllegalStateException("expected exactly one listener trace record, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    /** Reject duplicate object members before JsonObject's last-member lookup can hide them. */
    private static void validateNoDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> seen = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!seen.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                validateNoDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            JsonArray array = value.asArray();
            for (int index = 0; index < array.size(); index++) {
                validateNoDuplicateKeys(array.get(index));
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
        Set<String> expected = new HashSet<>(Arrays.asList(expectedNames));
        if (!expected.equals(new HashSet<>(object.names()))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject objectStep(JsonValue value, String description) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(description + " must be an object");
        }
        return value.asObject();
    }

    private static void validateShape(JsonArray steps) {
        if (steps.size() != SEND_COUNT + 2) {
            throw new IllegalArgumentException("scenario must contain one case, seven SupportBean sends, and one trigger");
        }

        JsonObject marker = objectStep(steps.get(0), "case step");
        requireFields(marker, "op", "case");
        if (!"case".equals(marker.getString("op", "")) || !CASE.equals(marker.getString("case", ""))) {
            throw new IllegalArgumentException("scenario must start with case " + CASE);
        }

        for (int index = 0; index < SEND_COUNT; index++) {
            JsonObject step = objectStep(steps.get(index + 1), "SupportBean step " + index);
            requireFields(step, "op", "eventType", "payload");
            if (!"send".equals(step.getString("op", ""))
                    || !"SupportBean".equals(step.getString("eventType", ""))) {
                throw new IllegalArgumentException("scenario must contain the seven SupportBean sends in order");
            }
            JsonObject payload = objectStep(step.get("payload"), "SupportBean payload " + index);
            requireFields(payload, "theString", "intPrimitive");
            JsonValue stringValue = payload.get("theString");
            if (stringValue == null || stringValue.isNull() || !stringValue.isString()
                    || !SEND_STRINGS[index].equals(stringValue.asString())) {
                throw new IllegalArgumentException("SupportBean theString does not match the frozen sequence");
            }
            int intValue = requireInteger(payload, "intPrimitive");
            if (intValue != SEND_INTS[index]) {
                throw new IllegalArgumentException("SupportBean intPrimitive does not match the frozen sequence");
            }
        }

        JsonObject trigger = objectStep(steps.get(SEND_COUNT + 1), "SupportBean_S0 trigger");
        requireFields(trigger, "op", "eventType", "payload");
        if (!"send".equals(trigger.getString("op", ""))
                || !"SupportBean_S0".equals(trigger.getString("eventType", ""))) {
            throw new IllegalArgumentException("scenario must end with a SupportBean_S0 trigger");
        }
        JsonObject triggerPayload = objectStep(trigger.get("payload"), "SupportBean_S0 payload");
        requireFields(triggerPayload, "id");
        if (requireInteger(triggerPayload, "id") != -1) {
            throw new IllegalArgumentException("SupportBean_S0 trigger id must be -1");
        }
    }

    private static int requireInteger(JsonObject object, String name) {
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

    private static void runCase(JsonArray steps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(HashableMultiKey.class);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(RUNTIME_URI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL, compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(RUNTIME_URI));
            EPStatement statement = findStatement(deployment);
            statement.addListener(new TraceWriter(records, statement, runtime));
            replay(steps, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replay(JsonArray steps, EPRuntime runtime) {
        for (int index = 0; index < SEND_COUNT; index++) {
            JsonObject payload = steps.get(index + 1).asObject().get("payload").asObject();
            SupportBean event = new SupportBean(payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventBean(event, "SupportBean");
        }
        JsonObject triggerPayload = steps.get(SEND_COUNT + 1).asObject().get("payload").asObject();
        runtime.getEventService().sendEventBean(
                new SupportBean_S0(triggerPayload.getInt("id", 0)), "SupportBean_S0");
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", CASE)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
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
            if (value instanceof HashableMultiKey) {
                return normalizeMultiKey((HashableMultiKey) value);
            }
            if (value instanceof EventBean) {
                return eventValue((EventBean) value);
            }
            if (value instanceof Map) {
                Map<?, ?> map = (Map<?, ?>) value;
                List<String> names = new ArrayList<>();
                for (Object key : map.keySet()) {
                    names.add(String.valueOf(key));
                }
                names.sort(String::compareTo);
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(map.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Collection) {
                JsonArray array = new JsonArray();
                for (Object item : (Collection<?>) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Object[]) {
                JsonArray array = new JsonArray();
                for (Object item : (Object[]) value) {
                    array.add(normalize(item));
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

        private JsonValue normalizeMultiKey(HashableMultiKey key) {
            Object[] keys = key.getKeys();
            if (keys.length != 2 || !(keys[0] instanceof String) || !(keys[1] instanceof Number)) {
                throw new IllegalStateException("sorted multi-criteria key is not a (String, int) pair");
            }
            JsonObject fields = new JsonObject()
                    .add("intPrimitive", normalize(keys[1]))
                    .add("theString", normalize(keys[0]));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
    }
}
