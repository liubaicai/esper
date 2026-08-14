import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
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
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the shared rowrecog-aggregation parity
 * scenario. Mirrors RowRecogAggregation.RowRecogMeasureAggregation and
 * RowRecogMeasureAggregationPartitioned: MATCH_RECOGNIZE measures with
 * max/min/first/last/count/sum aggregations and iterator snapshots.
 */
public final class RowRecogAggregationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private RowRecogAggregationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: RowRecogAggregationScenarioOracle <scenario.json>");
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

        String[] cases = {"unpartitioned", "partitioned"};
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
        beanType.put("cat", String.class);
        beanType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportRecogBean", beanType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-rowrecog-aggregation-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        String epl;
        if ("unpartitioned".equals(caseName)) {
            epl = "@name('s0') select * from SupportRecogBean#keepall " +
                    "match_recognize (" +
                    "  measures A.theString as a_string, C.theString as c_string, " +
                    "       max(B.value) as maxb, min(B.value) as minb, " +
                    "       2*min(B.value) as minb2x, last(B.value) as lastb, " +
                    "       first(B.value) as firstb, count(B.value) as countb " +
                    "  all matches pattern (A B* C) " +
                    "  define A as (A.value = 0), B as (B.value != 1), C as (C.value = 1)" +
                    ") order by a_string";
        } else if ("partitioned".equals(caseName)) {
            epl = "@name('s0') select * from SupportRecogBean#keepall " +
                    "match_recognize (" +
                    "  partition by cat " +
                    "  measures A.cat as cat, A.theString as a_string, D.theString as d_string, " +
                    "       sum(C.value) as sumc, sum(B.value) as sumb, " +
                    "       sum(B.value + A.value) as sumaplusb, sum(C.value + A.value) as sumaplusc " +
                    "  all matches pattern (A B B C C D) " +
                    "  define A as (A.value >= 10), B as (B.value > 1), " +
                    "         C as (C.value < -1), D as (D.value = 999)" +
                    ") order by cat";
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-rowrecog-aggregation-" + caseName));
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
            replayCase(allSteps, caseName, runtime, statement, writer);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, EPStatement statement,
                                   TraceWriter writer) {
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
            } else if ("snapshot".equals(op)) {
                if (!statement.getName().equals(step.getString("statement", ""))) {
                    throw new IllegalArgumentException("unknown statement " + step.getString("statement", ""));
                }
                writer.snapshot();
            } else {
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportRecogBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("cat", payload.getString("cat", null));
        event.put("value", payload.get("value").asInt());
        runtime.getEventService().sendEventMap(event, "SupportRecogBean");
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

        private void snapshot() {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append("snapshot", 0, events.toArray(new EventBean[0]), null);
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
                    Object value = event.get(name);
                    if (value == null) {
                        fields.add(name, new JsonObject().add("state", "null"));
                    } else if (value instanceof Double) {
                        fields.add(name, (Double) value);
                    } else if (value instanceof Long) {
                        fields.add(name, (Long) value);
                    } else if (value instanceof Integer) {
                        fields.add(name, (Integer) value);
                    } else {
                        fields.add(name, value.toString());
                    }
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }
    }
}
