import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.compiler.ConfigurationCompilerPlugInAggregationMultiFunction;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.approx.countminsketch.CountMinSketchAggState;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.regressionlib.support.extend.aggfunc.SupportCountBackAggregationFunctionForge;
import com.espertech.esper.regressionlib.support.extend.aggmultifunc.SupportReferenceCountedMapForge;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.io.FileWriter;
import java.io.IOException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.IdentityHashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for InfraTableResetAggregationState ords 0-5: table
 * aggregation reset() through on-merge update. Ord 0 resets the unkeyed sum
 * column by name (asum.reset()), ord 1 resets the whole row through the table
 * alias (mt.reset()), ord 2 resets selected columns of keyed groups through
 * merge where-clauses (avgone/winone via SupportBean_S0, avgtwo/wintwo via
 * SupportBean_S1), ord 3 resets a twelve-column unkeyed table including
 * plug-in and count-min-sketch aggregations while an s0 listener reads
 * MyTable.myWordcms.countMinSketchFrequency(p10), ord 4 replays the four
 * tryInvalidCompile probes, and ord 5 replays the compile-only doc sample.
 *
 * <p>Each case runs on one shared runtime with per-case deployment state
 * cleared at the case marker (matching undeployAll boundaries). The scenario
 * drives deploy/deployed/send/snapshot/build-error/build/undeploy-all steps;
 * the oracle records deployed markers, s0 listener deliveries, table iterator
 * snapshots, compile-rejected probes and the compiled marker.
 *
 * <p>Normalization notes: snapshot rows render through the same row shape as
 * listener rows ({"kind":"row","fields":{...}} under "new", fields sorted).
 * SupportBean/SupportBean_S0/SupportBean_S1 underlyings render their pinned
 * property maps; window(*) cells arrive as SupportBean[] and maxbyever as a
 * single SupportBean; referenceCountedMap arrives as a Map; the
 * countMinSketch() cell is a CountMinSketchAggState whose identity is not
 * observable (the Java suite reads it only through countMinSketchFrequency),
 * so it renders as the stable {"__type":"CountMinSketchAggState"} marker.
 * Doubles render as JSON numbers. Compile-rejected records carry the pinned
 * expectError prefix after the oracle verifies the caught message starts
 * with it (SupportMessageAssertUtil.assertMessage semantics).
 */
public class InfraTableResetScenarioOracle {
    private static final String SCENARIO_ID = "infra-table-reset-aggregation-state";

    public static void main(String[] args) throws Exception {
        if (args.length != 2) {
            System.err.println("usage: InfraTableResetScenarioOracle <scenario.json> <output.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(readFile(args[0])).asObject();
        JsonArray steps = scenario.get("steps").asArray();
        JsonObject out = new JsonObject();
        out.add("version", "esper-parity/v1");
        out.add("id", SCENARIO_ID);
        JsonArray records = new JsonArray();
        out.add("records", records);

        Configuration config = config();
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
                deploymentIds.clear();
                deployedModules.clear();
                statementsByName.clear();
                sequences.clear();
                listened.clear();
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
                    JsonObject rec = marker(caseName, "deployed", label, sequences);
                    records.add(rec);
                    break;
                }
                case "send":
                    sendStep(runtime, step);
                    break;
                case "snapshot":
                    snapshotStep(caseName, step, records, statementsByName, sequences);
                    break;
                case "build":
                    buildStep(caseName, step, records, sequences);
                    break;
                case "build-error":
                    buildErrorStep(caseName, step, records, sequences);
                    break;
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

    private static JsonObject marker(String caseName, String operation, String label,
                                     Map<String, Integer> sequences) {
        String key = label + ":" + operation;
        int seq = sequences.getOrDefault(key, 0) + 1;
        sequences.put(key, seq);
        JsonObject rec = new JsonObject();
        rec.add("case", caseName);
        rec.add("operation", operation);
        rec.add("statement", label);
        rec.add("sequence", seq);
        rec.add("time", "0");
        return rec;
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
            if ("s0".equals(stmt.getName())) {
                attachListener(runtime, caseName, stmt, records, sequences, listened);
            }
        }
    }

    /**
     * Iterates the named table statement, mirroring assertIterator /
     * assertPropsPerRowIteratorAnyOrder: one {"operation":"snapshot"} record
     * whose "new" array carries the iterator rows in iterator order (the
     * differential compare normalizes any-order pins).
     */
    private static void snapshotStep(String caseName, JsonObject step,
                                     JsonArray records, Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences) {
        String label = step.getString("statement", "");
        EPStatement statement = statementsByName.get(label);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot " + label);
        }
        JsonObject rec = marker(caseName, "snapshot", label, sequences);
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            rows.add(normalizeEvent(it.next()));
        }
        if (rows.size() > 0) {
            rec.add("new", rows);
        }
        records.add(rec);
    }

    /**
     * Compile-only step mirroring env.compile: compiles against the runtime
     * configuration without a path and without deploying, then emits the
     * {"operation":"compiled"} marker.
     */
    private static void buildStep(String caseName, JsonObject step, JsonArray records,
                                  Map<String, Integer> sequences) throws Exception {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        com.espertech.esper.compiler.client.CompilerArguments compilerArgs = new com.espertech.esper.compiler.client.CompilerArguments(config());
        com.espertech.esper.compiler.client.EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        records.add(marker(caseName, "compiled", label, sequences));
    }

    /**
     * Expected-invalid compile probe mirroring tryInvalidCompile: compiles
     * with a bare configuration (no path), requires an EPCompileException
     * whose message starts with the pinned expectError prefix, then emits
     * {"operation":"compile-rejected","value":<pinned prefix>}.
     */
    private static void buildErrorStep(String caseName, JsonObject step, JsonArray records,
                                       Map<String, Integer> sequences) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            com.espertech.esper.compiler.client.CompilerArguments compilerArgs = new com.espertech.esper.compiler.client.CompilerArguments(config());
            com.espertech.esper.compiler.client.EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject rec = marker(caseName, "compile-rejected", label, sequences);
        if (!expected.isEmpty()) {
            rec.add("value", expected);
        }
        records.add(rec);
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
        if (value instanceof SupportBean) {
            SupportBean bean = (SupportBean) value;
            JsonObject obj = new JsonObject();
            obj.add("theString", bean.getTheString());
            obj.add("intPrimitive", bean.getIntPrimitive());
            return obj;
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
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean event : (EventBean[]) value) {
                array.add(normalizeEvent(event));
            }
            return array;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalizeValue(item));
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
        if (value instanceof CountMinSketchAggState) {
            // The cell's object identity is not observable: the pinned suite
            // reads it only through countMinSketchFrequency (asserted by the
            // s0 listener records), so render a stable type marker.
            JsonObject obj = new JsonObject();
            obj.add("__type", "CountMinSketchAggState");
            return obj;
        }
        if (value instanceof Double || value instanceof Float) {
            return Json.value(((Number) value).doubleValue());
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
        config.getCompiler().addPlugInAggregationFunctionForge("myaggsingle",
            SupportCountBackAggregationFunctionForge.class.getName());
        config.getCompiler().addPlugInAggregationMultiFunction(
            new ConfigurationCompilerPlugInAggregationMultiFunction(
                "referenceCountedMap".split(","), SupportReferenceCountedMapForge.class.getName()));
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
