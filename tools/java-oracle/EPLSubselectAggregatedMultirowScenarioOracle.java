import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for EPLSubselectAggregatedMultirowAndColumn scenarios.
 *
 * Covers the 12 included executions (15 scenario cases; 17 case names with
 * the unfiltered/filtered and nodelete/redeploy pairs sharing executions).
 * EPLSubselectMulticolumnInvalid is excluded (compile-time-only, approved
 * difference).
 *
 * Canonicalization: grouped-row collections (subq/e1/e2) are sorted ascending
 * by the per-case sortKey (c0 lexicographic for every case except
 * indexshare-multikey-array, which sorts by c1 numeric ASC), matching the
 * upstream suite's getSortMapMultiRow sort. int[] values render as arrays of
 * longs. Nested row maps render as bare sorted-fields objects.
 */
public class EPLSubselectAggregatedMultirowScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectAggregatedMultirowScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S1.class);
        Map<String, Object> intArrayType = new HashMap<>();
        intArrayType.put("id", String.class);
        intArrayType.put("array", int[].class);
        intArrayType.put("value", int.class);
        config.getCommon().addEventType("SupportEventWithIntArray", intArrayType);
        Map<String, Object> manyArrayType = new HashMap<>();
        manyArrayType.put("id", String.class);
        manyArrayType.put("intOne", int[].class);
        manyArrayType.put("value", int.class);
        config.getCommon().addEventType("SupportEventWithManyArray", manyArrayType);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectAggregatedMultirowScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epl = buildEPL(caseName); // {infra, s0}; infra may be ""
            if (!epl[0].isEmpty()) {
                deploy(runtime, epl[0]);
            }
            // Index-share cases pre-populate the named window before s0 deploys
            // (a later-deployed subquery must observe pre-existing window rows),
            // so s0 deployment is deferred until the first trigger-type send.
            String deferredTriggerType = deferredTriggerType(caseName);
            EPStatement[] s0 = new EPStatement[] {null};
            int[] seq = new int[] {0};
            if (deferredTriggerType == null) {
                deployS0(runtime, epl[1], caseName, records, seq, s0);
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
                    String type = step.getString("eventType", "");
                    if (s0[0] == null && type.equals(deferredTriggerType)) {
                        deployS0(runtime, epl[1], caseName, records, seq, s0);
                    }
                    sendEvent(runtime, step);
                } else if ("snapshot".equals(op)) {
                    if (s0[0] == null) {
                        throw new IllegalStateException("snapshot before s0 deploy in case " + caseName);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", step.getString("statement", "s0"));
                    record.add("sequence", 0);
                    record.add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                    JsonArray newArr = new JsonArray();
                    java.util.Iterator<EventBean> it = s0[0].iterator();
                    while (it.hasNext()) {
                        newArr.add(renderRow(it.next(), caseName));
                    }
                    record.add("new", newArr);
                    records.add(record);
                }
            }
            if (s0[0] == null) {
                throw new IllegalStateException("s0 was never deployed in case " + caseName);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void deploy(EPRuntime runtime, String epl) throws Exception {
        CompilerArguments args = new CompilerArguments(runtime.getConfigurationDeepCopy());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, args);
        runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    private static void deployS0(EPRuntime runtime, String epl, String caseName, List<JsonObject> records, int[] seq, EPStatement[] s0) throws Exception {
        CompilerArguments args = new CompilerArguments(runtime.getConfigurationDeepCopy());
        // Named-window cases compile s0 against the runtime's existing
        // deployments so create-window types resolve.
        args.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, args);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                s0[0] = candidate;
            }
        }
        if (s0[0] == null) {
            throw new IllegalStateException("no s0 statement in case " + caseName);
        }
        s0[0].addListener((newData, oldData, statement, rt) -> {
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
                    newArr.add(renderRow(event, caseName));
                }
                record.add("new", newArr);
                records.add(record);
            }
        });
    }

    private static JsonObject renderRow(EventBean event, String caseName) {
        JsonObject newItem = new JsonObject();
        newItem.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop), caseName));
        }
        newItem.add("fields", fields);
        return newItem;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean();
                event.setTheString(payload.getString("theString", ""));
                event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                event.setLongPrimitive(payload.getLong("longPrimitive", 0L));
                runtime.getEventService().sendEventBean(event, "SupportBean");
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                String p00 = payload.getString("p00", null);
                if (p00 != null) {
                    event.setP00(p00);
                }
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                break;
            }
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(payload.getInt("id", 0));
                String p10 = payload.getString("p10", null);
                if (p10 != null) {
                    event.setP10(p10);
                }
                runtime.getEventService().sendEventBean(event, "SupportBean_S1");
                break;
            }
            case "SupportEventWithIntArray": {
                Map<String, Object> event = new HashMap<>();
                event.put("id", payload.getString("id", ""));
                event.put("array", intArray(payload.get("array")));
                event.put("value", payload.getInt("value", 0));
                runtime.getEventService().sendEventMap(event, "SupportEventWithIntArray");
                break;
            }
            case "SupportEventWithManyArray": {
                Map<String, Object> event = new HashMap<>();
                event.put("id", payload.getString("id", ""));
                event.put("intOne", intArray(payload.get("intOne")));
                event.put("value", payload.getInt("value", 0));
                runtime.getEventService().sendEventMap(event, "SupportEventWithManyArray");
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    private static int[] intArray(JsonValue value) {
        JsonArray arr = value.asArray();
        int[] result = new int[arr.size()];
        for (int i = 0; i < arr.size(); i++) {
            result[i] = arr.get(i).asInt();
        }
        return result;
    }

    /** Per-case canonical sort key for grouped-row collections (upstream getSortMapMultiRow). */
    private static String sortKey(String caseName) {
        return "indexshare-multikey-array".equals(caseName) ? "c1" : "c0";
    }

    /** Trigger type whose first send deploys s0 (named-window pre-population cases). */
    private static String deferredTriggerType(String caseName) {
        switch (caseName) {
            case "indexshare-uncorrelated":
            case "indexshare-correlated":
            case "indexshare-multikey-array":
                return "SupportBean_S0";
            default:
                return null;
        }
    }

    private static String[] buildEPL(String caseName) {
        switch (caseName) {
            case "multirow-nodatawindow":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean group by theString)"
                        + ".take(10) as subq from SupportBean_S0"};
            case "multirow-enum-unfiltered":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall group by theString)"
                        + ".take(100) as subq from SupportBean_S0 as s0"};
            case "multirow-enum-filtered":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall"
                        + " where intPrimitive > 100 group by theString)"
                        + ".take(100) as subq from SupportBean_S0 as s0"};
            case "multirow-correlated-enum":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall"
                        + " where intPrimitive = s0.id group by theString)"
                        + ".take(100) as subq from SupportBean_S0 as s0"};
            case "multirow-correlated-having":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall"
                        + " where intPrimitive = s0.id group by theString having sum(intPrimitive) > 10)"
                        + ".take(100) as subq from SupportBean_S0 as s0"};
            case "indexshare-uncorrelated":
                return new String[] {
                    "@Hint('enable_window_subquery_indexshare') @public create window SBWindow#keepall as SupportBean;"
                        + "insert into SBWindow select * from SupportBean;",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SBWindow group by theString)"
                        + ".take(10) as e1 from SupportBean_S0"};
            case "indexshare-correlated":
                return new String[] {
                    "@Hint('enable_window_subquery_indexshare') @public create window SBWindow#keepall as SupportBean;"
                        + "insert into SBWindow select * from SupportBean;",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SBWindow"
                        + " where theString = s0.p00 group by theString)"
                        + ".take(10) as e1 from SupportBean_S0 as s0"};
            case "grouped-row-nodelete":
            case "grouped-row-nodelete-redeploy":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall"
                        + " group by theString) as subq from SupportBean_S0 as s0"};
            case "grouped-row-namedwindow-delete":
                return new String[] {
                    "@public create window MyWindow#keepall as SupportBean;"
                        + "insert into MyWindow select * from SupportBean;"
                        + "on SupportBean_S1 delete from MyWindow where id = intPrimitive;",
                    "@name('s0') @Hint('disable_reclaim_group') select (select theString as c0, sum(intPrimitive) as c1"
                        + " from MyWindow group by theString) as subq from SupportBean_S0 as s0"};
            case "grouped-row-multigroup":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, intPrimitive as c1, theString||'x' as c2,"
                        + " intPrimitive * 1000 as c3, sum(longPrimitive) as c4 from SupportBean#keepall"
                        + " group by theString, intPrimitive) as subq from SupportBean_S0 as s0"};
            case "grouped-iterator-exprdef":
                return new String[] {"",
                    "@name('s0') expression getGroups { (select theString as c0, sum(intPrimitive) as c1"
                        + " from SupportBean#keepall group by theString) }"
                        + " select getGroups() as e1, getGroups().take(10) as e2 from SupportBean_S0#lastevent()"};
            case "grouped-context-partitioned":
                return new String[] {"",
                    "create context MyCtx partition by theString from SupportBean, p00 from SupportBean_S0;"
                        + "@name('s0') context MyCtx select (select theString as c0, sum(intPrimitive) as c1"
                        + " from SupportBean#keepall group by theString) as subq from SupportBean_S0 as s0"};
            case "grouped-row-whaving":
                return new String[] {"",
                    // Verbatim duplicated annotation from the upstream suite (compiles in Esper).
                    "@name('s0') @name('s0')select (select theString as c0, sum(intPrimitive) as c1"
                        + " from SupportBean#keepall group by theString having sum(intPrimitive) > 10)"
                        + " as subq from SupportBean_S0"};
            case "grouped-keepall-take":
                return new String[] {"",
                    "@name('s0') select (select theString as c0, sum(intPrimitive) as c1 from SupportBean#keepall()"
                        + " group by theString).take(10) as e1 from SupportBean_S0"};
            case "multikey-array-scalar":
                return new String[] {"",
                    "@name('s0') select (select sum(value) as c0 from SupportEventWithIntArray#keepall group by array)"
                        + " as subq from SupportBean"};
            case "indexshare-multikey-array":
                return new String[] {
                    "@Hint('enable_window_subquery_indexshare') @public create window MyWindow#keepall"
                        + " as SupportEventWithManyArray;insert into MyWindow select * from SupportEventWithManyArray;",
                    "@name('s0') select (select intOne as c0, sum(value) as c1 from MyWindow group by intOne)"
                        + ".take(10) as e1 from SupportBean_S0"};
            default:
                throw new IllegalStateException("unknown case: " + caseName);
        }
    }

    private static JsonValue normalize(Object value, String caseName) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof SupportBean) {
            Map<String, Object> props = new HashMap<>();
            props.put("theString", ((SupportBean) value).getTheString());
            props.put("intPrimitive", ((SupportBean) value).getIntPrimitive());
            props.put("longPrimitive", ((SupportBean) value).getLongPrimitive());
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fieldsObject(props, caseName));
            return row;
        }
        if (value instanceof EventBean) {
            Object underlying = ((EventBean) value).getUnderlying();
            if (underlying instanceof Map) {
                JsonObject row = new JsonObject();
                row.add("kind", "row");
                row.add("fields", fieldsObject((Map<?, ?>) underlying, caseName));
                return row;
            }
            return normalize(underlying, caseName);
        }
        if (value instanceof Map) {
            return fieldsObject((Map<?, ?>) value, caseName);
        }
        if (value instanceof Collection) {
            // Grouped-row collections (subq/e1/e2): canonicalize by sorting the
            // rows ascending on the case's sortKey, mirroring the upstream
            // suite's getSortMapMultiRow (Java group-key order is not contractual).
            String key = sortKey(caseName);
            List<Map<?, ?>> rows = new ArrayList<>();
            for (Object item : (Collection<?>) value) {
                rows.add((Map<?, ?>) item);
            }
            rows.sort((left, right) -> {
                Object lv = left.get(key);
                Object rv = right.get(key);
                if (lv instanceof Number && rv instanceof Number) {
                    return Long.compare(((Number) lv).longValue(), ((Number) rv).longValue());
                }
                return String.valueOf(lv).compareTo(String.valueOf(rv));
            });
            JsonArray arr = new JsonArray();
            for (Map<?, ?> row : rows) {
                arr.add(fieldsObject(row, caseName));
            }
            return arr;
        }
        if (value instanceof int[] || value instanceof Integer[]) {
            JsonArray arr = new JsonArray();
            if (value instanceof int[]) {
                for (int item : (int[]) value) {
                    arr.add((long) item);
                }
            } else {
                for (Integer item : (Integer[]) value) {
                    arr.add(item == null ? Json.value(null) : Json.value(item.longValue()));
                }
            }
            return arr;
        }
        if (value instanceof Object[]) {
            JsonArray arr = new JsonArray();
            for (Object item : (Object[]) value) {
                arr.add(normalize(item, caseName));
            }
            return arr;
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

    private static JsonObject fieldsObject(Map<?, ?> values, String caseName) {
        JsonObject fields = new JsonObject();
        for (Object key : new TreeSet<>(values.keySet())) {
            fields.add(String.valueOf(key), normalize(values.get(key), caseName));
        }
        return fields;
    }
}
