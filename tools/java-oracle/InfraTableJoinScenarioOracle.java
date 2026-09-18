import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBeanSimple;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.io.PrintWriter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Scenario oracle for InfraTableJoin ords 0/3/4/5 (from-clause, unkeyed table,
 * outer join, inner join with on-clause). Ords 1/2 (index choice, coercion)
 * are intentionally-different because they assert JVM-internal query plans.
 *
 * <p>Each case runs on one shared runtime with per-case deployment state
 * cleared at the case marker (matching undeployAll boundaries). The scenario
 * drives deploy/deployed/send/faf/undeploy/undeploy-all steps; the oracle
 * records deployed markers and listener deliveries.
 */
public class InfraTableJoinScenarioOracle {
    private static final String SCENARIO_ID = "infra-table-join";

    public static void main(String[] args) throws Exception {
        if (args.length != 2) {
            System.err.println("usage: InfraTableJoinScenarioOracle <scenario.json> <output.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(readFile(args[0])).asObject();
        JsonArray steps = scenario.get("steps").asArray();
        JsonObject out = new JsonObject();
        out.add("version", "esper-parity/v1");
        out.add("id", SCENARIO_ID);
        JsonArray records = new JsonArray();
        out.add("records", records);

        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getCommon().addEventType(SupportBeanSimple.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("default", config);
        runtime.initialize();

        Map<String, String> deploymentIds = new HashMap<>();
        List<EPCompiled> deployedModules = new ArrayList<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        Map<String, Integer> sequences = new HashMap<>();
        Set<EPStatement> listened = Collections.newSetFromMap(new IdentityHashMap<>());
        boolean inCase = false;
        String caseName = "";
        for (JsonValue stepVal : steps) {
            JsonObject step = stepVal.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                caseName = step.getString("case", "");
                inCase = true;
                {
                    deploymentIds.clear();
                    deployedModules.clear();
                    statementsByName.clear();
                    sequences.clear();
                    listened.clear();
                }
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (op) {
                case "deploy":
                    deployStep(runtime, caseName, step, records, deploymentIds, statementsByName, sequences, listened, deployedModules);
                    break;
                case "deployed": {
                    String label = step.getString("statement", "");
                    String key = label + ":deployed";
                    int seq = sequences.getOrDefault(key, 0) + 1;
                    sequences.put(key, seq);
                    JsonObject rec = new JsonObject();
                    rec.add("case", caseName);
                    rec.add("operation", "deployed");
                    rec.add("statement", label);
                    rec.add("sequence", seq);
                    rec.add("time", "0");
                    records.add(rec);
                    break;
                }
                case "send":
                    sendStep(runtime, step);
                    break;
                case "faf": {
                    String epl = step.getString("epl", "");
                    com.espertech.esper.compiler.client.CompilerArguments fafArgs = new com.espertech.esper.compiler.client.CompilerArguments(config());
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled fafCompiled = com.espertech.esper.compiler.client.EPCompilerProvider.getCompiler().compileQuery(epl, fafArgs);
                    runtime.getFireAndForgetService().executeQuery(fafCompiled);
                    break;
                }
                case "undeploy": {
                    String label = step.getString("statement", "");
                    String depId = deploymentIds.get(label);
                    if (depId != null) {
                        runtime.getDeploymentService().undeploy(depId);
                        deploymentIds.remove(label);
                    }
                    break;
                }
                case "undeploy-all":
                    undeployAll(runtime, deploymentIds, statementsByName, listened);
                    break;
                default:
                    throw new IllegalStateException("unsupported op " + op);
            }
        }

        FileWriter writer = new FileWriter(args[1]);
        writer.write(out.toString());
        writer.close();
    }

    private static void deployStep(EPRuntime runtime, String caseName, JsonObject step,
                                   JsonArray records, Map<String, String> deploymentIds,
                                   Map<String, EPStatement> statementsByName,
                                   Map<String, Integer> sequences,
                                   Set<EPStatement> listened,
                                   List<EPCompiled> deployedModules) throws Exception {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        com.espertech.esper.compiler.client.CompilerArguments compilerArgs = new com.espertech.esper.compiler.client.CompilerArguments(config());
        for (EPCompiled deployed : deployedModules) {
            compilerArgs.getPath().add(deployed);
        }
        EPCompiled compiled = com.espertech.esper.compiler.client.EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        deployedModules.add(compiled);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        deploymentIds.put(label, deployment.getDeploymentId());
        for (EPStatement stmt : deployment.getStatements()) {
            statementsByName.put(stmt.getName(), stmt);
            if ("s0".equals(stmt.getName()) || "join".equals(stmt.getName())) {
                attachListener(runtime, caseName, stmt, records, sequences, listened);
            }
        }
    }

    private static void attachListener(EPRuntime runtime, String caseName, EPStatement stmt,
                                       JsonArray records, Map<String, Integer> sequences,
                                       Set<EPStatement> listened) {
        if (listened.contains(stmt)) {
            return;
        }
        listened.add(stmt);
        stmt.addListener(new UpdateListener() {
            public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
                String key = statement.getName() + ":listener";
                int seq = sequences.getOrDefault(key, 0) + 1;
                sequences.put(key, seq);
                JsonObject rec = new JsonObject();
                rec.add("case", caseName);
                rec.add("operation", "listener");
                rec.add("statement", statement.getName());
                rec.add("sequence", seq);
                rec.add("time", "0");
                if (newEvents != null && newEvents.length > 0) {
                    JsonArray newArr = new JsonArray();
                    for (EventBean event : newEvents) {
                        newArr.add(normalizeEvent(event));
                    }
                    rec.add("new", newArr);
                }
                if (oldEvents != null && oldEvents.length > 0) {
                    JsonArray oldArr = new JsonArray();
                    for (EventBean event : oldEvents) {
                        oldArr.add(normalizeEvent(event));
                    }
                    rec.add("old", oldArr);
                }
                records.add(rec);
            }
        });
    }

    private static JsonObject normalizeEvent(EventBean event) {
        JsonObject row = new JsonObject();
        row.add("kind", "row");
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalizeValue(event.get(name)));
        }
        row.add("fields", fields);
        return row;
    }

