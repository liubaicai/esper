import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.PropertyAccessException;
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
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

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

/**
 * Java oracle for the expr-filter-optimizable-value-limited parity scenario
 * (pinned Esper 9.0.0 commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
 * Replays the nine listener-observable executions of
 * regression-lib/.../suite/expr/filter/ExprFilterOptimizableValueLimitedExpr
 * against one fresh runtime per case, with the pinned EPL deployed through
 * the compiler plus deployment service:
 *
 * - from-pattern-single (ordinal 1): every a=SupportBean_S0 ->
 *   SupportBean(a.p00 || a.p01 = theString); the filter value is derived from
 *   pattern variable a and changes when 'every' re-seeds S0.
 * - from-pattern-multi (ordinal 2): every [2] a=SupportBean_S0 ->
 *   b=SupportBean_S1 -> SupportBean(theString = a[0].p00 || b.p10); the value
 *   is an array element plus the b property.
 * - from-pattern-constant (ordinal 3):
 *   every SupportBean_S0 -> SupportBean_S1 -> SupportBean('a' || 'x' =
 *   theString); a constant-folded value inside a three-event pattern.
 * - from-pattern-half-constant (ordinal 4):
 *   every s0=SupportBean_S0 -> s1=SupportBean_S1 -> SupportBean('a' ||
 *   s1.p10 = theString); half-literal value.
 * - from-pattern-with-dot-method (ordinal 5): pattern [a=SupportBean ->
 *   b=SupportBean(theString=a.getTheString())]; the value comes from a method
 *   call on the prior pattern event and the fired row projects a/b tags.
 * - context-with-start (ordinal 6): context MyContext start SupportBean_S0 as
 *   s0; the filter reads context.s0.p00 || context.s0.p01 of the single
 *   started partition.
 * - in-set-of-value-wcoercion (ordinal 12): pattern [a=SupportBean_S0 ->
 *   b=SupportBean_S1 -> c=SupportBean_S2 -> every SupportBean(longPrimitive in
 *   (a.id, b.id, c.id))]; int ids coerced to long for the IN list.
 * - in-range-wcoercion (ordinal 13): pattern [a=SupportBean_S0 ->
 *   b=SupportBean_S1 -> every SupportBean(longPrimitive in/not in [a.id - 2 :
 *   b.id + 2])]; two deployments, RANGE_CLOSED then NOT_RANGE_CLOSED, each
 *   replayed with the same five boundary events (7/8/100/202/203).
 * - or-rewrite (ordinal 14): context MyContext start SupportBean_S0 as s0;
 *   the filter is an OR of the two context-value concatenations.
 *
 * The remaining executions of the pinned file are not replayed:
 * EqualsIsConstant, EqualsSubstitutionParams, EqualsConstantVariable,
 * EqualsCoercion and RelOpCoercion are covered by the
 * filter-val-optimizable scenario; Disqualify asserts only compile-time
 * plan-forge operators (BOOLEAN_EXPRESSION/REBOOL) with no event stream or
 * listener and is intentionally-different. The FilterService operator
 * assertions (EQUAL/IS/LESS/RANGE_* / IN_LIST_OF_VALUES) are engine-internal
 * planning artifacts and are out of scope for the fire/no-fire trace.
 *
 * Listener records follow the standard protocol: one record per delivery,
 * per-statement sequence numbering from 1 per deployment, time rendered from
 * engine time, new/old arrays rendered with the scalar normalization rules
 * (Integer/Long/Short/Byte -> long, other Number -> double including
 * BigDecimal, null -> {state:null}). Pattern tag values are the raw map
 * payloads, rendered with their submitted keys (S0/S1/S2 map tags); nested
 * EventBean values are rendered with __type plus sorted properties. Bean tag
 * values (SupportBean in from-pattern-with-dot-method) have no EventBean or
 * Map wrapper at the tag position and therefore render as the bean's
 * toString().
 *
 * Event types follow the pinned suite's configuration: SupportBean is the
 * real bean class (required for the pinned a.getTheString() method call in
 * from-pattern-with-dot-method), while SupportBean_S0/S1/S2 are map event
 * types carrying the full scenario schema (S0: id/p00-p03/value, S1:
 * id/p10-p13, S2: id/p20-p23).
 * Scenario step ops: case markers ("case"), sends ("send") and pinned module
 * deployments ("deploy" with statement label matching the pinned plan).
 */
public class ExprFilterOptimizableValueLimitedScenarioOracle {

    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private static final String[] CASES = {
        "from-pattern-single",
        "from-pattern-multi",
        "from-pattern-constant",
        "from-pattern-half-constant",
        "from-pattern-with-dot-method",
        "context-with-start",
        "in-set-of-value-wcoercion",
        "in-range-wcoercion",
        "or-rewrite"
    };

    private ExprFilterOptimizableValueLimitedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ExprFilterOptimizableValueLimitedScenarioOracle <scenario.json>");
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
                    sendEvent(runtime, step);
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
                    deployPinned(runtime, config, records, caseName, deployIndex, plan[deployIndex]);
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
                                     String caseName, int deployIndex, ScenarioPlan plan) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(plan.epl(), new CompilerArguments(config));
        DeploymentOptions options = new DeploymentOptions().setDeploymentId("parity-efo-" + caseName + "-" + deployIndex);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, options);
        for (String observe : plan.observed()) {
            EPStatement statement = findStatement(deployment, observe);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
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
            case "from-pattern-single" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every a=SupportBean_S0 -> SupportBean(a.p00 || a.p01 = theString)]",
                    new String[]{"s0"}),
            };
            case "from-pattern-multi" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every [2] a=SupportBean_S0 -> b=SupportBean_S1 -> SupportBean(theString = a[0].p00 || b.p10)]",
                    new String[]{"s0"}),
            };
            case "from-pattern-constant" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every SupportBean_S0 -> SupportBean_S1 -> SupportBean('a' || 'x' = theString)]",
                    new String[]{"s0"}),
            };
            case "from-pattern-half-constant" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[every s0=SupportBean_S0 -> s1=SupportBean_S1 -> SupportBean('a' || s1.p10 = theString)]",
                    new String[]{"s0"}),
            };
            case "from-pattern-with-dot-method" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern [a=SupportBean -> b=SupportBean(theString=a.getTheString())]",
                    new String[]{"s0"}),
            };
            case "context-with-start" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "create context MyContext start SupportBean_S0 as s0;\n" +
                        "@name('s0') context MyContext select * from SupportBean(theString = context.s0.p00 || context.s0.p01)",
                    new String[]{"s0"}),
            };
            case "in-set-of-value-wcoercion" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern [a=SupportBean_S0 -> b=SupportBean_S1 -> c=SupportBean_S2 -> every SupportBean(longPrimitive in (a.id, b.id, c.id))]",
                    new String[]{"s0"}),
            };
            case "in-range-wcoercion" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern [a=SupportBean_S0 -> b=SupportBean_S1 -> every SupportBean(longPrimitive in [a.id - 2 : b.id + 2])]",
                    new String[]{"s0"}),
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern [a=SupportBean_S0 -> b=SupportBean_S1 -> every SupportBean(longPrimitive not in [a.id - 2 : b.id + 2])]",
                    new String[]{"s0"}),
            };
            case "or-rewrite" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "create context MyContext start SupportBean_S0 as s0;\n" +
                        "@name('s0') context MyContext select * from SupportBean(theString = context.s0.p00 || context.s0.p01 or theString = context.s0.p01 || context.s0.p00);",
                    new String[]{"s0"}),
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static Configuration buildConfiguration() {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        // SupportBean is registered as the real bean class: the pinned
        // from-pattern-with-dot-method EPL resolves a.getTheString() as a
        // bean instance method, which a map type cannot provide.
        config.getCommon().addEventType("SupportBean", SupportBean.class);

        Map<String, Object> s0 = new LinkedHashMap<>();
        s0.put("id", int.class);
        s0.put("p00", String.class);
        s0.put("p01", String.class);
        s0.put("p02", String.class);
        s0.put("p03", String.class);
        s0.put("value", int.class);
        config.getCommon().addEventType("SupportBean_S0", s0);

        Map<String, Object> s1 = new LinkedHashMap<>();
        s1.put("id", int.class);
        s1.put("p10", String.class);
        s1.put("p11", String.class);
        s1.put("p12", String.class);
        s1.put("p13", String.class);
        config.getCommon().addEventType("SupportBean_S1", s1);

        Map<String, Object> s2 = new LinkedHashMap<>();
        s2.put("id", int.class);
        s2.put("p20", String.class);
        s2.put("p21", String.class);
        s2.put("p22", String.class);
        s2.put("p23", String.class);
        config.getCommon().addEventType("SupportBean_S2", s2);
        return config;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean event = new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0));
                event.setBoolPrimitive(payload.getBoolean("boolPrimitive", false));
                event.setLongPrimitive(payload.getLong("longPrimitive", 0L));
                event.setDoublePrimitive(payload.getDouble("doublePrimitive", 0.0));
                runtime.getEventService().sendEventBean(event, eventType);
            }
            case "SupportBean_S0" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getInt("id", 0));
                event.put("p00", payload.getString("p00", null));
                event.put("p01", payload.getString("p01", null));
                event.put("p02", payload.getString("p02", null));
                event.put("p03", payload.getString("p03", null));
                event.put("value", payload.getInt("value", 0));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportBean_S1" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getInt("id", 0));
                event.put("p10", payload.getString("p10", null));
                event.put("p11", payload.getString("p11", null));
                event.put("p12", payload.getString("p12", null));
                event.put("p13", payload.getString("p13", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportBean_S2" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getInt("id", 0));
                event.put("p20", payload.getString("p20", null));
                event.put("p21", payload.getString("p21", null));
                event.put("p22", payload.getString("p22", null));
                event.put("p23", payload.getString("p23", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            default -> throw new IllegalStateException("unknown eventType: " + eventType);
        }
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
}
