import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanCombinedProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.runtime.client.DeploymentOptions;
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
 * Java oracle for PatternComplexPropertyAccess ords 0-2: complex property
 * paths (mapped/indexed/array/nested/combined) inside pattern filter
 * predicates over SupportBeanComplexProps/SupportBeanCombinedProps.
 *
 * Replays the eighteen spellings across three executions on one runtime,
 * mirroring the regression-suite harness: both bean event types, internal
 * timer disabled, and the rethrowing exception handler so statement
 * failures surface to the sender thread. Each case deploys the byte-exact
 * EPL script from the scenario (the PatternTestHarness USE_EPL text
 * "select * from pattern [atom]" for the sixteen ord 0 EventExpressionCase
 * atoms, the verbatim compileDeploy texts for ords 1-2), sends the pinned
 * event sequence, records s0 listener deliveries, and undeploys all
 * before the next case.
 *
 * Captured pattern tags arrive in select-* rows as the underlying beans.
 * SupportBeanComplexProps and SupportBeanCombinedProps expose their
 * mapped/indexed properties through parameterized getters only, so the
 * trace renders each captured bean from the payload that constructed it
 * (EPLOtherPatternEventProperties precedent): absent nullable fields
 * render {"state":"null"} and absent collection fields render null,
 * matching the Go schema rendering. The ord 2 sends carry a
 * simpleProperty marker (eventOne/eventTwo) replacing the regression
 * suite's assertSame identity assertions.
 */
