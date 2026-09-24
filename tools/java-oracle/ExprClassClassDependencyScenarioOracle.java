import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.regressionlib.support.util.SupportExceptionHandlerFactoryRethrow;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the three ExprClassClassDependency
 * executions (ordinals 0-2) replayed as one differential chain covering
 * inlined_class class-to-class dependencies:
 *
 * all-local (ord 0, ExprClassClassDependencyAllLocal): one statement carries
 * two inlined_class blocks — MyUtil.someFunction wraps |p| and MyClass.doIt
 * calls MyUtil.someFunction(">"+p+"<"). Same-module inlined classes compile
 * in a single Janino batch, so MyClass sees MyUtil regardless of textual
 * order. One SupportBean("E1",1) yields a single listener invocation with
 * c0="|>E1<|" (assertEqualsNew).
 *
 * invalid (ord 1, ExprClassClassDependencyInvalid): compile-only. The
 * @public create inlined_class MyUtil compiles onto the path (env.compile +
 * path.add, never deployed), then two tryInvalidCompile probes pin the Java
 * message prefixes — a statement-local inlined_class and a create
 * inlined_class depending on the path-provided class both fail because
 * inlined-class Janino cannot see path-provided classes. The prefixes differ
 * only by trailing space ("Failed to compile an inlined-class:" vs "...: ").
 *
 * classpath (ord 2, ExprClassClassDependencyClasspath): two independent
 * deploy/undeploy cycles where inlined MyUtil.doIt calls
 * ExprClassClassDependency.supportQuoteString — first by fully-qualified
 * name, then by import (the import line has no trailing newline, so the ';'
 * and "public class MyUtil {" share one source line). Each cycle's
 * SupportBean("E1",1) yields c0="'E1'".
 *
 * Mirroring the regression harness, each case runs on a fresh runtime with
 * the internal timer disabled, compiles the pinned EPL verbatim
 * (inlined_class triple-quote class text, the missing trailing space on the
 * invalid create-class EPL and the import-line quirk included), attaches one
 * s0 listener, sends the pinned SupportBean payloads, then undeploys. The
 * TraceWriter skips null/null listener callbacks; String values render
 * as-is.
 */
