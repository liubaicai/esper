import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.Map;
import java.util.Properties;
import java.util.Set;
import java.util.TreeSet;

/**
 * Java oracle for the EPL other pattern-event-properties parity scenario.
 *
 * Covers all four EPLOtherPatternEventProperties executions, one fresh
 * runtime per case. wildcard-simple-pattern replays
 * EPLOtherWildcardSimplePattern (ord 0): select * from
 * pattern [a=SupportBean] observed through the s0 listener; one default
 * SupportBean send delivers {a=event}. wildcard-or-pattern replays
 * EPLOtherWildcardOrPattern (ord 1): select * from
 * pattern [every(a=SupportBean or b=SupportBeanComplexProps)]; a default
 * SupportBean send delivers {a=event,b=null}, then a default
 * SupportBeanComplexProps send delivers {b=event,a=null}.
 * properties-simple-pattern replays EPLOtherPropertiesSimplePattern
 * (ord 2): select a, a as myEvent, a.intPrimitive as myInt, a.theString;
 * one SupportBean{intPrimitive=1,theString="test"} send delivers the event
 * under both a and myEvent plus the property columns.
 * properties-or-pattern replays EPLOtherPropertiesOrPattern (ord 3): the
 * nine-column projection over the every-or pattern; the
 * SupportBeanComplexProps send delivers simple/indexed/nestedVal with the
 * a-side columns null, then SupportBean{intPrimitive=2,theString="test2"}
 * delivers myInt/a.theString with the b-side columns null.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive (the
 * default bean sends theString null) and SupportBeanComplexProps payloads
 * carrying the makeDefaultBean values: simpleProperty "simple", mapped
 * {keyOne:valueOne,keyTwo:valueTwo}, indexed {1,2}, mapProperty
 * {xOne:yOne,xTwo:yTwo}, arrayProperty {10,20,30} and nested
 * {nestedValue,nestedNested:{nestedNestedValue}}.
 *
 * Observations are listener records only: {case, operation, statement,
 * sequence, time, new, old} emitted for the s0 statement. The Java
 * executions assert event identity with assertSame; identity is not
 * trace-observable, so tagged-event columns render the bean's pinned field
 * projection instead — SupportBean as {theString,intPrimitive} and
 * SupportBeanComplexProps as the six-property schema projection whose
 * nested/map members render as plain JSON objects, matching the Go
 * struct-field rendering exactly.
 */
public final class EPLOtherPatternEventPropertiesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-pattern-event-properties";
    private static final String DESCRIPTION =
            "EPLOtherPatternEventProperties ords 0-3: event-pattern tag "
                    + "projections. Ord 0 selects * (the tagged event) from "
                    + "pattern [a=SupportBean]. Ord 1 selects * from pattern "
                    + "[every(a=SupportBean or b=SupportBeanComplexProps)] with "
                    + "the absent OR-branch tag projecting null. Ord 2 selects "
                    + "the tagged event twice plus its intPrimitive/theString "
                    + "properties. Ord 3 selects both tagged events, their "
                    + "aliases, and the simple/indexed/nested property paths "
                    + "across the every-or pattern (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/epl/other/"
                    + "EPLOtherPatternEventProperties.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/"
                    + "EPLOtherPatternEventProperties.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-97cfec67b539a40837da",
            "java-runtime-157bfa584c8111e1ec07",
            "java-runtime-ec5a7e8cfd338e68a315",
            "java-runtime-e469a171adadf0bcd221"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherWildcardSimplePattern",
            "EPLOtherWildcardOrPattern",
            "EPLOtherPropertiesSimplePattern",
            "EPLOtherPropertiesOrPattern"
    };
    private static final String[] STATIC_IDS = {
            "java-526cb327bfe0c4b23edb",
            "java-e66c56ee60a585967eac",
            "java-6f9f30d76350678d1ff0",
            "java-e3432d4a451aede65cac"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "wildcard-simple-pattern",
            "wildcard-or-pattern",
            "properties-simple-pattern",
            "properties-or-pattern"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; one deploy selects * from pattern [a=SupportBean]; one "
                    + "default SupportBean send delivers {a=event}",
            "listener; one deploy selects * from pattern [every(a=SupportBean "
                    + "or b=SupportBeanComplexProps)]; a default SupportBean "
                    + "send delivers {a=event,b=null} then a default "
                    + "SupportBeanComplexProps send delivers {b=event,a=null}",
            "listener; one deploy selects a, a as myEvent, a.intPrimitive as "
                    + "myInt, a.theString from pattern [a=SupportBean]; one "
                    + "SupportBean{intPrimitive=1,theString=test} send delivers "
                    + "{a=event,myEvent=event,myInt=1,a.theString=test}",
            "listener; one deploy selects the nine-column projection from "
                    + "pattern [every(a=SupportBean or b=SupportBeanComplexProps)]; "
                    + "a default SupportBeanComplexProps send delivers "
                    + "{b=event,simple=simple,indexed=1,nestedVal=nestedValue} "
                    + "with the a-side columns null, then "
                    + "SupportBean{intPrimitive=2,theString=test2} delivers "
                    + "{myInt=2,a.theString=test2} with the b-side columns null"
    };

    // Verbatim transcriptions of EPLOtherPatternEventProperties:
    // setupSimplePattern (line 125) wraps select criteria "*" (ord 0,
    // line 35) and "a, a as myEvent, a.intPrimitive as myInt, a.theString"
    // (ord 2, line 70); setupOrPattern (lines 130-131) wraps "*" (ord 1,
    // line 48) and the nine-column criteria (ord 3, lines 90-91).
    private static final String EPL_WILDCARD_SIMPLE =
            "@name('s0') select * from pattern [a=SupportBean]";
    private static final String EPL_OR_WILDCARD =
            "@name('s0') select * from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]";
    private static final String EPL_SIMPLE_PROPS =
            "@name('s0') select a, a as myEvent, a.intPrimitive as myInt, a.theString "
                    + "from pattern [a=SupportBean]";
    private static final String EPL_OR_PROPS =
            "@name('s0') select a, a as myAEvent, b, b as myBEvent, a.intPrimitive as myInt, "
                    + "a.theString, b.simpleProperty as simple, b.indexed[0] as indexed, "
                    + "b.nested.nestedValue as nestedVal from pattern [every(a=SupportBean "
                    + "or b=SupportBeanComplexProps)]";

    private static final int EXPECTED_STEPS = 10;
    private static final int EXPECTED_RECORDS = 6;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|fields|listen. Send payloads
     * render as their compact JSON; each Java execution deploys its
     * statement before sending, so the oracle deploys the pinned EPL at
     * case start and the steps carry sends only.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("wildcard-simple-pattern", new String[]{
                sendKey("wildcard-simple-pattern", "SupportBean",
                        "{\"theString\":null,\"intPrimitive\":0}"),
        });
        CASE_STEPS.put("wildcard-or-pattern", new String[]{
                sendKey("wildcard-or-pattern", "SupportBean",
                        "{\"theString\":null,\"intPrimitive\":0}"),
                sendKey("wildcard-or-pattern", "SupportBeanComplexProps",
                        "{\"simpleProperty\":\"simple\",\"mapped\":{\"keyOne\":\"valueOne\",\"keyTwo\":\"valueTwo\"},"
                                + "\"indexed\":[1,2],\"mapProperty\":{\"xOne\":\"yOne\",\"xTwo\":\"yTwo\"},"
                                + "\"arrayProperty\":[10,20,30],\"nested\":{\"nestedValue\":\"nestedValue\","
                                + "\"nestedNested\":{\"nestedNestedValue\":\"nestedNestedValue\"}}}"),
        });
        CASE_STEPS.put("properties-simple-pattern", new String[]{
                sendKey("properties-simple-pattern", "SupportBean",
                        "{\"theString\":\"test\",\"intPrimitive\":1}"),
        });
        CASE_STEPS.put("properties-or-pattern", new String[]{
                sendKey("properties-or-pattern", "SupportBeanComplexProps",
                        "{\"simpleProperty\":\"simple\",\"mapped\":{\"keyOne\":\"valueOne\",\"keyTwo\":\"valueTwo\"},"
                                + "\"indexed\":[1,2],\"mapProperty\":{\"xOne\":\"yOne\",\"xTwo\":\"yTwo\"},"
                                + "\"arrayProperty\":[10,20,30],\"nested\":{\"nestedValue\":\"nestedValue\","
                                + "\"nestedNested\":{\"nestedNestedValue\":\"nestedNestedValue\"}}}"),
                sendKey("properties-or-pattern", "SupportBean",
                        "{\"theString\":\"test2\",\"intPrimitive\":2}"),
        });
    }

    private static String sendKey(String caseName, String eventType, String payloadJson) {
        return "send|" + caseName + "||" + eventType + "||" + payloadJson + "||";
    }

    private EPLOtherPatternEventPropertiesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherPatternEventPropertiesScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). SupportBean and SupportBeanComplexProps are
     * preconfigured beans; the internal timer is disabled and the
     * rethrowing exception handler surfaces statement failures to the
     * sender thread. The case's pinned EPL deploys before the sends,
     * mirroring compileDeploy(epl).addListener("s0"); undeployAll runs on
     * the way out.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanComplexProps.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        // The SupportBeanComplexProps bean exposes no collection getters for
        // its mapped/indexed accessor properties, so the listener renders
        // tagged-event columns from the payload that constructed each bean.
        Map<SupportBeanComplexProps, JsonObject> complexPayloads = new IdentityHashMap<>();
        try {
            EPDeployment deployment = compileDeploy(runtime, configuration, caseEpl(caseName));
            for (EPStatement statement : deployment.getStatements()) {
                if ("s0".equals(statement.getName())) {
                    statement.addListener(
                            listener(caseName, sequences, records, runtime, complexPayloads));
                }
            }

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
                if ("send".equals(operation)) {
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"), complexPayloads);
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
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

    /** compileDeploy mirrors env.compileDeploy(epl): module compile against
     * the full Configuration followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration, String epl)
            throws EPCompileException, EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; the default istream selector means only a new
     * array renders and only when non-empty. Each Java execution asserts
     * exactly one new row per delivery (assertEventNew).
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           Map<SupportBeanComplexProps, JsonObject> complexPayloads) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", rows(newEvents, complexPayloads));
            record.add("old", rows(oldEvents, complexPayloads));
            records.add(record);
        };
    }

    /** Canonical row rendering over the event type's sorted property names. */
    private static JsonArray rows(EventBean[] events,
                                  Map<SupportBeanComplexProps, JsonObject> complexPayloads) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject rowFields = new JsonObject();
            for (String prop : new TreeSet<>(
                    Arrays.asList(event.getEventType().getPropertyNames()))) {
                rowFields.add(prop, normalize(event.get(prop), complexPayloads));
            }
            item.add("fields", rowFields);
            array.add(item);
        }
        return array;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, Map/EventBean values as kind:row objects,
     * and Object[]/int[] as JSON arrays. Tagged-event columns arrive as
     * the underlying bean: SupportBean renders its asserted-field
     * projection {theString,intPrimitive} and SupportBeanComplexProps
     * renders the six-property schema projection from the payload that
     * constructed it (the bean exposes no collection getters for the
     * mapped/indexed accessor properties).
     */
    private static JsonValue normalize(Object value,
                                       Map<SupportBeanComplexProps, JsonObject> complexPayloads) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
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
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key), complexPayloads));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(
                    Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop), complexPayloads));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBean) {
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString(), complexPayloads));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive(), complexPayloads));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBeanComplexProps) {
            JsonObject payload = complexPayloads.get(value);
            if (payload == null) {
                throw new IllegalStateException(
                        "SupportBeanComplexProps without a recorded payload");
            }
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(payload.names())) {
                fields.add(prop, renderJson(payload.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element, complexPayloads));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Renders one scenario JSON value into the trace protocol: JSON null
     * becomes the tagged {"state":"null"} object, objects and arrays
     * render recursively as plain JSON (matching the Go struct/map field
     * rendering), and scalars pass through.
     */
    private static JsonValue renderJson(JsonValue value) {
        if (value == null || value.isNull()) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value.isObject()) {
            JsonObject fields = new JsonObject();
            for (Member member : value.asObject()) {
                fields.add(member.getName(), renderJson(member.getValue()));
            }
            return fields;
        }
        if (value.isArray()) {
            JsonArray items = new JsonArray();
            for (JsonValue element : value.asArray()) {
                items.add(renderJson(element));
            }
            return items;
        }
        return value;
    }

    /**
     * Sends one pinned event: SupportBean builds the bean from theString/
     * intPrimitive (a JSON null theString stays null) and
     * SupportBeanComplexProps builds the makeDefaultBean shape from the
     * payload's six properties, recording the payload for tagged-event
     * rendering.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload,
                                  Map<SupportBeanComplexProps, JsonObject> complexPayloads) {
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean();
                JsonValue theString = payload.get("theString");
                if (theString instanceof JsonString) {
                    event.setTheString(theString.asString());
                }
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null && intPrimitive.isNumber()) {
                    event.setIntPrimitive((int) longInteger(intPrimitive, "intPrimitive"));
                }
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBeanComplexProps": {
                Properties mapped = new Properties();
                JsonObject mappedJson = object(payload.get("mapped"), "mapped");
                for (Member member : mappedJson) {
                    mapped.put(member.getName(), stringValue(member.getValue(), "mapped"));
                }
                JsonArray indexedJson = array(payload.get("indexed"), "indexed");
                int[] indexed = new int[indexedJson.size()];
                for (int index = 0; index < indexedJson.size(); index++) {
                    indexed[index] = (int) longInteger(indexedJson.get(index), "indexed");
                }
                Map<String, String> mapProperty = new HashMap<>();
                JsonObject mapJson = object(payload.get("mapProperty"), "mapProperty");
                for (Member member : mapJson) {
                    mapProperty.put(member.getName(), stringValue(member.getValue(), "mapProperty"));
                }
                JsonArray arrayJson = array(payload.get("arrayProperty"), "arrayProperty");
                int[] arrayProperty = new int[arrayJson.size()];
                for (int index = 0; index < arrayJson.size(); index++) {
                    arrayProperty[index] = (int) longInteger(arrayJson.get(index), "arrayProperty");
                }
                JsonObject nested = object(payload.get("nested"), "nested");
                JsonObject nestedNested = object(nested.get("nestedNested"), "nestedNested");
                SupportBeanComplexProps event = new SupportBeanComplexProps(
                        string(payload, "simpleProperty"), mapped, indexed, mapProperty,
                        arrayProperty, string(nested, "nestedValue"),
                        string(nestedNested, "nestedNestedValue"));
                complexPayloads.put(event, payload);
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
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
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the single deploy EPL of the case. */
    private static String caseEpl(String caseName) {
        return switch (caseName) {
            case "wildcard-simple-pattern" -> EPL_WILDCARD_SIMPLE;
            case "wildcard-or-pattern" -> EPL_OR_WILDCARD;
            case "properties-simple-pattern" -> EPL_SIMPLE_PROPS;
            case "properties-or-pattern" -> EPL_OR_PROPS;
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|fields|listen with the
     * payload compacted and fields rendered comma-joined. Unknown fields
     * are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload", "fields", "listen"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + fieldsText + "|" + string(step, "listen");
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
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

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static String stringValue(JsonValue value, String label) {
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(label + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
