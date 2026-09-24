import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EPException;
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

import java.lang.reflect.Method;
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
 * Direct Esper 9.0.0 oracle for the four ExprClassForEPLObjects executions
 * (ordinals 0-3) replayed as one differential chain covering the visibility
 * boundaries of inlined_class classes:
 *
 * from-clause-method (ord 0, ExprClassResolutionFromClauseMethod): a
 * path-deployed @public create inlined_class MyFromClauseMethod whose static
 * getBeans() feeds the method: from-clause join. The dependent select
 * compiles against the path and its EPCompiled must not re-bundle the
 * create-class bytes (asserted inside the deploy step). One
 * SupportBean("E1",10) yields a single listener invocation carrying the
 * ordered rows c0=1 then c0=2 (assertPropsPerRowLastNew).
 *
 * output-col-type (ord 1, ExprClassResolutionOutputColType): the
 * statement-local inlined MyBean's static getBean(intPrimitive) is selected
 * as c0; the deploy step asserts the c0 property type's simple name is
 * "MyBean" and the trace renders the instance through getId(), so c0=10.
 *
 * invalid (ord 2, ExprClassResolutionInvalid): five tryInvalidCompile
 * probes pin the Java message prefixes — inlined classes are invisible to
 * annotation classes, create-schema bean types, nestable schema/window
 * property types and table column types. The table probe is "skip"-pinned:
 * the compile must fail but the message is unvalidated.
 *
 * script (ord 3, ExprClassResolutionScript): a js: expression whose
 * Java.type("MyScriptResult") lookup compiles and deploys but fails at
 * runtime because the Nashorn classloader cannot see inlined classes. The
 * send step verifies the EPException prefix
 * "java.lang.RuntimeException: Unexpected exception in statement 's0'"
 * (SupportExceptionHandlerFactoryRethrow mirrors SupportConfigFactory) and
 * the unrepresentable step pins the record documenting the contract — Go
 * ScriptCall folds the provider error to Null, so no Go runtime-error
 * boundary exists.
 *
 * Mirroring the regression harness, each case runs on a fresh runtime with
 * the internal timer disabled, compiles the pinned EPL verbatim
 * (inlined_class triple-quote class text, duplicated @name('s0') and
 * single-line concatenation quirks included), attaches one s0 listener,
 * sends the pinned SupportBean payloads, then undeploys. The TraceWriter
 * skips null/null listener callbacks; Integer, String and Boolean values
 * render as-is and a MyBean renders through its getId() method like the
 * reflective assertion.
 */
public final class ExprClassForEPLObjectsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-class-for-epl-objects";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassForEPLObjects.java";

    private static final String DESCRIPTION =
            "ExprClassForEPLObjects ordinals 0-3 (all executions): from-clause-method deploys @public create "
                    + "inlined_class MyFromClauseMethod on the path then selects s.id as c0 from SupportBean joined with "
                    + "method:MyFromClauseMethod.getBeans() (duplicated @name('s0') pinned verbatim), yielding one listener "
                    + "invocation with ordered c0=1 then c0=2 for SupportBean(\"E1\",10); output-col-type selects "
                    + "MyBean.getBean(intPrimitive) as c0 typed as the inlined MyBean class, rendering c0 through getId() "
                    + "as 10; invalid runs five tryInvalidCompile probes (annotation class, create-schema bean type, "
                    + "schema/window property types, table column type) that fail because inlined classes are invisible to "
                    + "those resolution sites; script deploys a js: expression whose Java.type(\"MyScriptResult\") lookup "
                    + "fails at runtime with EPException 'java.lang.RuntimeException: Unexpected exception in statement 's0''. "
                    + "Go has no inlined_class directive, so class members bind as typed Go expressions (MethodProviderFunc "
                    + "rows, Construct+GetId, RegisterScript+ScriptCall); the script case's runtime failure folds to Null "
                    + "in Go and is pinned as an unrepresentable record.";

    private static final String[] CASES = {
            "from-clause-method", "output-col-type", "invalid", "script"};
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-cb993aa0d17d61b0d45f",
            "java-runtime-203a97dc1cba0371469c",
            "java-runtime-1335a465f707da5fce60",
            "java-runtime-785742544d6ada82189d"
    };
    private static final String[] EXECUTIONS = {
            "ExprClassResolutionFromClauseMethod",
            "ExprClassResolutionOutputColType",
            "ExprClassResolutionInvalid",
            "ExprClassResolutionScript"
    };
    private static final String[] STATIC_IDS = {
            "java-3fd69b8c66fd163574ec",
            "java-3fd69b8c66fd163574ec",
            "java-3fd69b8c66fd163574ec",
            "java-3fd69b8c66fd163574ec"
    };
    private static final String[] OBSERVATIONS = {
            "listener; the path-deployed @public create inlined_class MyFromClauseMethod.getBeans() feeds the method: from-clause join: SupportBean(\"E1\",10) yields one listener invocation with ordered c0=1 then c0=2; the dependent EPCompiled carries no MyFromClauseMethod classes (asserted inside the oracle)",
            "listener; MyBean.getBean(intPrimitive) returns the inlined-class instance whose c0 property type is MyBean (asserted inside the oracle): SupportBean(\"E1\",10) yields c0 rendered through getId() as 10",
            "compile-error; five tryInvalidCompile probes pin the Java message prefixes: inlined classes are invisible to annotation classes, create-schema bean types, nestable schema/window property types and table column types; the table probe pins failure only (Java 'skip')",
            "unrepresentable; the js: script's Java.type(\"MyScriptResult\") fails at runtime because Nashorn cannot see inlined classes - Java sendEventBean raises EPException 'java.lang.RuntimeException: Unexpected exception in statement 's0'' while Go ScriptCall folds the provider error to Null"
    };

    // Pinned EPL transcriptions (verbatim from ExprClassForEPLObjects.java,
    // including the escapeClass trailing space+newline, the single-line
    // concatenation quirks and the duplicated @name('s0') annotation).
    private static final String CREATE_CLASS_EPL =
            "@public create inlined_class \"\"\"\n" +
                    "  public class MyFromClauseMethod {\n" +
                    "    public static MyBean[] getBeans() {\n" +
                    "       return new MyBean[] {new MyBean(1), new MyBean(2)};\n" +
                    "    }\n" +
                    "    public static class MyBean {\n" +
                    "      private final int id;" +
                    "      public MyBean(int id) {this.id = id;}\n" +
                    "      public int getId() {return id;}\n" +
                    "    }\n" +
                    "  }\n" +
                    "\"\"\" \n";
    private static final String FROM_CLAUSE_SELECT_EPL =
            "@name('s0')" +
                    "@name('s0') select s.id as c0 from SupportBean as e,\n" +
                    "method:MyFromClauseMethod.getBeans() as s";
    private static final String OUTPUT_COL_TYPE_EPL =
            "inlined_class \"\"\"\n" +
                    "  public class MyBean {\n" +
                    "    private final int id;" +
                    "    public MyBean(int id) {this.id = id;}\n" +
                    "    public int getId() {return id;}\n" +
                    "    public static MyBean getBean(int id) {return new MyBean(id);}\n" +
                    "  }\n" +
                    "\"\"\" \n" +
                    "@name('s0') select MyBean.getBean(intPrimitive) as c0 from SupportBean";
    private static final String SCRIPT_EPL =
            "inlined_class \"\"\"\n" +
                    "public class MyScriptResult {}" +
                    "\"\"\" \n" +
                    "expression Object[] js:myItemProducerScript() [\n" +
                    "myItemProducerScript();" +
                    "function myItemProducerScript() {" +
                    "  var arrayType = Java.type(\"MyScriptResult\");\n" +
                    "  var rows = new arrayType(2);\n" +
                    "  return rows;\n" +
                    "}]" +
                    "@name('s0') select myItemProducerScript() from SupportBean";

    private static final String[][] PROBES = {
            {"annotation-class",
                    "inlined_class \"\"\"\npublic @interface MyAnnotation{}\"\"\" \n@MyAnnotation @name('s0') select * from SupportBean\n",
                    "Failed to process statement annotations: Failed to resolve @-annotation class: Could not load annotation class by name 'MyAnnotation', please check imports"},
            {"schema-bean-type",
                    "inlined_class \"\"\"\npublic class MyEventBean {}\"\"\" \ncreate schema MyEvent as MyEventBean\n",
                    "Could not load class by name 'MyEventBean', please check imports"},
            {"schema-property-type",
                    "inlined_class \"\"\"\npublic class MyEventBean {}\"\"\" \ncreate schema MyEvent as (field1 MyEventBean)\n",
                    "Nestable type configuration encountered an unexpected property type name"},
            {"window-property-type",
                    "inlined_class \"\"\"\npublic class MyType {}\"\"\" \ncreate window MyWindow(myfield MyType)\n",
                    "Nestable type configuration encountered an unexpected property type name"},
            {"table-column-type",
                    "inlined_class \"\"\"\npublic class MyType {}\"\"\" \ncreate table MyTable(myfield MyType)\n",
                    ""},
    };

    private static final String SCRIPT_SEND_ERROR =
            "java.lang.RuntimeException: Unexpected exception in statement 's0'";
    private static final String SCRIPT_NOTE =
            "runtime script class lookup: Java.type(\"MyScriptResult\") fails under Nashorn because the "
                    + "script engine classloader cannot see inlined classes; Java sendEventBean raises EPException "
                    + "'java.lang.RuntimeException: Unexpected exception in statement 's0'' while Go ScriptCall folds "
                    + "the provider error to Null - no Go runtime-error boundary";

    // The display EPL each case entry carries: the case's deploy/probe EPLs
    // concatenated in replay order.
    private static final String[] CASE_EPLS = {
            CREATE_CLASS_EPL + FROM_CLAUSE_SELECT_EPL,
            OUTPUT_COL_TYPE_EPL,
            PROBES[0][1] + PROBES[1][1] + PROBES[2][1] + PROBES[3][1] + PROBES[4][1],
            SCRIPT_EPL
    };

    private static final int EXPECTED_STEPS = 20;
    private static final int EXPECTED_RECORDS = 8;

    private ExprClassForEPLObjectsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprClassForEPLObjectsScenarioOracle <scenario.json>");
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
        // sending thread wrapped as "Unexpected exception in statement
        // '<name>': <cause>" so the script-case send observes the pinned
        // EPException prefix.
        configuration.getRuntime().getExceptionHandling()
                .addClass(SupportExceptionHandlerFactoryRethrow.class);

        String runtimeURI = "parity-" + SCENARIO_ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            List<EPCompiled> deployedModules = new ArrayList<>();
            boolean scriptErrorVerified = false;
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
                        EPCompiled compiled = compileStep(configuration, deployedModules,
                                string(step, "epl"));
                        if ("from-clause-method".equals(caseName)
                                && "s0".equals(string(step, "statement"))) {
                            // Mirror the Java execution: path-provided
                            // create-class bytes are not re-bundled into the
                            // dependent EPCompiled.
                            for (Map.Entry<String, byte[]> classEntry : compiled.getClasses().entrySet()) {
                                if (classEntry.getKey().contains("MyFromClauseMethod")) {
                                    throw new IllegalStateException(
                                            "EPCompiled should not contain create-class class");
                                }
                            }
                        }
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions().setDeploymentId(
                                        SCENARIO_ID + "-" + caseIndex + "-" + deployedModules.size()));
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(writer);
                                if ("output-col-type".equals(caseName)) {
                                    // Mirror the Java execution: the c0
                                    // property type is the inlined class
                                    // itself, not Object.
                                    String simpleName = statement.getEventType()
                                            .getPropertyType("c0").getSimpleName();
                                    if (!"MyBean".equals(simpleName)) {
                                        throw new IllegalStateException(
                                                "c0 property type = " + simpleName + ", want MyBean");
                                    }
                                }
                            }
                        }
                        break;
                    case "send":
                        String expectError = step.getString("expectError", "");
                        if (expectError.isEmpty()) {
                            sendEvent(runtime, step);
                        } else {
                            // The script-case send must raise the pinned
                            // EPException out of sendEventBean.
                            String caught;
                            try {
                                sendEvent(runtime, step);
                                caught = null;
                            } catch (EPException ex) {
                                caught = ex.getMessage();
                            }
                            if (caught == null) {
                                throw new IllegalStateException("send step in case " + caseName
                                        + " unexpectedly succeeded");
                            }
                            if (!caught.startsWith(expectError)) {
                                throw new IllegalStateException("send error drift in case " + caseName
                                        + ": expected prefix [" + expectError + "] got [" + caught + "]");
                            }
                            scriptErrorVerified = true;
                        }
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, deployedModules, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(caseName, step, scriptErrorVerified, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployedModules.clear();
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
                                          List<EPCompiled> deployedModules, String epl)
            throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        for (EPCompiled deployed : deployedModules) {
            compilerArgs.getPath().add(deployed);
        }
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Compiles an expected-invalid probe and emits
     * {"operation":"compile-error"} carrying the pinned expectError prefix
     * after verifying the caught EPCompileException message starts with it
     * (SupportMessageAssertUtil.assertMessage semantics). An empty
     * expectError is the Java "skip" pin: the compile must fail but the
     * message is unvalidated and the record omits the value field.
     */
    private static void buildErrorStep(Configuration configuration, String caseName, JsonObject step,
                                       List<EPCompiled> deployedModules, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = string(step, "expectError");
        String caught;
        try {
            compileStep(configuration, deployedModules, epl);
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

    /**
     * Emits the pinned "unrepresentable" record for the script case's
     * runtime failure after verifying the send already observed the pinned
     * EPException prefix: Go ScriptCall folds the provider error to Null,
     * so no Go runtime-error boundary exists.
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            boolean scriptErrorVerified, JsonArray records) {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        if (!"script-java-type".equals(label) || !SCRIPT_NOTE.equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " is not pinned");
        }
        if (!scriptErrorVerified) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " reached without the verified script send failure");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
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
            throw new IllegalArgumentException("scenario must contain exactly four cases");
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
                        if (expected.expectError == null) {
                            requireFields(step, "op", "case", "eventType", "payload");
                        } else {
                            requireFields(step, "op", "case", "eventType", "payload", "expectError");
                            if (!expected.expectError.equals(string(step, "expectError"))) {
                                throw new IllegalArgumentException("send expectError " + cursor + " is not pinned");
                            }
                        }
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
                    case "unrepresentable":
                        requireFields(step, "op", "case", "statement", "expectError");
                        if (!expected.statement.equals(string(step, "statement"))
                                || !expected.expectError.equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException("unrepresentable step " + cursor + " is not pinned");
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
        schedules.put("from-clause-method", List.of(
                new ExpectedStep("deploy", "create-class", null, CREATE_CLASS_EPL, null, null, null),
                new ExpectedStep("deploy", "s0", null, FROM_CLAUSE_SELECT_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, null, "E1", 10),
                new ExpectedStep("undeploy-all", null, null, null, null, null, null)));
        schedules.put("output-col-type", List.of(
                new ExpectedStep("deploy", "s0", null, OUTPUT_COL_TYPE_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, null, "E1", 10),
                new ExpectedStep("undeploy-all", null, null, null, null, null, null)));
        List<ExpectedStep> invalid = new ArrayList<>();
        for (String[] probe : PROBES) {
            invalid.add(new ExpectedStep("build-error", probe[0], null, probe[1], probe[2], null, null));
        }
        schedules.put("invalid", invalid);
        schedules.put("script", List.of(
                new ExpectedStep("deploy", "s0", null, SCRIPT_EPL, null, null, null),
                new ExpectedStep("send", null, "SupportBean", null, SCRIPT_SEND_ERROR, "E1", 1),
                new ExpectedStep("unrepresentable", "script-java-type", null, null, SCRIPT_NOTE, null, null),
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
            if ("MyBean".equals(value.getClass().getSimpleName())) {
                // The Java execution asserts the output-col-type result
                // reflectively (getId() == 10); the trace renders the
                // instance through the same getter so both hosts carry
                // c0=10.
                try {
                    Method getId = value.getClass().getMethod("getId");
                    return Json.value(((Number) getId.invoke(value)).longValue());
                } catch (ReflectiveOperationException ex) {
                    throw new IllegalStateException("MyBean getId() is not readable", ex);
                }
            }
            return Json.value(String.valueOf(value));
        }
    }
}
