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
import com.espertech.esper.common.internal.support.SupportBean_A;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableOnDelete: on-trigger delete semantics over a
 * keepall named window and over a primary-key table.  Covers where-clause
 * deletes scoped to trigger-event versus infra-row properties (concat and
 * range predicates), pattern-triggered unconditional delete-all, and
 * event-triggered delete-all with deleted rows delivered as new data to the
 * on-delete listener.
 *
 * Replays the six executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean and SupportBean_A from
 * esper-common plus a local mirror of the regression SupportBean_B
 * (regression-lib is not on the oracle classpath), internal timer disabled,
 * and the rethrowing exception handler so statement failures surface to the
 * sender thread.  Listeners attach to CreateInfra, OnDelete, and Select with
 * per-statement sequence counters that restart per case; deployed markers,
 * iterator snapshots, and fire-and-forget count reads are emitted at their
 * step positions.  Milestone calls are HA checkpoint no-ops and emit nothing.
 */
public final class InfraNWTableOnDeleteScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-delete";
    private static final String DESCRIPTION =
            "InfraNWTableOnDelete on-trigger delete semantics over a keepall named window and over a "
                    + "primary-key table: where-clause deletes scoped to trigger-event versus infra-row "
                    + "properties (concat and range predicates), pattern-triggered unconditional delete-all, "
                    + "and event-triggered delete-all with deleted rows delivered as new data to the "
                    + "on-delete listener (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableOnDelete.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnDelete.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-6d190be4d9e8b76b11f3",
            "java-runtime-992fcd3db7f15b8254cd",
            "java-runtime-2ab09353c939b15ee52e",
            "java-runtime-e795679c4403d309f805",
            "java-runtime-11809b3f1d90b37d52b8",
            "java-runtime-b7a9b1b53c0009e7d347"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraDeleteCondition{namedWindow=true}",
            "InfraDeleteCondition{namedWindow=false}",
            "InfraDeletePattern{namedWindow=true}",
            "InfraDeletePattern{namedWindow=false}",
            "InfraDeleteAll{namedWindow=true}",
            "InfraDeleteAll{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-eb28fd3f17513a399ac9",
            "java-eb28fd3f17513a399ac9",
            "java-132a1f9e7f2c434a19e9",
            "java-132a1f9e7f2c434a19e9",
            "java-e3aeca986a0cfee60dc7",
            "java-e3aeca986a0cfee60dc7"
    };
    private static final String[] CASES = {
            "cond-nw", "cond-table", "pattern-nw", "pattern-table", "deleteall-nw", "deleteall-table"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; where-clause deletes scoped to trigger id vs row a/b over the keepall named window",
            "iterator+count; same where-clause deletes over the primary-key table with no listener deliveries",
            "listener; pattern-triggered unconditional delete-all over the keepall named window",
            "listener; pattern-triggered unconditional delete-all over the primary-key table",
            "listener; event-triggered delete-all delivering deleted rows as new data over the named window",
            "listener; event-triggered delete-all delivering deleted rows as new data over the table"
    };

    // Verbatim transcriptions of InfraNWTableOnDelete lines 52-69, 150-161 and
    // 223-237, including the @Name/@name and "MyInfra (a"/"MyInfra(a" quirks.
    private static final String EPL_CREATE_NW =
            "@name('CreateInfra') @public create window MyInfra#keepall as "
                    + "select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_NW_NAME =
            "@Name('CreateInfra') @public create window MyInfra#keepall as "
                    + "select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_TABLE =
            "@name('CreateInfra') @public create table MyInfra (a string primary key, b int)";
    private static final String EPL_CREATE_TABLE_TIGHT =
            "@name('CreateInfra') @public create table MyInfra(a string primary key, b int)";
    private static final String EPL_CREATE_TABLE_NAME =
            "@Name('CreateInfra') @public create table MyInfra (a string primary key, b int)";
    private static final String EPL_DELETE_COND_A =
            "on SupportBean_A delete from MyInfra where 'X' || a || 'X' = id";
    private static final String EPL_DELETE_COND_B =
            "on SupportBean_B delete from MyInfra where b < 5";
    private static final String EPL_DELETE_PATTERN =
            "@name('OnDelete') on pattern [every ea=SupportBean_A or every eb=SupportBean_B] "
                    + "delete from MyInfra";
    private static final String EPL_DELETE_ALL =
            "@Name('OnDelete') on SupportBean_A delete from MyInfra";
    private static final String EPL_INSERT =
            "insert into MyInfra select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_INSERT_NAME =
            "@Name('Insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_SELECT =
            "@Name('Select') select irstream MyInfra.a as a, b from MyInfra as s1";
    private static final String EPL_COUNT = "select count(*) as c0 from MyInfra";

    private static final String[] CASE_EPLS = {
            EPL_CREATE_NW, EPL_CREATE_TABLE, EPL_CREATE_NW,
            EPL_CREATE_TABLE_TIGHT, EPL_CREATE_NW_NAME, EPL_CREATE_TABLE_NAME
    };
    private static final String[] INFRA_FIELDS = {"a", "b"};
    private static final String[] COUNT_FIELDS = {"c0"};

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("CreateInfra", "OnDelete", "Select"));
    private static final int EXPECTED_STEPS = 144;
    private static final int EXPECTED_RECORDS = 106;

    private InfraNWTableOnDeleteScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnDeleteScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportBean_B.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASES) {
                runCase(caseName, configuration, runtime, allSteps, records);
            }
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

    /**
     * Replays one case's steps on the shared runtime; sequences restart per
     * case.  Deploys register their statement under the step label so deployed
     * markers and snapshots can resolve it; listeners attach to the named
     * statements the Java execution listens on.
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
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
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != 1) {
                        throw new IllegalStateException("deployment of " + label + " has "
                                + deployed.length + " statements, want 1");
                    }
                    EPStatement statement = deployed[0];
                    if (LISTENED_STATEMENTS.contains(statement.getName())) {
                        statement.addListener(listener(caseName, sequences, records, runtime));
                    }
                    statements.put(label, statement);
                    break;
                }
                case "deployed": {
                    String label = string(step, "statement");
                    if (!statements.containsKey(label)) {
                        throw new IllegalStateException(
                                "deployed marker for unknown statement " + label);
                    }
                    int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "deployed");
                    record.add("statement", label);
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    records.add(record);
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    String label = string(step, "statement");
                    EPStatement statement = statements.get(label);
                    if (statement == null) {
                        throw new IllegalStateException("snapshot statement " + label
                                + " was not deployed in case " + caseName);
                    }
                    String[] fields = stringArray(step.get("fields"), "fields");
                    JsonArray rows = new JsonArray();
                    for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
                        rows.add(projectedRow(iterator.next(), fields));
                    }
                    if ("any".equals(string(step, "mode"))) {
                        sortRowsCanonical(rows);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statement.getName());
                    record.add("sequence", 0);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "faf": {
                    // Fire-and-forget count against the infra, mirroring
                    // RegressionEnvironmentBase.compileExecuteFAF: compiled
                    // with the runtime path and executed on demand.
                    CompilerArguments fafArgs = new CompilerArguments(configuration);
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled query = EPCompilerProvider.getCompiler()
                            .compileQuery(string(step, "epl"), fafArgs);
                    EventBean[] result = runtime.getFireAndForgetService()
                            .executeQuery(query).getArray();
                    String[] fields = stringArray(step.get("fields"), "fields");
                    JsonArray rows = new JsonArray();
                    for (EventBean row : result) {
                        rows.add(projectedRow(row, fields));
                    }
                    int sequence = sequences.merge("faf", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "faf");
                    record.add("statement", "count");
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        runtime.getDeploymentService().undeployAll();
        statements.clear();
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty.  This mirrors
     * assertPropsNew/assertPropsOld/assertListener: the on-delete statement
     * receives deleted rows as new data, and the named window's own statement
     * plus the irstream consumer receive them as old data (named window only).
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

    /** Row projected to exactly the fields the step's assertions read. */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /**
     * Canonical row ordering for "any"-mode snapshots, mirroring the Go
     * runner's sortRowsCanonical freeze of Java's assertEqualsAnyOrder: rows
     * sort by their fields JSON (keys already sorted alphabetically).
     */
    private static void sortRowsCanonical(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue row : rows) {
            items.add(row);
        }
        items.sort(Comparator.comparing(row -> row.asObject().get("fields").toString()));
        while (rows.size() > 0) {
            rows.remove(0);
        }
        for (JsonValue item : items) {
            rows.add(item);
        }
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
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(string(payload, "id")), type);
                break;
            }
            case "SupportBean_B": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(string(payload, "id")), type);
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
        offset = validateCondCase(steps, offset, "cond-nw", EPL_CREATE_NW);
        offset = validateCondCase(steps, offset, "cond-table", EPL_CREATE_TABLE);
        offset = validatePatternCase(steps, offset, "pattern-nw", EPL_CREATE_NW, true);
        offset = validatePatternCase(steps, offset, "pattern-table", EPL_CREATE_TABLE_TIGHT, false);
        offset = validateDeleteAllCase(steps, offset, "deleteall-nw", EPL_CREATE_NW_NAME, true);
        offset = validateDeleteAllCase(steps, offset, "deleteall-table", EPL_CREATE_TABLE_NAME, false);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of InfraDeleteCondition (lines 219-279): both
     * conditional deletes deploy before the insert-into, the count read
     * precedes the iterator snapshot after E3, and the snapshot precedes the
     * count read after XE2X, E7, and B1.
     */
    private static int validateCondCase(JsonArray steps, int offset, String caseName,
                                        String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName, null);
        offset = validateDeployPair(steps, offset, caseName, "CreateInfra", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "DeleteCondA", EPL_DELETE_COND_A);
        offset = validateDeployPair(steps, offset, caseName, "DeleteCondB", EPL_DELETE_COND_B);
        offset = validateDeployPair(steps, offset, caseName, "Insert", EPL_INSERT);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        validateFaf(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "any");
        validateABSend(steps.get(offset++), caseName, "SupportBean_A", "XE2X");
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "any");
        validateFaf(steps.get(offset++), caseName);
        validateBeanSend(steps.get(offset++), caseName, "E7", 7);
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "any");
        validateFaf(steps.get(offset++), caseName);
        validateABSend(steps.get(offset++), caseName, "SupportBean_B", "B1");
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "any");
        validateFaf(steps.get(offset++), caseName);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraDeletePattern (lines 148-202): the named
     * window case additionally snapshots the always-empty OnDelete iterator
     * after E1 and after A1, mirroring the nw-only iterator assertions.
     */
    private static int validatePatternCase(JsonArray steps, int offset, String caseName,
                                           String createEpl, boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName, null);
        offset = validateDeployPair(steps, offset, caseName, "CreateInfra", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "OnDelete", EPL_DELETE_PATTERN);
        offset = validateDeployPair(steps, offset, caseName, "Insert", EPL_INSERT);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        if (namedWindow) {
            validateSnapshot(steps.get(offset++), caseName, "OnDelete", "ordered");
        }
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateABSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        if (namedWindow) {
            validateSnapshot(steps.get(offset++), caseName, "OnDelete", "ordered");
        }
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateABSend(steps.get(offset++), caseName, "SupportBean_B", "B1");
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraDeleteAll (lines 50-131): the OnDelete
     * iterator snapshot after E1 is unconditional (both nw and table), the
     * ones after each A2 are nw-only, and the count read follows each group.
     */
    private static int validateDeleteAllCase(JsonArray steps, int offset, String caseName,
                                             String createEpl, boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName, null);
        offset = validateDeployPair(steps, offset, caseName, "CreateInfra", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "OnDelete", EPL_DELETE_ALL);
        offset = validateDeployPair(steps, offset, caseName, "Insert", EPL_INSERT_NAME);
        offset = validateDeployPair(steps, offset, caseName, "Select", EPL_SELECT);
        validateABSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateFaf(steps.get(offset++), caseName);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateSnapshot(steps.get(offset++), caseName, "OnDelete", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateABSend(steps.get(offset++), caseName, "SupportBean_A", "A2");
        if (namedWindow) {
            validateSnapshot(steps.get(offset++), caseName, "OnDelete", "ordered");
        }
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateABSend(steps.get(offset++), caseName, "SupportBean_A", "A2");
        if (namedWindow) {
            validateSnapshot(steps.get(offset++), caseName, "OnDelete", "ordered");
        }
        validateSnapshot(steps.get(offset++), caseName, "CreateInfra", "ordered");
        validateFaf(steps.get(offset++), caseName);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateDeployPair(JsonArray steps, int offset, String caseName,
                                          String statement, String epl) {
        validateDeploy(steps.get(offset++), caseName, statement, epl);
        validateDeployed(steps.get(offset++), caseName, statement);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase, String mode) {
        JsonObject marker = object(value, "case marker");
        if (mode == null) {
            requireFields(marker, "op", "case");
        } else {
            requireFields(marker, "op", "case", "mode");
            if (!mode.equals(string(marker, "mode"))) {
                throw new IllegalArgumentException("case marker mode is not pinned for "
                        + expectedCase);
            }
        }
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

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), INFRA_FIELDS,
                "snapshot fields for " + caseName);
    }

    private static void validateFaf(JsonValue value, String caseName) {
        JsonObject step = object(value, "faf step");
        requireFields(step, "op", "case", "statement", "epl", "fields");
        if (!"faf".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"count".equals(string(step, "statement"))
                || !EPL_COUNT.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("faf step is not pinned for " + caseName);
        }
        validateStringArray(step.get("fields"), COUNT_FIELDS, "faf fields for " + caseName);
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

    private static void validateABSend(JsonValue value, String caseName, String eventType,
                                       String expectedId) {
        JsonObject step = object(value, eventType + " step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(eventType + " step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), eventType + " payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException(eventType + " payload is not pinned for " + caseName);
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

    private static String[] stringArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        String[] names = new String[items.size()];
        for (int index = 0; index < items.size(); index++) {
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException(label + " must be a string array");
            }
            names[index] = item.asString();
        }
        return names;
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

    /**
     * Local mirror of the regression SupportBean_B (regression-lib is not on
     * the oracle classpath); a single string id property like the suite bean.
     */
    public static class SupportBean_B {
        private final String id;

        public SupportBean_B(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }
}
