import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.PropertyAccessException;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.expr.EPLMethodInvocationContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.client.option.StatementSubstitutionParameterContext;
import com.espertech.esper.runtime.client.option.StatementSubstitutionParameterOption;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;

/**
 * Java oracle for the expr-filter-optimizable parity scenario (pinned Esper
 * 9.0.0 commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c). Replays nine
 * listener-observable executions of
 * regression-lib/.../suite/expr/filter/ExprFilterOptimizable.java against one
 * fresh runtime per case, with the pinned EPL deployed through the compiler
 * plus deployment service:
 *
 * - in-and-not-in-multivalue (ordinal 0): SupportInKeywordBean#length(2)
 *   "1 in/not in (ints)" plain filters, both pattern forms
 *   (every a=SupportInKeywordBean -> SupportBean(intPrimitive in/not in
 *   (a.ints))) and the initiated context pair s1 (#keepall where ...) / s2
 *   (filter) with context.mie.ints.
 * - method-invocation-context (ordinal 1): myCustomOkFunction(e) = "OK"
 *   filter; the UDF observes the EPLMethodInvocationContext and the trace
 *   emits four "observation" records (runtimeURI, functionName,
 *   statementUserObject, contextPartitionId) followed by the listener
 *   record of the same send, all carrying the same per-statement sequence.
 * - typeof (ordinal 2): typeof(e) = 'SupportOverrideBase' fires for the
 *   base event type and not for the derived SupportOverrideOne type.
 * - variable-and-separate-thread (ordinal 3): the myCheckServiceProvider
 *   variable holding a service whose check() returns true.
 * - or-to-in-rewrite (ordinal 5): all four operand orderings of
 *   theString='a' or theString='b'.
 * - or-context (ordinal 6): context initiated by SupportBean with the
 *   'select' statement filtering theString='A' or intPrimitive=1.
 * - pattern-udf (ordinal 7): pattern a=SupportBean() ->
 *   b=SupportBean(myCustomBigDecimalEquals(a.bigDecimal, b.bigDecimal)).
 * - deploy-time-constant (ordinal 8): substitution-parameter and variable
 *   forms for equals (both directions), long coercion, relop (both
 *   directions), in (both directions), in-array and between numeric and
 *   string.
 * - regex-many-or (ordinal 9): 17 identical regexp clauses joined by OR.
 *
 * Ordinal 4 (ExprFilterOptimizableInspectFilter) asserts only engine-internal
 * filter-plan/operator shape via getFilterSvcSingle and has no observable
 * fire/no-fire contract; it is intentionally-different and not replayed.
 * The invalid compile-only sub-cases (long -> int strict filter coercion in
 * the pattern and the intPrimitive=?:p0:long substitution) are skipped as
 * well. Plan-shape assertions (IN_LIST_OF_VALUES / EQUAL / GREATER /
 * RANGE_CLOSED) are engine-internal and out of scope for this trace.
 *
 * Listener records follow the standard protocol: one record per delivery,
 * per-statement sequence numbering from 1 per deployment, time rendered
 * from engine time, new/old arrays rendered with the scalar normalization
 * rules (Integer/Long/Short/Byte -> long, other Number -> double including
 * BigDecimal, null -> {state:null}). Pattern tag values are the raw map
 * payloads, rendered with their submitted keys; nested EventBean values are
 * rendered with __type plus sorted properties.
 *
 * Scenario step ops: case markers ("case"), sends ("send"), pinned module
 * deployments ("deploy" with statement label matching the pinned plan, and
 * "payload" carrying statement substitution parameters for the
 * deploy-time-constant case).
 */
public class ExprFilterOptimizableScenarioOracle {

    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private static final String[] CASES = {
        "in-and-not-in-multivalue",
        "method-invocation-context",
        "typeof",
        "variable-and-separate-thread",
        "or-to-in-rewrite",
        "or-context",
        "pattern-udf",
        "deploy-time-constant",
        "regex-many-or"
    };

