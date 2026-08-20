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

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregateFiltered's two supported
 * executions: ResultSetAggregateBlackWhitePercent and
 * ResultSetAggregateCountVariations.
 */
public final class ResultSetAggregateFilteredScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String BLACK_WHITE = "black-white-percent";
    private static final String COUNT_VARIATIONS = "count-variations";

    private ResultSetAggregateFilteredScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateFilteredScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!"resultset-aggregate-filtered".equals(scenarioID)) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenarioID);
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray() || stepsValue.asArray().size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        boolean sawBlackWhite = false;
        boolean sawCountVariations = false;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String operation = step.getString("op", "");
            if (!"case".equals(operation)) {
                continue;
            }
            String caseName = step.getString("case", "");
            if (BLACK_WHITE.equals(caseName)) {
                if (sawBlackWhite) {
                    throw new IllegalArgumentException("duplicate case: " + caseName);
                }
                sawBlackWhite = true;
            } else if (COUNT_VARIATIONS.equals(caseName)) {
                if (sawCountVariations) {
                    throw new IllegalArgumentException("duplicate case: " + caseName);
                }
                sawCountVariations = true;
            } else {
                throw new IllegalArgumentException("unsupported case: " + caseName);
            }
            runCase(steps, i, caseName, records);
        }
        if (!sawBlackWhite || !sawCountVariations) {
            throw new IllegalArgumentException("scenario must contain both filtered aggregate cases");
        }
        System.out.println(trace);
    }

    private static void runCase(JsonArray allSteps, int caseIndex, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("boolPrimitive", Boolean.class);
        beanType.put("intBoxed", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-resultset-aggregate-filtered-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl;
            if (BLACK_WHITE.equals(caseName)) {
                epl = "@name('s0') select count(*,boolPrimitive) as cb, " +
                        "count(*,not boolPrimitive) as cnb, count(*) as c, " +
                        "count(*,boolPrimitive)/count(*) as pct " +
                        "from SupportBean#length(3)";
            } else {
                epl = "@name('s0') select count(intBoxed, boolPrimitive) as c1, " +
                        "count(distinct intBoxed, boolPrimitive) as c2 " +
                        "from SupportBean#length(3)";
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-resultset-aggregate-filtered-" + caseName));
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
            replayCase(allSteps, caseIndex, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replayCase(JsonArray allSteps, int caseIndex, EPRuntime runtime) {
        for (int i = caseIndex + 1; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String operation = step.getString("op", "");
            if ("case".equals(operation)) {
                return;
            }
            if ("send".equals(operation)) {
                send(runtime, step);
            } else {
                throw new IllegalArgumentException("unsupported operation: " + operation);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        if (!"SupportBean".equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("unsupported event type: " + step.getString("eventType", ""));
        }
        JsonValue payloadValue = step.get("payload");
        if (payloadValue == null || !payloadValue.isObject()) {
            throw new IllegalArgumentException("SupportBean payload is required");
        }
        JsonObject payload = payloadValue.asObject();
        Map<String, Object> event = new HashMap<>();
        JsonValue boolValue = payload.get("boolPrimitive");
        if (boolValue == null || boolValue.isNull()) {
            throw new IllegalArgumentException("boolPrimitive is required");
        }
        event.put("boolPrimitive", boolValue.asBoolean());
        JsonValue intValue = payload.get("intBoxed");
        event.put("intBoxed", intValue == null || intValue.isNull() ? null : intValue.asInt());
        runtime.getEventService().sendEventMap(event, "SupportBean");
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
