import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.EPCompiled;
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
import java.io.FileReader;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the filter-rebool-optimizable parity
 * scenario. Uses map event types and sendEventMap. Unlike the single-stmt
 * filter-val oracle, every statement whose name starts with "s" in the
 * deployment gets a listener; each send step records one row per monitored
 * statement with the actual fired outcome plus the expected flag from the
 * scenario (comma-joined when a step applies to all statements).
 */
public final class FilterReboolScenarioOracle {
    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/filter-rebool-optimizable.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("scenario", scenario.getString("scenario", "filter-rebool-optimizable"));
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

        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> sbType = new HashMap<>();
        sbType.put("theString", String.class);
        sbType.put("intPrimitive", Integer.class);
        config.getCommon().addEventType("SupportBean", sbType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        s0Type.put("p02", String.class);
        s0Type.put("p03", String.class);
        config.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        s1Type.put("p12", String.class);
        s1Type.put("p13", String.class);
        config.getCommon().addEventType("SupportBean_S1", s1Type);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("filter-rebool-" + caseName, config);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            List<EPStatement> monitored = new ArrayList<>();
            final Map<String, Boolean> fired = new HashMap<>();
            for (EPStatement stmt : deployment.getStatements()) {
                final String name = stmt.getName();
                if (name.startsWith("s")) {
                    monitored.add(stmt);
                    fired.put(name, false);
                    stmt.addListener((newEvents, oldEvents, stmtRef, runtimeRef) -> {
                        if (newEvents != null && newEvents.length > 0) {
                            fired.put(name, true);
                        }
                    });
                }
            }
            if (monitored.isEmpty()) {
                throw new IllegalStateException("no monitored statements");
            }

            int step = 0;
            for (JsonValue sendVal : caseDef.get("send").asArray()) {
                JsonObject sendObj = sendVal.asObject();
                String type = sendObj.getString("type", "SupportBean");
                Map<String, Object> event = new HashMap<>();
                for (String key : sendObj.names()) {
                    if (key.equals("type") || key.equals("op") || key.equals("received")) {
                        continue;
                    }
                    JsonValue v = sendObj.get(key);
                    if (v.isNull()) {
                        event.put(key, null);
                    } else if (v.isString()) {
                        event.put(key, v.asString());
                    } else if (v.isNumber()) {
                        if (key.equals("intPrimitive") || key.equals("id")) {
                            event.put(key, (int) v.asDouble());
                        } else {
                            event.put(key, v.asDouble());
                        }
                    } else {
                        event.put(key, v.toString());
                    }
                }
                for (String name : fired.keySet()) {
                    fired.put(name, false);
                }
                runtime.getEventService().sendEventMap(event, type);
                JsonArray receivedArr = new JsonArray();
                for (EPStatement stmt : monitored) {
                    receivedArr.add(fired.get(stmt.getName()));
                }
                JsonObject row = new JsonObject();
                row.add("case", caseName);
                row.add("step", step++);
                row.add("received", receivedArr);
                JsonValue expVal = sendObj.get("received");
                if (expVal != null && expVal.isArray()) {
                    row.add("expected", expVal);
                } else {
                    row.add("expected", sendObj.getBoolean("received", false));
                }
                records.add(row);
            }
        } finally {
            runtime.destroy();
        }
    }
}
