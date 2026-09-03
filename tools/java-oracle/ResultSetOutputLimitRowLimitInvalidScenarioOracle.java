import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowLimit ordinal 8. */
public final class ResultSetOutputLimitRowLimitInvalidScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-limit-invalid";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowLimit ordinal 8: invalid variable row-limit compile diagnostics.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java";
    private static final String CASE = "invalid-variable-row-limit";
    private static final String RUNTIME = "java-runtime-1d2703c5e4976fa0dd90";
    private static final String EXECUTION = "ResultSetInvalid";
    private static final String STATIC_ID = "java-b2d63277ca12f283120d";
    private static final String VARIABLE_EPL = "@public create variable string myrows = 'abc'";
    private static final String[] STATEMENTS = {
            "limit-myrows", "offset-myrows", "limit-dummy", "offset-dummy"};
    private static final String[] EPLS = {
            "select * from SupportBean limit myrows",
            "select * from SupportBean limit 1, myrows",
            "select * from SupportBean limit dummy",
            "select * from SupportBean limit 1,dummy"};
    private static final String[] EXPECTED_ERRORS = {
            "Limit clause requires a variable of numeric type [select * from SupportBean limit myrows]",
            "Limit clause requires a variable of numeric type [select * from SupportBean limit 1, myrows]",
            "Limit clause variable by name 'dummy' has not been declared [select * from SupportBean limit dummy]",
            "Limit clause variable by name 'dummy' has not been declared [select * from SupportBean limit 1,dummy]"};

    private ResultSetOutputLimitRowLimitInvalidScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowLimitInvalidScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validateScenario(scenario);

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIME, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        JsonArray records = new JsonArray();
        try {
            List<EPCompiled> path = new ArrayList<>();
            EPCompiled variable = EPCompilerProvider.getCompiler().compile(
                    VARIABLE_EPL, compilerArguments(configuration, path));
            path.add(variable);
            runtime.getDeploymentService().deploy(variable,
                    new DeploymentOptions().setDeploymentId(ID));
            replay(scenario.get("steps").asArray(), scenario.get("cases").asArray(),
                    configuration, path, runtime, records);
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
        if (records.size() != STATEMENTS.length) {
            throw new IllegalStateException("expected exactly four compile-rejected records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void replay(JsonArray steps, JsonArray cases, Configuration configuration,
                               List<EPCompiled> path, EPRuntime runtime, JsonArray records) {
        if (cases.size() != 1 || !CASE.equals(object(cases.get(0), "case").getString("case", ""))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }
        boolean active = false;
        int sequence = 0;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                active = CASE.equals(string(step, "case"));
                continue;
            }
            if (!active) {
                continue;
            }
            if (!"build-error".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation);
            }
            String label = string(step, "statement");
            int statementIndex = indexOf(STATEMENTS, label);
            if (statementIndex < 0 || !EPLS[statementIndex].equals(string(step, "epl"))) {
                throw new IllegalArgumentException("build-error probe is not pinned: " + label);
            }
            String expected = string(step, "expectError");
            if (!EXPECTED_ERRORS[statementIndex].equals(expected)) {
                throw new IllegalArgumentException("expected diagnostic is not pinned: " + label);
            }
            String actual;
            try {
                EPCompilerProvider.getCompiler().compile(step.getString("epl", ""),
                        compilerArguments(configuration, path));
                actual = "<no-error>";
            } catch (EPCompileException ex) {
                actual = ex.getMessage();
            }
            if (!expected.equals(actual)) {
                throw new IllegalStateException("compile diagnostic drift for " + label
                        + ": expected [" + expected + "] got [" + actual + "]");
            }
            records.add(new JsonObject().add("case", CASE).add("operation", "compile-rejected")
                    .add("statement", label).add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("value", actual));
        }
        if (sequence != STATEMENTS.length) {
            throw new IllegalArgumentException("expected exactly four build-error probes");
        }
    }

    private static CompilerArguments compilerArguments(Configuration configuration, List<EPCompiled> path) {
        CompilerArguments arguments = new CompilerArguments(configuration);
        arguments.getPath().getCompileds().addAll(path);
        return arguments;
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version")) || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{EXECUTION}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(string(definition, "case")) || integer(definition, "ordinal") != 8
                || !RUNTIME.equals(string(definition, "runtimeId"))
                || !EXECUTION.equals(string(definition, "executionName"))
                || !"compile-only".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPLS[0].equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 1 + STATEMENTS.length) {
            throw new IllegalArgumentException("scenario must contain exactly five steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }
        for (int index = 0; index < STATEMENTS.length; index++) {
            JsonObject step = object(steps.get(index + 1), "build-error step");
            requireFields(step, "op", "statement", "epl", "expectError");
            if (!"build-error".equals(string(step, "op"))
                    || !STATEMENTS[index].equals(string(step, "statement"))
                    || !EPLS[index].equals(string(step, "epl"))
                    || !EXPECTED_ERRORS[index].equals(string(step, "expectError"))) {
                throw new IllegalArgumentException("build-error step is not pinned at index " + index);
            }
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expected) {
        if (object == null || object.size() != expected.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expected)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!actual.get(index).isString() || !expected[index].equals(actual.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned");
            }
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber) || !value.toString().matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return value.asInt();
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be an array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static int indexOf(String[] values, String wanted) {
        for (int index = 0; index < values.length; index++) {
            if (values[index].equals(wanted)) {
                return index;
            }
        }
        return -1;
    }
}
