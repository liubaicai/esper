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

/** Java oracle for ResultSetAggregatePrevNthIndexedFirstLast (ordinal 6). */
public final class ResultSetAggregateFirstLastWindowPrevNthScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-firstlastwindow-prev-nth";

    private ResultSetAggregateFirstLastWindowPrevNthScenarioOracle() {}

    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("usage: ResultSetAggregateFirstLastWindowPrevNthScenarioOracle <scenario.json>");
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", "")) || !ID.equals(scenario.getString("id", ""))) throw new IllegalArgumentException("unsupported scenario");
        JsonArray records = new JsonArray();
        runCase(scenario.get("steps").asArray(), records);
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void runCase(JsonArray steps, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", beanType);
        String runtimeURI = "parity-resultset-aggregate-firstlastwindow-prev-nth";
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, config);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = "@name('s0') select prev(intPrimitive,0) as p0, prev(intPrimitive,1) as p1, prev(intPrimitive,2) as p2, " +
                    "nth(intPrimitive,0) as n0, nth(intPrimitive,1) as n1, nth(intPrimitive,2) as n2, " +
                    "last(intPrimitive,0) as l1, last(intPrimitive,1) as l2, last(intPrimitive,2) as l3 from SupportBean#length(3)";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = deployment.getStatements()[0];
            final int[] sequence = {0};
            statement.addListener((newData, oldData, ignored, ignoredRuntime) -> {
                if (newData == null || newData.length == 0) return;
                JsonArray rows = new JsonArray();
                for (EventBean event : newData) rows.add(eventToJson(event));
                records.add(new JsonObject().add("case", "prev-nth").add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", ++sequence[0])
                        .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                        .add("new", rows));
            });
            for (JsonValue value : steps) {
                JsonObject step = value.asObject();
                if (!"send".equals(step.getString("op", ""))) continue;
                JsonObject payload = step.get("payload").asObject();
                Map<String, Object> event = new HashMap<>();
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventMap(event, "SupportBean");
            }
        } finally { runtime.destroy(); }
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
