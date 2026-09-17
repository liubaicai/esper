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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the named-window consumer parity scenario.
 * Mirrors InfraNamedWindowConsumer ordinals 0-2: InfraNamedWindowConsumerKeepAll
 * (keepall window + irstream select), InfraNamedWindowConsumerLengthWin
 * (length(2) window + aggregate consumer), and InfraNamedWindowConsumerWBatch
 * (expr_batch insert-into chain, EXCLUDEWHENINSTRUMENTED).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own).
 * Every case deploys its whole module at the first deploy step, mirroring the
 * Java execution's single compileDeploy (the named windows and schemas are
 * module-private so later statements only resolve them inside one module);
 * subsequent deploy steps are markers. Listeners attach to 'select' (keepall)
 * and 's0' (lengthwin); wbatch attaches none. Sends dispatch SupportBean beans
 * for keepall/lengthwin and 10000 identical maps for wbatch's send-batch step.
 */
public final class InfraNamedWindowConsumerScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-consumer";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowConsumer.java";

    private static final String[] CASES = {"keepall", "lengthwin", "wbatch"};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-c2d6b88fc4d77c643aab",
            "java-runtime-a707367e42b2736c2fcb",
            "java-runtime-229ba7f65962eb4594a1",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNamedWindowConsumerKeepAll",
            "InfraNamedWindowConsumerLengthWin",
            "InfraNamedWindowConsumerWBatch",
    };
    private static final String[] STATIC_IDS = {
            "java-04f7e86e470bf275affa",
            "java-04f7e86e470bf275affa",
            "java-04f7e86e470bf275affa",
    };
    private static final String[] JAVA_FLAGS = {"EXCLUDEWHENINSTRUMENTED"};
    private static final int EXPECTED_STEPS = 35;

    private static final String KEEPALL_MODULE =
            "@Name('create') create window MyWindow.win:keepall() as SupportBean;\n"
                    + "@Name('insert') insert into MyWindow select * from SupportBean;\n"
                    + "@Name('select') select irstream * from MyWindow;";
    private static final String LENGTHWIN_MODULE =
            "create window MyWindow#length(2) as SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean;\n"
                    + "@name('s0') select theString as c0, sum(intPrimitive) as c1 from MyWindow;";
    private static final String WBATCH_MODULE =
            "@buseventtype @public create schema IncomingEvent(id int);\n"
                    + "create schema RetainedEvent(id int);\n"
                    + "insert into RetainedEvent select * from IncomingEvent#expr_batch(current_count >= 10000);\n"
                    + "create window RetainedEventWindow#keepall as RetainedEvent;\n"
                    + "insert into RetainedEventWindow select * from RetainedEvent;";

    private static final String[][] MODULE_LABELS = {
            {"create", "insert", "select"},
            {"create", "insert", "s0"},
            {"schema-incoming", "schema-retained", "insert-retained", "create-window", "insert-window"},
    };
    private static final String[] MODULE_EPLS = {KEEPALL_MODULE, LENGTHWIN_MODULE, WBATCH_MODULE};
    private static final String[][] LISTENED = {
            {"select"},
            {"s0"},
            {},
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowConsumerScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(index, allSteps, records);
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
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
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
                            break;
                        }
                        deployModule(runtime, compilerArgs, statements, caseIndex,
                                sequences, records);
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
                        sendSupportBean(runtime, object(step.get("payload"), "payload"));
                        break;
                    case "send-batch": {
                        JsonObject payload = object(step.get("payload"), "payload");
                        int count = integer(step, "count");
                        Map<String, Object> event = new HashMap<>();
                        event.put("id", payload.get("id").asInt());
                        for (int i = 0; i < count; i++) {
                            runtime.getEventService().sendEventMap(event, "IncomingEvent");
                        }
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

    /** Compiles and deploys the case's whole module as one unit, mirroring the
     * Java execution's single compileDeploy: the named windows and schemas are
     * module-private so the consumers only resolve them inside one module. */
    private static void deployModule(EPRuntime runtime, CompilerArguments compilerArgs,
                                     Map<String, EPStatement> statements, int caseIndex,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(MODULE_EPLS[caseIndex], compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        compilerArgs.getPath().add(compiled);
        EPStatement[] deployed = deployment.getStatements();
        String[] labels = MODULE_LABELS[caseIndex];
        if (deployed.length != labels.length) {
            throw new IllegalStateException(CASES[caseIndex] + " module produced "
                    + deployed.length + " statements, want " + labels.length);
        }
        for (int index = 0; index < labels.length; index++) {
            EPStatement statement = deployed[index];
            for (String listened : LISTENED[caseIndex]) {
                if (listened.equals(statement.getName())) {
                    statement.addListener(listener(CASES[caseIndex], sequences, records, runtime));
                }
            }
            statements.put(labels[index], statement);
        }
    }

    private static void sendSupportBean(EPRuntime runtime, JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setTheString(payload.get("theString").asString());
        bean.setIntPrimitive(payload.get("intPrimitive").asInt());
        runtime.getEventService().sendEventBean(bean, "SupportBean");
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
        offset = validateKeepAll(steps, offset);
        offset = validateLengthWin(steps, offset);
        offset = validateWBatch(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateKeepAll(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "keepall");
        String[][] deploys = {
                {"create", "@Name('create') create window MyWindow.win:keepall() as SupportBean"},
                {"insert", "@Name('insert') insert into MyWindow select * from SupportBean"},
                {"select", "@Name('select') select irstream * from MyWindow"},
        };
        for (String[] deploy : deploys) {
            validateDeploy(steps.get(offset++), "keepall", deploy[0], deploy[1]);
            validateDeployed(steps.get(offset++), "keepall", deploy[0]);
        }
        validateSupportBeanSend(steps.get(offset++), "keepall", "E1", 10);
        validateSupportBeanSend(steps.get(offset++), "keepall", "E2", 10);
        validateUndeployAll(steps.get(offset++), "keepall");
        return offset;
    }

    private static int validateLengthWin(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "lengthwin");
        String[][] deploys = {
                {"create", "create window MyWindow#length(2) as SupportBean"},
                {"insert", "insert into MyWindow select * from SupportBean"},
                {"s0", "@name('s0') select theString as c0, sum(intPrimitive) as c1 from MyWindow"},
        };
        for (String[] deploy : deploys) {
            validateDeploy(steps.get(offset++), "lengthwin", deploy[0], deploy[1]);
            validateDeployed(steps.get(offset++), "lengthwin", deploy[0]);
        }
        validateSupportBeanSend(steps.get(offset++), "lengthwin", "E1", 10);
        validateSupportBeanSend(steps.get(offset++), "lengthwin", "E2", 20);
        validateSupportBeanSend(steps.get(offset++), "lengthwin", "E3", 25);
        validateSupportBeanSend(steps.get(offset++), "lengthwin", "E4", 26);
        validateUndeployAll(steps.get(offset++), "lengthwin");
        return offset;
    }

    private static int validateWBatch(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "wbatch");
        String[][] deploys = {
                {"schema-incoming", "@buseventtype @public create schema IncomingEvent(id int)"},
                {"schema-retained", "create schema RetainedEvent(id int)"},
                {"insert-retained", "insert into RetainedEvent select * from IncomingEvent#expr_batch(current_count >= 10000)"},
                {"create-window", "create window RetainedEventWindow#keepall as RetainedEvent"},
                {"insert-window", "insert into RetainedEventWindow select * from RetainedEvent"},
        };
        for (String[] deploy : deploys) {
            validateDeploy(steps.get(offset++), "wbatch", deploy[0], deploy[1]);
            validateDeployed(steps.get(offset++), "wbatch", deploy[0]);
        }
        JsonObject step = object(steps.get(offset++), "send-batch step");
        requireFields(step, "op", "case", "eventType", "payload", "count");
        if (!"send-batch".equals(string(step, "op"))
                || !"wbatch".equals(string(step, "case"))
                || !"IncomingEvent".equals(string(step, "eventType"))
                || integer(step, "count") != 10000) {
            throw new IllegalArgumentException("send-batch step is not pinned for wbatch");
        }
        JsonObject payload = object(step.get("payload"), "payload");
        requireFields(payload, "id");
        if (payload.get("id").asInt() != 1) {
            throw new IllegalArgumentException("send-batch payload is not pinned for wbatch");
        }
        validateUndeployAll(steps.get(offset++), "wbatch");
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
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

    private static void validateSupportBeanSend(JsonValue value, String caseName,
                                                String theString, int intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(payload.get("theString").asString())
                || payload.get("intPrimitive").asInt() != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName);
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

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray array = array(value, name);
        if (array.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!expected[index].equals(array.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned at index " + index);
            }
        }
    }

    private static void requireFields(JsonObject object, String... names) {
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("missing field: " + name);
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
            throw new IllegalArgumentException(name + " must be a string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be a number");
        }
        return value.asInt();
    }

    /** Rethrow handler so listener/deploy failures surface instead of logging. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return (handlerContext) -> {
                throw new RuntimeException(handlerContext.getThrowable());
            };
        }
    }
}
