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
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the expr-filter-opt-lkup-limited parity
 * scenario (ExprFilterOptimizableLookupableLimitedExpr observable executions:
 * ConstantEqualsNull, EqualsCoercion, EqualsOneStmt, InSetOfValue,
 * InRangeWCoercion including its not-in variant). Map event types;
 * sendEventMap; one JSON trace record per send step on stdout.
 */
public final class ExprFilterOptLkupLimitedScenarioOracle {
    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-filter-opt-lkup-limited.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("scenario", scenario.getString("scenario", "expr-filter-opt-lkup-limited"));
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
        sbType.put("doublePrimitive", Double.class);
        sbType.put("doubleBoxed", Double.class);
        sbType.put("longPrimitive", Long.class);
        sbType.put("longBoxed", Long.class);
        config.getCommon().addEventType("SupportBean", sbType);

        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        config.getCommon().addEventType("SupportBean_S0", s0Type);

        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        config.getCommon().addEventType("SupportBean_S1", s1Type);

        Map<String, Object> s2Type = new HashMap<>();
        s2Type.put("id", Integer.class);
        config.getCommon().addEventType("SupportBean_S2", s2Type);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-filter-lkup-" + caseName, config);
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
            final boolean[] fired = {false};
            statement.addListener((newEvents, oldEvents, stmt, runtimeRef) -> {
                if (newEvents != null && newEvents.length > 0) {
                    fired[0] = true;
                }
            });

            int step = 0;
            for (JsonValue sendVal : caseDef.get("send").asArray()) {
                JsonObject sendObj = sendVal.asObject();
                boolean expected = sendObj.getBoolean("received", false);
                String type = sendObj.getString("type", "SupportBean");
                Map<String, Object> event = new HashMap<>();
                for (String key : sendObj.names()) {
                    if (key.equals("type") || key.equals("op") || key.equals("received") || key.equals("longStep")) {
                        continue;
                    }
                    JsonValue v = sendObj.get(key);
                    if (v.isNull()) {
                        event.put(key, null);
                    } else if (v.isString()) {
                        event.put(key, v.asString());
                    } else if (v.isNumber()) {
                        double dv = v.asDouble();
                        if (key.equals("id")) {
                            event.put(key, (int) dv);
                        } else if (key.startsWith("long")) {
                            event.put(key, (long) dv);
                        } else {
                            event.put(key, dv);
                        }
                    } else {
                        event.put(key, v.toString());
                    }
                }
                fired[0] = false;
                runtime.getEventService().sendEventMap(event, type);
                JsonObject row = new JsonObject();
                row.add("case", caseName);
                row.add("step", step++);
                row.add("received", fired[0]);
                row.add("expected", expected);
                records.add(row);
            }
        } finally {
            runtime.destroy();
        }
    }
}
