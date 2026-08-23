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

        JsonArray allSteps;
        java.util.List<String> caseNames = new ArrayList<>();
        if (scenario.get("steps") != null) {
            allSteps = scenario.get("steps").asArray();
            for (JsonValue caseVal : scenario.get("cases").asArray()) {
                caseNames.add(caseVal.asObject().getString("case", ""));
            }
        } else {
            allSteps = new JsonArray();
            for (JsonValue caseVal : scenario.get("cases").asArray()) {
                JsonObject caseDef = caseVal.asObject();
                String caseName = caseDef.getString("name", "");
                caseNames.add(caseName);
                JsonObject marker = new JsonObject();
                marker.set("op", "case");
                marker.set("case", caseName);
                allSteps.add(marker);
                for (JsonValue stepVal : caseDef.get("steps").asArray()) {
                    JsonObject step = stepVal.asObject();
                    if ("send".equals(step.getString("op", ""))) {
                        JsonObject payload = new JsonObject();
                        step.set("eventType", step.getString("type", ""));
                        for (String member : step.names()) {
                            if (member.equals("op") || member.equals("type")) {
                                continue;
                            }
                            payload.set(member, step.get(member));
                        }
                        step.set("payload", payload);
                    }
                    allSteps.add(step);
                }
            }
        }
        for (String caseName : caseNames) {
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("scenario", scenario.getString("name", scenario.getString("description", "")));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    /** Pre-configured variables mirroring TestSuiteEPLVariable.configure(). */
    private static void applyCaseVariables(Configuration config, String caseName) {
        switch (caseName) {
            case "onset-simple":
                config.getCommon().addVariable("var_simple_set", boolean.class, true);
                break;
            case "onset-with-filter":
                config.getCommon().addVariable("papi_1", String.class, "begin");
                config.getCommon().addVariable("papi_2", boolean.class, true);
                config.getCommon().addVariable("papi_3", String.class, "value");
                break;
            case "onset-order-no-dup":
                config.getCommon().addVariable("var1OND", Integer.class, 12);
                config.getCommon().addVariable("var2OND", Integer.class, 2);
                config.getCommon().addVariable("var3OND", Integer.class, null);
                break;
            case "onset-order-dup":
                config.getCommon().addVariable("var1OD", Integer.class, 0);
                config.getCommon().addVariable("var2OD", Integer.class, 1);
                config.getCommon().addVariable("var3OD", Integer.class, 2);
                break;
            case "onset-runtime-order-multiple":
                config.getCommon().addVariable("var1ROM", Integer.class, null);
                config.getCommon().addVariable("var2ROM", Integer.class, 1);
                break;
            case "onset-coercion":
                config.getCommon().addVariable("var1COE", Float.class, null);
                config.getCommon().addVariable("var2COE", Double.class, null);
                config.getCommon().addVariable("var3COE", Long.class, null);
                break;
            default:
                break;
        }
    }

    /** Cases whose set-statement iterator initial state is part of the oracle. */
    private static boolean needsInitialSnapshot(String caseName) {
        switch (caseName) {
            case "onset-with-filter":
            case "onset-order-no-dup":
            case "onset-order-dup":
            case "onset-runtime-order-multiple":
            case "onset-coercion":
                return true;
            default:
                return false;
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " not found");
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
            case "onset-simple":
                return new String[] {
                    "@name('set') on SupportBean_S0 set var_simple_set = false",
                    "@name('s0') select var_simple_set as c0 from SupportBean"
                };
            case "onset-with-filter":
                return new String[] {
                    "@name('set') on SupportBean(theString like 'S%') set papi_1='end', papi_2=false, papi_3=null"
                };
            case "onset-order-no-dup":
                return new String[] {
                    "@name('set') on SupportBean set var1OND=intPrimitive, var2OND=var1OND+1, var3OND=var1OND+var2OND"
                };
            case "onset-order-dup":
                return new String[] {
                    "@name('set') on SupportBean set var1OD=intPrimitive, var2OD=var2OD, var1OD=intBoxed, var3OD=var3OD+1"
                };
            case "onset-runtime-order-multiple":
                return new String[] {
                    "@name('set') on SupportBean(theString like 'S%' or theString like 'B%') set var1ROM=intPrimitive, var2ROM=intBoxed",
                    "@name('s0') select var1ROM, var2ROM, theString from SupportBean(theString like 'E%' or theString like 'B%')"
                };
            case "onset-coercion":
                return new String[] {
                    "@name('set') on SupportBean set var1COE=intPrimitive, var2COE=intPrimitive, var3COE=intBoxed",
                    "@name('s0') select irstream var1COE, var2COE, var3COE, id from SupportBean_A#length(2)"
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

        applyCaseVariables(config, caseName);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesOnSetScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPLs(caseName);
            // Compile as one module so @public variables are visible across statements
            String module = String.join(";\n", epls) + ";";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            int[] seq = new int[] {0};
            if (needsInitialSnapshot(caseName)) {
                EPStatement setStmt = findStatement(deployment, "set");
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "snapshot");
                record.add("statement", "set");
                record.add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                record.add("sequence", 0);
                JsonArray rows = new JsonArray();
                java.util.Iterator<EventBean> it = setStmt.iterator();
                while (it.hasNext()) {
                    EventBean event = it.next();
                    JsonObject item = new JsonObject();
                    item.add("kind", "row");
                    JsonObject fields = new JsonObject();
                    for (String prop : event.getEventType().getPropertyNames()) {
                        Object value = event.get(prop);
                        if (value instanceof Number) {
                            fields.add(prop, Json.value(((Number) value).longValue()));
                        } else {
                            fields.add(prop, value == null ? null : String.valueOf(value));
                        }
                    }
                    item.add("fields", fields);
                    rows.add(item);
                }
                record.add("new", rows);
                records.add(record);
            }
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
                    record.add("operation", "listener");
                    record.add("statement", stmtName);
                    record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                    record.add("sequence", seq[0]);
                    if (newData != null) {
                        JsonArray newArr = new JsonArray();
                        for (EventBean event : newData) {
                            JsonObject newItem = new JsonObject();
                            newItem.add("kind", "row");
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
                            oldItem.add("kind", "row");
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

            boolean inCase = false;
            for (JsonValue stepVal : steps) {
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
                    String type = step.getString("eventType", "");
                    JsonObject payload = step.get("payload").asObject();
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            bean.setTheString(payload.getString("theString", ""));
                            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                            if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                                bean.setIntBoxed(payload.getInt("intBoxed", 0));
                            }
                            runtime.getEventService().sendEventBean(bean, "SupportBean");
                            break;
                        }
                        case "SupportBean_A":
                            runtime.getEventService().sendEventBean(
                                new SupportBean_A(payload.getString("id", "")), "SupportBean_A");
                            break;
                        case "SupportBean_S0": {
                            int id = payload.getInt("id", 0);
                            String p00 = payload.get("p00") != null && !payload.get("p00").isNull()
                                ? payload.getString("p00", "") : null;
                            String p01 = payload.get("p01") != null && !payload.get("p01").isNull()
                                ? payload.getString("p01", "") : null;
                            runtime.getEventService().sendEventBean(
                                new SupportBean_S0(id, p00, p01), "SupportBean_S0");
                            break;
                        }
                        case "SupportBean_S1": {
                            int id = payload.getInt("id", 0);
                            String p10 = payload.get("p10") != null && !payload.get("p10").isNull()
                                ? payload.getString("p10", "") : null;
                            String p11 = payload.get("p11") != null && !payload.get("p11").isNull()
                                ? payload.getString("p11", "") : null;
                            runtime.getEventService().sendEventBean(
                                new SupportBean_S1(id, p10, p11), "SupportBean_S1");
                            break;
                        }
                        case "SupportMarketDataBean": {
                            Map<String, Object> marketEvent = new HashMap<>();
                            marketEvent.put("symbol", payload.getString("symbol", ""));
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
