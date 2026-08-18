import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import java.io.FileReader;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the stream-selector-extended parity scenario.
 * Uses map event types and sendEventMap; output is a single JSON trace on stdout.
 */
public final class StreamSelectorExtendedScenarioOracle {
    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/stream-selector-extended.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("scenario", scenario.getString("scenario", "stream-selector-extended"));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            JsonObject caseDef = caseVal.asObject();
            runCase(caseDef, recordsArr);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonObject caseDef, JsonArray records) throws Exception {
        String caseName = caseDef.getString("case", "");
        String epl = caseDef.getString("epl", "");

        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> sbType = new HashMap<>();
        sbType.put("theString", String.class);
        sbType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", sbType);

        Map<String, Object> mdType = new HashMap<>();
        mdType.put("symbol", String.class);
        mdType.put("price", Double.class);
        mdType.put("volume", Long.class);
        config.getCommon().addEventType("SupportMarketDataBean", mdType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-stream-selector-extended-" + caseName, config);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement found = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    found = candidate;
                    break;
                }
            }
            if (found == null) {
                throw new IllegalStateException("statement 's0' not found in deployment");
            }
            final EPStatement statement = found;
            final EventType outputType = statement.getEventType();

            List<JsonObject> listenerRecords = new ArrayList<>();
            statement.addListener((newEvents, oldEvents, stmt, runtimeRef) -> {
                EventBean[] events = newEvents;
                if (events == null) {
                    return;
                }
                for (EventBean eb : events) {
                    JsonObject fieldsObj = new JsonObject();
                    for (String prop : outputType.getPropertyNames()) {
                        Object val = eb.get(prop);
                        if (val == null) {
                            fieldsObj.add(prop, JsonValue.NULL);
                        } else if (val instanceof Number) {
                            double d = ((Number) val).doubleValue();
                            if (d == Math.floor(d) && !Double.isInfinite(d)) {
                                fieldsObj.add(prop, JsonValue.valueOf((long) d));
                            } else {
                                fieldsObj.add(prop, JsonValue.valueOf(d));
                            }
                        } else if (val instanceof Boolean) {
                            fieldsObj.add(prop, (Boolean) val);
                        } else if (val instanceof EventBean) {
                            EventBean nested = (EventBean) val;
                            JsonObject nestedObj = new JsonObject();
                            for (String np : nested.getEventType().getPropertyNames()) {
                                Object nv = nested.get(np);
                                if (nv == null) {
                                    nestedObj.add(np, JsonValue.NULL);
                                } else if (nv instanceof Number) {
                                    double nd = ((Number) nv).doubleValue();
                                    if (nd == Math.floor(nd) && !Double.isInfinite(nd)) {
                                        nestedObj.add(np, JsonValue.valueOf((long) nd));
                                    } else {
                                        nestedObj.add(np, JsonValue.valueOf(nd));
                                    }
                                } else {
                                    nestedObj.add(np, String.valueOf(nv));
                                }
                            }
                            fieldsObj.add(prop, nestedObj);
                        } else {
                            fieldsObj.add(prop, String.valueOf(val));
                        }
                    }
                    JsonObject row = new JsonObject();
                    row.add("case", caseName);
                    row.add("sequence", listenerRecords.size());
                    row.add("fields", fieldsObj);
                    listenerRecords.add(row);
                }
            });

            for (JsonValue sendVal : caseDef.get("send").asArray()) {
                JsonObject sendObj = sendVal.asObject();
                String type = sendObj.getString("type", "SupportBean");
                Map<String, Object> event = new HashMap<>();
                for (String key : sendObj.names()) {
                    if (key.equals("type") || key.equals("op")) {
                        continue;
                    }
                    JsonValue v = sendObj.get(key);
                    if (v.isNull()) {
                        event.put(key, null);
                    } else if (v.isString()) {
                        event.put(key, v.asString());
                    } else if (v.isNumber()) {
                        if (key.equals("volume")) {
                            event.put(key, (long) v.asDouble());
                        } else if (key.equals("intPrimitive")) {
                            event.put(key, (int) v.asDouble());
                        } else {
                            event.put(key, v.asDouble());
                        }
                    } else {
                        event.put(key, v.toString());
                    }
                }
                runtime.getEventService().sendEventMap(event, type);
            }

            for (JsonObject record : listenerRecords) {
                records.add(record);
            }
        } finally {
            runtime.destroy();
        }
    }
}