public final class ExprClassClassDependencyScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-class-class-dependency";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassClassDependency.java";

    private static final String DESCRIPTION =
            "ExprClassClassDependency ordinals 0-2 (all executions): all-local deploys one statement carrying two "
                    + "inlined_class blocks (MyUtil.someFunction wraps |p|, MyClass.doIt calls "
                    + "MyUtil.someFunction(\">\"+p+\"<\")) and selects MyClass.doIt(theString) as c0, yielding "
                    + "c0=\"|>E1<|\" for SupportBean(\"E1\",1); invalid compiles @public create inlined_class MyUtil "
                    + "onto the path then runs two tryInvalidCompile probes (a statement-local inlined_class and a "
                    + "create inlined_class depending on the path-provided class) that fail because inlined-class "
                    + "Janino cannot see path-provided classes, pinning the prefixes \"Failed to compile an "
                    + "inlined-class:\" and \"Failed to compile an inlined-class: \" (trailing space); classpath runs "
                    + "two deploy/undeploy cycles where inlined MyUtil.doIt calls "
                    + "ExprClassClassDependency.supportQuoteString by fully-qualified name then by import (the "
                    + "import line has no trailing newline), each SupportBean(\"E1\",1) yielding c0=\"'E1'\". Go has "
                    + "no inlined_class directive, so class members bind as typed Go expressions (Func1 composition "
                    + "for all-local, a Func1 closure over the supportQuoteString helper for classpath); the invalid "
                    + "probes are unrepresentable on the typed Go surface and pin the Java prefixes only.";

    private static final String[] CASES = {
            "all-local", "invalid", "classpath"};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f53aa2ad87c7997d7c36",
            "java-runtime-247c1169dd5c24bd21f0",
            "java-runtime-bc85bc8d79bc09bb8500"
    };
    private static final String[] EXECUTIONS = {
            "ExprClassClassDependencyAllLocal",
            "ExprClassClassDependencyInvalid",
            "ExprClassClassDependencyClasspath"
    };
    private static final String[] STATIC_IDS = {
            "java-9027f7526d0d0a784faa",
            "java-c83ffad3efc5ef5bef30",
            "java-a8a6b01c7c18cdfb950a"
    };
    private static final String[] OBSERVATIONS = {
            "listener; the statement-local inlined_class pair compiles in one Janino batch so MyClass.doIt sees MyUtil.someFunction: SupportBean(\"E1\",1) yields c0=\"|>E1<|\" (MyUtil wraps |p| around MyClass's >p<)",
            "compile-error; two tryInvalidCompile probes pin the Java message prefixes: a statement-local inlined_class and a create inlined_class depending on the path-provided MyUtil both fail because inlined-class Janino cannot see path-provided classes (the prefixes differ only by trailing space)",
            "listener; two deploy/undeploy cycles: inlined MyUtil.doIt calls ExprClassClassDependency.supportQuoteString by fully-qualified name then by import (the import line has no trailing newline): each SupportBean(\"E1\",1) yields c0=\"'E1'\""
    };

    // Pinned EPL transcriptions (verbatim from ExprClassClassDependency.java,
    // including the escapeClass trailing space+newline, the missing trailing
    // space on the invalid create-class EPL and the import-line quirk).
    private static final String ALL_LOCAL_EPL =
            "@name('s0') " +
                    "inlined_class \"\"\"\n" +
                    "    public class MyUtil {\n" +
                    "        public static String someFunction(String parameter) {\n" +
                    "            return \"|\" + parameter + \"|\";\n" +
                    "        }\n" +
                    "    }\n" +
                    "\"\"\" \n" +
                    "inlined_class \"\"\"\n" +
                    "    public class MyClass {\n" +
                    "        public static String doIt(String parameter) {\n" +
                    "            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
                    "        }\n" +
                    "    }\n" +
                    "\"\"\" \n" +
                    "select MyClass.doIt(theString) as c0 from SupportBean\n";
    private static final String CREATE_CLASS_EPL =
            "@public create inlined_class \"\"\"\n" +
                    "    public class MyUtil {\n" +
                    "        public static String someFunction(String parameter) {\n" +
                    "            return \"|\" + parameter + \"|\";\n" +
                    "        }\n" +
                    "    }\n" +
                    "\"\"\"";
    private static final String CLASSPATH_FQN_EPL =
            "@name('s0') " +
                    "inlined_class \"\"\"\n" +
                    "    public class MyUtil {\n" +
                    "        public static String doIt(String parameter) {\n" +
                    "            return com.espertech.esper.regressionlib.suite.expr.clazz.ExprClassClassDependency.supportQuoteString(parameter);\n" +
                    "        }\n" +
                    "    }\n" +
                    "\"\"\" \n" +
                    "select MyUtil.doIt(theString) as c0 from SupportBean\n";
    private static final String CLASSPATH_IMPORT_EPL =
            "@name('s0') " +
                    "inlined_class \"\"\"\n" +
                    "    import com.espertech.esper.regressionlib.suite.expr.clazz.ExprClassClassDependency;" +
                    "    public class MyUtil {\n" +
                    "        public static String doIt(String parameter) {\n" +
                    "            return ExprClassClassDependency.supportQuoteString(parameter);\n" +
                    "        }\n" +
                    "    }\n" +
                    "\"\"\" \n" +
                    "select MyUtil.doIt(theString) as c0 from SupportBean\n";

    private static final String[][] PROBES = {
            {"local-on-path-class",
                    "inlined_class \"\"\"\n" +
                            "    public class MyClass {\n" +
                            "        public static String doIt(String parameter) {\n" +
                            "            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
                            "        }\n" +
                            "    }\n" +
                            "\"\"\" \n" +
                            "select MyClass.doIt(theString) as c0 from SupportBean\n",
                    "Failed to compile an inlined-class:"},
            {"create-on-path-class",
                    "create inlined_class \"\"\"\n" +
                            "    public class MyClass {\n" +
                            "        public static String doIt(String parameter) {\n" +
                            "            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
                            "        }\n" +
                            "    }\n" +
                            "\"\"\"",
                    "Failed to compile an inlined-class: "},
    };

    // The display EPL each case entry carries: the case's deploy/probe EPLs
    // concatenated in replay order.
    private static final String[] CASE_EPLS = {
            ALL_LOCAL_EPL,
            CREATE_CLASS_EPL + PROBES[0][1] + PROBES[1][1],
            CLASSPATH_FQN_EPL + CLASSPATH_IMPORT_EPL
    };

    private static final int EXPECTED_STEPS = 15;
    private static final int EXPECTED_RECORDS = 5;

    private ExprClassClassDependencyScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprClassClassDependencyScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        // Mirror SupportConfigFactory: statement exceptions rethrow on the
        // sending thread.
        configuration.getRuntime().getExceptionHandling()
                .addClass(SupportExceptionHandlerFactoryRethrow.class);

        String runtimeURI = "parity-" + SCENARIO_ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            List<EPCompiled> pathModules = new ArrayList<>();
            boolean active = false;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (operation) {
                    case "deploy":
                        EPCompiled compiled = compileStep(configuration, pathModules,
                                string(step, "epl"));
                        if ("invalid".equals(caseName)
                                && "create-class".equals(string(step, "statement"))) {
                            // Mirror the Java execution: env.compile +
                            // path.add only — the create-class module is
                            // never deployed.
                            pathModules.add(compiled);
                            break;
                        }
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions().setDeploymentId(
                                        SCENARIO_ID + "-" + caseIndex + "-" + pathModules.size()));
                        pathModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(writer);
                            }
                        }
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, pathModules, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        pathModules.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPCompiled compileStep(Configuration configuration,
                                          List<EPCompiled> pathModules, String epl)
            throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        for (EPCompiled pathModule : pathModules) {
            compilerArgs.getPath().add(pathModule);
        }
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Compiles an expected-invalid probe against the path and emits
     * {"operation":"compile-error"} carrying the pinned expectError prefix
     * after verifying the caught EPCompileException message starts with it
     * (SupportMessageAssertUtil.assertMessage semantics). The two probes'
     * prefixes differ only by trailing space and both are asserted.
     */
    private static void buildErrorStep(Configuration configuration, String caseName, JsonObject step,
                                       List<EPCompiled> pathModules, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = string(step, "expectError");
        String caught;
        try {
            compileStep(configuration, pathModules, epl);
            caught = null;
        } catch (EPCompileException ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        requireFields(payload, "theString", "intPrimitive");
        runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null),
                        payload.getInt("intPrimitive", 0)),
                eventType);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned steps. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        Map<String, List<ExpectedStep>> schedules = schedules();
        int cursor = 0;
        for (String caseName : CASES) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            for (ExpectedStep expected : schedules.get(caseName)) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!expected.op.equals(string(step, "op"))
                        || !caseName.equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (expected.op) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!expected.statement.equals(string(step, "statement"))
                                || !expected.epl.equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!expected.eventType.equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "intPrimitive");
                        if (!expected.theString.equals(string(payload, "theString"))
                                || integer(payload, "intPrimitive") != expected.intPrimitive) {
                            throw new IllegalArgumentException("send payload " + cursor + " is not pinned");
                        }
                        break;
                    case "build-error":
                        requireFields(step, "op", "case", "statement", "epl", "expectError");
                        if (!expected.statement.equals(string(step, "statement"))
                                || !expected.epl.equals(string(step, "epl"))
                                || !expected.expectError.equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException("build-error step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static final class ExpectedStep {
        private final String op;
        private final String statement;
        private final String eventType;
        private final String epl;
        private final String expectError;
        private final String theString;
        private final Integer intPrimitive;

        private ExpectedStep(String op, String statement, String eventType, String epl,
                             String expectError, String theString, Integer intPrimitive) {
            this.op = op;
            this.statement = statement;
            this.eventType = eventType;
            this.epl = epl;
            this.expectError = expectError;
            this.theString = theString;
            this.intPrimitive = intPrimitive;
        }
    }

    private static Map<String, List<ExpectedStep>> schedules() {
        Map<String, List<ExpectedStep>> schedules = new HashMap<>();
        schedules.put("all-local", List.of(
                new ExpectedStep("deploy", "s0", null, ALL_LOCAL_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, null, "E1", 1),
                new ExpectedStep("undeploy-all", null, null, null, null, null, null)));
        schedules.put("invalid", List.of(
                new ExpectedStep("deploy", "create-class", null, CREATE_CLASS_EPL, null, null, null),
                new ExpectedStep("build-error", PROBES[0][0], null, PROBES[0][1], PROBES[0][2], null, null),
                new ExpectedStep("build-error", PROBES[1][0], null, PROBES[1][1], PROBES[1][2], null, null)));
        schedules.put("classpath", List.of(
                new ExpectedStep("deploy", "s0", null, CLASSPATH_FQN_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, null, "E1", 1),
                new ExpectedStep("undeploy-all", null, null, null, null, null, null),
                new ExpectedStep("deploy", "s0", null, CLASSPATH_IMPORT_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, null, "E1", 1),
                new ExpectedStep("undeploy-all", null, null, null, null, null, null)));
        return schedules;
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

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
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
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
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
    }
}
