import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.PropertyAccessException;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.meta.EventTypeApplicationType;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for EventObjectArrayCore ordinals 0, 1, 3 and 4
 * (ordinal 2 EventObjectArrayQueryFields is already
 * case.object-array-query-fields and stays out of scope), replayed as one
 * differential chain:
 *
 * metadata (ord 0, EventObjectArrayMetadata): no EPL and no events — the
 * execution only introspects the preconfigured MyObjectArrayEvent type
 * (OBJECTARR application type, name, exactly three property descriptors:
 * myInt Integer, myString String, beanA SupportBeanComplexProps fragment;
 * order-insensitive). The assertions run before the case's steps and the
 * case emits a single deployed marker, exactly like the map-core metadata
 * precedent.
 *
 * nested-objects (ord 1, EventObjectArrayNestedObjects): one
 * deploy/send/undeploy-all cycle navigating beanA.simpleProperty,
 * beanA.nested.nestedValue, beanA.indexed[1] and
 * beanA.nested.nestedNested.nestedNestedValue over
 * MyObjectArrayEvent#length(5).
 *
 * nested-eventbean-array (ord 3, EventObjectArrayNestedEventBeanArray): a
 * two-statement objectarray schema module (NBAL_1(val string),
 * NBAL_2 (lvl1s NBAL_1[])) deploys first; select * from NBAL_1 takes one
 * send, undeployModuleContaining("s0") retires only the select (the schema
 * module stays deployed), then select lvl1s[0] as c0 from NBAL_2 yields the
 * raw Object[] carrier — not an event — rendered as a plain JSON array.
 *
 * invalid (ord 4, EventObjectArrayInvalid): two of Java's three
 * compile-phase rejections replay as build-error probes — select XXX
 * (unknown property) and select myString * 2 (String arithmetic). The
 * third probe select String.trim(myInt) is unrepresentable in the Go
 * fluent API (no static-method-call expression surface) and is documented
 * in the case notes only.
 *
 * Mirroring the sibling scenario oracles, each case runs in its own runtime
 * (URI event-object-array-core-<case>), deploys the pinned EPL per step,
 * records listener updates as sorted-field rows and undeploys at the Java
 * module boundaries. The {"_bean":"SupportBeanComplexProps"} payload tag
 * constructs SupportBeanComplexProps.makeDefaultBean(). Records follow the
 * standard protocol: per-statement listener sequences starting at one,
 * deployed/compile-error markers at sequence zero, and time frozen at
 * epoch zero because the internal timer is disabled and time is advanced
 * to zero only.
 */
