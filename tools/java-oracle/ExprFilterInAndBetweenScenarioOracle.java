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
 * Direct Esper 9.0.0 oracle for the expr-filter-in-and-between parity
 * scenario. Mirrors ExprFilterInAndBetween ordinals 0, 5, 6, 7 and 8:
 * ExprFilterInDynamic (dynamic in-sets over pattern tags, two sequential
 * deployments separated by undeployAll), ExprFilterReuse and
 * ExprFilterReuseNot (the tryReuse protocol: each statement deploys as its
 * own module, one send fires every listener, then undeployModuleContaining
 * removes s0..sn-1 one at a time with a send after each, and a final send
 * fires none), ExprFilterInMultipleNonMatchingFirst and
 * ExprFilterInMultipleWithBool (deploy-order-sensitive in+like filters).
 *
 * <p>Ordinal 4 (ExprFilterInInvalid) is a compile-failure-only execution
 * gated on filter index planning >= BASIC and carries no scenario case.
 * Java milestones are ordering markers with no virtual time, so they are
 * implicit in the step order; assertListenerInvoked/NotInvoked and
 * assertEventNew map to listener record presence/absence in the trace.
 *
 * <p>Event types are declared as maps carrying the Java bean property set:
 * SupportBean (20 properties), SupportBeanNumeric (9) and SupportBean_S0
 * (6). Sends populate Java bean defaults first (theString=null, boxed=null,
 * charPrimitive='\0', primitives 0/false) then overlay the payload, so
 * filter evaluation and select * rows match the bean executions. Pattern
 * tags therefore render as plain property maps, the same shape the Go
 * runner emits for its tag-map projections.
 */
public final class ExprFilterInAndBetweenScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-filter-in-and-between";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/"
                    + "ExprFilterInAndBetween.java";

    private static final String[] CASES = {
            "in-dynamic-pattern",
            "in-reuse-undeploy",
            "not-in-reuse-undeploy",
            "in-multiple-nonmatching-first",
            "in-multiple-with-bool",
    };
    private static final int[] ORDINALS = {0, 5, 6, 7, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-0d06d25e978384a0b008",
            "java-runtime-d16fe1fd7693643a5001",
            "java-runtime-9245458815076f0baee3",
            "java-runtime-a3336b696b1ae9b2c831",
            "java-runtime-ed506bfbf3734b4fff03",
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprFilterInDynamic",
            "ExprFilterReuse",
            "ExprFilterReuseNot",
            "ExprFilterInMultipleNonMatchingFirst",
            "ExprFilterInMultipleWithBool",
    };
    private static final String[] STATIC_IDS = {
            "java-17cece2bf9c2df0b27f1",
            "java-17cece2bf9c2df0b27f1",
            "java-17cece2bf9c2df0b27f1",
            "java-17cece2bf9c2df0b27f1",
            "java-17cece2bf9c2df0b27f1",
    };
    private static final String[] JAVA_FLAGS = {"OBSERVEROPS"};
    private static final int EXPECTED_STEPS = 181;

    private static final String EPL_DYNAMIC_A =
            "@name('s0') select * from pattern [a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo))]";
    private static final String EPL_DYNAMIC_B =
            "@name('s0') select * from pattern [a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02))]";
    private static final String EPL_NONMATCHING_A =
            "@name('A') select * from SupportBean(intPrimitive in (0,0,1) and theString like 'X%')";
    private static final String EPL_NONMATCHING_B =
            "@name('B') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')";
    private static final String EPL_WITHBOOL_ONE =
            "@name('s1') select * from SupportBean(intPrimitive in (0) and theString like 'X%')";
    private static final String EPL_WITHBOOL_TWO =
            "@name('s2') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')";

    /** ExprFilterReuse groups, in execution order (bodies without @name). */
    private static final String[][] REUSE_GROUPS = {
            {"select * from SupportBean(intBoxed in [2:4])",
                    "select * from SupportBean(intBoxed in [2:4])"},
            {"select * from SupportBean(intBoxed in (1, 2, 3))",
                    "select * from SupportBean(intBoxed in (1, 2, 3))"},
            {"select * from SupportBean(intBoxed in (2:3])",
                    "select * from SupportBean(intBoxed in (1:3])"},
            {"select * from SupportBean(intBoxed in (2, 3, 4))",
                    "select * from SupportBean(intBoxed in (1, 3))"},
            {"select * from SupportBean(intBoxed in (2, 3, 4))",
                    "select * from SupportBean(intBoxed in (1, 3))",
                    "select * from SupportBean(intBoxed in (8, 3))"},
            {"select * from SupportBean(intBoxed in (3, 1, 3))",
                    "select * from SupportBean(intBoxed in (3, 3))",
                    "select * from SupportBean(intBoxed in (1, 3))"},
            {"select * from SupportBean(boolPrimitive=false, intBoxed in (1, 2, 3))",
                    "select * from SupportBean(boolPrimitive=false, intBoxed in (3, 4))",
                    "select * from SupportBean(boolPrimitive=false, intBoxed in (3))"},
            {"select * from SupportBean(intBoxed in (1, 2, 3), longPrimitive >= 0)",
                    "select * from SupportBean(intBoxed in (3, 4), intPrimitive >= 0)",
                    "select * from SupportBean(intBoxed in (3), bytePrimitive < 1)"},
    };

    /** ExprFilterReuseNot groups, in execution order. */
    private static final String[][] REUSE_NOT_GROUPS = {
            {"select * from SupportBean(intBoxed not in [1:2])",
                    "select * from SupportBean(intBoxed not in [1:2])"},
            {"select * from SupportBean(intBoxed in (3, 1, 3))",
                    "select * from SupportBean(intBoxed not in (2, 1))",
                    "select * from SupportBean(intBoxed not between 0 and -3)"},
            {"select * from SupportBean(intBoxed not in (1, 4, 5))",
                    "select * from SupportBean(intBoxed not in (1, 4, 5))",
                    "select * from SupportBean(intBoxed not in (4, 5, 1))"},
            {"select * from SupportBean(intBoxed not in (3:4))",
                    "select * from SupportBean(intBoxed not in [1:3))",
                    "select * from SupportBean(intBoxed not in (1,1,1,33))"},
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprFilterInAndBetweenScenarioOracle <scenario.json>");
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

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
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
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                        deployments.put(label, deployment);
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
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
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "undeploy": {
                        // undeployModuleContaining: every statement deploys as
                        // its own module, so the statement key selects the
                        // whole deployment.
                        String label = string(step, "statement");
                        EPDeployment deployment = deployments.remove(label);
                        if (deployment == null) {
                            throw new IllegalStateException(
                                    "undeploy of unknown statement " + label);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
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

    /** Map event types mirroring the Java bean property sets. */
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

        Map<String, Object> numeric = new LinkedHashMap<>();
        numeric.put("intOne", Integer.class);
        numeric.put("intTwo", Integer.class);
        numeric.put("bigint", BigInteger.class);
        numeric.put("bigdec", BigDecimal.class);
        numeric.put("bigdecTwo", BigDecimal.class);
        numeric.put("doubleOne", double.class);
        numeric.put("doubleTwo", double.class);
        numeric.put("floatOne", float.class);
        numeric.put("floatTwo", float.class);
        configuration.getCommon().addEventType("SupportBeanNumeric", numeric);

        Map<String, Object> s0 = new LinkedHashMap<>();
        s0.put("id", int.class);
        s0.put("p00", String.class);
        s0.put("p01", String.class);
        s0.put("p02", String.class);
        s0.put("p03", String.class);
        s0.put("value", int.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0);
        return configuration;
    }

    /** Sends a map event pre-populated with the Java bean defaults. */
    private static void sendEvent(EPRuntime runtime, String eventType, JsonObject payload) {
        Map<String, Object> event = new LinkedHashMap<>();
        switch (eventType) {
            case "SupportBean":
                event.put("theString", null);
                event.put("boolPrimitive", false);
                event.put("intPrimitive", 0);
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
                overlay(event, payload, "theString", "intPrimitive", "intBoxed");
                break;
            case "SupportBeanNumeric":
                event.put("intOne", null);
                event.put("intTwo", null);
                event.put("bigint", null);
                event.put("bigdec", null);
                event.put("bigdecTwo", null);
                event.put("doubleOne", 0d);
                event.put("doubleTwo", 0d);
                event.put("floatOne", 0f);
                event.put("floatTwo", 0f);
                overlay(event, payload, "intOne", "intTwo");
                break;
            case "SupportBean_S0":
                event.put("id", 0);
                event.put("p00", null);
                event.put("p01", null);
                event.put("p02", null);
                event.put("p03", null);
                event.put("value", 0);
                overlay(event, payload, "id", "p00", "p01", "p02", "p03");
                break;
            default:
                throw new IllegalArgumentException("unknown eventType: " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
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

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
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
        offset = validateInDynamic(steps, offset);
        offset = validateReuse(steps, offset, "in-reuse-undeploy", REUSE_GROUPS,
                REUSE_DEPLOY_ORDERS);
        offset = validateReuse(steps, offset, "not-in-reuse-undeploy", REUSE_NOT_GROUPS);
        offset = validateNonMatchingFirst(steps, offset);
        offset = validateWithBool(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateInDynamic(JsonArray steps, int offset) {
        String caseName = "in-dynamic-pattern";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_DYNAMIC_A);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateSend(steps.get(offset++), caseName, "SupportBeanNumeric",
                "{\"intOne\":10,\"intTwo\":20}");
        validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intPrimitive\":10}");
        validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intPrimitive\":11}");
        validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intPrimitive\":20}");
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_DYNAMIC_B);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateSend(steps.get(offset++), caseName, "SupportBean_S0",
                "{\"id\":1,\"p00\":\"a\",\"p01\":\"b\",\"p02\":\"c\",\"p03\":\"d\"}");
        for (String value : new String[]{"a", "x", "b", "c", "d"}) {
            validateSend(steps.get(offset++), caseName, "SupportBean",
                    "{\"theString\":\"" + value + "\"}");
        }
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /** Validates the tryReuse step cadence: n deploy/deployed pairs, one send,
     * then n (undeploy, send) pairs and a final send. DEPLOY_ORDERS optionally
     * permutes the deploy step order per group: Java evaluates range filters in
     * TreeMap (min,max) index order — (1:3] fires before (2:3] regardless of
     * deploy order — while Go dispatches in deploy order, so reuse group 3
     * deploys s1 before s0 to keep both traces in the same observable order. */
    private static final int[][] REUSE_DEPLOY_ORDERS = {
            null, null, {1, 0}, null, null, null, null, null,
    };

    private static int validateReuse(JsonArray steps, int offset, String caseName,
                                     String[][] groups) {
        return validateReuse(steps, offset, caseName, groups, null);
    }

    private static int validateReuse(JsonArray steps, int offset, String caseName,
                                     String[][] groups, int[][] deployOrders) {
        validateCaseMarker(steps.get(offset++), caseName);
        for (int g = 0; g < groups.length; g++) {
            String[] group = groups[g];
            int[] order = deployOrders != null && g < deployOrders.length
                    ? deployOrders[g] : null;
            for (int i = 0; i < group.length; i++) {
                int index = order != null ? order[i] : i;
                String statement = "s" + index;
                validateDeploy(steps.get(offset++), caseName, statement,
                        "@name('" + statement + "')" + group[index]);
                validateDeployed(steps.get(offset++), caseName, statement);
            }
            validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intBoxed\":3}");
            for (int i = 0; i < group.length; i++) {
                validateUndeploy(steps.get(offset++), caseName, "s" + i);
                validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intBoxed\":3}");
            }
            validateSend(steps.get(offset++), caseName, "SupportBean", "{\"intBoxed\":3}");
        }
        return offset;
    }

    private static int validateNonMatchingFirst(JsonArray steps, int offset) {
        String caseName = "in-multiple-nonmatching-first";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "A", EPL_NONMATCHING_A);
        validateDeployed(steps.get(offset++), caseName, "A");
        validateDeploy(steps.get(offset++), caseName, "B", EPL_NONMATCHING_B);
        validateDeployed(steps.get(offset++), caseName, "B");
        validateSend(steps.get(offset++), caseName, "SupportBean",
                "{\"theString\":\"A\",\"intPrimitive\":0}");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateWithBool(JsonArray steps, int offset) {
        String caseName = "in-multiple-with-bool";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s1", EPL_WITHBOOL_ONE);
        validateDeployed(steps.get(offset++), caseName, "s1");
        validateDeploy(steps.get(offset++), caseName, "s2", EPL_WITHBOOL_TWO);
        validateDeployed(steps.get(offset++), caseName, "s2");
        validateSend(steps.get(offset++), caseName, "SupportBean",
                "{\"theString\":\"A\",\"intPrimitive\":1}");
        validateUndeployAll(steps.get(offset++), caseName);
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

    private static void validateUndeploy(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "undeploy step");
        requireFields(step, "op", "case", "statement");
        if (!"undeploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("undeploy step is not pinned for " + caseName + "/"
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
