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
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for ResultSetQueryTypeRowForAll ungrouped aggregate scenarios.
 *
 * Covers SumMinMax, Simple, and MinMaxWindowed by replaying deterministic
 * event sequences and recording observable output including old (remove) stream.
 */
public class ResultSetRowForAllScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetRowForAllScenarioOracle <scenario.json>");
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetRowForAllScenarioOracle", config);
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
                if (newData != null || oldData != null) {
                    seq[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("sequence", seq[0]);
                    if (newData != null) {
                        JsonArray newArr = new JsonArray();
                        for (EventBean event : newData) {
                            JsonObject newItem = new JsonObject();
                            JsonObject fields = new JsonObject();
                            for (String prop : event.getEventType().getPropertyNames()) {
                                Object value = event.get(prop);
                                fields.add(prop, value == null ? null : value.toString());
                            }
                            newItem.add("fields", fields);
                            newArr.add(newItem);
                        }
                        record.add("new", newArr);
                    }
                    if (oldData != null) {
                        JsonArray oldArr = new JsonArray();
                        for (EventBean event : oldData) {
                            JsonObject oldItem = new JsonObject();
                            JsonObject fields = new JsonObject();
                            for (String prop : event.getEventType().getPropertyNames()) {
                                Object value = event.get(prop);
                                fields.add(prop, value == null ? null : value.toString());
                            }
                            oldItem.add("fields", fields);
                            oldArr.add(oldItem);
                        }
                        record.add("old", oldArr);
                    }
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
                    String type = step.getString("type", "");
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            bean.setTheString(step.getString("theString", ""));
                            bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                            runtime.getEventService().sendEventBean(bean, type);
                            break;
                        }
                        case "SupportMarketDataBean": {
                            Map<String, Object> marketEvent = new HashMap<>();
                            marketEvent.put("symbol", step.getString("symbol", ""));
                            marketEvent.put("price", step.getDouble("price", 0.0));
                            runtime.getEventService().sendEventMap(marketEvent, type);
                            break;
                        }
                        default:
                            throw new IllegalStateException("unknown type: " + type);
                    }
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "sum-min-max" ->
                "@name('s0') select theString as c0, sum(intPrimitive) as c1, " +
                "min(intPrimitive) as c2, max(intPrimitive) as c3 from SupportBean";
            case "simple" ->
                "@name('s0') select irstream avg(price) as avgPrice, sum(price) as sumPrice, " +
                "min(price) as minPrice, max(price) as maxPrice, median(price) as medianPrice, " +
                "stddev(price) as stddevPrice, avedev(price) as avedevPrice, " +
                "count(*) as datacount, count(distinct price) as countDistinctPrice " +
                "from SupportMarketDataBean";
            case "min-max-windowed" ->
                "@name('s0') select irstream min(price) as minPrice, max(price) as maxPrice " +
                "from SupportMarketDataBean#length(2)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
