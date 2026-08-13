import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierHash;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
import com.espertech.esper.common.client.context.ContextPartitionSelectorHash;
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
import java.util.Comparator;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the shared ContextHash parity scenario.
 *
 * The runner intentionally uses the public compiler, runtime, statement
 * iterator and context administration APIs. It does not invoke regression
 * assertions, so the output is an independent trace artifact.
 */
public final class ContextHashScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextHashScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextHashScenarioOracle <scenario.json>");
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

        String[] cases = {"no-preallocate", "many-arg-crc32", "many-arg-hash-code", "partition-selection"};
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
        configuration.getCommon().addEventType("SupportBean", Class.forName("com.espertech.esper.common.internal.support.SupportBean"));
        if (caseName.startsWith("many-arg")) {
            configuration.getCommon().addEventType("SupportBean_S0", Class.forName("com.espertech.esper.common.internal.support.SupportBean_S0"));
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        String contextName;
        String epl;
        if ("no-preallocate".equals(caseName)) {
            contextName = "CtxHash";
            epl = "@name('ctx') create context CtxHash coalesce by consistent_hash_crc32(theString) from SupportBean granularity 16;" +
                    "@name('s0') context CtxHash select context.id as c0, theString as c1, sum(intPrimitive) as c2 from SupportBean group by theString;";
        } else if ("many-arg-crc32".equals(caseName) || "many-arg-hash-code".equals(caseName)) {
            contextName = "Ctx1";
            String hash = "many-arg-crc32".equals(caseName) ? "consistent_hash_crc32" : "hash_code";
            epl = "@name('ctx') create context Ctx1 as coalesce " + hash + "(theString, intPrimitive) from SupportBean granularity 1000000;" +
                    "@name('s0') context Ctx1 select intPrimitive as c1, sum(longPrimitive) as c2, prev(1, longPrimitive) as c3, prior(1, longPrimitive) as c4," +
                    "(select p00 from SupportBean_S0#length(2)) as c5 from SupportBean#length(3);";
        } else if ("partition-selection".equals(caseName)) {
            contextName = "MyCtx";
            epl = "@name('ctx') create context MyCtx as coalesce consistent_hash_crc32(theString) from SupportBean granularity 16 preallocate;" +
                    "@name('s0') context MyCtx select context.id as c0, theString as c1, sum(intPrimitive) as c2 from SupportBean#keepall group by theString;";
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-" + caseName));
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
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime, contextName);
            statement.addListener(writer);
            replayCase(allSteps, caseName, runtime, statement, writer, contextName);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, EPStatement statement,
                                   TraceWriter writer, String contextName) {
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
            } else if ("snapshot".equals(op)) {
                if (!statement.getName().equals(step.getString("statement", ""))) {
                    throw new IllegalArgumentException("unknown statement " + step.getString("statement", ""));
                }
                writer.snapshot("snapshot", null);
            } else if ("snapshot-selector".equals(op)) {
                if (!statement.getName().equals(step.getString("statement", ""))) {
                    throw new IllegalArgumentException("unknown statement " + step.getString("statement", ""));
                }
                String selector = step.getString("selector", "");
                if (!"hashes".equals(selector) && !"all".equals(selector)) {
                    throw new IllegalArgumentException("unsupported selector " + selector);
                }
                writer.snapshot("snapshot-selector", "all".equals(selector) ? null : hashSelector(step));
            }
        }
    }

    private static ContextPartitionSelectorHash hashSelector(JsonObject step) {
        Set<Integer> hashes = new HashSet<>();
        JsonArray values = step.get("hashes").asArray();
        for (int i = 0; i < values.size(); i++) {
            hashes.add(values.get(i).asInt());
        }
        return new ContextPartitionSelectorHash() {
            @Override
            public Set<Integer> getHashes() {
                return hashes;
            }
        };
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            try {
                Class<?> type = Class.forName("com.espertech.esper.common.internal.support.SupportBean");
                Object event = type.getDeclaredConstructor().newInstance();
                type.getMethod("setTheString", String.class).invoke(event, payload.getString("theString", null));
                type.getMethod("setIntPrimitive", int.class).invoke(event, payload.get("intPrimitive").asInt());
                if (payload.get("longPrimitive") != null) {
                    type.getMethod("setLongPrimitive", long.class).invoke(event, payload.get("longPrimitive").asLong());
                }
                runtime.getEventService().sendEventBean(event, eventType);
                return;
            } catch (ReflectiveOperationException error) {
                throw new IllegalStateException("cannot build SupportBean", error);
            }
        }
        if ("SupportBean_S0".equals(eventType)) {
            try {
                Class<?> type = Class.forName("com.espertech.esper.common.internal.support.SupportBean_S0");
                Object event = type.getConstructor(int.class, String.class).newInstance(
                        payload.get("id").asInt(), payload.getString("p00", null));
                runtime.getEventService().sendEventBean(event, eventType);
                return;
            } catch (ReflectiveOperationException error) {
                throw new IllegalStateException("cannot build SupportBean_S0", error);
            }
        }
        throw new IllegalArgumentException("unsupported event type " + eventType);
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String contextName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime, String contextName) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.contextName = contextName;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            append("listener", ++sequence, newEvents, oldEvents, null);
        }

        private void snapshot(String operation, ContextPartitionSelector selector) {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = selector == null ? statement.iterator() : statement.iterator(selector);
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append(operation, 0, events.toArray(new EventBean[0]), null, selector);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents,
                            ContextPartitionSelector selector) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", timeString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = results(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            record.add("partitions", partitions(selector));
            records.add(record);
        }

        private String timeString() {
            return Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString();
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

        private JsonArray partitions(ContextPartitionSelector selector) {
            ContextPartitionSelector effective = selector == null ? ContextPartitionSelectorAll.INSTANCE : selector;
            ContextPartitionCollection collection = runtime.getContextPartitionService().getContextPartitions(
                    statement.getDeploymentId(), contextName, effective);
            List<Map.Entry<Integer, ContextPartitionIdentifier>> entries = new ArrayList<>(collection.getIdentifiers().entrySet());
            entries.sort(Comparator.comparingInt(Map.Entry::getKey));
            JsonArray output = new JsonArray();
            for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
                if (!(entry.getValue() instanceof ContextPartitionIdentifierHash)) {
                    continue;
                }
                int hash = ((ContextPartitionIdentifierHash) entry.getValue()).getHash();
                output.add(new JsonObject()
                        .add("id", entry.getKey())
                        .add("key", "hash:" + hash)
                        .add("properties", new JsonObject().add("hash", hash)));
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
