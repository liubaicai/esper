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
 * Direct Esper 9.0.0 oracle for the subselect-aggregated-in-exists-any-all
 * parity scenario. Each case mirrors one execution of
 * EPLSubselectAggregatedInExistsAnyAll: SupportValueEvent triggers project
 * IN/NOT IN, EXISTS/NOT EXISTS or quantified ALL/ANY/SOME comparisons against
 * an aggregate subselect over SupportBean#keepall (ungrouped or grouped by
 * theString, optionally filtered by a having clause that reads
 * last/first(theString)). The grouped-exists cases read a named window
 * populated by SupportIdAndValueEvent and include a fire-and-forget
 * delete-all step.
 */
public final class EPLSubselectAggregatedInExistsAnyAllScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLSubselectAggregatedInExistsAnyAllScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLSubselectAggregatedInExistsAnyAllScenarioOracle <scenario.json>");
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

        String[] cases = {"ungrouped-in", "ungrouped-any-all", "ungrouped-exists", "ungrouped-having-exists",
                "ungrouped-having-in", "ungrouped-having-any-all", "ungrouped-having-equals",
                "grouped-in", "grouped-equals", "grouped-having-in", "grouped-having-equals",
                "grouped-exists", "grouped-having-exists"};
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
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> valueType = new HashMap<>();
        valueType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportValueEvent", valueType);
        Map<String, Object> idValueType = new HashMap<>();
        idValueType.put("id", String.class);
        idValueType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportIdAndValueEvent", idValueType);

        String epl;
        if ("ungrouped-in".equals(caseName)) {
            epl = "@name('s0') select value in (select sum(intPrimitive) from SupportBean#keepall) as c0," +
                    "value not in (select sum(intPrimitive) from SupportBean#keepall) as c1 " +
                    "from SupportValueEvent";
        } else if ("ungrouped-any-all".equals(caseName)) {
            epl = "@name('s0') select " +
                    "value < all (select sum(intPrimitive) from SupportBean#keepall) as c0, " +
                    "value < any (select sum(intPrimitive) from SupportBean#keepall) as c1, " +
                    "value < some (select sum(intPrimitive) from SupportBean#keepall) as c2 " +
                    "from SupportValueEvent";
        } else if ("ungrouped-exists".equals(caseName)) {
            epl = "@name('s0') select exists (select sum(intPrimitive) from SupportBean) as c0," +
                    "not exists (select sum(intPrimitive) from SupportBean) as c1 from SupportValueEvent";
        } else if ("ungrouped-having-exists".equals(caseName)) {
            epl = "@name('s0') select exists (select sum(intPrimitive) from SupportBean having sum(intPrimitive) < 15) as c0," +
                    "not exists (select sum(intPrimitive) from SupportBean  having sum(intPrimitive) < 15) as c1 from SupportValueEvent";
        } else if ("ungrouped-having-in".equals(caseName)) {
            epl = "@name('s0') select value in (select sum(intPrimitive) from SupportBean#keepall having last(theString) != 'E1') as c0," +
                    "value not in (select sum(intPrimitive) from SupportBean#keepall having last(theString) != 'E1') as c1 " +
                    "from SupportValueEvent";
        } else if ("ungrouped-having-any-all".equals(caseName)) {
            epl = "@name('s0') select " +
                    "value < all (select sum(intPrimitive) from SupportBean#keepall having last(theString) not in ('E1', 'E3')) as c0, " +
                    "value < any (select sum(intPrimitive) from SupportBean#keepall having last(theString) not in ('E1', 'E3')) as c1, " +
                    "value < some (select sum(intPrimitive) from SupportBean#keepall having last(theString) not in ('E1', 'E3')) as c2 " +
                    "from SupportValueEvent";
        } else if ("ungrouped-having-equals".equals(caseName)) {
            epl = "@name('s0') select " +
                    "value = all (select sum(intPrimitive) from SupportBean#keepall having last(theString) != 'E1') as c0, " +
                    "value = any (select sum(intPrimitive) from SupportBean#keepall having last(theString) != 'E1') as c1, " +
                    "value = some (select sum(intPrimitive) from SupportBean#keepall having last(theString) != 'E1') as c2 " +
                    "from SupportValueEvent";
        } else if ("grouped-in".equals(caseName)) {
            epl = "@name('s0') select value in (select sum(intPrimitive) from SupportBean#keepall group by theString) as c0," +
                    "value not in (select sum(intPrimitive) from SupportBean#keepall group by theString) as c1 " +
                    "from SupportValueEvent";
        } else if ("grouped-equals".equals(caseName)) {
            epl = "@name('s0') select " +
                    "value = all (select sum(intPrimitive) from SupportBean#keepall group by theString) as c0, " +
                    "value = any (select sum(intPrimitive) from SupportBean#keepall group by theString) as c1, " +
                    "value = some (select sum(intPrimitive) from SupportBean#keepall group by theString) as c2 " +
                    "from SupportValueEvent";
        } else if ("grouped-having-in".equals(caseName)) {
            epl = "@name('s0') select value in (select sum(intPrimitive) from SupportBean#keepall group by theString having last(theString) != 'E1') as c0," +
                    "value not in (select sum(intPrimitive) from SupportBean#keepall group by theString having last(theString) != 'E1') as c1 " +
                    "from SupportValueEvent";
        } else if ("grouped-having-equals".equals(caseName)) {
            epl = "@name('s0') select " +
                    "value = all (select sum(intPrimitive) from SupportBean#keepall group by theString having first(theString) != 'E1') as c0, " +
                    "value = any (select sum(intPrimitive) from SupportBean#keepall group by theString having first(theString) != 'E1') as c1, " +
                    "value = some (select sum(intPrimitive) from SupportBean#keepall group by theString having first(theString) != 'E1') as c2 " +
                    "from SupportValueEvent";
        } else if ("grouped-exists".equals(caseName)) {
            epl = "@name('create') @public create window MyWindow#keepall as (key string, anint int);\n" +
                    "@name('insert') insert into MyWindow(key, anint) select id, value from SupportIdAndValueEvent;\n" +
                    "@name('s0') select exists (select sum(anint) from MyWindow group by key) as c0," +
                    "not exists (select sum(anint) from MyWindow group by key) as c1 from SupportValueEvent";
        } else if ("grouped-having-exists".equals(caseName)) {
            epl = "@name('create') @public create window MyWindow#keepall as (key string, anint int);\n" +
                    "@name('insert') insert into MyWindow(key, anint) select id, value from SupportIdAndValueEvent;\n" +
                    "@name('s0') select exists (select sum(anint) from MyWindow group by key having sum(anint) < 15) as c0," +
                    "not exists (select sum(anint) from MyWindow group by key having sum(anint) < 15) as c1 from SupportValueEvent";
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-subselect-aggregated-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-subselect-aggregated-" + caseName));
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

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, TraceWriter writer) throws Exception {
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
            } else if ("faf".equals(op)) {
                EPCompiled faf = EPCompilerProvider.getCompiler().compileQuery("delete from MyWindow",
                        new CompilerArguments(runtime.getRuntimePath()));
                runtime.getFireAndForgetService().executeQuery(faf);
            } else {
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
        } else if ("SupportValueEvent".equals(eventType)) {
            event.put("value", payload.get("value").asInt());
        } else if ("SupportIdAndValueEvent".equals(eventType)) {
            event.put("id", payload.getString("id", null));
            event.put("value", payload.get("value").asInt());
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
