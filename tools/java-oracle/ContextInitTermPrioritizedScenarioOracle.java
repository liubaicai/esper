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
 * Deterministic Java trace generator for the context-init-term-prioritized
 * scenario (ContextInitTermPrioritized ordinals 0 and 1).  The oracle replays
 * the checked-in scenario JSON against the pinned Esper 9.0.0 runtime and
 * emits one JSON record per observable listener delivery plus one
 * compile-error record for the invalid probe; the Go parity runner replays
 * the same scenario and the differential comparator requires byte-identical
 * normalized records.
 *
 * <p>nonoverlapping-subquery mirrors
 * ContextInitTermPrioNonOverlappingSubqueryAndInvalid: the clock advances to
 * 10:00 inside the 9:00-17:00 RuleActivityTime cron context, the eight
 * module statements deploy one at a time through the accumulated module
 * path (the Java execution's shared RegressionPath), the listener attaches
 * to out, and the first SupportProductIdEvent(A1) passes the not-exists
 * guard on exactly one of the four prioritized inserts so out emits a
 * single {productID=A1} row.  The build-error probe then compiles the
 * context-free insert against the path and records the pinned
 * cross-context named-window subquery rejection prefix.
 *
 * <p>terminating-same-event mirrors
 * ContextInitTermPrioAtNowWithSelectedEventEnding: the @Priority(1) C1
 * context starts @now and ends on the same SupportBean event that the
 * @Priority(0) s0 statement selects, so E1 and E2 each emit one row.
 * SupportBean and SupportProductIdEvent register as Map event types with
 * the asserted fields only, matching the other init-term oracles.
 */
public class ContextInitTermPrioritizedScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-init-term-prioritized";
    private static final String DESCRIPTION =
            "ContextInitTermPrioritized ords 0 and 1: a cron-scheduled (9-to-5) initiated/terminated context whose firstunique named window is fed by four insert statements guarded by a not-exists subquery — the first A1 event lands exactly once and out emits one row — followed by a tryInvalidCompile probe pinning the cross-context named-window subquery rejection; the second execution ends each @now-initiated partition with the same SupportBean event that s0 selects.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermPrioritized.java";
    private static final String[] CASES = {
            "nonoverlapping-subquery",
            "terminating-same-event"
    };
    private static final int[] ORDINALS = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-bb247dc87cf118eb8661",
            "java-runtime-0c822c80cf402d017d61"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextInitTermPrioNonOverlappingSubqueryAndInvalid",
            "ContextInitTermPrioAtNowWithSelectedEventEnding"
    };
    private static final String[] STATIC_IDS = {
            "java-41c13254dc50886c2dd2",
            "java-517175a60c987d2e2397"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] LISTENERS = {"out", "s0"};

    // Verbatim transcriptions of ContextInitTermPrioritized lines 38-45
    // (the eight newline-prefixed module statements, deployed one per step
    // through the accumulated path) and 61-62 (the two-statement module).
    private static final String EPL_SUBQUERY_CTX =
            "@Name('ctx') @public create context RuleActivityTime as start (0, 9, *, *, *) end (0, 17, *, *, *)";
    private static final String EPL_SUBQUERY_WINDOW =
            "@Name('window') @public context RuleActivityTime create window EventsWindow#firstunique(productID) as SupportProductIdEvent";
    private static final String EPL_SUBQUERY_VARIABLE =
            "@Name('variable') create variable boolean IsOutputTriggered_2 = false";
    private static final String EPL_SUBQUERY_A =
            "@Name('A') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))";
    private static final String EPL_SUBQUERY_B =
            "@Name('B') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))";
    private static final String EPL_SUBQUERY_C =
            "@Name('C') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))";
    private static final String EPL_SUBQUERY_D =
            "@Name('D') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))";
    private static final String EPL_SUBQUERY_OUT =
            "@Name('out') context RuleActivityTime select * from EventsWindow";
    private static final String EPL_SUBQUERY_PROBE =
            "insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))";
    private static final String EPL_SUBQUERY_PROBE_ERROR =
            "Failed to validate subquery number 1 querying EventsWindow: Named window by name 'EventsWindow' has been declared for context 'RuleActivityTime' and can only be used within the same context";
    private static final String EPL_SAME_EVENT_CTX =
            "@Priority(1) create context C1 start @now end SupportBean";
    private static final String EPL_SAME_EVENT_S0 =
            "@name('s0') @Priority(0) context C1 select * from SupportBean";

    private static final String[] CASE_OBSERVATIONS = {
            "listener+compile-error; at 10:00 inside the 9:00-17:00 cron context the first SupportProductIdEvent(A1) passes the not-exists guard on exactly one of the four prioritized inserts, lands in EventsWindow and out emits one {productID=A1} row; the probe records the pinned cross-context named-window subquery rejection prefix",
            "listener; @Priority(1) context C1 starts @now and ends on the same SupportBean event that @Priority(0) s0 selects, so E1 and E2 each emit one row"
    };
    private static final String[] CASE_EPLS = {
            "\n " + EPL_SUBQUERY_CTX + ";\n " + EPL_SUBQUERY_WINDOW + ";\n " + EPL_SUBQUERY_VARIABLE
                    + ";\n " + EPL_SUBQUERY_A + ";\n " + EPL_SUBQUERY_B + ";\n " + EPL_SUBQUERY_C
                    + ";\n " + EPL_SUBQUERY_D + ";\n " + EPL_SUBQUERY_OUT,
            EPL_SAME_EVENT_CTX + ";\n" + EPL_SAME_EVENT_S0 + ";\n"
    };

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|at with the payload compacted.  Deploy steps
     * carry the byte-exact EPL text; the build-error step carries the pinned
     * expectError prefix; advance-time carries the at instant.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        String subquery = "nonoverlapping-subquery";
        CASE_STEPS.put(subquery, new String[]{
                "advance-time|" + subquery + "|||||||||||2002-05-01T10:00:00.000Z",
                "deploy|" + subquery + "|ctx||" + EPL_SUBQUERY_CTX + "||||||||",
                "deploy|" + subquery + "|window||" + EPL_SUBQUERY_WINDOW + "||||||||",
                "deploy|" + subquery + "|variable||" + EPL_SUBQUERY_VARIABLE + "||||||||",
                "deploy|" + subquery + "|A||" + EPL_SUBQUERY_A + "||||||||",
                "deploy|" + subquery + "|B||" + EPL_SUBQUERY_B + "||||||||",
                "deploy|" + subquery + "|C||" + EPL_SUBQUERY_C + "||||||||",
                "deploy|" + subquery + "|D||" + EPL_SUBQUERY_D + "||||||||",
                "deploy|" + subquery + "|out||" + EPL_SUBQUERY_OUT + "||||||||",
                "send|" + subquery + "||SupportProductIdEvent||{\"productID\":\"A1\"}|||||||",
                "build-error|" + subquery + "|subquery-context-mismatch||" + EPL_SUBQUERY_PROBE
                        + "||" + EPL_SUBQUERY_PROBE_ERROR + "||||||",
                "undeploy-all|" + subquery + "|||||||||||"
        });
        String sameEvent = "terminating-same-event";
        CASE_STEPS.put(sameEvent, new String[]{
                // Java compiles the context and s0 as ONE module (non-@public
                // C1 is invisible across modules), so the scenario carries a
                // single deploy step keyed by the traced statement s0.
                "deploy|" + sameEvent + "|s0||" + EPL_SAME_EVENT_CTX + ";\n" + EPL_SAME_EVENT_S0 + "||||||||",
                "send|" + sameEvent + "||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
                "send|" + sameEvent + "||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":1}|||||||",
                "undeploy-all|" + sameEvent + "|||||||||||"
        });
    }

    private static final int EXPECTED_STEPS = 18;
    private static final int EXPECTED_RECORDS = 4;

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextInitTermPrioritizedScenarioOracle <scenario.json>");
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
     * RegressionPath) and attach the listener to the case's traced
     * statement (out or s0), mirroring env.addListener.  The build-error
     * step compiles the pinned probe against the same path, mirroring
     * env.tryInvalidCompile(path, ...).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> productType = new HashMap<>();
        productType.put("productID", String.class);
        configuration.getCommon().addEventType("SupportProductIdEvent", productType);
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
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(string(step, "at")).toEpochMilli());
                        break;
                    case "deploy": {
                        String epl = string(step, "epl");
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        compilerArgs.getPath().getCompileds().addAll(deployedModules);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if (LISTENERS[caseIndex].equals(statement.getName())) {
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
                    case "build-error":
                        buildErrorStep(configuration, deployedModules, caseName, step, records);
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
     * Compiles an expected-invalid probe against the accumulated module
     * path (env.tryInvalidCompile(path, ...)) and emits
     * {"operation":"compile-error"} carrying the pinned expectError prefix
     * after verifying the caught message starts with it
     * (SupportMessageAssertUtil.assertMessage semantics).
     */
    private static void buildErrorStep(Configuration configuration, List<EPCompiled> deployedModules,
                                       String caseName, JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            compilerArgs.getPath().getCompileds().addAll(deployedModules);
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

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * Every delivery is a single new-only row carrying the full event
     * property set (productID for the window row; theString/intPrimitive
     * for the SupportBean row).
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
                Map<String, Object> event = new HashMap<>();
                event.put("theString", nullableString(payload, "theString"));
                event.put("intPrimitive", integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "SupportProductIdEvent": {
                requirePayloadFields(payload, "productID");
                Map<String, Object> event = new HashMap<>();
                event.put("productID", nullableString(payload, "productID"));
                runtime.getEventService().sendEventMap(event, type);
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
