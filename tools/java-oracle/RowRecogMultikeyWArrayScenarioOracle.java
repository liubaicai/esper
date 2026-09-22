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
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the two RowRecogMultikeyWArray executions
 * (match_recognize multi-key partition family) replayed as one differential
 * chain of two cases:
 *
 * partition-multikey-warray (ord 0, RowRecogPartitionMultikeyWArray):
 * partition by a primitive int[] property so Java selects MultiKeyArrayInt
 * with deep content equality (Arrays.equals/hashCode). Distinct array
 * instances with equal content share one partition: E10 [1,2] completes
 * E1's match, E11 [] completes E4's and E12 [1] completes E2's. Null
 * arrays share a single partition distinct from empty, so E13 null
 * completes E3's match.
 *
 * partition-multikey-plain (ord 1, RowRecogPartitionMultikeyPlain):
 * partition by intPrimitive,longPrimitive as a generated MultiKey over
 * boxed components; E10 completes E3's match, E11 completes E2's and E12
 * completes E1's.
 *
 * Mirroring SupportEvalRunner, each case deploys "@name('s0') <case EPL>"
 * once, sends every assertion event, then undeploys; the Java milestone(0)
 * is a harness no-op and there are no timers or iterator assertions. The
 * pinned case EPL is the contract text verbatim. SupportEventWithIntArray
 * (id string, array int[], value int) and SupportBean (theString string,
 * intPrimitive int, longPrimitive long, doublePrimitive double) are
 * declared as map event types because the regression-lib jar is not on
 * the oracle classpath; array is pinned to primitive int[] so Java
 * selects MultiKeyArrayInt. The TraceWriter skips null/null listener
 * callbacks.
 */
public final class RowRecogMultikeyWArrayScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-multikey-warray";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogMultikeyWArray.java";

    private static final String DESCRIPTION =
            "RowRecogMultikeyWArray ordinals 0-1 (two executions, the "
                    + "match_recognize multi-key partition family): "
                    + "partition-multikey-warray partitions by a primitive "
                    + "int[] property so Java selects MultiKeyArrayInt with "
                    + "deep content equality - distinct array instances "
                    + "with equal content share one partition, null arrays "
                    + "share a single partition and null is distinct from "
                    + "empty; partition-multikey-plain partitions by "
                    + "intPrimitive,longPrimitive as a generated multi-key "
                    + "over boxed components. Each case deploys s0 with the "
                    + "verbatim Java EPL, sends the assertion events and "
                    + "undeploys; there are no timers and no iterator "
                    + "assertions.";

    private static final String[] CASES = {
            "partition-multikey-warray", "partition-multikey-plain"};
    private static final int[] ORDINALS = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7a2e1b6edbc6c814817e",
            "java-runtime-11baac617fb4b4f194c6"};
    private static final String[] EXECUTIONS = {
            "RowRecogPartitionMultikeyWArray",
            "RowRecogPartitionMultikeyPlain"};
    private static final String[] STATIC_IDS = {
            "java-fa41ea09e7c778bf34af",
            "java-cec9c8f3503e647e5830"};
    private static final String[] OBSERVATIONS = {
            "listener; partition by array selects MultiKeyArrayInt with deep "
                    + "content equality; null arrays share one partition "
                    + "distinct from empty; emits {E1,E10}, {E4,E11}, "
                    + "{E2,E12} and {E3,E13}",
            "listener; partition by intPrimitive,longPrimitive over a "
                    + "generated multi-key of boxed components; emits "
                    + "{E3,E10}, {E2,E11} and {E1,E12}"
    };

    private static final String EPL_WARRAY =
            "@name('s0') select * from SupportEventWithIntArray "
                    + "match_recognize ("
                    + " partition by array"
                    + " measures A.id as a, B.id as b"
                    + " pattern (A B)"
                    + " define"
                    + " A as A.value = 1,"
                    + " B as B.value = 2"
                    + ")";
    private static final String EPL_PLAIN =
            "@name('s0') select * from SupportBean "
                    + "match_recognize ("
                    + " partition by intPrimitive, longPrimitive"
                    + " measures A.theString as a, B.theString as b"
                    + " pattern (A B)"
                    + " define"
                    + " A as A.doublePrimitive = 1,"
                    + " B as B.doublePrimitive = 2"
                    + ")";

    // The case-metadata EPL pins the single deployment of each case.
    private static final String[] CASE_EPLS = {
            EPL_WARRAY,
            EPL_PLAIN
    };

    // Pinned deploy EPLs per case, in deploy order.
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_WARRAY},
            {EPL_PLAIN}
    };

    // Pinned op sequences per case (after the case marker).
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "send", "send", "send",
                    "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "eventType|field|field|..." with <null> for a JSON-null array and
    // [n,n] for array contents. SupportEventWithIntArray sends encode
    // id|array|value and SupportBean sends encode
    // theString|intPrimitive|longPrimitive|doublePrimitive.
    private static final String[][] CASE_SENDS = {
            {"SupportEventWithIntArray|E1|[1,2]|1",
                    "SupportEventWithIntArray|E2|[1]|1",
                    "SupportEventWithIntArray|E3|<null>|1",
                    "SupportEventWithIntArray|E4|[]|1",
                    "SupportEventWithIntArray|E10|[1,2]|2",
                    "SupportEventWithIntArray|E11|[]|2",
                    "SupportEventWithIntArray|E12|[1]|2",
                    "SupportEventWithIntArray|E13|<null>|2"},
            {"SupportBean|E1|1|2|1", "SupportBean|E2|1|3|1",
                    "SupportBean|E3|2|2|1", "SupportBean|E10|2|2|2",
                    "SupportBean|E11|1|3|2", "SupportBean|E12|1|2|2"}
    };

    private static final int EXPECTED_STEPS = 20;
    private static final int EXPECTED_RECORDS = 7;

    private RowRecogMultikeyWArrayScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogMultikeyWArrayScenarioOracle <scenario.json>");
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
        Map<String, Object> arrayType = new HashMap<>();
        arrayType.put("id", String.class);
        arrayType.put("array", int[].class);
        arrayType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportEventWithIntArray", arrayType);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("longPrimitive", Long.class);
        beanType.put("doublePrimitive", Double.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

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
        if ("SupportEventWithIntArray".equals(eventType)) {
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("id", payload.getString("id", null));
            JsonValue array = payload.get("array");
            if (array == null || array.isNull()) {
                event.put("array", null);
            } else {
                JsonArray items = array.asArray();
                int[] values = new int[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    values[index] = (int) longNumber(items.get(index));
                }
                event.put("array", values);
            }
            event.put("value", payload.get("value").asInt());
            runtime.getEventService().sendEventMap(event, "SupportEventWithIntArray");
        } else if ("SupportBean".equals(eventType)) {
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            event.put("longPrimitive", payload.get("longPrimitive").asLong());
            event.put("doublePrimitive", payload.get("doublePrimitive").asDouble());
            runtime.getEventService().sendEventMap(event, "SupportBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
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
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
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
     * sends with pinned payloads and undeploy-all. Unknown step fields are
     * rejected.
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
            int deploys = 0;
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
                                || !CASE_DEPLOY_EPLS[caseIndex][deploys++].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String eventType = string(step, "eventType");
                        String actual;
                        if ("SupportEventWithIntArray".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "id", "array", "value");
                            actual = eventType + "|" + string(payload, "id") + "|"
                                    + arrayEncoding(payload.get("array")) + "|"
                                    + integer(payload, "value");
                        } else if ("SupportBean".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "theString", "intPrimitive",
                                    "longPrimitive", "doublePrimitive");
                            actual = eventType + "|" + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive") + "|"
                                    + longNumber(payload.get("longPrimitive")) + "|"
                                    + longNumber(payload.get("doublePrimitive"));
                        } else {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
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
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != CASE_DEPLOY_EPLS[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " step counts are not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String arrayEncoding(JsonValue value) {
        if (value == null || value.isNull()) {
            return "<null>";
        }
        JsonArray items = value.asArray();
        StringBuilder encoded = new StringBuilder("[");
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                encoded.append(',');
            }
            encoded.append(longNumber(items.get(index)));
        }
        return encoded.append(']').toString();
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
        long value = longNumber(object.get(name));
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonValue value) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException("value is outside the Java long range", ex);
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
