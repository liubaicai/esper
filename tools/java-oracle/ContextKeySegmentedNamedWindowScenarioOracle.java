import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportGroupSubgroupEvent;
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
 * context-key-segmented-named-window scenario (ContextKeySegmentedNamedWindow
 * ordinals 0 and 3).  The oracle replays the checked-in scenario JSON against
 * the pinned Esper 9.0.0 runtime and emits one JSON record per observable
 * listener delivery or fire-and-forget result; the Go parity runner replays
 * the same scenario and the differential comparator requires byte-identical
 * normalized records.
 * <p>Case keyed-named-window-basic (ord 0, ContextKeyedNamedWindowBasic):
 * deploys the verbatim four-statement module (two-key context Ctx over
 * grp/subGrp, a contexted unique(type) window EventData, the contexted
 * insert-into, and the contexted irstream consumer Test), sends one
 * SupportGroupSubgroupEvent(G1,SG1,1,10.45), and records the single new-only
 * Test delivery.  Case keyed-named-window-faf (ord 3,
 * ContextKeyedNamedWindowFAF): deploys the three statements through the
 * shared module path (env.compileDeploy(epl, path) semantics), sends
 * SupportBean G1/G2, and records the no-selector executeQuery rows after
 * each send.  The faf record carries the full sorted-property row because
 * the Java assertion reads only theString while the observable row is the
 * whole SupportBean event.
 */
public class ContextKeySegmentedNamedWindowScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-key-segmented-named-window";
    private static final String DESCRIPTION =
            "ContextKeySegmentedNamedWindow ords 0 and 3: a two-key keyed context over a unique(type) named window with a contexted insert-into and an irstream consumer (basic), and a keyed context over a keepall named window answering a no-selector fire-and-forget select * (FAF).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedNamedWindow.java";
    private static final String[] CASES = {
            "keyed-named-window-basic",
            "keyed-named-window-faf"
    };
    private static final int[] ORDINALS = {0, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-18f8400337cdcd1dffd3",
            "java-runtime-73bbdb8596de168d6d94"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextKeyedNamedWindowBasic",
            "ContextKeyedNamedWindowFAF"
    };
    private static final String[] STATIC_IDS = {
            "java-0554274bf96ee5ab94ce",
            "java-bf2785808c45262f93ce"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};

    // Verbatim transcriptions of ContextKeySegmentedNamedWindow lines 62-67
    // (basic) and 41-45 (faf).
    private static final String EPL_BASIC =
            "@Audit @Name('CTX') create context Ctx partition by grp, subGrp from SupportGroupSubgroupEvent;\n"
                    + "@Audit @Name('Window') context Ctx create window EventData#unique(type) as SupportGroupSubgroupEvent;"
                    + "@Audit @Name('Insert') context Ctx insert into EventData select * from SupportGroupSubgroupEvent;"
                    + "@Audit @Name('Test') context Ctx select irstream * from EventData;";
    private static final String EPL_FAF_CONTEXT =
            "@public create context SegmentedByString partition by theString from SupportBean";
    private static final String EPL_FAF_WINDOW =
            "@public context SegmentedByString create window MyWindow#keepall as SupportBean";
    private static final String EPL_FAF_INSERT =
            "context SegmentedByString insert into MyWindow select * from SupportBean";
    private static final String EPL_FAF_QUERY = "select * from MyWindow";

    private static final String[] CASE_OBSERVATIONS = {
            "listener; two-key keyed context Ctx (grp, subGrp) over a unique(type) named window fed by the contexted insert; one SupportGroupSubgroupEvent(G1,SG1,1,10.45) send delivers one new-only irstream row to Test",
            "faf; keyed SegmentedByString context over a keepall named window fed by the contexted insert; the no-selector executeQuery returns the G1 partition row after one send and both partition rows after the G2 send"
    };
    private static final String[] CASE_EPLS = {
            EPL_BASIC,
            EPL_FAF_CONTEXT + "\n" + EPL_FAF_WINDOW + "\n" + EPL_FAF_INSERT + "\n" + EPL_FAF_QUERY
    };

    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("keyed-named-window-basic", new String[]{
                "deploy|keyed-named-window-basic|module||" + EPL_BASIC + "|||||||",
                "send|keyed-named-window-basic||SupportGroupSubgroupEvent||{\"grp\":\"G1\",\"subGrp\":\"SG1\",\"type\":1,\"value\":10.45}||||||",
                "undeploy-all|keyed-named-window-basic||||||||||"
        });
        CASE_STEPS.put("keyed-named-window-faf", new String[]{
                "deploy|keyed-named-window-faf|module||" + EPL_FAF_CONTEXT + "|||||||",
                "deploy|keyed-named-window-faf|module||" + EPL_FAF_WINDOW + "|||||||",
                "deploy|keyed-named-window-faf|module||" + EPL_FAF_INSERT + "|||||||",
                "send|keyed-named-window-faf||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":0}||||||",
                "faf|keyed-named-window-faf|faf||" + EPL_FAF_QUERY + "|||||none||",
                "send|keyed-named-window-faf||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":0}||||||",
                "faf|keyed-named-window-faf|faf||" + EPL_FAF_QUERY + "|||||none||",
                "undeploy-all|keyed-named-window-faf||||||||||"
        });
    }

    private static final int EXPECTED_STEPS = 13;
    private static final int EXPECTED_RECORDS = 3;

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextKeySegmentedNamedWindowScenarioOracle <scenario.json>");
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
     * RegressionPath) and attach the listener to the statement named Test,
     * mirroring env.addListener (the context/window/insert statements
     * deploy unlistened).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportGroupSubgroupEvent.class);
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
                            if ("Test".equals(statement.getName())) {
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
                    case "faf":
                        fafStep(runtime, configuration, caseName, step, records, deployedModules);
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
     * Replays the fire-and-forget step, mirroring the Java execution's
     * env.compileFAF(query, path) plus the bare executeQuery(compiled): the
     * query compiles through compileQuery with the accumulated module path
     * and executes without a selector, which Esper resolves to every live
     * partition.  The emitted faf record carries the full row of each
     * returned event in result order.
     */
    private static void fafStep(EPRuntime runtime, Configuration configuration, String caseName,
                                JsonObject step, JsonArray records,
                                List<EPCompiled> deployedModules) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        if (!EPL_FAF_QUERY.equals(epl)) {
            throw new IllegalStateException("unsupported faf epl " + epl);
        }
        if (!"none".equals(string(step, "selector"))) {
            throw new IllegalStateException("unsupported faf selector "
                    + string(step, "selector"));
        }
        CompilerArguments fafArgs = new CompilerArguments(configuration);
        fafArgs.getPath().getCompileds().addAll(deployedModules);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery(epl, fafArgs);
        EPFireAndForgetQueryResult result = runtime.getFireAndForgetService()
                .executeQuery(compiled);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "faf");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = new JsonArray();
        for (EventBean event : result.getArray()) {
            rows.add(fullRow(event));
        }
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * The basic case's single send delivers one new-only row carrying the
     * four SupportGroupSubgroupEvent properties.
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
            case "SupportGroupSubgroupEvent": {
                requirePayloadFields(payload, "grp", "subGrp", "type", "value");
                SupportGroupSubgroupEvent bean = new SupportGroupSubgroupEvent(
                        nullableString(payload, "grp"),
                        nullableString(payload, "subGrp"),
                        integer(payload, "type"),
                        payload.get("value").asDouble());
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
