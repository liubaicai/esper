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
 * Direct Esper 9.0.0 oracle for the subselect-aggregated-single-value parity
 * scenario. Each case mirrors one execution of
 * EPLSubselectAggregatedSingleValue: SupportBean_S0/S1 trigger events project
 * single-value aggregate subselects over SupportBean (unbound, keepall or
 * length(3), uncorrelated or correlated through s0.p00/s0.id, with grouped
 * or ungrouped having), plus table-backed subselects with a having clause.
 */
public final class EPLSubselectAggregatedSingleValueScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLSubselectAggregatedSingleValueScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLSubselectAggregatedSingleValueScenarioOracle <scenario.json>");
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

        String[] cases = {"no-data-window", "having", "in-select-clause", "where-clause",
                "correlated-scene-two", "correlated-in-where-a", "correlated-in-where-b",
                "correlated-having", "grouped-uncorrelated-having", "grouped-correlated-having",
                "grouped-correlation-inside-having", "ungrouped-correlation-inside-having",
                "ungrouped-table-having", "grouped-table-having",
                "join-3stream-key-range-between", "join-3stream-key-range-no-reversal",
                "join-3stream-key-range-greater-than", "join-3stream-key-range-less-than",
                "join-2stream-range-between-s0-s1", "join-2stream-range-between-s1-s0",
                "join-2stream-range-no-reversal"};
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
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        configuration.getCommon().addEventType("SupportBean_S1", s1Type);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> st0Type = new HashMap<>();
        st0Type.put("id", String.class);
        st0Type.put("p01Long", Long.class);
        configuration.getCommon().addEventType("SupportBean_ST0", st0Type);
        Map<String, Object> st1Type = new HashMap<>();
        st1Type.put("id", String.class);
        st1Type.put("p11Long", Long.class);
        configuration.getCommon().addEventType("SupportBean_ST1", st1Type);
        Map<String, Object> st2Type = new HashMap<>();
        st2Type.put("id", String.class);
        st2Type.put("key2", String.class);
        st2Type.put("p20", Integer.class);
        configuration.getCommon().addEventType("SupportBean_ST2", st2Type);

        String epl;
        if ("no-data-window".equals(caseName)) {
            epl = "@name('s0') select p00 as c0, (select sum(intPrimitive) from SupportBean) as c1 from SupportBean_S0";
        } else if ("having".equals(caseName)) {
            epl = "@name('s0') select *, " +
                    "(select sum(intPrimitive) from SupportBean#keepall having sum(intPrimitive) > 100) as c0," +
                    "exists (select sum(intPrimitive) from SupportBean#keepall having sum(intPrimitive) > 100) as c1 " +
                    "from SupportBean_S0";
        } else if ("in-select-clause".equals(caseName)) {
            epl = "@name('s0') select (select s0.id + max(s1.id) from SupportBean_S1#length(3) as s1) as value from SupportBean_S0 as s0";
        } else if ("where-clause".equals(caseName)) {
            epl = "@name('s0') select (select sum(id) from SupportBean_S1#length(3) where id < 0) as value from SupportBean_S0";
        } else if ("correlated-scene-two".equals(caseName)) {
            epl = "@name('s0') select id, (select count(*) from SupportBean_S1#length(3) s1 where s1.p10 = s0.p00) as mycount from SupportBean_S0 s0";
        } else if ("correlated-in-where-a".equals(caseName)) {
            epl = "@name('s0') select p00 from SupportBean_S0 as s0 where id > " +
                    "(select sum(intPrimitive) from SupportBean#keepall where theString = s0.p00)";
        } else if ("correlated-in-where-b".equals(caseName)) {
            epl = "@name('s0') select p00 from SupportBean_S0 as s0 where id > " +
                    "(select sum(intPrimitive) from SupportBean#keepall where theString||'X' = s0.p00||'X')";
        } else if ("correlated-having".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) from SupportBean#keepall where theString = s0.p00 having sum(intPrimitive) > 10) as c0 from SupportBean_S0 as s0";
        } else if ("grouped-uncorrelated-having".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) from SupportBean#keepall group by theString having sum(intPrimitive) > 10) as c0 from SupportBean_S0 as s0";
        } else if ("grouped-correlated-having".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) from SupportBean#keepall where s0.id = intPrimitive group by theString having sum(intPrimitive) > 10) as c0 from SupportBean_S0 as s0";
        } else if ("grouped-correlation-inside-having".equals(caseName)) {
            epl = "@name('s0') select (select theString from SupportBean#keepall group by theString having sum(intPrimitive) = s0.id) as c0 from SupportBean_S0 as s0";
        } else if ("ungrouped-correlation-inside-having".equals(caseName)) {
            epl = "@name('s0') select (select last(theString) from SupportBean#keepall having sum(intPrimitive) = s0.id) as c0 from SupportBean_S0 as s0";
        } else if ("join-3stream-key-range-between".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where theString = st2.key2 and intPrimitive between s0.p01Long and s1.p11Long) " +
                    "from SupportBean_ST2#lastevent st2, SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("join-3stream-key-range-no-reversal".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where theString = st2.key2 and s1.p11Long >= intPrimitive and s0.p01Long <= intPrimitive) " +
                    "from SupportBean_ST2#lastevent st2, SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("join-3stream-key-range-greater-than".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where theString = st2.key2 and s1.p11Long > intPrimitive) " +
                    "from SupportBean_ST2#lastevent st2, SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("join-3stream-key-range-less-than".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where theString = st2.key2 and s1.p11Long < intPrimitive) " +
                    "from SupportBean_ST2#lastevent st2, SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("join-2stream-range-between-s0-s1".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where intPrimitive between s0.p01Long and s1.p11Long) " +
                    "from SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("join-2stream-range-between-s1-s0".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where intPrimitive between s1.p11Long and s0.p01Long) " +
                    "from SupportBean_ST1#lastevent s1, SupportBean_ST0#lastevent s0";
        } else if ("join-2stream-range-no-reversal".equals(caseName)) {
            epl = "@name('s0') select (select sum(intPrimitive) as sumi from SupportBean#keepall where intPrimitive >= s0.p01Long and intPrimitive <= s1.p11Long) " +
                    "from SupportBean_ST0#lastevent s0, SupportBean_ST1#lastevent s1";
        } else if ("ungrouped-table-having".equals(caseName)) {
            epl = "@public create table MyTable(total sum(int));\n" +
                    "@name('into') into table MyTable select sum(intPrimitive) as total from SupportBean;\n" +
                    "@name('s0') select (select sum(total) from MyTable having sum(total) > 100) as c0 from SupportBean_S0";
        } else if ("grouped-table-having".equals(caseName)) {
            epl = "@public create table MyTableWith2Keys(k1 string primary key, k2 string primary key, total sum(int));\n" +
                    "@name('into') into table MyTableWith2Keys select p10 as k1, p11 as k2, sum(id) as total from SupportBean_S1 group by p10, p11;\n" +
                    "@name('s0') select (select sum(total) from MyTableWith2Keys group by k1 having sum(total) > 100) as c0 from SupportBean_S0";
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-subselect-single-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-subselect-single-" + caseName));
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
        } else if ("SupportBean_S1".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            JsonValue p10 = payload.get("p10");
            event.put("p10", p10.isNull() ? null : p10.asString());
            JsonValue p11 = payload.get("p11");
            event.put("p11", p11.isNull() ? null : p11.asString());
        } else if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
        } else if ("SupportBean_ST0".equals(eventType)) {
            event.put("id", payload.getString("id", null));
            JsonValue p01Long = payload.get("p01Long");
            event.put("p01Long", p01Long.isNull() ? null : p01Long.asLong());
        } else if ("SupportBean_ST1".equals(eventType)) {
            event.put("id", payload.getString("id", null));
            JsonValue p11Long = payload.get("p11Long");
            event.put("p11Long", p11Long.isNull() ? null : p11Long.asLong());
        } else if ("SupportBean_ST2".equals(eventType)) {
            event.put("id", payload.getString("id", null));
            JsonValue key2 = payload.get("key2");
            event.put("key2", key2.isNull() ? null : key2.asString());
            event.put("p20", payload.get("p20").asInt());
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
