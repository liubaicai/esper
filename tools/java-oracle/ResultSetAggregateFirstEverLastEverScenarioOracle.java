import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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
 * Direct Esper 9.0.0 oracle for the three valid executions in
 * ResultSetAggregateFirstEverLastEver. The two first-last-ever cases use the
 * same statement and replay, with the first case compiled through an
 * EPStatementObjectModel parse/copy round-trip to mirror the Java SODA path.
 * The invalid countever(distinct ...) execution is deliberately excluded.
 */
public final class ResultSetAggregateFirstEverLastEverScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-first-ever-last-ever";
    private static final String FIRST_LAST_SODA_TRUE = "first-last-ever-soda-true";
    private static final String FIRST_LAST_SODA_FALSE = "first-last-ever-soda-false";
    private static final String ON_DELETE = "on-delete";

    private ResultSetAggregateFirstEverLastEverScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateFirstEverLastEverScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!ID.equals(scenarioID)) {
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

        String[] cases = {FIRST_LAST_SODA_TRUE, FIRST_LAST_SODA_FALSE, ON_DELETE};
        for (String caseName : cases) {
            if (hasCase(steps, caseName)) {
                runCase(steps, caseName, records);
            }
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("intBoxed", Integer.class);
        beanType.put("boolPrimitive", Boolean.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> triggerType = new HashMap<>();
        triggerType.put("id", String.class);
        configuration.getCommon().addEventType("SupportBean_A", triggerType);

        String runtimeURI = "parity-resultset-aggregate-first-ever-last-ever-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled;
            if (ON_DELETE.equals(caseName)) {
                String epl = "create window MyWindow#keepall as select * from SupportBean;\n" +
                        "insert into MyWindow select * from SupportBean;\n" +
                        "on SupportBean_A delete from MyWindow where theString = id;\n" +
                        "@name('s0') select firstever(theString) as firsteverstring, " +
                        "lastever(theString) as lasteverstring, " +
                        "countever(*) as counteverall from MyWindow";
                compiled = EPCompilerProvider.getCompiler().compile(epl,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                String epl = "@Audit @Name('s0') select " +
                        "firstever(theString) as firsteverstring, " +
                        "lastever(theString) as lasteverstring, " +
                        "first(theString) as firststring, " +
                        "last(theString) as laststring, " +
                        "countever(*) as cntstar, " +
                        "countever(intBoxed) as cntexpr, " +
                        "countever(*,boolPrimitive) as cntstarfiltered, " +
                        "countever(intBoxed,boolPrimitive) as cntexprfiltered " +
                        "from SupportBean.win:length(2)";
                if (FIRST_LAST_SODA_TRUE.equals(caseName)) {
                    EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
                    model = SerializableObjectCopier.copyMayFail(model);
                    Module module = new Module();
                    module.getItems().add(new ModuleItem(model));
                    module.setModuleText(model.toEPL());
                    compiled = EPCompilerProvider.getCompiler().compile(module,
                            new CompilerArguments(runtime.getRuntimePath()));
                } else {
                    compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                }
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-resultset-aggregate-first-ever-last-ever-" + caseName));
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
            if ("send".equals(op)) {
                send(runtime, step);
            } else if ("advance-time".equals(op)) {
                runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
            } else {
                throw new IllegalArgumentException("unsupported operation: " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonValue payloadValue = step.get("payload");
        if (payloadValue == null || !payloadValue.isObject()) {
            throw new IllegalArgumentException("payload is required for " + eventType);
        }
        JsonObject payload = payloadValue.asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.getInt("intPrimitive", 0));
            JsonValue boxed = payload.get("intBoxed");
            event.put("intBoxed", boxed == null || boxed.isNull() ? null : boxed.asInt());
            JsonValue bool = payload.get("boolPrimitive");
            event.put("boolPrimitive", bool == null || bool.isNull() ? false : bool.asBoolean());
        } else if ("SupportBean_A".equals(eventType)) {
            event.put("id", payload.getString("id", null));
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
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
