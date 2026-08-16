import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierInitiatedTerminated;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
import com.espertech.esper.common.client.context.ContextPartitionSelectorFiltered;
import com.espertech.esper.common.client.context.ContextPartitionSelectorById;
import com.espertech.esper.common.client.context.ContextPartitionSelectorSegmented;
import com.espertech.esper.common.client.context.InvalidContextPartitionSelector;
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
import java.util.Collections;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-init-term-partition-selection
 * parity scenario. Mirrors ContextInitTermContextPartitionSelection: a keyed
 * initiated-terminated context with two concurrent partitions, a grouped
 * keepall aggregate reading context.id and the initiating event property,
 * iterator snapshots by partition ID and by an initiating-event filtered
 * selector, an always-false filtered selector that still observes every
 * partition, and an invalid segmented selector rejected with
 * InvalidContextPartitionSelector.
 */
public final class ContextInitTermPartitionSelectionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermPartitionSelectionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermPartitionSelectionScenarioOracle <scenario.json>");
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
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-partition-selection", configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = "@name('ctx') @public create context MyCtx as initiated by SupportBean_S0 s0 terminated by SupportBean_S1(id=s0.id);\n" +
                    "@name('s0') context MyCtx select context.id as c0, context.s0.p00 as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
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
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime, "MyCtx");
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
                } else if ("snapshot".equals(op)) {
                    writer.snapshot("snapshot", null);
                } else if ("snapshot-selector".equals(op)) {
                    String selector = step.getString("selector", "");
                    if ("ids".equals(selector)) {
                        JsonArray ids = step.get("ids").asArray();
                        writer.snapshot("snapshot-selector", new ContextPartitionSelectorById() {
                            public java.util.Set<Integer> getContextPartitionIds() {
                                java.util.Set<Integer> set = new java.util.LinkedHashSet<>();
                                for (JsonValue value : ids) {
                                    set.add(value.asInt());
                                }
                                return set;
                            }
                        });
                    } else if ("filtered".equals(selector)) {
                        String property = step.getString("filterProperty", "");
                        String value = step.getString("filterValue", "");
                        writer.snapshot("snapshot-selector", new ContextPartitionSelectorFiltered() {
                            public boolean filter(ContextPartitionIdentifier identifier) {
                                ContextPartitionIdentifierInitiatedTerminated id = (ContextPartitionIdentifierInitiatedTerminated) identifier;
                                Object initiating = id.getProperties().get("s0");
                                String p00 = initiating == null ? null : (String) ((EventBean) initiating).get("p00");
                                return value != null && value.equals(p00);
                            }
                        });
                    } else if ("segmented".equals(selector)) {
                        try {
                            statement.iterator(new ContextPartitionSelectorSegmented() {
                                public List<Object[]> getPartitionKeys() {
                                    return null;
                                }
                            });
                            throw new IllegalStateException("expected InvalidContextPartitionSelector");
                        } catch (InvalidContextPartitionSelector expected) {
                            writer.selectorError(expected.getMessage());
                        }
                    } else {
                        throw new IllegalArgumentException("unsupported selector " + selector);
                    }
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

        private void snapshot(String operation, ContextPartitionSelector selector) {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = selector == null ? statement.iterator() : statement.iterator(selector);
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
            record.add("partitions", partitions(selector));
            records.add(record);
        }

        private void selectorError(String message) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "selector-error")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("value", message.startsWith("Invalid context partition selector") ? "invalid-context-partition-selector" : message);
            records.add(record);
        }

        private JsonArray partitions(ContextPartitionSelector selector) {
            ContextPartitionSelector effective = selector == null ? ContextPartitionSelectorAll.INSTANCE : selector;
            ContextPartitionCollection collection = runtime.getContextPartitionService().getContextPartitions(
                    statement.getDeploymentId(), contextName, effective);
            List<Map.Entry<Integer, ContextPartitionIdentifier>> entries = new ArrayList<>(collection.getIdentifiers().entrySet());
            entries.sort(java.util.Comparator.comparingInt(Map.Entry::getKey));
            JsonArray output = new JsonArray();
            for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
                ContextPartitionIdentifierInitiatedTerminated id = (ContextPartitionIdentifierInitiatedTerminated) entry.getValue();
                JsonObject properties = new JsonObject()
                        .add("startTime", id.getStartTime())
                        .add("endTime", id.getEndTime() == null ? Json.NULL : Json.value(id.getEndTime()));
                Object initiating = id.getProperties().get("s0");
                if (initiating != null) {
                    properties.add("initiating.p00", normalize(((EventBean) initiating).get("p00")));
                }
                output.add(new JsonObject()
                        .add("id", entry.getKey())
                        .add("key", "start:" + id.getStartTime())
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
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Long) {
                return Json.value(((Long) value).longValue());
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
