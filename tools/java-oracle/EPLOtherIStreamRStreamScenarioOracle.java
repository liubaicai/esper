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

/**
 * Java oracle for EPLOtherIStreamRStreamKeywords istream/rstream selector
 * scenarios. Each case deploys the same statements as the Java regression
 * execution (including the insert-into consumer where applicable), replays
 * the deterministic event sequence, and records observable listener output.
 *
 * OM/Compile variants are compile-API duplicates of RStreamOnly and are not
 * separately traced; RStreamOutputSnapshot is a compile-deploy-undeploy smoke
 * test without observable rows, excluded.
 */
public class EPLOtherIStreamRStreamScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLOtherIStreamRStreamScenarioOracle <scenario.json>");
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
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLOtherIStreamRStreamScenarioOracle", config);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> statements = new ArrayList<>();
            int[] seq = new int[] {0};
            // Compile all statements of the case as one module so the
            // insert-into declares the NextStream type for the consumer.
            String module = String.join(";\n", epls) + ";";
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
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
                    record.add("statement", stmtName);
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
                statements.add(candidate);
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
                    SupportBean bean = new SupportBean();
                    bean.setTheString(step.getString("theString", ""));
                    bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                    runtime.getEventService().sendEventBean(bean, "SupportBean");
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "rstream-only" -> new String[] {
                "@name('s0') select rstream * from SupportBean#length(3)"
            };
            case "rstream-insert-into" -> new String[] {
                "@name('s0') insert into NextStream select rstream s0.theString as theString from SupportBean#length(3) as s0",
                "@name('ii') select * from NextStream"
            };
            case "rstream-insert-into-rstream" -> new String[] {
                "@name('s0') insert rstream into NextStream select rstream s0.theString as theString from SupportBean#length(3) as s0",
                "@name('ii') select * from NextStream"
            };
            case "rstream-join" -> new String[] {
                "@name('s0') select rstream s1.intPrimitive as aID, s2.intPrimitive as bID " +
                "from SupportBean(theString='a')#length(2) as s1, " +
                "SupportBean(theString='b')#keepall as s2 " +
                "where s1.intPrimitive = s2.intPrimitive"
            };
            case "istream-only" -> new String[] {
                "@name('s0') select istream * from SupportBean#length(1)"
            };
            case "istream-insert-into-rstream" -> new String[] {
                "@name('s0') insert rstream into NextStream select istream a.theString as theString from SupportBean#length(1) as a",
                "@name('ii') select * from NextStream"
            };
            case "istream-join" -> new String[] {
                "@name('s0') select istream s1.intPrimitive as aID, s2.intPrimitive as bID " +
                "from SupportBean(theString='a')#length(2) as s1, " +
                "SupportBean(theString='b')#keepall as s2 " +
                "where s1.intPrimitive = s2.intPrimitive"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
