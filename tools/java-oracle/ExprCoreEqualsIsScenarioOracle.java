import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithManyArray;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.util.Arrays;

/** Direct Esper 9.0.0 oracle for the replayable ExprCoreEqualsIs executions. */
public final class ExprCoreEqualsIsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-equals-is";
    private static final String[] CASES = {
            "equals-coercion", "equals-same-type", "equals-array", "equals-null"
    };
    private static final String[] EVENT_TYPES = {
            "SupportBean", "SupportBean_S0", "SupportEventWithManyArray", "SupportBean"
    };
    private static final int[] SEND_COUNTS = {2, 2, 2, 2};

    private ExprCoreEqualsIsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-equals-is.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!SCENARIO_ID.equals(scenarioID)) {
            throw new IllegalArgumentException("unsupported scenario id " + scenarioID);
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateScenario(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateScenario(JsonArray steps) {
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("scenario is missing case " + CASES[caseIndex]);
            }
            JsonObject marker = steps.get(offset++).asObject();
            if (!"case".equals(marker.getString("op", "")) ||
                    !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("scenario case order mismatch at " + caseIndex);
            }
            for (int sendIndex = 0; sendIndex < SEND_COUNTS[caseIndex]; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !EVENT_TYPES[caseIndex].equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
                JsonValue payloadValue = step.get("payload");
                if (payloadValue == null || !payloadValue.isObject()) {
                    throw new IllegalArgumentException("scenario payload is required for " + CASES[caseIndex]);
                }
                validatePayload(payloadValue.asObject(), caseIndex, sendIndex);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        switch (CASES[caseIndex]) {
            case "equals-coercion":
                requireFieldCount(payload, 2, "equals-coercion");
                requireNumber(payload, "intPrimitive", 1);
                requireNumber(payload, "longPrimitive", sendIndex == 0 ? 1 : 2);
                return;
            case "equals-same-type":
                requireFieldCount(payload, 4, "equals-same-type");
                requireNumber(payload, "id", 1);
                requireString(payload, "p00", "a");
                requireString(payload, "p01", sendIndex == 0 ? "a" : "b");
                requireString(payload, "p02", sendIndex == 0 ? "a" : null);
                return;
            case "equals-array":
                validateArrayPayload(payload, sendIndex);
                return;
            case "equals-null":
                requireFieldCount(payload, 1, "equals-null");
                requireString(payload, "theString", sendIndex == 0 ? "x" : null);
                return;
            default:
                throw new IllegalArgumentException("unsupported case " + CASES[caseIndex]);
        }
    }

    private static void validateArrayPayload(JsonObject payload, int sendIndex) {
        requireFieldCount(payload, 9, "equals-array");
        requireString(payload, "id", "E1");
        requireIntArray(payload, "intOne", new int[]{1, 2});
        requireIntArray(payload, "intTwo", sendIndex == 0 ? new int[]{1, 2} : new int[]{1});
        requireIntegerArray(payload, "intBoxedOne", new Integer[]{1, 2});
        requireIntegerArray(payload, "intBoxedTwo", sendIndex == 0 ? new Integer[]{1, 2} : new Integer[]{1});
        requireInt2DimArray(payload, "int2DimOne", new int[][]{{1, 2}, {3, 4}});
        requireInt2DimArray(payload, "int2DimTwo",
                sendIndex == 0 ? new int[][]{{1, 2}, {3, 4}} : new int[][]{{1, 2}, {3}});
        requireObjectArray(payload, "objectOne",
                sendIndex == 0 ? new Object[]{'a', new Object[]{1}} : new Object[]{'a', 2});
        requireObjectArray(payload, "objectTwo",
                sendIndex == 0 ? new Object[]{'a', new Object[]{1}} : new Object[]{'a'});
    }

    private static void requireFieldCount(JsonObject payload, int expected, String caseName) {
        if (payload.names().size() != expected) {
            throw new IllegalArgumentException("payload field count mismatch for " + caseName);
        }
    }

    private static void requireNumber(JsonObject payload, String name, long expected) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber() || value.asLong() != expected) {
            throw new IllegalArgumentException("payload number mismatch for " + name);
        }
    }

    private static void requireString(JsonObject payload, String name, String expected) {
        JsonValue value = payload.get(name);
        if (expected == null) {
            if (value == null || !value.isNull()) {
                throw new IllegalArgumentException("payload null mismatch for " + name);
            }
        } else if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("payload string mismatch for " + name);
        }
    }

    private static void requireIntArray(JsonObject payload, String name, int[] expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isArray() || !Arrays.equals(expected, toIntArray(value))) {
            throw new IllegalArgumentException("payload int-array mismatch for " + name);
        }
    }

    private static void requireIntegerArray(JsonObject payload, String name, Integer[] expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isArray() || !Arrays.equals(expected, toIntegerArray(value))) {
            throw new IllegalArgumentException("payload boxed-array mismatch for " + name);
        }
    }

    private static void requireInt2DimArray(JsonObject payload, String name, int[][] expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isArray() || !Arrays.deepEquals(expected, toInt2DimArray(value))) {
            throw new IllegalArgumentException("payload 2D-array mismatch for " + name);
        }
    }

    private static void requireObjectArray(JsonObject payload, String name, Object[] expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isArray() || !Arrays.deepEquals(expected, toObjectArray(value))) {
            throw new IllegalArgumentException("payload object-array mismatch for " + name);
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        configuration.getCommon().addEventType("SupportEventWithManyArray", SupportEventWithManyArray.class);
        String runtimeName = "parity-expr-core-equals-is-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeName, configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    "@name('s0') " + eplFor(caseName),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 was not deployed");
            }
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplFor(String caseName) {
        if ("equals-coercion".equals(caseName)) {
            return "select intPrimitive=longPrimitive as c0, intPrimitive is longPrimitive as c1 from SupportBean";
        }
        if ("equals-same-type".equals(caseName)) {
            return "select p00 = p01 as c0, id = id as c1, p02 is not null as c2 from SupportBean_S0";
        }
        if ("equals-array".equals(caseName)) {
            return "select intOne=intTwo as c0, intOne is intTwo as c1, " +
                    "intBoxedOne=intBoxedTwo as c2, intBoxedOne is intBoxedTwo as c3, " +
                    "int2DimOne=int2DimTwo as c4, int2DimOne is int2DimTwo as c5, " +
                    "objectOne=objectTwo as c6, objectOne is objectTwo as c7 " +
                    "from SupportEventWithManyArray";
        }
        if ("equals-null".equals(caseName)) {
            return "select theString = null as c0, theString is null as c1, " +
                    "null = theString as c2, null is theString as c3, " +
                    "null = null as c4, null is null as c5 from SupportBean";
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active || !"send".equals(op)) {
                continue;
            }
            String eventType = step.getString("eventType", "");
            JsonObject payload = step.get("payload").asObject();
            if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(toSupportBean(payload), eventType);
            } else if ("SupportBean_S0".equals(eventType)) {
                runtime.getEventService().sendEventBean(toSupportBeanS0(payload), eventType);
            } else if ("SupportEventWithManyArray".equals(eventType)) {
                runtime.getEventService().sendEventBean(toManyArray(payload), eventType);
            } else {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setTheString(nullableString(payload, "theString"));
        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
        bean.setLongPrimitive(payload.getLong("longPrimitive", 0L));
        return bean;
    }

    private static SupportBean_S0 toSupportBeanS0(JsonObject payload) {
        return new SupportBean_S0(
                payload.getInt("id", 0),
                nullableString(payload, "p00"),
                nullableString(payload, "p01"),
                nullableString(payload, "p02"));
    }

    private static SupportEventWithManyArray toManyArray(JsonObject payload) {
        return new SupportEventWithManyArray(nullableString(payload, "id"))
                .withIntOne(toIntArray(payload.get("intOne")))
                .withIntTwo(toIntArray(payload.get("intTwo")))
                .withIntBoxedOne(toIntegerArray(payload.get("intBoxedOne")))
                .withIntBoxedTwo(toIntegerArray(payload.get("intBoxedTwo")))
                .withInt2DimOne(toInt2DimArray(payload.get("int2DimOne")))
                .withInt2DimTwo(toInt2DimArray(payload.get("int2DimTwo")))
                .withObjectOne(toObjectArray(payload.get("objectOne")))
                .withObjectTwo(toObjectArray(payload.get("objectTwo")));
    }

    private static int[] toIntArray(JsonValue value) {
        JsonArray array = value.asArray();
        int[] result = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            result[i] = array.get(i).asInt();
        }
        return result;
    }

    private static Integer[] toIntegerArray(JsonValue value) {
        JsonArray array = value.asArray();
        Integer[] result = new Integer[array.size()];
        for (int i = 0; i < array.size(); i++) {
            JsonValue item = array.get(i);
            result[i] = item.isNull() ? null : item.asInt();
        }
        return result;
    }

    private static int[][] toInt2DimArray(JsonValue value) {
        JsonArray array = value.asArray();
        int[][] result = new int[array.size()][];
        for (int i = 0; i < array.size(); i++) {
            result[i] = toIntArray(array.get(i));
        }
        return result;
    }

    private static Object[] toObjectArray(JsonValue value) {
        JsonArray array = value.asArray();
        Object[] result = new Object[array.size()];
        for (int i = 0; i < array.size(); i++) {
            result[i] = toObjectValue(array.get(i));
        }
        return result;
    }

    private static Object toObjectValue(JsonValue value) {
        if (value.isNull()) {
            return null;
        }
        if (value.isArray()) {
            return toObjectArray(value);
        }
        if (value.isString()) {
            String string = value.asString();
            if (string.length() != 1) {
                throw new IllegalArgumentException("object-array strings must be one character");
            }
            return string.charAt(0);
        }
        if (value.isNumber()) {
            return value.asInt();
        }
        throw new IllegalArgumentException("unsupported object-array value " + value);
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
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
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
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
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Float || value instanceof Double) {
                return Json.value(((Number) value).doubleValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
