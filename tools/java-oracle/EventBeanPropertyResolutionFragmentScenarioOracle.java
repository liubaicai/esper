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
import com.espertech.esper.regressionlib.support.bean.SupportBeanCombinedProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
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
import java.util.Properties;

/**
 * Java oracle for the event-bean-property-fragment parity scenario (pinned
 * Esper 9.0.0 commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c). Replays the
 * fifteen listener-observable executions of
 * regression-lib/.../suite/event/bean/EventBeanPropertyResolutionFragment.java
 * against one fresh runtime per case, with the pinned EPL compiled and
 * deployed through the compiler plus deployment service:
 *
 * - map-simple-types (ordinal 0): select * from MSTypeOne; the pinned payload
 *   carries p0simple/p0array/p0map keys which are NOT part of the event type
 *   (p0int/p0intarray/p0map), so the row renders p0int and p0intarray as null.
 * - object-array-simple-types (ordinal 1): select * from OASimple.
 * - wrapper-map (ordinal 2): select *, p0simple.p1id + 1 as plusone,
 *   p0bean as mybean from Frosty.
 * - wrapper-object-array (ordinal 3): same projection over WheatRoot.
 * - native-bean-fragment (ordinal 4): two phases, each with an undeployAll
 *   between: first SupportBeanComplexProps, then SupportBeanCombinedProps.
 * - map-nested (ordinal 5): select * from HomerunRoot.
 * - object-array-nested (ordinal 6): select * from GoalRoot.
 * - map-unnamed (ordinal 7): select * from FlywheelRoot; the p0simple property
 *   uses an inline (unnamed) nested map type.
 * - transposed-map (ordinal 8): pattern[one=GistMapOne until two=GistMapTwo];
 *   two GistMapOne events (id 1, id 2) are replayed before the terminating
 *   GistMapTwo event (id 3) in pinned order.
 * - transposed-object-array (ordinal 9): the same transpose over CashMapOne/
 *   CashMapTwo object-array events.
 * - map-beans (ordinal 10): select * from TXTypeRoot holding bean fragments.
 * - object-array-beans (ordinal 11): select * from LocalTypeRoot.
 * - map-3level (ordinal 12): select * from JimTypeRoot (three named levels).
 * - object-array-3level (ordinal 13): select * from JackTypeRoot.
 * - map-multi (ordinal 14): select * from MMOuterMap.
 *
 * Event types follow the pinned suite's configuration exactly
 * (regression-run/.../suite/event/TestSuiteEventBean.java lines 135-263):
 * MSTypeOne/OASimple are the simple map/object-array types; Frosty/Wheat
 * wrap a named level-one fragment plus a SupportBeanComplexProps bean
 * fragment; Homerun/Goal wrap a named fragment and an array-of-fragment;
 * FlywheelRoot wraps an INLINE unnamed map (no fragment); GistMapOne/
 * GistMapTwo and CashMapOne/CashMapTwo carry bean/complex/bean-array/
 * complex-array/map/map-array properties and are the until-transpose event
 * types; TXType/LocalType carry bean fragments inside a named level-one
 * map/object-array; JimType/JackType add a third named level; MMOuterMap
 * carries an MMInnerMap-valued fragment with bean, complex, bean-array and
 * inline inner-map properties. SupportBean, SupportBeanComplexProps and
 * SupportBeanCombinedProps are registered as real bean classes so that the
 * fragment resolution behavior matches the pinned assertions.
 *
 * Listener records follow the standard protocol: one record per delivery,
 * per-statement sequence numbering from 1 per deployment, time rendered from
 * engine time, new/old arrays rendered with the scalar normalization rules
 * (Integer/Long/Short/Byte -> long, other Number -> double including
 * BigDecimal, null -> {state:null}). Nested map values render with sorted
 * keys; EventBean values render with __type plus sorted properties; arrays
 * and collections render recursively. Because the pinned bean classes do not
 * implement toString, and the Java trace must be reproducible across runs,
 * bean values are rendered deterministically through their readable
 * property surface: SupportBean through its pinned toString (the class
 * implements it), SupportBeanSpecialGetterNested as {nestedValue,
 * nestedNestedValue}, SupportBeanComplexProps as {simpleProperty,
 * mapProperty, arrayProperty, nested, objectArray}, NestedLevOne as
 * {mapprop, nestLevOneVal} with NestedLevTwo values as {value}.
 *
 * The Java fragment API checks of the pinned executions (isFragment /
 * getFragmentType / getFragment / get) are not part of the EPL-observable
 * contract and are covered on the Go side as engine unit tests; this oracle
 * therefore records only the listener rows. The Go runner replays the same
 * scenario steps and its trace is compared against this oracle trace.
 *
 * Scenario step ops: case markers ("case"), sends ("send" with eventType and
 * a payload that is a JSON object for map/bean event types and a JSON array
 * for object-array event types, carrying the full pinned event payloads) and
 * pinned statement deployments ("deploy" with the statement label matching
 * the pinned plan; the pinned suite undeploys before each next deployment,
 * so a deploy op replaces the prior one).
 */
