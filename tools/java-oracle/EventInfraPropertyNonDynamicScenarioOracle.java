import com.espertech.esper.common.client.EPCompiled;
import java.io.StringReader;
import javax.xml.parsers.DocumentBuilderFactory;
import org.w3c.dom.Document;
import org.xml.sax.InputSource;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyGetter;
import com.espertech.esper.common.client.EventPropertyGetterIndexed;
import com.espertech.esper.common.client.EventPropertyGetterMapped;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeXMLDOM;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.avro.core.AvroConstant;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyIndexedKeyExpr;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyIndexedRuntimeIndex;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyMappedIndexed;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyMappedRuntimeKey;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyNestedIndexed;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyNestedNestedEscaped;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraPropertyNestedSimple;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.common.internal.util.JavaClassHelper;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;
import org.w3c.dom.Attr;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.w3c.dom.NodeList;

import java.io.FileReader;
import java.lang.reflect.Method;
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
 * JSON trace recorder for the EventInfraProperty* non-dynamic cluster.
 *
 * Replays deploy/deployed/types/send/undeploy/undeploy-all steps for the
 * seven executions on one runtime per case. Non-EPL "schema" deploy steps
 * configure the event type in the Configuration instead (the suite
 * registers these types via addEventType in the test runner); EPL-bearing
 * schema deploys compile through the compiler with path access to prior
 * schema modules.
 */
public final class EventInfraPropertyNonDynamicScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-infra-property-non-dynamic";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra";

    private static final String[] CASES = {
            "indexed-key-expr",
            "indexed-runtime-index",
            "mapped-indexed",
            "mapped-runtime-key",
            "nested-indexed",
            "nested-escaped",
            "nested-simple",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f5d82c0cf0cf2516d0ef",
            "java-runtime-10ce593e8d5ae6a9dd89",
            "java-runtime-b25d2dc5c7eb2c968105",
            "java-runtime-c4f7fe6163eff7d35461",
            "java-runtime-594a57ed26499f1ccd30",
            "java-runtime-0b17042e1ee4d75b7007",
            "java-runtime-848063976a9d6485ec20",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventInfraPropertyIndexedKeyExpr",
            "EventInfraPropertyIndexedRuntimeIndex",
            "EventInfraPropertyMappedIndexed",
            "EventInfraPropertyMappedRuntimeKey",
            "EventInfraPropertyNestedIndexed",
            "EventInfraPropertyNestedNestedEscaped",
            "EventInfraPropertyNestedSimple",
    };
    private static final String[] STATIC_IDS = {
            "java-461ebcf502cbdf9ba15c",
            "java-ed0cc8c7b0867c0cd502",
            "java-287c4bf86e9464e7b595",
            "java-899877f666cfc36adcb1",
            "java-ff678fbbd673309b12b2",
            "java-2d629e45d1948a6770ee",
            "java-eb62cc47423e42ec7671",
    };

    private static String pkg(String simpleName) {
        return "com.espertech.esper.regressionlib.suite.event.infra." + simpleName;
    }

    /** Java event-type name per (case, mode) — mirrors the suite constants. */
    private static String typeName(String caseName, String mode) {
        switch (caseName) {
            case "indexed-key-expr":
                switch (mode) {
                    case "objectarray": return "OAEvent";
                    case "map": return "MapEvent";
                    case "wrapper": return "SupportBean";
                    case "bean": return "MyIndexMappedSamplerBean";
                    case "json":
                    case "json-provided": return "JsonSchema";
                }
                break;
            case "indexed-runtime-index":
            case "mapped-runtime-key":
                return "LocalEvent";
            case "mapped-indexed":
                switch (mode) {
                    case "bean": return "MyIMEvent";
                    case "map": return "EventInfraPropertyMappedIndexedMap";
                    case "objectarray": return "EventInfraPropertyMappedIndexedOA";
                    case "avro": return "EventInfraPropertyMappedIndexedAvro";
                    case "json": return "EventInfraPropertyMappedIndexedJson";
                    case "json-provided": return "EventInfraPropertyMappedIndexedJsonProvided";
                }
                break;
            case "nested-indexed":
                switch (mode) {
                    case "bean": return "InfraNestedIndexPropTop";
                    case "map": return "EventInfraPropertyNestedIndexedMap";
                    case "objectarray": return "EventInfraPropertyNestedIndexedOA";
                    case "xml": return "EventInfraPropertyNestedIndexedXML";
                    case "avro": return "EventInfraPropertyNestedIndexedAvro";
                    case "json": return "EventInfraPropertyNestedIndexedJson";
                    case "json-provided": return "EventInfraPropertyNestedIndexedJsonProvided";
                }
                break;
            case "nested-escaped":
                switch (mode) {
                    case "bean": return "BeanLvl0";
                    case "map": return "MapLvl0";
                    case "objectarray": return "OALvl0";
                    case "json": return "JSONLvl0";
                    case "avro": return "AvroLvl0";
                }
                break;
            case "nested-simple":
                switch (mode) {
                    case "bean": return "InfraNestedSimplePropTop";
                    case "map": return "EventInfraPropertyNestedSimpleMap";
                    case "objectarray": return "EventInfraPropertyNestedSimpleOA";
                    case "xml": return "EventInfraPropertyNestedSimpleXML";
                    case "avro": return "EventInfraPropertyNestedSimpleAvro";
                    case "json": return "EventInfraPropertyNestedSimpleJson";
                    case "json-provided": return "EventInfraPropertyNestedSimpleJsonProvided";
                }
                break;
        }
        throw new IllegalArgumentException("unknown case/mode " + caseName + "/" + mode);
    }

    /** Pinned schema-creation EPL per (case, mode); "" means config-registered. */
    private static String pinnedSchemaEPL(String caseName, String mode) {
        String keyExpr = pkg("EventInfraPropertyIndexedKeyExpr");
        String runtimeIndex = pkg("EventInfraPropertyIndexedRuntimeIndex");
        String mappedIndexed = pkg("EventInfraPropertyMappedIndexed");
        String runtimeKey = pkg("EventInfraPropertyMappedRuntimeKey");
        String nestedIndexed = pkg("EventInfraPropertyNestedIndexed");
        String nestedSimple = pkg("EventInfraPropertyNestedSimple");
        switch (caseName) {
            case "indexed-key-expr":
                switch (mode) {
                    case "objectarray":
                        return "@public create objectarray schema OAEventInner(p0 string);\n" +
                                "@buseventtype @public create objectarray schema OAEvent(intarray int[], oainner OAEventInner[]);\n";
                    case "map":
                        return "create schema MapEventInner(p0 string);\n" +
                                "@public @buseventtype create schema MapEvent(intarray int[], mapinner MapEventInner[]);\n";
                    case "wrapper":
                        return "";
                    case "bean":
                        return "@public @buseventtype create schema MyIndexMappedSamplerBean as " + keyExpr + "$MyIndexMappedSamplerBean";
                    case "json":
                        return "@public @buseventtype create json schema JsonSchema(indexed int[], mapped java.util.Map);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + keyExpr + "$MyLocalJsonProvided') @public @buseventtype create json schema JsonSchema();\n";
                }
                break;
            case "indexed-runtime-index":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + runtimeIndex + "$LocalEvent;\n";
                    case "map":
                        return "@public @buseventtype create schema LocalEvent(indexed string[]);\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalEvent(indexed string[]);\n";
                    case "json":
                        return "@public @buseventtype create json schema LocalEvent(indexed string[]);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + runtimeIndex + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@name('schema') @public @buseventtype create avro schema LocalEvent(indexed string[]);\n";
                }
                break;
            case "mapped-indexed":
                switch (mode) {
                    case "json":
                        return "@public @buseventtype @name('schema') create json schema EventInfraPropertyMappedIndexedJson(indexed string[], mapped java.util.Map)";
                    case "json-provided":
                        return "@public @buseventtype @name('schema') @JsonSchema(className='" + mappedIndexed + "$MyLocalJsonProvided') create json schema EventInfraPropertyMappedIndexedJsonProvided()";
                    default:
                        return "";
                }
            case "mapped-runtime-key":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + runtimeKey + "$LocalEvent;\n";
                    case "map":
                        return "@public @buseventtype create schema LocalEvent(mapped java.util.Map);\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalEvent(mapped java.util.Map);\n";
                    case "json":
                        return "@public @buseventtype create json schema LocalEvent(mapped java.util.Map);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + runtimeKey + "$MyLocalJsonProvided') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@name('schema') @public @buseventtype create avro schema LocalEvent(mapped java.util.Map);\n";
                }
                break;
            case "nested-indexed":
                if ("json".equals(mode)) {
                    return "create json schema EventInfraPropertyNestedIndexedJson_4(lvl4 int);\n" +
                            "create json schema EventInfraPropertyNestedIndexedJson_3(lvl3 int, l4 EventInfraPropertyNestedIndexedJson_4[]);\n" +
                            "create json schema EventInfraPropertyNestedIndexedJson_2(lvl2 int, l3 EventInfraPropertyNestedIndexedJson_3[]);\n" +
                            "create json schema EventInfraPropertyNestedIndexedJson_1(lvl1 int, l2 EventInfraPropertyNestedIndexedJson_2[]);\n" +
                            "@name('types') @public @buseventtype create json schema EventInfraPropertyNestedIndexedJson(l1 EventInfraPropertyNestedIndexedJson_1[]);\n";
                }
                if ("json-provided".equals(mode)) {
                    return "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl4') create json schema EventInfraPropertyNestedIndexedJsonProvided_4();\n" +
                            "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl3') create json schema EventInfraPropertyNestedIndexedJsonProvided_3(lvl3 int, l4 EventInfraPropertyNestedIndexedJsonProvided_4[]);\n" +
                            "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl2') create json schema EventInfraPropertyNestedIndexedJsonProvided_2(lvl2 int, l3 EventInfraPropertyNestedIndexedJsonProvided_3[]);\n" +
                            "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedLvl1') create json schema EventInfraPropertyNestedIndexedJsonProvided_1(lvl1 int, l2 EventInfraPropertyNestedIndexedJsonProvided_2[]);\n" +
                            "@JsonSchema(className='" + nestedIndexed + "$MyLocalJSONProvidedTop') @name('types') @public @buseventtype create json schema EventInfraPropertyNestedIndexedJsonProvided(l1 EventInfraPropertyNestedIndexedJsonProvided_1[]);\n";
                }
                return "";
            case "nested-escaped":
                if ("types".equals(mode)) {
                    return "@name('types') @public @buseventtype create schema BeanLvl0 as " + pkg("EventInfraPropertyNestedNestedEscaped$SupportLvl0") + ";\n" +
                            "\n" +
                            "create schema MapLvl3(vlvl3 string);\n" +
                            "create schema MapLvl2(vlvl2 string, lvl3 MapLvl3);\n" +
                            "create schema MapLvl1(lvl2 MapLvl2);\n" +
                            "@public @buseventtype create schema MapLvl0(lvl1 MapLvl1);\n" +
                            "\n" +
                            "create objectarray schema OALvl3(vlvl3 string);\n" +
                            "create objectarray schema OALvl2(vlvl2 string, vlvl2dyn string, lvl3 OALvl3);\n" +
                            "create objectarray schema OALvl1(lvl2 OALvl2);\n" +
                            "@public @buseventtype create objectarray schema OALvl0(lvl1 OALvl1);\n" +
                            "\n" +
                            "@JsonSchema(dynamic=true) create json schema JSONLvl3(vlvl3 string);\n" +
                            "@JsonSchema(dynamic=true) create json schema JSONLvl2(vlvl2 string, lvl3 JSONLvl3);\n" +
                            "create json schema JSONLvl1(lvl2 JSONLvl2);\n" +
                            "@public @buseventtype create json schema JSONLvl0(lvl1 JSONLvl1);\n" +
                            "\n" +
                            "create avro schema AvroLvl3(vlvl3 string);\n" +
                            "create avro schema AvroLvl2(vlvl2 string, vlvl2dyn string, lvl3 AvroLvl3);\n" +
                            "create avro schema AvroLvl1(lvl2 AvroLvl2);\n" +
                            "@public @buseventtype create avro schema AvroLvl0(lvl1 AvroLvl1);\n";
                }
                return "";
            case "nested-simple":
                if ("json".equals(mode)) {
                    return "@public create json schema EventInfraPropertyNestedSimpleJson_4(lvl4 int);\n" +
                            "@public create json schema EventInfraPropertyNestedSimpleJson_3(lvl3 int, l4 EventInfraPropertyNestedSimpleJson_4);\n" +
                            "@public create json schema EventInfraPropertyNestedSimpleJson_2(lvl2 int, l3 EventInfraPropertyNestedSimpleJson_3);\n" +
                            "@public create json schema EventInfraPropertyNestedSimpleJson_1(lvl1 int, l2 EventInfraPropertyNestedSimpleJson_2);\n" +
                            "@name('types') @public @buseventtype create json schema EventInfraPropertyNestedSimpleJson(l1 EventInfraPropertyNestedSimpleJson_1);\n";
                }
                if ("json-provided".equals(mode)) {
                    return "@JsonSchema(className='" + nestedSimple + "$MyLocalJSONProvidedTop') @name('types') @public @buseventtype create json schema EventInfraPropertyNestedSimpleJsonProvided();\n";
                }
                return "";
        }
        return "";
    }

    /** Pinned s0 statement text per (case, mode) — mirrors einpS0EPL. */
    private static String pinnedS0(String caseName, String mode) {
        String t = typeName(caseName, mode);
        switch (caseName) {
            case "indexed-key-expr":
                switch (mode) {
                    case "objectarray":
                        return "@name('s0') select * from OAEvent;\n";
                    case "map":
                        return "@name('s0') select * from MapEvent;\n";
                    case "wrapper":
                        return "@name('s0') select {1, 2} as arr, *, Collections.singletonMap('A', 2) as mapped from SupportBean";
                    case "bean":
                        return "@name('s0') select * from MyIndexMappedSamplerBean";
                    case "json":
                    case "json-provided":
                        return "@name('s0') select * from JsonSchema;\n";
                }
                break;
            case "indexed-runtime-index":
                return "create constant variable int offsetNum = 0;" +
                        "@name('s0') select indexed(offsetNum+0) as c0, indexed(offsetNum+1) as c1 from LocalEvent as e;\n";
            case "mapped-indexed":
                return "@name('s0') select * from " + t;
            case "mapped-runtime-key":
                return "create constant variable string keyChar = 'a';" +
                        "@name('s0') select mapped(keyChar||'1') as c0, mapped(keyChar||'2') as c1 from LocalEvent as e;\n";
            case "nested-indexed":
                return "@name('s0') select " +
                        "l1[0].lvl1 as c0, " +
                        "exists(l1[0].lvl1) as exists_c0, " +
                        "l1[0].l2[0].lvl2 as c1, " +
                        "exists(l1[0].l2[0].lvl2) as exists_c1, " +
                        "l1[0].l2[0].l3[0].lvl3 as c2, " +
                        "exists(l1[0].l2[0].l3[0].lvl3) as exists_c2, " +
                        "l1[0].l2[0].l3[0].l4[0].lvl4 as c3, " +
                        "exists(l1[0].l2[0].l3[0].l4[0].lvl4) as exists_c3 " +
                        "from " + t;
            case "nested-escaped":
                return "@name('s0') select " +
                        "lvl1.lvl2.`vlvl2` as c0, " +
                        "lvl1.lvl2.lvl3.`vlvl3` as c1, " +
                        "`lvl1`.`lvl2`.`lvl3`.`vlvl3` as c2, " +
                        "lvl1.lvl2.`vlvl2dyn`? as c3 " +
                        " from " + t;
            case "nested-simple":
                return "@name('s0') select " +
                        "l1.lvl1 as c0, " +
                        "exists(l1.lvl1) as exists_c0, " +
                        "l1.l2.lvl2 as c1, " +
                        "exists(l1.l2.lvl2) as exists_c1, " +
                        "l1.l2.l3.lvl3 as c2, " +
                        "exists(l1.l2.l3.lvl3) as exists_c2, " +
                        "l1.l2.l3.l4.lvl4 as c3, " +
                        "exists(l1.l2.l3.l4.lvl4) as exists_c3 " +
                        "from " + t;
        }
        return "";
    }

    /** Pinned select-* statement for the nested getter/navigation passes. */
    private static String pinnedStar(String caseName, String mode) {
        switch (caseName) {
            case "mapped-indexed":
            case "nested-indexed":
            case "nested-simple":
            case "nested-escaped":
                return "@name('s0') select * from " + typeName(caseName, mode);
        }
        return "";
    }

    /** XSD for the nested-indexed XML type (maxOccurs=unbounded children). */
    private static final String NESTED_INDEXED_XSD = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
            "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
            "\t<xs:element name=\"myevent\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l1\" maxOccurs=\"unbounded\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l1\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l2\" maxOccurs=\"unbounded\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl1\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l2\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l3\" maxOccurs=\"unbounded\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl2\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l3\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l4\" maxOccurs=\"unbounded\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl3\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l4\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:attribute name=\"lvl4\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "</xs:schema>\n";

    /** XSD for the nested-simple XML type (single-occurrence children). */
    private static final String NESTED_SIMPLE_XSD = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
            "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
            "\t<xs:element name=\"myevent\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l1\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l1\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l2\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl1\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l2\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l3\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl2\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l3\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:sequence>\n" +
            "\t\t\t\t<xs:element ref=\"esper:l4\"/>\n" +
            "\t\t\t</xs:sequence>\n" +
            "\t\t\t<xs:attribute name=\"lvl3\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "\t<xs:element name=\"l4\">\n" +
            "\t\t<xs:complexType>\n" +
            "\t\t\t<xs:attribute name=\"lvl4\" type=\"xs:int\" use=\"required\"/>\n" +
            "\t\t</xs:complexType>\n" +
            "\t</xs:element>\n" +
            "</xs:schema>\n";

    /**
     * Registers the (case, mode) event type in the Configuration — mirrors
     * TestSuiteEventInfra's configureNestedIndexed/configureNestedSimple/
     * configureMappedIndexed blocks. Only called for modes whose pinned
     * schema EPL is empty; the rest deploy through EPL.
     */
    private static void registerType(Configuration config, String caseName, String mode) {
        String name = typeName(caseName, mode);
        switch (mode) {
            case "wrapper":
                if ("indexed-key-expr".equals(caseName)) {
                    config.getCommon().addEventType(SupportBean.class);
                    return;
                }
                break;
            case "bean":
                switch (caseName) {
                    case "mapped-indexed":
                        config.getCommon().addEventType(EventInfraPropertyMappedIndexed.MyIMEvent.class);
                        return;
                    case "nested-indexed":
                        config.getCommon().addEventType(EventInfraPropertyNestedIndexed.InfraNestedIndexPropTop.class);
                        return;
                    case "nested-simple":
                        config.getCommon().addEventType(EventInfraPropertyNestedSimple.InfraNestedSimplePropTop.class);
                        return;
                }
                break;
            case "map":
                switch (caseName) {
                    case "mapped-indexed":
                        config.getCommon().addEventType(name, twoEntryMap("indexed", String[].class, "mapped", Map.class));
                        return;
                    case "nested-indexed":
                        registerNestedMapTypes(config, name, true);
                        return;
                    case "nested-simple":
                        registerNestedMapTypes(config, name, false);
                        return;
                }
                break;
            case "objectarray":
                switch (caseName) {
                    case "mapped-indexed":
                        config.getCommon().addEventType(name,
                                new String[]{"indexed", "mapped"}, new Object[]{String[].class, Map.class});
                        return;
                    case "nested-indexed":
                        registerNestedOATypes(config, name, true);
                        return;
                    case "nested-simple":
                        registerNestedOATypes(config, name, false);
                        return;
                }
                break;
            case "xml":
                if ("nested-indexed".equals(caseName) || "nested-simple".equals(caseName)) {
                    ConfigurationCommonEventTypeXMLDOM meta = new ConfigurationCommonEventTypeXMLDOM();
                    meta.setRootElementName("myevent");
                    meta.setSchemaText("nested-indexed".equals(caseName) ? NESTED_INDEXED_XSD : NESTED_SIMPLE_XSD);
                    config.getCommon().addEventType(name, meta);
                    return;
                }
                break;
            case "avro":
                switch (caseName) {
                    case "mapped-indexed": {
                        Schema schema = SchemaBuilder.record("AvroSchema").fields()
                                .name("indexed").type(SchemaBuilder.array().items()
                                        .stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                                        .endString()).noDefault()
                                .name("mapped").type(SchemaBuilder.map().values()
                                        .stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                                        .endString()).noDefault()
                                .endRecord();
                        config.getCommon().addEventTypeAvro(name,
                                new com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro(schema));
                        return;
                    }
                    case "nested-indexed":
                        registerNestedAvroTypes(config, name, true);
                        return;
                    case "nested-simple":
                        registerNestedAvroTypes(config, name, false);
                        return;
                }
                break;
        }
        throw new IllegalArgumentException("no config registration for " + caseName + "/" + mode);
    }

    private static Map<String, Object> twoEntryMap(String k1, Object v1, String k2, Object v2) {
        Map<String, Object> map = new LinkedHashMap<>();
        map.put(k1, v1);
        map.put(k2, v2);
        return map;
    }

    private static void registerNestedMapTypes(Configuration config, String name, boolean indexed) {
        config.getCommon().addEventType(name + "_4", Collections.singletonMap("lvl4", int.class));
        String child = indexed ? name + "_4[]" : name + "_4";
        config.getCommon().addEventType(name + "_3", twoEntryMap("l4", child, "lvl3", int.class));
        child = indexed ? name + "_3[]" : name + "_3";
        config.getCommon().addEventType(name + "_2", twoEntryMap("l3", child, "lvl2", int.class));
        child = indexed ? name + "_2[]" : name + "_2";
        config.getCommon().addEventType(name + "_1", twoEntryMap("l2", child, "lvl1", int.class));
        config.getCommon().addEventType(name, Collections.singletonMap("l1", indexed ? name + "_1[]" : name + "_1"));
    }

    private static void registerNestedOATypes(Configuration config, String name, boolean indexed) {
        config.getCommon().addEventType(name + "_4", new String[]{"lvl4"}, new Object[]{int.class});
        config.getCommon().addEventType(name + "_3",
                new String[]{"l4", "lvl3"}, new Object[]{indexed ? name + "_4[]" : name + "_4", int.class});
        config.getCommon().addEventType(name + "_2",
                new String[]{"l3", "lvl2"}, new Object[]{indexed ? name + "_3[]" : name + "_3", int.class});
        config.getCommon().addEventType(name + "_1",
                new String[]{"l2", "lvl1"}, new Object[]{indexed ? name + "_2[]" : name + "_2", int.class});
        config.getCommon().addEventType(name, new String[]{"l1"}, new Object[]{indexed ? name + "_1[]" : name + "_1"});
    }

    private static void registerNestedAvroTypes(Configuration config, String name, boolean indexed) {
        Schema s4 = SchemaBuilder.record(name + "_4").fields().requiredInt("lvl4").endRecord();
        SchemaBuilder.FieldAssembler<Schema> f3 = SchemaBuilder.record(name + "_3").fields();
        Schema s3 = (indexed ? f3.name("l4").type(SchemaBuilder.array().items(s4)).noDefault()
                : f3.name("l4").type(s4).noDefault()).requiredInt("lvl3").endRecord();
        SchemaBuilder.FieldAssembler<Schema> f2 = SchemaBuilder.record(name + "_2").fields();
        Schema s2 = (indexed ? f2.name("l3").type(SchemaBuilder.array().items(s3)).noDefault()
                : f2.name("l3").type(s3).noDefault()).requiredInt("lvl2").endRecord();
        SchemaBuilder.FieldAssembler<Schema> f1 = SchemaBuilder.record(name + "_1").fields();
        Schema s1 = (indexed ? f1.name("l2").type(SchemaBuilder.array().items(s2)).noDefault()
                : f1.name("l2").type(s2).noDefault()).requiredInt("lvl1").endRecord();
        Schema schema = SchemaBuilder.record(name).fields()
                .name("l1").type(indexed ? SchemaBuilder.array().items(s1) : s1).noDefault()
                .endRecord();
        config.getCommon().addEventTypeAvro(name,
                new com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro(schema));
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EventInfraPropertyNonDynamicScenarioOracle <scenario.json>");
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
     * restart per iteration like the Go runner's fresh engine. Nested cases
     * keep the runtime warm across underlyings because Java undeploys only
     * the s0 module between passes.
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

                        if ("schema".equals(label)) {
                            String pinned = pinnedSchemaEPL(caseName, mode);
                            if (!pinned.equals(epl)) {
                                throw new IllegalStateException("schema EPL is not pinned for " + caseName + "/" + mode);
                            }
                            // The nested-escaped "types" deploy creates all
                            // five underlyings and carries no eventType pin.
                            if (!"types".equals(mode)
                                    && !typeName(caseName, mode).equals(step.getString("eventType", ""))) {
                                throw new IllegalStateException("schema eventType is not pinned for " + caseName + "/" + mode);
                            }
                            if (!epl.isEmpty()) {
                                EPCompiled compiled = EPCompilerProvider.getCompiler()
                                        .compile(epl, new CompilerArguments(config));
                                schemaCompiled.put("schema", compiled);
                                EPDeployment deployment = runtime.getDeploymentService()
                                        .deploy(compiled, new com.espertech.esper.runtime.client.DeploymentOptions());
                                state.schemaDeploymentId = deployment.getDeploymentId();
                            }
                            state.mode = mode;
                        } else {
                            String star = pinnedStar(caseName, mode);
                            if (!epl.equals(pinnedS0(caseName, mode)) && !epl.equals(star)) {
                                throw new IllegalStateException("deploy " + label + " EPL is not pinned in " + caseName + "/" + mode);
                            }
                            state.s0Star = epl.equals(star) || "indexed-key-expr".equals(caseName);
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
                        String label = step.getString("statement", "");
                        EventType eventType;
                        if (step.get("eventType") != null) {
                            // Type-level checks resolve the input event type:
                            // preconfigured (config) types first, then the last
                            // schema deployment (Java's deploymentId('types')).
                            String typeName = step.getString("eventType", "");
                            eventType = runtime.getEventTypeService().getEventTypePreconfigured(typeName);
                            if (eventType == null && state.schemaDeploymentId != null) {
                                eventType = runtime.getEventTypeService()
                                        .getEventType(state.schemaDeploymentId, typeName);
                            }
                            if (eventType == null) {
                                throw new IllegalStateException("no event type " + typeName + " for " + caseName);
                            }
                        } else {
                            EPDeployment s0 = deployments.get("s0");
                            eventType = runtime.getDeploymentService()
                                    .getStatement(s0.getDeploymentId(), "s0").getEventType();
                        }
                        JsonArray order = step.get("propertyOrder").asArray();
                        JsonObject pins = step.get("propertyTypes").asObject();
                        for (JsonValue nameValue : order) {
                            String propName = nameValue.asString();
                            String expected = pins.getString(propName, "");
                            if ("<invalid>".equals(expected)) {
                                if (eventType.isProperty(propName)
                                        || eventType.getPropertyType(propName) != null) {
                                    throw new IllegalStateException("property " + propName
                                            + " unexpectedly resolves on " + eventType.getName());
                                }
                            } else if ("exists".equals(expected)) {
                                // assertNotNull(getGetter) + assertTrue(isProperty)
                                if (!eventType.isProperty(propName)
                                        || eventType.getGetter(propName) == null) {
                                    throw new IllegalStateException("no getter for " + propName
                                            + " on " + eventType.getName());
                                }
                            } else {
                                Class<?> propertyType = eventType.getPropertyType(propName);
                                if (propertyType == null) {
                                    throw new IllegalStateException("no property type for " + propName);
                                }
                                if (!"l1".equals(propName)) {
                                    // l1 pins carry the fragment type name
                                    // (e.g. EventInfraPropertyNestedIndexedMap_1[])
                                    // rather than the property type; all other
                                    // named pins assert the boxed type name.
                                    String actual = JavaClassHelper.getBoxedType(propertyType).getSimpleName();
                                    assertEquals(expected, actual);
                                }
                            }
                            int sequence = sequences.merge(label + ":types", 1, Integer::sum);
                            JsonObject record = new JsonObject();
                            record.add("case", caseName);
                            record.add("operation", "types");
                            record.add("statement", label);
                            record.add("sequence", sequence);
                            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                            record.add("name", propName);
                            record.add("value", expected);
                            records.add(record);
                        }
                        break;
                    }
                    case "send":
                        state.mode = step.getString("mode", state.mode);
                        sendEvent(runtime, state, caseName, state.mode,
                                step.getString("eventType", ""), step.get("payload").asObject());
                        emitGetterProbes(caseName, state);
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
                        state.schemaDeploymentId = null;
                        state.lastS0 = null;
                        state.s0Star = false;
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
     * Config-registered types resolve at runtime creation: every schema deploy
     * step whose pinned EPL is empty registers its (case, mode) type up front,
     * matching the suite runner's session-level addEventType calls.
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
                String mode = step.getString("mode", "");
                if (pinnedSchemaEPL(caseName, mode).isEmpty()) {
                    registerType(config, caseName, mode);
                }
            }
        }
    }

    /**
     * Per-case replay state: sequences roll the listener/deployed counters
     * and lastS0 captures the latest select-* event for the getter probes.
     */
    private static final class CaseState {
        Map<String, Integer> sequences;
        JsonArray records;
        EPRuntime runtime;
        EventBean lastS0;
        String mode;
        String schemaDeploymentId;
        boolean s0Star;
    }

    /**
     * Mirrors the oracle listener: one record per statement callback; captures
     * the latest s0 event for the getter probes. XML nested-indexed select-*
     * rows re-wrap element children to arrays because Java's getter returns
     * NodeLists for maxOccurs=unbounded properties.
     */
    private static UpdateListener listener(String caseName, CaseState state) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = state.sequences.merge(statement.getName(), 1, Integer::sum);
            boolean xmlIndexed = "xml".equals(state.mode) && "nested-indexed".equals(caseName);
            JsonArray newRows = rows(newEvents, xmlIndexed);
            JsonArray oldRows = rows(oldEvents, xmlIndexed);
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
            if ("s0".equals(statement.getName()) && state.s0Star
                    && newEvents != null && newEvents.length > 0) {
                state.lastS0 = newEvents[newEvents.length - 1];
            }
        };
    }

    /** Probe kinds mirror the Java getter APIs exercised per select-* send. */
    private static final int PROBE_VALUE = 0;
    private static final int PROBE_INDEXED = 1;
    private static final int PROBE_MAPPED = 2;
    private static final int PROBE_INVALID = 3;
    private static final int PROBE_FRAGMENT = 4;
    private static final int PROBE_FRAGMENT_EXISTS = 5;

    private static final class Probe {
        final String name;
        final int kind;
        final String path;
        final String arg;
        Probe(String name, int kind, String path, String arg) {
            this.name = name;
            this.kind = kind;
            this.path = path;
            this.arg = arg;
        }
    }

    /**
     * Per-send probe list for a case/mode pair on the select-* pass — mirrors
     * einpProbes. Names are the Java property names the records carry.
     */
    private static List<Probe> probesFor(String caseName, String mode) {
        boolean xml = "xml".equals(mode);
        List<Probe> probes = new ArrayList<>();
        switch (caseName) {
            case "indexed-key-expr":
                switch (mode) {
                    case "objectarray":
                        probes.add(new Probe("intarray", PROBE_INDEXED, "intarray", "1"));
                        probes.add(new Probe("oainner", PROBE_INDEXED, "oainner", "1"));
                        probes.add(new Probe("dummy", PROBE_INVALID, "dummy", null));
                        break;
                    case "map":
                        probes.add(new Probe("intarray", PROBE_INDEXED, "intarray", "1"));
                        probes.add(new Probe("mapinner", PROBE_INDEXED, "mapinner", "1"));
                        probes.add(new Probe("dummy", PROBE_INVALID, "dummy", null));
                        break;
                    case "wrapper":
                        probes.add(new Probe("arr", PROBE_INDEXED, "arr", "1"));
                        probes.add(new Probe("mapped", PROBE_MAPPED, "mapped", "A"));
                        break;
                    case "bean":
                        probes.add(new Probe("listOfInt", PROBE_INDEXED, "listOfInt", "1"));
                        probes.add(new Probe("iterableOfInt", PROBE_INDEXED, "iterableOfInt", "1"));
                        break;
                    case "json":
                    case "json-provided":
                        probes.add(new Probe("indexed", PROBE_INDEXED, "indexed", "1"));
                        probes.add(new Probe("mapped", PROBE_MAPPED, "mapped", "keyOne"));
                        break;
                }
                break;
            case "mapped-indexed":
                probes.add(new Probe("mapped", PROBE_MAPPED, "mapped", "k1"));
                probes.add(new Probe("indexed", PROBE_INDEXED, "indexed", "1"));
                for (String path : Arrays.asList("xxxx", "mapped[1]", "indexed('a')", "mapped.x", "indexed.x")) {
                    probes.add(new Probe(path, PROBE_INVALID, path, null));
                }
                break;
            case "nested-escaped":
                probes.add(new Probe("lvl1.lvl2.`vlvl2`", PROBE_VALUE, "lvl1.lvl2.`vlvl2`", null));
                probes.add(new Probe("lvl1.lvl2.lvl3.`vlvl3`", PROBE_VALUE, "lvl1.lvl2.lvl3.`vlvl3`", null));
                probes.add(new Probe("`lvl1`.`lvl2`.`lvl3`.`vlvl3`", PROBE_VALUE, "`lvl1`.`lvl2`.`lvl3`.`vlvl3`", null));
                probes.add(new Probe("lvl1.lvl2.`vlvl2dyn`?", PROBE_VALUE, "lvl1.lvl2.`vlvl2dyn`?", null));
                probes.add(new Probe("lvl1.lvl2.`lvl3`", PROBE_FRAGMENT, "lvl1.lvl2.`lvl3`", null));
                break;
            case "nested-indexed":
                probes.add(new Probe("l1[0].lvl1", PROBE_VALUE, "l1[0].lvl1", null));
                probes.add(new Probe("l1[0].l2[0].lvl2", PROBE_VALUE, "l1[0].l2[0].lvl2", null));
                probes.add(new Probe("l1[0].l2[0].l3[0].lvl3", PROBE_VALUE, "l1[0].l2[0].l3[0].lvl3", null));
                probes.add(new Probe("l1[0].l2[0].l3[0].l4[0].lvl4", PROBE_VALUE, "l1[0].l2[0].l3[0].l4[0].lvl4", null));
                if (xml) {
                    for (String path : Arrays.asList("l1", "l1[0].l2", "l1[0].l2[0].l3", "l1[0].l2[0].l3[0].l4",
                            "l1[0]", "l1[0].l2[0]", "l1[0].l2[0].l3[0]", "l1[0].l2[0].l3[0].l4[0]")) {
                        probes.add(new Probe(path, PROBE_FRAGMENT_EXISTS, path, null));
                    }
                } else {
                    for (String path : Arrays.asList("l1", "l1[0].l2", "l1[0].l2[0].l3", "l1[0].l2[0].l3[0].l4")) {
                        probes.add(new Probe(path, PROBE_FRAGMENT, path, null));
                    }
                    for (String path : Arrays.asList("l1[0]", "l1[0].l2[0]", "l1[0].l2[0].l3[0]", "l1[0].l2[0].l3[0].l4[0]")) {
                        probes.add(new Probe(path, PROBE_FRAGMENT, path, null));
                    }
                }
                for (String path : Arrays.asList("l2", "l2[0]", "l1[0].l3", "l1[0].l2[0].l3[0].x")) {
                    probes.add(new Probe(path, PROBE_INVALID, path, null));
                }
                break;
            case "nested-simple":
                probes.add(new Probe("l1.lvl1", PROBE_VALUE, "l1.lvl1", null));
                probes.add(new Probe("l1.l2.lvl2", PROBE_VALUE, "l1.l2.lvl2", null));
                probes.add(new Probe("l1.l2.l3.lvl3", PROBE_VALUE, "l1.l2.l3.lvl3", null));
                probes.add(new Probe("l1.l2.l3.l4.lvl4", PROBE_VALUE, "l1.l2.l3.l4.lvl4", null));
                if (xml) {
                    // Java calls assertFragments twice: once on "l1.l2" alone,
                    // then on the four chain paths — l1.l2 is duplicated.
                    for (String path : Arrays.asList("l1.l2", "l1", "l1.l2", "l1.l2.l3", "l1.l2.l3.l4")) {
                        probes.add(new Probe(path, PROBE_FRAGMENT_EXISTS, path, null));
                    }
                } else {
                    for (String path : Arrays.asList("l1.l2", "l1", "l1.l2", "l1.l2.l3", "l1.l2.l3.l4")) {
                        probes.add(new Probe(path, PROBE_FRAGMENT, path, null));
                    }
                }
                for (String path : Arrays.asList("l2", "l1.l3", "l1.xxx", "l1.l2.x", "l1.l2.l3.x", "l1.lvl1.x")) {
                    probes.add(new Probe(path, PROBE_INVALID, path, null));
                }
                break;
        }
        return probes;
    }

    /**
     * Emits the per-send getter-probe records that Java runs inside
     * assertEventNew on the select-* pass: getGetter/event.get value probes,
     * getFragment fragment probes, and tryInvalidProperty exists-false pins.
     */
    private static void emitGetterProbes(String caseName, CaseState state) {
        if (!state.s0Star) {
            return;
        }
        List<Probe> probes = probesFor(caseName, state.mode);
        if (probes.isEmpty()) {
            return;
        }
        EventBean event = state.lastS0;
        if (event == null) {
            throw new IllegalStateException("no s0 event captured for getter probe");
        }
        boolean xmlIndexed = "xml".equals(state.mode) && "nested-indexed".equals(caseName);
        for (Probe probe : probes) {
            boolean exists = false;
            JsonValue rendered = Json.NULL;
            switch (probe.kind) {
                case PROBE_INDEXED: {
                    EventPropertyGetterIndexed getter =
                            event.getEventType().getGetterIndexed(probe.path);
                    if (getter != null) {
                        exists = true;
                        rendered = normalize(getter.get(event, Integer.parseInt(probe.arg)), xmlIndexed);
                    }
                    break;
                }
                case PROBE_MAPPED: {
                    EventPropertyGetterMapped getter =
                            event.getEventType().getGetterMapped(probe.path);
                    if (getter != null) {
                        exists = true;
                        rendered = normalize(getter.get(event, probe.arg), xmlIndexed);
                    }
                    break;
                }
                case PROBE_VALUE: {
                    Object value;
                    try {
                        value = event.get(probe.path);
                        exists = true;
                    } catch (RuntimeException ex) {
                        value = null;
                    }
                    if (exists) {
                        rendered = normalize(value, xmlIndexed);
                    }
                    break;
                }
                case PROBE_INVALID: {
                    EventPropertyGetter getter = event.getEventType().getGetter(probe.path);
                    exists = getter != null;
                    if (exists) {
                        try {
                            rendered = normalize(event.get(probe.path), xmlIndexed);
                        } catch (RuntimeException ex) {
                            exists = false;
                        }
                    }
                    break;
                }
                case PROBE_FRAGMENT_EXISTS: {
                    exists = event.getFragment(probe.path) != null;
                    break;
                }
                case PROBE_FRAGMENT: {
                    Object fragment;
                    try {
                        fragment = event.getFragment(probe.path);
                    } catch (RuntimeException ex) {
                        fragment = null;
                    }
                    exists = fragment != null;
                    if (exists) {
                        rendered = normalizeFragment(fragment, xmlIndexed);
                    }
                    break;
                }
            }
            int sequence = state.sequences.merge("s0:getter", 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "getter");
            record.add("statement", "s0");
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
            record.add("name", probe.name);
            JsonObject value = new JsonObject();
            value.add("exists", exists);
            value.add("value", rendered);
            record.add("value", value);
            state.records.add(record);
        }
    }

    /**
     * Fragment normalization: EventBean values render via their property map,
     * raw beans via getters, and arrays/NodeLists element-wise — matching the
     * Go schema-field rendering of nested fragments.
     */
    private static JsonValue normalizeFragment(Object fragment, boolean xmlArrays) {
        if (fragment instanceof NodeList) {
            JsonArray array = new JsonArray();
            NodeList list = (NodeList) fragment;
            for (int i = 0; i < list.getLength(); i++) {
                array.add(normalize(list.item(i), xmlArrays));
            }
            return array;
        }
        return normalize(fragment, xmlArrays);
    }

    private static void sendEvent(EPRuntime runtime, CaseState state, String caseName, String mode,
                                  String eventType, JsonObject payload) throws Exception {
        switch (mode) {
            case "bean":
            case "wrapper":
                sendBean(runtime, caseName, mode, eventType, payload);
                return;
            case "map":
                sendMap(runtime, caseName, eventType, payload);
                return;
            case "objectarray":
                sendObjectArray(runtime, caseName, eventType, payload);
                return;
            case "xml":
                sendXml(runtime, caseName, eventType, payload);
                return;
            case "avro":
                sendAvro(runtime, state, caseName, eventType, payload);
                return;
            case "json":
            case "json-provided":
                sendJson(runtime, caseName, eventType, payload);
                return;
            default:
                throw new IllegalArgumentException("unknown send mode " + mode);
        }
    }

    /** FBEAN-equivalent senders per case: payload ints/strings build the beans. */
    private static void sendBean(EPRuntime runtime, String caseName, String mode,
                                 String eventType, JsonObject payload) {
        switch (caseName) {
            case "indexed-key-expr":
                if ("wrapper".equals(mode)) {
                    runtime.getEventService().sendEventBean(new SupportBean(), eventType);
                    return;
                }
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyIndexedKeyExpr.MyIndexMappedSamplerBean(), eventType);
                return;
            case "indexed-runtime-index":
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyIndexedRuntimeIndex.LocalEvent(new String[]{"a", "b"}),
                        eventType);
                return;
            case "mapped-indexed":
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyMappedIndexed.MyIMEvent(
                                new String[]{"v1", "v2"}, Collections.singletonMap("k1", "v1")),
                        eventType);
                return;
            case "mapped-runtime-key": {
                Map<String, String> mapped = new LinkedHashMap<>();
                mapped.put("a1", "x");
                mapped.put("a2", "y");
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyMappedRuntimeKey.LocalEvent(mapped), eventType);
                return;
            }
            case "nested-indexed": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl4 l4 =
                        new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl4(lvl4);
                EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl3 l3 =
                        new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl3(
                                new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl4[]{l4}, lvl3);
                EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl2 l2 =
                        new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl2(
                                new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl3[]{l3}, lvl2);
                EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl1 l1 =
                        new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl1(
                                new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl2[]{l2}, lvl1);
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyNestedIndexed.InfraNestedIndexPropTop(
                                new EventInfraPropertyNestedIndexed.InfraNestedIndexedPropLvl1[]{l1}),
                        eventType);
                return;
            }
            case "nested-escaped": {
                String vlvl2 = payload.getString("vlvl2", "");
                String vlvl2dyn = payload.getString("vlvl2dyn", "");
                String vlvl3 = payload.getString("vlvl3", "");
                EventInfraPropertyNestedNestedEscaped.SupportLvl3 l3 =
                        new EventInfraPropertyNestedNestedEscaped.SupportLvl3(vlvl3);
                EventInfraPropertyNestedNestedEscaped.SupportLvl2 l2 =
                        new EventInfraPropertyNestedNestedEscaped.SupportLvl2(vlvl2, vlvl2dyn, l3);
                EventInfraPropertyNestedNestedEscaped.SupportLvl1 l1 =
                        new EventInfraPropertyNestedNestedEscaped.SupportLvl1(l2);
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyNestedNestedEscaped.SupportLvl0(l1), eventType);
                return;
            }
            case "nested-simple": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl4 l4 =
                        new EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl4(lvl4);
                EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl3 l3 =
                        new EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl3(l4, lvl3);
                EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl2 l2 =
                        new EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl2(l3, lvl2);
                EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl1 l1 =
                        new EventInfraPropertyNestedSimple.InfraNestedSimplePropLvl1(l2, lvl1);
                runtime.getEventService().sendEventBean(
                        new EventInfraPropertyNestedSimple.InfraNestedSimplePropTop(l1), eventType);
                return;
            }
        }
        throw new IllegalArgumentException("no bean sender for " + caseName);
    }

    /** FMAP-equivalent senders; nested cases rebuild the chain from ints/strings. */
    private static void sendMap(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "nested-indexed": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                Map<String, Object> l4 = Collections.singletonMap("lvl4", lvl4);
                Map<String, Object> l3 = twoEntryMap("l4", new Map[]{l4}, "lvl3", lvl3);
                Map<String, Object> l2 = twoEntryMap("l3", new Map[]{l3}, "lvl2", lvl2);
                Map<String, Object> l1 = twoEntryMap("l2", new Map[]{l2}, "lvl1", lvl1);
                runtime.getEventService().sendEventMap(Collections.singletonMap("l1", new Map[]{l1}), eventType);
                return;
            }
            case "nested-simple": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                Map<String, Object> l4 = Collections.singletonMap("lvl4", lvl4);
                Map<String, Object> l3 = twoEntryMap("l4", l4, "lvl3", lvl3);
                Map<String, Object> l2 = twoEntryMap("l3", l3, "lvl2", lvl2);
                Map<String, Object> l1 = twoEntryMap("l2", l2, "lvl1", lvl1);
                runtime.getEventService().sendEventMap(Collections.singletonMap("l1", l1), eventType);
                return;
            }
            case "nested-escaped": {
                String vlvl2 = payload.getString("vlvl2", "");
                String vlvl2dyn = payload.getString("vlvl2dyn", "");
                String vlvl3 = payload.getString("vlvl3", "");
                Map<String, Object> l3 = Collections.singletonMap("vlvl3", vlvl3);
                Map<String, Object> l2 = new LinkedHashMap<>();
                l2.put("vlvl2", vlvl2);
                l2.put("lvl3", l3);
                l2.put("vlvl2dyn", vlvl2dyn);
                Map<String, Object> l1 = Collections.singletonMap("lvl2", l2);
                runtime.getEventService().sendEventMap(Collections.singletonMap("lvl1", l1), eventType);
                return;
            }
        }
        runtime.getEventService().sendEventMap(toMap(payload), eventType);
    }

    /** FOA-equivalent senders; nested cases rebuild positional rows. */
    private static void sendObjectArray(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "nested-indexed": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                Object[] l4 = new Object[]{lvl4};
                Object[] l3 = new Object[]{new Object[]{l4}, lvl3};
                Object[] l2 = new Object[]{new Object[]{l3}, lvl2};
                Object[] l1 = new Object[]{new Object[]{l2}, lvl1};
                runtime.getEventService().sendEventObjectArray(new Object[]{new Object[]{l1}}, eventType);
                return;
            }
            case "nested-simple": {
                int lvl1 = payload.getInt("lvl1", 0), lvl2 = payload.getInt("lvl2", 0),
                        lvl3 = payload.getInt("lvl3", 0), lvl4 = payload.getInt("lvl4", 0);
                Object[] l4 = new Object[]{lvl4};
                Object[] l3 = new Object[]{l4, lvl3};
                Object[] l2 = new Object[]{l3, lvl2};
                Object[] l1 = new Object[]{l2, lvl1};
                runtime.getEventService().sendEventObjectArray(new Object[]{l1}, eventType);
                return;
            }
            case "nested-escaped": {
                String vlvl2 = payload.getString("vlvl2", "");
                String vlvl2dyn = payload.getString("vlvl2dyn", "");
                String vlvl3 = payload.getString("vlvl3", "");
                Object[] l3 = new Object[]{vlvl3};
                Object[] l2 = new Object[]{vlvl2, vlvl2dyn, l3};
                Object[] l1 = new Object[]{l2};
                runtime.getEventService().sendEventObjectArray(new Object[]{l1}, eventType);
                return;
            }
        }
        JsonValue values = payload.get("values");
        if (values != null && values.isArray()) {
            runtime.getEventService().sendEventObjectArray(toCells(values.asArray()), eventType);
            return;
        }
        // Flat one-column OA payloads (mapped, indexed) send their values.
        Object[] cells = new Object[payload.names().size()];
        int index = 0;
        for (String name : payload.names()) {
            cells[index++] = toJava(payload.get(name));
        }
        runtime.getEventService().sendEventObjectArray(cells, eventType);
    }

    /** FXML-equivalent senders: nested cases template the myevent document. */
    private static void sendXml(EPRuntime runtime, String caseName, String eventType, JsonObject payload)
            throws Exception {
        if ("nested-indexed".equals(caseName) || "nested-simple".equals(caseName)) {
            String xml = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                    "<myevent>\n" +
                    "\t<l1 lvl1=\"" + payload.getInt("lvl1", 0) + "\">\n" +
                    "\t\t<l2 lvl2=\"" + payload.getInt("lvl2", 0) + "\">\n" +
                    "\t\t\t<l3 lvl3=\"" + payload.getInt("lvl3", 0) + "\">\n" +
                    "\t\t\t\t<l4 lvl4=\"" + payload.getInt("lvl4", 0) + "\">\n" +
                    "\t\t\t\t</l4>\n" +
                    "\t\t\t</l3>\n" +
                    "\t\t</l2>\n" +
                    "\t</l1>\n" +
                    "</myevent>";
            DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
            factory.setNamespaceAware(true);
            Document document = factory.newDocumentBuilder().parse(new InputSource(new StringReader(xml)));
            runtime.getEventService().sendEventXMLDOM(document, eventType);
            return;
        }
        throw new IllegalArgumentException("no xml sender for " + caseName);
    }

    /** FJSON-equivalent senders; nested cases rebuild the document shape. */
    private static void sendJson(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "nested-indexed": {
                // Java's FJSON templates lvl values as strings; the int-typed
                // JSON schema coerces them.
                JsonObject l4 = new JsonObject().add("lvl4", Integer.toString(payload.getInt("lvl4", 0)));
                JsonObject l3 = new JsonObject()
                        .add("lvl3", Integer.toString(payload.getInt("lvl3", 0)))
                        .add("l4", new JsonArray().add(l4));
                JsonObject l2 = new JsonObject()
                        .add("lvl2", Integer.toString(payload.getInt("lvl2", 0)))
                        .add("l3", new JsonArray().add(l3));
                JsonObject l1 = new JsonObject()
                        .add("lvl1", Integer.toString(payload.getInt("lvl1", 0)))
                        .add("l2", new JsonArray().add(l2));
                runtime.getEventService().sendEventJson(
                        new JsonObject().add("l1", new JsonArray().add(l1)).toString(), eventType);
                return;
            }
            case "nested-simple": {
                JsonObject l4 = new JsonObject().add("lvl4", payload.getInt("lvl4", 0));
                JsonObject l3 = new JsonObject()
                        .add("lvl3", payload.getInt("lvl3", 0)).add("l4", l4);
                JsonObject l2 = new JsonObject()
                        .add("lvl2", payload.getInt("lvl2", 0)).add("l3", l3);
                JsonObject l1 = new JsonObject()
                        .add("lvl1", payload.getInt("lvl1", 0)).add("l2", l2);
                runtime.getEventService().sendEventJson(
                        new JsonObject().add("l1", l1).toString(), eventType);
                return;
            }
            case "nested-escaped": {
                JsonObject l3 = new JsonObject().add("vlvl3", payload.getString("vlvl3", ""));
                JsonObject l2 = new JsonObject()
                        .add("vlvl2", payload.getString("vlvl2", ""))
                        .add("vlvl2dyn", payload.getString("vlvl2dyn", ""))
                        .add("lvl3", l3);
                JsonObject l1 = new JsonObject().add("lvl2", l2);
                runtime.getEventService().sendEventJson(
                        new JsonObject().add("lvl1", l1).toString(), eventType);
                return;
            }
        }
        runtime.getEventService().sendEventJson(payload.toString(), eventType);
    }

    /**
     * Avro senders: flat payloads rebuild via the schema; nested cases wrap
     * the ints/strings into the l1/lvl1 / lvl1.lvl2 chain like the suite's
     * FAVRO lambdas.
     */
    private static void sendAvro(EPRuntime runtime, CaseState state, String caseName,
                                 String eventType, JsonObject payload) {
        Schema schema = avroSchemaFor(runtime, state, eventType);
        JsonObject shaped = payload;
        if ("nested-indexed".equals(caseName)) {
            JsonObject l4 = new JsonObject().add("lvl4", payload.get("lvl4"));
            JsonObject l3 = new JsonObject()
                    .add("lvl3", payload.get("lvl3")).add("l4", new JsonArray().add(l4));
            JsonObject l2 = new JsonObject()
                    .add("lvl2", payload.get("lvl2")).add("l3", new JsonArray().add(l3));
            JsonObject l1 = new JsonObject()
                    .add("lvl1", payload.get("lvl1")).add("l2", new JsonArray().add(l2));
            shaped = new JsonObject().add("l1", new JsonArray().add(l1));
        } else if ("nested-simple".equals(caseName)) {
            JsonObject l4 = new JsonObject().add("lvl4", payload.get("lvl4"));
            JsonObject l3 = new JsonObject()
                    .add("lvl3", payload.get("lvl3")).add("l4", l4);
            JsonObject l2 = new JsonObject()
                    .add("lvl2", payload.get("lvl2")).add("l3", l3);
            JsonObject l1 = new JsonObject()
                    .add("lvl1", payload.get("lvl1")).add("l2", l2);
            shaped = new JsonObject().add("l1", l1);
        } else if ("nested-escaped".equals(caseName)) {
            JsonObject l3 = new JsonObject().add("vlvl3", payload.get("vlvl3"));
            JsonObject l2 = new JsonObject()
                    .add("vlvl2", payload.get("vlvl2"))
                    .add("vlvl2dyn", payload.get("vlvl2dyn"))
                    .add("lvl3", l3);
            shaped = new JsonObject().add("lvl1", new JsonObject().add("lvl2", l2));
        }
        GenericData.Record record = toAvroRecord(schema, shaped);
        runtime.getEventService().sendEventAvro(record, eventType);
    }

    /**
     * Resolves an Avro event type's schema: preconfigured (config) types via
     * getEventTypePreconfigured, EPL-created types via the last schema
     * deployment (Java's runtimeAvroSchemaByDeployment).
     */
    private static Schema avroSchemaFor(EPRuntime runtime, CaseState state, String eventType) {
        EventType type = runtime.getEventTypeService().getEventTypePreconfigured(eventType);
        if (type != null) {
            return SupportAvroUtil.getAvroSchema(type);
        }
        if (state.schemaDeploymentId != null) {
            EventType deployed = runtime.getEventTypeService()
                    .getEventType(state.schemaDeploymentId, eventType);
            if (deployed != null) {
                return SupportAvroUtil.getAvroSchema(deployed);
            }
        }
        throw new IllegalArgumentException("no avro schema for " + eventType);
    }

    /**
     * Rebuilds Avro values by navigating the schema: object values map onto
     * the record branch, arrays recurse through the element type, and plain
     * map-typed fields stay maps.
     */
    private static Object toAvroValue(Schema fieldSchema, JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (fieldSchema.getType() == Schema.Type.UNION) {
            for (Schema branch : fieldSchema.getTypes()) {
                if (branch.getType() == Schema.Type.NULL) {
                    continue;
                }
                Object converted = tryAvroValue(branch, value);
                if (converted != null) {
                    return converted;
                }
            }
            return null;
        }
        return tryAvroValue(fieldSchema, value);
    }

    private static Object tryAvroValue(Schema schema, JsonValue value) {
        switch (schema.getType()) {
            case RECORD:
                if (value.isObject()) {
                    return toAvroRecord(schema, value.asObject());
                }
                return null;
            case ARRAY:
                if (!value.isArray()) {
                    return null;
                }
                List<Object> items = new ArrayList<>();
                for (JsonValue item : value.asArray()) {
                    items.add(toAvroValue(schema.getElementType(), item));
                }
                return items;
            case MAP:
                if (!value.isObject()) {
                    return null;
                }
                Map<String, Object> map = new LinkedHashMap<>();
                for (String name : value.asObject().names()) {
                    map.put(name, toAvroValue(schema.getValueType(), value.asObject().get(name)));
                }
                return map;
            default:
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

    /** Decodes the objectarray "values" payload to nested Object[] cells. */
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
            double number = value.asDouble();
            if (number == Math.floor(number) && !Double.isInfinite(number)) {
                return (int) number;
            }
            return number;
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
    private static JsonArray rows(EventBean[] events, boolean xmlArrays) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event, xmlArrays));
        }
        return array;
    }

    private static JsonObject row(EventBean event, boolean xmlArrays) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name), xmlArrays));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * strings/numbers/booleans passthrough, XML nodes via nodeToJson (matching
     * the Go myevent document model; xmlArrays wraps element children in
     * arrays for unbounded properties), NodeLists as arrays, beans via
     * beanToJson (sorted getter-derived properties), EventBean fragments as
     * plain objects, Avro records via their field map, maps as plain objects,
     * arrays as JSON arrays.
     */
    private static JsonValue normalize(Object value, boolean xmlArrays) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return beanFragmentToJson((EventBean) value, xmlArrays);
        }
        if (value instanceof Node) {
            // Xerces DOM nodes implement NodeList too: check Node first so
            // elements render through nodeToJson, and only genuine node lists
            // (Node[] or DeferredNodeListImpl values) hit the array branch.
            return nodeToJson((Node) value, xmlArrays);
        }
        if (value instanceof NodeList) {
            JsonArray array = new JsonArray();
            NodeList list = (NodeList) value;
            for (int i = 0; i < list.getLength(); i++) {
                array.add(normalizeNested(list.item(i), xmlArrays));
            }
            return array;
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
                object.add(name, normalizeNested(record.get(name), xmlArrays));
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
                object.add(key, normalizeNested(map.get(key), xmlArrays));
            }
            return object;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object entry : (Collection<?>) value) {
                array.add(normalizeNested(entry, xmlArrays));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = java.lang.reflect.Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalizeNested(java.lang.reflect.Array.get(value, index), xmlArrays));
            }
            return array;
        }
        return beanToJson(value, xmlArrays);
    }

    /**
     * Nested values (inside maps, records, arrays and beans) render a plain
     * JSON null while top-level row fields and probe values keep the tagged
     * {"state":"null"} object, matching the Go normalizer.
     */
    private static JsonValue normalizeNested(Object value, boolean xmlArrays) {
        if (value == null) {
            return Json.NULL;
        }
        return normalize(value, xmlArrays);
    }

    /**
     * Renders an XML node into the Go document model: elements with no
     * attributes and no element children render as trimmed text; attributes
     * become "@name" keys; when xmlArrays is set (maxOccurs=unbounded
     * properties resolve NodeLists) element children always render as arrays,
     * otherwise repeated children collapse to arrays; non-empty mixed text
     * lands under "#text".
     */
    private static JsonValue nodeToJson(Node node, boolean xmlArrays) {
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
            if (!xmlArrays && items.size() == 1) {
                object.add(entry.getKey(), nodeToJson(items.get(0), xmlArrays));
            } else {
                JsonArray array = new JsonArray();
                for (Node item : items) {
                    array.add(nodeToJson(item, xmlArrays));
                }
                object.add(entry.getKey(), array);
            }
        }
        if (!trimmed.isEmpty()) {
            object.add("#text", trimmed);
        }
        return object;
    }

    /** Nested EventBean values render as plain property objects. */
    private static JsonValue beanFragmentToJson(EventBean event, boolean xmlArrays) {
        JsonObject object = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            object.add(name, normalizeNested(event.get(name), xmlArrays));
        }
        return object;
    }

    /** Renders a Java bean via its public getters (sorted property names). */
    private static JsonValue beanToJson(Object bean, boolean xmlArrays) {
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
                object.add(name, normalizeNested(fieldValue, xmlArrays));
            } catch (Exception e) {
                throw new IllegalStateException("bean render failed for " + name, e);
            }
        }
        return object;
    }
}
