import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.PropertyAccessException;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportEnum;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportInstanceMethodBean;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
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
 * Direct Esper 9.0.0 oracle for the expr-filter-expressions parity scenario.
 * Mirrors ExprFilterExpressions ordinals 7, 17 and 27:
 *
 * <ul>
 * <li>ExprFilterOverInClause (ord 7): s0 deploys
 * pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)]
 * (no space after 'pattern'), then s1 deploys
 * pattern [every event1=SupportTradeEvent(userId in ('100','101'))] (space
 * after 'pattern', no amount filter). 'every' is cumulative: the second send
 * fires both statements. assertEqualsNew("sN","event1.id",id) maps to the
 * listener record's event1.id field.
 * <li>ExprFilterStaticFunc (ord 17): the MultiStmtAssertUtil
 * runIsInvokedWTestdata matrix — eight statements s0..s7 deploy as
 * "@name('s<i>') " + body, then 'a','b','c' sends assert
 * assertListenerInvokedFlag per statement ([F,T,F] for s0-s6, [F,F,F] for the
 * unsatisfiable s7). The pinned expected flags ride inside each send payload.
 * <li>ExprFilterInstanceMethodWWildcard (ord 27): three
 * deploy/send/undeployAll cycles over SupportInstanceMethodBean asserting
 * s0.myInstanceMethodAlwaysTrue() [T,T,T],
 * s0.myInstanceMethodEventBean(s0,'x',1) [F,T,F] and the '*' wildcard
 * argument [F,T,F] — the wildcard passes the same stream EventBean as the
 * s0 alias.
 * </ul>
 *
 * <p>Java milestones are ordering markers with no virtual time, so they are
 * implicit in the step order. Listener callbacks buffer deliveries; each
 * send drains them in deploy order and emits one record per deployed
 * statement — a listener record carrying the row on fire, or a
 * listener-not-invoked count record — mirroring
 * assertEqualsNew/assertListenerInvokedFlag. A delivery that contradicts the
 * pinned expected flag is a replay error. All records share one case-local
 * sequence so the record order is the step order.
 *
 * <p>SupportBean and SupportTradeEvent are declared as map event types
 * carrying the Java bean property sets; sends populate the Java bean
 * defaults first (SupportBean: theString=null, intPrimitive=-1 like
 * sendBeanString, charPrimitive='\0', boxed columns null; SupportTradeEvent:
 * ccypair/direction null) then overlay the payload, so select * rows match
 * the bean executions. SupportInstanceMethodBean registers as the real bean
 * class because the filter invokes instance methods on it.
 */
