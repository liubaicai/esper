/**
 * Java oracle for the event-infra-545 parity slice: the contained quad
 * (EventInfraContainedSimple/Nested/NestedArray/IndexedWithIndex, flags [],
 * six underlyings each), EventInfraEventRenderer (seven underlyings,
 * whitespace-stripped JSON/XML render pins), EventInfraEventSender
 * (OBSERVEROPS; send/route happy paths with the Java assertion-message probes
 * emitted as unrepresentable records whose note carries the pinned message),
 * EventInfraManufacturer (STATICHOOK; forge SPI exercised for real with
 * observable make/makeUnderlying rows) and EventInfraSuperType (OBSERVEROPS;
 * four-statement dispatch-flag matrix; Json inherits verified in-process).
 */
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventSender;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeObjectArray;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeXMLDOM;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.render.JSONEventRenderer;
import com.espertech.esper.common.client.render.XMLEventRenderer;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.common.internal.event.core.EventBeanManufacturer;
import com.espertech.esper.common.internal.event.core.EventBeanManufacturerForge;
import com.espertech.esper.common.internal.event.core.EventTypeSPI;
import com.espertech.esper.common.internal.event.core.EventTypeUtility;
import com.espertech.esper.common.internal.event.core.WriteablePropertyDescriptor;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.regressionlib.support.bean.SupportBean_G;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerImplA;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerInterface;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraContainedIndexedWithIndex;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraContainedNested;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraContainedNestedArray;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraContainedSimple;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraEventRenderer;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraEventSender;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraManufacturer;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraSuperType;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;
import org.w3c.dom.Document;
import org.xml.sax.InputSource;

import javax.xml.parsers.DocumentBuilderFactory;
import java.io.FileReader;
import java.io.StringReader;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.apache.avro.SchemaBuilder.record;
import static com.espertech.esper.common.internal.avro.core.AvroConstant.PROP_JAVA_STRING_KEY;
import static com.espertech.esper.common.internal.avro.core.AvroConstant.PROP_JAVA_STRING_VALUE;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertSame;
import static org.junit.Assert.assertTrue;
import static org.junit.Assert.fail;

