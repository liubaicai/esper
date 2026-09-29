import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0_Container;
import com.espertech.esper.regressionlib.support.bean.SupportCollection;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Deterministic Java trace generator for the expr-enum-invalid-args
 * scenario: the five enumeration-method invalid-argument executions from
 * five different suite files.  The oracle replays the checked-in scenario
 * JSON against the pinned Esper 9.0.0 runtime and emits one compile-error
 * record per invalid probe; the Go parity runner replays the same
 * scenario and the differential comparator requires byte-identical
 * normalized records.
 *
 * <p>min-invalid mirrors ExprEnumMinMax ord 4 (ExprEnumInvalid): two
 * path-less tryInvalidCompile probes over SupportBean_ST0_Container pin
 * the 0-parameter-footprint collection-of-events rejection for
 * contained.min() and the Null-type selector rejection for
 * contained.min(x => null).
 *
 * <p>minby-invalid mirrors ExprEnumMinMaxBy ord 2
 * (ExprEnumMinMaxByInvalid): one path-less probe pins the Null-type
 * selector rejection for contained.minBy(x => null).
 *
 * <p>orderby-invalid mirrors ExprEnumOrderBy ord 4
 * (ExprEnumOrderByInvalid): two path-less probes pin the same
 * 0-parameter-footprint rejection for contained.orderBy() and the
 * Null-type selector rejection for strvals.orderBy(v => null) over
 * SupportCollection.
 *
 * <p>take-invalid mirrors ExprEnumTakeAndTakeLast ord 2
 * (ExprEnumTakeInvalid): one path-less probe pins the
 * non-null-expression-parameter rejection for strvals.take(null).
 *
 * <p>takewhile-invalid mirrors ExprEnumTakeWhileAndWhileLast ord 2
 * (ExprEnumTakeWhileInvalid): one path-less probe pins the same
 * rejection for strvals.takeWhile(x => null).
 *
 * <p>The oracle asserts that each caught Java message starts with the
 * verbatim Java-source prefix kept in JAVA_ASSERT_PREFIXES, mirroring
 * SupportMessageAssertUtil.assertMessage startsWith semantics. The
 * recorded compile-error value carries the contract-pinned expectError
 * clause instead: a verbatim substring of the Java assertion text that
 * is the full assertion for the two 0-parameter-footprint probes but a
 * mid-message clause for the four Null-type selector probes and the two
 * non-null-expression-parameter probes (the Go wording those records
 * pin).
 */
public class ExprEnumInvalidArgsScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-enum-invalid-args";
    private static final String DESCRIPTION =
            "Enum-method invalid-argument compile probes across five suite files: ExprEnumInvalid (ExprEnumMinMax ord 4) pins the 0-parameter-footprint collection-of-events rejection for contained.min() and the Null-type selector rejection for contained.min(x => null) over SupportBean_ST0_Container.contained : List<SupportBean_ST0>; ExprEnumMinMaxByInvalid (ExprEnumMinMaxBy ord 2) pins the Null-type selector rejection for contained.minBy(x => null); ExprEnumOrderByInvalid (ExprEnumOrderBy ord 4) pins the same 0-parameter-footprint rejection for contained.orderBy() and the Null-type selector rejection for strvals.orderBy(v => null) over SupportCollection.strvals : List<String>; ExprEnumTakeInvalid (ExprEnumTakeAndTakeLast ord 2) pins the non-null-expression-parameter rejection for strvals.take(null); ExprEnumTakeWhileInvalid (ExprEnumTakeWhileAndWhileLast ord 2) pins the same rejection for strvals.takeWhile(x => null). All seven probes go through the path-less env.tryInvalidCompile overload; compile-error records carry the pinned expectError value while the oracle asserts the verbatim Java assertion text with startsWith semantics (Java sources regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java, ExprEnumMinMaxBy.java, ExprEnumOrderBy.java, ExprEnumTakeAndTakeLast.java, ExprEnumTakeWhileAndWhileLast.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMaxBy.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumOrderBy.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeAndTakeLast.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeWhileAndWhileLast.java"
    };
    private static final String[] CASES = {
            "min-invalid",
            "minby-invalid",
            "orderby-invalid",
            "take-invalid",
            "takewhile-invalid"
    };
    private static final int[] ORDINALS = {4, 2, 4, 2, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-67deab0a6d57e2bc57fa",
            "java-runtime-5556be14d552b0228acc",
            "java-runtime-f721caa1c77ad596eca4",
            "java-runtime-3c4f6374416fe5a2ee04",
            "java-runtime-50b5bc269985fbfd9ed2"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprEnumInvalid",
            "ExprEnumMinMaxByInvalid",
            "ExprEnumOrderByInvalid",
            "ExprEnumTakeInvalid",
            "ExprEnumTakeWhileInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-167415d6e7af87a7a111",
            "java-4083b006f50db9c59884",
            "java-0277b2cbf963ec07d2c0",
            "java-3f7b6e1e84fe78b4a816",
            "java-0fec7ea37236a9b16857"
    };
    private static final String[] JAVA_FLAGS = {};

    // Verbatim EPL texts transcribed from the Java executions.
    private static final String EPL_MIN_EVENT_INPUT =
            "select contained.min() from SupportBean_ST0_Container";
    private static final String EPL_MIN_NULL_SELECTOR =
            "select contained.min(x => null) from SupportBean_ST0_Container";
    private static final String EPL_MINBY_NULL_SELECTOR =
            "select contained.minBy(x => null) from SupportBean_ST0_Container";
    private static final String EPL_ORDERBY_EVENT_INPUT =
            "select contained.orderBy() from SupportBean_ST0_Container";
    private static final String EPL_ORDERBY_NULL_SELECTOR =
            "select strvals.orderBy(v => null) from SupportCollection";
    private static final String EPL_TAKE_NULL_COUNT =
            "select strvals.take(null) from SupportCollection";
    private static final String EPL_TAKEWHILE_NULL_PREDICATE =
            "select strvals.takeWhile(x => null) from SupportCollection";

    // Pinned record values (the Go wording carried by compile-error records).
    private static final String ERR_MIN_EVENT_INPUT =
            "Failed to validate select-clause expression 'contained.min()': Invalid input for built-in enumeration method 'min' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '" + SupportBean_ST0.class.getName() + "'";
    private static final String ERR_NULL_TYPE =
            "Null-type is not allowed";
    private static final String ERR_ORDERBY_EVENT_INPUT =
            "Failed to validate select-clause expression 'contained.orderBy()': Invalid input for built-in enumeration method 'orderBy' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '" + SupportBean_ST0.class.getName() + "'";
    private static final String ERR_TAKE_NULL_COUNT =
            "Failed to validate enumeration method 'take', expected a non-null result for expression parameter 0 but received a null-typed expression";
    private static final String ERR_TAKEWHILE_NULL_PREDICATE =
            "Failed to validate enumeration method 'takeWhile', expected a non-null result for expression parameter 0 but received a null-typed expression";

    /**
     * The verbatim Java-source assertion text per probe label: the full
     * expected message passed to env.tryInvalidCompile, which the oracle
     * asserts with startsWith (SupportMessageAssertUtil.assertMessage
     * semantics).  Note the select-clause expression quoting in the Java
     * assertions normalizes lambda-parameter calls to their 0-parameter
     * footprint ('contained.min()', 'contained.minBy()',
     * 'strvals.orderBy()', 'strvals.takeWhile()') while 'take' quotes the
     * EPL text verbatim ('strvals.take(null)').  The emitted record value
     * carries only the pinned expectError clause.
     */
    private static final Map<String, String> JAVA_ASSERT_PREFIXES = new HashMap<>();
    static {
        JAVA_ASSERT_PREFIXES.put("min-event-input", ERR_MIN_EVENT_INPUT);
        JAVA_ASSERT_PREFIXES.put("min-null-selector",
                "Failed to validate select-clause expression 'contained.min()': Null-type is not allowed");
        JAVA_ASSERT_PREFIXES.put("minby-null-selector",
                "Failed to validate select-clause expression 'contained.minBy()': Null-type is not allowed");
        JAVA_ASSERT_PREFIXES.put("orderby-event-input", ERR_ORDERBY_EVENT_INPUT);
        JAVA_ASSERT_PREFIXES.put("orderby-null-selector",
                "Failed to validate select-clause expression 'strvals.orderBy()': Null-type is not allowed");
        JAVA_ASSERT_PREFIXES.put("take-null-count",
                "Failed to validate select-clause expression 'strvals.take(null)': " + ERR_TAKE_NULL_COUNT);
        JAVA_ASSERT_PREFIXES.put("takewhile-null-predicate",
                "Failed to validate select-clause expression 'strvals.takeWhile()': " + ERR_TAKEWHILE_NULL_PREDICATE);
    }

    private static final String[] CASE_OBSERVATIONS = {
            "compile-error; two tryInvalidCompile probes over SupportBean_ST0_Container pin the 0-parameter-footprint collection-of-events rejection for contained.min() and the Null-type selector rejection for contained.min(x => null)",
            "compile-error; one tryInvalidCompile probe over SupportBean_ST0_Container pins the Null-type selector rejection for contained.minBy(x => null)",
            "compile-error; two tryInvalidCompile probes pin the 0-parameter-footprint collection-of-events rejection for contained.orderBy() over SupportBean_ST0_Container and the Null-type selector rejection for strvals.orderBy(v => null) over SupportCollection",
            "compile-error; one tryInvalidCompile probe over SupportCollection pins the non-null-expression-parameter rejection for strvals.take(null)",
            "compile-error; one tryInvalidCompile probe over SupportCollection pins the non-null-expression-parameter rejection for strvals.takeWhile(x => null)"
    };
    private static final String[] CASE_EPLS = {
            EPL_MIN_EVENT_INPUT,
            EPL_MINBY_NULL_SELECTOR,
            EPL_ORDERBY_EVENT_INPUT,
            EPL_TAKE_NULL_COUNT,
            EPL_TAKEWHILE_NULL_PREDICATE
    };

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|at with the payload compacted.  Build-error
     * steps carry the byte-exact EPL text plus the pinned expectError
     * record value and the compileWithoutPath marker (every probe uses
     * the path-less env.tryInvalidCompile overload).
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        String min = "min-invalid";
        CASE_STEPS.put(min, new String[]{
                "build-error|" + min + "|min-event-input||" + EPL_MIN_EVENT_INPUT
                        + "||" + ERR_MIN_EVENT_INPUT + "|1|||||",
                "build-error|" + min + "|min-null-selector||" + EPL_MIN_NULL_SELECTOR
                        + "||" + ERR_NULL_TYPE + "|1|||||",
                "undeploy-all|" + min + "|||||||||||"
        });
        String minby = "minby-invalid";
        CASE_STEPS.put(minby, new String[]{
                "build-error|" + minby + "|minby-null-selector||" + EPL_MINBY_NULL_SELECTOR
                        + "||" + ERR_NULL_TYPE + "|1|||||",
                "undeploy-all|" + minby + "|||||||||||"
        });
        String orderby = "orderby-invalid";
        CASE_STEPS.put(orderby, new String[]{
                "build-error|" + orderby + "|orderby-event-input||" + EPL_ORDERBY_EVENT_INPUT
                        + "||" + ERR_ORDERBY_EVENT_INPUT + "|1|||||",
                "build-error|" + orderby + "|orderby-null-selector||" + EPL_ORDERBY_NULL_SELECTOR
                        + "||" + ERR_NULL_TYPE + "|1|||||",
                "undeploy-all|" + orderby + "|||||||||||"
        });
        String take = "take-invalid";
        CASE_STEPS.put(take, new String[]{
                "build-error|" + take + "|take-null-count||" + EPL_TAKE_NULL_COUNT
                        + "||" + ERR_TAKE_NULL_COUNT + "|1|||||",
                "undeploy-all|" + take + "|||||||||||"
        });
        String takewhile = "takewhile-invalid";
        CASE_STEPS.put(takewhile, new String[]{
                "build-error|" + takewhile + "|takewhile-null-predicate||" + EPL_TAKEWHILE_NULL_PREDICATE
                        + "||" + ERR_TAKEWHILE_NULL_PREDICATE + "|1|||||",
                "undeploy-all|" + takewhile + "|||||||||||"
        });
    }

    private static final int EXPECTED_STEPS = 17;
    private static final int EXPECTED_RECORDS = 7;

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprEnumInvalidArgsScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays the case's steps on a fresh runtime with the internal timer
     * disabled and the clock initialized at epoch.  Every case registers
     * SupportBean_ST0_Container (contained : List<SupportBean_ST0>) and
     * SupportCollection (strvals : List<String>) as bean event types,
     * mirroring TestSuiteExprEnum.configure restricted to this scenario.
     * Build-error steps compile path-less, mirroring the
     * env.tryInvalidCompile(epl, message) overload every probe uses.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean_ST0_Container.class);
        configuration.getCommon().addEventType(SupportCollection.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
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
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
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
     * Compiles an expected-invalid probe, mirroring env.tryInvalidCompile:
     * the compile is path-less (the Java executions never pass a
     * RegressionPath to these probes).  The compile must fail and the
     * caught message must start with the verbatim Java-source prefix
     * pinned in JAVA_ASSERT_PREFIXES (assertMessage startsWith
     * semantics); the emitted compile-error record carries the pinned
     * expectError value.
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String javaPrefix = JAVA_ASSERT_PREFIXES.get(label);
        if (javaPrefix == null) {
            throw new IllegalStateException("no pinned Java assert prefix for probe " + label);
        }
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(javaPrefix)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + javaPrefix + "] got [" + caught + "]");
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

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|at with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields", "at"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText + "|" + string(step, "at");
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
    }

    private static void validateStringArray(JsonValue value, String[] expected, String field) {
        JsonArray actual = array(value, field);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(field + " length " + actual.size()
                    + " != " + expected.length);
        }
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(actual.get(i).asString())) {
                throw new IllegalArgumentException(field + "[" + i + "] "
                        + actual.get(i).asString() + " != " + expected[i]);
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

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String field) {
        return object.get(field).asInt();
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
