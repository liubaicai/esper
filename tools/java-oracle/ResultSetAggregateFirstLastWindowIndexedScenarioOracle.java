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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/** Java oracle for ResultSetAggregateFirstLastIndexed (ordinal 5). */
public final class ResultSetAggregateFirstLastWindowIndexedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-firstlastwindow-indexed";

    private ResultSetAggregateFirstLastWindowIndexedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateFirstLastWindowIndexedScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", "")) || !ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario");
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateShape(steps);
        JsonArray records = new JsonArray();
        runCase(steps, records);
        if (records.size() != 4) {
            throw new IllegalStateException("expected four listener records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        if (steps.size() != 5 || !"case".equals(steps.get(0).asObject().getString("op", ""))
                || !"indexed".equals(steps.get(0).asObject().getString("case", ""))) {
            throw new IllegalArgumentException("scenario must contain one indexed case and four sends");
        }
        for (int i = 1; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"send".equals(step.getString("op", "")) || !"SupportBean".equals(step.getString("eventType", ""))) {
                throw new IllegalArgumentException("scenario must contain SupportBean sends only");
            }
            JsonValue payload = step.get("payload");
            if (payload == null || !payload.isObject() || !payload.asObject().names().contains("intPrimitive")
                    || payload.asObject().size() != 1
                    || payload.asObject().get("intPrimitive").isNull()
                    || payload.asObject().getInt("intPrimitive", 0) != 10 + i - 1) {
                throw new IllegalArgumentException("scenario sends must contain intPrimitive 10..13");
            }
        }
    }

    private static void runCase(JsonArray steps, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", beanType);
        String runtimeURI = "parity-resultset-aggregate-firstlastwindow-indexed";
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, config);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = "@name('s0') select first(intPrimitive,0) as f0, first(intPrimitive,1) as f1, "
                    + "first(intPrimitive,2) as f2, first(intPrimitive,3) as f3, last(intPrimitive,0) as l0, "
                    + "last(intPrimitive,1) as l1, last(intPrimitive,2) as l2, last(intPrimitive,3) as l3 "
                    + "from SupportBean#length(3)";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = deployment.getStatements()[0];
            final int[] sequence = {0};
            statement.addListener((newData, oldData, ignored, ignoredRuntime) -> {
                if (newData == null || newData.length == 0) return;
                JsonArray rows = new JsonArray();
                for (EventBean event : newData) rows.add(eventToJson(event));
                records.add(new JsonObject().add("case", "indexed").add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", ++sequence[0])
                        .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                        .add("new", rows));
            });
            for (int i = 1; i < steps.size(); i++) {
                JsonObject payload = steps.get(i).asObject().get("payload").asObject();
                Map<String, Object> event = new HashMap<>();
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventMap(event, "SupportBean");
            }
        } finally {
            runtime.destroy();
        }
    }

    private static JsonObject eventToJson(EventBean event) {
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) fields.add(name, normalize(event.get(name)));
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue normalize(Object value) {
        if (value == null) return new JsonObject().add("state", "null");
        if (value instanceof Number) return Json.value(((Number) value).longValue());
        return Json.value(String.valueOf(value));
    }
}
