import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.compiler.client.CompilerArguments;
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
 * Direct Esper 9.0.0 oracle for the resultset-row-for-all parity scenario
 * (ResultSetQueryTypeRowForAll observable executions: Simple, SumMinMax,
 * MinMaxWindowed, WWindowAgg, NamedWindowWindow). Map event types;
 * sendEventMap; each send step captures actual new/old row properties and
 * emits one JSON record comparing them against the scenario expectation.
 */
public final class ResultSetRowForAllScenarioOracle {
    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/resultset-row-for-all.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("scenario", "resultset-row-for-all"));
        root.add("scenario", scenario.getString("scenario", "resultset-row-for-all"));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            runCase(caseVal.asObject(), recordsArr);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonObject caseDef, JsonArray records) throws Exception {
        String caseName = caseDef.getString("case", "");
        String epl = caseDef.getString("epl", "");
        String pre = caseDef.getString("pre", "");

        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> sbType = new HashMap<>();
        sbType.put("theString", String.class);
        sbType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", sbType);

        Map<String, Object> mdType = new HashMap<>();
        mdType.put("symbol", String.class);
        mdType.put("price", Double.class);
        config.getCommon().addEventType("SupportMarketDataBean", mdType);

        Map<String, Object> saType = new HashMap<>();
        saType.put("id", String.class);
        config.getCommon().addEventType("SupportBean_A", saType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-rowforall-" + caseName, config);
        try {
            String module = pre.isEmpty() ? epl : pre + "\n" + epl;
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
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
            final List<Map<String, Object>> newProps = new ArrayList<>();
            final List<Map<String, Object>> oldProps = new ArrayList<>();
            statement.addListener((newEvents, oldEvents, stmt, runtimeRef) -> {
                newProps.clear();
                oldProps.clear();
                if (newEvents != null) {
                    for (EventBean event : newEvents) {
                        newProps.add(propsOf(event));
                    }
                }
                if (oldEvents != null) {
                    for (EventBean event : oldEvents) {
                        oldProps.add(propsOf(event));
                    }
                }
            });

            int step = 0;
            for (JsonValue sendVal : caseDef.get("send").asArray()) {
                JsonObject sendObj = sendVal.asObject();
                String type = sendObj.getString("type", "SupportMarketDataBean");
                Map<String, Object> event = new HashMap<>();
                for (String key : sendObj.names()) {
                    if (key.equals("type") || key.equals("new") || key.equals("old")) {
                        continue;
                    }
                    JsonValue v = sendObj.get(key);
                    if (v.isNull()) {
                        event.put(key, null);
                    } else if (v.isString()) {
                        event.put(key, v.asString());
                    } else if (v.isNumber()) {
                        double dv = v.asDouble();
                        if (key.equals("intPrimitive")) {
                            event.put(key, (int) dv);
                        } else if (key.equals("price")) {
                            event.put(key, dv);
                        } else {
                            event.put(key, dv);
                        }
                    }
                }
                newProps.clear();
                oldProps.clear();
                runtime.getEventService().sendEventMap(event, type);

                JsonObject row = new JsonObject();
                row.add("case", caseName);
                row.add("step", step++);
                JsonObject expected = (sendObj.get("new") == null || sendObj.get("new").isNull()) ? null : sendObj.get("new").asObject();
                JsonObject expectedOld = (sendObj.get("old") == null || sendObj.get("old").isNull()) ? null : sendObj.get("old").asObject();
                row.add("received", single(newProps, expected));
                row.add("receivedOld", single(oldProps, expectedOld));
                row.add("expected", expected == null ? (JsonValue) Json.value(null) : normalized(expected));
                row.add("expectedOld", expectedOld == null ? (JsonValue) Json.value(null) : normalized(expectedOld));
                records.add(row);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static JsonObject raw(Map<String, Object> actual) {
        JsonObject out = new JsonObject();
        for (Map.Entry<String, Object> e : actual.entrySet()) {
            Object v = e.getValue();
            if (v == null) {
                out.add(e.getKey(), Json.value(null));
            } else if (v instanceof Number) {
                out.add(e.getKey(), Json.value(((Number) v).doubleValue()));
            } else if (v instanceof Object[]) {
                JsonArray arr = new JsonArray();
                for (Object item : (Object[]) v) {
                    if (item instanceof Number) {
                        arr.add(((Number) item).doubleValue());
                    } else if (item == null) {
                        arr.add(Json.value(null));
                    } else {
                        arr.add(String.valueOf(item));
                    }
                }
                out.add(e.getKey(), arr);
            } else {
                out.add(e.getKey(), String.valueOf(v));
            }
        }
        return out;
    }

    private static Map<String, Object> propsOf(EventBean event) {
        Map<String, Object> props = new HashMap<>();
        for (String name : event.getEventType().getPropertyNames()) {
            props.put(name, event.get(name));
        }
        return props;
    }

    private static JsonObject normalized(JsonObject expected) {
        JsonObject out = new JsonObject();
        for (String name : expected.names()) {
            JsonValue v = expected.get(name);
            out.add(name, v);
        }
        return out;
    }

    private static JsonObject single(List<Map<String, Object>> props, JsonObject expected) {
        // Compare only the fields named in the expectation. A missing
        // expectation means "no assertion": emit the raw first-row properties
        // (probe mode) instead of failing.
        JsonObject out = new JsonObject();
        if (expected == null) {
            if (props.isEmpty()) {
                return out;
            }
            return raw(props.get(0));
        }
        if (props.size() != 1) {
            throw new IllegalStateException("expected exactly 1 event, got " + props.size());
        }
        Map<String, Object> actual = props.get(0);
        for (String name : expected.names()) {
            Object v = actual.get(name);
            if (v == null) {
                out.add(name, Json.value(null));
            } else if (v instanceof Number) {
                out.add(name, Json.value(((Number) v).doubleValue()));
            } else if (v instanceof Object[]) {
                JsonArray arr = new JsonArray();
                for (Object item : (Object[]) v) {
                    if (item instanceof Number) {
                        arr.add(((Number) item).doubleValue());
                    } else if (item == null) {
                        arr.add(Json.value(null));
                    } else if (item instanceof Map) {
                        JsonObject o = new JsonObject();
                        for (Map.Entry<?, ?> e : ((Map<?, ?>) item).entrySet()) {
                            Object val = e.getValue();
                            if (val instanceof Number) {
                                o.add(String.valueOf(e.getKey()), Json.value(((Number) val).doubleValue()));
                            } else if (val == null) {
                                o.add(String.valueOf(e.getKey()), Json.value(null));
                            } else {
                                o.add(String.valueOf(e.getKey()), String.valueOf(val));
                            }
                        }
                        arr.add(o);
                    } else {
                        arr.add(String.valueOf(item));
                    }
                }
                out.add(name, arr);
            } else {
                out.add(name, String.valueOf(v));
            }
        }
        return out;
    }
}
