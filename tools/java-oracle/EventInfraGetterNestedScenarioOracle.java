/**
 * Java oracle for the event-infra-getter-nested parity slice: the five
 * EventInfraGetterNested* and EventInfraGetterSimple*Fragment executions (all
 * ord 0, no flags) emit one JSON trace record per deployed marker, listener
 * invocation and getter probe on stdout; the assertions the Java sources
 * perform (exists/value/fragment per send, s1 column assertions) are
 * verified in-process before the getter record is emitted.
 */
import com.espertech.esper.common.client.EPCompiled;

import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyGetter;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeXMLDOM;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterNestedArray;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterNestedSimple;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterNestedSimpleDeep;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterSimpleFragment;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterSimpleNoFragment;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;
import org.w3c.dom.Document;
import org.xml.sax.InputSource;

import javax.xml.parsers.DocumentBuilderFactory;
import java.io.FileReader;
import java.io.StringReader;
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

import static org.apache.avro.SchemaBuilder.record;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;
import static org.junit.Assert.fail;

public final class EventInfraGetterNestedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-infra-getter-nested";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra";
    private static final String XML_TYPE = "EventInfraGetterSimpleNoFragmentXML";

    private static final String[] CASES = {
            "nested-array",
            "nested-simple",
            "nested-simple-deep",
            "simple-fragment",
            "simple-no-fragment",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-718503411fc69f8f01c3",
            "java-runtime-95388a521ff7fc9ae02e",
            "java-runtime-08402ea2d9e12c7ad46c",
            "java-runtime-1eb5dcd8d9611c9f40eb",
            "java-runtime-9c65668855ac0fc31533",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventInfraGetterNestedArray",
            "EventInfraGetterNestedSimple",
            "EventInfraGetterNestedSimpleDeep",
            "EventInfraGetterSimpleFragment",
            "EventInfraGetterSimpleNoFragment",
    };
    private static final String[] STATIC_IDS = {
            "java-fa2a2775be3aa199b6a8",
            "java-45de20b4c43758274efe",
            "java-45fb81d0945532503b7c",
            "java-1d9833f8696776c207ae",
            "java-2be3bd8162600f6a48eb",
    };

    private static final String EPL_S0 = "@name('s0') select * from LocalEvent";
    private static final String EPL_S0_XML = "@name('s0') select * from " + XML_TYPE;
    private static final String EPL_S1_ARRAY =
            "@name('s1') select property[0].id as c0, property[1].id as c1," +
            " exists(property[0].id) as c2, exists(property[1].id) as c3," +
            " typeof(property[0].id) as c4, typeof(property[1].id) as c5" +
            " from LocalEvent;\n";
    private static final String EPL_S1_NESTED =
            "@name('s1') select property.id as c0, exists(property.id) as c1, typeof(property.id) as c2 from LocalEvent;\n";
    private static final String EPL_S1_DEEP =
            "@name('s1') select property.leaf.id as c0, exists(property.leaf.id) as c1, typeof(property.leaf.id) as c2 from LocalEvent;\n";
    private static final String EPL_S1_PROPERTY =
            "@name('s1') select property as c0, exists(property) as c1, typeof(property) as c2 from LocalEvent;\n";
    private static final String EPL_S1_XML =
            "@name('s1') select property as c0, exists(property) as c1, typeof(property) as c2 from " + XML_TYPE + ";\n";

    private static String pkg(String simpleName) {
        return "com.espertech.esper.regressionlib.suite.event.infra." + simpleName;
    }

    /** Pinned schema EPL per (case, mode); empty for the config XML type. */
    private static String pinnedSchemaEPL(String caseName, String mode) {
        switch (caseName) {
            case "nested-array":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalInnerEvent as " + pkg("EventInfraGetterNestedArray$LocalInnerEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterNestedArray$LocalEvent") + ";\n";
                    case "map":
                    case "objectarray":
                    case "json":
                    case "avro":
                        return "@public @buseventtype create " + mode + " schema LocalInnerEvent(id string);\n" +
                                "@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent[]);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterNestedArray$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                }
                break;
            case "nested-simple":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalInnerEvent as " + pkg("EventInfraGetterNestedSimple$LocalInnerEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterNestedSimple$LocalEvent") + ";\n";
                    case "map":
                    case "objectarray":
                    case "json":
                        return "@public @buseventtype create " + mode + " schema LocalInnerEvent(id string);\n" +
                                "@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterNestedSimple$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                }
                break;
            case "nested-simple-deep":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterNestedSimpleDeep$LocalEvent") + ";\n";
                    case "map":
                    case "objectarray":
                    case "json":
                    case "avro":
                        return "@public @buseventtype create " + mode + " schema LocalLeafEvent(id string);\n" +
                                "@public @buseventtype create " + mode + " schema LocalInnerEvent(leaf LocalLeafEvent);\n" +
                                "@name('schema') @public @buseventtype create " + mode + " schema LocalEvent(property LocalInnerEvent);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterNestedSimpleDeep$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                }
                break;
            case "simple-fragment":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterSimpleFragment$LocalEvent") + ";\n";
                    case "map":
                        return "@public @buseventtype create schema LocalInnerEvent();\n" +
                                "@public @buseventtype create schema LocalEvent(property LocalInnerEvent);\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalInnerEvent();\n" +
                                "@public @buseventtype create objectarray schema LocalEvent(property LocalInnerEvent);\n";
                    case "json":
                        return "@public @buseventtype create json schema LocalInnerEvent();\n" +
                                "@public @buseventtype create json schema LocalEvent(property LocalInnerEvent);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterSimpleFragment$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@public @buseventtype create avro schema LocalInnerEvent();\n" +
                                "@name('schema') @public @buseventtype create avro schema LocalEvent(property LocalInnerEvent);\n";
                }
                break;
            case "simple-no-fragment":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterSimpleNoFragment$LocalEvent") + ";\n";
                    case "map":
                        return "@public @buseventtype create schema LocalEvent(property string);\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalEvent(property string);\n";
                    case "json":
                        return "@public @buseventtype create json schema LocalEvent(property string);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterSimpleNoFragment$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@name('schema') @public @buseventtype create avro schema LocalEvent(property string);\n";
                    case "xml":
                        return "";
                }
                break;
        }
        return null;
    }

    /** Pinned s1 EPL per (case, mode). */
    private static String pinnedS1(String caseName, String mode) {
        if ("xml".equals(mode)) {
            return EPL_S1_XML;
        }
        switch (caseName) {
            case "nested-array":
                return EPL_S1_ARRAY;
            case "nested-simple":
                return EPL_S1_NESTED;
            case "nested-simple-deep":
                return EPL_S1_DEEP;
            case "simple-fragment":
            case "simple-no-fragment":
                return EPL_S1_PROPERTY;
        }
        return null;
    }

    /** Pinned s0 EPL per mode. */
    private static String pinnedS0(String mode) {
        return "xml".equals(mode) ? EPL_S0_XML : EPL_S0;
    }

    /** Pinned probe paths per case (the Java getGetter names). */
    private static String[] probes(String caseName) {
        switch (caseName) {
            case "nested-array":
                return new String[]{"property[0].id", "property[1].id"};
            case "nested-simple":
                return new String[]{"property.id"};
            case "nested-simple-deep":
                return new String[]{"property.leaf.id"};
            case "simple-fragment":
            case "simple-no-fragment":
                return new String[]{"property"};
        }
        return new String[0];
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EventInfraGetterNestedScenarioOracle <scenario.json>");
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
     * Replays one execution's scenario steps on a single runtime (Java reuses
     * one RegressionEnvironment across underlyings; undeploy-all between
     * iterations mirrors undeployAll). The XML type is registered on the
     * configuration before the runtime starts, like configureGetterTypes.
     */
    private static void replayCase(JsonObject scenario, String caseName, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().getEventMeta().setEnableXMLXSD(true);
        config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        if ("simple-no-fragment".equals(caseName)) {
            ConfigurationCommonEventTypeXMLDOM meta = new ConfigurationCommonEventTypeXMLDOM();
            meta.setRootElementName(XML_TYPE);
            String schema = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                    "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
                    "\t<xs:element name=\"" + XML_TYPE + "\">\n" +
                    "\t\t<xs:complexType>\n" +
                    "\t\t\t<xs:attribute name=\"property\" type=\"xs:string\" use=\"required\"/>\n" +
                    "\t\t</xs:complexType>\n" +
                    "\t</xs:element>\n" +
                    "</xs:schema>\n";
            meta.setSchemaText(schema);
            config.getCommon().addEventType(XML_TYPE, meta);
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
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
                            if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("schema EPL is not pinned for " + caseName + "/" + mode);
                            }
                            state.schemaEpl = epl;
                            state.mode = mode;
                        } else {
                            String pinned = "s0".equals(label) ? pinnedS0(state.mode) : pinnedS1(caseName, state.mode);
                            if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("deploy " + label + " EPL is not pinned in case " + caseName);
                            }
                            if ("s0".equals(label)) {
                                state.s0Epl = epl;
                            } else {
                                // The Java sources compile schema+s0+s1 as a
                                // single module (env.compileDeploy); deploying
                                // them separately cannot resolve nested
                                // fragment types like LocalInnerEvent, so the
                                // merged module deploys at the s1 step.
                                String module = state.schemaEpl + state.s0Epl + ";\n" + epl;
                                EPCompiled compiled = EPCompilerProvider.getCompiler()
                                        .compile(module, new CompilerArguments(config));
                                EPDeployment deployment;
                                try {
                                    deployment = runtime.getDeploymentService()
                                            .deploy(compiled, new com.espertech.esper.runtime.client.DeploymentOptions());
                                } catch (Exception e) {
                                    throw new IllegalStateException("deploy failed for "
                                            + caseName + "/" + mode, e);
                                }
                                state.deployment = deployment;
                                for (EPStatement statement : deployment.getStatements()) {
                                    statement.addListener(listener(caseName, state));
                                }
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
                    case "send":
                        sendEvent(runtime, caseName, step.getString("mode", ""),
                                step.getString("eventType", ""), step.get("payload").asObject(), state);
                        emitGetterRecords(caseName, state);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        state.deployment = null;
                        state.schemaEpl = null;
                        state.s0Epl = null;
                        state.lastS0 = null;
                        state.mode = null;
                        // Each underlying is a fresh module set: listener,
                        // deployed and getter sequence counters restart.
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

    private static final class CaseState {
        JsonArray records;
        Map<String, Integer> sequences;
        EPRuntime runtime;
        EPDeployment deployment;
        String schemaEpl;
        String s0Epl;
        EventBean lastS0;
        String mode;
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; captures the latest s0 event for the getter probes.
     */
    private static UpdateListener listener(String caseName, CaseState state) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = state.sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(caseName, statement.getName(), newEvents);
            JsonArray oldRows = rows(caseName, statement.getName(), oldEvents);
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
            if ("s0".equals(statement.getName()) && newEvents != null && newEvents.length > 0) {
                state.lastS0 = newEvents[newEvents.length - 1];
            }
        };
    }

    /**
     * Emits the getter-probe records after a send: one record per probe path
     * carrying exists (isExistsProperty), value (get) and fragment
     * (getFragment != null), and asserting the pinned Java expectations
     * in-process before the record is emitted.
     */
    private static void emitGetterRecords(String caseName, CaseState state) {
        EventBean event = state.lastS0;
        if (event == null) {
            throw new IllegalStateException("no s0 event captured for getter probe in " + caseName);
        }
        for (String path : probes(caseName)) {
            EventPropertyGetter getter = event.getEventType().getGetter(path);
            if (getter == null) {
                throw new IllegalStateException("getter " + path + " missing in " + caseName);
            }
            boolean exists = getter.isExistsProperty(event);
            Object value = getter.get(event);
            Object fragment = getter.getFragment(event);
            JsonObject probe = new JsonObject();
            probe.add("exists", exists);
            probe.add("value", exists ? normalize(value) : Json.NULL);
            probe.add("fragment", fragment != null);
            int sequence = state.sequences.merge("s0:getter", 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "getter");
            record.add("statement", "s0");
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
            record.add("name", path);
            record.add("value", probe);
            state.records.add(record);
            // In-process assertions mirroring the Java assertGetter methods:
            // leaf probes always report a null fragment while simple-fragment
            // requires the fragment iff the nested event is present.
            if ("simple-fragment".equals(caseName)) {
                assertTrue(exists);
                assertEquals(fragment != null, value != null);
            } else {
                assertNull(fragment);
            }
        }
    }

    /** sendEvent rebuilds the Java sender for the (case, mode, payload) pin. */
    private static void sendEvent(EPRuntime runtime, String caseName, String mode,
                                  String eventType, JsonObject payload,
                                  CaseState state) {
        switch (mode) {
            case "bean":
                sendBean(runtime, caseName, eventType, payload);
                return;
            case "map":
                runtime.getEventService().sendEventMap(toMapPayload(caseName, payload), eventType);
                return;
            case "objectarray":
                sendObjectArray(runtime, caseName, eventType, payload);
                return;
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson(toJsonPayload(caseName, payload), eventType);
                return;
            case "avro":
                sendAvro(runtime, caseName, eventType, payload, state);
                return;
            case "xml":
                sendXML(runtime, eventType, payload);
                return;
            default:
                throw new IllegalArgumentException("unknown send mode " + mode);
        }
    }

    private static void sendBean(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "nested-array": {
                JsonValue ids = payload.get("ids");
                EventInfraGetterNestedArray.LocalInnerEvent[] property;
                if (ids == null || ids.isNull()) {
                    property = null;
                } else {
                    JsonArray arr = ids.asArray();
                    property = new EventInfraGetterNestedArray.LocalInnerEvent[arr.size()];
                    for (int i = 0; i < arr.size(); i++) {
                        property[i] = new EventInfraGetterNestedArray.LocalInnerEvent(stringOrNull(arr.get(i)));
                    }
                }
                runtime.getEventService().sendEventBean(new EventInfraGetterNestedArray.LocalEvent(property), eventType);
                return;
            }
            case "nested-simple": {
                boolean present = payload.getBoolean("present", false);
                EventInfraGetterNestedSimple.LocalInnerEvent property =
                        present ? new EventInfraGetterNestedSimple.LocalInnerEvent(stringOrNull(payload.get("id"))) : null;
                runtime.getEventService().sendEventBean(new EventInfraGetterNestedSimple.LocalEvent(property), eventType);
                return;
            }
            case "nested-simple-deep": {
                boolean root = payload.getBoolean("root", false);
                boolean inner = payload.getBoolean("inner", false);
                EventInfraGetterNestedSimpleDeep.LocalEvent event;
                if (root) {
                    event = new EventInfraGetterNestedSimpleDeep.LocalEvent(null);
                } else if (inner) {
                    event = new EventInfraGetterNestedSimpleDeep.LocalEvent(
                            new EventInfraGetterNestedSimpleDeep.LocalInnerEvent(null));
                } else {
                    event = new EventInfraGetterNestedSimpleDeep.LocalEvent(
                            new EventInfraGetterNestedSimpleDeep.LocalInnerEvent(
                                    new EventInfraGetterNestedSimpleDeep.LocalLeafEvent(stringOrNull(payload.get("id")))));
                }
                runtime.getEventService().sendEventBean(event, eventType);
                return;
            }
            case "simple-fragment":
                runtime.getEventService().sendEventBean(new EventInfraGetterSimpleFragment.LocalEvent(
                        payload.getBoolean("value", false) ? new EventInfraGetterSimpleFragment.LocalInnerEvent() : null), eventType);
                return;
            case "simple-no-fragment":
                runtime.getEventService().sendEventBean(new EventInfraGetterSimpleNoFragment.LocalEvent(
                        stringOrNull(payload.get("property"))), eventType);
                return;
            default:
                throw new IllegalArgumentException("no bean sender for " + caseName);
        }
    }

    /** Rebuilds the exact map the Java sender passes to sendEventMap. */
    private static Map<String, Object> toMapPayload(String caseName, JsonObject payload) {
        switch (caseName) {
            case "nested-array": {
                JsonValue ids = payload.get("ids");
                Map[] property;
                if (ids == null || ids.isNull()) {
                    property = null;
                } else {
                    JsonArray arr = ids.asArray();
                    property = new Map[arr.size()];
                    for (int i = 0; i < arr.size(); i++) {
                        property[i] = Collections.singletonMap("id", stringOrNull(arr.get(i)));
                    }
                }
                return Collections.singletonMap("property", property);
            }
            case "nested-simple": {
                boolean present = payload.getBoolean("present", false);
                Map<String, Object> property = present
                        ? Collections.singletonMap("id", stringOrNull(payload.get("id")))
                        : null;
                return Collections.singletonMap("property", property);
            }
            case "nested-simple-deep": {
                boolean root = payload.getBoolean("root", false);
                boolean inner = payload.getBoolean("inner", false);
                Map<String, Object> event = new LinkedHashMap<>();
                if (!root) {
                    if (inner) {
                        event.put("property", Collections.singletonMap("leaf", null));
                    } else {
                        Map<String, Object> leaf = Collections.singletonMap("id", stringOrNull(payload.get("id")));
                        event.put("property", Collections.singletonMap("leaf", leaf));
                    }
                }
                return event;
            }
            case "simple-fragment": {
                boolean value = payload.getBoolean("value", false);
                return Collections.singletonMap("property", value ? Collections.emptyMap() : null);
            }
            case "simple-no-fragment":
                return Collections.singletonMap("property", stringOrNull(payload.get("property")));
            default:
                throw new IllegalArgumentException("no map sender for " + caseName);
        }
    }

    private static void sendObjectArray(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "nested-array": {
                JsonValue ids = payload.get("ids");
                Object[][] property;
                if (ids == null || ids.isNull()) {
                    // Java encodes the null-array send as Object[][]{null}.
                    property = new Object[][]{null};
                } else {
                    JsonArray arr = ids.asArray();
                    property = new Object[arr.size()][];
                    for (int i = 0; i < arr.size(); i++) {
                        property[i] = new Object[]{stringOrNull(arr.get(i))};
                    }
                }
                runtime.getEventService().sendEventObjectArray(new Object[]{property}, eventType);
                return;
            }
            case "nested-simple": {
                boolean present = payload.getBoolean("present", false);
                Object[] property = present ? new Object[]{stringOrNull(payload.get("id"))} : null;
                runtime.getEventService().sendEventObjectArray(new Object[]{property}, eventType);
                return;
            }
            case "nested-simple-deep": {
                boolean root = payload.getBoolean("root", false);
                boolean inner = payload.getBoolean("inner", false);
                Object[] event = new Object[1];
                if (!root) {
                    if (inner) {
                        event[0] = new Object[]{null};
                    } else {
                        event[0] = new Object[]{new Object[]{stringOrNull(payload.get("id"))}};
                    }
                }
                runtime.getEventService().sendEventObjectArray(event, eventType);
                return;
            }
            case "simple-fragment": {
                boolean value = payload.getBoolean("value", false);
                runtime.getEventService().sendEventObjectArray(new Object[]{value ? new Object[0] : null}, eventType);
                return;
            }
            case "simple-no-fragment":
                runtime.getEventService().sendEventObjectArray(new Object[]{stringOrNull(payload.get("property"))}, eventType);
                return;
            default:
                throw new IllegalArgumentException("no objectarray sender for " + caseName);
        }
    }

    /** Rebuilds the exact JSON document the Java sender writes. */
    private static String toJsonPayload(String caseName, JsonObject payload) {
        JsonObject event = new JsonObject();
        switch (caseName) {
            case "nested-array": {
                JsonValue ids = payload.get("ids");
                JsonValue property;
                if (ids == null || ids.isNull()) {
                    property = Json.NULL;
                } else {
                    JsonArray arr = new JsonArray();
                    for (JsonValue item : ids.asArray()) {
                        JsonObject inner = new JsonObject();
                        if (item.isNull()) {
                            inner.add("id", Json.NULL);
                        } else {
                            inner.add("id", item.asString());
                        }
                        arr.add(inner);
                    }
                    property = arr;
                }
                event.add("property", property);
                return event.toString();
            }
            case "nested-simple": {
                boolean present = payload.getBoolean("present", false);
                if (present) {
                    JsonObject inner = new JsonObject();
                    JsonValue id = payload.get("id");
                    if (id == null || id.isNull()) {
                        inner.add("id", Json.NULL);
                    } else {
                        inner.add("id", id.asString());
                    }
                    event.add("property", inner);
                }
                return event.toString();
            }
            case "nested-simple-deep": {
                boolean root = payload.getBoolean("root", false);
                boolean inner = payload.getBoolean("inner", false);
                if (!root) {
                    if (inner) {
                        event.add("property", new JsonObject().add("leaf", Json.NULL));
                    } else {
                        JsonObject leaf = new JsonObject();
                        JsonValue id = payload.get("id");
                        if (id == null || id.isNull()) {
                            leaf.add("id", Json.NULL);
                        } else {
                            leaf.add("id", id.asString());
                        }
                        event.add("property", new JsonObject().add("leaf", leaf));
                    }
                }
                return event.toString();
            }
            case "simple-fragment": {
                boolean value = payload.getBoolean("value", false);
                event.add("property", value ? new JsonObject() : Json.NULL);
                return event.toString();
            }
            case "simple-no-fragment": {
                JsonValue property = payload.get("property");
                if (property == null || property.isNull()) {
                    event.add("property", Json.NULL);
                } else {
                    event.add("property", property.asString());
                }
                return event.toString();
            }
            default:
                throw new IllegalArgumentException("no json sender for " + caseName);
        }
    }

    private static void sendAvro(EPRuntime runtime, String caseName, String eventType,
                                 JsonObject payload, CaseState state) {
        EPDeployment schemaDeployment = state.deployment;
        Schema schema = null;
        if (schemaDeployment != null) {
            EventType eventTypeObj = runtime.getEventTypeService()
                    .getEventType(schemaDeployment.getDeploymentId(), eventType);
            schema = SupportAvroUtil.getAvroSchema(eventTypeObj);
        }
        switch (caseName) {
            case "nested-array": {
                GenericData.Record event = new GenericData.Record(schema);
                JsonValue ids = payload.get("ids");
                if (ids == null || ids.isNull()) {
                    // Java writes an empty list for the null-array send.
                    event.put("property", Collections.emptyList());
                } else {
                    Schema innerSchema = schema.getField("property").schema().getElementType();
                    Collection<GenericData.Record> arr = new ArrayList<>();
                    for (JsonValue item : ids.asArray()) {
                        GenericData.Record inner = new GenericData.Record(innerSchema);
                        inner.put("id", stringOrNull(item));
                        arr.add(inner);
                    }
                    event.put("property", arr);
                }
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            case "nested-simple":
                // The Java source builds this sender but never invokes it; the
                // scenario carries no avro sends for nested-simple.
                throw new IllegalArgumentException("nested-simple has no pinned avro send");
            case "nested-simple-deep": {
                GenericData.Record event = new GenericData.Record(schema);
                boolean root = payload.getBoolean("root", false);
                boolean inner = payload.getBoolean("inner", false);
                if (!root) {
                    Schema innerSchema = schema.getField("property").schema();
                    GenericData.Record innerRecord = new GenericData.Record(innerSchema);
                    if (!inner) {
                        Schema leafSchema = innerSchema.getField("leaf").schema();
                        GenericData.Record leaf = new GenericData.Record(leafSchema);
                        leaf.put("id", stringOrNull(payload.get("id")));
                        innerRecord.put("leaf", leaf);
                    }
                    event.put("property", innerRecord);
                }
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            case "simple-fragment": {
                GenericData.Record event = new GenericData.Record(schema);
                boolean value = payload.getBoolean("value", false);
                event.put("property", value ? new GenericData.Record(schema.getField("property").schema()) : null);
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            case "simple-no-fragment": {
                String property = stringOrNull(payload.get("property"));
                Schema sendSchema;
                if (property == null) {
                    // The Java sender builds an ad-hoc optionalString record
                    // for the null send instead of the registered schema.
                    sendSchema = record("name").fields().optionalString("property").endRecord();
                } else {
                    sendSchema = schema;
                }
                GenericData.Record event = new GenericData.Record(sendSchema);
                event.put("property", property);
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            default:
                throw new IllegalArgumentException("no avro sender for " + caseName);
        }
    }

    /** Replays the SupportXML.sendXMLEvent sender for the config XML type. */
    private static void sendXML(EPRuntime runtime, String eventType, JsonObject payload) {
        String property = stringOrNull(payload.get("property"));
        String doc = "<" + eventType + (property != null ? " property=\"" + property + "\"" : "") + "/>";
        try {
            DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
            factory.setNamespaceAware(true);
            Document document = factory.newDocumentBuilder().parse(new InputSource(new StringReader(doc)));
            runtime.getEventService().sendEventXMLDOM(document, eventType);
        } catch (Exception e) {
            fail("XML send failed: " + e.getMessage());
        }
    }

    private static String stringOrNull(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    /** Canonical row rendering with sorted property names for a stable order. */
    private static JsonArray rows(String caseName, String statement, EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(caseName, statement, event));
        }
        return array;
    }

    private static JsonObject row(String caseName, String statement, EventBean event) {
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
     * strings/numbers/booleans passthrough, beans via beanToJson (sorted
     * getter-derived properties), EventBean fragments as plain objects, Avro
     * records via their field map, maps as plain objects, arrays and
     * collections as JSON arrays — the same shapes the Go normalizer emits.
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
            for (Object item : (Collection<?>) value) {
                array.add(normalizeNested(item));
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
