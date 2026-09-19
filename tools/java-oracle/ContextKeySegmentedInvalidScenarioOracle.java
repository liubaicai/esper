import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.ISupportA;
import com.espertech.esper.regressionlib.support.bean.ISupportBaseAB;
import com.espertech.esper.runtime.client.DeploymentOptions;
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
 * Java oracle for ContextKeySegmentedInvalid (ord 19).  One case replays
 * on one fresh runtime:
 *
 * invalid (ordinal 19, ContextKeySegmentedInvalid): nine tryInvalidCompile
 * probes record the pinned Java message prefixes — a per-stream filter on
 * the partition spec, an unknown partition property, mismatched key
 * counts, a cross-stream key-type mismatch, a duplicate event type, a
 * subtype/supertype duplicate, a statement on an unlisted event type, a
 * named window in the partition criteria, and a named window whose schema
 * type is not listed by its segmented context.  Probes 1-6 compile
 * without the runtime path, mirroring env.tryInvalidCompile's path-less
 * compileWCheckedEx; probes 7-9 compile with the path after the
 * SegmentedByAString, MyWindow, SomeSchema and TheSomeSchemaCtx fixture
 * deploys, mirroring env.tryInvalidCompile(path, ...).
 */
public final class ContextKeySegmentedInvalidScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-key-segmented-invalid";
    private static final String DESCRIPTION =
            "ContextKeySegmentedInvalid (ord 19): nine tryInvalidCompile probes pin the Java message "
                    + "prefixes for segmented-context rejections — a per-stream filter on the "
                    + "partition spec, an unknown partition property, mismatched key counts, a "
                    + "cross-stream key-type mismatch, a duplicate event type, a subtype/supertype "
                    + "duplicate, a statement on an unlisted event type, a named window in the "
                    + "partition criteria, and a named window whose schema type is not listed by its "
                    + "segmented context. Probes 7-9 compile against the runtime path after their "
                    + "fixture deploys; probes 1-6 compile without it.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextKeySegmented.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-5e338406dcc2ca1aaf6a"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextKeySegmentedInvalid"
    };
    // The execution carries no per-execution static id (the analyzer is
    // skipped); the suite static id is pinned instead.
    private static final String[] STATIC_IDS = {
            "java-11c10ab303bcf5d97107"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "invalid"
    };
    private static final int[] ORDINALS = {19};
    private static final String[] CASE_OBSERVATIONS = {
            "compile-error; nine probes record the pinned Java message prefixes: the first six "
                    + "compile without the runtime path, the last three compile with the path after "
                    + "deploying SegmentedByAString, MyWindow, SomeSchema and TheSomeSchemaCtx "
                    + "fixtures; the Java execution carries no per-execution static id (analyzer "
                    + "skipped) and pins the suite static java-11c10ab303bcf5d97107"
    };

    // Verbatim transcriptions of ContextKeySegmented lines 471, 475, 479,
    // 483, 487, 491, 496-497, 501-502, 506-508.
    private static final String EPL_PROBE_FILTER =
            "create context SegmentedByAString partition by string from SupportBean(dummy = 1)";
    private static final String EPL_PROBE_UNKNOWN_KEY =
            "create context SegmentedByAString partition by dummy from SupportBean";
    private static final String EPL_PROBE_KEY_COUNT =
            "create context SegmentedByAString partition by theString from SupportBean, id, p00 "
                    + "from SupportBean_S0";
    private static final String EPL_PROBE_KEY_TYPE =
            "create context SegmentedByAString partition by theString from SupportBean, id "
                    + "from SupportBean_S0";
    private static final String EPL_PROBE_DUPLICATE_TYPE =
            "create context SegmentedByAString partition by theString from SupportBean, theString "
                    + "from SupportBean";
    private static final String EPL_PROBE_SUBTYPE =
            "create context SegmentedByAString partition by baseAB from ISupportBaseAB, a "
                    + "from ISupportA";
    private static final String EPL_CONTEXT =
            "@public create context SegmentedByAString partition by theString from SupportBean";
    private static final String EPL_PROBE_UNLISTED_TYPE =
            "context SegmentedByAString select * from SupportBean_S0";
    private static final String EPL_WINDOW =
            "@public create window MyWindow#keepall as SupportBean";
    private static final String EPL_PROBE_WINDOW_CRITERIA =
            "@public create context SegmentedByWhat partition by theString from MyWindow";
    private static final String EPL_SCHEMA =
            "@public create schema SomeSchema(ipAddress string)";
    private static final String EPL_CONTEXT_TWO =
            "@public create context TheSomeSchemaCtx Partition By ipAddress From SomeSchema";
    private static final String EPL_PROBE_WINDOW_SCHEMA =
            "@public context TheSomeSchemaCtx create window MyEvent#time(30 sec) (ipAddress string)";

    private static final String ERR_FILTER =
            "Failed to validate filter expression 'dummy=1': Property named 'dummy' is not valid "
                    + "in any stream [";
    private static final String ERR_UNKNOWN_KEY =
            "For context 'SegmentedByAString' property name 'dummy' not found on type SupportBean [";
    private static final String ERR_KEY_COUNT =
            "For context 'SegmentedByAString' expected the same number of property names for each "
                    + "event type, found 1 properties for event type 'SupportBean' and 2 properties "
                    + "for event type 'SupportBean_S0' [create context SegmentedByAString partition "
                    + "by theString from SupportBean, id, p00 from SupportBean_S0]";
    private static final String ERR_KEY_TYPE =
            "For context 'SegmentedByAString' for context 'SegmentedByAString' found mismatch of "
                    + "property types, property 'theString' of type 'String' compared to property "
                    + "'id' of type 'Integer' [";
    private static final String ERR_DUPLICATE_TYPE =
            "For context 'SegmentedByAString' the event type 'SupportBean' is listed twice [";
    private static final String ERR_SUBTYPE =
            "For context 'SegmentedByAString' the event type 'ISupportA' is listed twice: Event "
                    + "type 'ISupportA' is a subtype or supertype of event type 'ISupportBaseAB' [";
    private static final String ERR_UNLISTED_TYPE =
            "Segmented context 'SegmentedByAString' requires that any of the event types that are "
                    + "listed in the segmented context also appear in any of the filter expressions "
                    + "of the statement, type 'SupportBean_S0' is not one of the types listed [";
    private static final String ERR_WINDOW_CRITERIA =
            "Partition criteria may not include named windows [@public create context "
                    + "SegmentedByWhat partition by theString from MyWindow]";
    private static final String ERR_WINDOW_SCHEMA =
            "Segmented context 'TheSomeSchemaCtx' requires that named windows are associated to an "
                    + "existing event type and that the event type is listed among the partitions "
                    + "defined by the create-context statement";

    private static final String[] CASE_EPLS = {
            EPL_PROBE_FILTER
    };

    private static final int EXPECTED_STEPS = 15;
    private static final int EXPECTED_RECORDS = 9;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * build-error steps carry the pinned expectError prefix and the
     * compileWithoutPath marker for the path-less probes.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("invalid", new String[]{
                "build-error|invalid|partition-filter-property||" + EPL_PROBE_FILTER
                        + "||" + ERR_FILTER + "|1||||",
                "build-error|invalid|unknown-key-property||" + EPL_PROBE_UNKNOWN_KEY
                        + "||" + ERR_UNKNOWN_KEY + "|1||||",
                "build-error|invalid|mismatched-key-count||" + EPL_PROBE_KEY_COUNT
                        + "||" + ERR_KEY_COUNT + "|1||||",
                "build-error|invalid|mismatched-key-type||" + EPL_PROBE_KEY_TYPE
                        + "||" + ERR_KEY_TYPE + "|1||||",
                "build-error|invalid|duplicate-type||" + EPL_PROBE_DUPLICATE_TYPE
                        + "||" + ERR_DUPLICATE_TYPE + "|1||||",
                "build-error|invalid|duplicate-subtype||" + EPL_PROBE_SUBTYPE
                        + "||" + ERR_SUBTYPE + "|1||||",
                "deploy|invalid|ctx||" + EPL_CONTEXT + "|||||||",
                "build-error|invalid|unlisted-statement-type||" + EPL_PROBE_UNLISTED_TYPE
                        + "||" + ERR_UNLISTED_TYPE + "|||||",
                "deploy|invalid|window||" + EPL_WINDOW + "|||||||",
                "build-error|invalid|named-window-partition-criteria||" + EPL_PROBE_WINDOW_CRITERIA
                        + "||" + ERR_WINDOW_CRITERIA + "|||||",
                "deploy|invalid|schema||" + EPL_SCHEMA + "|||||||",
                "deploy|invalid|ctx2||" + EPL_CONTEXT_TWO + "|||||||",
                "build-error|invalid|named-window-unlisted-schema||" + EPL_PROBE_WINDOW_SCHEMA
                        + "||" + ERR_WINDOW_SCHEMA + "|||||",
                "undeploy-all|invalid||||||||||",
        });
    }

    private ContextKeySegmentedInvalidScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextKeySegmentedInvalidScenarioOracle <scenario.json>");
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
     * Replays the case's steps on a fresh runtime.  Deploy steps compile
     * with the runtime path (env.compileDeploy(epl, path)); build-error
     * steps compile with or without the path per their compileWithoutPath
     * marker, mirroring env.tryInvalidCompile's two forms.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(ISupportBaseAB.class);
        configuration.getCommon().addEventType(ISupportA.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
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
                    case "deploy": {
                        String epl = string(step, "epl");
                        EPCompiled compiled = compileModule(epl, configuration, runtime);
                        runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        break;
                    }
                    case "build-error":
                        buildErrorStep(runtime, configuration, caseName, step, records);
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
     * Module compile with the runtime path, mirroring
     * RegressionEnvironmentBase.compileDeploy(epl, path): the compiler sees
     * the configuration plus the already-deployed module path so the
     * fixture deploys and the path-ful probes resolve earlier module
     * objects.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics).  Probes marked compileWithoutPath compile without the
     * runtime path, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx; the rest compile with the path.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
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

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
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
     * mode|selector|ids|fields with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields"));
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
                + "|" + fieldsText;
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

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
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
