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
 * Direct Esper 9.0.0 oracle for rollup output-limit parity scenarios.
 * Case "last" mirrors ResultSetOutputLast{join=false}; case "last-sorted"
 * mirrors ResultSetOutputLastSorted{join=false}; case "first" mirrors
 * ResultSetOutputFirst{join=false}; case "first-sorted" mirrors
 * ResultSetOutputFirstSorted{join=false}; case "snapshot-order-limit"
 * mirrors ResultSetOutputSnapshotOrderWLimit; case "snapshot" mirrors
 * ResultSet6OutputLimitSnapshot{join=false}; case "last-market" mirrors
 * ResultSet4OutputLimitLast; case "first-market" mirrors
 * ResultSet5OutputLimitFirst; case "no-limit-market" mirrors
 * ResultSet1NoOutputLimit; case "default-market" mirrors
 * ResultSet2OutputLimitDefault; case "all" mirrors
 * ResultSetOutputAll{join=false}; case "all-sorted" mirrors
 * ResultSetOutputAllSorted{join=false}; case "first-having" mirrors
 * ResultSetOutputFirstHaving{join=false}; case "default" mirrors
 * ResultSetNoJoinDefault; case "last-aggregate" mirrors
 * ResultSetNoJoinLast; case "no-output" mirrors
 * ResultSetNoOutputClauseView; case "default-output" mirrors
 * ResultSet5DefaultNoHavingNoJoin; case "default-having" mirrors
 * ResultSet7DefaultHavingNoJoin.
 */
public final class RollupOutputLastScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private RollupOutputLastScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: RollupOutputLastScenarioOracle <scenario.json>");
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

        String[] cases = {"last", "last-sorted", "first", "first-sorted", "snapshot-order-limit", "snapshot", "last-market", "first-market", "no-limit-market", "default-market", "all", "all-sorted", "first-having", "default", "last-aggregate", "no-output", "default-output", "default-having"};
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
        beanType.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        if ("snapshot".equals(caseName) || "last-market".equals(caseName) || "first-market".equals(caseName) || "no-limit-market".equals(caseName) || "default-market".equals(caseName) || "default".equals(caseName) || "last-aggregate".equals(caseName) || "no-output".equals(caseName) || "default-output".equals(caseName) || "default-having".equals(caseName)) {
            Map<String, Object> marketType = new HashMap<>();
            marketType.put("symbol", String.class);
            marketType.put("volume", Long.class);
            marketType.put("price", Double.class);
            configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-rollup-output-last-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        String epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                "output last every 1 second";
        if ("last-sorted".equals(caseName)) {
            epl += " order by theString, intPrimitive";
        } else if ("first".equals(caseName)) {
            epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                    "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                    "output first every 1 second";
        } else if ("first-sorted".equals(caseName)) {
            epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                    "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                    "output first every 1 second order by theString, intPrimitive";
        } else if ("snapshot-order-limit".equals(caseName)) {
            epl = "@Name('s0') select theString as c0, sum(intPrimitive) as c1 " +
                    "from SupportBean group by rollup(theString) " +
                    "output snapshot every 1 seconds order by sum(intPrimitive) limit 3";
        } else if ("snapshot".equals(caseName)) {
            epl = "@Name('s0') select symbol, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) group by rollup(symbol) " +
                    "output snapshot every 1 seconds";
        } else if ("last-market".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) group by rollup(symbol) " +
                    "output last every 1 seconds";
        } else if ("first-market".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) group by rollup(symbol) " +
                    "output first every 1 seconds";
        } else if ("no-limit-market".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) group by rollup(symbol)";
        } else if ("default-market".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) group by rollup(symbol) " +
                    "output every 1 seconds";
        } else if ("all".equals(caseName)) {
            epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                    "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                    "output all every 1 second";
        } else if ("all-sorted".equals(caseName)) {
            epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                    "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                    "output all every 1 second order by theString, intPrimitive";
        } else if ("first-having".equals(caseName)) {
            epl = "@Name('s0') select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 " +
                    "from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) " +
                    "having sum(longBoxed) > 100 output first every 1 second";
        } else if ("default".equals(caseName)) {
            epl = "@Name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean#length(5) " +
                    "where symbol in ('DELL', 'IBM', 'GE') group by symbol output every 2 events";
        } else if ("last-aggregate".equals(caseName)) {
            epl = "@Name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean#length(5) " +
                    "where symbol in ('DELL', 'IBM', 'GE') group by symbol output last every 2 events";
        } else if ("no-output".equals(caseName)) {
            epl = "@Name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean#length(5) " +
                    "where symbol in ('DELL', 'IBM', 'GE') group by symbol";
        } else if ("default-output".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, volume, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) " +
                    "group by symbol output every 1 seconds";
        } else if ("default-having".equals(caseName)) {
            epl = "@Name('s0') select irstream symbol, volume, sum(price) " +
                    "from SupportMarketDataBean#time(5.5 sec) " +
                    "group by symbol having sum(price) > 50 output every 1 seconds";
        } else if (!"last".equals(caseName)) {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-rollup-output-last-" + caseName));
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
            } else if ("advance-time".equals(op)) {
                runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
            } else {
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportBean".equals(eventType)) {
            if (!"SupportMarketDataBean".equals(eventType)) {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            event.put("volume", payload.get("volume").asLong());
            event.put("price", payload.get("price").asDouble());
            runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
            return;
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("intPrimitive", payload.get("intPrimitive").asInt());
        event.put("longBoxed", payload.get("longBoxed").asLong());
        runtime.getEventService().sendEventMap(event, "SupportBean");
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
                    .add("sequence", sequence + 1)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = oldEvents == null ? new JsonArray() : results(oldEvents);
            if (newArray.size() == 0 && oldArray.size() == 0) {
                return;
            }
            sequence++;
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
