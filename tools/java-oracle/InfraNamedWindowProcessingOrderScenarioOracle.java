import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.client.scopetest.SupportListener;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the named-window processing-order parity
 * scenario. Mirrors InfraNamedWindowProcessingOrder ordinals 0-6: the six
 * event-representation variants of InfraDispatchBackQueue (OBJECTARRAY, MAP,
 * AVRO, JSON, JSONCLASSPROVIDED, DEFAULT) and InfraOrderedDeleteAndSelect.
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own).
 * Deploy steps compile individually against the accumulating runtime path so
 * later statements resolve the @public schemas and window; the create-window
 * statement deploys like every other EPL. Listeners attach to 'select'
 * (dispatch cases) and 's0' (ordered-delete-select); deployed markers follow
 * each deploy; sends dispatch per the case's event representation exactly as
 * the Java executions do (objectarray value array, empty map, empty-schema
 * Avro record, "{}" JSON, empty map for DEFAULT).
 */
public final class InfraNamedWindowProcessingOrderScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-processing-order";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowProcessingOrder.java";

    private static final String[] CASES = {
            "dispatch-objectarray", "dispatch-map", "dispatch-avro", "dispatch-json",
            "dispatch-json-provided", "dispatch-default", "ordered-delete-select"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5, 6};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d103aeca629813a82acf",
            "java-runtime-0c77245232ebd6281a47",
            "java-runtime-cef00c40d92d73111b6e",
            "java-runtime-21dd5ef783d585d85dfd",
            "java-runtime-59a4da55e98ca67b95c2",
            "java-runtime-99f94152e6a94e3e60bc",
            "java-runtime-c56598034a18ee372891"};
    private static final String[] EXECUTION_NAMES = {
            "InfraDispatchBackQueue{OBJECTARRAY}",
            "InfraDispatchBackQueue{MAP}",
            "InfraDispatchBackQueue{AVRO}",
            "InfraDispatchBackQueue{JSON}",
            "InfraDispatchBackQueue{JSONCLASSPROVIDED}",
            "InfraDispatchBackQueue{DEFAULT}",
            "InfraOrderedDeleteAndSelect"};
    private static final String[] STATIC_IDS = {
            "java-78aff9650bba15e78056", "java-78aff9650bba15e78056",
            "java-78aff9650bba15e78056", "java-78aff9650bba15e78056",
            "java-78aff9650bba15e78056", "java-78aff9650bba15e78056",
            "java-78aff9650bba15e78056"};
    private static final String[] JAVA_FLAGS = {"EXCLUDEWHENINSTRUMENTED"};

    private static final Set<String> LISTENED = new HashSet<>(Arrays.asList("select", "s0"));
    private static final int EXPECTED_STEPS = 138;

    private InfraNamedWindowProcessingOrderScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowProcessingOrderScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            runCase(caseName, allSteps, records);
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
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        compilerArgs.getConfiguration().getCommon().getEventMeta().getAvroSettings()
                .setEnableAvro(true);
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
                        if (statements.containsKey(label)) {
                            // ordered-delete-select deploys its whole module at
                            // the first deploy step (the window is
                            // module-private); later deploy steps are markers.
                            break;
                        }
                        if ("ordered-delete-select".equals(caseName)) {
                            deployOrderedDeleteSelectModule(runtime, compilerArgs, statements,
                                    caseName, sequences, records);
                            break;
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        compilerArgs.getPath().add(compiled);
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("deploy of " + label + " produced "
                                    + deployed.length + " statements, want 1");
                        }
                        EPStatement statement = deployed[0];
                        if (LISTENED.contains(statement.getName())) {
                            statement.addListener(listener(caseName, sequences, records, runtime));
                        }
                        statements.put(label, statement);
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statements.containsKey(label)) {
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
                        sendEvent(runtime, caseName, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "undeploy": {
                        String label = string(step, "statement");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "undeploy targets unknown statement " + label);
                        }
                        String deploymentId = statement.getDeploymentId();
                        runtime.getDeploymentService().undeploy(deploymentId);
                        statements.values().removeIf(
                                registered -> deploymentId.equals(registered.getDeploymentId()));
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
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

    /** Compiles and deploys the ordered-delete-select module as one unit,
     * mirroring the Java execution's single compileDeploy: the named window
     * is module-private so the triggers only resolve it inside one module. */
    private static void deployOrderedDeleteSelectModule(EPRuntime runtime,
            CompilerArguments compilerArgs, Map<String, EPStatement> statements,
            String caseName, Map<String, Integer> sequences, JsonArray records) throws Exception {
        String epl = "create window MyWindow#lastevent as select * from SupportBean;\n"
                + "insert into MyWindow select * from SupportBean;\n"
                + "on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 7;\n"
                + "on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 5;\n"
                + "on MyWindow e insert into ResultStream select e.* from MyWindow;\n"
                + "@name('s0') select * from ResultStream;";
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        compilerArgs.getPath().add(compiled);
        EPStatement[] deployed = deployment.getStatements();
        String[] labels = {"create", "insert", "delete7", "delete5", "oninsert", "s0"};
        if (deployed.length != labels.length) {
            throw new IllegalStateException("ordered-delete-select module produced "
                    + deployed.length + " statements, want " + labels.length);
        }
        for (int index = 0; index < labels.length; index++) {
            EPStatement statement = deployed[index];
            if (LISTENED.contains(statement.getName())) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
            statements.put(labels[index], statement);
        }
    }

    private static void sendEvent(EPRuntime runtime, String caseName, String type,
                                  JsonObject payload) {
        if ("SupportBean".equals(type)) {
            SupportBean bean = new SupportBean();
            bean.setTheString(nullableString(payload, "theString"));
            JsonValue intPrimitive = payload.get("intPrimitive");
            if (intPrimitive != null && intPrimitive.isNumber()) {
                bean.setIntPrimitive((int) longInteger(intPrimitive, "intPrimitive"));
            }
            runtime.getEventService().sendEventBean(bean, type);
            return;
        }
        String repr = string(payload, "repr");
        JsonObject fields = object(payload.get("fields"), "fields");
        String dummy = nullableString(fields, "dummy");
        switch (repr) {
            case "objectarray":
                runtime.getEventService().sendEventObjectArray(new Object[]{dummy}, type);
                break;
            case "map":
            case "default":
                runtime.getEventService().sendEventMap(new HashMap<>(), type);
                break;
            case "avro": {
                // Mirrors the Java execution: an empty-schema GenericData.Record
                // (SchemaBuilder.record("soemthing").fields().endRecord()).
                Schema schema = SchemaBuilder.record("soemthing").fields().endRecord();
                runtime.getEventService().sendEventAvro(new GenericData.Record(schema), type);
                break;
            }
            case "json":
            case "json-provided":
                runtime.getEventService().sendEventJson("{}", type);
                break;
            default:
                throw new IllegalArgumentException("unknown representation: " + repr);
        }
    }

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
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
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object. */
    private static JsonValue normalize(Object value) {
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
        return Json.value(String.valueOf(value));
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
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
        for (int index = 0; index < 6; index++) {
            offset = validateDispatch(steps, offset, CASES[index], REPRS[index]);
        }
        offset = validateOrderedDeleteSelect(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static final String[] REPRS = {
            "objectarray", "map", "avro", "json", "json-provided", "default"};
    private static final String JSON_PROVIDED_BASE =
            "com.espertech.esper.regressionlib.suite.infra.namedwindow."
                    + "InfraNamedWindowProcessingOrder$MyLocalJsonProvided";

    /** Per-statement annotation text mirroring
     * EventRepresentationChoice.getAnnotationTextWJsonProvided: each
     * statement gets its own MyLocalJsonProvided<Name> class. */
    private static String ann(String repr, String provided) {
        switch (repr) {
            case "objectarray":
                return "@EventRepresentation('objectarray') ";
            case "map":
                return "@EventRepresentation('map') ";
            case "avro":
                return "@EventRepresentation('avro') ";
            case "json":
                return "@EventRepresentation('json') ";
            case "json-provided":
                return "@JsonSchema(className='" + JSON_PROVIDED_BASE + provided
                        + "') @EventRepresentation('json') ";
            default:
                // DEFAULT annotationText is empty; Java still concatenates
                // annotationText + " " so the statement gains a leading space.
                return " ";
        }
    }

    /** Exact dispatch-case step sequence mirroring InfraDispatchBackQueue
     * lines 58-103: three annotated create-schema deploys, the insert-into
     * chain, create window, insert into window, on-update, irstream select,
     * two sends, undeploy-all. */
    private static int validateDispatch(JsonArray steps, int offset, String caseName, String repr) {
        validateCaseMarker(steps.get(offset++), caseName);
        String[][] deploys = {
                {"schema-start", ann(repr, "StartValueEvent") + "@buseventtype @public create schema StartValueEvent as (dummy string)"},
                {"schema-forward", ann(repr, "TestForwardEvent") + "@buseventtype @public create schema TestForwardEvent as (prop1 string)"},
                {"schema-input", ann(repr, "TestInputEvent") + "@buseventtype @public create schema TestInputEvent as (dummy string)"},
                {"insert-forward", "insert into TestForwardEvent select'V1' as prop1 from TestInputEvent"},
                {"create", ann(repr, "NamedWin") + "@public create window NamedWin#unique(prop1) (prop1 string, prop2 string)"},
                {"insert-window", "insert into NamedWin select 'V1' as prop1, 'O1' as prop2 from StartValueEvent"},
                {"update", "on TestForwardEvent update NamedWin as work set prop2 = 'U1' where work.prop1 = 'V1'"},
                {"select", "@name('select') select irstream prop1, prop2 from NamedWin"},
        };
        for (String[] deploy : deploys) {
            validateDeploy(steps.get(offset++), caseName, deploy[0], deploy[1]);
            validateDeployed(steps.get(offset++), caseName, deploy[0]);
        }
        validateReprSend(steps.get(offset++), caseName, "StartValueEvent", repr);
        validateReprSend(steps.get(offset++), caseName, "TestInputEvent", repr);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /** Exact ordered-delete-select step sequence mirroring
     * InfraOrderedDeleteAndSelect lines 112-132. */
    private static int validateOrderedDeleteSelect(JsonArray steps, int offset) {
        String caseName = "ordered-delete-select";
        validateCaseMarker(steps.get(offset++), caseName);
        String[][] deploys = {
                {"create", "create window MyWindow#lastevent as select * from SupportBean"},
                {"insert", "insert into MyWindow select * from SupportBean"},
                {"delete7", "on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 7"},
                {"delete5", "on MyWindow e delete from MyWindow win where win.theString=e.theString and e.intPrimitive = 5"},
                {"oninsert", "on MyWindow e insert into ResultStream select e.* from MyWindow"},
                {"s0", "@name('s0') select * from ResultStream"},
        };
        for (String[] deploy : deploys) {
            validateDeploy(steps.get(offset++), caseName, deploy[0], deploy[1]);
            validateDeployed(steps.get(offset++), caseName, deploy[0]);
        }
        validateBeanSend(steps.get(offset++), caseName, "E1", 7);
        validateBeanSend(steps.get(offset++), caseName, "E2", 8);
        validateBeanSend(steps.get(offset++), caseName, "E3", 5);
        validateBeanSend(steps.get(offset++), caseName, "E4", 6);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateReprSend(JsonValue value, String caseName, String eventType,
                                         String repr) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName + "/"
                    + eventType);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        requireFields(payload, "repr", "fields");
        if (!repr.equals(string(payload, "repr"))) {
            throw new IllegalArgumentException("send repr is not pinned for " + caseName + "/"
                    + eventType);
        }
        JsonObject fields = object(payload.get("fields"), "fields");
        if (!"dummyValue".equals(string(fields, "dummy"))) {
            throw new IllegalArgumentException("send fields are not pinned for " + caseName + "/"
                    + eventType);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String theString,
                                         long intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (!theString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName + "/" + theString);
        }
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + caseName);
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

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static void requireFields(JsonObject object, String... names) {
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("missing field: " + name);
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length mismatch");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!expected[index].equals(actual.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned at index " + index);
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
            throw new IllegalArgumentException("missing string field: " + name);
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException("missing numeric field: " + name);
        }
        return (int) Double.parseDouble(value.toString());
    }

    private static long longInteger(JsonValue value, String name) {
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException("missing numeric field: " + name);
        }
        return (long) Double.parseDouble(value.toString());
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        public void init(ExceptionHandlerFactoryContext context) {
        }

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
