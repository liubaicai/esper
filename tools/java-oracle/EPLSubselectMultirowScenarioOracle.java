import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
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
import java.util.HashMap;
import java.util.Map;
import java.util.TreeSet;

/** Direct Esper oracle for the two observable EPLSubselectMultirow executions. */
public final class EPLSubselectMultirowScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "subselect-multirow";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String[] CASES = {
        "multirow-single-column",
        "multirow-underlying-correlated"
    };
    private static final String[] RUNTIME_IDS = {
        "java-runtime-29c2087cc4243e9b7a50",
        "java-runtime-64eb1701d14bdbcefc86"
    };

    private EPLSubselectMultirowScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLSubselectMultirowScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        runSingleColumn(steps, records);
        runUnderlyingCorrelated(steps, records);

        JsonObject trace = new JsonObject()
            .add("version", VERSION)
            .add("id", SCENARIO_ID)
            .add("javaCommit", JAVA_COMMIT)
            .add("java", System.getProperty("java.version"))
            .add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unexpected scenario id " + scenario.getString("id", ""));
        }

        JsonValue runtimesValue = scenario.get("javaRuntimes");
        if (runtimesValue == null || !runtimesValue.isArray() || runtimesValue.asArray().size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must list exactly two Java runtimes");
        }
        for (int i = 0; i < RUNTIME_IDS.length; i++) {
            JsonValue runtimeValue = runtimesValue.asArray().get(i);
            if (runtimeValue == null || !runtimeValue.isString() || !RUNTIME_IDS[i].equals(runtimeValue.asString())) {
                throw new IllegalArgumentException("scenario Java runtime order mismatch at " + i);
            }
        }

        JsonValue casesValue = scenario.get("cases");
        if (casesValue == null || !casesValue.isArray() || casesValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonValue caseValue = casesValue.asArray().get(i);
            if (caseValue == null || !caseValue.isObject()) {
                throw new IllegalArgumentException("scenario case metadata is not an object at " + i);
            }
            JsonObject caseObject = caseValue.asObject();
            requireFields(caseObject, "case metadata", "case", "runtimeId");
            requireString(caseObject, "case", CASES[i]);
            requireString(caseObject, "runtimeId", RUNTIME_IDS[i]);
        }

        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        String[][] eventTypes = {
            {"SupportBean", "SupportBean", "SupportBean", "SupportBean", "SupportBean_S0", "SupportBean_S0", "SupportBean", "SupportBean_S0"},
            {"SupportBean_S0", "SupportBean", "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean_S0"}
        };
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("scenario is missing case " + CASES[caseIndex]);
            }
            JsonObject marker = objectStep(steps.get(offset++), "case marker");
            requireFields(marker, "case marker", "op", "case");
            requireString(marker, "op", "case");
            requireString(marker, "case", CASES[caseIndex]);
            for (int sendIndex = 0; sendIndex < eventTypes[caseIndex].length; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject send = objectStep(steps.get(offset++), "send");
                requireFields(send, "send", "op", "eventType", "payload");
                requireString(send, "op", "send");
                requireString(send, "eventType", eventTypes[caseIndex][sendIndex]);
                JsonValue payloadValue = send.get("payload");
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

    private static JsonObject objectStep(JsonValue value, String kind) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException("scenario " + kind + " is not an object");
        }
        return value.asObject();
    }

    private static void requireFields(JsonObject object, String kind, String... names) {
        if (object.names().size() != names.length) {
            throw new IllegalArgumentException("scenario " + kind + " has unsupported metadata");
        }
        for (String name : names) {
            if (!object.names().contains(name)) {
                throw new IllegalArgumentException("scenario " + kind + " is missing field " + name);
            }
        }
    }

    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        if (caseIndex == 0) {
            if (sendIndex < 4 || sendIndex == 6) {
                requireFields(payload, "single-column SupportBean payload", "theString", "intPrimitive");
                String[] strings = {"T1", "T2", "T3", "T1", "T1"};
                int[] values = {5, 10, 15, 6, 5};
                int beanIndex = sendIndex < 4 ? sendIndex : 4;
                requireString(payload, "theString", strings[beanIndex]);
                requireNumber(payload, "intPrimitive", values[beanIndex]);
            } else {
                requireFields(payload, "single-column trigger payload", "id");
                requireNumber(payload, "id", 0);
            }
            return;
        }

        if (sendIndex == 0 || sendIndex == 2 || sendIndex == 5) {
            requireFields(payload, "correlated SupportBean_S0 payload", "id", "p00");
            int[] ids = {1, 2, 3};
            String[] keys = {"T1", "T1", "T2"};
            int triggerIndex = sendIndex == 0 ? 0 : sendIndex == 2 ? 1 : 2;
            requireNumber(payload, "id", ids[triggerIndex]);
            requireString(payload, "p00", keys[triggerIndex]);
        } else {
            requireFields(payload, "correlated SupportBean payload", "theString", "intPrimitive");
            int beanIndex = sendIndex == 1 ? 0 : sendIndex == 3 ? 1 : 2;
            String[] strings = {"T1", "T2", "T2"};
            int[] values = {10, 20, 30};
            requireString(payload, "theString", strings[beanIndex]);
            requireNumber(payload, "intPrimitive", values[beanIndex]);
        }
    }

    private static void requireString(JsonObject object, String name, String expected) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("scenario string mismatch for " + name);
        }
    }

    private static void requireNumber(JsonObject object, String name, int expected) {
        JsonValue value = object.get(name);
        if (value == null || value.isNull() || !value.isNumber() || value.asInt() != expected) {
            throw new IllegalArgumentException("scenario number mismatch for " + name);
        }
    }

    private static void runSingleColumn(JsonArray allSteps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-subselect-multirow-single-column", configuration);
        runtime.getEventService().advanceTime(0);
        try {
            deploy(runtime,
                "@public create window SupportWindow#length(3) as SupportBean;\n"
                    + "insert into SupportWindow select * from SupportBean;",
                "parity-subselect-multirow-single-column-infra");
            EPDeployment currentDeployment = deploy(runtime,
                "@name('s0') select p00, (select window(intPrimitive) from SupportBean#keepall sb) as val "
                    + "from SupportBean_S0 as s0",
                "parity-subselect-multirow-single-column-direct");
            EPStatement currentStatement = findStatement(currentDeployment);
            validateMetadata(currentStatement, Integer[].class);
            Sequence sequence = new Sequence();
            currentStatement.addListener(new TraceWriter(records, CASES[0], sequence));

            boolean active = false;
            int triggerCount = 0;
            for (int i = 0; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    active = CASES[0].equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if (!"send".equals(op)) {
                    throw new IllegalArgumentException("unsupported operation " + op + " in " + CASES[0]);
                }
                sendEvent(runtime, step);
                if ("SupportBean_S0".equals(step.getString("eventType", ""))) {
                    triggerCount++;
                    if (triggerCount == 1) {
                        runtime.getDeploymentService().undeploy(currentDeployment.getDeploymentId());
                        currentDeployment = deploy(runtime,
                            "@name('s0') select p00, (select window(intPrimitive) from SupportWindow) as val "
                                + "from SupportBean_S0 as s0",
                            "parity-subselect-multirow-single-column-late");
                        currentStatement = findStatement(currentDeployment);
                        validateMetadata(currentStatement, Integer[].class);
                        currentStatement.addListener(new TraceWriter(records, CASES[0], sequence));
                    }
                }
            }
            if (triggerCount != 3) {
                throw new IllegalArgumentException("single-column case requires three SupportBean_S0 triggers");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void runUnderlyingCorrelated(JsonArray allSteps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-subselect-multirow-underlying-correlated", configuration);
        runtime.getEventService().advanceTime(0);
        try {
            EPDeployment deployment = deploy(runtime,
                "@name('s0') select p00, (select window(sb.*) from SupportBean#keepall sb "
                    + "where theString = s0.p00) as val from SupportBean_S0 as s0",
                "parity-subselect-multirow-underlying-correlated");
            EPStatement statement = findStatement(deployment);
            validateMetadata(statement, SupportBean[].class);
            Sequence sequence = new Sequence();
            statement.addListener(new TraceWriter(records, CASES[1], sequence));
            replayCase(allSteps, CASES[1], runtime, 3);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, int expectedTriggers) {
        boolean active = false;
        int triggerCount = 0;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if (!"send".equals(op)) {
                throw new IllegalArgumentException("unsupported operation " + op + " in " + caseName);
            }
            sendEvent(runtime, step);
            if ("SupportBean_S0".equals(step.getString("eventType", ""))) {
                triggerCount++;
            }
        }
        if (triggerCount != expectedTriggers) {
            throw new IllegalArgumentException(caseName + " requires " + expectedTriggers + " SupportBean_S0 triggers");
        }
    }

    private static EPDeployment deploy(EPRuntime runtime, String epl, String deploymentId) throws Exception {
        CompilerArguments arguments = new CompilerArguments(runtime.getConfigurationDeepCopy());
        arguments.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, arguments);
        return runtime.getDeploymentService().deploy(compiled,
            new DeploymentOptions().setDeploymentId(deploymentId));
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement result = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                if (result != null) {
                    throw new IllegalStateException("multiple s0 statements were deployed");
                }
                result = candidate;
            }
        }
        if (result == null) {
            throw new IllegalStateException("statement s0 was not deployed");
        }
        return result;
    }

    private static void validateMetadata(EPStatement statement, Class<?> valueType) {
        EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
        Map<String, Class<?>> properties = new HashMap<>();
        for (EventPropertyDescriptor descriptor : descriptors) {
            properties.put(descriptor.getPropertyName(), descriptor.getPropertyType());
        }
        if (properties.size() != 2 || !String.class.equals(properties.get("p00"))
            || !valueType.equals(properties.get("val"))) {
            throw new IllegalStateException("unexpected s0 metadata: p00=" + properties.get("p00")
                + ", val=" + properties.get("val"));
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean": {
                SupportBean event = new SupportBean();
                event.setTheString(payload.getString("theString", ""));
                event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(event, "SupportBean");
                return;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                String p00 = payload.getString("p00", null);
                if (p00 != null) {
                    event.setP00(p00);
                }
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                return;
            }
            default:
                throw new IllegalArgumentException("unsupported event type " + eventType);
        }
    }

    private static final class Sequence {
        private long value;
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final Sequence sequence;

        private TraceWriter(JsonArray records, String caseName, Sequence sequence) {
            this.records = records;
            this.caseName = caseName;
            this.sequence = sequence;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "listener")
                .add("statement", statement.getName())
                .add("sequence", ++sequence.value)
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                .add("new", rows(newEvents));
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray output = new JsonArray();
        if (events == null) {
            return output;
        }
        for (EventBean event : events) {
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            output.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return output;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer[]) {
            return arrayValues((Object[]) value, false);
        }
        if (value instanceof SupportBean[]) {
            return arrayValues((Object[]) value, true);
        }
        if (value instanceof EventBean[]) {
            return arrayValues((Object[]) value, true);
        }
        if (value instanceof Object[]) {
            return arrayValues((Object[]) value, false);
        }
        if (value instanceof SupportBean) {
            return supportBeanRow((SupportBean) value);
        }
        if (value instanceof EventBean) {
            return eventRow((EventBean) value);
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

    private static JsonArray arrayValues(Object[] values, boolean sort) {
        JsonValue[] rendered = new JsonValue[values.length];
        for (int i = 0; i < values.length; i++) {
            rendered[i] = normalize(values[i]);
        }
        if (sort) {
            Arrays.sort(rendered, (left, right) -> left.toString().compareTo(right.toString()));
        }
        JsonArray output = new JsonArray();
        for (JsonValue value : rendered) {
            output.add(value);
        }
        return output;
    }

    private static JsonValue supportBeanRow(SupportBean bean) {
        JsonObject fields = new JsonObject()
            .add("intPrimitive", normalize(bean.getIntPrimitive()))
            .add("theString", normalize(bean.getTheString()));
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue eventRow(EventBean event) {
        Object underlying = event.getUnderlying();
        if (underlying instanceof SupportBean) {
            return supportBeanRow((SupportBean) underlying);
        }
        if (underlying instanceof Map) {
            JsonObject fields = new JsonObject();
            Map<?, ?> map = (Map<?, ?>) underlying;
            for (Object key : new TreeSet<>(map.keySet())) {
                fields.add(String.valueOf(key), normalize(map.get(key)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        throw new IllegalStateException("unsupported event array element "
            + (underlying == null ? "null" : underlying.getClass().getName()));
    }
}