public final class ExprFilterExpressionsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-filter-expressions";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/"
                    + "ExprFilterExpressions.java";

    private static final String[] CASES = {
            "over-in-clause",
            "static-func",
            "instance-method-wildcard",
    };
    private static final int[] ORDINALS = {7, 17, 27};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-e9c9627ad604f3404620",
            "java-runtime-15341d9e0dc15c4b2fc3",
            "java-runtime-0a80365acd3e6ab25238",
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprFilterOverInClause",
            "ExprFilterStaticFunc",
            "ExprFilterInstanceMethodWWildcard",
    };
    private static final String[] STATIC_IDS = {
            "java-9d2f2743881d949bbddc",
            "java-314bbbdca28d0445fdd0",
            "java-576e4853b035fd6064f5",
    };
    private static final String[] JAVA_FLAGS = {};
    private static final int EXPECTED_STEPS = 48;

    private static final String LIB =
            "com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib";

    private static final String EPL_OVER_S0 =
            "@name('s0') select * from pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)]";
    private static final String EPL_OVER_S1 =
            "@name('s1') select * from pattern [every event1=SupportTradeEvent(userId in ('100','101'))]";

    /** ExprFilterStaticFunc bodies in assertion order (without @name). */
    private static final String[] STATIC_BODIES = {
            "select * from SupportBean(" + LIB + ".isStringEquals('b', theString))",
            "select * from SupportBean(" + LIB + ".isStringEquals('bx', theString || 'x'))",
            "select * from SupportBean('b'=theString," + LIB + ".isStringEquals('bx', theString || 'x'))",
            "select * from SupportBean('b'=theString, theString='b', theString != 'a')",
            "select * from SupportBean(theString != 'a', theString != 'c')",
            "select * from SupportBean(theString = 'b', theString != 'c')",
            "select * from SupportBean(theString != 'a' and theString != 'c')",
            "select * from SupportBean(theString = 'a' and theString = 'c' and "
                    + LIB + ".isStringEquals('bx', theString || 'x'))",
    };

    /** tryFilterInstanceMethod bodies in execution order (without @name). */
    private static final String[] INSTANCE_BODIES = {
            "select * from SupportInstanceMethodBean(s0.myInstanceMethodAlwaysTrue()) as s0",
            "select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(s0, 'x', 1)) as s0",
            "select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(*, 'x', 1)) as s0",
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprFilterExpressionsScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(index, allSteps, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /** Replays one case's steps on a fresh runtime (one runtime per Java execution). */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = buildConfiguration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        int[] sequence = {0};
        Map<String, List<JsonObject>> deliveries = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
        List<String> deployedOrder = new ArrayList<>();
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        try {
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        compilerArgs.getPath().add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            statement.addListener(bufferingListener(deliveries));
                        }
                        deployments.put(label, deployment);
                        deployedOrder.add(label);
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", ++sequence[0]);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send": {
                        JsonObject payload = object(step.get("payload"), "payload");
                        JsonObject expected = object(payload.get("expected"), "expected");
                        sendEvent(runtime, string(step, "eventType"), payload);
                        drainDeliveries(caseName, deployedOrder, deliveries, expected,
                                sequence, records, runtime);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
                        deployedOrder.clear();
                        deliveries.clear();
                        compilerArgs = new CompilerArguments(runtime.getRuntimePath());
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

    /**
     * Emits one record per deployed statement in deploy order: a listener
     * record per buffered delivery when the send fired the statement, or a
     * listener-not-invoked count record — mirroring
     * assertEqualsNew/assertListenerInvokedFlag. The pinned expected flags
     * must cover exactly the deployed statements; a delivery that
     * contradicts the flag is a replay error.
     */
    private static void drainDeliveries(String caseName, List<String> deployedOrder,
                                        Map<String, List<JsonObject>> deliveries,
                                        JsonObject expected, int[] sequence,
                                        JsonArray records, EPRuntime runtime) {
        if (expected.names().size() != deployedOrder.size()) {
            throw new IllegalStateException("send expected " + expected.names().size()
                    + " statements, " + deployedOrder.size() + " deployed");
        }
        for (String name : deployedOrder) {
            JsonValue flag = expected.get(name);
            if (flag == null || !flag.isBoolean()) {
                throw new IllegalStateException("send has no expected flag for " + name);
            }
            boolean want = flag.asBoolean();
            List<JsonObject> batches = deliveries.remove(name);
            boolean fired = batches != null && !batches.isEmpty();
            if (fired != want) {
                throw new IllegalStateException("statement " + name + " fired=" + fired
                        + ", want " + want);
            }
            if (fired) {
                for (JsonObject delivery : batches) {
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "listener");
                    record.add("statement", name);
                    record.add("sequence", ++sequence[0]);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    JsonArray newRows = delivery.get("new").asArray();
                    if (newRows.size() > 0) {
                        record.add("new", newRows);
                    }
                    JsonValue oldRows = delivery.get("old");
                    if (oldRows != null && oldRows.asArray().size() > 0) {
                        record.add("old", oldRows.asArray());
                    }
                    records.add(record);
                }
            } else {
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "count");
                record.add("statement", name);
                record.add("sequence", ++sequence[0]);
                record.add("time", Instant.ofEpochMilli(
                        runtime.getEventService().getCurrentTime()).toString());
                record.add("name", "listener-not-invoked");
                record.add("count", 0);
                records.add(record);
            }
        }
        if (!deliveries.isEmpty()) {
            throw new IllegalStateException("send delivered to undeployed statements: "
                    + deliveries.keySet());
        }
    }

    /** Buffers each delivery's rendered new/old rows per statement name. */
    private static UpdateListener bufferingListener(Map<String, List<JsonObject>> deliveries) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            JsonObject delivery = new JsonObject();
            delivery.add("new", rows(newEvents));
            delivery.add("old", rows(oldEvents));
            deliveries.computeIfAbsent(statement.getName(), key -> new ArrayList<>())
                    .add(delivery);
        };
    }

    /** Map event types mirroring the Java bean property sets, plus the real
     * SupportInstanceMethodBean class for the instance-method filters. */
    private static Configuration buildConfiguration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        Map<String, Object> supportBean = new LinkedHashMap<>();
        supportBean.put("theString", String.class);
        supportBean.put("boolPrimitive", boolean.class);
        supportBean.put("intPrimitive", int.class);
        supportBean.put("longPrimitive", long.class);
        supportBean.put("charPrimitive", char.class);
        supportBean.put("shortPrimitive", short.class);
        supportBean.put("bytePrimitive", byte.class);
        supportBean.put("floatPrimitive", float.class);
        supportBean.put("doublePrimitive", double.class);
        supportBean.put("boolBoxed", Boolean.class);
        supportBean.put("intBoxed", Integer.class);
        supportBean.put("longBoxed", Long.class);
        supportBean.put("charBoxed", Character.class);
        supportBean.put("shortBoxed", Short.class);
        supportBean.put("byteBoxed", Byte.class);
        supportBean.put("floatBoxed", Float.class);
        supportBean.put("doubleBoxed", Double.class);
        supportBean.put("bigDecimal", BigDecimal.class);
        supportBean.put("bigInteger", BigInteger.class);
        supportBean.put("enumValue", SupportEnum.class);
        configuration.getCommon().addEventType("SupportBean", supportBean);

        Map<String, Object> trade = new LinkedHashMap<>();
        trade.put("id", int.class);
        trade.put("userId", String.class);
        trade.put("ccypair", String.class);
        trade.put("direction", String.class);
        trade.put("amount", int.class);
        configuration.getCommon().addEventType("SupportTradeEvent", trade);

        configuration.getCommon().addEventType(SupportInstanceMethodBean.class);
        return configuration;
    }

    /** Sends an event pre-populated with the Java bean defaults. SupportBean
     * mirrors sendBeanString (new SupportBean(theString, -1)); the
     * SupportTradeEvent 3-argument constructor leaves ccypair/direction
     * null. */
    private static void sendEvent(EPRuntime runtime, String eventType, JsonObject payload) {
        switch (eventType) {
            case "SupportBean": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("theString", null);
                event.put("boolPrimitive", false);
                event.put("intPrimitive", -1);
                event.put("longPrimitive", 0L);
                event.put("charPrimitive", '\0');
                event.put("shortPrimitive", (short) 0);
                event.put("bytePrimitive", (byte) 0);
                event.put("floatPrimitive", 0f);
                event.put("doublePrimitive", 0d);
                event.put("boolBoxed", null);
                event.put("intBoxed", null);
                event.put("longBoxed", null);
                event.put("charBoxed", null);
                event.put("shortBoxed", null);
                event.put("byteBoxed", null);
                event.put("floatBoxed", null);
                event.put("doubleBoxed", null);
                event.put("bigDecimal", null);
                event.put("bigInteger", null);
                event.put("enumValue", null);
                overlay(event, payload, "theString");
                runtime.getEventService().sendEventMap(event, eventType);
                return;
            }
            case "SupportTradeEvent": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", 0);
                event.put("userId", null);
                event.put("ccypair", null);
                event.put("direction", null);
                event.put("amount", 0);
                overlay(event, payload, "id", "userId", "ccypair", "direction", "amount");
                runtime.getEventService().sendEventMap(event, eventType);
                return;
            }
            case "SupportInstanceMethodBean": {
                JsonValue x = payload.get("x");
                if (x == null || !x.isNumber()) {
                    throw new IllegalArgumentException("SupportInstanceMethodBean payload requires x");
                }
                runtime.getEventService().sendEventBean(
                        new SupportInstanceMethodBean(x.asInt()), eventType);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown eventType: " + eventType);
        }
    }

    private static void overlay(Map<String, Object> event, JsonObject payload, String... keys) {
        for (String key : keys) {
            JsonValue value = payload.get(key);
            if (value == null) {
                continue;
            }
            if (value.isString()) {
                event.put(key, value.asString());
            } else if (value.isNumber()) {
                event.put(key, (int) value.asLong());
            } else if (value.isBoolean()) {
                event.put(key, value.asBoolean());
            } else if (value.isNull()) {
                event.put(key, null);
            } else {
                throw new IllegalArgumentException("unsupported payload value for " + key);
            }
        }
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
            Object value;
            try {
                value = event.get(name);
            } catch (PropertyAccessException unreadable) {
                continue;
            }
            fields.add(name, normalize(value));
        }
        item.add("fields", fields);
        return item;
    }

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object; EventBean values render with __type plus
     * sorted properties, maps and collections recurse. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean eventBean) {
            JsonObject object = new JsonObject();
            object.add("__type", eventBean.getEventType().getName());
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
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
        if (value instanceof Collection<?> collection) {
            JsonArray array = new JsonArray();
            for (Object item : collection) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = java.lang.reflect.Array.getLength(value);
            for (int i = 0; i < length; i++) {
                array.add(normalize(java.lang.reflect.Array.get(value, i)));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateOverInClause(steps, offset);
        offset = validateStaticFunc(steps, offset);
        offset = validateInstanceMethod(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateOverInClause(JsonArray steps, int offset) {
        String caseName = "over-in-clause";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_OVER_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateSend(steps.get(offset++), caseName, "SupportTradeEvent",
                "{\"id\":1,\"userId\":\"100\",\"amount\":1001,\"expected\":{\"s0\":true}}");
        validateDeploy(steps.get(offset++), caseName, "s1", EPL_OVER_S1);
        validateDeployed(steps.get(offset++), caseName, "s1");
        validateSend(steps.get(offset++), caseName, "SupportTradeEvent",
                "{\"id\":2,\"userId\":\"100\",\"amount\":1001,\"expected\":{\"s0\":true,\"s1\":true}}");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateStaticFunc(JsonArray steps, int offset) {
        String caseName = "static-func";
        validateCaseMarker(steps.get(offset++), caseName);
        for (int i = 0; i < STATIC_BODIES.length; i++) {
            String statement = "s" + i;
            validateDeploy(steps.get(offset++), caseName, statement,
                    "@name('" + statement + "') " + STATIC_BODIES[i]);
            validateDeployed(steps.get(offset++), caseName, statement);
        }
        validateSend(steps.get(offset++), caseName, "SupportBean",
                "{\"theString\":\"a\",\"expected\":{\"s0\":false,\"s1\":false,\"s2\":false,"
                        + "\"s3\":false,\"s4\":false,\"s5\":false,\"s6\":false,\"s7\":false}}");
        validateSend(steps.get(offset++), caseName, "SupportBean",
                "{\"theString\":\"b\",\"expected\":{\"s0\":true,\"s1\":true,\"s2\":true,"
                        + "\"s3\":true,\"s4\":true,\"s5\":true,\"s6\":true,\"s7\":false}}");
        validateSend(steps.get(offset++), caseName, "SupportBean",
                "{\"theString\":\"c\",\"expected\":{\"s0\":false,\"s1\":false,\"s2\":false,"
                        + "\"s3\":false,\"s4\":false,\"s5\":false,\"s6\":false,\"s7\":false}}");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateInstanceMethod(JsonArray steps, int offset) {
        String caseName = "instance-method-wildcard";
        validateCaseMarker(steps.get(offset++), caseName);
        boolean[][] expected = {
                {true, true, true},
                {false, true, false},
                {false, true, false},
        };
        for (int cycle = 0; cycle < INSTANCE_BODIES.length; cycle++) {
            validateDeploy(steps.get(offset++), caseName, "s0",
                    "@name('s0') " + INSTANCE_BODIES[cycle]);
            validateDeployed(steps.get(offset++), caseName, "s0");
            for (int i = 0; i < 3; i++) {
                validateSend(steps.get(offset++), caseName, "SupportInstanceMethodBean",
                        "{\"x\":" + i + ",\"expected\":{\"s0\":" + expected[cycle][i] + "}}");
            }
            validateUndeployAll(steps.get(offset++), caseName);
        }
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName,
                                       String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    /** The expected payload is compared as parsed JSON so member order is
     * pinned by the scenario file itself. */
    private static void validateSend(JsonValue value, String caseName,
                                     String eventType, String expectedPayload) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName);
        }
        JsonObject expected = Json.parse(expectedPayload).asObject();
        if (!expected.equals(object(step.get("payload"), "payload"))) {
            throw new IllegalArgumentException("send payload is not pinned for " + caseName);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray array = array(value, name);
        if (array.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!expected[index].equals(array.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned at index " + index);
            }
        }
    }

    private static void requireFields(JsonObject object, String... names) {
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("missing field: " + name);
            }
        }
    }

    private static JsonObject object(JsonValue value, String name) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(name + " must be an object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be an array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be a number");
        }
        return value.asInt();
    }

    /** Rethrow handler so listener/deploy failures surface instead of logging. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return (handlerContext) -> {
                throw new RuntimeException(handlerContext.getThrowable());
            };
        }
    }
}
