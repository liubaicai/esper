/*
 * Java oracle for the event-infra-getter-dynamic parity slice: replays the
 * five EventInfraGetterDynamic* executions (all ord 0, no flags) and emits one
 * JSON trace record per deployed marker, listener invocation and getter probe
 * on stdout. The object-array iterations with no sender verify the null
 * getter in-process before emitting the getter record; the Avro dynamic
 * iterations replay the Java behavior off-trace (schema deploy, s0/s1 deploy,
 * sends, assertions) before emitting the unrepresentable marker, proving the
 * Java contract exists while Go lacks the Avro dynamic surface.
 */
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyGetter;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.avro.core.AvroConstant;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.client.scopetest.SupportUpdateListener;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterDynamicIndexexPropertyPredefined;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterDynamicNested;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterDynamicNestedDeep;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterDynamicSimple;
import com.espertech.esper.regressionlib.suite.event.infra.EventInfraGetterDynamicSimplePropertyPredefined;
import com.espertech.esper.common.internal.event.bean.core.BeanEventType;
import com.espertech.esper.regressionlib.support.json.SupportJsonEventTypeUtil;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;

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
import static org.junit.Assert.assertNull;

public final class EventInfraGetterDynamicScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "event-infra-getter-dynamic";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/infra";

    private static final String[] CASES = {
            "dynamic-simple",
            "dynamic-nested",
            "dynamic-nested-deep",
            "indexed-predefined",
            "simple-predefined",
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-23791e63adf0caaff235",
            "java-runtime-c6db3a43dfabaf9c0f86",
            "java-runtime-5ef37219e1c801e38af7",
            "java-runtime-1a68e3e356c953b9c774",
            "java-runtime-0d912287d52a89b8725c",
    };
    private static final String[] EXECUTION_NAMES = {
            "EventInfraGetterDynamicSimple",
            "EventInfraGetterDynamicNested",
            "EventInfraGetterDynamicNestedDeep",
            "EventInfraGetterDynamicIndexexPropertyPredefined",
            "EventInfraGetterDynamicSimplePropertyPredefined",
    };
    private static final String[] STATIC_IDS = {
            "java-aab99c02485fd4c7259e",
            "java-681d397823021b2f5617",
            "java-6d027a1951a0f0c26280",
            "java-d89f3bc0bdc8ad41ac17",
            "java-c869682acfea77412912",
    };

    private static final String EPL_S0 = "@name('s0') select * from LocalEvent";
    private static final String EPL_S1_SIMPLE =
            "@name('s1') select property? as c0, exists(property?) as c1, typeof(property?) as c2 from LocalEvent;\n";
    private static final String EPL_S1_NESTED =
            "@name('s1') select property?.id as c0, exists(property?.id) as c1, typeof(property?.id) as c2 from LocalEvent;\n";
    private static final String EPL_S1_DEEP =
            "@name('s1') select property?.leaf.id as c0, exists(property?.leaf.id) as c1, typeof(property?.leaf.id) as c2 from LocalEvent;\n";
    private static final String EPL_S1_INDEXED =
            "@name('s1') select array[0]? as c0, array[1]? as c1,exists(array[0]?) as c2, exists(array[1]?) as c3, typeof(array[0]?) as c4, typeof(array[1]?) as c5 from LocalEvent;\n";

    private static String pkg(String simpleName) {
        return "com.espertech.esper.regressionlib.suite.event.infra." + simpleName;
    }

    /** Pinned schema EPL per (case, mode). */
    private static String pinnedSchemaEPL(String caseName, String mode) {
        switch (caseName) {
            case "dynamic-simple":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterDynamicSimple$LocalEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEventSubA as " + pkg("EventInfraGetterDynamicSimple$LocalEventSubA") + ";\n";
                    case "map":
                        return "@public @buseventtype create schema LocalEvent();\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalEvent();\n" +
                                "@public @buseventtype create objectarray schema LocalEventSubA (property string) inherits LocalEvent;\n";
                    case "json":
                        return "@public @buseventtype @JsonSchema(dynamic=true) create json schema LocalEvent();\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterDynamicSimple$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@public @buseventtype create avro schema LocalEvent();\n";
                }
                break;
            case "dynamic-nested":
            case "dynamic-nested-deep":
                switch (mode) {
                    case "bean":
                        // NestedDeep pins the Nested file's classes.
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterDynamicNested$LocalEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEventSubA as " + pkg("EventInfraGetterDynamicNested$LocalEventSubA") + ";\n";
                    case "map":
                    case "objectarray":
                    case "json":
                    case "avro":
                        return "@public @buseventtype @JsonSchema(dynamic=true) create " + mode + " schema LocalEvent();\n";
                    case "json-provided":
                        String base = caseName.equals("dynamic-nested")
                                ? "EventInfraGetterDynamicNested$MyLocalJsonProvided"
                                : "EventInfraGetterDynamicNestedDeep$MyLocalJsonProvided";
                        return "@JsonSchema(className='" + pkg(base) + "') @public @buseventtype create json schema LocalEvent();\n";
                }
                break;
            case "indexed-predefined":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalInnerEvent as " + pkg("EventInfraGetterDynamicIndexexPropertyPredefined$LocalInnerEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterDynamicIndexexPropertyPredefined$LocalEvent") + ";\n" +
                                "@public @buseventtype create schema LocalEventSubA as " + pkg("EventInfraGetterDynamicIndexexPropertyPredefined$LocalEventSubA") + ";\n";
                    case "map":
                        return "@public @buseventtype create schema LocalInnerEvent();\n" +
                                "@public @buseventtype create schema LocalEvent(array LocalInnerEvent[]);\n";
                    case "objectarray":
                        return "@public @buseventtype create objectarray schema LocalEvent();\n" +
                                "@public @buseventtype create objectarray schema LocalEventSubA (array string[]) inherits LocalEvent;\n";
                    case "json":
                        return "@public @buseventtype create json schema LocalInnerEvent();\n" +
                                "@public @buseventtype create json schema LocalEvent(array LocalInnerEvent[]);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterDynamicIndexexPropertyPredefined$MyLocalJsonProvided") + "') @public @buseventtype create json schema LocalEvent();\n";
                    case "avro":
                        return "@public @buseventtype create avro schema LocalInnerEvent();\n" +
                                "@public @buseventtype create avro schema LocalEvent(array LocalInnerEvent[]);\n";
                }
                break;
            case "simple-predefined":
                switch (mode) {
                    case "bean":
                        return "@public @buseventtype create schema LocalEvent as " + pkg("EventInfraGetterDynamicSimplePropertyPredefined$LocalEvent") + ";\n";
                    case "map":
                    case "objectarray":
                    case "json":
                    case "avro":
                        return "@name('schema') @buseventtype @public create " + mode + " schema LocalEvent(property string);\n";
                    case "json-provided":
                        return "@JsonSchema(className='" + pkg("EventInfraGetterDynamicSimplePropertyPredefined$MyLocalJsonProvided") + "') @buseventtype @public create json schema LocalEvent();\n";
                }
                break;
        }
        return null;
    }

    private static String pinnedS1(String caseName) {
        switch (caseName) {
            case "dynamic-simple":
            case "simple-predefined":
                return EPL_S1_SIMPLE;
            case "dynamic-nested":
                return EPL_S1_NESTED;
            case "dynamic-nested-deep":
                return EPL_S1_DEEP;
            case "indexed-predefined":
                return EPL_S1_INDEXED;
        }
        return null;
    }

    /** Pinned probe paths per case (the Java getGetter names). */
    private static String[] probes(String caseName) {
        switch (caseName) {
            case "dynamic-simple":
            case "simple-predefined":
                return new String[]{"property?"};
            case "dynamic-nested":
                return new String[]{"property?.id"};
            case "dynamic-nested-deep":
                return new String[]{"property?.leaf.id"};
            case "indexed-predefined":
                return new String[]{"array[0]?", "array[1]?"};
        }
        return new String[0];
    }

    private static final String[] MODES = {"bean", "map", "objectarray", "json", "json-provided", "avro"};

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EventInfraGetterDynamicScenarioOracle <scenario.json>");
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
     * iterations mirrors undeployAll).
     */
    private static void replayCase(JsonObject scenario, String caseName, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
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
                        if ("schema".equals(label)) {
                            String mode = step.getString("mode", "");
                            String pinned = pinnedSchemaEPL(caseName, mode);
                            if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("schema EPL is not pinned for " + caseName + "/" + mode);
                            }
                            EPCompiled compiled = EPCompilerProvider.getCompiler()
                                    .compile(epl, new CompilerArguments(config));
                            schemaCompiled.put("schema", compiled);
                            runtime.getDeploymentService().deploy(compiled, new com.espertech.esper.runtime.client.DeploymentOptions());
                        } else {
                            String pinned = "s0".equals(label) ? EPL_S0 : pinnedS1(caseName);
                            if (pinned == null || !pinned.equals(epl)) {
                                throw new IllegalStateException("deploy " + label + " EPL is not pinned in case " + caseName);
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
                    case "send":
                        sendEvent(runtime, caseName, step.getString("mode", ""),
                                step.getString("eventType", ""), step.get("payload").asObject());
                        emitGetterRecords(caseName, state);
                        break;
                    case "types": {
                        // sender==null iteration: the event type's dynamic
                        // getter is null (assertNull in the Java source).
                        String path = step.getString("name", "");
                        EventType eventType = runtime.getDeploymentService()
                                .getStatement(deployments.get("s0").getDeploymentId(), "s0")
                                .getEventType();
                        EventPropertyGetter getter = eventType.getGetter(path);
                        if (getter != null) {
                            throw new IllegalStateException("getter " + path + " unexpectedly resolvable");
                        }
                        int sequence = sequences.merge("s0:getter", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "getter");
                        record.add("statement", "s0");
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                        record.add("name", path);
                        JsonObject value = new JsonObject();
                        value.add("state", "null");
                        record.add("value", value);
                        records.add(record);
                        break;
                    }
                    case "unrepresentable":
                        verifyAvroDynamic(caseName, config, step.getString("expectError", ""));
                        JsonObject marker = new JsonObject();
                        marker.add("case", caseName);
                        marker.add("operation", "unrepresentable");
                        marker.add("statement", step.getString("statement", ""));
                        marker.add("sequence", 0);
                        marker.add("value", step.getString("expectError", ""));
                        records.add(marker);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
                        schemaCompiled.clear();
                        state.lastS0 = null;
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
        EventBean lastS0;
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
     * (getFragment != null), and asserting the pinned expectations in-process.
     */
    private static void emitGetterRecords(String caseName, CaseState state) {
        EventBean event = state.lastS0;
        if (event == null) {
            throw new IllegalStateException("no s0 event captured for getter probe in " + caseName);
        }
        boolean beanBacked = event.getEventType() instanceof BeanEventType
                || SupportJsonEventTypeUtil.isBeanBackedJson(event.getEventType());
        boolean quirky = "dynamic-simple".equals(caseName);
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
            if (exists) {
                if ("indexed-predefined".equals(caseName)) {
                    probe.add("value", normalizeIndexed(value));
                } else {
                    probe.add("value", normalize(value));
                }
            } else {
                probe.add("value", Json.NULL);
            }
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
            // In-process assertions mirroring assertGetter/assertGetters.
            if ("indexed-predefined".equals(caseName)) {
                assertEquals(exists, value != null);
                assertEquals(beanBacked && exists, fragment != null);
            } else {
                assertNull(fragment);
                if (!quirky && !exists && value != null) {
                    throw new IllegalStateException("getter " + path + " get()!=null while missing");
                }
            }
        }
    }

    /** Indexed probe values render like Java's get()!=null assertions. */
    private static JsonValue normalizeIndexed(Object value) {
        return normalize(value);
    }

    /** sendEvent rebuilds the Java sender for the (case, mode, payload) pin. */
    private static void sendEvent(EPRuntime runtime, String caseName, String mode,
                                  String eventType, JsonObject payload) {
        switch (mode) {
            case "bean":
                sendBean(runtime, caseName, eventType, payload);
                return;
            case "map":
                runtime.getEventService().sendEventMap(toMap(payload), eventType);
                return;
            case "objectarray":
                sendObjectArray(runtime, caseName, eventType, payload);
                return;
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson(payload.toString(), eventType);
                return;
            case "avro":
                sendAvro(runtime, caseName, eventType, payload);
                return;
            default:
                throw new IllegalArgumentException("unknown send mode " + mode);
        }
    }

    private static void sendBean(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "dynamic-simple":
                if ("LocalEvent".equals(eventType)) {
                    runtime.getEventService().sendEventBean(new EventInfraGetterDynamicSimple.LocalEvent(), eventType);
                } else {
                    runtime.getEventService().sendEventBean(
                            new EventInfraGetterDynamicSimple.LocalEventSubA(stringOrNull(payload.get("property"))), eventType);
                }
                return;
            case "dynamic-nested":
                if ("LocalEvent".equals(eventType)) {
                    runtime.getEventService().sendEventBean(new EventInfraGetterDynamicNested.LocalEvent(), eventType);
                } else {
                    JsonObject inner = payload.get("property").asObject();
                    runtime.getEventService().sendEventBean(new EventInfraGetterDynamicNested.LocalEventSubA(
                            new EventInfraGetterDynamicNested.LocalInnerEvent(stringOrNull(inner.get("id")))), eventType);
                }
                return;
            case "dynamic-nested-deep": {
                EventInfraGetterDynamicNestedDeep.LocalEvent event;
                if (payload.isEmpty()) {
                    event = new EventInfraGetterDynamicNestedDeep.LocalEvent();
                } else {
                    JsonObject inner = payload.get("property").asObject();
                    EventInfraGetterDynamicNestedDeep.LocalInnerEvent innerBean;
                    if (inner.get("leaf").isNull()) {
                        innerBean = new EventInfraGetterDynamicNestedDeep.LocalInnerEvent(null);
                    } else {
                        JsonObject leaf = inner.get("leaf").asObject();
                        innerBean = new EventInfraGetterDynamicNestedDeep.LocalInnerEvent(
                                new EventInfraGetterDynamicNestedDeep.LocalLeafEvent(stringOrNull(leaf.get("id"))));
                    }
                    event = new EventInfraGetterDynamicNestedDeep.LocalEventSubA(innerBean);
                }
                runtime.getEventService().sendEventBean(event, "LocalEvent");
                return;
            }
            case "indexed-predefined":
                if ("LocalEvent".equals(eventType)) {
                    runtime.getEventService().sendEventBean(new EventInfraGetterDynamicIndexexPropertyPredefined.LocalEvent(), eventType);
                } else {
                    EventInfraGetterDynamicIndexexPropertyPredefined.LocalInnerEvent[] array;
                    if (payload.get("array") == null || payload.get("array").isNull()) {
                        array = null;
                    } else {
                        JsonArray items = payload.get("array").asArray();
                        array = new EventInfraGetterDynamicIndexexPropertyPredefined.LocalInnerEvent[items.size()];
                        for (int i = 0; i < items.size(); i++) {
                            array[i] = new EventInfraGetterDynamicIndexexPropertyPredefined.LocalInnerEvent();
                        }
                    }
                    runtime.getEventService().sendEventBean(
                            new EventInfraGetterDynamicIndexexPropertyPredefined.LocalEventSubA(array), eventType);
                }
                return;
            case "simple-predefined":
                runtime.getEventService().sendEventBean(new EventInfraGetterDynamicSimplePropertyPredefined.LocalEvent(
                        stringOrNull(payload.get("property"))), eventType);
                return;
            default:
                throw new IllegalArgumentException("no bean sender for " + caseName);
        }
    }

    private static void sendObjectArray(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        if ("dynamic-simple".equals(caseName)) {
            if ("LocalEvent".equals(eventType)) {
                runtime.getEventService().sendEventObjectArray(new Object[0], eventType);
            } else {
                runtime.getEventService().sendEventObjectArray(new Object[]{stringOrNull(payload.get("property"))}, eventType);
            }
            return;
        }
        if ("simple-predefined".equals(caseName)) {
            runtime.getEventService().sendEventObjectArray(new Object[]{stringOrNull(payload.get("property"))}, eventType);
            return;
        }
        throw new IllegalArgumentException("no objectarray sender for " + caseName);
    }

    private static void sendAvro(EPRuntime runtime, String caseName, String eventType, JsonObject payload) {
        switch (caseName) {
            case "indexed-predefined": {
                Schema inner = SchemaBuilder.record("inner").fields().endRecord();
                Schema schema = SchemaBuilder.record("name").fields()
                        .name("array").type(SchemaBuilder.array().items(inner)).noDefault().endRecord();
                GenericData.Record event = new GenericData.Record(schema);
                if (payload.get("array") == null || payload.get("array").isNull()) {
                    event.put("array", Collections.emptyList());
                } else {
                    JsonArray items = payload.get("array").asArray();
                    Collection<GenericData.Record> inners = new ArrayList<>();
                    for (int i = 0; i < items.size(); i++) {
                        inners.add(new GenericData.Record(inner));
                    }
                    event.put("array", inners);
                }
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            case "simple-predefined": {
                Schema schema = SchemaBuilder.record("name").fields().optionalString("property").endRecord();
                GenericData.Record event = new GenericData.Record(schema);
                event.put("property", stringOrNull(payload.get("property")));
                runtime.getEventService().sendEventAvro(event, eventType);
                return;
            }
            default:
                throw new IllegalArgumentException("no avro sender for " + caseName);
        }
    }

    /**
     * Replays the Java Avro dynamic iteration off-trace (schema deploy, s0/s1
     * deploy, sends and the pinned assertions) proving the Java contract
     * before the unrepresentable marker is emitted.
     */
    private static void verifyAvroDynamic(String caseName, Configuration config, String note) throws Exception {
        if (note == null || note.isEmpty()) {
            throw new IllegalStateException("unrepresentable note is not pinned");
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-verify-" + caseName, config);
        try {
            String schemaEpl = pinnedSchemaEPL(caseName, "avro");
            EPCompiled schema = EPCompilerProvider.getCompiler().compile(schemaEpl, new CompilerArguments(config));
            CompilerArguments args = new CompilerArguments(config);
            args.getPath().add(schema);
            runtime.getDeploymentService().deploy(schema, new com.espertech.esper.runtime.client.DeploymentOptions());
            EPDeployment s0dep = runtime.getDeploymentService().deploy(
                    EPCompilerProvider.getCompiler().compile(EPL_S0, args),
                    new com.espertech.esper.runtime.client.DeploymentOptions());
            EPStatement s0 = runtime.getDeploymentService().getStatement(s0dep.getDeploymentId(), "s0");
            EPDeployment s1dep = runtime.getDeploymentService().deploy(
                    EPCompilerProvider.getCompiler().compile(pinnedS1(caseName), args),
                    new com.espertech.esper.runtime.client.DeploymentOptions());
            EPStatement s1 = runtime.getDeploymentService().getStatement(s1dep.getDeploymentId(), "s1");
            SupportUpdateListener l0 = new SupportUpdateListener();
            SupportUpdateListener l1 = new SupportUpdateListener();
            s0.addListener(l0);
            s1.addListener(l1);

            // Only dynamic-simple's Avro iteration asserts exists()=true on
            // absent (beanBackedJsonOrAvro=true in the source); nested/deep
            // assert plain exists and are wrapped in assertThat.
            boolean avroAlwaysExists = "dynamic-simple".equals(caseName);
            // {property='a'} -> exists true, value 'a', typeof String.
            l0.reset(); l1.reset();
            sendAvroDynamic(runtime, caseName, "a", false);
            assertDynamicRow(caseName, l0, l1, true, "a", avroAlwaysExists);
            // {property=null} -> exists true, value null, typeof null.
            l0.reset(); l1.reset();
            sendAvroDynamic(runtime, caseName, null, false);
            assertDynamicRow(caseName, l0, l1, true, null, avroAlwaysExists);
            // absent / {leaf=null} for deep.
            if ("dynamic-nested-deep".equals(caseName)) {
                l0.reset(); l1.reset();
                sendAvroDynamicDeepLeafNull(runtime);
                assertDynamicRow(caseName, l0, l1, false, null, avroAlwaysExists);
            }
            l0.reset(); l1.reset();
            sendAvroDynamic(runtime, caseName, null, true);
            assertDynamicRow(caseName, l0, l1, avroAlwaysExists, null, avroAlwaysExists);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void assertDynamicRow(String caseName, SupportUpdateListener l0, SupportUpdateListener l1,
                                         boolean exists, String value, boolean beanBackedOrAvro) {
        if (l0.getNewDataList().size() != 1 || l1.getNewDataList().size() != 1) {
            throw new IllegalStateException(caseName + " avro verify: unexpected listener counts");
        }
        EventBean s0event = l0.getLastNewData()[0];
        EventBean s1event = l1.getLastNewData()[0];
        for (String path : probes(caseName)) {
            EventPropertyGetter getter = s0event.getEventType().getGetter(path);
            if (getter == null) {
                throw new IllegalStateException(caseName + " avro verify: getter " + path + " missing");
            }
            boolean actual = getter.isExistsProperty(s0event);
            boolean expected = beanBackedOrAvro || exists;
            assertEquals(expected, actual);
            assertEquals(value, getter.get(s0event));
            assertNull(getter.getFragment(s0event));
        }
        assertEquals(value, s1event.get("c0"));
        boolean expectedExists = beanBackedOrAvro || exists;
        assertEquals(expectedExists, s1event.get("c1"));
        assertEquals(value != null ? "String" : null, s1event.get("c2"));
    }

    private static void sendAvroDynamic(EPRuntime runtime, String caseName, String value, boolean absent) {
        switch (caseName) {
            case "dynamic-simple": {
                Schema schema = SchemaBuilder.record("name").fields().name("property").type()
                        .unionOf().stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                        .endString().and().nullType().endUnion().noDefault().endRecord();
                GenericData.Record event = new GenericData.Record(schema);
                if (!absent && value != null) {
                    event.put("property", value);
                }
                runtime.getEventService().sendEventAvro(event, "LocalEvent");
                return;
            }
            case "dynamic-nested": {
                Schema innerSchema = SchemaBuilder.record("inner").fields().name("id").type()
                        .stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                        .endString().noDefault().endRecord();
                Schema schema = SchemaBuilder.record("name").fields().name("property").type(innerSchema).noDefault().endRecord();
                GenericData.Record event = new GenericData.Record(schema);
                if (!absent) {
                    GenericData.Record inner = new GenericData.Record(innerSchema);
                    inner.put("id", value);
                    event.put("property", inner);
                }
                runtime.getEventService().sendEventAvro(event, "LocalEvent");
                return;
            }
            case "dynamic-nested-deep": {
                Schema leafSchema = SchemaBuilder.record("leaf").fields().name("id").type()
                        .stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                        .endString().noDefault().endRecord();
                Schema innerSchema = SchemaBuilder.record("inner").fields().name("leaf").type(leafSchema).noDefault().endRecord();
                Schema topSchema = SchemaBuilder.record("top").fields().name("property").type(innerSchema).noDefault().endRecord();
                GenericData.Record event = new GenericData.Record(topSchema);
                if (!absent) {
                    GenericData.Record leaf = new GenericData.Record(leafSchema);
                    leaf.put("id", value);
                    GenericData.Record inner = new GenericData.Record(innerSchema);
                    inner.put("leaf", leaf);
                    event.put("property", inner);
                }
                runtime.getEventService().sendEventAvro(event, "LocalEvent");
                return;
            }
            default:
                throw new IllegalArgumentException("no avro-dynamic sender for " + caseName);
        }
    }

    /** Deep {leaf:null} send: property present with a null leaf. */
    private static void sendAvroDynamicDeepLeafNull(EPRuntime runtime) {
        Schema leafSchema = SchemaBuilder.record("leaf").fields().name("id").type()
                .stringBuilder().prop(AvroConstant.PROP_JAVA_STRING_KEY, AvroConstant.PROP_JAVA_STRING_VALUE)
                .endString().noDefault().endRecord();
        Schema innerSchema = SchemaBuilder.record("inner").fields().name("leaf").type(leafSchema).noDefault().endRecord();
        Schema topSchema = SchemaBuilder.record("top").fields().name("property").type(innerSchema).noDefault().endRecord();
        GenericData.Record event = new GenericData.Record(topSchema);
        event.put("property", new GenericData.Record(innerSchema));
        runtime.getEventService().sendEventAvro(event, "LocalEvent");
    }

    private static String stringOrNull(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
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
            return value.asDouble();
        }
        if (value.isArray()) {
            JsonArray array = value.asArray();
            Map<String, Object>[] items = new Map[array.size()];
            for (int i = 0; i < array.size(); i++) {
                items[i] = toMap(array.get(i).asObject());
            }
            return items;
        }
        if (value.isObject()) {
            return toMap(value.asObject());
        }
        return value.toString();
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
        boolean indexedS1 = "indexed-predefined".equals(caseName) && "s1".equals(statement);
        for (String name : names) {
            Object raw = event.get(name);
            if (indexedS1 && ("c0".equals(name) || "c1".equals(name)
                    || "c4".equals(name) || "c5".equals(name))) {
                // The Java assertProps asserts only non-null for these columns;
                // render them as booleans (exists-style) instead of the raw value.
                fields.add(name, raw != null);
            } else {
                fields.add(name, normalize(raw));
            }
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
