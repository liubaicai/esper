import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
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
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for ContextKeySegmentedSubselectPrevPrior (ord 8).  One case
 * replays on one fresh runtime:
 *
 * subselect-prev-prior (ordinal 8, ContextKeySegmentedSubselectPrevPrior):
 * deploys "@Name('context') @public create context SegmentedByString
 * partition by theString from SupportBean" and "@Name('s0') context
 * SegmentedByString select theString, (select prev(0, id) from
 * SupportBean_S0#keepall) as col1 from SupportBean" with a listener on s0.
 * SupportBean_S0 is not in the partition spec, so its events broadcast into
 * every existing partition's subquery keepall without allocating
 * partitions; new partitions start empty and a multi-row subquery yields
 * null.  The send sequence delivers {G1,null}, {G1,1}, {G2,null}, {G2,2},
 * {G1,null}; s0 is then undeployed (undeployModuleContaining) and
 * redeployed with prior(0, id), replaying the identical sequence with the
 * identical expected values.  "types" steps pin the asserted select-clause
 * property types theString String and col1 Integer.
 */
public final class ContextKeySegmentedSubselectPrevPriorScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-key-segmented-subselect-prev-prior";
    private static final String DESCRIPTION =
            "ContextKeySegmentedSubselectPrevPrior (ord 8): a single-type segmented context partitions "
                    + "SupportBean by theString; statement s0 selects theString plus a scalar subquery "
                    + "over the unlisted SupportBean_S0#keepall (prev(0, id), then prior(0, id) after "
                    + "undeployModuleContaining). S0 events are not in the partition spec, so they "
                    + "broadcast into every existing partition's subquery window without allocating "
                    + "partitions; new partitions start empty and a multi-row subquery yields null. "
                    + "The undeploy+redeploy round resets partition state and replays the same send "
                    + "sequence with the same expected values.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextKeySegmented.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-2afbd86618b752a6dd5b"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextKeySegmentedSubselectPrevPrior"
    };
    private static final String[] STATIC_IDS = {
            "java-1dbfa1926e47afc1a7b4"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "subselect-prev-prior"
    };
    private static final int[] ORDINALS = {8};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+types; s0's scalar subquery over SupportBean_S0#keepall resolves per partition: "
                    + "G1 sees prev/prior(0,id)=1 after S0(1,E1) broadcasts into the existing G1 "
                    + "partition, G2 starts empty (null) then sees 2 after S0(2,E2), and G1's second "
                    + "send returns null once its keepall holds two rows (multi-row scalar -> null); "
                    + "undeployModuleContaining('s0') plus redeploy with prior(0,id) resets partition "
                    + "state and reproduces the identical sequence"
    };

    // Verbatim transcriptions of ContextKeySegmented lines 761, 764-765 and
    // 788-789.
    private static final String EPL_CONTEXT =
            "@Name('context') @public create context SegmentedByString partition by theString "
                    + "from SupportBean";
    private static final String EPL_PREV =
            "@Name('s0') context SegmentedByString "
                    + "select theString, (select prev(0, id) from SupportBean_S0#keepall) as col1 "
                    + "from SupportBean";
    private static final String EPL_PRIOR =
            "@Name('s0') context SegmentedByString "
                    + "select theString, (select prior(0, id) from SupportBean_S0#keepall) as col1 "
                    + "from SupportBean";

    private static final String[] CASE_EPLS = {
            EPL_PREV
    };

    // Java-asserted event-type surface per case/statement: the asserted
    // property name-to-type pairs of the s0 select clause.
    private static final Map<String, String[][]> EXPECTED_PROPERTIES = new HashMap<>();
    static {
        EXPECTED_PROPERTIES.put("subselect-prev-prior/s0",
                new String[][]{{"theString", "String"}, {"col1", "Integer"}});
    }

    private static final int EXPECTED_STEPS = 22;
    private static final int EXPECTED_RECORDS = 12;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * send payloads render as their compact JSON.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("subselect-prev-prior", new String[]{
                "deploy|subselect-prev-prior|ctx||" + EPL_CONTEXT + "|||||||",
                "deploy|subselect-prev-prior|s0||" + EPL_PREV + "|||||||",
                "types|subselect-prev-prior|s0|||||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":10}||||||",
                "send|subselect-prev-prior||SupportBean_S0||{\"id\":1,\"p00\":\"E1\"}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":11}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":20}||||||",
                "send|subselect-prev-prior||SupportBean_S0||{\"id\":2,\"p00\":\"E2\"}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":21}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":12}||||||",
                "undeploy|subselect-prev-prior|s0|||||||||",
                "deploy|subselect-prev-prior|s0||" + EPL_PRIOR + "|||||||",
                "types|subselect-prev-prior|s0|||||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":10}||||||",
                "send|subselect-prev-prior||SupportBean_S0||{\"id\":1,\"p00\":\"E1\"}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":11}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":20}||||||",
                "send|subselect-prev-prior||SupportBean_S0||{\"id\":2,\"p00\":\"E2\"}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":21}||||||",
                "send|subselect-prev-prior||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":12}||||||",
                "undeploy-all|subselect-prev-prior||||||||||",
        });
    }

    private ContextKeySegmentedSubselectPrevPriorScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextKeySegmentedSubselectPrevPriorScenarioOracle <scenario.json>");
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
     * Replays the case's steps on a fresh runtime.  Deploys register their
     * deployment under the step label so the targeted undeploy can resolve
     * it; the listener attaches to the statement named s0, mirroring
     * env.addListener.
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
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            Map<String, EPDeployment> deployments = new HashMap<>();
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
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        EPCompiled compiled = compileModule(epl, configuration, runtime);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
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
                    case "types":
                        typesStep(caseName, statements, string(step, "statement"), records,
                                runtime);
                        break;
                    case "undeploy": {
                        String label = string(step, "statement");
                        EPDeployment deployment = deployments.get(label);
                        if (deployment == null) {
                            throw new IllegalStateException("undeploy label " + label
                                    + " was not deployed in case " + caseName);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        deployments.remove(label);
                        statements.values().removeIf(
                                statement -> statement.getDeploymentId()
                                        .equals(deployment.getDeploymentId()));
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployments.clear();
                        statements.clear();
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
     * the configuration plus the already-deployed module path so s0
     * resolves the @public SegmentedByString context.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Emits a {"operation":"types"} record carrying exactly the
     * Java-asserted event-type surface for the statement: the asserted
     * property name-to-type pairs.  The actual event type is verified
     * against the pinned surface before recording so a drift fails the
     * oracle (env.assertStatement semantics).
     */
    private static void typesStep(String caseName, Map<String, EPStatement> statements,
                                  String label, JsonArray records, EPRuntime runtime) {
        EPStatement statement = statements.get(label);
        if (statement == null) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        String key = caseName + "/" + label;
        EventType eventType = statement.getEventType();
        JsonObject value = new JsonObject();
        String[][] properties = EXPECTED_PROPERTIES.get(key);
        if (properties != null) {
            JsonObject pinned = new JsonObject();
            for (String[] pair : properties) {
                Class<?> propertyType = eventType.getPropertyType(pair[0]);
                String actual = propertyType == null ? "null" : propertyType.getSimpleName();
                if (!pair[1].equals(actual)) {
                    throw new IllegalStateException("property type drift for " + key + "."
                            + pair[0] + ": expected " + pair[1] + " got " + actual);
                }
                pinned.add(pair[0], pair[1]);
            }
            value.add("properties", pinned);
        }
        if (value.size() == 0) {
            throw new IllegalStateException("types step for " + key + " pins no asserted surface");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * Each delivery must be a single new-only row rendering the sorted
     * property surface {col1, theString}; col1 is null when the subquery
     * is empty or multi-row.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           EPStatement statement) {
        return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + caseName
                        + " listener callback must contain one new-only row");
            }
            EventBean event = newEvents[0];
            JsonObject row = fullRow(event);
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", new JsonArray().add(row));
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
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
                requireFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                requireFields(payload, "id", "p00");
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"),
                        nullableString(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
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
