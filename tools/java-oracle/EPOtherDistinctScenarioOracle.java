import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * Java oracle for EPLOtherDistinct select-distinct scenarios.
 *
 * Covers the OutputSimpleColumn, OutputLimitEveryColumn, and BatchWindow
 * runtimes by replaying deterministic SupportBean sequences and recording the
 * deduplicated listener output rows in first-seen insertion order.
 */
public class EPOtherDistinctScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPOtherDistinctScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPOtherDistinct-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            EPStatement stmt = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    stmt = candidate;
                    break;
                }
            }
            if (stmt == null) {
                throw new IllegalStateException("statement s0 not found");
            }

            int[] seq = new int[] {0};
            stmt.addListener((newData, oldData, statement, rt) -> {
                if (newData != null && newData.length > 0) {
                    seq[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "listener");
                    record.add("statement", statement.getName());
                    record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                    record.add("sequence", seq[0]);
                    JsonArray newArr = new JsonArray();
                    for (EventBean event : newData) {
                        JsonObject newItem = new JsonObject();
                        newItem.add("kind", "row");
                        JsonObject fields = new JsonObject();
                        fields.add("theString", event.get("theString") == null ? null : String.valueOf(event.get("theString")));
                        Object ip = event.get("intPrimitive");
                        fields.add("intPrimitive", ip instanceof Number ? Json.value(((Number) ip).longValue()) : null);
                        newItem.add("fields", fields);
                        newArr.add(newItem);
                    }
                    record.add("new", newArr);
                    records.add(record);
                }
            });

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if ("send".equals(op)) {
                    String type = step.getString("eventType", "");
                    if (!"SupportBean".equals(type)) {
                        continue;
                    }
                    JsonObject payload = step.get("payload").asObject();
                    SupportBean bean = new SupportBean();
                    bean.setTheString(payload.getString("theString", ""));
                    bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                    runtime.getEventService().sendEventBean(bean, type);
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "distinct-simple-column" ->
                "@name('s0') select distinct theString,intPrimitive from SupportBean#keepall";
            case "distinct-output-every-column" ->
                "@name('s0') @IterableUnbound select distinct theString,intPrimitive from SupportBean#keepall output every 3 events";
            case "distinct-batch-window" ->
                "@name('s0') select distinct theString,intPrimitive from SupportBean#length_batch(3)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
