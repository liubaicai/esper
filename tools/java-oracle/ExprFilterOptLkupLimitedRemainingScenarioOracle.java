import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.filterspec.FilterOperator;
import com.espertech.esper.common.internal.filterspec.FilterSpecParamForge;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.filter.SupportFilterPlanHook;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.UpdateListener;

import java.io.FileReader;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the expr-filter-opt-lkup-limited-remaining
 * parity scenario (ExprFilterOptimizableLookupableLimitedExpr remaining
 * executions: EqualsOneStmtWPatternSharingIndex, EqualsMultiStmtSharingIndex,
 * Disqualify, CurrentTimestampWEquals, CurrentTimestampCompare). Map event
 * types; sendEventMap; one JSON trace record per deployed marker, listener
 * invocation and unrepresentable probe on stdout.
 *
 * The disqualify case is compile-only (STATICHOOK): the oracle compiles the
 * objects preamble onto the runtime path, then compiles each pinned probe
 * with the INTERNAL_FILTERSPEC hook and asserts the BOOLEAN_EXPRESSION filter
 * operator in-process before emitting the pinned note record.
 */
public final class ExprFilterOptLkupLimitedRemainingScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-filter-opt-lkup-limited-remaining";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterOptimizableLookupableLimitedExpr.java";

    private static final String[] CASES = {
            "equals-pattern-sharing",
            "equals-multi-stmt-sharing",
            "disqualify",
            "current-timestamp-equals",
            "current-timestamp-compare",
    };
    private static final int[] ORDINALS = {1, 2, 6, 7, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-bfeb8b79de810cba8674",
            "java-runtime-2d0d6df65e1884579f65",
            "java-runtime-530153bd9731b1f9e439",
            "java-runtime-20e9b043737a1b6b2c0d",
            "java-runtime-0cd68bf813f2dd238a93",
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprFilterOptLkupEqualsOneStmtWPatternSharingIndex",
            "ExprFilterOptLkupEqualsMultiStmtSharingIndex",
            "ExprFilterOptLkupDisqualify",
            "ExprFilterOptLkupCurrentTimestampWEquals",
            "ExprFilterOptLkupCurrentTimestampCompare",
    };
    private static final String[] STATIC_IDS = {
            "java-d829f38c7827ed840c38",
            "java-f492b3ef1c2cf8f82889",
            "java-2bdf30f91311c50a7b13",
            "java-ce489fee11685f77be34",
            "java-7a65d419ecb8b96a4c38",
    };

    private static final String EPL_PATTERN_SHARING =
            "@name('s0') select * from pattern[every s0=SupportBean_S0 -> every SupportBean_S1('ax' = p10 || p11)] order by s0.id asc;\n";
    private static final String EPL_MULTI_STMT =
            "@name('s0') select * from SupportBean_S0(p00 || p01 = 'ax');\n" +
            "@name('s1') select * from SupportBean_S0(p00 || p01 = 'ax');\n" +
            "create constant variable string VAR = 'ax';\n" +
            "@name('s2') select * from SupportBean_S0(p00 || p01 = VAR);\n" +
            "create context MyContextOne start SupportBean_S1 as s1;\n" +
            "@name('s3') context MyContextOne select * from SupportBean_S0(p00 || p01 = context.s1.p10);\n" +
            "create context MyContextTwo start SupportBean_S1 as s1;\n" +
            "@name('s4') context MyContextTwo select * from pattern[a=SupportBean_S1 -> SupportBean_S0(a.p10 = p00     ||     p01)];\n";
    private static final String EPL_OBJECTS =
            "@public create variable string MYVARIABLE_NONCONSTANT = 'abc';\n" +
            "@public create table MyTable(tablecol string);\n" +
            "@public create window MyWindow#keepall as SupportBean;\n" +
            "@public create inlined_class \"\"\"\n" +
            "  public class Helper {\n" +
            "    public static String doit(Object param) { return null;}\n" +
            "    public static String doit(Object one, Object two) { return null;}\n" +
            "  }\n" +
            "\"\"\";\n" +
            "@public create expression MyDeclaredExpr { (select theString from MyWindow) };\n" +
            "@public create expression MyHandThrough {v => v};\n" +
            "@public create expression string js:MyJavaScript(param) [\"a\"];\n";
    private static final String EPL_CURRENT_TIMESTAMP_EQUALS =
            "@name('s0') select * from pattern[a=SupportBean -> SupportBean(a.longPrimitive = current_timestamp() + longPrimitive)];\n";
    private static final String EPL_CURRENT_TIMESTAMP_COMPARE =
            "@name('s0') select * from SupportBean(current_timestamp().getSecondOfMinute()%2=0);\n";

    private static final String HOOK =
            "@Hook(type=HookType.INTERNAL_FILTERSPEC, hook='com.espertech.esper.regressionlib.support.filter.SupportFilterPlanHook')";

    /** Pinned disqualify probes: label, byte-exact EPL, asserted event type, note. */
    private static final String[][] PROBES = {
            {"variable-nonconstant",
                    HOOK + "select * from SupportBean(theString||MYVARIABLE_NONCONSTANT='ax')",
                    "SupportBean",
                    "SupportBean(theString||MYVARIABLE_NONCONSTANT='ax') compiles with filter plan BOOLEAN_EXPRESSION (non-constant variable); no Go filter-plan hook"},
            {"table-column",
                    HOOK + "select * from SupportBean(theString||MyTable.tablecol='ax')",
                    "SupportBean",
                    "SupportBean(theString||MyTable.tablecol='ax') compiles with filter plan BOOLEAN_EXPRESSION (table column); no Go filter-plan hook"},
            {"subquery",
                    HOOK + "select * from SupportBean(theString||(select theString from MyWindow)='ax')",
                    "SupportBean",
                    "SupportBean(theString||(select theString from MyWindow)='ax') compiles with filter plan BOOLEAN_EXPRESSION (subquery); no Go filter-plan hook"},
            {"lambda",
                    HOOK + "select * from SupportBeanArrayCollMap(id || setOfString.where(v => v=id).firstOf() = 'ax')",
                    "SupportBeanArrayCollMap",
                    "SupportBeanArrayCollMap(id || setOfString.where(v => v=id).firstOf() = 'ax') compiles with filter plan BOOLEAN_EXPRESSION (lambda); no Go filter-plan hook"},
            {"script",
                    HOOK + "select * from pattern[s0=SupportBean_S0 -> SupportBean(MyJavaScript(theString)='x')]",
                    "SupportBean",
                    "pattern[s0=SupportBean_S0 -> SupportBean(MyJavaScript(theString)='x')] compiles with filter plan BOOLEAN_EXPRESSION (script expression); no Go filter-plan hook"},
            {"current-timestamp",
                    HOOK + "select * from SupportBean(current_timestamp()=1)",
                    "SupportBean",
                    "SupportBean(current_timestamp()=1) compiles with filter plan BOOLEAN_EXPRESSION (current_timestamp); no Go filter-plan hook"},
            {"inlined-class",
                    HOOK + "inlined_class \"\"\"\n" +
                    "  public class LocalHelper {\n" +
                    "    public static String doit(Object param) {\n" +
                    "      return null;\n" +
                    "    }\n" +
                    "  }\n" +
                    "\"\"\"\n" +
                    "select * from SupportBean(LocalHelper.doit(theString) = 'abc')",
                    "SupportBean",
                    "SupportBean(LocalHelper.doit(theString) = 'abc') compiles with filter plan BOOLEAN_EXPRESSION (statement-local inlined class); no Go filter-plan hook"},
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ExprFilterOptLkupLimitedRemainingScenarioOracle <scenario.json>");
        }
        JsonObject scenario;
        try (FileReader reader = new FileReader(args[0])) {
            scenario = Json.parse(reader).asObject();
        }
        validateMetadata(scenario);
        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            replayCase(scenario, CASES[index], index, records);
        }
        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    private static void validateMetadata(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))
                || !ID.equals(scenario.getString("id", ""))
                || !JAVA_COMMIT.equals(scenario.getString("javaCommit", ""))
                || !JAVA_SOURCE.equals(scenario.getString("javaSource", ""))) {
            throw new IllegalArgumentException("scenario metadata does not match the pinned values");
        }
        JsonArray cases = scenario.get("cases").asArray();
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly " + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = cases.get(index).asObject();
            if (!CASES[index].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[index].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }
    }

    private static void replayCase(JsonObject scenario, String caseName, int caseIndex,
                                   JsonArray records) throws Exception {
        Configuration config = new Configuration();
        // The regression harness runs with the internal timer disabled so
        // advanceTime is the only clock; without this the daemon thread
        // races the pinned advance-time steps and rewrites currentTime to
        // wall clock.
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new LinkedHashMap<>();
        beanType.put("longPrimitive", Long.class);
        beanType.put("theString", String.class);
        config.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new LinkedHashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        s0Type.put("p02", String.class);
        s0Type.put("p03", String.class);
        s0Type.put("value", Integer.class);
        config.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new LinkedHashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        s1Type.put("p12", String.class);
        s1Type.put("p13", String.class);
        config.getCommon().addEventType("SupportBean_S1", s1Type);
        Map<String, Object> collMapType = new LinkedHashMap<>();
        collMapType.put("id", String.class);
        collMapType.put("setOfString", String[].class);
        config.getCommon().addEventType("SupportBeanArrayCollMap", collMapType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + caseName, config);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            Map<String, EPDeployment> deployments = new HashMap<>();
            boolean inCase = false;
            for (JsonValue stepValue : scenario.get("steps").asArray()) {
                JsonObject step = stepValue.asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "deploy": {
                        String label = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        String pinned = pinnedDeploy(caseName, label);
                        if (pinned == null || !pinned.equals(epl)) {
                            throw new IllegalStateException("deploy " + label
                                    + " EPL is not pinned in case " + caseName);
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, new CompilerArguments(config));
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statement.addListener(listener(caseName, sequences, records, runtime));
                        }
                        break;
                    }
                    case "deployed": {
                        String label = step.getString("statement", "");
                        if (!deployments.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, step.getString("eventType", ""),
                                step.get("payload").asObject());
                        break;
                    case "unrepresentable":
                        unrepresentableStep(caseName, step, runtime, config, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    private static String pinnedDeploy(String caseName, String label) {
        switch (caseName) {
            case "equals-pattern-sharing":
                return "s0".equals(label) ? EPL_PATTERN_SHARING : null;
            case "equals-multi-stmt-sharing":
                return "module".equals(label) ? EPL_MULTI_STMT : null;
            case "current-timestamp-equals":
                return "s0".equals(label) ? EPL_CURRENT_TIMESTAMP_EQUALS : null;
            case "current-timestamp-compare":
                return "s0".equals(label) ? EPL_CURRENT_TIMESTAMP_COMPARE : null;
            default:
                return null;
        }
    }

    /**
     * Compiles the pinned probe in-process with the INTERNAL_FILTERSPEC hook,
     * asserts the BOOLEAN_EXPRESSION filter operator, then emits the pinned
     * note record. The objects preamble is compiled onto the path once per
     * probe run, mirroring assertDisqualified(env, path, typeName, epl).
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            EPRuntime runtime, Configuration config,
                                            JsonArray records) throws Exception {
        if (!"disqualify".equals(caseName)) {
            throw new IllegalStateException("unrepresentable step outside the disqualify case");
        }
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String note = step.getString("expectError", "");
        String[] probe = null;
        for (String[] candidate : PROBES) {
            if (candidate[0].equals(label)) {
                probe = candidate;
                break;
            }
        }
        if (probe == null || !probe[1].equals(epl) || !probe[3].equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label + " is not pinned");
        }
        CompilerArguments objectsArgs = new CompilerArguments(config);
        EPCompiled objects = EPCompilerProvider.getCompiler().compile(EPL_OBJECTS, objectsArgs);
        CompilerArguments probeArgs = new CompilerArguments(config);
        probeArgs.getPath().add(runtime.getRuntimePath());
        probeArgs.getPath().add(objects);
        SupportFilterPlanHook.reset();
        EPCompilerProvider.getCompiler().compile(epl, probeArgs);
        FilterSpecParamForge forge =
                SupportFilterPlanHook.assertPlanSingleForTypeAndReset(probe[2]);
        if (forge.getFilterOperator() != FilterOperator.BOOLEAN_EXPRESSION) {
            throw new IllegalStateException("probe " + label + " filter operator = "
                    + forge.getFilterOperator() + ", want BOOLEAN_EXPRESSION");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        Map<String, Object> event = new LinkedHashMap<>();
        for (String name : payload.names()) {
            JsonValue value = payload.get(name);
            if (value.isNull()) {
                event.put(name, null);
            } else if (value.isString()) {
                event.put(name, value.asString());
            } else if (value.isNumber()) {
                double number = value.asDouble();
                if (name.equals("longPrimitive")) {
                    event.put(name, (long) number);
                } else {
                    event.put(name, (int) number);
                }
            } else {
                event.put(name, value.toString());
            }
        }
        runtime.getEventService().sendEventMap(event, type);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty, and a
     * listener invocation that carries neither stream is a contract
     * violation.
     */
    private static UpdateListener listener(
            String caseName, Map<String, Integer> sequences,
            JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, EventBean fragments as nested row objects,
     * maps as sorted-key objects, Java arrays and collections as JSON arrays
     * — the same shapes the Go normalizer emits.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Map<?, ?> map) {
            JsonObject object = new JsonObject();
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                object.add(key, normalize(map.get(key)));
            }
            return object;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object item : (Collection<?>) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = java.lang.reflect.Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(java.lang.reflect.Array.get(value, index)));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }
}
