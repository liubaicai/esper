import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
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
import java.util.ArrayList;
import java.util.List;

/**
 * Java oracle for EPLSubselectExists EXISTS/NOT EXISTS subquery scenarios.
 *
 * Covers the InSelect, SceneOne, Filtered, TwoExistsFiltered, and NotExists
 * runtimes by replaying deterministic event sequences and recording the
 * observable output.
 */
public class EPLSubselectExistsScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectExistsScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getCommon().addEventType(SupportBean_S2.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectExistsScenarioOracle", config);
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
                if (newData != null) {
                    for (EventBean event : newData) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("sequence", seq[0]);
                        JsonArray newArr = new JsonArray();
                        JsonObject newItem = new JsonObject();
                        JsonObject fields = new JsonObject();
                        for (String prop : event.getEventType().getPropertyNames()) {
                            Object value = event.get(prop);
                            fields.add(prop, value == null ? null : value.toString());
                        }
                        newItem.add("fields", fields);
                        newArr.add(newItem);
                        record.add("new", newArr);
                        records.add(record);
                    }
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
                    String type = step.getString("type", "");
                    int id = step.getInt("id", 0);
                    Object event = switch (type) {
                        case "SupportBean_S0" -> new SupportBean_S0(id);
                        case "SupportBean_S1" -> new SupportBean_S1(id);
                        case "SupportBean_S2" -> new SupportBean_S2(id);
                        default -> throw new IllegalStateException("unknown type: " + type);
                    };
                    runtime.getEventService().sendEventBean(event, type);
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "exists-in-select" ->
                "@name('s0') select exists (select * from SupportBean_S1#length(1000)) as value from SupportBean_S0";
            case "exists-scene-one" ->
                "@name('s0') select id from SupportBean_S0 where exists (select * from SupportBean_S1#length(1000))";
            case "exists-filtered" ->
                "@name('s0') select id from SupportBean_S0 as s0 where exists (select * from SupportBean_S1#length(1000) as s1 where s1.id=s0.id)";
            case "two-exists-filtered" ->
                "@name('s0') select id from SupportBean_S0 as s0 where " +
                "exists (select * from SupportBean_S1#length(1000) as s1 where s1.id=s0.id) " +
                "and " +
                "exists (select * from SupportBean_S2#length(1000) as s2 where s2.id=s0.id)";
            case "not-exists" ->
                "@name('s0') select id from SupportBean_S0 where not exists (select * from SupportBean_S1#length(1000))";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
