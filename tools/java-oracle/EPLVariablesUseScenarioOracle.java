import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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
import java.util.List;

/**
 * Java oracle for EPLVariablesUse variable-in-filter scenarios.
 *
 * Covers VariableInFilter, VariableInFilterBoolean, and SimpleSameModule
 * by replaying deterministic event sequences and recording observable output.
 */
public class EPLVariablesUseScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLVariablesUseScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S0.class);
        // Register variables needed by each case
        switch (caseName) {
            case "variable-in-filter":
                config.getCommon().addVariable("var1IF", String.class, null);
                break;
            case "variable-in-filter-boolean":
                config.getCommon().addVariable("var1IFB", String.class, null);
                config.getCommon().addVariable("var2IFB", String.class, null);
                break;
            case "simple-same-module":
                config.getCommon().addVariable("var_simple_module_const", Boolean.class, true);
                break;
        }
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesUseScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPLs(caseName);
            List<EPStatement> allStmts = new ArrayList<>();
            for (String epl : epls) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement stmt : deployment.getStatements()) {
                    allStmts.add(stmt);
                }
            }

            // Find the select statement (named "s0")
            EPStatement selectStmt = null;
            for (EPStatement stmt : allStmts) {
                if ("s0".equals(stmt.getName())) {
                    selectStmt = stmt;
                    break;
                }
            }
            if (selectStmt == null) {
                throw new IllegalStateException("statement s0 not found");
            }

            int[] seq = new int[] {0};
            selectStmt.addListener((newData, oldData, statement, rt) -> {
                if (newData != null) {
                    for (EventBean event : newData) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                        record.add("sequence", seq[0]);
                        JsonArray newArr = new JsonArray();
                        JsonObject newItem = new JsonObject();
                        newItem.add("kind", "row");
                        JsonObject fields = new JsonObject();
                        for (String prop : event.getEventType().getPropertyNames()) {
                            Object value = event.get(prop);
                            fields.add(prop, value == null ? null : value.toString());
                        }
                        newItem.add("fields", fields);
                        newArr.add(newItem);
                        record.add("new", newArr);
                        records.add(record);
                    }
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
                    String type = step.getString("eventType", "");
                    JsonObject payload = step.get("payload").asObject();
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            String ts = payload.getString("theString", null);
                            bean.setTheString(ts);
                            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                            runtime.getEventService().sendEventBean(bean, type);
                            break;
                        }
                        case "SupportBean_S0": {
                            SupportBean_S0 s0 = new SupportBean_S0(
                                payload.getInt("id", 0),
                                payload.getString("p00", ""),
                                payload.getString("p01", "")
                            );
                            runtime.getEventService().sendEventBean(s0, type);
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

    private static String[] buildEPLs(String caseName) {
        return switch (caseName) {
            case "variable-in-filter" -> new String[]{
                "@name('set') on SupportBean_S0 set var1IF = p00",
                "@name('s0') select theString, intPrimitive from SupportBean(theString = var1IF)"
            };
            case "variable-in-filter-boolean" -> new String[]{
                "@name('set') on SupportBean_S0 set var1IFB = p00, var2IFB = p01",
                "@name('s0') select theString, intPrimitive from SupportBean(theString = var1IFB or theString = var2IFB)"
            };
            case "simple-same-module" -> new String[]{
                "@name('s0') select var_simple_module_const as c0 from SupportBean"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
