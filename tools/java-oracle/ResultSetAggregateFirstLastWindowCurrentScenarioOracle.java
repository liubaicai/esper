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
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the current-window first/last/window access
 * executions in ResultSetAggregateFirstLastWindow.
 */
public final class ResultSetAggregateFirstLastWindowCurrentScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-firstlastwindow-current";
    private static final String NO_GROUP = "no-group";
    private static final String GROUP = "group";
    private static final String[] CASES = {NO_GROUP, GROUP};

    private ResultSetAggregateFirstLastWindowCurrentScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateFirstLastWindowCurrentScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id");
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray() || stepsValue.asArray().size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateShape(steps);

        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        int nextCase = 0;
        int noGroupSends = 0;
        int groupSends = 0;
        String active = "";
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                if (nextCase >= CASES.length || !CASES[nextCase].equals(step.getString("case", ""))) {
                    throw new IllegalArgumentException("cases must appear once in source order");
                }
                active = CASES[nextCase++];
                continue;
            }
            if (!"send".equals(op)) {
                throw new IllegalArgumentException("unsupported operation: " + op);
            }
            if (!"SupportBean".equals(step.getString("eventType", ""))) {
                throw new IllegalArgumentException("unsupported event type");
            }
            JsonValue payload = step.get("payload");
            if (payload == null || !payload.isObject()) {
                throw new IllegalArgumentException("send payload is required");
            }
            if (NO_GROUP.equals(active)) {
                noGroupSends++;
            } else if (GROUP.equals(active)) {
                groupSends++;
            } else {
                throw new IllegalArgumentException("send appears before a case");
            }
        }
        if (nextCase != CASES.length || noGroupSends != 4 || groupSends != 7) {
            throw new IllegalArgumentException("scenario must contain the two current-window cases and 11 sends");
        }
    }

    private static void runCase(JsonArray steps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        String runtimeURI = "parity-resultset-aggregate-firstlastwindow-current-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl;
            if (NO_GROUP.equals(caseName)) {
                epl = "@Name('s0') select first(theString) as firststring, " +
                        "last(theString) as laststring, first(intPrimitive) as firstint, " +
                        "last(intPrimitive) as lastint, window(intPrimitive) as allint " +
                        "from SupportBean.win:length(2)";
            } else {
                epl = "@Name('s0') select theString, first(theString) as firststring, " +
                        "last(theString) as laststring, first(intPrimitive) as firstint, " +
                        "last(intPrimitive) as lastint, window(intPrimitive) as allint " +
                        "from SupportBean#length(5) group by theString order by theString asc";
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
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
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replay(steps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
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
            if (active && "send".equals(op)) {
                JsonObject payload = step.get("payload").asObject();
                Map<String, Object> event = new HashMap<>();
                event.put("theString", payload.getString("theString", null));
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventMap(event, "SupportBean");
            }
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
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
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
