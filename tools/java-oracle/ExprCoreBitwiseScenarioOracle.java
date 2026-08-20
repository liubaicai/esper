import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.util.Arrays;

/**
 * Direct Esper 9.0.0 oracle for the observable bitwise expression executions.
 * The Java source's object-model execution is represented by the same EPL
 * after its SODA round-trip; the trace records the shared runtime contract.
 */
public final class ExprCoreBitwiseScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String EPL = "select bytePrimitive&byteBoxed as myFirstProperty, " +
            "shortPrimitive|shortBoxed as mySecondProperty, " +
            "intPrimitive|intBoxed as myThirdProperty, " +
            "longPrimitive^longBoxed as myFourthProperty, " +
            "boolPrimitive&boolBoxed as myFifthProperty from SupportBean";

    private ExprCoreBitwiseScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-bitwise.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (scenarioID.isBlank()) {
            throw new IllegalArgumentException("scenario id is required");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        if (hasCase(steps, "bitwise-op")) {
            runCase(steps, "bitwise-op", records);
        }
        if (hasCase(steps, "bitwise-op-om")) {
            runCase(steps, "bitwise-op-om", records);
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (JsonValue value : steps) {
            JsonObject step = value.asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-expr-core-bitwise-" + caseName, configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile("@name('s0') " + EPL,
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
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
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
            if (active && "send".equals(op)) {
                runtime.getEventService().sendEventBean(toSupportBean(step.get("payload").asObject()), "SupportBean");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setBytePrimitive((byte) payload.get("bytePrimitive").asInt());
        bean.setByteBoxed((byte) payload.get("byteBoxed").asInt());
        bean.setShortPrimitive((short) payload.get("shortPrimitive").asInt());
        bean.setShortBoxed((short) payload.get("shortBoxed").asInt());
        bean.setIntPrimitive(payload.get("intPrimitive").asInt());
        bean.setIntBoxed(payload.get("intBoxed").asInt());
        bean.setLongPrimitive(payload.get("longPrimitive").asLong());
        bean.setLongBoxed(payload.get("longBoxed").asLong());
        bean.setBoolPrimitive(payload.get("boolPrimitive").asBoolean());
        bean.setBoolBoxed(payload.get("boolBoxed").asBoolean());
        return bean;
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
