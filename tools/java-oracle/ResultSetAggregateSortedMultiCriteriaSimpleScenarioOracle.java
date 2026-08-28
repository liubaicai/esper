import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
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
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for ResultSetAggregateMultipleCriteriaSimple. */
public final class ResultSetAggregateSortedMultiCriteriaSimpleScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-multi-criteria-simple";
    private static final String CASE = "multiple-criteria-simple";
    private static final String RUNTIME_URI = "parity-resultset-aggregate-sorted-multi-criteria-simple";
    private static final int SEND_COUNT = 4;

    private ResultSetAggregateSortedMultiCriteriaSimpleScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateSortedMultiCriteriaSimpleScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))
                || !ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario");
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateShape(steps);

        JsonArray records = new JsonArray();
        runCase(steps, records);
        if (records.size() != SEND_COUNT) {
            throw new IllegalStateException("expected four listener records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        if (steps.size() != SEND_COUNT + 1) {
            throw new IllegalArgumentException("scenario must contain one case and four sends");
        }
        JsonObject marker = steps.get(0).asObject();
        if (!"case".equals(marker.getString("op", ""))
                || !CASE.equals(marker.getString("case", ""))) {
            throw new IllegalArgumentException("scenario must start with case " + CASE);
        }
        for (int index = 1; index < steps.size(); index++) {
            JsonObject step = steps.get(index).asObject();
            if (!"send".equals(step.getString("op", ""))
                    || !"SupportBean".equals(step.getString("eventType", ""))) {
                throw new IllegalArgumentException("scenario must contain SupportBean sends only");
            }
            JsonValue payloadValue = step.get("payload");
            if (payloadValue == null || !payloadValue.isObject()) {
                throw new IllegalArgumentException("SupportBean payload is required");
            }
            JsonObject payload = payloadValue.asObject();
            if (payload.size() != 2
                    || !payload.names().contains("theString")
                    || !payload.names().contains("intPrimitive")
                    || payload.get("theString") == null
                    || payload.get("theString").isNull()
                    || !payload.get("theString").isString()
                    || !isInteger(payload, "intPrimitive")) {
                throw new IllegalArgumentException(
                        "SupportBean payload must contain string theString and integer intPrimitive");
            }
        }
    }

    private static boolean isInteger(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber()) {
            return false;
        }
        try {
            return value.asInt() == value.asDouble();
        } catch (RuntimeException ex) {
            return false;
        }
    }

    private static void runCase(JsonArray steps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(RUNTIME_URI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = "@name('s0') select sorted(theString desc, intPrimitive desc) as c0 "
                    + "from SupportBean#keepall";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                    new CompilerArguments(runtime.getRuntimePath()));
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
        for (int index = 1; index < steps.size(); index++) {
            JsonObject payload = steps.get(index).asObject().get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventMap(event, "SupportBean");
        }
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
    }
}
