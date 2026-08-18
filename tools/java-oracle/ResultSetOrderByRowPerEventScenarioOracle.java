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

/**
 * Java oracle for ResultSetOrderByRowPerEvent executions:
 * RowPerEventSum, Aliases, AggOrderWithSum, RowPerEventSumHaving, RowPerEventJoin.
 */
public class ResultSetOrderByRowPerEventScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetOrderByRowPerEventScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            JsonObject caseDef = caseVal.asObject();
            String caseName = caseDef.getString("case", "");
            String epl = caseDef.getString("epl", "");
            JsonArray steps = caseDef.get("steps").asArray();
            runCase(caseName, epl, steps, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("scenario", scenario.getString("name", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(String caseName, String epl, JsonArray steps, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        marketType.put("volume", Long.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);

        Map<String, Object> sbType = new HashMap<>();
        sbType.put("theString", String.class);
        config.getCommon().addEventType("SupportBeanString", sbType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-orderby-rowperevent-" + caseName, config);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement 's0' not found in deployment");
            }

            List<JsonObject> listenerRecords = new ArrayList<>();
            statement.addListener((oldEvents, newEvents, stmt, runtimeRef) -> {
                JsonArray newJson = new JsonArray();
                EventBean[] events = (newEvents != null && newEvents.length > 0) ? newEvents : oldEvents;
                if (events != null) {
                    for (EventBean eb : events) {
                        JsonObject fields = new JsonObject();
                        for (String prop : eb.getEventType().getPropertyNames()) {
                            Object val = eb.get(prop);
                            if (val == null) {
                                fields.add(prop, Json.NULL);
                            } else if (val instanceof Number) {
                                fields.add(prop, ((Number) val).doubleValue());
                            } else {
                                fields.add(prop, String.valueOf(val));
                            }
                        }
                        JsonObject row = new JsonObject();
                        row.add("fields", fields);
                        newJson.add(row);
                    }
                    listenerRecords.add(new JsonObject()
                            .add("case", caseName)
                            .add("sequence", listenerRecords.size())
                            .add("new", newJson));
                }
            });

            int seq = 0;
            for (JsonValue stepVal : steps) {
                JsonObject step = stepVal.asObject();
                String type = step.getString("type", "");
                Map<String, Object> event = new HashMap<>();
                for (String key : step.names()) {
                    if ("type".equals(key) || "op".equals(key)) continue;
                    JsonValue v = step.get(key);
                    if (v.isNull()) {
                        event.put(key, null);
                    } else if (v.isString()) {
                        event.put(key, v.asString());
                    } else if (v.isNumber()) {
                        if (key.equals("volume")) {
                            event.put(key, (long) v.asDouble());
                        } else {
                            event.put(key, v.asDouble());
                        }
                    } else {
                        event.put(key, v.toString());
                    }
                }
                runtime.getEventService().sendEventMap(event, type);
            }
            // Debug: check if listener was invoked
            System.err.println("case=" + caseName + " listenerRecords=" + listenerRecords.size());

            for (JsonObject listenerRecord : listenerRecords) {
                records.add(listenerRecord);
            }
        } finally {
            runtime.destroy();
        }
    }
}
