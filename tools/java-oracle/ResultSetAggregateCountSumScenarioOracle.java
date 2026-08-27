import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.GroupByClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.StreamSelector;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the resultset-aggregate-count-sum parity
 * scenario. Case "count-one-view" mirrors ResultSetAggregateCountOneView:
 * irstream grouped count(*)/count(distinct volume)/count(volume) over
 * SupportMarketDataBean#length(3) with the DELL/IBM sequence including null
 * volumes and duplicate values, exercising distinct-count recompute on window
 * expiry. Case "count-join" mirrors ResultSetAggregateCountJoin: the same
 * statement over a SupportBeanString#length(100) seed join. Case
 * "count-simple" mirrors ResultSetAggregateCountSimple: ungrouped count(*)
 * over SupportMarketDataBean#time(1). Case "sum-named-window-remove-group"
 * mirrors ResultSetAggregateSumNamedWindowRemoveGroup: a keepall named
 * window with insert and on-delete triggers feeding grouped sum with group
 * removal (null sum) and iterator snapshots. Cases "count-plus-star",
 * "count-having", "sum-having", "count-one-view-om" and "nested-avg"
 * mirror the five executions at source ordinals 1 through 5.
 */
public final class ResultSetAggregateCountSumScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ResultSetAggregateCountSumScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateCountSumScenarioOracle <scenario.json>");
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

        String[] cases = {"count-one-view", "count-join", "count-simple", "sum-named-window-remove-group",
                "count-plus-star", "count-having", "sum-having", "count-one-view-om", "nested-avg"};
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("volume", Long.class);
        marketType.put("price", Double.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        Map<String, Object> stringType = new HashMap<>();
        stringType.put("theString", String.class);
        configuration.getCommon().addEventType("SupportBeanString", stringType);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("longBoxed", Long.class);
        beanType.put("longPrimitive", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> aType = new HashMap<>();
        aType.put("id", String.class);
        configuration.getCommon().addEventType("SupportBean_A", aType);
        Map<String, Object> bType = new HashMap<>();
        bType.put("id", String.class);
        configuration.getCommon().addEventType("SupportBean_B", bType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-count-sum-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled;
            if ("count-one-view-om".equals(caseName)) {
                EPStatementObjectModel model = new EPStatementObjectModel();
                model.setSelectClause(SelectClause.create().streamSelector(StreamSelector.RSTREAM_ISTREAM_BOTH)
                        .add("symbol")
                        .add(Expressions.countStar(), "countAll")
                        .add(Expressions.countDistinct("volume"), "countDistVol")
                        .add(Expressions.count("volume"), "countVol"));
                model.setFromClause(FromClause.create(
                        FilterStream.create("SupportMarketDataBean").addView("length", Expressions.constant(3))));
                model.setWhereClause(Expressions.or()
                        .add(Expressions.eq("symbol", "DELL"))
                        .add(Expressions.eq("symbol", "IBM"))
                        .add(Expressions.eq("symbol", "GE")));
                model.setGroupByClause(GroupByClause.create("symbol"));
                model = SerializableObjectCopier.copyMayFail(model);

                String expected = "select irstream symbol, " +
                        "count(*) as countAll, " +
                        "count(distinct volume) as countDistVol, " +
                        "count(volume) as countVol" +
                        " from SupportMarketDataBean#length(3) " +
                        "where symbol=\"DELL\" or symbol=\"IBM\" or symbol=\"GE\" " +
                        "group by symbol";
                if (!expected.equals(model.toEPL())) {
                    throw new IllegalStateException("count-one-view-om model toEPL mismatch: " + model.toEPL());
                }
                model.setAnnotations(java.util.Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
                Module module = new Module();
                module.getItems().add(new ModuleItem(model));
                module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                String epl;
                if ("count-one-view".equals(caseName)) {
                    epl = "@Name('s0') select irstream symbol, " +
                            "count(*) as countAll, " +
                            "count(distinct volume) as countDistVol, " +
                            "count(all volume) as countVol" +
                            " from SupportMarketDataBean#length(3) " +
                            "where symbol='DELL' or symbol='IBM' or symbol='GE' " +
                            "group by symbol";
                } else if ("count-join".equals(caseName)) {
                    epl = "@Name('s0') select irstream symbol, " +
                            "count(*) as countAll, " +
                            "count(distinct volume) as countDistVol, " +
                            "count(volume) as countVol " +
                            " from SupportBeanString#length(100) as one, " +
                            "SupportMarketDataBean#length(3) as two " +
                            "where (symbol='DELL' or symbol='IBM' or symbol='GE') " +
                            "  and one.theString = two.symbol " +
                            "group by symbol";
                } else if ("count-simple".equals(caseName)) {
                    epl = "@Name('s0') select count(*) as cnt from SupportMarketDataBean#time(1)";
                } else if ("sum-named-window-remove-group".equals(caseName)) {
                    epl = "@name('create') create window MyWindow#keepall as select * from SupportBean;\n" +
                            "@name('insert') insert into MyWindow select * from SupportBean;\n" +
                            "@name('delete1') on SupportBean_A a delete from MyWindow w where w.theString = a.id;\n" +
                            "@name('delete2') on SupportBean_B delete from MyWindow;\n" +
                            "@Name('s0') select theString, sum(intPrimitive) as mysum from MyWindow group by theString order by theString";
                } else if ("count-plus-star".equals(caseName)) {
                    epl = "@name('s0') select *, count(*) as cnt from SupportMarketDataBean";
                } else if ("count-having".equals(caseName)) {
                    epl = "@name('s0') select irstream sum(intPrimitive) as mysum from SupportBean having sum(intPrimitive) = 2";
                } else if ("sum-having".equals(caseName)) {
                    epl = "@name('s0') select irstream count(*) as mysum from SupportBean having count(*) = 2";
                } else if ("nested-avg".equals(caseName)) {
                    epl = "@name('s0') select symbol, count(*) as cnt, avg(count(*)) as val from SupportMarketDataBean#length(3)" +
                            "group by symbol order by symbol asc";
                } else {
                    throw new IllegalArgumentException("unsupported case " + caseName);
                }
                compiled = EPCompilerProvider.getCompiler().compile(epl,
                        new CompilerArguments(runtime.getRuntimePath()));
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-count-sum-" + caseName));
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
            } else if ("snapshot".equals(op)) {
                writer.snapshot();
            } else {
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportMarketDataBean".equals(eventType)) {
            event.put("symbol", payload.getString("symbol", null));
            JsonValue volume = payload.get("volume");
            event.put("volume", volume.isNull() ? null : volume.asLong());
            event.put("price", payload.get("price").asDouble());
            event.put("feed", "f1");
        } else if ("SupportBeanString".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
        } else if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            event.put("longBoxed", payload.get("longBoxed").asLong());
            event.put("longPrimitive", payload.get("longPrimitive").asLong());
        } else if ("SupportBean_A".equals(eventType)) {
            event.put("id", payload.getString("id", null));
        } else if ("SupportBean_B".equals(eventType)) {
            event.put("id", payload.getString("id", null));
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
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
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
