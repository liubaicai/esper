import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeMap;
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
 * Direct Esper 9.0.0 oracle for the dt-interval-ops parity scenario. Uses
 * long-field (epoch-millis) map event types A/B with start/end properties,
 * mirroring ExprDTIntervalOps.assertExpressionForType with the LONG field
 * type: "select * from A#lastevent as a, B#lastevent as b where a.OP(b)".
 * Each case sends one B event then a sequence of A events; one trace row is
 * emitted per A send with the listener outcome and the expected flag.
 */
public final class DTIntervalOpsScenarioOracle {
    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/dt-interval-ops.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("scenario", scenario.getString("scenario", "dt-interval-ops"));
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
        String op = caseDef.getString("op", "before");
        long bStart = caseDef.getLong("bStart", 0L);
        long bEnd = caseDef.getLong("bEnd", 0L);

        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> intervalType = new HashMap<>();
        intervalType.put("st", Long.class);
        intervalType.put("en", Long.class);
        config.getCommon().addEventType("A", intervalType);
        config.getCommon().addEventType("B", intervalType);
        ConfigurationCommonEventTypeMap mapCfg = new ConfigurationCommonEventTypeMap();
        mapCfg.setStartTimestampPropertyName("st");
        mapCfg.setEndTimestampPropertyName("en");
        config.getCommon().addMapConfiguration("A", mapCfg);
        config.getCommon().addMapConfiguration("B", mapCfg);
        JsonValue paramsVal = caseDef.get("params");
        String paramText = "";
        if (paramsVal != null && paramsVal.isArray()) {
            for (JsonValue p : paramsVal.asArray()) {
                paramText += ", " + p.toString();
            }
        }
        String select = caseDef.getString("select", "");
        String epl;
        if (select.isEmpty()) {
            epl = "@name('s0') select * from A#lastevent as a, B#lastevent as b where a." + op + "(b" + paramText + ")";
        } else {
            epl = "@name('s0') select " + select + " from A#lastevent as a, B#lastevent as b";
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("dt-interval-" + caseName, config);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            final boolean[] fired = {false};
            final java.util.List<Object> lastValues = new java.util.ArrayList<>();
            for (EPStatement stmt : deployment.getStatements()) {
                if ("s0".equals(stmt.getName())) {
                    stmt.addListener((newEvents, oldEvents, stmtRef, runtimeRef) -> {
                        fired[0] = newEvents != null && newEvents.length > 0;
                        lastValues.clear();
                        if (newEvents != null && newEvents.length > 0) {
                            for (String prop : newEvents[0].getEventType().getPropertyNames()) {
                                lastValues.add(newEvents[0].get(prop));
                            }
                        }
                    });
                }
            }

            Map<String, Object> bEvent = new HashMap<>();
            bEvent.put("st", bStart);
            bEvent.put("en", bEnd);
            runtime.getEventService().sendEventMap(bEvent, "B");

            int step = 0;
            for (JsonValue sendVal : caseDef.get("send").asArray()) {
                JsonObject sendObj = sendVal.asObject();
                long aStart = (long) sendObj.getDouble("aStart", 0);
                long aEnd = (long) sendObj.getDouble("aEnd", 0);
                Map<String, Object> aEvent = new HashMap<>();
                aEvent.put("st", aStart);
                aEvent.put("en", aEnd);
                fired[0] = false;
                lastValues.clear();
                runtime.getEventService().sendEventMap(aEvent, "A");
                JsonObject row = new JsonObject();
                row.add("case", caseName);
                row.add("step", step++);
                if (select.isEmpty()) {
                    row.add("received", fired[0]);
                    row.add("expected", sendObj.getBoolean("received", false));
                } else {
                    JsonArray values = new JsonArray();
                    JsonArray expectedValues = new JsonArray();
                    for (Object v : lastValues) {
                        values.add(v == null ? Json.value(null) : Json.value((Boolean) v));
                    }
                    for (JsonValue ev : sendObj.get("values").asArray()) {
                        expectedValues.add(ev);
                    }
                    row.add("received", values);
                    row.add("expected", expectedValues);
                }
                records.add(row);
            }
        } finally {
            runtime.destroy();
        }
    }
}
