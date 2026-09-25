import com.espertech.esper.common.client.EPCompiled;
import java.io.StringReader;
import javax.xml.parsers.DocumentBuilderFactory;
import org.w3c.dom.Document;
import org.xml.sax.InputSource;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeXMLDOM;
import com.espertech.esper.common.client.type.EPType;
import com.espertech.esper.common.client.type.EPTypeClass;
import com.espertech.esper.regressionlib.support.bean.SupportBeanDynRoot;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerImplA;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerImplB;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerImplC;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerInterface;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.EventPropertyGetter;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNested;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedDeep;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedRootedNonSimple;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicNestedRootedSimple;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyDynamicSimple;
import com.espertech.esper.common.internal.avro.core.AvroConstant;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;
import org.w3c.dom.Attr;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.w3c.dom.NodeList;

import java.io.FileReader;
import java.lang.reflect.Method;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.Assert.assertEquals;

/**
 * JSON trace recorder for the EventInfraPropertyDynamic* cluster.
 *
 * Replays deploy/listener/types/send/getter/undeploy steps for the five
 * executions on one runtime per case. Non-json "schema" deploy steps configure
 * the event type in the Configuration instead of compiling schema EPL (the
 * suite registers these types via addEventType in the test runner).
 */
public final class EventInfraPropertyDynamicScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-infra-property-dynamic";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra";

    private static final String[] CASES = {
            "dynamic-simple",
            "dynamic-nested",
            "nested-deep",
            "rooted-nonsimple",
            "rooted-simple",
            "nonsimple",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9f5b65c0125c70ea8760",
            "java-runtime-f686347226e7681f0325",
            "java-runtime-0285612baa5f5321fae2",
            "java-runtime-a1c916427bb1c85757f8",
            "java-runtime-4cd646a570fcd69e826a",
            "java-runtime-d6c08cf3d40f13126684",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventInfraPropertyDynamicSimple",
            "EventInfraPropertyDynamicNested",
            "EventInfraPropertyDynamicNestedDeep",
            "EventInfraPropertyDynamicNestedRootedNonSimple",
            "EventInfraPropertyDynamicNestedRootedSimple",
            "EventInfraPropertyDynamicNonSimple",
    };
    private static final String[] STATIC_IDS = {
            "java-1582f18f3ce77af9f4ff",
            "java-bba8d0912642e3573aeb",
            "java-90c1583c627d7e5f6219",
            "java-b13358549c04fc9cedfb",
            "java-48d6c20971992821958e",
            "java-a58ae9e754fe80b6b8da",
    };

    private static String pkg(String simpleName) {
        return "com.espertech.esper.regressionlib.suite.event.infra." + simpleName;
    }

    /** Java event-type name per (case, mode) — mirrors the suite constants. */
    private static String typeName(String caseName, String mode) {
        String suffix;
        switch (mode) {
            case "bean":
                if ("nonsimple".equals(caseName)) {
                    return SupportBeanComplexProps.class.getSimpleName();
                }
                suffix = "dynamic-simple".equals(caseName) || "rooted-simple".equals(caseName)
                        ? null : "SupportBeanDynRoot";
                if (suffix == null) {
                    return SupportMarkerInterface.class.getSimpleName();
                }
                return suffix;
            case "map": suffix = "Map"; break;
            case "objectarray": suffix = "OA"; break;
            case "xml": suffix = "XML"; break;
            case "avro": suffix = "Avro"; break;
            case "json": suffix = "Json"; break;
            case "json-provided": suffix = "JsonProvided"; break;
            default: throw new IllegalArgumentException("unknown mode " + mode);
        }
        String base;
        switch (caseName) {
            case "dynamic-simple": base = "EventInfraPropertyDynamicSimple"; break;
            case "dynamic-nested": base = "EventInfraPropertyDynamicNested"; break;
            case "nested-deep": base = "EventInfraPropertyDynamicNestedDeep"; break;
            case "rooted-nonsimple": base = "EventInfraPropertyDynamicNestedRootedNonSimple"; break;
            case "rooted-simple": base = "EventInfraPropertyDynamicNestedRootedSimple"; break;
            case "nonsimple": base = "EventInfraPropertyDynamicNonSimple"; break;
            default: throw new IllegalArgumentException("unknown case " + caseName);
        }
        return base + suffix;
    }

    /** Pinned schema-creation EPL for json/json-provided modes. */
    private static String pinnedSchemaEPL(String caseName, String mode) {
        if (!"json".equals(mode) && !"json-provided".equals(mode)) {
            return "";
        }
        switch (caseName) {
            case "dynamic-simple":
                return "@JsonSchema(dynamic=true) @public @buseventtype create json schema EventInfraPropertyDynamicSimpleJson()";
            case "dynamic-nested":
                if ("json".equals(mode)) {
                    return "@JsonSchema(dynamic=true) create json schema Undefined();\n" +
                            "@public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedJson(item Undefined)";
                }
                return "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNested$MyLocalJsonProvidedItem") + "') @public @buseventtype @name('schema') create json schema Item();\n" +
                        "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNested$MyLocalJsonProvided") + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedJsonProvided(item Item)";
            case "nested-deep":
                if ("json".equals(mode)) {
                    return "@JsonSchema(dynamic=true) create json schema Item();\n" +
                            "@public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedDeepJson(item Item)";
                }
                return "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNestedDeep$MyLocalJsonProvided") + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedDeepJsonProvided()";
            case "rooted-nonsimple":
                if ("json".equals(mode)) {
                    return "@public @buseventtype @name('schema') @JsonSchema(dynamic=true) create json schema EventInfraPropertyDynamicNestedRootedNonSimpleJson()";
                }
                return "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNestedRootedNonSimple$MyLocalJsonProvided") + "') @public @buseventtype @name('schema') @JsonSchema(dynamic=true) create json schema EventInfraPropertyDynamicNestedRootedNonSimpleJsonProvided()";
            case "rooted-simple":
                if ("json".equals(mode)) {
                    return "@JsonSchema(dynamic=true) @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedRootedSimpleJson()";
                }
                return "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNestedRootedSimple$MyLocalJsonProvided") + "') @public @buseventtype @name('schema') create json schema EventInfraPropertyDynamicNestedRootedSimpleJsonProvided()";
            case "nonsimple":
                if ("json".equals(mode)) {
                    return "@JsonSchema(dynamic=true) @public @buseventtype create json schema EventInfraPropertyDynamicNonSimpleJson()";
                }
                return "@JsonSchema(className='" + pkg("EventInfraPropertyDynamicNonSimple$MyLocalJsonProvided") + "') @public @buseventtype create json schema EventInfraPropertyDynamicNonSimpleJsonProvided()";
        }
        return null;
    }

    /** Pinned s0 statement text per (case, mode, rep) — mirrors eipdS0EPL. */
    private static String pinnedS0(String caseName, String mode, String rep) {
        String t = typeName(caseName, mode);
        switch (caseName) {
            case "dynamic-simple":
                return "@name('s0') select id? as myid, exists(id?) as exists_myid from " + t;
            case "dynamic-nested": {
                String annotation = "";
                switch (rep) {
                    case "objectarray": annotation = "@EventRepresentation('objectarray')"; break;
                    case "map": annotation = "@EventRepresentation('map')"; break;
                    case "avro":
                        annotation = "@EventRepresentation('avro')@AvroSchemaField(name='myid',schema='[\"int\",{\"type\":\"string\",\"avro.java.string\":\"String\"},\"null\"]')";
                        break;
                    case "json": annotation = "@EventRepresentation('json')"; break;
                }
                return "@name('s0') " + annotation + " select " +
                        "item.id? as myid, " +
                        "exists(item.id?) as exists_myid " +
                        "from " + t + ";\n" +
                        "@name('s1') select * from " + t + ";\n";
            }
            case "nested-deep":
                return "@name('s0') select " +
                        " item.nested?.nestedValue as n1, " +
                        " exists(item.nested?.nestedValue) as exists_n1, " +
                        " item.nested?.nestedValue? as n2, " +
                        " exists(item.nested?.nestedValue?) as exists_n2, " +
                        " item.nested?.nestedNested.nestedNestedValue as n3, " +
                        " exists(item.nested?.nestedNested.nestedNestedValue) as exists_n3, " +
                        " item.nested?.nestedNested?.nestedNestedValue as n4, " +
                        " exists(item.nested?.nestedNested?.nestedNestedValue) as exists_n4, " +
                        " item.nested?.nestedNested.nestedNestedValue? as n5, " +
                        " exists(item.nested?.nestedNested.nestedNestedValue?) as exists_n5, " +
                        " item.nested?.nestedNested?.nestedNestedValue? as n6, " +
                        " exists(item.nested?.nestedNested?.nestedNestedValue?) as exists_n6 " +
                        " from " + t;
            case "rooted-nonsimple":
                return "@name('s0') select " +
                        "item?.indexed[0] as indexed1, " +
                        "exists(item?.indexed[0]) as exists_indexed1, " +
                        "item?.indexed[1]? as indexed2, " +
                        "exists(item?.indexed[1]?) as exists_indexed2, " +
                        "item?.arrayProperty[1]? as array, " +
                        "exists(item?.arrayProperty[1]?) as exists_array, " +
                        "item?.mapped('keyOne') as mapped1, " +
                        "exists(item?.mapped('keyOne')) as exists_mapped1, " +
                        "item?.mapped('keyTwo')? as mapped2,  " +
                        "exists(item?.mapped('keyTwo')?) as exists_mapped2,  " +
                        "item?.mapProperty('xOne')? as map, " +
                        "exists(item?.mapProperty('xOne')?) as exists_map " +
                        " from " + t;
            case "rooted-simple":
                return "@name('s0') select " +
                        "simpleProperty? as simple, " +
                        "exists(simpleProperty?) as exists_simple, " +
                        "nested?.nestedValue as nested, " +
                        "exists(nested?.nestedValue) as exists_nested, " +
                        "nested?.nestedNested.nestedNestedValue as nestedNested, " +
                        "exists(nested?.nestedNested.nestedNestedValue) as exists_nestedNested " +
                        "from " + t;
            case "nonsimple":
                return "@name('s0') select " +
                        "indexed[0]? as indexed1, " +
                        "exists(indexed[0]?) as exists_indexed1, " +
                        "indexed[1]? as indexed2, " +
                        "exists(indexed[1]?) as exists_indexed2, " +
                        "mapped('keyOne')? as mapped1, " +
                        "exists(mapped('keyOne')?) as exists_mapped1, " +
                        "mapped('keyTwo')? as mapped2,  " +
                        "exists(mapped('keyTwo')?) as exists_mapped2  " +
                        "from " + t;
        }
        return null;
    }

    /** Pinned second-pass select-* statement (nested-deep only). */
    private static String pinnedSelectAll(String caseName, String mode) {
        if (!"nested-deep".equals(caseName)) {
            return null;
        }
        return "@name('s0') select * from " + typeName(caseName, mode);
    }

    private static final String EMPTY_XSD = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
            "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
            "\t<xs:element name=\"myevent\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "</xs:schema>\n";

    private static final String ITEM_XSD = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
            "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
            "\t<xs:element name=\"myevent\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:item\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"item\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "</xs:schema>\n";

    /**
     * Registers the (case, mode) event type in the Configuration — mirrors
     * TestSuiteEventInfra's configure* blocks for these five executions.
     */
    private static void registerType(Configuration config, String caseName, String mode) {
        String name = typeName(caseName, mode);
        switch (mode) {
            case "bean":
                if ("nonsimple".equals(caseName)) {
                    config.getCommon().addEventType(name, SupportBeanComplexProps.class);
                } else if ("dynamic-simple".equals(caseName) || "rooted-simple".equals(caseName)) {
                    config.getCommon().addEventType(name, SupportMarkerInterface.class);
                } else {
                    config.getCommon().addEventType(name, SupportBeanDynRoot.class);
                }
                return;
            case "map":
                if ("dynamic-simple".equals(caseName) || "rooted-simple".equals(caseName)
                        || "rooted-nonsimple".equals(caseName) || "nonsimple".equals(caseName)) {
                    config.getCommon().addEventType(name, Collections.emptyMap());
                } else {
                    Map<String, Object> top = Collections.singletonMap("item", Map.class);
                    config.getCommon().addEventType(name, top);
                }
                return;
            case "objectarray":
                registerObjectArrayTypes(config, caseName, name);
                return;
            case "xml": {
                ConfigurationCommonEventTypeXMLDOM meta = new ConfigurationCommonEventTypeXMLDOM();
                meta.setRootElementName("myevent");
                boolean itemSchema = "dynamic-nested".equals(caseName) || "nested-deep".equals(caseName);
                meta.setSchemaText(itemSchema ? ITEM_XSD : EMPTY_XSD);
                config.getCommon().addEventType(name, meta);
                return;
            }
            case "avro":
                registerAvroTypes(config, caseName, name);
                return;
            default:
                // json/json-provided deploy via schema EPL instead.
                return;
        }
    }

    private static void registerObjectArrayTypes(Configuration config, String caseName, String name) {
        switch (caseName) {
            case "dynamic-simple":
                config.getCommon().addEventType(name,
                        new String[]{"somefield", "id"}, new Object[]{Object.class, Object.class});
                return;
            case "dynamic-nested":
                config.getCommon().addEventType(name,
                        new String[]{"item"}, new Object[]{Object.class});
                return;
            case "nested-deep": {
                String type3 = name + "_3";
                config.getCommon().addEventType(type3, new String[]{"nestedNestedValue"}, new Object[]{Object.class});
                String type2 = name + "_2";
                config.getCommon().addEventType(type2, new String[]{"nestedNested", "nestedValue"}, new Object[]{type3, Object.class});
                String type1 = name + "_1";
                config.getCommon().addEventType(type1, new String[]{"nested"}, new Object[]{type2});
                config.getCommon().addEventType(name, new String[]{"item"}, new Object[]{type1});
                return;
            }
            case "rooted-nonsimple": {
                String nestedName = name + "_1";
                config.getCommon().addEventType(nestedName,
                        new String[]{"indexed", "mapped", "arrayProperty", "mapProperty"},
                        new Object[]{int[].class, Map.class, int[].class, Map.class});
                config.getCommon().addEventType(name,
                        new String[]{"someprop", "item"}, new Object[]{String.class, nestedName});
                return;
            }
            case "rooted-simple": {
                String type2 = name + "_2";
                config.getCommon().addEventType(type2, new String[]{"nestedNestedValue"}, new Object[]{Object.class});
                String type1 = name + "_1";
                config.getCommon().addEventType(type1, new String[]{"nestedValue", "nestedNested"}, new Object[]{Object.class, type2});
                config.getCommon().addEventType(name,
                        new String[]{"simpleProperty", "nested"}, new Object[]{Object.class, type1});
                return;
            }
            case "nonsimple":
                config.getCommon().addEventType(name,
                        new String[]{"indexed", "mapped"}, new Object[]{int[].class, Map.class});
                return;
        }
    }

    private static void registerAvroTypes(Configuration config, String caseName, String name) {
        Schema schema;
        switch (caseName) {
            case "dynamic-simple":
                schema = SchemaBuilder.record(name).fields()
                        .name("id").type().unionOf()
                        .nullType().and().intType().and().booleanType().endUnion().noDefault()
                        .endRecord();
                break;
            case "dynamic-nested": {
                Schema s1 = SchemaBuilder.record(name + "_1").fields()
                        .name("id").type().unionOf()
                        .intBuilder().endInt()
                        .and().stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE).endString()
                        .and().nullType()
                        .endUnion().noDefault()
                        .endRecord();
                schema = SchemaBuilder.record(name).fields().name("item").type(s1).noDefault().endRecord();
                break;
            }
            case "nested-deep": {
                Schema s3 = SchemaBuilder.record(name + "_3").fields().optionalInt("nestedNestedValue").endRecord();
                Schema s2 = SchemaBuilder.record(name + "_2").fields()
                        .optionalInt("nestedValue")
                        .name("nestedNested").type().unionOf().intType().and().type(s3).endUnion().noDefault()
                        .endRecord();
                Schema s1 = SchemaBuilder.record(name + "_1").fields()
                        .name("nested").type().unionOf().intType().and().type(s2).endUnion().noDefault()
                        .endRecord();
                schema = SchemaBuilder.record(name).fields().name("item").type(s1).noDefault().endRecord();
                break;
            }
            case "rooted-nonsimple": {
                Schema s1 = SchemaBuilder.record(name + "_1").fields()
                        .name("indexed").type().unionOf().nullType().and().intType().and().array().items().intType().endUnion().noDefault()
                        .name("mapped").type().unionOf().nullType().and().intType().and().map().values().intType().endUnion().noDefault()
                        .name("arrayProperty").type().unionOf().nullType().and().intType().and().array().items().intType().endUnion().noDefault()
                        .name("mapProperty").type().unionOf().nullType().and().intType().and().map().values().intType().endUnion().noDefault()
                        .endRecord();
                schema = SchemaBuilder.record(name).fields()
                        .name("item").type().unionOf().intType().and().type(s1).endUnion().noDefault()
                        .endRecord();
                break;
            }
            case "nonsimple":
                schema = SchemaBuilder.record(name).fields()
                        .name("indexed").type().unionOf().nullType().and().intType().and().array().items().intType().endUnion().noDefault()
                        .name("mapped").type().unionOf().nullType().and().intType().and().map().values().intType().endUnion().noDefault()
                        .endRecord();
                break;
            case "rooted-simple": {
                Schema s3 = SchemaBuilder.record(name + "_3").fields().optionalInt("nestedNestedValue").endRecord();
                Schema s2 = SchemaBuilder.record(name + "_2").fields()
                        .optionalInt("nestedValue")
                        .name("nestedNested").type().unionOf().intType().and().type(s3).endUnion().noDefault()
                        .endRecord();
                schema = SchemaBuilder.record(name).fields()
                        .name("simpleProperty").type().unionOf().intType().and().stringType().endUnion().noDefault()
                        .name("nested").type().unionOf().intType().and().type(s2).endUnion().noDefault()
                        .endRecord();
                break;
            }
            default:
                throw new IllegalArgumentException("no avro schema for " + caseName);
        }
        config.getCommon().addEventTypeAvro(name, new com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro(schema));
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EventInfraPropertyDynamicScenarioOracle <scenario.json>");
        }
        JsonObject scenario;
        try (FileReader reader = new FileReader(args[0])) {
            scenario = Json.parse(reader).asObject();
        }
        validateMetadata(scenario);
        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            replayCase(scenario, caseName, records);
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
                    || definition.getInt("ordinal", -1) != 0
                    || !RUNTIME_IDS[index].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[index].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }
    }

    /**
     * Replays one execution's scenario steps on a single runtime; undeploy-all
     * between iterations mirrors env.undeployAll() and sequence counters
     * restart per iteration like the Go runner's fresh engine.
     */
    private static void replayCase(JsonObject scenario, String caseName, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        config.getCommon().getEventMeta().setEnableXMLXSD(true);
        registerAllTypes(config, scenario, caseName);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            Map<String, EPDeployment> deployments = new HashMap<>();
            Map<String, EPCompiled> schemaCompiled = new HashMap<>();
            CaseState state = new CaseState();
            state.records = records;
            state.sequences = sequences;
            state.runtime = runtime;
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
                    case "deploy": {
                        String label = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        String mode = step.getString("mode", "");

                        String rep = step.getString("name", "");
                        if ("schema".equals(label)) {
                            String pinned = pinnedSchemaEPL(caseName, mode);
                            if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("schema EPL is not pinned for " + caseName + "/" + mode);
                            }
                            String expectedType = typeName(caseName, mode);
                            if (!expectedType.equals(step.getString("eventType", ""))) {
                                throw new IllegalStateException("schema eventType is not pinned for " + caseName + "/" + mode);
                            }
                            if (epl.isEmpty()) {
                                // Non-json modes: type registered via the
                                // configuration like the suite runner does.
                                registerType(config, caseName, mode);
                            } else {
                                EPCompiled compiled = EPCompilerProvider.getCompiler()
                                        .compile(epl, new CompilerArguments(config));
                                schemaCompiled.put("schema", compiled);
                                runtime.getDeploymentService().deploy(compiled, new com.espertech.esper.runtime.client.DeploymentOptions());
                            }
                            state.mode = mode;
                            state.rep = rep;
                        } else {
                            String pinned = pinnedS0(caseName, mode, rep);
                            String selectAll = pinnedSelectAll(caseName, mode);
                            if (selectAll != null && selectAll.equals(epl)) {
                                // nested-deep second pass redeploys select *.
                            } else if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("deploy " + label + " EPL is not pinned in " + caseName + "/" + mode + "/" + rep);
                            }
                            CompilerArguments args = new CompilerArguments(config);
                            if (schemaCompiled.containsKey("schema")) {
                                args.getPath().add(schemaCompiled.get("schema"));
                            }
                            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, args);
                            EPDeployment deployment = runtime.getDeploymentService()
                                    .deploy(compiled, new com.espertech.esper.runtime.client.DeploymentOptions());
                            deployments.put(label, deployment);
                            for (EPStatement statement : deployment.getStatements()) {
                                statement.addListener(listener(caseName, state));
                            }
                        }
                        break;
                    }
                    case "deployed": {
                        String label = step.getString("statement", "");
                        int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "types": {
                        EPDeployment s0 = deployments.get("s0");
                        EventType eventType = runtime.getDeploymentService()
                                .getStatement(s0.getDeploymentId(), "s0").getEventType();
                        JsonArray order = step.get("propertyOrder").asArray();
                        JsonObject pins = step.get("propertyTypes").asObject();
                        for (JsonValue nameValue : order) {
                            String propName = nameValue.asString();
                            Class<?> propertyType = eventType.getPropertyType(propName);
                            if (propertyType == null) {
                                throw new IllegalStateException("no property type for " + propName);
                            }
                            String expected = pins.getString(propName, "");
                            String actual = propertyType.getSimpleName();
                            assertEquals(expected, actual);
                            int sequence = sequences.merge("s0:types", 1, Integer::sum);
                            JsonObject record = new JsonObject();
                            record.add("case", caseName);
                            record.add("operation", "types");
                            record.add("statement", "s0");
                            record.add("sequence", sequence);
                            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                            record.add("name", propName);
                            record.add("value", actual);
                            records.add(record);
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, caseName, state.mode,
                                step.getString("eventType", ""), step.get("payload").asObject());
                        emitGetterRecord(caseName, state);
                        break;
                    case "undeploy": {
                        String label = step.getString("statement", "");
                        runtime.getDeploymentService()
                                .undeploy(deployments.remove(label).getDeploymentId());
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
                        schemaCompiled.clear();
                        state.lastS1 = null;
                        sequences.clear();
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
     * Common-config event types resolve at runtime creation, so every
     * non-json underlying for the case is registered up front (the suite
     * registers them at session level too). Deploy "schema" steps then only
     * pin metadata for non-json modes.
     */
    private static void registerAllTypes(Configuration config, JsonObject scenario, String caseName) {
        boolean inCase = false;
        for (JsonValue stepValue : scenario.get("steps").asArray()) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(step.getString("op", ""))) {
                inCase = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!inCase) {
                continue;
            }
            if ("deploy".equals(step.getString("op", ""))
                    && "schema".equals(step.getString("statement", ""))) {
                registerType(config, caseName, step.getString("mode", ""));
            }
        }
    }

    private static final class CaseState {
        JsonArray records;
        Map<String, Integer> sequences;
        EPRuntime runtime;
        EventBean lastS1;
        String mode;
        String rep;
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; captures the latest s1 event for the getter probe.
     */
    private static UpdateListener listener(String caseName, CaseState state) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = state.sequences.merge(statement.getName(), 1, Integer::sum);
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
                    Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            state.records.add(record);
            if ("s1".equals(statement.getName()) && newEvents != null && newEvents.length > 0) {
                state.lastS1 = newEvents[newEvents.length - 1];
            }
        };
    }

    /**
     * Emits the dynamic-nested getter record: Java probes the raw item.id?
     * getter on the last s1 (select-*) event after each send; XML values are
     * nodes, so the value renders the node's text content.
     */
    private static void emitGetterRecord(String caseName, CaseState state) {
        if (!"dynamic-nested".equals(caseName)) {
            return;
        }
        EventBean event = state.lastS1;
        if (event == null) {
            throw new IllegalStateException("no s1 event captured for getter probe");
        }
        EventPropertyGetter getter = event.getEventType().getGetter("item.id?");
        if (getter == null) {
            throw new IllegalStateException("item.id? getter missing on select-* event");
        }
        boolean exists = getter.isExistsProperty(event);
        Object value = getter.get(event);
        JsonObject probe = new JsonObject();
        probe.add("exists", exists);
        if (exists) {
            if (value instanceof Node) {
                probe.add("value", normalizeNodeText((Node) value));
            } else {
                probe.add("value", normalize(value));
            }
        }
        int sequence = state.sequences.merge("s1:getter", 1, Integer::sum);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "getter");
        record.add("statement", "s1");
        record.add("sequence", sequence);
        record.add("time",
                Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
        record.add("name", "item.id?");
        record.add("value", probe);
        state.records.add(record);
    }

    /** XML node probe values render their text content like xmlToValue. */
    private static JsonValue normalizeNodeText(Node node) {
        if (node instanceof Attr) {
            return Json.value(((Attr) node).getValue());
        }
        return Json.value(node.getTextContent());
    }

    /** sendEvent rebuilds the Java sender for the (case, mode, payload) pin. */
    private static void sendEvent(EPRuntime runtime, String caseName, String mode,
                                  String eventType, JsonObject payload) throws Exception {
        switch (mode) {
            case "bean":
                sendBean(runtime, caseName, eventType, payload);
                return;
            case "map":
                runtime.getEventService().sendEventMap(toMap(payload), eventType);
                return;
            case "objectarray":
                JsonValue values = payload.get("values");
                if (values != null && values.isArray()) {
                    runtime.getEventService().sendEventObjectArray(toCells(values.asArray()), eventType);
                    return;
                }
                if ("dynamic-nested".equals(caseName)
                        && "oa-s0-101".equals(payload.getString("variant", ""))) {
                    runtime.getEventService().sendEventObjectArray(
                            new Object[]{new SupportBean_S0(101)}, eventType);
                    return;
                }
                throw new IllegalArgumentException("no objectarray sender for " + caseName + " payload " + payload);
            case "xml":
                sendXml(runtime, eventType, payload.getString("xml", ""));
                return;
            case "avro":
                sendAvro(runtime, caseName, eventType, payload);
                return;
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson(payload.toString(), eventType);
                return;
            default:
                throw new IllegalArgumentException("unknown send mode " + mode);
        }
    }

    /** FXML-equivalent sender: fragments are wrapped in <myevent>. */
    private static void sendXml(EPRuntime runtime, String eventType, String fragment) throws Exception {
        String xml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                "<myevent>\n" +
                "  " + fragment + "\n" +
                "</myevent>\n";
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.setNamespaceAware(true);
        Document document = factory.newDocumentBuilder().parse(new InputSource(new StringReader(xml)));
        runtime.getEventService().sendEventXMLDOM(document, eventType);
    }

    private static void sendBean(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        String variant = payload.getString("variant", "");
        switch (caseName) {
            case "dynamic-simple":
                switch (variant) {
                    case "impl-a":
                        runtime.getEventService().sendEventBean(new SupportMarkerImplA("e1"), eventType);
                        return;
                    case "impl-b":
                        runtime.getEventService().sendEventBean(new SupportMarkerImplB(1), eventType);
                        return;
                    case "impl-c":
                        runtime.getEventService().sendEventBean(new SupportMarkerImplC(), eventType);
                        return;
                }
                break;
            case "dynamic-nested":
                switch (variant) {
                    case "s0-101":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot(new SupportBean_S0(101)), eventType);
                        return;
                    case "str-abc":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot("abc"), eventType);
                        return;
                    case "a-e1":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot(new SupportBean_A("e1")), eventType);
                        return;
                    case "b-e2":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot(new SupportBean_B("e2")), eventType);
                        return;
                    case "s1-102":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot(new SupportBean_S1(102)), eventType);
                        return;
                }
                break;
            case "nonsimple":
                if ("complex-default".equals(variant)) {
                    runtime.getEventService().sendEventBean(
                            SupportBeanComplexProps.makeDefaultBean(), eventType);
                    return;
                }
                break;
            case "nested-deep":
                switch (variant) {
                    case "complex-default":
                        runtime.getEventService().sendEventBean(
                                new SupportBeanDynRoot(SupportBeanComplexProps.makeDefaultBean()), eventType);
                        return;
                    case "complex-mutated": {
                        SupportBeanComplexProps bean = SupportBeanComplexProps.makeDefaultBean();
                        bean.getNested().setNestedValue("nested1");
                        bean.getNested().getNestedNested().setNestedNestedValue("nested2");
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot(bean), eventType);
                        return;
                    }
                    case "str-abc":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot("abc"), eventType);
                        return;
                }
                break;
            case "rooted-nonsimple":
                switch (variant) {
                    case "str-xxx":
                        runtime.getEventService().sendEventBean(new SupportBeanDynRoot("xxx"), eventType);
                        return;
                    case "complex-default":
                        runtime.getEventService().sendEventBean(
                                new SupportBeanDynRoot(SupportBeanComplexProps.makeDefaultBean()), eventType);
                        return;
                }
                break;
            case "rooted-simple":
                switch (variant) {
                    case "complex-default":
                        runtime.getEventService().sendEventBean(SupportBeanComplexProps.makeDefaultBean(), eventType);
                        return;
                    case "impl-a-x":
                        runtime.getEventService().sendEventBean(new SupportMarkerImplA("x"), eventType);
                        return;
                }
                break;
        }
        throw new IllegalArgumentException("no bean sender for " + caseName + " variant " + variant);
    }

    /**
     * Decodes the objectarray "values" payload; {"values":[…]} cells unwrap to
     * nested Object[] rows matching the Go runner's eipdUnwrapOACell.
     */
    private static Object[] toCells(JsonArray values) {
        Object[] cells = new Object[values.size()];
        for (int i = 0; i < values.size(); i++) {
            cells[i] = toCell(values.get(i));
        }
        return cells;
    }

    private static Object toCell(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isArray()) {
            return toCells(value.asArray());
        }
        if (value.isObject()) {
            JsonObject object = value.asObject();
            JsonValue inner = object.get("values");
            if (inner != null && inner.isArray()) {
                return toCells(inner.asArray());
            }
            return toMap(object);
        }
        if (value.isString()) {
            return value.asString();
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.isNumber()) {
            return (int) value.asDouble();
        }
        return value.toString();
    }

    private static void sendAvro(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        Schema schema = SupportAvroUtil.getAvroSchema(
                runtime.getEventTypeService().getEventTypePreconfigured(eventType));
        GenericData.Record record = (GenericData.Record) toAvroRecord(schema, payload);
        runtime.getEventService().sendEventAvro(record, eventType);
    }

    /**
     * Rebuilds Avro values by navigating the registered schema like the suite
     * does with runtimeAvroSchemaPreconfigured + findUnionRecordSchemaSingle:
     * object values map onto the union's record branch, arrays and scalars
     * pass through as lists/scalars.
     */
    private static Object toAvroValue(Schema fieldSchema, JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isObject()) {
            Schema target = branchOf(fieldSchema, Schema.Type.RECORD);
            if (target != null) {
                return toAvroRecord(target, value.asObject());
            }
            // Map-typed fields (mapped/mapProperty) stay plain maps.
            Map<String, Object> map = new LinkedHashMap<>();
            for (String name : value.asObject().names()) {
                map.put(name, toAvroScalarOrNested(value.asObject().get(name)));
            }
            return map;
        }
        if (value.isArray()) {
            JsonArray array = value.asArray();
            List<Object> items = new ArrayList<>();
            for (JsonValue item : array) {
                items.add(toAvroScalar(item));
            }
            return items;
        }
        return toAvroScalar(value);
    }

    private static GenericData.Record toAvroRecord(Schema schema, JsonObject object) {
        GenericData.Record record = new GenericData.Record(schema);
        for (String fieldName : object.names()) {
            Schema.Field field = schema.getField(fieldName);
            if (field == null) {
                throw new IllegalArgumentException("no avro field " + fieldName + " in " + schema.getFullName());
            }
            record.put(fieldName, toAvroValue(field.schema(), object.get(fieldName)));
        }
        return record;
    }

    /** Returns the union branch of the requested type, or null. */
    private static Schema branchOf(Schema schema, Schema.Type type) {
        if (schema.getType() == Schema.Type.UNION) {
            for (Schema branch : schema.getTypes()) {
                if (branch.getType() == type) {
                    return branch;
                }
            }
            return null;
        }
        return schema.getType() == type ? schema : null;
    }

    /** Map values may nest arrays/scalars but never records in this scenario. */
    private static Object toAvroScalarOrNested(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isArray()) {
            List<Object> items = new ArrayList<>();
            for (JsonValue item : value.asArray()) {
                items.add(toAvroScalar(item));
            }
            return items;
        }
        if (value.isObject()) {
            Map<String, Object> map = new LinkedHashMap<>();
            for (String name : value.asObject().names()) {
                map.put(name, toAvroScalarOrNested(value.asObject().get(name)));
            }
            return map;
        }
        return toAvroScalar(value);
    }

    private static Object toAvroScalar(JsonValue value) {
        if (value.isString()) {
            return value.asString();
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.isNumber()) {
            return (int) value.asDouble();
        }
        return null;
    }

    private static Map<String, Object> toMap(JsonObject payload) {
        Map<String, Object> event = new LinkedHashMap<>();
        for (String name : payload.names()) {
            event.put(name, toJava(payload.get(name)));
        }
        return event;
    }

    private static Object toJava(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isString()) {
            return value.asString();
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.isNumber()) {
            return (int) value.asDouble();
        }
        if (value.isArray()) {
            JsonArray array = value.asArray();
            Object[] items = new Object[array.size()];
            for (int i = 0; i < array.size(); i++) {
                items[i] = toJava(array.get(i));
            }
            return items;
        }
        if (value.isObject()) {
            return toMap(value.asObject());
        }
        return value.toString();
    }

    /** Canonical row rendering with sorted property names. */
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
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * strings/numbers/booleans passthrough, XML nodes via nodeToJson (matching
     * the Go myevent document model), beans via beanToJson (sorted getter-
     * derived properties), EventBean fragments as plain objects, Avro records
     * via their field map, maps as plain objects, arrays as JSON arrays.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return beanFragmentToJson((EventBean) value);
        }
        if (value instanceof Node) {
            return nodeToJson((Node) value);
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
        if (value instanceof CharSequence || value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        if (value instanceof GenericData.Record) {
            GenericData.Record record = (GenericData.Record) value;
            JsonObject object = new JsonObject();
            List<String> names = new ArrayList<>();
            for (Schema.Field field : record.getSchema().getFields()) {
                names.add(field.name());
            }
            Collections.sort(names);
            for (String name : names) {
                object.add(name, normalizeNested(record.get(name)));
            }
            return object;
        }
        if (value instanceof Map<?, ?> map) {
            JsonObject object = new JsonObject();
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                object.add(key, normalizeNested(map.get(key)));
            }
            return object;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object entry : (Collection<?>) value) {
                array.add(normalizeNested(entry));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = java.lang.reflect.Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalizeNested(java.lang.reflect.Array.get(value, index)));
            }
            return array;
        }
        return beanToJson(value);
    }

    /**
     * Nested values (inside maps, records, arrays and beans) render a plain
     * JSON null while top-level row fields and probe values keep the tagged
     * {"state":"null"} object, matching the Go normalizer.
     */
    private static JsonValue normalizeNested(Object value) {
        if (value == null) {
            return Json.NULL;
        }
        return normalize(value);
    }

    /**
     * Renders an XML node into the Go document model: elements with no
     * attributes and no element children render as trimmed text; attributes
     * become "@name" keys; repeated children collapse to arrays; non-empty
     * mixed text lands under "#text".
     */
    private static JsonValue nodeToJson(Node node) {
        if (node instanceof Attr) {
            return Json.value(((Attr) node).getValue());
        }
        if (!(node instanceof Element)) {
            return Json.value(node.getTextContent());
        }
        Element element = (Element) node;
        JsonObject object = new JsonObject();
        var attributes = element.getAttributes();
        for (int i = 0; i < attributes.getLength(); i++) {
            Attr attr = (Attr) attributes.item(i);
            object.add("@" + attr.getName(), attr.getValue());
        }
        Map<String, List<Node>> children = new LinkedHashMap<>();
        StringBuilder text = new StringBuilder();
        NodeList nodes = element.getChildNodes();
        for (int i = 0; i < nodes.getLength(); i++) {
            Node child = nodes.item(i);
            if (child instanceof Element) {
                children.computeIfAbsent(((Element) child).getTagName(), k -> new ArrayList<>()).add(child);
            } else if (child.getNodeType() == Node.TEXT_NODE
                    || child.getNodeType() == Node.CDATA_SECTION_NODE) {
                text.append(child.getTextContent());
            }
        }
        String trimmed = text.toString().trim();
        if (attributes.getLength() == 0 && children.isEmpty()) {
            return Json.value(trimmed);
        }
        for (Map.Entry<String, List<Node>> entry : children.entrySet()) {
            List<Node> items = entry.getValue();
            if (items.size() == 1) {
                object.add(entry.getKey(), nodeToJsonNested(items.get(0)));
            } else {
                JsonArray array = new JsonArray();
                for (Node item : items) {
                    array.add(nodeToJsonNested(item));
                }
                object.add(entry.getKey(), array);
            }
        }
        if (!trimmed.isEmpty()) {
            object.add("#text", trimmed);
        }
        return object;
    }

    private static JsonValue nodeToJsonNested(Node node) {
        return nodeToJson(node);
    }

    /** Nested EventBean values render as plain property objects. */
    private static JsonValue beanFragmentToJson(EventBean event) {
        JsonObject object = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            object.add(name, normalizeNested(event.get(name)));
        }
        return object;
    }

    /** Renders a Java bean via its public getters (sorted property names). */
    private static JsonValue beanToJson(Object bean) {
        JsonObject object = new JsonObject();
        List<String> names = new ArrayList<>();
        Map<String, Method> getters = new HashMap<>();
        for (Method method : bean.getClass().getMethods()) {
            if (method.getParameterCount() != 0 || method.getReturnType() == void.class) {
                continue;
            }
            String name = null;
            if (method.getName().startsWith("get") && method.getName().length() > 3) {
                name = Character.toLowerCase(method.getName().charAt(3)) + method.getName().substring(4);
            } else if (method.getName().startsWith("is") && method.getName().length() > 2) {
                name = Character.toLowerCase(method.getName().charAt(2)) + method.getName().substring(3);
            }
            if (name != null && !"class".equals(name)) {
                getters.put(name, method);
                names.add(name);
            }
        }
        for (java.lang.reflect.Field field : bean.getClass().getFields()) {
            if (java.lang.reflect.Modifier.isStatic(field.getModifiers())) {
                continue;
            }
            if (!getters.containsKey(field.getName())) {
                names.add(field.getName());
            }
        }
        Collections.sort(names);
        for (String name : names) {
            try {
                Method getter = getters.get(name);
                Object fieldValue = getter != null ? getter.invoke(bean)
                        : bean.getClass().getField(name).get(bean);
                object.add(name, normalizeNested(fieldValue));
            } catch (Exception e) {
                throw new IllegalStateException("bean render failed for " + name, e);
            }
        }
        return object;
    }
}