public final class PatternComplexPropertyAccess575ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "pattern-complex-property-access-575";
    private static final String DESCRIPTION =
            "PatternComplexPropertyAccess ords 0-2: complex property access in pattern filters "
                    + "over SupportBeanComplexProps/SupportBeanCombinedProps — 16 "
                    + "EventExpressionCase spellings (mapped key, indexed, arrayProperty with an "
                    + "in-range check, nested and nested-nested navigation, and "
                    + "indexed[i].mapped(k).value combined chains including the wrong-value, "
                    + "missing-key, out-of-range-index and unknown-key no-fires), an every "
                    + "indexed[0]=3 filter capturing tag a, and the "
                    + "every-a->b(indexed[0]=a.indexed[0]) correlated followed-by with a single "
                    + "{eventOne,eventTwo} delivery.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternComplexPropertyAccess.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-5fd2cb676f155061c987",
            "java-runtime-5d046d758dced3b2d879",
            "java-runtime-fd23ca72c8ba2aaf67ad"
    };
    private static final String[] EXECUTION_NAMES = {
            "PatternComplexProperties",
            "PatternIndexedFilterProp",
            "PatternIndexedValueProp"
    };
    private static final String[] STATIC_IDS = {
            "java-be2858d5e4c76bbf7f85",
            "java-ec97467eeb93b4ff0e67",
            "java-9504034cfc1c929363d8"
    };

    // Verbatim transcription of PatternComplexPropertyAccess: the sixteen
    // EventExpressionCase atoms inside the PatternTestHarness USE_EPL
    // "select * from pattern [...]" wrapper, then the ord 1/2
    // compileDeploy texts.
    private static final String EPL_MAPPED_KEY =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "mapped('keyOne') = 'valueOne')]";
    private static final String EPL_INDEXED_1_EQ_2 =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps(indexed[1] = 2)]";
    private static final String EPL_INDEXED_0_EQ_2 =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps(indexed[0] = 2)]";
    private static final String EPL_ARRAY_1_EQ_20 =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps(arrayProperty[1] = 20)]";
    private static final String EPL_ARRAY_1_RANGE =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "arrayProperty[1] in (10:30))]";
    private static final String EPL_ARRAY_2_EQ_20 =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps(arrayProperty[2] = 20)]";
    private static final String EPL_NESTED_VALUE =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "nested.nestedValue = 'nestedValue')]";
    private static final String EPL_NESTED_DUMMY =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "nested.nestedValue = 'dummy')]";
    private static final String EPL_NESTED_NESTED =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "nested.nestedNested.nestedNestedValue = 'nestedNestedValue')]";
    private static final String EPL_NESTED_NESTED_X =
            "@name('s0') select * from pattern [s=SupportBeanComplexProps("
            + "nested.nestedNested.nestedNestedValue = 'x')]";
    private static final String EPL_COMBINED_INDEXED_1 =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "indexed[1].mapped('1mb').value = '1ma1')]";
    private static final String EPL_COMBINED_INDEXED_0 =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "indexed[0].mapped('1ma').value = 'x')]";
    private static final String EPL_COMBINED_ARRAY_0 =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "array[0].mapped('0ma').value = '0ma0')]";
    private static final String EPL_COMBINED_ARRAY_2 =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "array[2].mapped('x').value = 'x')]";
    private static final String EPL_COMBINED_ARRAY_879787 =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "array[879787].mapped('x').value = 'x')]";
    private static final String EPL_COMBINED_ARRAY_XXX =
            "@name('s0') select * from pattern [s=SupportBeanCombinedProps("
            + "array[0].mapped('xxx').value = 'x')]";
    private static final String EPL_INDEXED_FILTER_PROP =
            "@name('s0') select * from pattern[every a=SupportBeanComplexProps(indexed[0]=3)]";
    private static final String EPL_INDEXED_VALUE_PROP =
            "@name('s0') select * from pattern[every a=SupportBeanComplexProps -> "
            + "b=SupportBeanComplexProps(indexed[0] = a.indexed[0])]";

    private static final String[] CASE_NAMES = {
            "mapped-key", "indexed-1-eq-2", "indexed-0-eq-2-no-fire",
            "array-1-eq-20", "array-1-in-range", "array-2-eq-20-no-fire",
            "nested-value", "nested-value-no-fire",
            "nested-nested-value", "nested-nested-value-no-fire",
            "combined-indexed-mapped", "combined-indexed-mapped-no-fire",
            "combined-array-mapped", "combined-array-mapped-missing-key",
            "combined-array-out-of-range", "combined-array-unknown-key",
            "indexed-filter-prop", "indexed-value-prop"
    };
    private static final int[] CASE_ORDINALS = {
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2
    };
    private static final String[] CASE_EPLS = {
            EPL_MAPPED_KEY, EPL_INDEXED_1_EQ_2, EPL_INDEXED_0_EQ_2,
            EPL_ARRAY_1_EQ_20, EPL_ARRAY_1_RANGE, EPL_ARRAY_2_EQ_20,
            EPL_NESTED_VALUE, EPL_NESTED_DUMMY,
            EPL_NESTED_NESTED, EPL_NESTED_NESTED_X,
            EPL_COMBINED_INDEXED_1, EPL_COMBINED_INDEXED_0,
            EPL_COMBINED_ARRAY_0, EPL_COMBINED_ARRAY_2,
            EPL_COMBINED_ARRAY_879787, EPL_COMBINED_ARRAY_XXX,
            EPL_INDEXED_FILTER_PROP, EPL_INDEXED_VALUE_PROP
    };

    private static final String LISTENED_STATEMENT = "s0";
    private static final int EXPECTED_RECORDS = 11;
    private static final int EXPECTED_STEPS = 92;

    private PatternComplexPropertyAccess575ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternComplexPropertyAccess575ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBeanComplexProps.class);
        configuration.getCommon().addEventType(SupportBeanCombinedProps.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        Map<SupportBeanComplexProps, JsonObject> complexPayloads = new IdentityHashMap<>();
        Map<SupportBeanCombinedProps, JsonObject> combinedPayloads = new IdentityHashMap<>();
        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASE_NAMES) {
                runCase(caseName, configuration, runtime, allSteps, records,
                        complexPayloads, combinedPayloads);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
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

    /** Replays one case's steps on the shared runtime; sequence restarts per case. */
    private static void runCase(String caseName, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps,
                                JsonArray records,
                                Map<SupportBeanComplexProps, JsonObject> complexPayloads,
                                Map<SupportBeanCombinedProps, JsonObject> combinedPayloads)
            throws Exception {
        int[] seq = new int[] {0};
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (string(step, "op")) {
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(configuration);
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        if (LISTENED_STATEMENT.equals(statement.getName())) {
                            statement.addListener(listener(caseName, seq, records,
                                    complexPayloads, combinedPayloads));
                        }
                    }
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"),
                            complexPayloads, combinedPayloads);
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
    }

    /** Listener emitting one record per new row; an old stream is a drift failure. */
    private static UpdateListener listener(String caseName, int[] seq, JsonArray records,
                                           Map<SupportBeanComplexProps, JsonObject> complexPayloads,
                                           Map<SupportBeanCombinedProps, JsonObject> combinedPayloads) {
        return (newEvents, oldEvents, statement, runtime) -> {
            if (oldEvents != null && oldEvents.length > 0) {
                throw new IllegalStateException("unexpected old stream for " + caseName + "/"
                        + statement.getName());
            }
            if (newEvents == null) {
                return;
            }
            for (EventBean event : newEvents) {
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("sequence", seq[0]);
                record.add("time",
                        Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                JsonArray newRows = new JsonArray();
                newRows.add(row(event, complexPayloads, combinedPayloads));
                record.add("new", newRows);
                records.add(record);
            }
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject row(EventBean event,
                                  Map<SupportBeanComplexProps, JsonObject> complexPayloads,
                                  Map<SupportBeanCombinedProps, JsonObject> combinedPayloads) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name), complexPayloads, combinedPayloads));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object. Map values render as kind:row objects,
     * EventBeans recurse into their property names, and
     * SupportBeanComplexProps/SupportBeanCombinedProps render the pinned
     * payload projection recorded at send time.
     */
    private static JsonValue normalize(Object value,
                                       Map<SupportBeanComplexProps, JsonObject> complexPayloads,
                                       Map<SupportBeanCombinedProps, JsonObject> combinedPayloads) {
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
                fields.add(key, normalize(mapValue.get(key), complexPayloads, combinedPayloads));
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
                fields.add(prop, normalize(inner.get(prop), complexPayloads, combinedPayloads));
            }
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
            return complexProjection(payload);
        }
        if (value instanceof SupportBeanCombinedProps) {
            JsonObject payload = combinedPayloads.get(value);
            if (payload == null) {
                throw new IllegalStateException(
                        "SupportBeanCombinedProps without a recorded payload");
            }
            return combinedProjection(payload);
        }
        if (value instanceof EventBean[]) {
            JsonArray items = new JsonArray();
            for (EventBean item : (EventBean[]) value) {
                items.add(row(item, complexPayloads, combinedPayloads));
            }
            return items;
        }
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element, complexPayloads, combinedPayloads));
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
     * SupportBeanComplexProps captured-bean projection: the six payload
     * properties rendered raw when present. Absent nullable fields
     * (simpleProperty, nested) render {"state":"null"} and absent
     * collection fields (mapped, indexed, mapProperty, arrayProperty)
     * render null, matching the Go schema rendering where pointer fields
     * map to Null and nil maps/slices marshal as null.
     */
    private static JsonObject complexProjection(JsonObject payload) {
        JsonObject fields = new JsonObject();
        fields.add("arrayProperty", payloadField(payload, "arrayProperty", false));
        fields.add("indexed", payloadField(payload, "indexed", false));
        fields.add("mapProperty", payloadField(payload, "mapProperty", false));
        fields.add("mapped", payloadField(payload, "mapped", false));
        fields.add("nested", payloadField(payload, "nested", true));
        fields.add("simpleProperty", payloadField(payload, "simpleProperty", true));
        JsonObject rowObj = new JsonObject();
        rowObj.add("kind", "row");
        rowObj.add("fields", fields);
        return rowObj;
    }

    /**
     * SupportBeanCombinedProps captured-bean projection: the two payload
     * properties (getArray() aliases getIndexed()) rendered raw when
     * present and null when absent, matching the Go nil-slice rendering.
     */
    private static JsonObject combinedProjection(JsonObject payload) {
        JsonObject fields = new JsonObject();
        fields.add("array", payloadField(payload, "array", false));
        fields.add("indexed", payloadField(payload, "indexed", false));
        JsonObject rowObj = new JsonObject();
        rowObj.add("kind", "row");
        rowObj.add("fields", fields);
        return rowObj;
    }

    /** One projected payload field: raw JSON when present, else the Go-kind null. */
    private static JsonValue payloadField(JsonObject payload, String name, boolean nullState) {
        JsonValue value = payload.get(name);
        if (value != null) {
            return value;
        }
        if (nullState) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        return Json.NULL;
    }

    /**
     * Sends one pinned event: SupportBeanComplexProps builds the full
     * constructor shape (absent optional parts stay null, like the
     * regression int[]-only constructor leaves them) and
     * SupportBeanCombinedProps builds the NestedLevOne array — both
     * recording the payload for captured-bean rendering.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload,
                                  Map<SupportBeanComplexProps, JsonObject> complexPayloads,
                                  Map<SupportBeanCombinedProps, JsonObject> combinedPayloads) {
        switch (type) {
            case "SupportBeanComplexProps": {
                Properties mapped = null;
                JsonValue mappedJson = payload.get("mapped");
                if (mappedJson != null) {
                    mapped = new Properties();
                    for (Member member : mappedJson.asObject()) {
                        mapped.put(member.getName(), stringValue(member.getValue(), "mapped"));
                    }
                }
                int[] indexed = intArray(payload.get("indexed"), "indexed");
                Map<String, String> mapProperty = null;
                JsonValue mapJson = payload.get("mapProperty");
                if (mapJson != null) {
                    mapProperty = new HashMap<>();
                    for (Member member : mapJson.asObject()) {
                        mapProperty.put(member.getName(),
                                stringValue(member.getValue(), "mapProperty"));
                    }
                }
                int[] arrayProperty = intArray(payload.get("arrayProperty"), "arrayProperty");
                String nestedValue = null;
                String nestedNestedValue = null;
                JsonValue nestedJson = payload.get("nested");
                if (nestedJson != null) {
                    JsonObject nested = nestedJson.asObject();
                    nestedValue = string(nested, "nestedValue");
                    JsonObject nestedNested = object(nested.get("nestedNested"), "nestedNested");
                    nestedNestedValue = string(nestedNested, "nestedNestedValue");
                }
                String simpleProperty = null;
                JsonValue simpleJson = payload.get("simpleProperty");
                if (simpleJson != null) {
                    simpleProperty = simpleJson.asString();
                }
                SupportBeanComplexProps event = new SupportBeanComplexProps(
                        simpleProperty, mapped, indexed, mapProperty, arrayProperty,
                        nestedValue, nestedNestedValue);
                complexPayloads.put(event, payload);
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBeanCombinedProps": {
                JsonValue elementsJson = payload.get("indexed");
                if (elementsJson == null) {
                    elementsJson = payload.get("array");
                }
                JsonArray elements = array(elementsJson, "indexed");
                SupportBeanCombinedProps.NestedLevOne[] nested =
                        new SupportBeanCombinedProps.NestedLevOne[elements.size()];
                for (int index = 0; index < elements.size(); index++) {
                    JsonValue element = elements.get(index);
                    if (element.isNull()) {
                        continue; // trailing element left null on purpose
                    }
                    JsonObject member = element.asObject();
                    JsonObject mappedJson = object(member.get("mapped"), "mapped");
                    String[][] keysAndValues = new String[mappedJson.size()][2];
                    int slot = 0;
                    for (Member entry : mappedJson) {
                        JsonObject levTwo = object(entry.getValue(), "mapped value");
                        keysAndValues[slot][0] = entry.getName();
                        keysAndValues[slot][1] = string(levTwo, "value");
                        slot++;
                    }
                    nested[index] = new SupportBeanCombinedProps.NestedLevOne(keysAndValues);
                }
                SupportBeanCombinedProps event = new SupportBeanCombinedProps(nested);
                combinedPayloads.put(event, payload);
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static int[] intArray(JsonValue value, String label) {
        if (value == null) {
            return null;
        }
        JsonArray items = array(value, label);
        int[] result = new int[items.size()];
        for (int index = 0; index < items.size(); index++) {
            result[index] = (int) longInteger(items.get(index), label);
        }
        return result;
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASE_NAMES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[CASE_ORDINALS[index]].equals(
                            string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[CASE_ORDINALS[index]].equals(
                            string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (int index = 0; index < CASE_NAMES.length; index++) {
            offset = validateCaseSteps(steps, offset, CASE_NAMES[index], CASE_EPLS[index]);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateCaseSteps(JsonArray steps, int offset, String caseName,
                                         String epl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", epl);
        while (true) {
            JsonObject step = object(steps.get(offset), "case step");
            if ("undeploy-all".equals(string(step, "op"))) {
                requireFields(step, "op", "case");
                if (!caseName.equals(string(step, "case"))) {
                    throw new IllegalArgumentException("undeploy-all is not pinned for "
                            + caseName);
                }
                return offset + 1;
            }
            validateSend(steps.get(offset++), caseName);
        }
    }

    private static void validateSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        String eventType = string(step, "eventType");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !(eventType.equals("SupportBeanComplexProps")
                        || eventType.equals("SupportBeanCombinedProps"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        if (eventType.equals("SupportBeanComplexProps")) {
            for (Member member : payload) {
                switch (member.getName()) {
                    case "simpleProperty":
                    case "mapped":
                    case "indexed":
                    case "mapProperty":
                    case "arrayProperty":
                    case "nested":
                        break;
                    default:
                        throw new IllegalArgumentException(
                                "SupportBeanComplexProps payload is not pinned for " + caseName);
                }
            }
            if (!payload.names().contains("indexed")) {
                throw new IllegalArgumentException(
                        "SupportBeanComplexProps payload is not pinned for " + caseName);
            }
        } else {
            for (Member member : payload) {
                if (!member.getName().equals("indexed") && !member.getName().equals("array")) {
                    throw new IllegalArgumentException(
                            "SupportBeanCombinedProps payload is not pinned for " + caseName);
                }
            }
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
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

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
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
        if (!(value instanceof JsonNumber)) {
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
