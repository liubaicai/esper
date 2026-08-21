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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

public class EPOrderBySimpleScenarioOracle {
    public static void main(String[] args) throws Exception {
        if (args.length != 1) { System.err.println("usage: <scenario.json>"); System.exit(1); }
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
        for (JsonObject record : records) { recordsArr.add(record); }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        Map<String, Object> mdbType = new HashMap<>();
        mdbType.put("symbol", String.class);
        mdbType.put("price", Double.class);
        mdbType.put("volume", Long.class);
        config.getCommon().addEventType("SupportMarketDataBean", mdbType);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPOrderBySimple-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epls[0], new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            int[] seq = {0};
            for (EPStatement stmt : deployment.getStatements()) {
                if ("s0".equals(stmt.getName())) {
                    stmt.addListener((newData, oldData, statement, rt) -> {
                        if (newData != null && newData.length > 0) {
                            seq[0]++;
                            JsonObject record = new JsonObject();
                            record.add("case", caseName);
                            record.add("operation", "listener");
                            record.add("statement", statement.getName());
                            record.add("sequence", seq[0]);
                            record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                            JsonArray newArr = new JsonArray();
                            for (EventBean event : newData) {
                                JsonObject newItem = new JsonObject();
                                newItem.add("kind", "row");
                                JsonObject fields = new JsonObject();
                                for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                                    fields.add(prop, normalize(event.get(prop)));
                                }
                                newItem.add("fields", fields);
                                newArr.add(newItem);
                            }
                            record.add("new", newArr);
                            records.add(record);
                        }
                    });
                }
            }
            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) continue;
                if ("send".equals(op)) {
                    String eventType = step.getString("eventType", "");
                    if (!"SupportMarketDataBean".equals(eventType)) continue;
                    JsonObject payload = step.get("payload").asObject();
                    Map<String, Object> eventMap = new HashMap<>();
                    eventMap.put("symbol", payload.getString("symbol", ""));
                    eventMap.put("price", payload.getDouble("price", 0));
                    eventMap.put("volume", (long) payload.getDouble("volume", 0));
                    runtime.getEventService().sendEventMap(eventMap, "SupportMarketDataBean");
                }
            }
        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "orderby-simple" -> new String[]{
                "@name('s0')select symbol from SupportMarketDataBean#length(5) output every 6 events order by price"
            };
            case "orderby-descending" -> new String[]{
                "@name('s0')select symbol from SupportMarketDataBean#length(5) output every 6 events order by price desc"
            };
            case "orderby-multiple-keys" -> new String[]{
                "@name('s0')select symbol from SupportMarketDataBean#length(10) output every 6 events order by symbol, price"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }
}