    private static JsonValue normalizeValue(Object value) {
        if (value instanceof EventBean) {
            value = ((EventBean) value).getUnderlying();
        }
        if (value == null) {
            return Json.NULL;
        }
        if (value instanceof SupportBean_S0) {
            SupportBean_S0 bean = (SupportBean_S0) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("p00", bean.getP00());
            obj.add("p01", bean.getP01());
            return obj;
        }
        if (value instanceof SupportBean_S1) {
            SupportBean_S1 bean = (SupportBean_S1) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("p10", bean.getP10());
            return obj;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalizeValue(item));
            }
            return array;
        }
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean event : (EventBean[]) value) {
                array.add(normalizeEvent(event));
            }
            return array;
        }
        if (value instanceof Map) {
            JsonObject obj = new JsonObject();
            Map<?, ?> map = (Map<?, ?>) value;
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                obj.add(key, normalizeValue(map.get(key)));
            }
            return obj;
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }

    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Object event;
        switch (type) {
            case "SupportBean":
                SupportBean bean = new SupportBean();
                bean.setTheString(payload.getString("theString", null));
                bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                bean.setLongPrimitive(payload.getLong("longPrimitive", 0L));
                event = bean;
                break;
            case "SupportBean_S0":
                event = new SupportBean_S0(
                    payload.getInt("id", 0),
                    payload.getString("p00", null),
                    payload.getString("p01", null));
                break;
            case "SupportBean_S1":
                event = new SupportBean_S1(
                    payload.getInt("id", 0),
                    payload.getString("p10", null));
                break;
            case "SupportBeanSimple":
                event = new SupportBeanSimple(
                    payload.getString("myString", null),
                    payload.getInt("myInt", 0));
                break;
            default:
                throw new IllegalStateException("unsupported event type " + type);
        }
        runtime.getEventService().sendEventBean(event, type);
    }

    private static void undeployAll(EPRuntime runtime, Map<String, String> deploymentIds,
                                    Map<String, EPStatement> statementsByName,
                                    Set<EPStatement> listened) {
        for (String depId : deploymentIds.values()) {
            try {
                runtime.getDeploymentService().undeploy(depId);
            } catch (Exception e) {
                // already undeployed
            }
        }
        deploymentIds.clear();
        statementsByName.clear();
        listened.clear();
    }

    private static Configuration config() {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getCommon().addEventType(SupportBeanSimple.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        return config;
    }

    private static String readFile(String path) throws IOException {
        StringBuilder sb = new StringBuilder();
        java.io.BufferedReader reader = new java.io.BufferedReader(new java.io.FileReader(path));
        String line;
        while ((line = reader.readLine()) != null) {
            sb.append(line).append('\n');
        }
        reader.close();
        return sb.toString();
    }
}