public final class EventInfra545ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-infra-545";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra";

    private static final String[] CASES = {
            "contained-simple", "contained-nested", "contained-nested-array",
            "contained-indexed", "renderer", "sender", "manufacturer", "supertype",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-6de635c8b30a4103c24c",
            "java-runtime-7fbe252dc4d607cf0da6",
            "java-runtime-597b6eca244190805083",
            "java-runtime-d0c21881fb79f2ca6b4a",
            "java-runtime-788241891a0cf2f7b34c",
            "java-runtime-87613a44bc6e8ae3ffa1",
            "java-runtime-70823aef36342bc74b8b",
            "java-runtime-c176a2422bef1680520b",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventInfraContainedSimple",
            "EventInfraContainedNested",
            "EventInfraContainedNestedArray",
            "EventInfraContainedIndexedWithIndex",
            "EventInfraEventRenderer",
            "EventInfraEventSender",
            "EventInfraManufacturer",
            "EventInfraSuperType",
    };

    private static final String NOW = Instant.ofEpochMilli(0).toString();
    private static final String EXPECTED_JSON = "{\"myInt\":1,\"myString\":\"abc\",\"nested\":{\"myInsideInt\":10}}";
    private static final String EXPECTED_XML = "<?xmlversion=\"1.0\"encoding=\"UTF-8\"?><root><myInt>1</myInt><myString>abc</myString><nested><myInsideInt>10</myInsideInt></nested></root>";
    private static final String MANUFACTURER_NOTE =
            "EventBeanManufacturerForge/make/makeUnderlying is an internal SPI with no Go boundary; construct-and-assert rows cover the observable assertions";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventInfra545ScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario;
        try (FileReader reader = new FileReader(args[0])) {
            scenario = Json.parse(reader).asObject();
        }
        JsonObject out = new JsonObject();
        out.add("version", VERSION);
        out.add("id", ID);
        out.add("javaCommit", JAVA_COMMIT);
        out.add("javaSource", JAVA_SOURCE);
        JsonArray runtimes = new JsonArray();
        for (String id : RUNTIME_IDS) {
            runtimes.add(id);
        }
        out.add("javaRuntimes", runtimes);
        JsonArray names = new JsonArray();
        for (String name : EXECUTION_NAMES) {
            names.add(name);
        }
        out.add("javaNames", names);
        out.add("java", System.getProperty("java.version"));

        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            replayCase(scenario, caseName, records);
        }
        out.add("records", records);
        System.out.println(out.toString());
    }

    private static void replayCase(JsonObject scenario, String caseName, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        config.getCommon().getEventMeta().setEnableXMLXSD(true);
        configure545(config);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            State state = new State();
            state.records = records;
            state.sequences = new HashMap<>();
            state.runtime = runtime;
            boolean inCase = false;
            for (JsonValue stepValue : scenario.get("steps").asArray()) {
                JsonObject step = stepValue.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (op) {
                    case "deploy": {
                        String statement = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        String mode = step.getString("mode", "");
                        if (!mode.isEmpty()) {
                            state.mode = mode;
                        }
                        if (epl.isEmpty()) {
                            break;
                        }
                        // RegressionPath semantics: the shared compiler path
                        // accumulates every deployed module so later modules
                        // see earlier-created event types at compile and
                        // deploy time.
                        if (caseName.startsWith("contained")) {
                            // Contained bean/nested types only resolve inside
                            // the declaring module's generated code, so the
                            // schema + selects deploy as one module at the
                            // last select step (mirroring env.compileDeploy).
                            if ("schema".equals(statement)) {
                                state.schemaEpl = epl;
                                break;
                            }
                            state.selectEpls.put(statement, epl);
                            boolean last = "s1".equals(statement)
                                    || !"nested-array".equals(caseName.substring(10))
                                    && !"indexed".equals(caseName.substring(10));
                            if (!last) {
                                break;
                            }
                            StringBuilder module = new StringBuilder(state.schemaEpl);
                            for (String select : state.selectEpls.values()) {
                                module.append(select);
                            }
                            epl = module.toString();
                        }
                        CompilerArguments args = new CompilerArguments(config);
                        args.setPath(state.path);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, args);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        if ("schema".equals(statement)) {
                            // Only schema-declaring modules join the path,
                            // mirroring env.compileDeploy(epl, path): the
                            // sender's trigger/select redeploys run without
                            // it.
                            state.path.add(compiled);
                        }
                        state.deployments.put(statement, deployment);
                        for (EPStatement stmt : deployment.getStatements()) {
                            if (!"trigger".equals(stmt.getName())) {
                                stmt.addListener(listener(caseName, state));
                            }
                            if ("trigger".equals(stmt.getName())) {
                                stmt.addListener((n, o, s0, rt) -> {
                                    if (state.routeUnderlying != null) {
                                        runtime.getEventService()
                                                .getEventSender(state.routeType)
                                                .routeEvent(state.routeUnderlying);
                                    }
                                });
                            }
                        }
                        break;
                    }
                    case "deployed": {
                        String statement = step.getString("statement", "");
                        int sequence = state.sequences.merge(statement + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", statement);
                        record.add("sequence", sequence);
                        record.add("time", NOW);
                        records.add(record);
                        break;
                    }
                    case "send":
                        send(runtime, caseName, state, step);
                        break;
                    case "value":
                        emitValue(runtime, caseName, state, step);
                        break;
                    case "unrepresentable":
                        emitUnrepresentable(runtime, caseName, state, step);
                        break;
                    case "undeploy": {
                        String statement = step.getString("statement", "");
                        EPDeployment deployment = state.deployments.remove(statement);
                        if (deployment == null) {
                            throw new IllegalStateException("no deployment tracked for " + statement);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        state.deployments.clear();
                        state.fired.clear();
                        state.selectEpls.clear();
                        state.schemaEpl = null;
                        state.path = new com.espertech.esper.compiler.client.CompilerPath();
                        state.lastEvent = null;
                        state.mode = null;
                        state.sequences.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + op);
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

    private static final class State {
        JsonArray records;
        Map<String, Integer> sequences;
        Map<String, EPDeployment> deployments = new LinkedHashMap<>();
        Map<String, Boolean> fired = new HashMap<>();
        Map<String, String> selectEpls = new LinkedHashMap<>();
        String schemaEpl;
        String routeType;
        Object routeUnderlying;
        com.espertech.esper.compiler.client.CompilerPath path = new com.espertech.esper.compiler.client.CompilerPath();
        EventBean lastEvent;
        String mode;
        EPRuntime runtime;
    }

    /** Listener emitting one record per invocation per statement. */
    private static UpdateListener listener(String caseName, State state) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            if ("supertype".equals(caseName)) {
                state.fired.put(statement.getName(), true);
                return;
            }
            int sequence = state.sequences.merge(statement.getName() + ":listener", 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", NOW);
            JsonArray rows = new JsonArray();
            for (EventBean event : newEvents) {
                JsonObject row = new JsonObject();
                row.add("kind", "row");
                JsonObject fields = new JsonObject();
                if (caseName.startsWith("contained")) {
                    // Contained listeners emit the element's id.
                    fields.add("id", String.valueOf(event.get("id")));
                } else {
                    fields.add("delivered", true);
                }
                row.add("fields", fields);
                rows.add(row);
                state.lastEvent = event;
            }
            record.add("new", rows);
            state.records.add(record);
        };
    }

    private static void send(EPRuntime runtime, String caseName, State state, JsonObject step) throws Exception {
        String eventType = step.getString("eventType", "");
        String mode = step.getString("mode", "");
        JsonObject payload = step.get("payload") != null ? step.get("payload").asObject() : new JsonObject();
        if (caseName.startsWith("contained")) {
            sendContained(runtime, caseName.substring("contained-".length()), mode, payload);
            return;
        }
        if ("supertype".equals(caseName)) {
            sendSuperType(runtime, eventType);
            int sequence = state.sequences.merge(eventType + ":dispatch", 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "dispatch");
            record.add("statement", eventType);
            record.add("sequence", sequence);
            record.add("time", NOW);
            record.add("value", flagsJson(caseName, state, eventType));
            state.records.add(record);
            return;
        }
        if ("renderer".equals(caseName)) {
            sendRenderer(runtime, mode, eventType);
            return;
        }
        if ("sender".equals(caseName)) {
            String routeOp = payload.getString("op", "send");
            String sendMode = payload.getString("mode", "");
            if (sendMode.isEmpty()) {
                sendMode = state.mode;
            }
            Object underlying = senderUnderlying(runtime, sendMode, eventType);
            if ("route".equals(routeOp)) {
                // routeEvent delivers only inside an active dispatch, so the
                // trigger statement's listener routes (mirroring Java).
                state.routeType = eventType;
                state.routeUnderlying = underlying;
                runtime.getEventService().sendEventMap(Collections.emptyMap(), "TriggerEvent");
                state.routeType = null;
                state.routeUnderlying = null;
            } else {
                runtime.getEventService().getEventSender(eventType).sendEvent(underlying);
            }
            return;
        }
        throw new IllegalArgumentException("send for case " + caseName);
    }

    private static final boolean[][] SUPERTYPE_EXPECTED = {
            {true, false, false, false},
            {true, true, false, false},
            {true, false, true, false},
            {true, false, true, true},
    };

    private static JsonArray flagsJson(String caseName, State state, String sentType) {
        // The dispatch record carries the four invocation flags in s0..s3
        // order and asserts the pinned Java matrix row in-process.
        int sentIndex = "Type_1".equals(suffix(sentType)) ? 1
                : "Type_2".equals(suffix(sentType)) ? 2
                : "Type_2_1".equals(suffix(sentType)) ? 3 : 0;
        JsonArray arr = new JsonArray();
        for (int i = 0; i < 4; i++) {
            boolean flag = state.fired.getOrDefault("s" + i, false);
            assertEquals("dispatch flag s" + i + " for " + sentType,
                    SUPERTYPE_EXPECTED[sentIndex][i], flag);
            arr.add(flag);
        }
        state.fired.clear();
        return arr;
    }

    private static String suffix(String typeName) {
        return typeName.substring(typeName.indexOf('_') + 1);
    }

    private static void sendContained(EPRuntime runtime, String family, String mode, JsonObject payload) {
        JsonArray ids = payload.get("ids").asArray();
        switch (mode) {
            case "bean":
                sendContainedBean(runtime, family, ids);
                return;
            case "map":
                runtime.getEventService().sendEventMap(containedMap(family, ids), "LocalEvent");
                return;
            case "objectarray":
                runtime.getEventService().sendEventObjectArray(containedObjectArray(family, ids), "LocalEvent");
                return;
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson(containedJson(family, ids), "LocalEvent");
                return;
            case "avro": {
                EventType type = runtime.getEventTypeService().getBusEventType("LocalEvent");
                Schema schema = SupportAvroUtil.getAvroSchema(type);
                runtime.getEventService().sendEventAvro(containedAvro(family, ids, schema), "LocalEvent");
                return;
            }
            default:
                throw new IllegalArgumentException("unknown contained mode " + mode);
        }
    }

    private static void sendContainedBean(EPRuntime runtime, String family, JsonArray ids) {
        switch (family) {
            case "simple":
                runtime.getEventService().sendEventBean(
                        new EventInfraContainedSimple.LocalEvent(
                                new EventInfraContainedSimple.LocalInnerEvent(ids.get(0).asString())),
                        "LocalEvent");
                return;
            case "nested":
                runtime.getEventService().sendEventBean(
                        new EventInfraContainedNested.LocalEvent(
                                new EventInfraContainedNested.LocalInnerEvent(
                                        new EventInfraContainedNested.LocalLeafEvent(ids.get(0).asString()))),
                        "LocalEvent");
                return;
            case "nested-array": {
                EventInfraContainedNestedArray.LocalInnerEvent[] property =
                        new EventInfraContainedNestedArray.LocalInnerEvent[ids.size()];
                for (int i = 0; i < ids.size(); i++) {
                    property[i] = new EventInfraContainedNestedArray.LocalInnerEvent(
                            new EventInfraContainedNestedArray.LocalLeafEvent(ids.get(i).asString()));
                }
                runtime.getEventService().sendEventBean(
                        new EventInfraContainedNestedArray.LocalEvent(property), "LocalEvent");
                return;
            }
            case "indexed": {
                EventInfraContainedIndexedWithIndex.LocalInnerEvent[] indexed =
                        new EventInfraContainedIndexedWithIndex.LocalInnerEvent[ids.size()];
                for (int i = 0; i < ids.size(); i++) {
                    indexed[i] = new EventInfraContainedIndexedWithIndex.LocalInnerEvent(ids.get(i).asString());
                }
                runtime.getEventService().sendEventBean(
                        new EventInfraContainedIndexedWithIndex.LocalEvent(indexed), "LocalEvent");
                return;
            }
        }
        throw new IllegalArgumentException("unknown contained family " + family);
    }

    private static Map<String, Object> containedMap(String family, JsonArray ids) {
        Map<String, Object> event = new LinkedHashMap<>();
        switch (family) {
            case "simple":
                event.put("property", Collections.singletonMap("id", ids.get(0).asString()));
                return event;
            case "nested":
                event.put("property", Collections.singletonMap("leaf",
                        Collections.singletonMap("id", ids.get(0).asString())));
                return event;
            case "nested-array": {
                Map<String, Object>[] elems = new Map[ids.size()];
                for (int i = 0; i < ids.size(); i++) {
                    elems[i] = Collections.singletonMap("leaf",
                            Collections.singletonMap("id", ids.get(i).asString()));
                }
                event.put("property", elems);
                return event;
            }
            case "indexed": {
                Map<String, Object>[] elems = new Map[ids.size()];
                for (int i = 0; i < ids.size(); i++) {
                    elems[i] = Collections.singletonMap("id", ids.get(i).asString());
                }
                event.put("indexed", elems);
                return event;
            }
        }
        throw new IllegalArgumentException("unknown contained family " + family);
    }

    private static Object[] containedObjectArray(String family, JsonArray ids) {
        switch (family) {
            case "simple":
                return new Object[]{new Object[]{ids.get(0).asString()}};
            case "nested":
                return new Object[]{new Object[]{new Object[]{ids.get(0).asString()}}};
            case "nested-array": {
                Object[][] elems = new Object[ids.size()][];
                for (int i = 0; i < ids.size(); i++) {
                    elems[i] = new Object[]{new Object[]{ids.get(i).asString()}};
                }
                return new Object[]{elems};
            }
            case "indexed": {
                Object[][] elems = new Object[ids.size()][];
                for (int i = 0; i < ids.size(); i++) {
                    elems[i] = new Object[]{ids.get(i).asString()};
                }
                return new Object[]{elems};
            }
        }
        throw new IllegalArgumentException("unknown contained family " + family);
    }

    private static String containedJson(String family, JsonArray ids) {
        return containedJsonText(family, ids);
    }

    private static String containedJsonText(String family, JsonArray ids) {
        Map<String, Object> event = new LinkedHashMap<>();
        switch (family) {
            case "simple": {
                Map<String, Object> inner = new LinkedHashMap<>();
                inner.put("id", ids.get(0).asString());
                event.put("property", inner);
                break;
            }
            case "nested": {
                Map<String, Object> leaf = new LinkedHashMap<>();
                leaf.put("id", ids.get(0).asString());
                Map<String, Object> inner = new LinkedHashMap<>();
                inner.put("leaf", leaf);
                event.put("property", inner);
                break;
            }
            case "nested-array": {
                List<Object> elems = new ArrayList<>();
                for (JsonValue id : ids) {
                    Map<String, Object> leaf = new LinkedHashMap<>();
                    leaf.put("id", id.asString());
                    Map<String, Object> inner = new LinkedHashMap<>();
                    inner.put("leaf", leaf);
                    elems.add(inner);
                }
                event.put("property", elems);
                break;
            }
            case "indexed": {
                List<Object> elems = new ArrayList<>();
                for (JsonValue id : ids) {
                    Map<String, Object> inner = new LinkedHashMap<>();
                    inner.put("id", id.asString());
                    elems.add(inner);
                }
                event.put("indexed", elems);
                break;
            }
        }
        StringBuilder json = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, Object> entry : event.entrySet()) {
            if (!first) {
                json.append(',');
            }
            first = false;
            json.append('"').append(entry.getKey()).append("\":").append(toJsonString(entry.getValue()));
        }
        json.append('}');
        return json.toString();
    }

    private static String toJsonString(Object value) {
        if (value instanceof String) {
            return "\"" + value + "\"";
        }
        if (value instanceof Map) {
            StringBuilder out = new StringBuilder("{");
            boolean first = true;
            for (Map.Entry<?, ?> entry : ((Map<?, ?>) value).entrySet()) {
                if (!first) {
                    out.append(',');
                }
                first = false;
                out.append('"').append(entry.getKey()).append("\":").append(toJsonString(entry.getValue()));
            }
            out.append('}');
            return out.toString();
        }
        if (value instanceof List) {
            StringBuilder out = new StringBuilder("[");
            boolean first = true;
            for (Object item : (List<?>) value) {
                if (!first) {
                    out.append(',');
                }
                first = false;
                out.append(toJsonString(item));
            }
            out.append(']');
            return out.toString();
        }
        return String.valueOf(value);
    }

    private static GenericData.Record containedAvro(String family, JsonArray ids, Schema schema) {
        GenericData.Record event = new GenericData.Record(schema);
        switch (family) {
            case "simple": {
                Schema innerSchema = schema.getField("property").schema();
                GenericData.Record inner = new GenericData.Record(innerSchema);
                inner.put("id", ids.get(0).asString());
                event.put("property", inner);
                return event;
            }
            case "nested": {
                Schema innerSchema = schema.getField("property").schema();
                Schema leafSchema = innerSchema.getField("leaf").schema();
                GenericData.Record leaf = new GenericData.Record(leafSchema);
                leaf.put("id", ids.get(0).asString());
                GenericData.Record inner = new GenericData.Record(innerSchema);
                inner.put("leaf", leaf);
                event.put("property", inner);
                return event;
            }
            case "nested-array": {
                Schema innerSchema = schema.getField("property").schema().getElementType();
                Schema leafSchema = innerSchema.getField("leaf").schema();
                List<GenericData.Record> elems = new ArrayList<>();
                for (JsonValue id : ids) {
                    GenericData.Record leaf = new GenericData.Record(leafSchema);
                    leaf.put("id", id.asString());
                    GenericData.Record inner = new GenericData.Record(innerSchema);
                    inner.put("leaf", leaf);
                    elems.add(inner);
                }
                event.put("property", elems);
                return event;
            }
            case "indexed": {
                Schema innerSchema = schema.getField("indexed").schema().getElementType();
                List<GenericData.Record> elems = new ArrayList<>();
                for (JsonValue id : ids) {
                    GenericData.Record inner = new GenericData.Record(innerSchema);
                    inner.put("id", id.asString());
                    elems.add(inner);
                }
                event.put("indexed", elems);
                return event;
            }
        }
        throw new IllegalArgumentException("unknown contained family " + family);
    }

    private static void sendRenderer(EPRuntime runtime, String mode, String eventType) throws Exception {
        switch (mode) {
            case "bean":
                runtime.getEventService().sendEventBean(
                        new EventInfraEventRenderer.MyEvent(1, "abc",
                                new EventInfraEventRenderer.MyInsideEvent(10)), eventType);
                return;
            case "map": {
                Map<String, Object> inner = new HashMap<>();
                inner.put("myInsideInt", 10);
                Map<String, Object> top = new HashMap<>();
                top.put("myInt", 1);
                top.put("myString", "abc");
                top.put("nested", inner);
                runtime.getEventService().sendEventMap(top, eventType);
                return;
            }
            case "objectarray":
                runtime.getEventService().sendEventObjectArray(
                        new Object[]{1, "abc", new Object[]{10}}, eventType);
                return;
            case "xml": {
                Document doc = DocumentBuilderFactory.newInstance().newDocumentBuilder()
                        .parse(new InputSource(new StringReader(
                                "<myevent myInt=\"1\" myString=\"abc\"><nested myInsideInt=\"10\"/></myevent>")));
                runtime.getEventService().sendEventXMLDOM(doc.getDocumentElement(), eventType);
                return;
            }
            case "avro": {
                Schema schema = SupportAvroUtil.getAvroSchema(
                        runtime.getEventTypeService().getEventTypePreconfigured(eventType));
                GenericData.Record inner = new GenericData.Record(schema.getField("nested").schema());
                inner.put("myInsideInt", 10);
                GenericData.Record record = new GenericData.Record(schema);
                record.put("myInt", 1);
                record.put("myString", "abc");
                record.put("nested", inner);
                runtime.getEventService().sendEventAvro(record, eventType);
                return;
            }
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson(
                        "{\n  \"myInt\": 1,\n  \"myString\": \"abc\",\n  \"nested\": {\n    \"myInsideInt\": 10\n  }\n}",
                        eventType);
                return;
        }
        throw new IllegalArgumentException("unknown renderer mode " + mode);
    }

    private static Object senderUnderlying(EPRuntime runtime, String mode, String eventType) throws Exception {
        Object underlying;
        switch (mode) {
            case "bean":
                underlying = new SupportBean();
                break;
            case "marker-impl":
                underlying = new SupportMarkerImplA("Q2");
                break;
            case "marker-g":
                underlying = new SupportBean_G("Q3");
                break;
            case "map":
                underlying = new HashMap<>();
                break;
            case "objectarray":
                underlying = new Object[]{};
                break;
            case "xml":
                underlying = DocumentBuilderFactory.newInstance().newDocumentBuilder()
                        .parse(new InputSource(new StringReader("<myevent/>"))).getDocumentElement();
                break;
            case "avro":
                underlying = new GenericData.Record(SupportAvroUtil.getAvroSchema(
                        runtime.getEventTypeService().getEventTypePreconfigured(eventType)));
                break;
            case "json":
                underlying = "{}";
                break;
            default:
                throw new IllegalArgumentException("unknown sender mode " + mode);
        }
        return underlying;
    }

    private static void sendSuperType(EPRuntime runtime, String eventType) {
        if (eventType.startsWith("Bean_")) {
            Object underlying;
            switch (eventType) {
                case "Bean_Type_Root":
                    underlying = new EventInfraSuperType.Bean_Type_Root();
                    break;
                case "Bean_Type_1":
                    underlying = new EventInfraSuperType.Bean_Type_1();
                    break;
                case "Bean_Type_2":
                    underlying = new EventInfraSuperType.Bean_Type_2();
                    break;
                default:
                    underlying = new EventInfraSuperType.Bean_Type_2_1();
                    break;
            }
            runtime.getEventService().sendEventBean(underlying, eventType);
            return;
        }
        if (eventType.startsWith("Map_")) {
            runtime.getEventService().sendEventMap(new HashMap<>(), eventType);
            return;
        }
        if (eventType.startsWith("OA_")) {
            runtime.getEventService().sendEventObjectArray(new Object[0], eventType);
            return;
        }
        if (eventType.startsWith("Avro_")) {
            runtime.getEventService().sendEventAvro(
                    new GenericData.Record(record("fake").fields().endRecord()), eventType);
            return;
        }
        if (eventType.startsWith("Json_")) {
            runtime.getEventService().sendEventJson("{}", eventType);
            return;
        }
        throw new IllegalArgumentException("unknown supertype event " + eventType);
    }

    private static void emitValue(EPRuntime runtime, String caseName, State state, JsonObject step) throws Exception {
        String name = step.getString("name", "");
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "value");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", state.sequences.merge(step.getString("statement", "") + ":value", 1, Integer::sum));
        record.add("time", NOW);
        record.add("name", name);
        if ("renderer".equals(caseName)) {
            EventBean event = state.lastEvent;
            if (event == null) {
                throw new IllegalStateException("renderer value step has no captured event");
            }
            if ("json".equals(name)) {
                JSONEventRenderer renderer = runtime.getRenderEventService().getJSONRenderer(event.getEventType());
                String json = renderer.render(event).replaceAll("(\\s|\\n|\\t)", "");
                assertEquals(EXPECTED_JSON, json);
                record.add("value", json);
            } else {
                XMLEventRenderer renderer = runtime.getRenderEventService().getXMLRenderer(event.getEventType());
                String xml = renderer.render("root", event).replaceAll("(\\s|\\n|\\t)", "");
                assertEquals(EXPECTED_XML, xml);
                record.add("value", xml);
            }
        } else if ("manufacturer".equals(caseName)) {
            // The manufacturer forge runs for real; only the emitted row is
            // the observable construct-and-assert outcome.
            JsonObject row = manufacturerRow(state.runtime, state.deployments.get("schema"), state.mode);
            record.add("value", row);
        }
        state.records.add(record);
    }

    /** Runs the pinned make/makeUnderlying assertions and returns the row. */
    private static JsonObject manufacturerRow(EPRuntime runtime, EPDeployment deployment, String mode) throws Exception {
        EventTypeSPI type = (EventTypeSPI) deployment.getStatements()[0].getEventType();
        Set<WriteablePropertyDescriptor> writables = EventTypeUtility.getWriteableProperties(type, true, true);
        WriteablePropertyDescriptor[] props = new WriteablePropertyDescriptor[2];
        props[0] = findProp(writables, "p1");
        props[1] = findProp(writables, "p2");
        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
        EventBeanManufacturerForge forge = EventTypeUtility.getManufacturer(type, props,
                spi.getServicesContext().getClasspathImportServiceRuntime(), true,
                spi.getServicesContext().getEventTypeAvroHandler());
        EventBeanManufacturer manufacturer = forge.getManufacturer(spi.getServicesContext().getEventBeanTypedEventFactory());
        EventBean event = manufacturer.make(new Object[]{"a", 1});
        assertSame(event.getEventType(), type);
        Object underlying = manufacturer.makeUnderlying(new Object[]{"a", 1});
        JsonObject row = new JsonObject();
        if ("bean".equals(mode)) {
            EventInfraManufacturer.MyLocalBeanEvent bean =
                    (EventInfraManufacturer.MyLocalBeanEvent) underlying;
            assertEquals("a", bean.getP1());
            assertEquals(1, bean.getP2());
            row.add("p1", bean.getP1());
            row.add("p2", bean.getP2());
        } else if ("avro".equals(mode)) {
            GenericData.Record rec = (GenericData.Record) underlying;
            assertEquals("a", rec.get("p1"));
            assertEquals(1, rec.get("p2"));
            row.add("p1", (String) rec.get("p1"));
            row.add("p2", (Integer) rec.get("p2"));
        } else if ("json-provided".equals(mode)) {
            EventInfraManufacturer.MyLocalJsonProvided received =
                    (EventInfraManufacturer.MyLocalJsonProvided) underlying;
            assertEquals("a", received.p1);
            assertEquals(1, received.p2);
            row.add("p1", received.p1);
            row.add("p2", received.p2);
        } else if ("objectarray".equals(mode)) {
            Object[] arr = (Object[]) underlying;
            assertEquals("a", arr[0]);
            assertEquals(1, arr[1]);
            row.add("p1", (String) arr[0]);
            row.add("p2", (Integer) arr[1]);
        } else {
            Map<?, ?> map = (Map<?, ?>) underlying;
            assertEquals("a", map.get("p1"));
            assertEquals(1, map.get("p2"));
            row.add("p1", (String) map.get("p1"));
            row.add("p2", (Integer) map.get("p2"));
        }
        return row;
    }

    private static WriteablePropertyDescriptor findProp(Set<WriteablePropertyDescriptor> writables, String name) {
        for (WriteablePropertyDescriptor prop : writables) {
            if (prop.getPropertyName().equals(name)) {
                return prop;
            }
        }
        fail();
        return null;
    }

    private static void emitUnrepresentable(EPRuntime runtime, String caseName, State state, JsonObject step) {
        String statement = step.getString("statement", "");
        String note = step.getString("expectError", "");
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", statement);
        record.add("sequence", state.sequences.merge(statement + ":unrepresentable", 1, Integer::sum));
        record.add("time", NOW);
        record.add("value", note);
        if ("sender".equals(caseName)) {
            // Exercise the pinned Java failure in-process: the message text is
            // asserted here while the Go side records only the boundary.
            try {
                verifySenderProbe(runtime, statement, note);
            } catch (AssertionError ae) {
                throw ae;
            } catch (Exception ex) {
                throw new RuntimeException(ex);
            }
        } else if ("manufacturer".equals(caseName)) {
            assertEquals(MANUFACTURER_NOTE, note);
        } else if ("supertype".equals(caseName)) {
            // JSON inherits is unrepresentable on the Go boundary; the oracle
            // still dispatches and verifies the pinned matrix row so the
            // record denotes a verified (not skipped) Java assertion.
            runtime.getEventService().sendEventJson("{}", statement);
            int sentIndex = "Type_1".equals(suffix(statement)) ? 1
                    : "Type_2".equals(suffix(statement)) ? 2
                    : "Type_2_1".equals(suffix(statement)) ? 3 : 0;
            for (int i = 0; i < 4; i++) {
                assertEquals("json-inherits flag s" + i + " for " + statement,
                        SUPERTYPE_EXPECTED[sentIndex][i],
                        state.fired.getOrDefault("s" + i, false));
            }
            state.fired.clear();
        }
        state.records.add(record);
    }

    private static void verifySenderProbe(EPRuntime runtime, String statement, String note) throws Exception {
        if ("ABC".equals(statement)) {
            try {
                runtime.getEventService().getEventSender("ABC");
                fail();
            } catch (Exception ex) {
                assertTrue(ex.getMessage().contains("Event type named 'ABC' could not be found"));
            }
            return;
        }
        // s0 probes: send/route the pinned wrong object per mode. The probe
        // object is identified by the message text's expected-type phrase.
        String typeName = state0TypeForMessage(note);
        EventSender sender = runtime.getEventService().getEventSender(typeName);
        Object wrong = wrongObject(note, runtime);
        try {
            sender.sendEvent(wrong);
            fail();
        } catch (Exception ex) {
            assertTrue("probe message mismatch: " + ex.getMessage() + " != " + note,
                    ex.getMessage() != null && ex.getMessage().equals(note));
        }
        try {
            sender.routeEvent(wrong);
            fail();
        } catch (Exception ex) {
            assertTrue("probe message mismatch: " + ex.getMessage() + " != " + note,
                    ex.getMessage() != null && ex.getMessage().equals(note));
        }
    }

    private static String state0TypeForMessage(String note) {
        if (note.contains("'SupportBean'")) {
            return "SupportBean";
        }
        if (note.contains("java.util.Map")) {
            return EventInfraEventSender.MAP_TYPENAME;
        }
        if (note.contains("Object[]")) {
            return EventInfraEventSender.OA_TYPENAME;
        }
        if (note.contains("Document or Element") || note.contains("root element name")) {
            return EventInfraEventSender.XML_TYPENAME;
        }
        if (note.contains("GenericData.Record")) {
            return EventInfraEventSender.AVRO_TYPENAME;
        }
        if (note.contains("Json-formatted")) {
            return EventInfraEventSender.JSON_TYPENAME;
        }
        throw new IllegalArgumentException("unrecognized probe message: " + note);
    }

    private static Object wrongObject(String note, EPRuntime runtime) throws Exception {
        if (note.contains("root element name")) {
            return DocumentBuilderFactory.newInstance().newDocumentBuilder()
                    .parse(new InputSource(new StringReader("<xxxx/>")));
        }
        if (note.contains("does not equal, extend or implement")) {
            return new SupportBean_G("G1");
        }
        return new SupportBean();
    }

    /**
     * Registers the config-side types mirroring TestSuiteEventInfra's
     * configure() for the eight executions (supertype maps/OA/avro/beans,
     * renderer XML/map/OA/avro, sender XML/map/OA/avro, manufacturer avro,
     * SupportBean marker types).
     */
    private static void configure545(Configuration config) {
        for (Class<?> clazz : new Class<?>[]{SupportBean.class,
                SupportMarkerInterface.class,
                EventInfraEventRenderer.MyEvent.class,
                EventInfraSuperType.Bean_Type_Root.class,
                EventInfraSuperType.Bean_Type_1.class,
                EventInfraSuperType.Bean_Type_2.class,
                EventInfraSuperType.Bean_Type_2_1.class}) {
            config.getCommon().addEventType(clazz);
        }
        // supertype maps
        config.getCommon().addEventType("Map_Type_Root", Collections.emptyMap());
        config.getCommon().addEventType("Map_Type_1", Collections.emptyMap(), new String[]{"Map_Type_Root"});
        config.getCommon().addEventType("Map_Type_2", Collections.emptyMap(), new String[]{"Map_Type_Root"});
        config.getCommon().addEventType("Map_Type_2_1", Collections.emptyMap(), new String[]{"Map_Type_2"});
        // supertype object-array
        config.getCommon().addEventType("OA_Type_Root", new String[0], new Object[0]);
        ConfigurationCommonEventTypeObjectArray oa1 = new ConfigurationCommonEventTypeObjectArray();
        oa1.setSuperTypes(Collections.singleton("OA_Type_Root"));
        config.getCommon().addEventType("OA_Type_1", new String[0], new Object[0], oa1);
        ConfigurationCommonEventTypeObjectArray oa2 = new ConfigurationCommonEventTypeObjectArray();
        oa2.setSuperTypes(Collections.singleton("OA_Type_Root"));
        config.getCommon().addEventType("OA_Type_2", new String[0], new Object[0], oa2);
        ConfigurationCommonEventTypeObjectArray oa21 = new ConfigurationCommonEventTypeObjectArray();
        oa21.setSuperTypes(Collections.singleton("OA_Type_2"));
        config.getCommon().addEventType("OA_Type_2_1", new String[0], new Object[0], oa21);
        // supertype avro
        Schema fake = record("fake").fields().endRecord();
        String[][] avroSupers = {
                {"Avro_Type_Root", null},
                {"Avro_Type_1", "Avro_Type_Root"},
                {"Avro_Type_2", "Avro_Type_Root"},
                {"Avro_Type_2_1", "Avro_Type_2"},
        };
        for (String[] pair : avroSupers) {
            ConfigurationCommonEventTypeAvro meta = new ConfigurationCommonEventTypeAvro();
            meta.setAvroSchema(fake);
            if (pair[1] != null) {
                meta.setSuperTypes(Collections.singleton(pair[1]));
            }
            config.getCommon().addEventTypeAvro(pair[0], meta);
        }
        // sender XML type (root element 'myevent')
        ConfigurationCommonEventTypeXMLDOM senderMeta = new ConfigurationCommonEventTypeXMLDOM();
        senderMeta.setRootElementName("myevent");
        senderMeta.setSchemaText("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
                "\t<xs:element name=\"myevent\">\n" +
                "\t\t<xs:complexType>\n" +
                "\t\t</xs:complexType>\n" +
                "\t</xs:element>\n" +
                "</xs:schema>\n");
        config.getCommon().addEventType(EventInfraEventSender.XML_TYPENAME, senderMeta);
        config.getCommon().addEventType(EventInfraEventSender.MAP_TYPENAME, Collections.emptyMap());
        config.getCommon().addEventType(EventInfraEventSender.OA_TYPENAME, new String[0], new Object[0]);
        config.getCommon().addEventTypeAvro(EventInfraEventSender.AVRO_TYPENAME,
                new ConfigurationCommonEventTypeAvro(SchemaBuilder.record(EventInfraEventSender.AVRO_TYPENAME).fields().endRecord()));
        // renderer XML type (nested element + attributes)
        ConfigurationCommonEventTypeXMLDOM rendererMeta = new ConfigurationCommonEventTypeXMLDOM();
        rendererMeta.setRootElementName("myevent");
        rendererMeta.setSchemaText("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
                "<xs:schema targetNamespace=\"http://www.espertech.com/schema/esper\" elementFormDefault=\"qualified\" xmlns:esper=\"http://www.espertech.com/schema/esper\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
                "\t<xs:element name=\"myevent\">\n" +
                "\t\t<xs:complexType>\n" +
                "\t\t\t<xs:sequence minOccurs=\"0\" maxOccurs=\"unbounded\">\n" +
                "\t\t\t\t<xs:choice>\n" +
                "\t\t\t\t\t<xs:element ref=\"esper:nested\" minOccurs=\"1\" maxOccurs=\"1\"/>\n" +
                "\t\t\t\t</xs:choice>\n" +
                "\t\t\t</xs:sequence>\n" +
                "\t\t\t<xs:attribute name=\"myInt\" type=\"xs:int\" use=\"required\"/>\n" +
                "\t\t\t<xs:attribute name=\"myString\" type=\"xs:string\" use=\"required\"/>\n" +
                "\t\t</xs:complexType>\n" +
                "\t</xs:element>\n" +
                "\t<xs:element name=\"nested\">\n" +
                "\t\t<xs:complexType>\n" +
                "\t\t\t<xs:attribute name=\"myInsideInt\" type=\"xs:int\" use=\"required\"/>\n" +
                "\t\t</xs:complexType>\n" +
                "\t</xs:element>\n" +
                "</xs:schema>\n");
        config.getCommon().addEventType(EventInfraEventRenderer.XML_TYPENAME, rendererMeta);
        Map<String, Object> inner = new LinkedHashMap<>();
        inner.put("myInsideInt", "int");
        Map<String, Object> top = new LinkedHashMap<>();
        top.put("myInt", "int");
        top.put("myString", "string");
        top.put("nested", inner);
        config.getCommon().addEventType(EventInfraEventRenderer.MAP_TYPENAME, top);
        config.getCommon().addEventType(EventInfraEventRenderer.OA_TYPENAME + "_1",
                new String[]{"myInsideInt"}, new Object[]{int.class});
        config.getCommon().addEventType(EventInfraEventRenderer.OA_TYPENAME,
                new String[]{"myInt", "myString", "nested"},
                new Object[]{int.class, String.class, EventInfraEventRenderer.OA_TYPENAME + "_1"});
        Schema rendererInner = SchemaBuilder.record(EventInfraEventRenderer.AVRO_TYPENAME + "_inside")
                .fields().name("myInsideInt").type().intType().noDefault().endRecord();
        Schema rendererSchema = SchemaBuilder.record(EventInfraEventRenderer.AVRO_TYPENAME)
                .fields()
                .name("myInt").type().intType().noDefault()
                .name("myString").type().stringBuilder().prop(PROP_JAVA_STRING_KEY, PROP_JAVA_STRING_VALUE).endString().noDefault()
                .name("nested").type(rendererInner).noDefault()
                .endRecord();
        config.getCommon().addEventTypeAvro(EventInfraEventRenderer.AVRO_TYPENAME,
                new ConfigurationCommonEventTypeAvro(rendererSchema));
        // manufacturer avro
        Schema mfrSchema = SchemaBuilder.record(EventInfraManufacturer.AVRO_TYPENAME)
                .fields()
                .name("p1").type().stringBuilder().prop(PROP_JAVA_STRING_KEY, PROP_JAVA_STRING_VALUE).endString().noDefault()
                .name("p2").type().intType().noDefault()
                .endRecord();
        config.getCommon().addEventTypeAvro(EventInfraManufacturer.AVRO_TYPENAME,
                new ConfigurationCommonEventTypeAvro(mfrSchema));
    }
}
