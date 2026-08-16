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
 * Direct Esper 9.0.0 oracle for the context-init-term-keyed-aggregation
 * parity scenario. Mirrors ContextInitTermAggregationGrouped: a keyed
 * initiated-terminated context where SummedEvent rows aggregate grouped by
 * key inside each partition, TermEvent terminates the matching partition,
 * and a later InitEvent starts a fresh partition.
 */
public final class ContextInitTermKeyedAggregationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermKeyedAggregationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermKeyedAggregationScenarioOracle <scenario.json>");
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

        String[] cases = {"keyed-aggregation"};
        for (String caseName : cases) {
            if (!hasCase(steps, caseName)) {
                continue;
            }
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> summedType = new HashMap<>();
        summedType.put("grp", String.class);
        summedType.put("key", String.class);
        summedType.put("value", Integer.class);
        configuration.getCommon().addEventType("SummedEvent", summedType);
        Map<String, Object> initType = new HashMap<>();
        initType.put("grp", String.class);
        configuration.getCommon().addEventType("InitEvent", initType);
        Map<String, Object> termType = new HashMap<>();
        termType.put("grp", String.class);
        configuration.getCommon().addEventType("TermEvent", termType);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-init-term-keyed", configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String[] statements = {
                    "create context MyContext " +
                            "initiated by InitEvent as i " +
                            "terminated by TermEvent(grp = i.grp);\n" +
                            "@name('s0') context MyContext " +
                            "select key as c0, sum(value) as c1 " +
                            "from SummedEvent(grp = context.i.grp) group by key",
            };
            String[] traced = {"s0"};
            Map<String, TraceWriter> writers = new HashMap<>();
            for (String epl : statements) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                for (EPStatement candidate : deployment.getStatements()) {
                    String name = candidate.getName();
                    boolean wanted = false;
                    for (String tracedName : traced) {
                        if (tracedName.equals(name)) {
                            wanted = true;
                            break;
                        }
                    }
                    if (!wanted) {
                        continue;
                    }
                    TraceWriter writer = new TraceWriter(records, caseName, candidate, runtime);
                    candidate.addListener(writer);
                    writers.put(name, writer);
                }
            }
            replayCase(allSteps, caseName, runtime, records, writers);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static boolean hasCase(JsonArray allSteps, String wanted) {
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, JsonArray records,
                                   Map<String, TraceWriter> writers) {
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
        if ("SummedEvent".equals(eventType)) {
            event.put("grp", payload.getString("grp", null));
            event.put("key", payload.getString("key", null));
            event.put("value", payload.get("value").asInt());
        } else if ("InitEvent".equals(eventType)) {
            event.put("grp", payload.getString("grp", null));
        } else if ("TermEvent".equals(eventType)) {
            event.put("grp", payload.getString("grp", null));
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
