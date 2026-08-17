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
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for EPLOtherSelectExprStreamSelector stream wildcard scenarios.
 *
 * Covers the NoJoinWildcardWithAlias, JoinWildcardWithAlias, AloneJoinAlias,
 * and AloneJoinNoAlias runtimes by replaying deterministic event sequences and
 * recording the observable output column names and event identity.
 *
 * SupportMarketDataBean is registered as a Map event type to avoid compiling
 * the full regression-lib module; the observable contract (property names,
 * event identity, column presence) is identical.
 */
public class EPLOtherStreamSelectorScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLOtherStreamSelectorScenarioOracle <scenario.json>");
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("volume", Long.class);
        marketType.put("price", Double.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLOtherStreamSelectorScenarioOracle", config);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            EPStatement stmt = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    stmt = candidate;
                    break;
                }
            }
            if (stmt == null) {
                throw new IllegalStateException("statement s0 not found");
            }

            int[] seq = new int[] {0};
            stmt.addListener((newData, oldData, statement, rt) -> {
                if (newData != null) {
                    for (EventBean event : newData) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("sequence", seq[0]);
                        JsonArray newArr = new JsonArray();
                        JsonObject newItem = new JsonObject();
                        JsonObject fields = new JsonObject();
                        for (String prop : event.getEventType().getPropertyNames()) {
                            Object value = event.get(prop);
                            if (value instanceof EventBean) {
                                EventBean inner = (EventBean) value;
                                fields.add(prop, inner.getEventType().getName() + ":" + summarizeUnderlying(inner));
                            } else {
                                fields.add(prop, value == null ? null : value.toString());
                            }
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
                    String type = step.getString("type", "");
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            bean.setTheString(step.getString("theString", ""));
                            bean.setIntPrimitive(step.getInt("intPrimitive", 0));
                            runtime.getEventService().sendEventBean(bean, type);
                            break;
                        }
                        case "SupportMarketDataBean": {
                            Map<String, Object> marketEvent = new HashMap<>();
                            marketEvent.put("symbol", step.getString("symbol", ""));
                            marketEvent.put("volume", step.getLong("volume", 0L));
                            marketEvent.put("price", step.getDouble("price", 0.0));
                            runtime.getEventService().sendEventMap(marketEvent, type);
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

    private static String summarizeUnderlying(EventBean event) {
        Object underlying = event.getUnderlying();
        if (underlying instanceof SupportBean) {
            SupportBean b = (SupportBean) underlying;
            return "SupportBean(theString=" + b.getTheString() + ",intPrimitive=" + b.getIntPrimitive() + ")";
        }
        if (underlying instanceof Map) {
            @SuppressWarnings("unchecked")
            Map<String, Object> m = (Map<String, Object>) underlying;
            return "SupportMarketDataBean(symbol=" + m.get("symbol") + ",volume=" + m.get("volume") + ")";
        }
        return String.valueOf(underlying);
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "nojoin-wildcard-alias" ->
                "@name('s0') select *, win.* as s0 from SupportBean#length(3) as win";
            case "join-wildcard-alias" ->
                "@name('s0') select *, s1.* as s1stream, s0.* as s0stream from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-join-alias" ->
                "@name('s0') select s1.* as s1 from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-join-noalias" ->
                "@name('s0') select s1.* from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
