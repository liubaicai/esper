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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the silent-delete named-window parity
 * scenario. Mirrors InfraNamedWindowOnDelete.InfraNamedWindowSilentDeleteOnDelete
 * and InfraNamedWindowSilentDeleteOnDeleteMany: @hint('silent_delete') strips
 * the deleted rows from the named window's own-statement delivery while the
 * on-delete output (deleted rows as new data) and tail-view consumers still
 * observe the delta. Listeners attach to 'create', 'delete' and 'count';
 * deployed markers are emitted at their step positions.
 */
public final class InfraNamedWindowOnDeleteSilentScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-on-delete-silent";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnDelete.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-38dd6f716920e46075c7",
            "java-runtime-6bce45980de6b3f7aa9f"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNamedWindowSilentDeleteOnDelete",
            "InfraNamedWindowSilentDeleteOnDeleteMany"
    };
    private static final String[] STATIC_IDS = {
            "java-06bf0eb71230b3119293",
            "java-06bf0eb71230b3119293"
    };
    private static final String[] CASES = {"silent-delete", "silent-delete-many"};
    private static final int[] ORDINALS = {5, 6};

    // Verbatim transcriptions of the two executions' module EPL (lines 80-83
    // and 51-54 of InfraNamedWindowOnDelete.java): one compileDeploy module
    // per case so the unnamed insert and the on-delete trigger resolve the
    // non-public window.
    private static final String EPL_MODULE =
            "@name('create') create window MyWindow#length(2) as SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean;\n"
                    + "@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow "
                    + "where p00 = theString;\n"
                    + "@name('count') select count(*) as cnt from MyWindow;";
    private static final String EPL_MODULE_MANY =
            "@name('create') create window MyWindow#groupwin(theString)#length(2) as SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean;\n"
                    + "@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow;\n"
                    + "@name('count') select count(*) as cnt from MyWindow;";

    private static final Set<String> LISTENED =
            new HashSet<>(Arrays.asList("create", "delete", "count"));
    private static final int EXPECTED_STEPS = 27;

    private InfraNamedWindowOnDeleteSilentScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOnDeleteSilentScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASES) {
                runCase(caseName, configuration, runtime, allSteps, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
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
                    String epl = string(step, "epl");
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(epl, compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    // The module deploys create/insert/delete/count in EPL
                    // order; the unnamed insert binds by position.
                    String[] labels = {"create", "insert", "delete", "count"};
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != labels.length) {
                        throw new IllegalStateException("module deployment has "
                                + deployed.length + " statements, want " + labels.length);
                    }
                    for (int index = 0; index < deployed.length; index++) {
                        EPStatement statement = deployed[index];
                        if (LISTENED.contains(statement.getName())) {
                            statement.addListener(listener(caseName, sequences, records, runtime));
                        }
                        statements.put(labels[index], statement);
                    }
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
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                    break;
                default:
                    throw new IllegalArgumentException("unknown op: " + operation);
            }
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
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull()
                        ? null : theString.asString());
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                JsonValue p00 = payload.get("p00");
                runtime.getEventService().sendEventBean(new SupportBean_S0(
                        (int) longInteger(payload.get("id"), "id"),
                        p00 == null || p00.isNull() ? null : p00.asString()), type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

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
        offset = validateCase(steps, offset, "silent-delete", EPL_MODULE,
                new String[][]{
                        {"SupportBean", "E1", "1"},
                        {"SupportBean_S0", null, "E1"},
                        {"SupportBean", "E2", "2"},
                        {"SupportBean", "E3", "3"},
                        {"SupportBean", "E4", "4"},
                        {"SupportBean_S0", null, "E4"},
                        {"SupportBean_S0", null, "E3"},
                        {"SupportBean_S0", null, "EX"},
                });
        offset = validateCase(steps, offset, "silent-delete-many", EPL_MODULE_MANY,
                new String[][]{
                        {"SupportBean", "A", "1"},
                        {"SupportBean", "A", "2"},
                        {"SupportBean", "B", "3"},
                        {"SupportBean", "B", "4"},
                        {"SupportBean_S0", null, null},
                });
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Pins one case's step shape: case marker, four deploy/deployed pairs in
     * create/insert/delete/count order, the send sequence, undeploy-all.
     * sends entries are {eventType, theString, intPrimitive-or-p00}.
     */
    private static int validateCase(JsonArray steps, int offset, String caseName,
                                    String moduleEpl, String[][] sends) {
        String[] deployStatements = {"create", "insert", "delete", "count"};
        if (!"case".equals(string(object(steps.get(offset), "case step"), "op"))
                || !caseName.equals(string(object(steps.get(offset), "case step"), "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + caseName);
        }
        offset++;
        JsonObject deploy = object(steps.get(offset), "deploy step");
        if (!"deploy".equals(string(deploy, "op"))
                || !caseName.equals(string(deploy, "case"))
                || !"module".equals(string(deploy, "statement"))
                || !moduleEpl.equals(string(deploy, "epl"))) {
            throw new IllegalArgumentException("module deploy step is not pinned for " + caseName);
        }
        offset++;
        for (String deployStatement : deployStatements) {
            JsonObject deployed = object(steps.get(offset), "deployed step");
            if (!"deployed".equals(string(deployed, "op"))
                    || !caseName.equals(string(deployed, "case"))
                    || !deployStatement.equals(string(deployed, "statement"))) {
                throw new IllegalArgumentException("deployed step is not pinned for "
                        + caseName + "/" + deployStatement);
            }
            offset++;
        }
        for (String[] send : sends) {
            JsonObject step = object(steps.get(offset), "send step");
            if (!"send".equals(string(step, "op"))
                    || !caseName.equals(string(step, "case"))
                    || !send[0].equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("send step is not pinned for " + caseName);
            }
            JsonObject payload = object(step.get("payload"), "send payload");
            if ("SupportBean".equals(send[0])) {
                if (!send[1].equals(string(payload, "theString"))
                        || longInteger(payload.get("intPrimitive"), "intPrimitive")
                                != Long.parseLong(send[2])) {
                    throw new IllegalArgumentException("SupportBean payload is not pinned for "
                            + caseName);
                }
            } else {
                if (longInteger(payload.get("id"), "id") != 0) {
                    throw new IllegalArgumentException("SupportBean_S0 id is not pinned for "
                            + caseName);
                }
                JsonValue p00 = payload.get("p00");
                if (send[2] == null) {
                    if (p00 != null && !p00.isNull()) {
                        throw new IllegalArgumentException("SupportBean_S0 p00 must be absent for "
                                + caseName);
                    }
                } else if (p00 == null || !send[2].equals(p00.asString())) {
                    throw new IllegalArgumentException("SupportBean_S0 p00 is not pinned for "
                            + caseName);
                }
            }
            offset++;
        }
        JsonObject undeploy = object(steps.get(offset), "undeploy-all step");
        if (!"undeploy-all".equals(string(undeploy, "op"))
                || !caseName.equals(string(undeploy, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
        return offset + 1;
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
