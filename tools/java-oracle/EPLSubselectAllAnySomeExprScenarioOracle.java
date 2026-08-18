import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.TreeSet;

/**
 * Java oracle for EPLSubselectAllAnySomeExpr quantified-comparison scenarios.
 *
 * Covers 4 behavioral executions (5 scenario cases; relational-all-om is the
 * fresh-statement OM re-deploy round of RelationalOpAll). InvalidSubselect is
 * excluded (compile-time-only, approved difference). NullOrNoRows executions
 * are covered separately by case.subquery-empty-quantifiers.
 */
public class EPLSubselectAllAnySomeExprScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectAllAnySomeExprScenarioOracle <scenario.json>");
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
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectAllAnySomeExprScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epls[0], new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0Statements.add(candidate);
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
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
                    String type = step.getString("eventType", "SupportBean");
                    JsonObject payload = step.get("payload").asObject();
                    SupportBean event = new SupportBean();
                    event.setTheString(payload.getString("theString", ""));
                    event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                    runtime.getEventService().sendEventBean(event, "SupportBean");
                }
            }

        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "relational-all", "relational-all-om" -> new String[]{
                "@name('s0') select "
                    + "intPrimitive > all (select intPrimitive from SupportBean(theString like \"S%\")#keepall) as g, "
                    + "intPrimitive >= all (select intPrimitive from SupportBean(theString like \"S%\")#keepall) as ge, "
                    + "intPrimitive < all (select intPrimitive from SupportBean(theString like \"S%\")#keepall) as l, "
                    + "intPrimitive <= all (select intPrimitive from SupportBean(theString like \"S%\")#keepall) as le "
                    + "from SupportBean(theString like \"E%\")"
            };
            case "relational-some" -> new String[]{
                "@name('s0') select "
                    + "intPrimitive > any (select intPrimitive from SupportBean(theString like 'S%')#keepall) as g, "
                    + "intPrimitive >= any (select intPrimitive from SupportBean(theString like 'S%')#keepall) as ge, "
                    + "intPrimitive < any (select intPrimitive from SupportBean(theString like 'S%')#keepall) as l, "
                    + "intPrimitive <= any (select intPrimitive from SupportBean(theString like 'S%')#keepall) as le "
                    + " from SupportBean(theString like 'E%')"
            };
            case "equals-not-equals-all" -> new String[]{
                "@name('s0') select "
                    + "intPrimitive=all(select intPrimitive from SupportBean(theString like 'S%')#keepall) as eq, "
                    + "intPrimitive != all (select intPrimitive from SupportBean(theString like 'S%')#keepall) as neq, "
                    + "intPrimitive <> all (select intPrimitive from SupportBean(theString like 'S%')#keepall) as sqlneq, "
                    + "not intPrimitive = all (select intPrimitive from SupportBean(theString like 'S%')#keepall) as nneq "
                    + " from SupportBean(theString like 'E%')"
            };
            case "equals-any-or-some" -> new String[]{
                "@name('s0') select "
                    + "intPrimitive = SOME (select intPrimitive from SupportBean(theString like 'S%')#keepall) as r1, "
                    + "intPrimitive = ANY (select intPrimitive from SupportBean(theString like 'S%')#keepall) as r2, "
                    + "intPrimitive != SOME (select intPrimitive from SupportBean(theString like 'S%')#keepall) as r3, "
                    + "intPrimitive <> ANY (select intPrimitive from SupportBean(theString like 'S%')#keepall) as r4 "
                    + "from SupportBean(theString like 'E%')"
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
