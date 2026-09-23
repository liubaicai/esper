import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
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
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
 * Deterministic Java trace generator for the
 * context-key-segmented-named-window-subquery scenario
 * (ContextKeySegmentedNamedWindow ordinals 4 and 5).  The oracle replays the
 * checked-in scenario JSON against the pinned Esper 9.0.0 runtime and emits
 * one JSON record per observable listener delivery; the Go parity runner
 * replays the same scenario and the differential comparator requires
 * byte-identical normalized records.
 *
 * <p>Both cases share the tryAssertionSubqueryNW event matrix: a context-FREE
 * keepall window over SupportBean_S0 is read by the correlated scalar
 * subquery (select p00 ... where sb.intPrimitive = s0.id) of the keyed-context
 * s0 statement.  The correlation is not partition-scoped: every partition
 * reads the same global window, so G2 sees s1 and G1 sees s2 after it was
 * inserted while only G3 events had flowed; val0 is null when no row matches.
 * The unshared case deploys one module without @public; the shared case
 * deploys four statements through the accumulated module path with @public
 * context/window and the enable_window_subquery_indexshare hint on the
 * create-window (observable output identical).  Java's milestone calls are
 * harness no-ops and carry no steps.
 */
public class ContextKeySegmentedNamedWindowSubqueryScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-key-segmented-named-window-subquery";
    private static final String DESCRIPTION =
            "ContextKeySegmentedNamedWindow ords 4 and 5: a context-free keepall named window over SupportBean_S0 read by a correlated scalar subquery (select p00 ... where sb.intPrimitive = s0.id) from a keyed-context SupportBean statement; the shared variant adds @public path sharing and the enable_window_subquery_indexshare hint with identical observable output.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedNamedWindow.java";
    private static final String[] CASES = {
            "keyed-subquery-nw-index-unshared",
            "keyed-subquery-nw-index-shared"
    };
    private static final int[] ORDINALS = {4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7347c7d16d52e5ea0d30",
            "java-runtime-af7bcf071474f57227fb"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextKeyedSubqueryNamedWindowIndexUnShared",
            "ContextKeyedSubqueryNamedWindowIndexShared"
    };
    private static final String[] STATIC_IDS = {
            "java-b6c806b84e6ac1511ac5",
            "java-79ea11275b4e65d10869"
    };
    private static final String[] JAVA_FLAGS = {};

    // Verbatim transcriptions of ContextKeySegmentedNamedWindow lines 116-121
    // (unshared, one module) and 98-107 (shared, four path-shared deploys).
    private static final String EPL_UNSHARED =
            "@Name('context') create context SegmentedByString partition by theString from SupportBean;\n"
                    + "create window MyWindowThree#keepall as SupportBean_S0;\n"
                    + "insert into MyWindowThree select * from SupportBean_S0;\n"
                    + "@Name('s0') context SegmentedByString "
                    + "select theString, intPrimitive, (select p00 from MyWindowThree as s0 where sb.intPrimitive = s0.id) as val0 "
                    + "from SupportBean as sb;\n";
    private static final String EPL_SHARED_CONTEXT =
            "@Name('context') @public create context SegmentedByString partition by theString from SupportBean";
    private static final String EPL_SHARED_WINDOW =
            "@Hint('enable_window_subquery_indexshare') @public create window MyWindowTwo#keepall as SupportBean_S0";
    private static final String EPL_SHARED_INSERT =
            "insert into MyWindowTwo select * from SupportBean_S0";
    private static final String EPL_SHARED_S0 =
            "@Name('s0') context SegmentedByString "
                    + "select theString, intPrimitive, (select p00 from MyWindowTwo as s0 where sb.intPrimitive = s0.id) as val0 "
                    + "from SupportBean as sb";

    private static final String[] CASE_OBSERVATIONS = {
            "listener; single-module deploy (no @public); the contexted s0 subquery reads the global window: G1(10)/G2(10) see s1, G3(20) sees null until S0(20,s2) lands, then G3(20)/G1(20) see s2",
            "listener; four path-shared deploys with @public context/window and the enable_window_subquery_indexshare hint on the create-window; observable output identical to the unshared variant"
    };
    private static final String[] CASE_EPLS = {
            EPL_UNSHARED,
            EPL_SHARED_CONTEXT + "\n" + EPL_SHARED_WINDOW + "\n" + EPL_SHARED_INSERT + "\n" + EPL_SHARED_S0
    };

    private static final String SEND_S0_10 =
            "send|%s||SupportBean_S0||{\"id\":10,\"p00\":\"s1\"}||||||";
    private static final String SEND_G1_10 =
            "send|%s||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":10}||||||";
    private static final String SEND_G2_10 =
            "send|%s||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":10}||||||";
    private static final String SEND_G3_20 =
            "send|%s||SupportBean||{\"theString\":\"G3\",\"intPrimitive\":20}||||||";
    private static final String SEND_S0_20 =
            "send|%s||SupportBean_S0||{\"id\":20,\"p00\":\"s2\"}||||||";
    private static final String SEND_G1_20 =
            "send|%s||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":20}||||||";

    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        String unshared = "keyed-subquery-nw-index-unshared";
        CASE_STEPS.put(unshared, new String[]{
                "deploy|" + unshared + "|module||" + EPL_UNSHARED + "|||||||",
                String.format(SEND_S0_10, unshared),
                String.format(SEND_G1_10, unshared),
                String.format(SEND_G2_10, unshared),
                String.format(SEND_G3_20, unshared),
                String.format(SEND_S0_20, unshared),
                String.format(SEND_G3_20, unshared),
                String.format(SEND_G1_20, unshared),
                "undeploy-all|" + unshared + "||||||||||"
        });
        String shared = "keyed-subquery-nw-index-shared";
        CASE_STEPS.put(shared, new String[]{
                "deploy|" + shared + "|module||" + EPL_SHARED_CONTEXT + "|||||||",
                "deploy|" + shared + "|module||" + EPL_SHARED_WINDOW + "|||||||",
                "deploy|" + shared + "|module||" + EPL_SHARED_INSERT + "|||||||",
                "deploy|" + shared + "|module||" + EPL_SHARED_S0 + "|||||||",
                String.format(SEND_S0_10, shared),
                String.format(SEND_G1_10, shared),
                String.format(SEND_G2_10, shared),
                String.format(SEND_G3_20, shared),
                String.format(SEND_S0_20, shared),
                String.format(SEND_G3_20, shared),
                String.format(SEND_G1_20, shared),
                "undeploy-all|" + shared + "||||||||||"
        });
    }

    private static final int EXPECTED_STEPS = 23;
    private static final int EXPECTED_RECORDS = 10;

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextKeySegmentedNamedWindowSubqueryScenarioOracle <scenario.json>");
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
     * disabled and the clock initialized at epoch.  Deploy steps compile
     * with the accumulated module path (the Java execution's shared
     * RegressionPath for the shared case; the unshared case's single module
     * needs no path) and attach the listener to the statement named s0,
     * mirroring env.addListener.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
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
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        compilerArgs.getPath().getCompileds().addAll(deployedModules);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime, statement));
                            }
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
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
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * Every delivery is a single new-only row carrying theString,
     * intPrimitive and val0 (null when the correlation matches no window
     * row).
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           EPStatement statement) {
        return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            if (newEvents != null && newEvents.length > 0) {
                JsonArray rows = new JsonArray();
                for (EventBean event : newEvents) {
                    rows.add(fullRow(event));
                }
                record.add("new", rows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                JsonArray rows = new JsonArray();
                for (EventBean event : oldEvents) {
                    rows.add(fullRow(event));
                }
                record.add("old", rows);
            }
            records.add(record);
        };
    }

    private static JsonObject fullRow(EventBean event) {
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object.
     */
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
                requirePayloadFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                requirePayloadFields(payload, "id", "p00");
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"),
                        nullableString(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalStateException("unsupported event type " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
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

    private static void requirePayloadFields(JsonObject payload, String... names) {
        for (String name : names) {
            if (payload.get(name) == null) {
                throw new IllegalStateException("payload missing field " + name);
            }
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

    private static String nullableString(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        return value == null || value.isNull() ? null : value.asString();
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
