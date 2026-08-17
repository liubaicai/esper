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
 * Oracle for the EPLInsertIntoPopulateEventTypeColumnBean subquery-populated
 * bean event-type column executions: FromSubquerySingle (objectarray/map x
 * filter/no-filter) and FromSubqueryMulti (objectarray/map x filter/no-filter
 * where the filter is a no-op where 1=1). Registered event types use the real
 * SupportBean/SupportBean_S0 bean classes from the fixed esper-common
 * classpath so trace values reflect the original bean assertions exactly.
 * Internal timer disabled so trace timestamps are fixed at epoch zero.
 */
public final class EPLInsertIntoEventColSubqueryBeanScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLInsertIntoEventColSubqueryBeanScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLInsertIntoEventColSubqueryBeanScenarioOracle <scenario.json>");
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
            "bean-subquery-single-objectarray-nofilter",
            "bean-subquery-single-objectarray-filter",
            "bean-subquery-single-map-nofilter",
            "bean-subquery-single-map-filter",
            "bean-subquery-multi-objectarray-nofilter",
            "bean-subquery-multi-objectarray-filter",
            "bean-subquery-multi-map-nofilter",
            "bean-subquery-multi-map-filter"
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
        boolean single = caseName.contains("single");
        boolean filter = caseName.endsWith("-filter");

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);

        if (objectArray) {
            if (single) {
                configuration.getCommon().addEventType("EventOne",
                    new String[]{"sb"}, new Object[]{SupportBean_S0.class});
            } else {
                configuration.getCommon().addEventType("EventOne",
                    new String[]{"sbarr"}, new Object[]{SupportBean_S0[].class});
            }
        } else {
            Map<String, Object> def = new LinkedHashMap<>();
            def.put(single ? "sb" : "sbarr", single ? SupportBean_S0.class : SupportBean_S0[].class);
            configuration.getCommon().addEventType("EventOne", def);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-eventcol-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            List<String> epls = eplsFor(single, filter);
            EPStatement statement = null;
            int deploymentIndex = 0;
            for (String epl : epls) {
                CompilerArguments compilerArguments = new CompilerArguments(configuration);
                compilerArguments.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId("parity-eventcol-" + caseName + "-" + (deploymentIndex++)));
                if (statement == null) {
                    for (EPStatement candidate : deployment.getStatements()) {
                        if ("s0".equals(candidate.getName())) {
                            statement = candidate;
                            break;
                        }
                    }
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

    private static List<String> eplsFor(boolean single, boolean filter) {
        List<String> epls = new ArrayList<>();
        if (single) {
            String where = filter ? " where id >= 100" : "";
            epls.add("@name('s0') insert into EventOne select " +
                "(select * from SupportBean_S0#length(2)" + where + ") as sb from SupportBean");
        } else {
            String where = filter ? " where 1=1" : "";
            epls.add("@name('s0') @public insert into EventOne select " +
                "(select * from SupportBean_S0#keepall" + where + ") as sbarr from SupportBean");
        }
        return epls;
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
            if (value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            // Bean underlyings (SupportBean_S0 fragments) normalize to their
            // observable properties so Java/Go traces compare field-for-field.
            if (value instanceof SupportBean_S0 bean) {
                JsonObject object = new JsonObject();
                object.add("__type", "SupportBean_S0");
                object.add("id", normalize(bean.getId()));
                object.add("p00", normalize(bean.getP00()));
                object.add("p01", normalize(bean.getP01()));
                return object;
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