    /** EvalContext captured by myCustomOkFunction during the last send. */
    private static volatile EPLMethodInvocationContext observedMethodInvocationContext;

    private ExprFilterOptimizableScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ExprFilterOptimizableScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(
            Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();

        List<JsonObject> records = new ArrayList<>();
        for (String caseName : CASES) {
            if (hasCase(allSteps, caseName)) {
                runCase(allSteps, caseName, records);
            }
        }

        JsonObject root = new JsonObject()
            .add("version", "esper-parity/v1")
            .add("id", scenario.getString("id", ""))
            .add("scenario", scenario.getString("description", ""))
            .add("javaCommit", COMMIT)
            .add("java", System.getProperty("java.version"));
        JsonArray recordsArray = new JsonArray();
        for (JsonObject record : records) {
            recordsArray.add(record);
        }
        root.add("records", recordsArray);
        System.out.println(root.toString());
    }

    private static boolean hasCase(JsonArray allSteps, String wanted) {
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = buildConfiguration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-efo-" + caseName, config);
        ((EPRuntimeSPI) runtime).initialize(0L);

        ScenarioPlan[] plan = plans(caseName);
        int deployIndex = 0;
        boolean active = false;
        try {
            for (int i = 0; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("send".equals(op)) {
                    // ExprFilterOptimizableVariableAndSeparateThread sends the
                    // event from a separate single-thread executor and gates
                    // on a 10-second latch; keep that execution shape.
                    if ("variable-and-separate-thread".equals(caseName)) {
                        sendOnSeparateThread(runtime, step);
                    } else {
                        sendEvent(runtime, step);
                    }
                    continue;
                }
                if ("deploy".equals(op)) {
                    String label = step.getString("statement", "");
                    if (deployIndex >= plan.length || !label.equals(plan[deployIndex].label())) {
                        throw new IllegalStateException(
                            "case " + caseName + " deploy step " + deployIndex + " does not match its pinned plan");
                    }
                    // The pinned suite undeploys each sub-case before the
                    // next deployment; a new deploy op replaces the prior one.
                    runtime.getDeploymentService().undeployAll();
                    deployPinned(runtime, config, records, caseName, deployIndex, step);
                    deployIndex++;
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }
            if (deployIndex != plan.length) {
                throw new IllegalStateException(
                    "case " + caseName + " has " + deployIndex + " deploy steps, plan expects " + plan.length);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void deployPinned(EPRuntime runtime, Configuration config, List<JsonObject> records,
                                     String caseName, int deployIndex, JsonObject step) throws Exception {
        ScenarioPlan plan = plans(caseName)[deployIndex];
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(plan.epl(), new CompilerArguments(config));
        DeploymentOptions options = new DeploymentOptions().setDeploymentId("parity-efo-" + caseName + "-" + deployIndex);
        Map<String, Object> params = extractParameters(step);
        if (!params.isEmpty()) {
            options.setStatementSubstitutionParameter(new DeploymentSubstitutionParams(params));
        }
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, options);
        for (String observe : plan.observed()) {
            EPStatement statement = findStatement(deployment, observe);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
        }
    }

    private static Map<String, Object> extractParameters(JsonObject step) {
        Map<String, Object> params = new LinkedHashMap<>();
        JsonValue payload = step.get("payload");
        if (!(payload instanceof JsonObject)) {
            return params;
        }
        JsonObject paramsObject = payload.asObject();
        for (String name : paramsObject.names()) {
            JsonValue value = paramsObject.get(name);
            if (value instanceof JsonString) {
                params.put(name, ((JsonString) value).asString());
            } else if (value instanceof JsonNumber) {
                params.put(name, (int) ((JsonNumber) value).asLong());
            } else if (value instanceof JsonArray) {
                JsonArray array = value.asArray();
                int[] intArray = new int[array.size()];
                for (int i = 0; i < array.size(); i++) {
                    intArray[i] = (int) ((JsonNumber) array.get(i)).asLong();
                }
                params.put(name, intArray);
            } else {
                throw new IllegalStateException("unsupported substitution parameter value for " + name);
            }
        }
        return params;
    }

    private static final class DeploymentSubstitutionParams implements StatementSubstitutionParameterOption {
        private final Map<String, Object> values;

        private DeploymentSubstitutionParams(Map<String, Object> values) {
            this.values = values;
        }

        public void setStatementParameters(StatementSubstitutionParameterContext env) {
            for (Map.Entry<String, Object> entry : values.entrySet()) {
                env.setObject(entry.getKey(), entry.getValue());
            }
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    /** Pinned module transcriptions, in pinned execution order per case. */
    private static ScenarioPlan[] plans(String caseName) {
        return switch (caseName) {
            case "in-and-not-in-multivalue" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportInKeywordBean#length(2) where 1 in (ints)",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every a=SupportInKeywordBean -> SupportBean(intPrimitive in (a.ints))]",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportInKeywordBean#length(2) where 1 not in (ints)",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every a=SupportInKeywordBean -> SupportBean(intPrimitive not in (a.ints))]",
                    new String[]{"s0"}),
                new ScenarioPlan("s1",
                    "create context MyContext initiated by SupportInKeywordBean as mie terminated after 24 hours;\n" +
                        "@name('s1') context MyContext select * from SupportBean#keepall where intPrimitive in (context.mie.ints);\n" +
                        "@name('s2') context MyContext select * from SupportBean(intPrimitive in (context.mie.ints));",
                    new String[]{"s1", "s2"}),
            };
            case "method-invocation-context" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean e where myCustomOkFunction(e) = \"OK\"",
                    new String[]{"s0"}),
            };
            case "typeof" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportOverrideBase(typeof(e) = 'SupportOverrideBase') as e",
                    new String[]{"s0"}),
            };
            case "variable-and-separate-thread" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean(myCheckServiceProvider.check())",
                    new String[]{"s0"}),
            };
            case "or-to-in-rewrite" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean(theString = 'a' or theString = 'b')",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean(theString = 'a' or 'b' = theString)",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean('a' = theString or 'b' = theString)",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from SupportBean('a' = theString or theString = 'b')",
                    new String[]{"s0"}),
            };
            case "or-context" -> new ScenarioPlan[] {
                new ScenarioPlan("select",
                    "@name('ctx') create context MyContext initiated by SupportBean terminated after 24 hours;\n" +
                        "@name('select') context MyContext select * from SupportBean(theString='A' or intPrimitive=1)",
                    new String[]{"select"}),
            };
            case "pattern-udf" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[a=SupportBean() -> b=SupportBean(myCustomBigDecimalEquals(a.bigDecimal, b.bigDecimal))]",
                    new String[]{"s0"}),
            };
            case "deploy-time-constant" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(theString=?:p0:string)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(?:p0:string=theString)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(theString=var_optimizable_equals)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(var_optimizable_equals=theString)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(longPrimitive=?:p0:int)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(?:p0:int=longPrimitive)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive>?:p0:int)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(?:p0:int<intPrimitive)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive>var_optimizable_relop)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(var_optimizable_relop<intPrimitive)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive in (?:p0:int, ?:p1:int))", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive in (var_optimizable_start, var_optimizable_end))", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive in (?:p0:int[primitive]))", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive in (var_optimizable_array))", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive between ?:p0:int and ?:p1:int)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(intPrimitive between var_optimizable_start and var_optimizable_end)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(theString between ?:p0:string and ?:p1:string)", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBean(theString between var_optimizable_start_string and var_optimizable_end_string)", new String[]{"s0"}),
            };
            case "regex-many-or" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", regexManyOrEpl(), new String[]{"s0"}),
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    /** 17 identical theString regexp ".*test.*" clauses joined by OR. */
    private static String regexManyOrEpl() {
        StringBuilder epl = new StringBuilder(
            "@name('s0') select * from SupportBean(theString regexp \".*test.*\"");
        for (int i = 2; i <= 17; i++) {
            epl.append(" or theString regexp \".*test.*\"");
        }
        return epl.append(")").toString();
    }

    private static Configuration buildConfiguration() {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        Map<String, Object> supportBean = new LinkedHashMap<>();
        supportBean.put("theString", String.class);
        supportBean.put("intPrimitive", int.class);
        supportBean.put("longPrimitive", long.class);
        supportBean.put("bigDecimal", BigDecimal.class);
        config.getCommon().addEventType("SupportBean", supportBean);

        Map<String, Object> inKeyword = new LinkedHashMap<>();
        inKeyword.put("ints", int[].class);
        inKeyword.put("longs", long[].class);
        inKeyword.put("mapOfIntKey", Map.class);
        inKeyword.put("collOfInt", Collection.class);
        config.getCommon().addEventType("SupportInKeywordBean", inKeyword);

        Map<String, Object> overrideBase = new LinkedHashMap<>();
        overrideBase.put("val", String.class);
        config.getCommon().addEventType("SupportOverrideBase", overrideBase);

        Map<String, Object> overrideOne = new LinkedHashMap<>();
        overrideOne.put("val", String.class);
        overrideOne.put("valOne", String.class);
        config.getCommon().addEventType("SupportOverrideOne", overrideOne);

        config.getCommon().addVariable("myCheckServiceProvider", LocalCheckServiceProvider.class,
            new LocalCheckServiceProvider());
        config.getCommon().addVariable("var_optimizable_equals", String.class, "abc", true);
        config.getCommon().addVariable("var_optimizable_relop", int.class, 10, true);
        config.getCommon().addVariable("var_optimizable_start", int.class, 10, true);
        config.getCommon().addVariable("var_optimizable_end", int.class, 11, true);
        config.getCommon().addVariable("var_optimizable_array", "int[]", new Integer[]{10, 11}, true);
        config.getCommon().addVariable("var_optimizable_start_string", String.class, "c", true);
        config.getCommon().addVariable("var_optimizable_end_string", String.class, "d", true);

        config.getCompiler().addPlugInSingleRowFunction("myCustomOkFunction",
            ExprFilterOptimizableScenarioOracle.class.getName(), "myCustomOkFunction");
        config.getCompiler().addPlugInSingleRowFunction("myCustomBigDecimalEquals",
            ExprFilterOptimizableScenarioOracle.class.getName(), "myCustomBigDecimalEquals");
        return config;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("theString", payload.getString("theString", null));
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                event.put("longPrimitive", payload.getLong("longPrimitive", 0L));
                event.put("bigDecimal", bigDecimal(payload));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportInKeywordBean" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("ints", intArray(payload.get("ints")));
                event.put("longs", longArray(payload.get("longs")));
                event.put("mapOfIntKey", intKeyMap(payload.get("mapOfIntKey")));
                event.put("collOfInt", intList(payload.get("collOfInt")));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportOverrideBase" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("val", payload.getString("val", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportOverrideOne" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("val", payload.getString("val", null));
                event.put("valOne", payload.getString("valOne", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            default -> throw new IllegalStateException("unknown eventType: " + eventType);
        }
    }

    private static void sendOnSeparateThread(EPRuntime runtime, JsonObject step) throws Exception {
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try {
            Future<?> future = executor.submit(() -> sendEvent(runtime, step));
            // Mirrors the pinned latch.await(10 seconds): the fire must
            // complete within the timeout.
            future.get(10, TimeUnit.SECONDS);
        } finally {
            executor.shutdown();
        }
    }

    private static BigDecimal bigDecimal(JsonObject payload) {
        JsonValue value = payload.get("bigDecimal");
        if (value instanceof JsonNumber) {
            return BigDecimal.valueOf(((JsonNumber) value).asLong());
        }
        return null;
    }

    private static int[] intArray(JsonValue value) {
        if (value instanceof JsonArray) {
            JsonArray array = value.asArray();
            int[] result = new int[array.size()];
            for (int i = 0; i < array.size(); i++) {
                result[i] = (int) ((JsonNumber) array.get(i)).asLong();
            }
            return result;
        }
        return null;
    }

    private static long[] longArray(JsonValue value) {
        if (value instanceof JsonArray) {
            JsonArray array = value.asArray();
            long[] result = new long[array.size()];
            for (int i = 0; i < array.size(); i++) {
                result[i] = ((JsonNumber) array.get(i)).asLong();
            }
            return result;
        }
        return null;
    }

    private static Map<Integer, String> intKeyMap(JsonValue value) {
        if (value instanceof JsonObject) {
            JsonObject map = value.asObject();
            Map<Integer, String> result = new LinkedHashMap<>();
            for (String key : map.names()) {
                result.put(Integer.valueOf(key), map.getString(key, null));
            }
            return result;
        }
        return null;
    }

    private static List<Integer> intList(JsonValue value) {
        if (value instanceof JsonArray) {
            JsonArray array = value.asArray();
            List<Integer> result = new ArrayList<>(array.size());
            for (int i = 0; i < array.size(); i++) {
                result.add((int) ((JsonNumber) array.get(i)).asLong());
            }
            return result;
        }
        return null;
    }

    /** Single-row function capturing the EvalContext of the filtered event. */
    public static String myCustomOkFunction(Object event, EPLMethodInvocationContext context) {
        observedMethodInvocationContext = context;
        return "OK";
    }

    /** BigDecimal comparison via compareTo, as asserted by the pattern filter. */
    public static boolean myCustomBigDecimalEquals(BigDecimal first, BigDecimal second) {
        return first.compareTo(second) == 0;
    }

    private record ScenarioPlan(String label, String epl, String[] observed) {
    }

    private static final class TraceWriter implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(List<JsonObject> records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            sequence++;
            String time = Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString();
            if ("method-invocation-context".equals(caseName) && observedMethodInvocationContext != null) {
                EPLMethodInvocationContext context = observedMethodInvocationContext;
                observedMethodInvocationContext = null;
                records.add(observation("runtimeURI", context.getRuntimeURI(), sequence, time));
                records.add(observation("functionName", context.getFunctionName(), sequence, time));
                records.add(observation("statementUserObject", context.getStatementUserObject(), sequence, time));
                records.add(observation("contextPartitionId", context.getContextPartitionId(), sequence, time));
            }
            JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "listener")
                .add("statement", statement.getName())
                .add("sequence", sequence)
                .add("time", time);
            JsonArray newArray = rows(newEvents);
            JsonArray oldArray = rows(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            records.add(record);
        }

        private JsonObject observation(String name, Object value, long sequence, String time) {
            return new JsonObject()
                .add("case", caseName)
                .add("operation", "observation")
                .add("statement", statement.getName())
                .add("sequence", sequence)
                .add("time", time)
                .add("name", name)
                .add("value", normalize(value));
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray array = new JsonArray();
            if (events == null) {
                return array;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                java.util.Arrays.sort(names);
                for (String name : names) {
                    Object value;
                    try {
                        value = event.get(name);
                    } catch (PropertyAccessException unreadable) {
                        continue;
                    }
                    fields.add(name, normalize(value));
                }
                array.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return array;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean eventBean) {
                JsonObject object = new JsonObject();
                object.add("__type", eventBean.getEventType().getName());
                String[] names = eventBean.getEventType().getPropertyNames().clone();
                java.util.Arrays.sort(names);
                for (String name : names) {
                    Object member;
                    try {
                        member = eventBean.get(name);
                    } catch (PropertyAccessException unreadable) {
                        continue;
                    }
                    object.add(name, normalize(member));
                }
                return object;
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
            if (value instanceof Collection<?> collection) {
                JsonArray array = new JsonArray();
                for (Object item : collection) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value != null && value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            return Json.value(String.valueOf(value));
        }
    }

    /** Service object held by the myCheckServiceProvider variable. */
    public static class LocalCheckServiceProvider {
        public boolean check() {
            return true;
        }
    }
}
