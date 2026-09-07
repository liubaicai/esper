import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.EPUndeployException;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableSubqCorrelCoerce: correlated scalar subqueries
 * with int-to-long coercion, the col2 = es.e2 string equality and the col1 =
 * es.e1 coerced equality probing a keepall named window or a primary-key table
 * through implicit subquery indexes, the enable_window_subquery_indexshare
 * create hint, the disable_window_subquery_indexshare consumer hint, and the
 * explicit two-key create index MyIndex (col2, col1).
 *
 * Replays the eight executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: the two map schemas are created by
 * deploying the @public @buseventtype create schema statements exactly as the
 * source does, internal timer disabled, and the rethrowing exception handler so
 * statement failures surface to the sender thread.  The listener attaches to
 * the s0 consume statement only (schema, create, insert, and index deploy
 * silently) with a per-case sequence counter that continues across the
 * mid-case undeploy and redeploy of s0, so the redeployed statement's first
 * record is sequence 7 and proves the subquery index repopulates from the
 * existing infra contents.  Mid-case undeploys resolve the owning deployment
 * by statement name, mirroring the harness undeployModuleContaining.  Records
 * carry a new array only (default istream selector) with sorted field names.
 */
public final class InfraNWTableSubqCorrelCoerceScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-subq-correl-coerce";
    private static final String DESCRIPTION =
            "InfraNWTableSubqCorrelCoerce correlated scalar subqueries with int-to-long coercion: the col2 = es.e2 "
                    + "string equality and col1 = es.e1 coerced equality probe a keepall named window or a primary-key "
                    + "table through implicit subquery indexes, shared indexes via the enable_window_subquery_indexshare "
                    + "create hint, disable_window_subquery_indexshare consumer hints, and explicit two-key create index "
                    + "MyIndex (col2, col1), including a mid-case statement undeploy and redeploy proving the subquery "
                    + "index repopulates from existing infra contents (Java source regression-lib/src/main/java/"
                    + "com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelCoerce.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableSubqCorrelCoerce.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-0f3833a87dcd9b209935",
            "java-runtime-f1f621deec2314404f2a",
            "java-runtime-4f850a69b870e08ebfd5",
            "java-runtime-1616d20fc73fe4be679c",
            "java-runtime-0422895f8568f52e0956",
            "java-runtime-9161da818506b8194079",
            "java-runtime-3a94283dcac4e9ae4dea",
            "java-runtime-ce6e53f117adccdf9f59"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=false}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=true}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=false}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=true}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=true, createExplicitIndex=false}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=true, createExplicitIndex=true}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=false, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=false}",
            "InfraNWTableSubqCorrelCoerceSimple{namedWindow=false, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false, createExplicitIndex=true}"
    };
    private static final String[] STATIC_IDS = {
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660",
            "java-5a8fbdfa647a5bef3660"
    };

    private static final String CASE_NW_NO_SHARE = "nw-no-share";
    private static final String CASE_NW_NO_SHARE_INDEX = "nw-no-share-index";
    private static final String CASE_NW_SHARE = "nw-share";
    private static final String CASE_NW_SHARE_INDEX = "nw-share-index";
    private static final String CASE_NW_SHARE_DISABLE = "nw-share-disable";
    private static final String CASE_NW_SHARE_DISABLE_INDEX = "nw-share-disable-index";
    private static final String CASE_TABLE_NO_SHARE = "table-no-share";
    private static final String CASE_TABLE_NO_SHARE_INDEX = "table-no-share-index";

    // Verbatim transcriptions of InfraNWTableSubqCorrelCoerce lines 56-57, 64-66,
    // 68-69, 71, 74, 78, and 80; c1, c2, create, and insert are unnamed in the
    // Java source and the schema deploy statements carry the source's local
    // variable names as statement labels.
    private static final String EPL_SCHEMA_EVENT =
            "@public @buseventtype create schema EventSchema(e0 string, e1 int, e2 string)";
    private static final String EPL_SCHEMA_WINDOW =
            "@public @buseventtype create schema WindowSchema(col0 string, col1 long, col2 string)";
    private static final String EPL_CREATE_NW =
            "@public create window MyInfra#keepall as WindowSchema";
    private static final String EPL_CREATE_NW_INDEX_SHARE =
            "@Hint('enable_window_subquery_indexshare') " + EPL_CREATE_NW;
    private static final String EPL_CREATE_TABLE =
            "@public create table MyInfra (col0 string primary key, col1 long, col2 string)";
    private static final String EPL_INSERT =
            "insert into MyInfra select * from WindowSchema";
    private static final String EPL_INDEX =
            "@name('index') create index MyIndex on MyInfra (col2, col1)";
    private static final String EPL_CONSUME =
            "@name('s0') select e0, (select col0 from MyInfra where col2 = es.e2 and col1 = es.e1) "
                    + "as val from EventSchema es";
    private static final String EPL_CONSUME_DISABLE_SHARE =
            "@Hint('disable_window_subquery_indexshare') " + EPL_CONSUME;

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(Arrays.asList("s0"));
    private static final int EXPECTED_RECORDS = 56;
    private static final int EXPECTED_STEPS = 176;

    private InfraNWTableSubqCorrelCoerceScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableSubqCorrelCoerceScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_NW_NO_SHARE, runtime, allSteps, records);
            runCase(CASE_NW_NO_SHARE_INDEX, runtime, allSteps, records);
            runCase(CASE_NW_SHARE, runtime, allSteps, records);
            runCase(CASE_NW_SHARE_INDEX, runtime, allSteps, records);
            runCase(CASE_NW_SHARE_DISABLE, runtime, allSteps, records);
            runCase(CASE_NW_SHARE_DISABLE_INDEX, runtime, allSteps, records);
            runCase(CASE_TABLE_NO_SHARE, runtime, allSteps, records);
            runCase(CASE_TABLE_NO_SHARE_INDEX, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
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

    /** Replays one case's steps on the shared runtime; sequences restart per case. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (string(step, "op")) {
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        if (LISTENED_STATEMENTS.contains(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                    }
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "undeploy":
                    undeployModuleContaining(runtime, string(step, "statement"));
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Resolves the deployment owning the named statement and undeploys it,
     * mirroring RegressionEnvironmentBase.undeployModuleContaining for the
     * source's env.undeployModuleContaining("s0") and ("index") calls.
     */
    private static void undeployModuleContaining(EPRuntime runtime, String statementName)
            throws EPUndeployException {
        for (String deploymentId : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deploymentId);
            for (EPStatement statement : info.getStatements()) {
                if (statement.getName().equals(statementName)) {
                    runtime.getDeploymentService().undeploy(deploymentId);
                    return;
                }
            }
        }
        throw new IllegalStateException("Failed to find deployment with statement '"
                + statementName + "'");
    }

    /**
     * Listener emitting one record per invocation with a per-case sequence
     * counter keyed by statement name; the counter therefore continues across
     * the mid-case undeploy and redeploy of s0 (fresh listener, sequence 7),
     * and the default istream selector means only a new array renders.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
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

    /**
     * Sends the map events exactly as the source's sendWindow and sendEvent
     * helpers do against the deployed map schemas: col1 as long and e1 as int
     * (the int-to-long coercion under test happens inside the engine).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "WindowSchema": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("col0", string(payload, "col0"));
                event.put("col1", longInteger(payload.get("col1"), "col1"));
                event.put("col2", string(payload, "col2"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "EventSchema": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("e0", string(payload, "e0"));
                event.put("e1", (int) longInteger(payload.get("e1"), "e1"));
                event.put("e2", string(payload, "e2"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        String[] expectedCases = {
                CASE_NW_NO_SHARE, CASE_NW_NO_SHARE_INDEX, CASE_NW_SHARE, CASE_NW_SHARE_INDEX,
                CASE_NW_SHARE_DISABLE, CASE_NW_SHARE_DISABLE_INDEX, CASE_TABLE_NO_SHARE,
                CASE_TABLE_NO_SHARE_INDEX};
        int[] expectedOrdinals = {0, 1, 2, 3, 4, 5, 6, 7};
        String[] expectedEpls = {
                EPL_CONSUME, EPL_CONSUME, EPL_CONSUME, EPL_CONSUME,
                EPL_CONSUME_DISABLE_SHARE, EPL_CONSUME_DISABLE_SHARE, EPL_CONSUME, EPL_CONSUME};
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !expectedEpls[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCase(steps, offset, CASE_NW_NO_SHARE, EPL_CREATE_NW, EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_NW_NO_SHARE_INDEX, EPL_CREATE_NW,
                EPL_CONSUME, true);
        offset = validateCase(steps, offset, CASE_NW_SHARE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_NW_SHARE_INDEX, EPL_CREATE_NW_INDEX_SHARE,
                EPL_CONSUME, true);
        offset = validateCase(steps, offset, CASE_NW_SHARE_DISABLE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_CONSUME_DISABLE_SHARE, false);
        offset = validateCase(steps, offset, CASE_NW_SHARE_DISABLE_INDEX,
                EPL_CREATE_NW_INDEX_SHARE, EPL_CONSUME_DISABLE_SHARE, true);
        offset = validateCase(steps, offset, CASE_TABLE_NO_SHARE, EPL_CREATE_TABLE,
                EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_TABLE_NO_SHARE_INDEX, EPL_CREATE_TABLE,
                EPL_CONSUME, true);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact per-case step sequence mirroring InfraNWTableSubqCorrelCoerce
     * lines 55-119: the two schema deploys, the create statement, the insert,
     * the optional explicit two-key index deploy, the consume statement, the
     * ten population and probe sends, the undeploy of the deployment
     * containing s0, the redeployed consume, the late E6 send, the second s0
     * undeploy, the optional index undeploy, and undeployAll.
     */
    private static int validateCase(JsonArray steps, int offset, String caseName,
                                    String createEpl, String consumeEpl, boolean withIndex) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "c1", EPL_SCHEMA_EVENT);
        validateDeploy(steps.get(offset++), caseName, "c2", EPL_SCHEMA_WINDOW);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        if (withIndex) {
            validateDeploy(steps.get(offset++), caseName, "index", EPL_INDEX);
        }
        validateDeploy(steps.get(offset++), caseName, "consume", consumeEpl);
        validateWindowSend(steps.get(offset++), caseName, "W1", 10, "c31");
        validateEventSend(steps.get(offset++), caseName, "E1", 10, "c31");
        validateEventSend(steps.get(offset++), caseName, "E2", 11, "c32");
        validateWindowSend(steps.get(offset++), caseName, "W2", 11, "c32");
        validateEventSend(steps.get(offset++), caseName, "E3", 11, "c32");
        validateWindowSend(steps.get(offset++), caseName, "W3", 11, "c31");
        validateWindowSend(steps.get(offset++), caseName, "W4", 10, "c32");
        validateEventSend(steps.get(offset++), caseName, "E4", 11, "c31");
        validateEventSend(steps.get(offset++), caseName, "E5", 10, "c31");
        validateEventSend(steps.get(offset++), caseName, "E6", 10, "c32");
        validateUndeploy(steps.get(offset++), caseName, "s0");
        validateDeploy(steps.get(offset++), caseName, "consume", consumeEpl);
        validateEventSend(steps.get(offset++), caseName, "E6", 10, "c32");
        validateUndeploy(steps.get(offset++), caseName, "s0");
        if (withIndex) {
            validateUndeploy(steps.get(offset++), caseName, "index");
        }
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateWindowSend(JsonValue value, String caseName, String expectedCol0,
                                           long expectedCol1, String expectedCol2) {
        JsonObject step = object(value, "WindowSchema step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"WindowSchema".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("WindowSchema step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "WindowSchema payload");
        requireFields(payload, "col0", "col1", "col2");
        if (!expectedCol0.equals(string(payload, "col0"))
                || longInteger(payload.get("col1"), "col1") != expectedCol1
                || !expectedCol2.equals(string(payload, "col2"))) {
            throw new IllegalArgumentException("WindowSchema payload is not pinned for " + caseName);
        }
    }

    private static void validateEventSend(JsonValue value, String caseName, String expectedE0,
                                          long expectedE1, String expectedE2) {
        JsonObject step = object(value, "EventSchema step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"EventSchema".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("EventSchema step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "EventSchema payload");
        requireFields(payload, "e0", "e1", "e2");
        if (!expectedE0.equals(string(payload, "e0"))
                || longInteger(payload.get("e1"), "e1") != expectedE1
                || !expectedE2.equals(string(payload, "e2"))) {
            throw new IllegalArgumentException("EventSchema payload is not pinned for " + caseName);
        }
    }

    private static void validateUndeploy(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "undeploy step");
        requireFields(step, "op", "case", "statement");
        if (!"undeploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("undeploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
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
