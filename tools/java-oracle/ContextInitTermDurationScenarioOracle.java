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
 * Direct Esper 9.0.0 oracle for the context-init-term-duration parity
 * scenario. Mirrors six ContextInitTerm executions with duration-based,
 * calendar-month and same-event termination:
 *
 * <ul>
 *   <li>ContextStartEndAfterZeroInitiatedNow — start after 0 sec end after 60 sec</li>
 *   <li>ContextStartEndEndSameEventAsAnalyzed — same event terminates (not included vs included via insert-into)</li>
 *   <li>ContextInitTermFilterInitiatedFilterAllTerminated — filter start, filter-all end</li>
 *   <li>ContextInitTermFilterAndAfter1Min — filter start + after 1 min</li>
 *   <li>ContextInitTermPatternIntervalZeroInitiatedNow — pattern interval(0) or every 1 min + after 60 sec</li>
 *   <li>ContextStartEndStartNowCalMonthScoped — start by event, end after 1 month</li>
 * </ul>
 */
public final class ContextInitTermDurationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermDurationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermDurationScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        configuration.getCommon().addEventType("SupportBean_S1", s1Type);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-duration-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String initialTime = initialAdvanceTime(allSteps, caseName);
            if (initialTime != null) {
                runtime.getEventService().advanceTime(Instant.parse(initialTime).toEpochMilli());
            }
            String epl;
            String listenerStatement;
            switch (caseName) {
                case "start-after-zero":
                    epl = "@public create context CtxPerId start after 0 sec end after 60 sec;\n" +
                            "@name('s0') context CtxPerId select theString as c0, intPrimitive as c1 from SupportBean";
                    listenerStatement = "s0";
                    break;
                case "same-event-not-included":
                    epl = "@public create context MyCtx as " +
                            "start SupportBean " +
                            "end SupportBean(intPrimitive=11);\n" +
                            "@name('s0') context MyCtx " +
                            "select min(intPrimitive) as c1, max(intPrimitive) as c2, sum(intPrimitive) as c3, avg(intPrimitive) as c4 from SupportBean " +
                            "output snapshot when terminated";
                    listenerStatement = "s0";
                    break;
                case "same-event-included":
                    epl = "create schema MyCtxTerminate(theString string);\n" +
                            "create context MyCtx as start SupportBean end MyCtxTerminate;\n" +
                            "@name('s0') context MyCtx " +
                            "select min(intPrimitive) as c1, max(intPrimitive) as c2, sum(intPrimitive) as c3, avg(intPrimitive) as c4 from SupportBean " +
                            "output snapshot when terminated;\n" +
                            "insert into MyCtxTerminate select theString from SupportBean(intPrimitive=11)";
                    listenerStatement = "s0";
                    break;
                case "filter-all-terminated":
                    epl = "create context MyContext as " +
                            "initiated by SupportBean_S0 " +
                            "terminated by SupportBean_S1;\n" +
                            "@name('s0') context MyContext select sum(intPrimitive) as c1 from SupportBean";
                    listenerStatement = "s0";
                    break;
                case "filter-after-1-min":
                    epl = "@Name('CTX') create context CtxInitiated " +
                            "initiated by SupportBean_S0 as sb0 " +
                            "terminated after 1 minute;\n" +
                            "@Name('S1') context CtxInitiated select theString as c1, sum(intPrimitive) as c2, context.sb0.p00 as c3 from SupportBean";
                    listenerStatement = "S1";
                    break;
                case "pattern-interval-zero":
                    epl = "create context CtxPerId initiated by pattern [timer:interval(0) or every timer:interval(1 min)] terminated after 60 sec;\n" +
                            "@name('s0') context CtxPerId select theString as c0, sum(intPrimitive) as c1 from SupportBean";
                    listenerStatement = "s0";
                    break;
                case "cal-month-scoped":
                    epl = "@public create context MyCtx start SupportBean_S1 end after 1 month;\n" +
                            "@name('s0') context MyCtx select * from SupportBean";
                    listenerStatement = "s0";
                    break;
                default:
                    throw new IllegalArgumentException("unsupported case " + caseName);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if (listenerStatement.equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement " + listenerStatement + " not found");
            }
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);

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
                } else {
                    throw new IllegalArgumentException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String initialAdvanceTime(JsonArray allSteps, String wanted) {
        boolean active = false;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = wanted.equals(step.getString("case", ""));
                continue;
            }
            if (active && "advance-time".equals(op)) {
                return step.getString("at", null);
            }
        }
        return null;
    }

    private static void send(EPRuntime runtime, JsonObject step) {        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", ""));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", nullableString(payload, "p00"));
            event.put("p01", nullableString(payload, "p01"));
        } else if ("SupportBean_S1".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p10", nullableString(payload, "p10"));
            event.put("p11", nullableString(payload, "p11"));
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
            if (value instanceof Double || value instanceof Float) {
                double d = ((Number) value).doubleValue();
                if (d == Math.rint(d) && !Double.isInfinite(d)) {
                    return Json.value((long) d);
                }
                return Json.value(d);
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
