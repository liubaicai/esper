import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionSelectorSegmented;
import com.espertech.esper.common.client.context.ContextPartitionVariableState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.util.DeploymentIdNamePair;
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
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-key-segmented-allocation-time
 * parity scenario. Replays three ContextKeySegmented executions, each on a
 * fresh runtime with the clock pinned at zero:
 *
 * <ul>
 * <li>pattern-fire-when-allocated (ord 25
 * ContextKeySegmentedWPatternFireWhenAllocated): a keyed context partitioned
 * by theString where timer:interval(0) fires synchronously at partition
 * allocation — s0 delivers {key1:allocating-key} and the on-pattern trigger
 * sets the per-partition variable lastString=context.key1 in the same
 * allocation. Later events for the same key produce nothing; a new key
 * allocates a new partition and fires again. read-variable steps replay the
 * execution's getVariableValue(pair, SupportSelectorPartitioned(key))
 * assertion through a segmented selector carrying the partition key in
 * filterValue.</li>
 * <li>regex-filter (ord 28 ContextKeySegmentedRegExFilter): a map-typed
 * MyEventWPartition partitioned by partitionId terminated after 15 minutes;
 * the allocating event is evaluated against the statement's like "%hello%"
 * filter and outputs. The termination never fires because the scenario
 * carries no advance-time step.</li>
 * <li>subtype (ord 6 ContextKeySegmentedSubtype): the bean hierarchy
 * ISupportBaseAB{baseAB} &lt;- ISupportA{a} &lt;- ISupportAImpl is modeled
 * with map event types and supertype declarations (the oracle classpath
 * carries no regression-lib beans); ISupportAImpl events allocate partitions
 * through the ISupportBaseAB-declared baseAB key and count per partition
 * under the ISupportA-typed statement.</li>
 * </ul>
 *
 * <p>Java env.milestone calls are harness no-ops and carry no steps; the
 * send order preserves the assertion order exactly.
 */
public final class ContextKeySegmentedAllocationTimeScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private static final String EPL_PATTERN_FIRE_WHEN_ALLOCATED =
            "create context MyContext partition by theString from SupportBean;\n" +
            "@name('s0') context MyContext select context.key1 as key1 from pattern[timer:interval(0)];\n" +
            "context MyContext create variable String lastString = null;\n" +
            "context MyContext on pattern[timer:interval(0)] set lastString = context.key1;\n";

    private static final String EPL_REGEX_FILTER =
            "@public @buseventtype create schema MyEventWPartition as (number int, description string, partitionId string);\n" +
            "create context MyContext partition by partitionId from MyEventWPartition terminated after 15 minutes;\n" +
            "@name('s0') context MyContext select * from MyEventWPartition(description like \"%hello%\");\n";

    private static final String EPL_SUBTYPE =
            "@Name('context') create context SegmentedByString partition by baseAB from ISupportBaseAB;\n" +
            "@name('s0') context SegmentedByString select count(*) as col1 from ISupportA;\n";

    private ContextKeySegmentedAllocationTimeScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextKeySegmentedAllocationTimeScenarioOracle <scenario.json>");
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
        String epl;
        switch (caseName) {
            case "pattern-fire-when-allocated": {
                Map<String, Object> beanType = new HashMap<>();
                beanType.put("theString", String.class);
                beanType.put("intPrimitive", Integer.class);
                configuration.getCommon().addEventType("SupportBean", beanType);
                epl = EPL_PATTERN_FIRE_WHEN_ALLOCATED;
                break;
            }
            case "regex-filter":
                // MyEventWPartition is created by the module's own
                // @public @buseventtype create schema statement.
                epl = EPL_REGEX_FILTER;
                break;
            case "subtype": {
                // The bean hierarchy ISupportBaseAB{baseAB} <- ISupportA{a}
                // <- ISupportAImpl is modeled with map event types and
                // supertype declarations; the oracle classpath carries no
                // regression-lib beans.
                configuration.getCommon().addEventType("ISupportBaseAB",
                        Collections.singletonMap("baseAB", (Object) String.class));
                configuration.getCommon().addEventType("ISupportA",
                        Collections.singletonMap("a", (Object) String.class),
                        new String[]{"ISupportBaseAB"});
                configuration.getCommon().addEventType("ISupportAImpl",
                        Collections.emptyMap(),
                        new String[]{"ISupportA"});
                epl = EPL_SUBTYPE;
                break;
            }
            default:
                throw new IllegalArgumentException("unsupported case " + caseName);
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-key-segmented-allocation-time-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
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
                } else if ("read-variable".equals(op)) {
                    readVariable(runtime, deployment, caseName, step, records);
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
        switch (eventType) {
            case "SupportBean":
                event.put("theString", payload.getString("theString", ""));
                event.put("intPrimitive", payload.get("intPrimitive").asInt());
                break;
            case "MyEventWPartition":
                event.put("number", payload.get("number").asInt());
                event.put("description", payload.getString("description", ""));
                event.put("partitionId", payload.getString("partitionId", ""));
                break;
            case "ISupportAImpl":
                event.put("a", payload.getString("a", ""));
                event.put("baseAB", payload.getString("baseAB", ""));
                break;
            default:
                throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    /**
     * Replays the execution's getVariableValue(pair,
     * SupportSelectorPartitioned(theString)) assertion: the step's
     * filterValue carries the single partition key for the segmented
     * selector and the record mirrors the sibling oracles' variable shape.
     */
    private static void readVariable(EPRuntime runtime, EPDeployment deployment, String caseName,
                                     JsonObject step, JsonArray records) {
        String name = step.getString("name", "");
        String owner = step.getString("statement", "");
        String key = step.getString("filterValue", "");
        if (key.isEmpty()) {
            throw new IllegalStateException("read-variable " + name + " requires a partition key in filterValue");
        }
        DeploymentIdNamePair pair = new DeploymentIdNamePair(deployment.getDeploymentId(), name);
        ContextPartitionSelectorSegmented selector = new ContextPartitionSelectorSegmented() {
            @Override
            public List<Object[]> getPartitionKeys() {
                return Collections.singletonList(new Object[]{key});
            }
        };
        Map<DeploymentIdNamePair, List<ContextPartitionVariableState>> states =
                runtime.getVariableService().getVariableValue(Collections.singleton(pair), selector);
        List<ContextPartitionVariableState> list = states.get(pair);
        if (states.size() != 1 || list == null || list.size() != 1) {
            throw new IllegalStateException("variable states for " + name + " are not a single state");
        }
        JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "variable")
                .add("statement", owner)
                .add("sequence", 0)
                .add("name", name);
        Object value = list.get(0).getState();
        if (value == null) {
            record.add("value", new JsonObject().add("state", "null"));
        } else {
            record.add("value", Json.value(String.valueOf(value)));
        }
        records.add(record);
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

        private JsonValue eventFields(String[] names, java.util.function.Function<String, Object> getter) {
            JsonObject fields = new JsonObject();
            String[] sorted = names.clone();
            java.util.Arrays.sort(sorted);
            for (String name : sorted) {
                fields.add(name, normalize(getter.apply(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean) {
                return eventFields(((EventBean) value).getEventType().getPropertyNames(), key -> ((EventBean) value).get(key));
            }
            if (value instanceof Map) {
                Map<?, ?> map = (Map<?, ?>) value;
                String[] keys = map.keySet().toArray(new String[0]);
                java.util.Arrays.sort(keys);
                return eventFields(keys, key -> map.get(key));
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