public final class EventObjectArrayCoreScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "event-object-array-core";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayCore.java";

    private static final String DESCRIPTION =
            "EventObjectArrayCore object-array event core (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): metadata introspects the preconfigured MyObjectArrayEvent (OBJECTARR application type, name, exactly three descriptors — myInt:Integer, myString:String, beanA:SupportBeanComplexProps fragment — order-insensitive) with no events, acknowledged by a deployed marker (ord 0, EventObjectArrayMetadata); nested-objects navigates beanA.simpleProperty, beanA.nested.nestedValue, beanA.indexed[1] and beanA.nested.nestedNested.nestedNestedValue over MyObjectArrayEvent#length(5) (ord 1, EventObjectArrayNestedObjects); nested-eventbean-array deploys a two-statement objectarray schema module (NBAL_1(val string), NBAL_2 (lvl1s NBAL_1[])), selects * from NBAL_1 for one send, undeploys the s0 module while the schema module stays, then selects lvl1s[0] as c0 from NBAL_2 yielding the raw Object[] carrier (ord 3, EventObjectArrayNestedEventBeanArray); invalid covers two of Java's three compile-phase rejections — unknown property XXX and String arithmetic myString * 2 — while the third probe select String.trim(myInt) is unrepresentable (Go has no static-method-call expression surface) (ord 4, EventObjectArrayInvalid). Ord 2 EventObjectArrayQueryFields is covered by case.object-array-query-fields and stays out of scope (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayCore.java).";

    private static final String[] CASES = {
            "metadata", "nested-objects", "nested-eventbean-array", "invalid"};
    private static final int[] ORDINALS = {0, 1, 3, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-17dd924d2ddc5c9577fe",
            "java-runtime-52d765c3469a8eddd95e",
            "java-runtime-8f3cd3e434650bc6364f",
            "java-runtime-4f63efae89e8c0bd4e66"
    };
    private static final String[] EXECUTIONS = {
            "EventObjectArrayMetadata",
            "EventObjectArrayNestedObjects",
            "EventObjectArrayNestedEventBeanArray",
            "EventObjectArrayInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-0556d6715c212191e007",
            "java-0556d6715c212191e007",
            "java-0556d6715c212191e007",
            "java-0556d6715c212191e007"
    };
    private static final String[] OBSERVATIONS = {
            "deployed; no EPL and no events — the execution only introspects the preconfigured MyObjectArrayEvent type (OBJECTARR application type, name MyObjectArrayEvent, exactly three property descriptors: myInt Integer, myString String, beanA SupportBeanComplexProps fragment; order-insensitive) so the case emits a single deployed marker",
            "listener; one deploy/send/undeploy-all cycle over MyObjectArrayEvent{myInt:int,myString:string,beanA:SupportBeanComplexProps}: beanA.simpleProperty/beanA.nested.nestedValue/beanA.indexed[1]/beanA.nested.nestedNested.nestedNestedValue yields simple=simple,nested=nestedValue,indexed=2,nestednested=nestedNestedValue",
            "listener; schema module deploy then two select cycles: select * from NBAL_1 yields val=somevalue; undeployModuleContaining('s0') retires only the select (the schema module stays deployed); select lvl1s[0] as c0 from NBAL_2 yields c0=[somevalue] as the raw Object[] carrier, not an event",
            "compile-error; two build-error probes — select XXX (unknown property) and select myString * 2 (String arithmetic) — are rejected at build; the third Java probe select String.trim(myInt) is unrepresentable in the Go fluent API (no static-method-call expression) and is documented here only"
    };

    // Verbatim transcriptions of the pinned statement texts: the ord 1
    // statement keeps the missing spaces after commas and the space before
    // 'from' from the Java string concatenation; the ord 3 module keeps the
    // NBAL_1(val string) / NBAL_2 (lvl1s NBAL_1[]) paren spacing and the
    // inter-statement newline; the ord 4 probes carry no @name.
    private static final String EPL_NESTED_OBJECTS =
            "@name('s0') select beanA.simpleProperty as simple,beanA.nested.nestedValue as nested,beanA.indexed[1] as indexed,beanA.nested.nestedNested.nestedNestedValue as nestednested from MyObjectArrayEvent#length(5)";
    private static final String EPL_SCHEMAS =
            "@buseventtype @public create objectarray schema NBAL_1(val string);\n"
                    + "@buseventtype @public create objectarray schema NBAL_2 (lvl1s NBAL_1[]);\n";
    private static final String EPL_SELECT_NBAL1 = "@name('s0') select * from NBAL_1";
    private static final String EPL_SELECT_NBAL2 = "@name('s0') select lvl1s[0] as c0 from NBAL_2";
    private static final String EPL_PROBE_UNKNOWN = "select XXX from MyObjectArrayEvent#length(5)";
    private static final String EPL_PROBE_STRING = "select myString * 2 from MyObjectArrayEvent#length(5)";

    private static final String[] CASE_EPLS = {
            "",
            EPL_NESTED_OBJECTS,
            EPL_SCHEMAS + EPL_SELECT_NBAL1 + "\n" + EPL_SELECT_NBAL2 + "\n",
            EPL_PROBE_UNKNOWN + "\n" + EPL_PROBE_STRING + "\n"
    };

    // Pinned per-case step sequences. Each entry is
    // {op, statement, eventType, epl, payload, expectError}; fields not
    // meaningful for the op stay empty. The schema-module deploy is keyed
    // "schemas"; the build-error probes pin no expectError (Java's
    // tryInvalidCompile "skip").
    private static final String[][][] CASE_STEPS = {
            {
                    {"deployed", "metadata", "", "", "", ""},
            },
            {
                    {"deploy", "s0", "", EPL_NESTED_OBJECTS, "", ""},
                    {"send", "", "MyObjectArrayEvent", "",
                            "[3,\"some string\",{\"_bean\":\"SupportBeanComplexProps\"}]", ""},
                    {"undeploy-all", "", "", "", "", ""},
            },
            {
                    {"deploy", "schemas", "", EPL_SCHEMAS, "", ""},
                    {"deploy", "s0", "", EPL_SELECT_NBAL1, "", ""},
                    {"send", "", "NBAL_1", "", "[\"somevalue\"]", ""},
                    {"undeploy", "s0", "", "", "", ""},
                    {"deploy", "s0", "", EPL_SELECT_NBAL2, "", ""},
                    {"send", "", "NBAL_2", "", "[[[\"somevalue\"]]]", ""},
                    {"undeploy-all", "", "", "", "", ""},
            },
            {
                    {"build-error", "unknown-property", "", EPL_PROBE_UNKNOWN, "", ""},
                    {"build-error", "string-arithmetic", "", EPL_PROBE_STRING, "", ""},
            },
    };

    private static final int EXPECTED_RECORDS = 6;

    private EventObjectArrayCoreScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EventObjectArrayCoreScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, int caseIndex,
                                JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        registerEventTypes(configuration);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            // Pinned pre-execution assertions, mirroring the execution's
            // assertThat: the metadata case introspects the preconfigured
            // type before any step.
            if ("metadata".equals(caseName)) {
                assertMetadata(runtime);
            }
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            Map<String, EPDeployment> deployments = new HashMap<>();
            // RegressionPath equivalent: every compiled module stays on the
            // compile path for later statements in the same case (the schema
            // module keeps NBAL_1/NBAL_2 visible after the select undeploys).
            List<EPCompiled> deployedModules = new ArrayList<>();
            boolean active = false;
            int stepIndex = 0;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                String[] pinned = CASE_STEPS[caseIndex][stepIndex];
                if (!operation.equals(pinned[0])) {
                    throw new IllegalStateException("case " + caseName + " step " + stepIndex
                            + " op " + operation + " is not pinned");
                }
                stepIndex++;
                switch (operation) {
                    case "deploy": {
                        String epl = string(step, "epl");
                        if (!epl.equals(pinned[3])) {
                            throw new IllegalArgumentException("deploy epl is not pinned for case "
                                    + caseName + " step " + (stepIndex - 1));
                        }
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        for (EPCompiled deployed : deployedModules) {
                            compilerArgs.getPath().add(deployed);
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                compilerArgs);
                        deployedModules.add(compiled);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions().setDeploymentId(
                                        SCENARIO_ID + "-" + caseIndex + "-" + stepIndex));
                        deployments.put(string(step, "statement"), deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statement.addListener(writer);
                        }
                        break;
                    }
                    case "deployed": {
                        // Only the metadata case (which deploys nothing)
                        // emits the marker.
                        if (!"metadata".equals(caseName)) {
                            throw new IllegalStateException(
                                    "unexpected deployed op for case " + caseName);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", string(step, "statement"));
                        record.add("sequence", 0);
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "undeploy": {
                        // undeployModuleContaining("s0"): the statement key
                        // selects the whole deployment; the schema module
                        // stays deployed.
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
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Registers the preconfigured event type, mirroring
     * TestSuiteEventObjectArray.configure: MyObjectArrayEvent{myInt:int,
     * myString:string, beanA:SupportBeanComplexProps} via the
     * String[]/Object[] addEventType form. The bean-typed property is a
     * fragment; the NBAL schemas deploy through the module EPL instead.
     */
    private static void registerEventTypes(Configuration configuration) {
        configuration.getCommon().addEventType("MyObjectArrayEvent",
                new String[]{"myInt", "myString", "beanA"},
                new Object[]{Integer.class, String.class, SupportBeanComplexProps.class});
    }

    /**
     * Mirrors the execution's assertThat: OBJECTARR application type, the
     * MyObjectArrayEvent name and exactly three property descriptors —
     * myInt Integer, myString String, beanA SupportBeanComplexProps
     * fragment — compared order-insensitively like
     * SupportEventPropUtil.assertPropsEquals.
     */
    private static void assertMetadata(EPRuntime runtime) {
        EventType type = runtime.getEventTypeService().getEventTypePreconfigured("MyObjectArrayEvent");
        check(type != null, "preconfigured type MyObjectArrayEvent not found");
        check(type.getMetadata().getApplicationType() == EventTypeApplicationType.OBJECTARR,
                "expected OBJECTARR application type, got " + type.getMetadata().getApplicationType());
        check("MyObjectArrayEvent".equals(type.getMetadata().getName()),
                "expected name MyObjectArrayEvent, got " + type.getMetadata().getName());

        Map<String, EventPropertyDescriptor> byName = new LinkedHashMap<>();
        for (EventPropertyDescriptor desc : type.getPropertyDescriptors()) {
            check(!byName.containsKey(desc.getPropertyName()),
                    "duplicate descriptor " + desc.getPropertyName());
            byName.put(desc.getPropertyName(), desc);
        }
        check(byName.size() == 3, "expected three descriptors, got " + byName.keySet());
        for (String name : new String[]{"myInt", "myString", "beanA"}) {
            check(byName.containsKey(name), "missing descriptor " + name);
        }

        EventPropertyDescriptor myInt = byName.get("myInt");
        check(myInt.getPropertyType() == Integer.class, "myInt type " + myInt.getPropertyType());
        check(!myInt.isFragment() && !myInt.isIndexed() && !myInt.isMapped(),
                "myInt flags fragment=" + myInt.isFragment() + " indexed=" + myInt.isIndexed());

        EventPropertyDescriptor myString = byName.get("myString");
        check(myString.getPropertyType() == String.class, "myString type " + myString.getPropertyType());
        check(!myString.isFragment() && !myString.isIndexed() && !myString.isMapped(),
                "myString flags fragment=" + myString.isFragment() + " indexed=" + myString.isIndexed());

        EventPropertyDescriptor beanA = byName.get("beanA");
        check(beanA.getPropertyType() == SupportBeanComplexProps.class,
                "beanA type " + beanA.getPropertyType());
        check(beanA.isFragment(), "beanA must be a fragment");
        check(!beanA.isIndexed() && !beanA.isMapped(),
                "beanA flags indexed=" + beanA.isIndexed() + " mapped=" + beanA.isMapped());
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    /**
     * Runs the expected-invalid probe (env.tryInvalidCompile with the
     * "skip" message assertion): the EPL must fail compilation and the
     * record carries a value only when the scenario pins expectError —
     * Java omits the value field for "skip"-pinned probes.
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = step.getString("expectError", "");
        String epl = string(step, "epl");
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonValue payload = step.get("payload");
        switch (eventType) {
            case "MyObjectArrayEvent", "NBAL_1", "NBAL_2" ->
                    runtime.getEventService().sendEventObjectArray(
                            toObjectArray(payload.asArray()), eventType);
            default -> throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static Object[] toObjectArray(JsonArray items) {
        Object[] values = new Object[items.size()];
        for (int index = 0; index < values.length; index++) {
            values[index] = toJavaValue(items.get(index));
        }
        return values;
    }

    /**
     * Converts a JSON payload value to the Java value Esper expects:
     * {"_bean":...} constructs the pinned bean, plain objects become Maps,
     * arrays become Object[] and numbers become Integer/Long/Double.
     */
    private static Object toJavaValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isObject()) {
            JsonObject object = value.asObject();
            JsonValue beanTag = object.get("_bean");
            if (beanTag != null && beanTag.isString()) {
                return toBean(beanTag.asString(), object);
            }
            return toMap(object);
        }
        if (value.isArray()) {
            return toObjectArray(value.asArray());
        }
        if (value.isNumber()) {
            String text = value.toString();
            if (text.indexOf('.') >= 0 || text.indexOf('e') >= 0 || text.indexOf('E') >= 0) {
                return value.asDouble();
            }
            long number = value.asLong();
            if (number >= Integer.MIN_VALUE && number <= Integer.MAX_VALUE) {
                return (int) number;
            }
            return number;
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        return value.asString();
    }

    private static Map<String, Object> toMap(JsonObject object) {
        Map<String, Object> map = new LinkedHashMap<>();
        for (Member member : object) {
            map.put(member.getName(), toJavaValue(member.getValue()));
        }
        return map;
    }

    private static Object toBean(String bean, JsonObject object) {
        switch (bean) {
            case "SupportBeanComplexProps" -> {
                return SupportBeanComplexProps.makeDefaultBean();
            }
            default -> throw new IllegalArgumentException("unsupported bean tag: " + bean);
        }
    }

    private static Object readProperty(EventBean event, String prop) {
        try {
            return event.get(prop);
        } catch (PropertyAccessException unreadable) {
            return UNREADABLE;
        }
    }

    private static final Object UNREADABLE = new Object() {
        @Override
        public String toString() {
            return "<unreadable>";
        }
    };

    private static JsonObject eventRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(readProperty(event, prop)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonObject row(JsonObject fields) {
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value == UNREADABLE) {
            return Json.value("<unreadable>");
        }
        if (value instanceof EventBean event) {
            return eventRow(event);
        }
        if (value instanceof EventBean[] events) {
            JsonArray array = new JsonArray();
            for (EventBean event : events) {
                array.add(event == null ? normalize(null) : eventRow(event));
            }
            return array;
        }
        if (value instanceof Map<?, ?> mapValue) {
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            return row(fields);
        }
        if (value instanceof List<?> list) {
            JsonArray array = new JsonArray();
            for (Object item : list) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof SupportBeanComplexProps bean) {
            // Never selected by this suite; rendered for completeness from
            // the makeDefaultBean readable properties.
            JsonObject fields = new JsonObject();
            fields.add("arrayProperty", normalize(bean.getArrayProperty()));
            JsonArray indexed = new JsonArray();
            indexed.add(bean.getIndexed(0));
            indexed.add(bean.getIndexed(1));
            fields.add("indexed", indexed);
            fields.add("mapProperty", normalize(bean.getMapProperty()));
            fields.add("nested", normalize(bean.getNested()));
            fields.add("simpleProperty", normalize(bean.getSimpleProperty()));
            return row(fields);
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNested", normalize(nested.getNestedNested()));
            fields.add("nestedValue", normalize(nested.getNestedValue()));
            return row(fields);
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNestedNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNestedValue", normalize(nested.getNestedNestedValue()));
            return row(fields);
        }
        if (value instanceof Object[] objects) {
            JsonArray array = new JsonArray();
            for (Object object : objects) {
                array.add(normalize(object));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            int length = Array.getLength(value);
            JsonArray array = new JsonArray();
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            double number = ((Number) value).doubleValue();
            if (number == Math.rint(number) && !Double.isInfinite(number)) {
                return Json.value((long) number);
            }
            return Json.value(number);
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned steps. Deploy steps carry the verbatim EPL; send steps
     * carry the pinned event type and compacted payload; build-error steps
     * carry the verbatim probe EPL and an optional expectError. Unknown
     * step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        int expected = CASES.length;
        for (String[][] caseSteps : CASE_STEPS) {
            expected += caseSteps.length;
        }
        if (steps.size() != expected) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + expected + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            for (String[] pinned : CASE_STEPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!pinned[0].equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (pinned[0]) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!pinned[1].equals(string(step, "statement"))
                                || !pinned[3].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "deployed":
                        requireFields(step, "op", "case", "statement");
                        if (!pinned[1].equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deployed step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!pinned[2].equals(string(step, "eventType"))
                                || !pinned[4].equals(compact(step.get("payload")))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy":
                        requireFields(step, "op", "case", "statement");
                        if (!pinned[1].equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("undeploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    case "build-error":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!pinned[1].equals(string(step, "statement"))
                                || !pinned[3].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("build-error step " + cursor + " is not pinned");
                        }
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String compact(JsonValue value) {
        if (value == null) {
            return "";
        }
        return value.toString();
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private final Map<String, Long> sequences = new HashMap<>();

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            long sequence = sequences.merge(statement.getName(), 1L, Long::sum);
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(eventRow(event));
            }
            return output;
        }
    }
}
