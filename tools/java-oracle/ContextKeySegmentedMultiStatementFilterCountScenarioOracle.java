import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierPartitioned;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-key-segmented-multi-statement-filter-count parity
 * scenario. Mirrors ContextKeySegmentedViewSceneOne (keyed context with a
 * per-partition length(2) window projecting prevwindow, irstream new/old and
 * iterator) and ContextKeySegmentedViewSceneTwo (keyed context with a
 * per-partition lastevent window producing new/old replacement pairs).
 */
public final class ContextKeySegmentedMultiStatementFilterCountScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextKeySegmentedMultiStatementFilterCountScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextKeySegmentedMultiStatementFilterCountScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-keyed-multi-statement", configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl;
            String listenerStatement;
            switch (caseName) {
                case "multi-statement":
                    epl = "@Name('context') @public create context SegmentedByAString " +
                            "partition by theString from SupportBean, p00 from SupportBean_S0;\n" +
                            "@Name('s0') context SegmentedByAString select sum(id) as col1 from SupportBean_S0;\n" +
                            "@Name('s1') context SegmentedByAString select sum(intPrimitive) as col1 from SupportBean";
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
            EPStatement statementS1 = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s1".equals(candidate.getName())) {
                    statementS1 = candidate;
                    break;
                }
            }
            if (statementS1 == null) {
                throw new IllegalStateException("statement s1 not found");
            }
            TraceWriter writerS1 = new TraceWriter(records, caseName, statementS1, runtime);
            statementS1.addListener(writerS1);

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
                } else if ("snapshot".equals(op)) {
                    writer.snapshot("snapshot");
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
        String eventType = step.getString("eventType", "SupportBean");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", ""));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", payload.getString("p00", ""));
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
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement source, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", source.getName())
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

        private void snapshot(String operation) {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(events.toArray(new EventBean[0]));
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            record.add("partitions", partitions());
            records.add(record);
        }

        private JsonArray partitions() {
            ContextPartitionCollection collection = runtime.getContextPartitionService().getContextPartitions(
                    statement.getDeploymentId(), "PartitionedByString", ContextPartitionSelectorAll.INSTANCE);
            List<Map.Entry<Integer, ContextPartitionIdentifier>> entries = new ArrayList<>(collection.getIdentifiers().entrySet());
            entries.sort(java.util.Comparator.comparingInt(Map.Entry::getKey));
            JsonArray output = new JsonArray();
            for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
                ContextPartitionIdentifierPartitioned id = (ContextPartitionIdentifierPartitioned) entry.getValue();
                JsonObject properties = new JsonObject();
                output.add(new JsonObject()
                        .add("id", entry.getKey())
                        .add("key", "key:" + id.getKeys()[0])
                        .add("properties", properties));
            }
            return output;
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
            if (value instanceof EventBean[]) {
                EventBean[] events = (EventBean[]) value;
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    JsonObject fields = new JsonObject();
                    String[] names = event.getEventType().getPropertyNames().clone();
                    java.util.Arrays.sort(names);
                    for (String name : names) {
                        fields.add(name, normalize(event.get(name)));
                    }
                    array.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
                return array;
            }
            if (value instanceof Map[]) {
                Map<?, ?>[] maps = (Map<?, ?>[]) value;
                JsonArray array = new JsonArray();
                for (Map<?, ?> map : maps) {
                    JsonObject fields = new JsonObject();
                    java.util.List<String> names = new ArrayList<>();
                    for (Object key : map.keySet()) {
                        names.add(String.valueOf(key));
                    }
                    java.util.Collections.sort(names);
                    for (String name : names) {
                        fields.add(name, normalize(map.get(name)));
                    }
                    array.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
                return array;
            }
            if (value instanceof Object[]) {
                Object[] objects = (Object[]) value;
                JsonArray array = new JsonArray();
                for (Object element : objects) {
                    array.add(normalize(element));
                }
                return array;
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
