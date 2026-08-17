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
 * Java oracle for ResultSetAggregateFirstLastWindow extended executions:
 * Subquery, OutputRateLimiting, LateInitialize, MixedNamedWindow.
 */
public class ResultSetAggregateFLWExtScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetAggregateFLWExtScenarioOracle <scenario.json>");
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

    private static void runCase(JsonArray steps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_A.class);
        config.getCommon().addEventType(SupportBean_S0.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("FLWExtOracle", config);
        try {
            switch (caseName) {
                case "subquery":
                    runSubquery(runtime, config, steps, caseName, records);
                    break;
                case "output-rate-limiting":
                    runOutputRateLimiting(runtime, config, steps, caseName, records);
                    break;
                case "late-initialize":
                    runLateInitialize(runtime, config, steps, caseName, records);
                    break;
                case "mixed-named-window":
                    runMixedNamedWindow(runtime, config, steps, caseName, records);
                    break;
                default:
                    throw new IllegalArgumentException("unknown case: " + caseName);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void runSubquery(EPRuntime runtime, Configuration config, JsonArray steps,
                                     String caseName, List<JsonObject> records) throws Exception {
        String epl = "@name('s0') select id, (select window(sb.*) from SupportBean#length(2) as sb) as w from SupportBean_A";
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        int[] seq = attachListener(deployment, caseName, records);

        for (JsonValue stepVal : steps) {
            JsonObject step = stepVal.asObject();
            if ("send".equals(step.getString("op", ""))) {
                sendEvent(runtime, step);
            }
        }
    }

    private static void runOutputRateLimiting(EPRuntime runtime, Configuration config, JsonArray steps,
                                               String caseName, List<JsonObject> records) throws Exception {
        String epl = "@name('s0') select sum(intPrimitive) si, window(sa.intPrimitive) wi " +
            "from SupportBean#keepall as sa output every 2 events";
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        int[] seq = attachListener(deployment, caseName, records);

        for (JsonValue stepVal : steps) {
            JsonObject step = stepVal.asObject();
            if ("send".equals(step.getString("op", ""))) {
                sendEvent(runtime, step);
            }
        }
    }

    private static void runLateInitialize(EPRuntime runtime, Configuration config, JsonArray steps,
                                           String caseName, List<JsonObject> records) throws Exception {
        // Deploy window + insert first; @public makes the window visible to later compilations
        String setupEpl = "@public create window MyWindowTwo#keepall as select * from SupportBean;\n" +
            "insert into MyWindowTwo select * from SupportBean";
        EPCompiled setupCompiled = EPCompilerProvider.getCompiler().compile(setupEpl, new CompilerArguments(config));
        runtime.getDeploymentService().deploy(setupCompiled, new DeploymentOptions());

        int[] seq = new int[] {0};
        for (JsonValue stepVal : steps) {
            JsonObject step = stepVal.asObject();
            String op = step.getString("op", "");
            if ("send".equals(op)) {
                sendEvent(runtime, step);
            } else if ("deploy-consumer".equals(op)) {
                String consumerEpl = "@name('s0') select first(theString) firststring, " +
                    "window(theString) windowstring, last(theString) laststring from MyWindowTwo";
                EPCompiled consumerCompiled = EPCompilerProvider.getCompiler().compile(consumerEpl, new CompilerArguments(runtime.getRuntimePath()));
                EPDeployment consumerDeployment = runtime.getDeploymentService().deploy(consumerCompiled, new DeploymentOptions());
                int[] consumerSeq = attachListener(consumerDeployment, caseName, records);
                seq[0] = consumerSeq[0];
            }
        }
    }

    private static void runMixedNamedWindow(EPRuntime runtime, Configuration config, JsonArray steps,
                                             String caseName, List<JsonObject> records) throws Exception {
        String setupEpl = "@public create window ABCWin#keepall as select * from SupportBean;\n" +
            "insert into ABCWin select * from SupportBean;\n" +
            "on SupportBean_S0 delete from ABCWin where intPrimitive = id";
        EPCompiled setupCompiled = EPCompilerProvider.getCompiler().compile(setupEpl, new CompilerArguments(config));
        runtime.getDeploymentService().deploy(setupCompiled, new DeploymentOptions());

        String consumerEpl = "@name('s0') select theString c0, sum(intPrimitive) c1, window(intPrimitive) c2 " +
            "from ABCWin group by theString";
        EPCompiled consumerCompiled = EPCompilerProvider.getCompiler().compile(consumerEpl, new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment consumerDeployment = runtime.getDeploymentService().deploy(consumerCompiled, new DeploymentOptions());
        attachListener(consumerDeployment, caseName, records);

        for (JsonValue stepVal : steps) {
            JsonObject step = stepVal.asObject();
            if ("send".equals(step.getString("op", ""))) {
                sendEvent(runtime, step);
            }
        }
    }

    private static int[] attachListener(EPDeployment deployment, String caseName, List<JsonObject> records) {
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
                        newArr.add(eventToJson(event));
                    }
                    record.add("new", newArr);
                }
                if (oldData != null) {
                    JsonArray oldArr = new JsonArray();
                    for (EventBean event : oldData) {
                        oldArr.add(eventToJson(event));
                    }
                    record.add("old", oldArr);
                }
                records.add(record);
            });
        }
        return seq;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("type", "");
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(step.getString("theString", ""));
                bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                if (step.get("longPrimitive") != null && !step.get("longPrimitive").isNull()) {
                    bean.setLongPrimitive(step.getLong("longPrimitive", 0));
                }
                runtime.getEventService().sendEventBean(bean, "SupportBean");
                break;
            }
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(
                    new SupportBean_A(step.getString("id", "")), "SupportBean_A");
                break;
            case "SupportBean_S0":
                runtime.getEventService().sendEventBean(
                    new SupportBean_S0(step.getInt("id", 0)), "SupportBean_S0");
                break;
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static JsonObject eventToJson(EventBean event) {
        JsonObject item = new JsonObject();
        JsonObject fields = new JsonObject();
        for (String prop : event.getEventType().getPropertyNames()) {
            Object value = event.get(prop);
            if (value == null) {
                fields.add(prop, (String) null);
            } else if (value instanceof Object[]) {
                JsonArray arr = new JsonArray();
                for (Object o : (Object[]) value) {
                    if (o instanceof EventBean) {
                        EventBean bean = (EventBean) o;
                        JsonObject beanObj = new JsonObject();
                        for (String bp : bean.getEventType().getPropertyNames()) {
                            Object bv = bean.get(bp);
                            beanObj.add(bp, bv == null ? (String) null : bv.toString());
                        }
                        arr.add(beanObj);
                    } else {
                        arr.add(o == null ? (String) null : o.toString());
                    }
                }
                fields.add(prop, arr);
            } else {
                fields.add(prop, value.toString());
            }
        }
        item.add("fields", fields);
        return item;
    }
}
