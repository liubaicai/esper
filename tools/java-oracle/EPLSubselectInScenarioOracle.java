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
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the subselect-in parity scenario. Each case
 * mirrors one execution of EPLSubselectIn: IN/NOT IN subselects in the
 * select clause, filter criteria and where clause, over SupportBean_S1
 * length windows with eviction, expression forms on both sides, nullable
 * string and boxed numeric coercions, null rows, and the correlated
 * keepall index shapes exercised by EPLSubselectInSingleIndex/MultiIndex.
 */
public final class EPLSubselectInScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLSubselectInScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLSubselectInScenarioOracle <scenario.json>");
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

        String[] cases = {"in-select", "in-select-om", "in-select-compile", "in-filter-criteria",
                "in-select-where", "in-select-where-expressions", "in-nullable", "in-nullable-coercion",
                "in-null-row", "in-single-index", "in-multi-index", "not-in-null-row", "not-in-select",
                "not-in-nullable-coercion"};
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
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intBoxed", Integer.class);
        beanType.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        String epl;
        if ("in-select".equals(caseName) || "in-select-om".equals(caseName) || "in-select-compile".equals(caseName)) {
            epl = "@name('s0') select id in (select id from SupportBean_S1#length(1000)) as value from SupportBean_S0";
        } else if ("in-filter-criteria".equals(caseName)) {
            epl = "@name('s0') select id from SupportBean_S0(id in (select id from SupportBean_S1#length(2)))";
        } else if ("in-select-where".equals(caseName)) {
            epl = "@name('s0') select id in (select id from SupportBean_S1#length(1000) where id > 0) as value from SupportBean_S0";
        } else if ("in-select-where-expressions".equals(caseName)) {
            epl = "@name('s0') select 3*id in (select 2*id from SupportBean_S1#length(1000)) as value from SupportBean_S0";
        } else if ("in-nullable".equals(caseName)) {
            epl = "@name('s0') select id from SupportBean_S0 as s0 where p00 in (select p10 from SupportBean_S1#length(1000))";
        } else if ("in-nullable-coercion".equals(caseName)) {
            epl = "@name('s0') select longBoxed from SupportBean(theString='A') as s0 " +
                    "where longBoxed in (select intBoxed from SupportBean(theString='B')#length(1000))";
        } else if ("in-null-row".equals(caseName)) {
            epl = "@name('s0') select intBoxed from SupportBean(theString='A') as s0 " +
                    "where intBoxed in (select longBoxed from SupportBean(theString='B')#length(1000))";
        } else if ("in-single-index".equals(caseName)) {
            epl = "@Name('s0') select (select p00 from SupportBean_S0#keepall() as s0 where s0.p01 in (s1.p10, s1.p11)) as c0 from SupportBean_S1 as s1";
        } else if ("in-multi-index".equals(caseName)) {
            epl = "@Name('s0') select (select p00 from SupportBean_S0#keepall() as s0 where s1.p11 in (s0.p00, s0.p01)) as c0 from SupportBean_S1 as s1";
        } else if ("not-in-null-row".equals(caseName)) {
            epl = "@name('s0') select intBoxed from SupportBean(theString='A') as s0 " +
                    "where intBoxed not in (select longBoxed from SupportBean(theString='B')#length(1000))";
        } else if ("not-in-select".equals(caseName)) {
            epl = "@name('s0') select not id in (select id from SupportBean_S1#length(1000)) as value from SupportBean_S0";
        } else if ("not-in-nullable-coercion".equals(caseName)) {
            epl = "@name('s0') select longBoxed from SupportBean(theString='A') as s0 " +
                    "where longBoxed not in (select intBoxed from SupportBean(theString='B')#length(1000))";
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-subselect-in-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-subselect-in-" + caseName));
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
            replayCase(allSteps, caseName, runtime, writer);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, TraceWriter writer) {
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
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            JsonValue p00 = payload.get("p00");
            event.put("p00", p00.isNull() ? null : p00.asString());
            JsonValue p01 = payload.get("p01");
            event.put("p01", p01.isNull() ? null : p01.asString());
        } else if ("SupportBean_S1".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            JsonValue p10 = payload.get("p10");
            event.put("p10", p10.isNull() ? null : p10.asString());
            JsonValue p11 = payload.get("p11");
            event.put("p11", p11.isNull() ? null : p11.asString());
        } else if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            JsonValue intBoxed = payload.get("intBoxed");
            event.put("intBoxed", intBoxed.isNull() ? null : intBoxed.asInt());
            JsonValue longBoxed = payload.get("longBoxed");
            event.put("longBoxed", longBoxed.isNull() ? null : longBoxed.asLong());
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
