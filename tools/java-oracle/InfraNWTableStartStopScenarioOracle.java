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
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableStartStop: consumer and inserter start/stop
 * lifecycles over a keepall named window and over a primary-key table.
 *
 * Replays the four executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean from esper-common,
 * internal timer disabled, and the rethrowing exception handler so statement
 * failures surface to the sender thread.  Listeners attach to create and
 * select only (create table is silent and the insert into is never listened)
 * with per-statement sequence counters that restart per case and persist
 * across an intra-case undeploy/redeploy of the same statement name.  Mid-case
 * undeploys resolve the owning deployment by statement name, mirroring the
 * harness undeployModuleContaining.  Iterator assertions render as snapshot
 * records with sequence 0; "mode":"any" steps mirror the harness
 * AnyOrder comparisons while plain steps compare positionally.
 */
public final class InfraNWTableStartStopScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-start-stop";
    private static final String DESCRIPTION =
            "InfraNWTableStartStop consumer and inserter start/stop lifecycles over a keepall "
                    + "named window and over a primary-key table: named-window istream listeners with silent "
                    + "table listeners, mid-case undeploy and redeploy of the insert and select statements "
                    + "resolved by statement name, iterator snapshots of the infra contents after each "
                    + "start/stop transition, and the redeployed consumer immediately observing current "
                    + "window contents (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableStartStop.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableStartStop.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-e14796f7892a57096988",
            "java-runtime-7ef0dbf9f8ea7e311720",
            "java-runtime-6c0691c9d5549b937cfc",
            "java-runtime-683b282c41f311ed8429"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraStartStopConsumer{namedWindow=true}",
            "InfraStartStopConsumer{namedWindow=false}",
            "InfraStartStopInserter{namedWindow=true}",
            "InfraStartStopInserter{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-2fe58125cb5f265d5fd1",
            "java-2fe58125cb5f265d5fd1",
            "java-c4a5793bec169880d651",
            "java-c4a5793bec169880d651"
    };

    private static final String CASE_CONSUMER_NW = "consumer-nw";
    private static final String CASE_CONSUMER_TABLE = "consumer-table";
    private static final String CASE_INSERTER_NW = "inserter-nw";
    private static final String CASE_INSERTER_TABLE = "inserter-table";

    // Verbatim transcriptions of InfraNWTableStartStop lines 46-48, 52, 57,
    // 114-116, 120, and 125, including the capital-N annotation of the
    // consumer-variant select and the unnamed consumer-variant insert.
    private static final String EPL_CREATE_NW =
            "@name('create') @public create window MyInfra#keepall as "
                    + "select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_TABLE =
            "@name('create') @public create table MyInfra(a string primary key, b int primary key)";
    private static final String EPL_INSERT_NAMED =
            "@name('insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_INSERT_UNNAMED =
            "insert into MyInfra select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_SELECT_INSERTER = "@name('select') select a, b from MyInfra as s1";
    private static final String EPL_SELECT_CONSUMER = "@Name('select') select a, b from MyInfra as s1";

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("create", "select"));
    private static final int EXPECTED_RECORDS = 24;
    private static final int EXPECTED_STEPS = 62;

    private InfraNWTableStartStopScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableStartStopScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_CONSUMER_NW, runtime, allSteps, records);
            runCase(CASE_CONSUMER_TABLE, runtime, allSteps, records);
            runCase(CASE_INSERTER_NW, runtime, allSteps, records);
            runCase(CASE_INSERTER_TABLE, runtime, allSteps, records);
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
        Map<String, EPStatement> statementsByName = new HashMap<>();
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
                        statementsByName.put(statement.getName(), statement);
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
                case "snapshot": {
                    EPStatement statement = statementsByName.get(string(step, "statement"));
                    if (statement == null) {
                        throw new IllegalStateException("snapshot targets unknown statement in case "
                                + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName));
                    break;
                }
                case "undeploy":
                    undeployModuleContaining(runtime, string(step, "statement"), statementsByName);
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Resolves the deployment owning the named statement and undeploys it,
     * mirroring RegressionEnvironmentBase.undeployModuleContaining.
     */
    private static void undeployModuleContaining(EPRuntime runtime, String statementName,
                                                 Map<String, EPStatement> statementsByName)
            throws EPUndeployException {
        for (String deploymentId : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deploymentId);
            for (EPStatement statement : info.getStatements()) {
                if (statement.getName().equals(statementName)) {
                    runtime.getDeploymentService().undeploy(deploymentId);
                    for (EPStatement removed : info.getStatements()) {
                        statementsByName.remove(removed.getName());
                    }
                    return;
                }
            }
        }
        throw new IllegalStateException("Failed to find deployment with statement '"
                + statementName + "'");
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty.
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

    /**
     * Snapshot of the statement iterator: one record with sequence 0 holding
     * the rows in the engine's iteration order, mirroring the generic
     * differential replay convention where listener records carry per-statement
     * sequences and snapshot records do not.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        List<EventBean> current = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            current.add(iterator.next());
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("new", rows(current.toArray(new EventBean[0])));
        return record;
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if (!"SupportBean".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        SupportBean bean = new SupportBean();
        bean.setTheString(string(payload, "theString"));
        bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
        runtime.getEventService().sendEventBean(bean, type);
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
        validateStringArray(scenario.get("javaFlags"), new String[]{"OBSERVEROPS"}, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        String[] expectedCases = {
                CASE_CONSUMER_NW, CASE_CONSUMER_TABLE, CASE_INSERTER_NW, CASE_INSERTER_TABLE};
        int[] expectedOrdinals = {0, 1, 2, 3};
        int[] expectedSnapshots = {5, 4, 3, 2};
        String[] expectedEpls = {
                EPL_CREATE_NW, EPL_CREATE_TABLE, EPL_CREATE_NW, EPL_CREATE_TABLE};
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != expectedSnapshots[index]
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
        offset = validateConsumerCase(steps, offset, CASE_CONSUMER_NW, EPL_CREATE_NW);
        offset = validateConsumerCase(steps, offset, CASE_CONSUMER_TABLE, EPL_CREATE_TABLE);
        offset = validateInserterCase(steps, offset, CASE_INSERTER_NW, EPL_CREATE_NW);
        offset = validateInserterCase(steps, offset, CASE_INSERTER_TABLE, EPL_CREATE_TABLE);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact consumer-case step sequence mirroring InfraStartStopConsumer lines
     * 111-168: the consumer select is undeployed after E1 and redeployed before
     * E3, and only the named-window variant records istream listener output.
     */
    private static int validateConsumerCase(JsonArray steps, int offset, String caseName,
                                            String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_UNNAMED);
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_CONSUMER);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "create", null);
        validateUndeploy(steps.get(offset++), caseName, "select");
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_CONSUMER);
        validateSnapshot(steps.get(offset++), caseName, "select", "any");
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        if (CASE_CONSUMER_NW.equals(caseName)) {
            validateSnapshot(steps.get(offset++), caseName, "select", null);
        }
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateUndeploy(steps.get(offset++), caseName, "select");
        validateBeanSend(steps.get(offset++), caseName, "E4", 4);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact inserter-case step sequence mirroring InfraStartStopInserter lines
     * 43-94: the named insert is undeployed after E1 and redeployed before E3,
     * so E2 and E4 never reach the infra.
     */
    private static int validateInserterCase(JsonArray steps, int offset, String caseName,
                                            String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_NAMED);
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_INSERTER);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "create", null);
        validateUndeploy(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_NAMED);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        if (CASE_INSERTER_NW.equals(caseName)) {
            validateSnapshot(steps.get(offset++), caseName, "select", null);
        }
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateUndeploy(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E4", 4);
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

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        if (expectedMode == null) {
            requireFields(step, "op", "case", "statement");
        } else {
            requireFields(step, "op", "case", "statement", "mode");
        }
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        if (expectedMode != null && !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot mode is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUndeploy(JsonValue value, String caseName, String expectedStatement) {
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
