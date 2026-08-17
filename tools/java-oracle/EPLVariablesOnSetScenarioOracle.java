import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_A;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
 * Java oracle for EPLVariablesOnSet remaining executions.
 *
 * Covers typed-select (Compile/ObjectModel semantics), scene-two (@public
 * multi-module variables + irstream select), and subquery (subquery on set
 * RHS with empty-result null assignment).
 */
public class EPLVariablesOnSetScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLVariablesOnSetScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            JsonObject caseDef = caseVal.asObject();
            String caseName = caseDef.getString("name", "");
            JsonArray steps = caseDef.get("steps").asArray();
            runCase(steps, caseName, records);
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

    private static String[] buildEPLs(String caseName) {
        switch (caseName) {
            case "typed-select":
                return new String[] {
                    "create variable double var1C = 10d",
                    "create variable long var2C = 11l",
                    "@name('s0') select var1C, var2C, id from SupportBean_A",
                    "@name('set') on SupportBean set var1C=intPrimitive, var2C=intBoxed"
                };
            case "scene-two":
                return new String[] {
                    "@public create variable int resvar = 1",
                    "@public create variable int durvar = 10",
                    "@name('s1') on SupportBean set resvar=intPrimitive, durvar=intPrimitive",
                    "@name('s2') select irstream resvar, durvar, symbol from SupportMarketDataBean"
                };
            case "subquery":
                return new String[] {
                    "create variable String var1SS = 'a'",
                    "create variable String var2SS = 'b'",
                    "@name('s0') on SupportBean_S0 as s0str set " +
                    "var1SS = (select p10 from SupportBean_S1#lastevent), " +
                    "var2SS = (select p11 || s0str.p01 from SupportBean_S1#lastevent)"
                };
            default:
                throw new IllegalArgumentException("unknown case: " + caseName);
        }
    }


    private static void runCase(JsonArray steps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_A.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesOnSetScenarioOracle", config);
        try {
            String[] epls = buildEPLs(caseName);
            // Compile as one module so @public variables are visible across statements
            String module = String.join(";\n", epls) + ";";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            int[] seq = new int[] {0};
            for (EPStatement candidate : deployment.getStatements()) {
                String stmtName = candidate.getName();
                if (stmtName == null) {
                    continue;
                }
                candidate.addListener((newData, oldData, statement, rt) -> {
                    if (newData == null && oldData == null) {
                        return;
                    }
                    seq[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("stmt", stmtName);
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
                });
            }

            for (JsonValue stepVal : steps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("send".equals(op)) {
                    String type = step.getString("type", "");
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            bean.setTheString(step.getString("theString", ""));
                            bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                            if (step.get("intBoxed") != null && !step.get("intBoxed").isNull()) {
                                bean.setIntBoxed(step.getInt("intBoxed", 0));
                            }
                            runtime.getEventService().sendEventBean(bean, "SupportBean");
                            break;
                        }
                        case "SupportBean_A":
                            runtime.getEventService().sendEventBean(
                                new SupportBean_A(step.getString("id", "")), "SupportBean_A");
                            break;
                        case "SupportBean_S0": {
                            int id = step.getInt("id", 0);
                            String p00 = step.get("p00") != null && !step.get("p00").isNull()
                                ? step.getString("p00", "") : null;
                            String p01 = step.get("p01") != null && !step.get("p01").isNull()
                                ? step.getString("p01", "") : null;
                            runtime.getEventService().sendEventBean(
                                new SupportBean_S0(id, p00, p01), "SupportBean_S0");
                            break;
                        }
                        case "SupportBean_S1": {
                            int id = step.getInt("id", 0);
                            String p10 = step.get("p10") != null && !step.get("p10").isNull()
                                ? step.getString("p10", "") : null;
                            String p11 = step.get("p11") != null && !step.get("p11").isNull()
                                ? step.getString("p11", "") : null;
                            runtime.getEventService().sendEventBean(
                                new SupportBean_S1(id, p10, p11), "SupportBean_S1");
                            break;
                        }
                        case "SupportMarketDataBean": {
                            Map<String, Object> marketEvent = new HashMap<>();
                            marketEvent.put("symbol", step.getString("symbol", ""));
                            marketEvent.put("price", 0.0);
                            marketEvent.put("volume", 0L);
                            marketEvent.put("feed", "");
                            runtime.getEventService().sendEventMap(marketEvent, "SupportMarketDataBean");
                            break;
                        }
                        default:
                            throw new IllegalArgumentException("unknown event type: " + type);
                    }
                }
            }
        } finally {
            runtime.destroy();
        }
    }
}
