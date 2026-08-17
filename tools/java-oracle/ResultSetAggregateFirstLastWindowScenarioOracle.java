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
 * Java oracle for ResultSetAggregateFirstLastWindow executions covered by
 * existing Go aggregate API: UnboundedSimple, WindowedGrouped, BatchWindow,
 * BatchWindowGrouped, WindowAndSumWGroup.
 */
public class ResultSetAggregateFirstLastWindowScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetAggregateFirstLastWindowScenarioOracle <scenario.json>");
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

    private static String buildEPL(String caseName) {
        switch (caseName) {
            case "unbounded-simple":
                return "@name('s0') select first(theString) as c0, last(theString) as c1 from SupportBean";
            case "windowed-grouped":
                return "@name('s0') select theString, first(theString) firststring, last(theString) laststring, " +
                    "first(intPrimitive) firstint, last(intPrimitive) lastint, window(intPrimitive) allint " +
                    "from SupportBean#length(5) group by theString order by theString asc";
            case "batch-window":
                return "@name('s0') select irstream first(theString) fs, window(theString) ws, last(theString) ls " +
                    "from SupportBean#length_batch(2) as sb";
            case "batch-window-grouped":
                return "@name('s0') select theString, first(intPrimitive) fi, window(intPrimitive) wi, " +
                    "last(intPrimitive) li from SupportBean#length_batch(6) as sb " +
                    "group by theString order by theString asc";
            case "window-and-sum-wgroup":
                return "@name('s0') select theString c0, sum(intPrimitive) c1, " +
                    "window(intPrimitive*longPrimitive) c2 from SupportBean#length(3) " +
                    "group by theString order by theString asc";
            default:
                throw new IllegalArgumentException("unknown case: " + caseName);
        }
    }

    private static void runCase(JsonArray steps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("ResultSetAggregateFLWOracle", config);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
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

            for (JsonValue stepVal : steps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("send".equals(op)) {
                    String type = step.getString("type", "");
                    if ("SupportBean".equals(type)) {
                        SupportBean bean = new SupportBean();
                        bean.setTheString(step.getString("theString", ""));
                        bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                        if (step.get("longPrimitive") != null && !step.get("longPrimitive").isNull()) {
                            bean.setLongPrimitive(step.getLong("longPrimitive", 0));
                        }
                        if (step.get("doublePrimitive") != null && !step.get("doublePrimitive").isNull()) {
                            bean.setDoublePrimitive(step.getDouble("doublePrimitive", 0));
                        }
                        runtime.getEventService().sendEventBean(bean, "SupportBean");
                    }
                }
            }
        } finally {
            runtime.destroy();
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
                    arr.add(o == null ? (String) null : o.toString());
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
