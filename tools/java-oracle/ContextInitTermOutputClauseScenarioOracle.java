import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-init-term-output-clause parity
 * scenario. Mirrors the five output-clause executions of ContextInitTerm:
 * output all every 2 events and when terminated with group-by/order-by,
 * output when count_insert with a when-terminated condition, output only
 * when terminated with count_insert, output when true then-set plus
 * when-terminated then-set variable assignments, and output only when
 * terminated with a then-set assignment. The every-minute context
 * (initiated by pattern [every timer:at(*, *, *, *, *)] terminated after
 * 1 min) starts a partition every second.
 */
public final class ContextInitTermOutputClauseScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermOutputClauseScenarioOracle() {
    }

    private static final String CTX_EVERY_MINUTE =
        "@public create context EveryMinute as " +
        "initiated by pattern[every timer:at(*, *, *, *, *)] " +
        "terminated after 1 min";

    private static final String CTX_EVERY_S0 =
        "@public create context EverySupportBeanS0 as " +
        "initiated by SupportBean_S0 as s0 " +
        "terminated after 1 min";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermOutputClauseScenarioOracle <scenario.json>");
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

        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            String caseName = step.getString("case", "");
            if (caseName.isBlank()) {
                throw new IllegalArgumentException("case without name at step " + i);
            }
            runCase(steps, i, caseName, records);
        }
        System.out.println(trace);
    }

    private static void runCase(JsonArray allSteps, int caseIndex, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("intBoxed", Integer.class);
        beanType.put("shortBoxed", Short.class);
        beanType.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-init-term-out-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String contextEpl;
            String statementEpl;
            String variableEpl = null;
            switch (caseName) {
                case "op-all-every2-terminated":
                    contextEpl = CTX_EVERY_MINUTE;
                    statementEpl = "@name('s0') context EveryMinute " +
                            "select theString as c1, sum(intPrimitive) as c2 from SupportBean " +
                            "group by theString output all every 2 events and when terminated order by theString asc";
                    break;
                case "op-when-expr-when-terminated":
                    contextEpl = CTX_EVERY_MINUTE;
                    statementEpl = "@name('s0') context EveryMinute " +
                            "select theString as c0 from SupportBean " +
                            "output when count_insert>1 and when terminated and count_insert>0";
                    break;
                case "op-only-when-terminated":
                    contextEpl = CTX_EVERY_MINUTE;
                    statementEpl = "@name('s0') context EveryMinute " +
                            "select theString as c0 from SupportBean " +
                            "output when terminated and count_insert > 0";
                    break;
                case "op-when-set-variable":
                    contextEpl = CTX_EVERY_MINUTE;
                    variableEpl = "@name('var') @public create variable int myvar = 0";
                    statementEpl = "@name('s0') context EveryMinute select theString as c0 from SupportBean " +
                            "output when true " +
                            "then set myvar=1 " +
                            "and when terminated " +
                            "then set myvar=2";
                    break;
                case "op-only-terminated-set":
                    contextEpl = CTX_EVERY_S0;
                    variableEpl = "@name('var') @public create variable int myvar = 0";
                    statementEpl = "@name('s0') context EverySupportBeanS0 select theString as c0 from SupportBean " +
                            "output when terminated " +
                            "then set myvar=10";
                    break;
                default:
                    throw new IllegalArgumentException("unsupported case " + caseName);
            }
            StringBuilder epl = new StringBuilder();
            if (variableEpl != null) {
                epl.append(variableEpl).append(";\n");
            }
            epl.append(contextEpl).append(";\n").append(statementEpl);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    epl.toString(), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 not found");
            }
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);

            boolean active = false;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    send(runtime, step);
                } else if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                } else if ("read-variable".equals(op)) {
                    String name = step.getString("name", "");
                    Object value = runtime.getVariableService().getVariableValue(deployment.getDeploymentId(), name);
                    JsonObject variableRecord = new JsonObject()
                            .add("case", caseName)
                            .add("operation", "variable")
                            .add("name", name);
                    if (value == null) {
                        variableRecord.add("value", new JsonObject().add("state", "null"));
                    } else {
                        variableRecord.add("value", Json.value(((Number) value).intValue()));
                    }
                    records.add(variableRecord);
                } else {
                    throw new IllegalArgumentException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", ""));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            JsonValue intBoxed = payload.get("intBoxed");
            event.put("intBoxed", intBoxed.isNull() ? null : intBoxed.asInt());
            JsonValue shortBoxed = payload.get("shortBoxed");
            event.put("shortBoxed", shortBoxed.isNull() ? null : (short) shortBoxed.asInt());
            JsonValue longBoxed = payload.get("longBoxed");
            event.put("longBoxed", longBoxed.isNull() ? null : longBoxed.asLong());
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", nullableString(payload, "p00"));
            event.put("p01", nullableString(payload, "p01"));
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
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
