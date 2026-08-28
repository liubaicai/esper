import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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

/** Java oracle for ResultSetAggregateNTh. */
public final class ResultSetAggregateNThScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-nth";
    private static final String CASE = "nth";
    private static final String EPL = "@name('s0') select theString, nth(intPrimitive,0) as int1, nth(intPrimitive,1) as int2 "
            + "from SupportBean#keepall group by theString output last every 3 events order by theString";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("usage: ResultSetAggregateNThScenarioOracle <scenario.json>");
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", "")) || !ID.equals(scenario.getString("id", ""))) throw new IllegalArgumentException("unsupported scenario");
        JsonArray steps = scenario.get("steps").asArray();
        validateShape(steps);
        JsonArray records = new JsonArray();
        runPhase(steps, records, false);
        runPhase(steps, records, true);
        if (records.size() != 6) throw new IllegalStateException("expected six listener records, got " + records.size());
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        if (steps.size() != 10 || !"case".equals(steps.get(0).asObject().getString("op", ""))
                || !CASE.equals(steps.get(0).asObject().getString("case", ""))) throw new IllegalArgumentException("scenario must contain one nth case and nine sends");
        for (int i = 1; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            JsonObject payload = step.get("payload").asObject();
            if (!"send".equals(step.getString("op", "")) || !"SupportBean".equals(step.getString("eventType", ""))
                    || payload.size() != 2 || !payload.names().contains("theString") || !payload.names().contains("intPrimitive")) throw new IllegalArgumentException("scenario must contain SupportBean sends with theString and intPrimitive");
        }
    }

    private static void runPhase(JsonArray steps, JsonArray records, boolean soda) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class); beanType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", beanType);
        String runtimeURI = "parity-resultset-aggregate-nth";
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI + (soda ? "-soda" : "-epl"), config);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled;
            if (soda) {
                EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(EPCompilerProvider.getCompiler().eplToModel(EPL, config));
                Module module = new Module(); module.getItems().add(new ModuleItem(model)); module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(runtime.getRuntimePath()));
            } else compiled = EPCompilerProvider.getCompiler().compile(EPL, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = deployment.getStatements()[0];
            final int[] sequence = {0};
            statement.addListener((newData, oldData, ignored, ignoredRuntime) -> {
                if (newData == null || newData.length == 0) return;
                JsonArray rows = new JsonArray(); for (EventBean event : newData) rows.add(eventToJson(event));
                records.add(new JsonObject().add("case", soda ? "soda" : "epl").add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", ++sequence[0] + (soda ? 3 : 0))
                        .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString()).add("new", rows));
            });
            for (int i = 1; i < steps.size(); i++) {
                JsonObject p = steps.get(i).asObject().get("payload").asObject();
                Map<String, Object> event = new HashMap(); event.put("theString", p.getString("theString", null)); event.put("intPrimitive", p.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventMap(event, "SupportBean");
            }
            runtime.getDeploymentService().undeployAll();
        } finally { runtime.destroy(); }
    }

    private static JsonObject eventToJson(EventBean event) {
        JsonObject fields = new JsonObject(); String[] names = event.getEventType().getPropertyNames().clone(); Arrays.sort(names);
        for (String name : names) fields.add(name, normalize(event.get(name)));
        return new JsonObject().add("kind", "row").add("fields", fields);
    }
    private static JsonValue normalize(Object value) { if (value == null) return new JsonObject().add("state", "null"); if (value instanceof Number) return Json.value(((Number) value).longValue()); return Json.value(String.valueOf(value)); }
}
