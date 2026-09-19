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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-nested-initterm parity scenario.
 * Replays five ContextNested executions, each on a fresh runtime with the
 * clock pinned at zero:
 *
 * <ul>
 * <li>filter-overlap (ord 4 ContextNestedPartitionedWithFilterOverlap): a
 * segmented parent partitioned by SupportBean_S0.id with a non-overlapping
 * start/end child correlated on SupportBean_S1(id=te.id); the statement
 * joins S0#firstevent x S0#lastevent and delivers the firstEvent event to a
 * subscriber. The asserted observable is the delivered event's p00 sequence
 * A,B,B per iteration, including the silent re-termination no-op; the loop
 * runs twice.</li>
 * <li>filter-nonoverlap-broadcast (ord 5
 * ContextNestedPartitionedWithFilterNonOverlap): a filtered segmented
 * parent (intPrimitive &gt; 0) with an overlapping initiated child
 * terminated after 60 seconds. S0 initiations broadcast to every existing
 * parent partition and parent-stream events broadcast to all child leaves
 * under their parent, so SB("E1",6) emits two grouped-sum rows. Virtual
 * time is seeded to 2002-05-01T08:00:00 and never advanced, so the
 * termination never fires. The Java assertContextPropsNested descriptor
 * assertions are harness-only and carry no steps.</li>
 * <li>multikey-correlated-term (ord 17
 * ContextNestedPartitionWithMultiPropsAndTerm): a two-key segmented parent
 * with a start/end child whose end correlates on
 * id=e1.intPrimitive AND p00=e1.theString; output last when terminated
 * emits one row {"E1",0,2} while the (E2,1) partition stays open.</li>
 * <li>category-initterm-distinct (ord 33
 * ContextNestedCategoryOverInitTermDistinct): a category parent
 * (grp1/grp2/grp3 on intPrimitive) with a distinct-initiated filter child
 * terminated by SupportBean(theString='B'); the per-send cnt sequence
 * 1,2,1,1,(none),1,2,(none),1 proves distinct suppression, routed
 * termination and re-initiation.</li>
 * <li>initterm-endevent-projection (ord 34
 * ContextNestedKeySegmentedWInitTermEndEvent): a segmented parent with a
 * start/end child naming startevent/endevent; output all when terminated
 * emits one row {id,c0=startevent,c1=endevent} carrying both boundary
 * events.</li>
 * </ul>
 *
 * <p>Java env.milestone calls, repeated setSubscriber calls and the
 * JVM-internal filter-service-count assertion are harness no-ops and carry
 * no steps; the send order preserves the assertion order exactly. Event
 * types are declared as map types carrying the asserted properties,
 * consistent with the bean-representation conventions of this suite.
 */
