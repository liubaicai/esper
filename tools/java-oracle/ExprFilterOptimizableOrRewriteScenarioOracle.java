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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Map;

public final class ExprFilterOptimizableOrRewriteScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ExprFilterOptimizableOrRewriteScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ExprFilterOptimizableOrRewriteScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
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
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        String[] cases = {
            "two-or", "three-or", "three-with-overlap", "four-or",
            "with-and", "inner-or", "not-equals-or", "not-equals-consolidate",
            "boolean-simple", "boolean-and", "and-or-multi"
        };
        for (String caseName : cases) {
            if (!hasCase(steps, caseName)) {
                continue;
            }
            runCase(steps, caseName, records);
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

        Map<String, Object> supportBeanType = new HashMap<>();
        supportBeanType.put("theString", String.class);
        supportBeanType.put("intPrimitive", int.class);
        supportBeanType.put("intBoxed", Integer.class);
        supportBeanType.put("longPrimitive", long.class);
        supportBeanType.put("longBoxed", Long.class);
        supportBeanType.put("doublePrimitive", double.class);
        supportBeanType.put("doubleBoxed", Double.class);
        supportBeanType.put("boolPrimitive", boolean.class);
        configuration.getCommon().addEventType("SupportBean", supportBeanType);

        Map<String, Object> intAlphaType = new HashMap<>();
        intAlphaType.put("a", int.class);
        intAlphaType.put("b", int.class);
        intAlphaType.put("c", int.class);
        intAlphaType.put("d", int.class);
        intAlphaType.put("e", int.class);
        configuration.getCommon().addEventType("IntAlpha", intAlphaType);

        Map<String, Object> stringAlphaType = new HashMap<>();
        stringAlphaType.put("a", String.class);
        stringAlphaType.put("b", String.class);
        stringAlphaType.put("c", String.class);
        configuration.getCommon().addEventType("StringAlpha", stringAlphaType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-forw-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        String epl = switch (caseName) {
            case "two-or" -> "@name('s0') select * from SupportBean(theString = 'a' or intPrimitive = 1)";
            case "three-or" -> "@name('s0') select * from SupportBean(theString = 'a' or intPrimitive = 1 or longPrimitive = 2)";
            case "three-with-overlap" -> "@name('s0') select * from SupportBean(theString = 'a' or theString = 'b' or intPrimitive = 1)";
            case "four-or" -> "@name('s0') select * from SupportBean(theString = 'a' or intPrimitive = 1 or longPrimitive = 10 or doublePrimitive = 100)";
            case "with-and" -> "@name('s0') select * from SupportBean((theString = 'a' and intPrimitive = 1) or (theString = 'b' and intPrimitive = 2))";
            case "inner-or" -> "@name('s0') select * from SupportBean(theString = 'a' and (intPrimitive = 1 or longPrimitive = 10))";
            case "not-equals-or" -> "@name('s0') select * from IntAlpha(a != 1 and a != 2 and (b = 1 or c = 1))";
            case "not-equals-consolidate" -> "@name('s0') select * from IntAlpha(a != 1 and a != 2 and (a != 3 or a != 4))";
            case "boolean-simple" -> "@name('s0') select * from StringAlpha(a like 'a%' and (b = 'b' or c = 'c'))";
            case "boolean-and" -> "@name('s0') select * from StringAlpha((a = 'a' or a like 'A%') and (b = 'b' or b like 'B%'))";
            case "and-or-multi" -> "@name('s0') select * from IntAlpha(a = 1 and (b = 1 or c = 1) and (d = 1 or e = 1))";
            default -> throw new IllegalArgumentException("unsupported case " + caseName);
        };

        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-forw-" + caseName));
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
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        switch (eventType) {
            case "SupportBean" -> {
                event.put("theString", payload.getString("theString", ""));
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                if (payload.get("intBoxed") != null) event.put("intBoxed", payload.get("intBoxed").asInt());
                event.put("longPrimitive", payload.getLong("longPrimitive", 0L));
                if (payload.get("longBoxed") != null) event.put("longBoxed", payload.get("longBoxed").asLong());
                event.put("doublePrimitive", payload.getDouble("doublePrimitive", 0.0));
                if (payload.get("doubleBoxed") != null) event.put("doubleBoxed", payload.get("doubleBoxed").asDouble());
                if (payload.get("boolPrimitive") != null) event.put("boolPrimitive", payload.get("boolPrimitive").asBoolean());
                runtime.getEventService().sendEventMap(event, "SupportBean");
            }
            case "IntAlpha" -> {
                event.put("a", payload.getInt("a", 0));
                event.put("b", payload.getInt("b", 0));
                event.put("c", payload.getInt("c", 0));
                event.put("d", payload.getInt("d", 0));
                event.put("e", payload.getInt("e", 0));
                runtime.getEventService().sendEventMap(event, "IntAlpha");
            }
            case "StringAlpha" -> {
                event.put("a", payload.getString("a", ""));
                event.put("b", payload.getString("b", ""));
                event.put("c", payload.getString("c", ""));
                runtime.getEventService().sendEventMap(event, "StringAlpha");
            }
            default -> throw new IllegalArgumentException("unsupported event type " + eventType);
        }
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
            append("listener", ++sequence, newEvents, oldEvents);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = results(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
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
                java.util.Arrays.sort(names);
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