public class EventBeanPropertyResolutionFragmentScenarioOracle {

    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private static final String[] CASES = {
        "map-simple-types",
        "object-array-simple-types",
        "wrapper-map",
        "wrapper-object-array",
        "native-bean-fragment",
        "map-nested",
        "object-array-nested",
        "map-unnamed",
        "transposed-map",
        "transposed-object-array",
        "map-beans",
        "object-array-beans",
        "map-3level",
        "object-array-3level",
        "map-multi"
    };

    private EventBeanPropertyResolutionFragmentScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventBeanPropertyResolutionFragmentScenarioOracle <scenario.json>");
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
        Schemas schemas = new Schemas();
        registerTypes(config, schemas);
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
                    sendEvent(runtime, schemas, step);
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
                    deployPinned(runtime, config, schemas, records, caseName, deployIndex, plan[deployIndex]);
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

    private static void deployPinned(EPRuntime runtime, Configuration config, Schemas schemas, List<JsonObject> records,
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
            case "map-simple-types" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from MSTypeOne", new String[]{"s0"}),
            };
            case "object-array-simple-types" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from OASimple", new String[]{"s0"}),
            };
            case "wrapper-map" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select *, p0simple.p1id + 1 as plusone, p0bean as mybean from Frosty",
                    new String[]{"s0"}),
            };
            case "wrapper-object-array" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select *, p0simple.p1id + 1 as plusone, p0bean as mybean from WheatRoot",
                    new String[]{"s0"}),
            };
            case "native-bean-fragment" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from SupportBeanComplexProps", new String[]{"s0"}),
                new ScenarioPlan("s0", "@name('s0') select * from SupportBeanCombinedProps", new String[]{"s0"}),
            };
            case "map-nested" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from HomerunRoot", new String[]{"s0"}),
            };
            case "object-array-nested" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from GoalRoot", new String[]{"s0"}),
            };
            case "map-unnamed" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from FlywheelRoot", new String[]{"s0"}),
            };
            case "transposed-map" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[one=GistMapOne until two=GistMapTwo]",
                    new String[]{"s0"}),
            };
            case "transposed-object-array" -> new ScenarioPlan[] {
                new ScenarioPlan("s0",
                    "@name('s0') select * from pattern[one=CashMapOne until two=CashMapTwo]",
                    new String[]{"s0"}),
            };
            case "map-beans" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from TXTypeRoot", new String[]{"s0"}),
            };
            case "object-array-beans" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from LocalTypeRoot", new String[]{"s0"}),
            };
            case "map-3level" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from JimTypeRoot", new String[]{"s0"}),
            };
            case "object-array-3level" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from JackTypeRoot", new String[]{"s0"}),
            };
            case "map-multi" -> new ScenarioPlan[] {
                new ScenarioPlan("s0", "@name('s0') select * from MMOuterMap", new String[]{"s0"}),
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static Configuration buildConfiguration() {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);

        // Beans are registered as real classes: the pinned fragment API
        // behavior and the EPL property resolution depend on the bean
        // property surface exactly as preconfigured by TestSuiteEventBean.
        config.getCommon().addEventType("SupportBean", SupportBean.class);
        config.getCommon().addEventType("SupportBeanComplexProps", SupportBeanComplexProps.class);
        config.getCommon().addEventType("SupportBeanCombinedProps", SupportBeanCombinedProps.class);
        return config;
    }

    /** The map/object-array type schemas mirroring TestSuiteEventBean L135-263. */
    private static final class Schemas {
        private final Map<String, Map<String, Object>> mapTypes = new LinkedHashMap<>();
        private final Map<String, List<Object>> oaTypes = new LinkedHashMap<>();
    }

    private static void registerTypes(Configuration config, Schemas schemas) {
        // MSTypeOne: a map with scalars only (p0map has no nested schema).
        Map<String, Object> mstype = new LinkedHashMap<>();
        mstype.put("p0int", int.class);
        mstype.put("p0intarray", int[].class);
        mstype.put("p0map", Map.class);
        config.getCommon().addEventType("MSTypeOne", mstype);
        schemas.mapTypes.put("MSTypeOne", mstype);

        // OASimple: the object-array twin of MSTypeOne.
        addOA(config, schemas, "OASimple",
            new String[]{"p0int", "p0intarray", "p0map"},
            new Object[]{int.class, int[].class, Map.class});

        // Frosty: named map fragment plus bean fragment.
        Map<String, Object> frostyLev0 = new LinkedHashMap<>();
        frostyLev0.put("p1id", int.class);
        config.getCommon().addEventType("FrostyLev0", frostyLev0);
        schemas.mapTypes.put("FrostyLev0", frostyLev0);

        Map<String, Object> frosty = new LinkedHashMap<>();
        frosty.put("p0simple", "FrostyLev0");
        frosty.put("p0bean", SupportBeanComplexProps.class);
        config.getCommon().addEventType("Frosty", frosty);
        schemas.mapTypes.put("Frosty", frosty);

        // Wheat: the object-array twin of Frosty.
        addOA(config, schemas, "WheatLev0", new String[]{"p1id"}, new Object[]{int.class});
        addOA(config, schemas, "WheatRoot",
            new String[]{"p0simple", "p0bean"},
            new Object[]{"WheatLev0", SupportBeanComplexProps.class});

        // Homerun: named map fragment plus array of the same fragment.
        Map<String, Object> homerunLev0 = new LinkedHashMap<>();
        homerunLev0.put("p1id", int.class);
        config.getCommon().addEventType("HomerunLev0", homerunLev0);
        schemas.mapTypes.put("HomerunLev0", homerunLev0);

        Map<String, Object> homerunRoot = new LinkedHashMap<>();
        homerunRoot.put("p0simple", "HomerunLev0");
        homerunRoot.put("p0array", "HomerunLev0[]");
        config.getCommon().addEventType("HomerunRoot", homerunRoot);
        schemas.mapTypes.put("HomerunRoot", homerunRoot);

        // Goal: the object-array twin of Homerun.
        addOA(config, schemas, "GoalLev0", new String[]{"p1id"}, new Object[]{int.class});
        addOA(config, schemas, "GoalRoot",
            new String[]{"p0simple", "p0array"},
            new Object[]{"GoalLev0", "GoalLev0[]"});

        // Flywheel: the p0simple property is an INLINE named-less map schema.
        Map<String, Object> flywheelTypeLev0 = new LinkedHashMap<>();
        flywheelTypeLev0.put("p1id", int.class);
        Map<String, Object> flywheelRoot = new LinkedHashMap<>();
        flywheelRoot.put("p0simple", flywheelTypeLev0);
        config.getCommon().addEventType("FlywheelRoot", flywheelRoot);
        schemas.mapTypes.put("FlywheelRoot", flywheelRoot);

        // Gist: the until-transpose map event types.
        Map<String, Object> gistInner = new LinkedHashMap<>();
        gistInner.put("p2id", int.class);
        config.getCommon().addEventType("GistInner", gistInner);
        schemas.mapTypes.put("GistInner", gistInner);

        Map<String, Object> typeMap = new LinkedHashMap<>();
        typeMap.put("id", int.class);
        typeMap.put("bean", SupportBean.class);
        typeMap.put("beanarray", SupportBean[].class);
        typeMap.put("complex", SupportBeanComplexProps.class);
        typeMap.put("complexarray", SupportBeanComplexProps[].class);
        typeMap.put("map", "GistInner");
        typeMap.put("maparray", "GistInner[]");
        config.getCommon().addEventType("GistMapOne", typeMap);
        config.getCommon().addEventType("GistMapTwo", typeMap);
        schemas.mapTypes.put("GistMapOne", typeMap);
        schemas.mapTypes.put("GistMapTwo", typeMap);

        // Cash: the object-array twin of Gist.
        addOA(config, schemas, "CashInner", new String[]{"p2id"}, new Object[]{int.class});
        addOA(config, schemas, "CashMapOne",
            new String[]{"id", "bean", "beanarray", "complex", "complexarray", "map", "maparray"},
            new Object[]{int.class, SupportBean.class, SupportBean[].class, SupportBeanComplexProps.class,
                SupportBeanComplexProps[].class, "CashInner", "CashInner[]"});
        addOA(config, schemas, "CashMapTwo",
            new String[]{"id", "bean", "beanarray", "complex", "complexarray", "map", "maparray"},
            new Object[]{int.class, SupportBean.class, SupportBean[].class, SupportBeanComplexProps.class,
                SupportBeanComplexProps[].class, "CashInner", "CashInner[]"});

        // TXType: map roots holding bean fragments.
        Map<String, Object> txTypeLev0 = new LinkedHashMap<>();
        txTypeLev0.put("p1simple", SupportBean.class);
        txTypeLev0.put("p1array", SupportBean[].class);
        txTypeLev0.put("p1complex", SupportBeanComplexProps.class);
        txTypeLev0.put("p1complexarray", SupportBeanComplexProps[].class);
        config.getCommon().addEventType("TXTypeLev0", txTypeLev0);
        schemas.mapTypes.put("TXTypeLev0", txTypeLev0);

        Map<String, Object> txTypeRoot = new LinkedHashMap<>();
        txTypeRoot.put("p0simple", "TXTypeLev0");
        txTypeRoot.put("p0array", "TXTypeLev0[]");
        config.getCommon().addEventType("TXTypeRoot", txTypeRoot);
        schemas.mapTypes.put("TXTypeRoot", txTypeRoot);

        // LocalType: the object-array twin of TXType.
        addOA(config, schemas, "LocalTypeLev0",
            new String[]{"p1simple", "p1array", "p1complex", "p1complexarray"},
            new Object[]{SupportBean.class, SupportBean[].class, SupportBeanComplexProps.class,
                SupportBeanComplexProps[].class});
        addOA(config, schemas, "LocalTypeRoot",
            new String[]{"p0simple", "p0array"},
            new Object[]{"LocalTypeLev0", "LocalTypeLev0[]"});

        // JimType: three named map levels.
        Map<String, Object> jimTypeLev1 = new LinkedHashMap<>();
        jimTypeLev1.put("p2id", int.class);
        config.getCommon().addEventType("JimTypeLev1", jimTypeLev1);
        schemas.mapTypes.put("JimTypeLev1", jimTypeLev1);

        Map<String, Object> jimTypeLev0 = new LinkedHashMap<>();
        jimTypeLev0.put("p1simple", "JimTypeLev1");
        jimTypeLev0.put("p1array", "JimTypeLev1[]");
        config.getCommon().addEventType("JimTypeLev0", jimTypeLev0);
        schemas.mapTypes.put("JimTypeLev0", jimTypeLev0);

        Map<String, Object> jimTypeRoot = new LinkedHashMap<>();
        jimTypeRoot.put("p0simple", "JimTypeLev0");
        jimTypeRoot.put("p0array", "JimTypeLev0[]");
        config.getCommon().addEventType("JimTypeRoot", jimTypeRoot);
        schemas.mapTypes.put("JimTypeRoot", jimTypeRoot);

        // JackType: the object-array twin of JimType.
        addOA(config, schemas, "JackTypeLev1", new String[]{"p2id"}, new Object[]{int.class});
        addOA(config, schemas, "JackTypeLev0",
            new String[]{"p1simple", "p1array"},
            new Object[]{"JackTypeLev1", "JackTypeLev1[]"});
        addOA(config, schemas, "JackTypeRoot",
            new String[]{"p0simple", "p0array"},
            new Object[]{"JackTypeLev0", "JackTypeLev0[]"});

        // MM: the p1innerMap property of MMInnerMap is an INLINE map schema;
        // MMInnerMap itself is a named map type.
        Map<String, Object> mmInner = new LinkedHashMap<>();
        mmInner.put("p2id", int.class);

        Map<String, Object> mmInnerMap = new LinkedHashMap<>();
        mmInnerMap.put("p1bean", SupportBean.class);
        mmInnerMap.put("p1beanComplex", SupportBeanComplexProps.class);
        mmInnerMap.put("p1beanArray", SupportBean[].class);
        mmInnerMap.put("p1innerId", int.class);
        mmInnerMap.put("p1innerMap", mmInner);
        config.getCommon().addEventType("MMInnerMap", mmInnerMap);
        schemas.mapTypes.put("MMInnerMap", mmInnerMap);

        Map<String, Object> mmOuterMap = new LinkedHashMap<>();
        mmOuterMap.put("p0simple", "MMInnerMap");
        mmOuterMap.put("p0array", "MMInnerMap[]");
        config.getCommon().addEventType("MMOuterMap", mmOuterMap);
        schemas.mapTypes.put("MMOuterMap", mmOuterMap);
    }

    /** Registers an object-array event type and its positional schema, mirroring
     * the pinned addEventType(name, String[], Object[]) signature. */
    private static void addOA(Configuration config, Schemas schemas, String name,
                              String[] propertyNames, Object[] propertyTypes) {
        config.getCommon().addEventType(name, propertyNames, propertyTypes);
        List<Object> schema = new ArrayList<>();
        Collections.addAll(schema, propertyTypes);
        schemas.oaTypes.put(name, schema);
    }

    private static void sendEvent(EPRuntime runtime, Schemas schemas, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonValue payload = step.get("payload");
        switch (eventType) {
            case "SupportBean" -> {
                JsonObject object = payload.asObject();
                runtime.getEventService().sendEventBean(
                    new SupportBean(object.getString("theString", null), object.getInt("intPrimitive", 0)),
                    eventType);
            }
            case "SupportBeanComplexProps" -> {
                runtime.getEventService().sendEventBean(complexProps(payload.asObject()), eventType);
            }
            case "SupportBeanCombinedProps" -> {
                runtime.getEventService().sendEventBean(combinedProps(payload.asObject()), eventType);
            }
            default -> {
                Map<String, Object> mapSchema = schemas.mapTypes.get(eventType);
                List<Object> oaSchema = schemas.oaTypes.get(eventType);
                if (mapSchema != null) {
                    runtime.getEventService().sendEventMap(convertPayloadMap(schemas, mapSchema, payload), eventType);
                } else if (oaSchema != null) {
                    runtime.getEventService().sendEventObjectArray(convertPayloadOA(schemas, oaSchema, payload), eventType);
                } else {
                    throw new IllegalStateException("unknown eventType: " + eventType);
                }
            }
        }
    }

    /** Builds a SupportBeanComplexProps instance from the payload fields. */
    private static SupportBeanComplexProps complexProps(JsonObject object) {
        Properties mapped = new Properties();
        JsonValue mappedValue = object.get("mapped");
        if (mappedValue != null && !mappedValue.isNull() && mappedValue.isObject()) {
            for (String key : mappedValue.asObject().names()) {
                mapped.put(key, mappedValue.asObject().get(key).asString());
            }
        }
        int[] indexed = intArray(object.get("indexed"));
        Map<String, String> mapProperty = new LinkedHashMap<>();
        JsonValue mapValue = object.get("mapProperty");
        if (mapValue != null && !mapValue.isNull() && mapValue.isObject()) {
            for (String key : mapValue.asObject().names()) {
                mapProperty.put(key, mapValue.asObject().get(key).asString());
            }
        }
        int[] arrayProperty = intArray(object.get("arrayProperty"));
        SupportBeanComplexProps bean = new SupportBeanComplexProps(
            object.getString("simpleProperty", null), mapped, indexed, mapProperty, arrayProperty,
            object.getString("nestedValue", null), object.getString("nestedNestedValue", null));
        JsonValue objectArray = object.get("objectArray");
        bean.setObjectArray(objectArray == null || objectArray.isNull() ? null : rawArray(objectArray));
        return bean;
    }

    /** Builds a SupportBeanCombinedProps instance from the nested payload. */
    private static SupportBeanCombinedProps combinedProps(JsonObject object) {
        JsonArray entries = object.get("array").asArray();
        SupportBeanCombinedProps.NestedLevOne[] nested = new SupportBeanCombinedProps.NestedLevOne[entries.size()];
        for (int i = 0; i < entries.size(); i++) {
            JsonValue entry = entries.get(i);
            nested[i] = entry.isNull() ? null : nestedLevOne(entry.asObject());
        }
        return new SupportBeanCombinedProps(nested);
    }

    private static SupportBeanCombinedProps.NestedLevOne nestedLevOne(JsonObject object) {
        String[][] keysAndValues = new String[object.size()][];
        int index = 0;
        for (String key : object.names()) {
            keysAndValues[index++] = new String[]{key, object.get(key).asString()};
        }
        return new SupportBeanCombinedProps.NestedLevOne(keysAndValues);
    }

    private static int[] intArray(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        JsonArray array = value.asArray();
        int[] out = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            out[i] = array.get(i).asInt();
        }
        return out;
    }

    private static Object[] rawArray(JsonValue value) {
        JsonArray array = value.asArray();
        Object[] out = new Object[array.size()];
        for (int i = 0; i < array.size(); i++) {
            out[i] = raw(array.get(i));
        }
        return out;
    }

    /** Converts a payload for a map event type: typed per schema, raw otherwise. */
    private static Map<String, Object> convertPayloadMap(Schemas schemas, Map<String, Object> schema, JsonValue value) {
        JsonObject object = value.asObject();
        Map<String, Object> out = new LinkedHashMap<>();
        for (String key : object.names()) {
            out.put(key, convert(schemas, schema.get(key), object.get(key)));
        }
        return out;
    }

    /** Converts a payload for an object-array event type, positionally typed. */
    private static Object[] convertPayloadOA(Schemas schemas, List<Object> schema, JsonValue value) {
        JsonArray array = value.asArray();
        Object[] out = new Object[schema.size()];
        for (int i = 0; i < schema.size(); i++) {
            out[i] = convert(schemas, schema.get(i), array.get(i));
        }
        return out;
    }

    /** Converts a payload value matching the pinned event type schema. */
    private static Object convert(Schemas schemas, Object typeRef, JsonValue value) {
        if (typeRef == null) {
            return raw(value);
        }
        if (typeRef instanceof String name) {
            if (name.endsWith("[]")) {
                return convertArray(schemas, name.substring(0, name.length() - 2), value);
            }
            Map<String, Object> namedMap = schemas.mapTypes.get(name);
            if (namedMap != null) {
                return convertPayloadMap(schemas, namedMap, value);
            }
            List<Object> namedOA = schemas.oaTypes.get(name);
            if (namedOA != null) {
                return convertPayloadOA(schemas, namedOA, value);
            }
            throw new IllegalStateException("unresolved named type: " + name);
        }
        if (typeRef instanceof Map<?, ?> inlineSchema) {
            return convertPayloadMap(schemas, nameMap(inlineSchema), value);
        }
        if (typeRef instanceof List<?> inlineSchema) {
            return convertPayloadOA(schemas, castList(inlineSchema), value);
        }
        Class<?> cls = (Class<?>) typeRef;
        if (cls == int.class || cls == Integer.class) {
            return value.asInt();
        }
        if (cls == int[].class) {
            return intArray(value);
        }
        if (cls == String.class) {
            return value.asString();
        }
        if (cls == Map.class) {
            return raw(value);
        }
        if (cls == SupportBean.class) {
            JsonObject object = value.asObject();
            return new SupportBean(object.getString("theString", null), object.getInt("intPrimitive", 0));
        }
        if (cls == SupportBean[].class) {
            JsonArray array = value.asArray();
            SupportBean[] out = new SupportBean[array.size()];
            for (int i = 0; i < array.size(); i++) {
                out[i] = (SupportBean) convert(schemas, SupportBean.class, array.get(i));
            }
            return out;
        }
        if (cls == SupportBeanComplexProps.class) {
            return complexProps(value.asObject());
        }
        if (cls == SupportBeanComplexProps[].class) {
            JsonArray array = value.asArray();
            SupportBeanComplexProps[] out = new SupportBeanComplexProps[array.size()];
            for (int i = 0; i < array.size(); i++) {
                out[i] = complexProps(array.get(i).asObject());
            }
            return out;
        }
        throw new IllegalStateException("unsupported type reference: " + cls);
    }

    /** Converts an array payload whose element type is a named map/object-array type. */
    private static Object[] convertArray(Schemas schemas, String elementType, JsonValue value) {
        JsonArray array = value.asArray();
        Object[] out = new Object[array.size()];
        for (int i = 0; i < array.size(); i++) {
            out[i] = convert(schemas, elementType, array.get(i));
        }
        return out;
    }

    @SuppressWarnings("unchecked")
    private static Map<String, Object> nameMap(Map<?, ?> source) {
        return (Map<String, Object>) source;
    }

    @SuppressWarnings("unchecked")
    private static List<Object> castList(List<?> source) {
        return (List<Object>) source;
    }

    /** Converts a JSON value to a raw, untyped Java value (nested maps/arrays/scalars). */
    private static Object raw(JsonValue value) {
        if (value.isNull()) {
            return null;
        }
        if (value.isObject()) {
            Map<String, Object> out = new LinkedHashMap<>();
            for (String key : value.asObject().names()) {
                out.put(key, raw(value.asObject().get(key)));
            }
            return out;
        }
        if (value.isArray()) {
            List<Object> out = new ArrayList<>();
            JsonArray array = value.asArray();
            for (int i = 0; i < array.size(); i++) {
                out.add(raw(array.get(i)));
            }
            return out;
        }
        if (value.isString()) {
            return value.asString();
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.isNumber()) {
            double number = value.asDouble();
            if (number == Math.rint(number) && !Double.isInfinite(number)) {
                return (int) number;
            }
            return number;
        }
        throw new IllegalStateException("unexpected JSON value: " + value);
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
            if (value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested nested) {
                JsonObject object = new JsonObject();
                object.add("nestedValue", normalize(nested.getNestedValue()));
                object.add("nestedNestedValue", normalize(
                    nested.getNestedNested() == null ? null : nested.getNestedNested().getNestedNestedValue()));
                return object;
            }
            if (value instanceof SupportBeanComplexProps complex) {
                JsonObject object = new JsonObject();
                object.add("simpleProperty", normalize(complex.getSimpleProperty()));
                object.add("mapProperty", normalize(complex.getMapProperty()));
                object.add("arrayProperty", normalize(complex.getArrayProperty()));
                object.add("nested", normalize(complex.getNested()));
                object.add("objectArray", normalize(complex.getObjectArray()));
                return object;
            }
            if (value instanceof SupportBeanCombinedProps.NestedLevOne levelOne) {
                JsonObject object = new JsonObject();
                object.add("mapprop", normalize(levelOne.getMapprop()));
                object.add("nestLevOneVal", normalize(levelOne.getNestLevOneVal()));
                return object;
            }
            if (value instanceof SupportBeanCombinedProps.NestedLevTwo levelTwo) {
                JsonObject object = new JsonObject();
                object.add("value", normalize(levelTwo.getValue()));
                return object;
            }
            // SupportBean implements toString deterministically
            // ("SupportBean(theString, intPrimitive)"); other bean values are
            // handled above and never reach this fallback.
            return Json.value(String.valueOf(value));
        }
    }
}