public final class ContextNestedInitTermScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private static final String EPL_FILTER_OVERLAP_CONTEXT =
            "@Audit('pattern-instances') @public create context TheContext"
                    + " context CtxSession partition by id from SupportBean_S0, "
                    + " context CtxStartEnd start SupportBean_S0 as te end SupportBean_S1(id=te.id)";

    private static final String EPL_FILTER_OVERLAP_STATEMENT =
            "@name('s0') context TheContext select firstEvent from SupportBean_S0#firstevent() as firstEvent"
                    + " inner join SupportBean_S0#lastevent as lastEvent";

    private static final String EPL_FILTER_NONOVERLAP_CONTEXT =
            "@name('ctx') @public create context NestedContext as "
                    + "context SegByString as partition by theString from SupportBean(intPrimitive > 0), "
                    + "context InitCtx initiated by SupportBean_S0 as s0 terminated after 60 seconds";

    private static final String EPL_FILTER_NONOVERLAP_STATEMENT =
            "@name('s0') context NestedContext select "
                    + "context.InitCtx.s0.p00 as c0, theString as c1, sum(intPrimitive) as c2 from SupportBean group by theString";

    private static final String EPL_MULTIKEY_CONTEXT =
            "@public create context NestedContext "
                    + "context PartitionedByKeys partition by theString, intPrimitive from SupportBean, "
                    + "context InitiateAndTerm start SupportBean as e1 "
                    + "end SupportBean_S0(id=e1.intPrimitive and p00=e1.theString)";

    private static final String EPL_MULTIKEY_STATEMENT =
            "@name('s0') context NestedContext "
                    + "select theString as c0, intPrimitive as c1, count(longPrimitive) as c2 from SupportBean \n"
                    + "output last when terminated";

    private static final String EPL_CATEGORY_CONTEXT =
            "@public create context NestedContext "
                    + "context ACtx group by intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2, group by intPrimitive > 0 as grp3 from SupportBean, "
                    + "context BCtx initiated by distinct(a.intPrimitive) SupportBean(theString='A') as a terminated by SupportBean(theString='B') ";

    private static final String EPL_CATEGORY_STATEMENT =
            "@name('s0') context NestedContext select count(*) as cnt from SupportBean(intPrimitive = context.BCtx.a.intPrimitive and theString != 'B')";

    private static final String EPL_ENDEVENT =
            "create context MyContext "
                    + "context OuterContext partition by theString from SupportBean,\n"
                    + "context InnerContext start SupportBean(intPrimitive = 1) as startevent end SupportBean(intPrimitive = 0) as endevent;\n"
                    + "@name('s0') context MyContext select context.id as id, context.InnerContext.startevent as c0, context.InnerContext.endevent as c1 from SupportBean(intPrimitive > 0) output all when terminated;\n";

    private ContextNestedInitTermScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextNestedInitTermScenarioOracle <scenario.json>");
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
        configuration.getCompiler().getByteCode().setAllowSubscriber(true);
        List<String> modules = new ArrayList<>();
        boolean subscriber = false;
        switch (caseName) {
            case "filter-overlap":
                configuration.getCommon().addEventType("SupportBean_S0", supportBeanS0Type());
                configuration.getCommon().addEventType("SupportBean_S1", supportBeanS1Type());
                modules.add(EPL_FILTER_OVERLAP_CONTEXT);
                modules.add(EPL_FILTER_OVERLAP_STATEMENT);
                subscriber = true;
                break;
            case "filter-nonoverlap-broadcast":
                configuration.getCommon().addEventType("SupportBean", supportBeanType());
                configuration.getCommon().addEventType("SupportBean_S0", supportBeanS0Type());
                modules.add(EPL_FILTER_NONOVERLAP_CONTEXT);
                modules.add(EPL_FILTER_NONOVERLAP_STATEMENT);
                break;
            case "multikey-correlated-term":
                configuration.getCommon().addEventType("SupportBean", supportBeanType());
                configuration.getCommon().addEventType("SupportBean_S0", supportBeanS0Type());
                modules.add(EPL_MULTIKEY_CONTEXT);
                modules.add(EPL_MULTIKEY_STATEMENT);
                break;
            case "category-initterm-distinct":
                configuration.getCommon().addEventType("SupportBean", supportBeanType());
                modules.add(EPL_CATEGORY_CONTEXT);
                modules.add(EPL_CATEGORY_STATEMENT);
                break;
            case "initterm-endevent-projection":
                configuration.getCommon().addEventType("SupportBean", supportBeanType());
                modules.add(EPL_ENDEVENT);
                break;
            default:
                throw new IllegalArgumentException("unsupported case " + caseName);
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-nested-initterm-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            int index = caseIndex + 1;
            // sendTimeEvent seeds the runtime clock before the context
            // deploys; leading advance-time steps replay that ordering.
            while (index < allSteps.size()) {
                JsonObject step = allSteps.get(index).asObject();
                if (!"advance-time".equals(step.getString("op", ""))) {
                    break;
                }
                advanceTime(runtime, step);
                index++;
            }
            EPStatement statement = null;
            for (String epl : modules) {
                CompilerArguments arguments = new CompilerArguments(configuration);
                arguments.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, arguments);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                for (EPStatement candidate : deployment.getStatements()) {
                    if ("s0".equals(candidate.getName())) {
                        statement = candidate;
                    }
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 not found");
            }
            if (subscriber) {
                statement.setSubscriber(new TraceSubscriber(records, caseName, statement, runtime));
            } else {
                statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            }

            for (; index < allSteps.size(); index++) {
                JsonObject step = allSteps.get(index).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    send(runtime, step);
                } else if ("advance-time".equals(op)) {
                    advanceTime(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static Map<String, Object> supportBeanType() {
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("longPrimitive", Long.class);
        return beanType;
    }

    private static Map<String, Object> supportBeanS0Type() {
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("id", Integer.class);
        beanType.put("p00", String.class);
        return beanType;
    }

    private static Map<String, Object> supportBeanS1Type() {
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("id", Integer.class);
        beanType.put("p10", String.class);
        return beanType;
    }

    private static void advanceTime(EPRuntime runtime, JsonObject step) {
        runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        switch (eventType) {
            case "SupportBean":
                event.put("theString", payload.getString("theString", ""));
                event.put("intPrimitive", payload.get("intPrimitive").asInt());
                event.put("longPrimitive", payload.get("longPrimitive") == null ? 0L : payload.get("longPrimitive").asLong());
                break;
            case "SupportBean_S0":
                event.put("id", payload.get("id").asInt());
                event.put("p00", payload.get("p00") == null ? null : payload.get("p00").asString());
                break;
            case "SupportBean_S1":
                event.put("id", payload.get("id").asInt());
                event.put("p10", payload.get("p10") == null ? null : payload.get("p10").asString());
                break;
            default:
                throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static JsonArray results(EventBean[] events) {
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

    private static JsonValue eventFields(String[] names, java.util.function.Function<String, Object> getter) {
        JsonObject fields = new JsonObject();
        String[] sorted = names.clone();
        java.util.Arrays.sort(sorted);
        for (String name : sorted) {
            fields.add(name, normalize(getter.apply(name)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue normalize(Object value) {
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
    }

    /**
     * Subscriber for the filter-overlap case's s0: the Java execution binds
     * SupportSubscriber's update(Object[], Object[]) signature, which
     * receives one element per delivered row. For the single-column
     * `select firstEvent` projection each element is the selected event's
     * underlying value; the record renders it under the firstEvent column
     * name, matching the Go subscriber's row shape.
     */
    public static final class TraceSubscriber {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceSubscriber(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        public void update(Object[] newData, Object[] oldData) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "subscriber")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = rows(newData);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldData != null && oldData.length > 0) {
                record.add("old", rows(oldData));
            }
            records.add(record);
        }

        private JsonArray rows(Object[] data) {
            JsonArray output = new JsonArray();
            if (data == null) {
                return output;
            }
            for (Object row : data) {
                Object value = row;
                // A one-element Object[] row unwraps to the same single
                // column value Esper delivers directly for this select.
                if (value instanceof Object[] && ((Object[]) value).length == 1) {
                    value = ((Object[]) value)[0];
                }
                JsonObject fields = new JsonObject();
                fields.add("firstEvent", normalize(value));
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }
    }
}
