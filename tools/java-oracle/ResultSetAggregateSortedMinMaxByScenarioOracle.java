import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for the first three sorted/minby executions. */
public final class ResultSetAggregateSortedMinMaxByScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-minmax-by";
    private static final String GROUPED = "grouped";
    private static final String OVERLAP = "overlap";
    private static final String OVER_WINDOW = "over-window";
    private static final String[] CASES = {GROUPED, OVERLAP, OVER_WINDOW};
    private static final int[] SEND_COUNTS = {7, 8, 10};

    private ResultSetAggregateSortedMinMaxByScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateSortedMinMaxByScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", "")) || !ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario");
        }
        JsonArray steps = scenario.get("steps").asArray();
        validateShape(steps);
        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        if (records.size() != 25) {
            throw new IllegalStateException("expected 25 listener records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        int expectedSteps = 0;
        for (int count : SEND_COUNTS) {
            expectedSteps += count + 1;
        }
        if (steps.size() != expectedSteps) {
            throw new IllegalArgumentException("scenario must contain the three cases and 25 sends");
        }
        int index = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = steps.get(index++).asObject();
            if (!"case".equals(marker.getString("op", ""))
                    || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            for (int eventIndex = 0; eventIndex < SEND_COUNTS[caseIndex]; eventIndex++) {
                JsonObject step = steps.get(index++).asObject();
                JsonValue payloadValue = step.get("payload");
                if (!"send".equals(step.getString("op", ""))
                        || !"SupportBean".equals(step.getString("eventType", ""))
                        || payloadValue == null || !payloadValue.isObject()) {
                    throw new IllegalArgumentException("scenario must contain SupportBean sends");
                }
                JsonObject payload = payloadValue.asObject();
                if (payload.size() != 3
                        || !payload.names().contains("theString")
                        || !payload.names().contains("intPrimitive")
                        || !payload.names().contains("longPrimitive")
                        || payload.get("theString").isNull()
                        || payload.get("intPrimitive").isNull()
                        || payload.get("longPrimitive").isNull()
                        || !payload.get("theString").isString()
                        || !payload.get("intPrimitive").isNumber()
                        || !payload.get("longPrimitive").isNumber()
                        || payload.getInt("intPrimitive", 0) != payload.getDouble("intPrimitive", 0)
                        || payload.getLong("longPrimitive", 0) != payload.getDouble("longPrimitive", 0)) {
                    throw new IllegalArgumentException("SupportBean payload must contain string and integer fields");
                }
            }
        }
    }

    private static void runCase(JsonArray steps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("longPrimitive", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        String runtimeURI = "parity-resultset-aggregate-sorted-minmax-by-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = epl(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replay(steps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String epl(String caseName) {
        switch (caseName) {
            case GROUPED:
                return "@name('s0') select window(*) as c0, "
                        + "sorted(intPrimitive desc) as c1, sorted(intPrimitive asc) as c2, "
                        + "maxby(intPrimitive) as c3, minby(intPrimitive) as c4, "
                        + "maxbyever(intPrimitive) as c5, minbyever(intPrimitive) as c6 "
                        + "from SupportBean#groupwin(longPrimitive)#length(3) group by longPrimitive";
            case OVERLAP:
                return "@name('s0') select "
                        + "maxbyever(intPrimitive).longPrimitive as c0,"
                        + "maxbyever(theString).longPrimitive as c1,"
                        + "minbyever(intPrimitive).longPrimitive as c2,"
                        + "minbyever(theString).longPrimitive as c3,"
                        + "maxby(intPrimitive).longPrimitive as c4,"
                        + "maxby(theString).longPrimitive as c5,"
                        + "minby(intPrimitive).longPrimitive as c6,"
                        + "minby(theString).longPrimitive as c7 "
                        + "from SupportBean#keepall";
            case OVER_WINDOW:
                return "@name('s0') select "
                        + "maxbyever(longPrimitive) as c0, minbyever(longPrimitive) as c1, "
                        + "maxby(longPrimitive).longPrimitive as c2, "
                        + "maxby(longPrimitive).theString as c3, "
                        + "maxby(longPrimitive).intPrimitive as c4, "
                        + "maxby(longPrimitive) as c5, "
                        + "minby(longPrimitive).longPrimitive as c6, "
                        + "minby(longPrimitive).theString as c7, "
                        + "minby(longPrimitive).intPrimitive as c8, "
                        + "minby(longPrimitive) as c9 "
                        + "from SupportBean#length(5)";
            default:
                throw new IllegalArgumentException("unsupported case " + caseName);
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replay(JsonArray steps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.getInt("intPrimitive", 0));
            event.put("longPrimitive", payload.getLong("longPrimitive", 0));
            runtime.getEventService().sendEventMap(event, "SupportBean");
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
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(eventValue(event));
            }
            return output;
        }

        private JsonObject eventValue(EventBean event) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean) {
                return eventValue((EventBean) value);
            }
            if (value instanceof Map) {
                Map<?, ?> map = (Map<?, ?>) value;
                List<String> names = new ArrayList<>();
                for (Object key : map.keySet()) {
                    names.add(String.valueOf(key));
                }
                names.sort(String::compareTo);
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(map.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[]) {
                JsonArray array = new JsonArray();
                for (Object item : (Object[]) value) {
                    array.add(normalize(item));
                }
                return array;
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
