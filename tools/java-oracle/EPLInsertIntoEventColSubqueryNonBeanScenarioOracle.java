import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Oracle for the EPLInsertIntoPopulateEventTypeColumnNonBean subquery-populated
 * non-bean (objectarray/map) event-type column executions:
 * FromSubquerySingle (objectarray/map x filter/no-filter),
 * FromSubqueryMulti (objectarray/map), and
 * FromSubqueryMultiFilter (objectarray/map with id-between-10-and-20).
 * The subquery projects named columns (p00 as e0_0, p01 as e0_1) from
 * SupportBean_S0 into an EventZero schema, which is then routed into an
 * event-typed column of EventOne. Internal timer disabled for deterministic
 * trace timestamps.
 */
public final class EPLInsertIntoEventColSubqueryNonBeanScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLInsertIntoEventColSubqueryNonBeanScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLInsertIntoEventColSubqueryNonBeanScenarioOracle <scenario.json>");
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

        String[] cases = {
            "nonbean-subquery-single-objectarray-nofilter",
            "nonbean-subquery-single-objectarray-filter",
            "nonbean-subquery-single-map-nofilter",
            "nonbean-subquery-single-map-filter",
            "nonbean-subquery-multi-objectarray-nofilter",
            "nonbean-subquery-multi-map-nofilter",
            "nonbean-subquery-multifilter-objectarray",
            "nonbean-subquery-multifilter-map"
        };
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
        boolean objectArray = caseName.contains("objectarray");
        boolean single = caseName.contains("-single-");
        boolean multiFilter = caseName.contains("-multifilter-");
        boolean filter = caseName.endsWith("-filter") || multiFilter;

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);

        // Register schemas programmatically for both objectarray and map.
        if (objectArray) {
            configuration.getCommon().addEventType("EventZero",
                new String[]{"e0_0", "e0_1"}, new Object[]{String.class, String.class});
            if (single) {
                configuration.getCommon().addEventType("EventOne",
                    new String[]{"ez"}, new Object[]{"EventZero"});
            } else if (multiFilter) {
                configuration.getCommon().addEventType("EventOne",
                    new String[]{"ez"}, new Object[]{"EventZero[]"});
            } else {
                configuration.getCommon().addEventType("EventOne",
                    new String[]{"e1_0", "ez"}, new Object[]{String.class, "EventZero[]"});
            }
        } else {
            Map<String, Object> zeroDef = new LinkedHashMap<>();
            zeroDef.put("e0_0", String.class);
            zeroDef.put("e0_1", String.class);
            configuration.getCommon().addEventType("EventZero", zeroDef);
            Map<String, Object> oneDef = new LinkedHashMap<>();
            if (single) {
                oneDef.put("ez", "EventZero");
            } else if (multiFilter) {
                oneDef.put("ez", "EventZero[]");
            } else {
                oneDef.put("e1_0", String.class);
                oneDef.put("ez", "EventZero[]");
            }
            configuration.getCommon().addEventType("EventOne", oneDef);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-nonbean-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = eplFor(caseName, single, filter, multiFilter);
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-nonbean-" + caseName));
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
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplFor(String caseName, boolean single, boolean filter, boolean multiFilter) {
        return statementEplFor(single, filter, multiFilter);
    }

    private static String schemaEplFor(String caseName, boolean single, boolean multiFilter) {
        if (!caseName.contains("-map-")) {
            return "";
        }
        StringBuilder sb = new StringBuilder();
        sb.append("@public create map schema EventZero(e0_0 string, e0_1 string);\n");
        if (single) {
            sb.append("@public create map schema EventOne(ez EventZero);\n");
        } else if (multiFilter) {
            sb.append("@public create map schema EventOne(ez EventZero[]);\n");
        } else {
            sb.append("@public create map schema EventOne(e1_0 string, ez EventZero[]);\n");
        }
        return sb.toString();
    }

    private static String statementEplFor(boolean single, boolean filter, boolean multiFilter) {
        StringBuilder sb = new StringBuilder();
        if (single) {
            String where = filter ? " where id >= 100" : "";
            sb.append("@name('s0') insert into EventOne select ")
              .append("(select p00 as e0_0, p01 as e0_1 from SupportBean_S0#lastevent")
              .append(where).append(") as ez from SupportBean;\n");
        } else if (multiFilter) {
            sb.append("@name('s0') insert into EventOne select ")
              .append("(select p00 as e0_0, p01 as e0_1 from SupportBean_S0#keepall where id between 10 and 20) as ez from SupportBean;\n");
        } else {
            sb.append("@name('s0') insert into EventOne select ")
              .append("theString as e1_0, ")
              .append("(select p00 as e0_0, p01 as e0_1 from SupportBean_S0#keepall) as ez from SupportBean;\n");
        }
        return sb.toString();
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
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
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean bean = new SupportBean(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBean_S0" -> {
                SupportBean_S0 bean = new SupportBean_S0(
                    payload.getInt("id", 0),
                    payload.getString("p00", null),
                    payload.getString("p01", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            default -> throw new IllegalArgumentException("unsupported event type " + eventType);
        }
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
            append(++sequence, newEvents, oldEvents);
        }

        private void append(long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
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
                    Object value;
                    try {
                        value = event.get(name);
                    } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                        continue;
                    }
                    fields.add(name, normalize(value));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean eventBean) {
                return normalizeEventBean(eventBean);
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
            if (value instanceof Map<?, ?> map) {
                JsonObject object = new JsonObject();
                List<String> keys = new ArrayList<>();
                for (Object key : map.keySet()) {
                    keys.add(String.valueOf(key));
                }
                java.util.Collections.sort(keys);
                for (String key : keys) {
                    object.add(key, normalize(map.get(key)));
                }
                return object;
            }
            if (value instanceof java.util.Collection<?> collection) {
                JsonArray array = new JsonArray();
                for (Object item : collection) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            return Json.value(String.valueOf(value));
        }

        private JsonValue normalizeEventBean(EventBean eventBean) {
            JsonObject object = new JsonObject();
            object.add("__type", eventBean.getEventType().getName());
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                Object value;
                try {
                    value = eventBean.get(name);
                } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                    continue;
                }
                object.add(name, normalize(value));
            }
            return object;
        }
    }
}
