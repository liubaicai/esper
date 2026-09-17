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
 * Java oracle for InfraNWTableOnUpdate: on-trigger update semantics over a
 * keepall named window and over a primary-key table.  Covers unqualified
 * set/where names resolving infra-row versus trigger-event properties with
 * insert-remove update delivery, a self-correlated subquery assignment
 * reading the pre-update row of the same infra (ESPER-507), and a
 * group-by-int-array aggregated subquery assignment whose multi-row result
 * becomes null.
 *
 * Replays the six executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean, SupportBean_S0 and
 * SupportBean_A from esper-common plus a local mirror of the regression
 * SupportEventWithIntArray (regression-lib is not on the oracle classpath),
 * internal timer disabled, and the rethrowing exception handler so statement
 * failures surface to the sender thread.  Listeners attach to 'create' and
 * 'update' for the SceneOne cases and to 'create' for the MultikeyWArray
 * cases, matching the Java executions; SubquerySelf attaches none.  The
 * multikey "FafInsert" deploy step is a fire-and-forget insert executed via
 * compileExecuteFAFNoResult semantics (compileQuery + executeQuery), not a
 * deployment, so it has no deployed marker.  Deployed markers and iterator
 * snapshots are emitted at their step positions; milestone calls are HA
 * checkpoint no-ops and emit nothing.
 */
public final class InfraNWTableOnUpdateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-update";
    private static final String DESCRIPTION =
            "InfraNWTableOnUpdate on-trigger update semantics over a keepall named window and over a "
                    + "primary-key table: unqualified set/where names resolving infra-row versus "
                    + "trigger-event properties with insert-remove update delivery, a self-correlated "
                    + "subquery assignment reading the pre-update row (ESPER-507), and a "
                    + "group-by-int-array aggregated subquery assignment whose multi-row result becomes "
                    + "null (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableOnUpdate.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnUpdate.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-f8090e148364d7b15116",
            "java-runtime-922adbe6628b17d5ec61",
            "java-runtime-046926326cad5f0172cd",
            "java-runtime-702d919aaadcbb188467",
            "java-runtime-5cc56f78a52d6a452948",
            "java-runtime-b23b2fbc3cc233237ac9"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableOnUpdateSceneOne{namedWindow=true}",
            "InfraNWTableOnUpdateSceneOne{namedWindow=false}",
            "InfraSubquerySelf{namedWindow=true}",
            "InfraSubquerySelf{namedWindow=false}",
            "InfraSubqueryMultikeyWArray{namedWindow=true}",
            "InfraSubqueryMultikeyWArray{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-bc54a78188b2249a0a9f",
            "java-bc54a78188b2249a0a9f",
            "java-c67318a7541b42eb7940",
            "java-c67318a7541b42eb7940",
            "java-9ab08590ab6533e21139",
            "java-9ab08590ab6533e21139"
    };
    private static final String[] CASES = {
            "sceneone-nw", "sceneone-table", "subqself-nw", "subqself-table",
            "multikey-nw", "multikey-table"
    };
    private static final int[] ORDINALS = {0, 1, 4, 5, 6, 7};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+iterator; on-update IR pairs over the keepall named window with the updated row "
                    + "reinserted at the tail of iteration order",
            "listener+iterator; same on-update over the primary-key table with no create-statement "
                    + "deliveries",
            "iterator; self-correlated subquery assignment reading the pre-update row of the same "
                    + "named window (ESPER-507)",
            "iterator; self-correlated subquery assignment reading the pre-update row of the same "
                    + "primary-key table (ESPER-507)",
            "listener+iterator; group-by-int-array aggregated subquery assignment over the named "
                    + "window, multi-row subquery result becoming null",
            "iterator; same group-by-int-array subquery assignment over the table with no listener "
                    + "deliveries"
    };

    // Verbatim transcriptions of InfraNWTableOnUpdate lines 58-65, 108-114,
    // 124 and 219-231, including the @Name("Self Update") double-quotes and
    // embedded newlines.
    private static final String EPL_CREATE_SCENEONE_NW =
            "@name('create') @public create window MyInfra.win:keepall() as SupportBean";
    private static final String EPL_CREATE_SCENEONE_TABLE =
            "@name('create') @public create table MyInfra(theString string, intPrimitive int primary key)";
    private static final String EPL_INSERT_SCENEONE =
            "@name('insert') insert into MyInfra select theString, intPrimitive from SupportBean";
    private static final String EPL_UPDATE_SCENEONE =
            "@name('update') on SupportBean_S0 update MyInfra set theString = p00 "
                    + "where intPrimitive = id";
    private static final String EPL_CREATE_SUBQSELF_NW =
            "@name('create') @public create window MyInfraSS#keepall as SupportBean";
    private static final String EPL_CREATE_SUBQSELF_TABLE =
            "@name('create') @public create table MyInfraSS(theString string primary key, "
                    + "intPrimitive int)";
    private static final String EPL_INSERT_SUBQSELF =
            "insert into MyInfraSS select theString, intPrimitive from SupportBean";
    private static final String EPL_UPDATE_SUBQSELF =
            "@Name(\"Self Update\")\n"
                    + "on SupportBean_A c\n"
                    + "update MyInfraSS s\n"
                    + "set intPrimitive = (select intPrimitive from MyInfraSS t "
                    + "where t.theString = c.id) + 1\n"
                    + "where s.theString = c.id";
    private static final String EPL_CREATE_MULTIKEY_NW =
            "@name('create') @public create window MyInfra#keepall() as (value int)";
    private static final String EPL_CREATE_MULTIKEY_TABLE =
            "@name('create') @public create table MyInfra(value int)";
    private static final String EPL_FAF_INSERT_MULTIKEY =
            "insert into MyInfra select 0 as value";
    private static final String EPL_UPDATE_MULTIKEY =
            "on SupportBean update MyInfra set value = (select sum(value) as c0 from "
                    + "SupportEventWithIntArray#keepall group by array)";

    private static final String[] CASE_EPLS = {
            EPL_CREATE_SCENEONE_NW, EPL_CREATE_SCENEONE_TABLE,
            EPL_CREATE_SUBQSELF_NW, EPL_CREATE_SUBQSELF_TABLE,
            EPL_CREATE_MULTIKEY_NW, EPL_CREATE_MULTIKEY_TABLE
    };
    private static final String[] INFRA_FIELDS = {"theString", "intPrimitive"};
    private static final String[] VALUE_FIELDS = {"value"};

    // Statement names each case attaches a listener to, matching the Java
    // executions: SceneOne listens on 'create' and 'update', SubquerySelf on
    // none, MultikeyWArray on 'create'.
    private static final Map<String, Set<String>> LISTENED_STATEMENTS = new HashMap<>();
    static {
        LISTENED_STATEMENTS.put("sceneone-nw", new HashSet<>(Arrays.asList("create", "update")));
        LISTENED_STATEMENTS.put("sceneone-table", new HashSet<>(Arrays.asList("create", "update")));
        LISTENED_STATEMENTS.put("subqself-nw", new HashSet<>());
        LISTENED_STATEMENTS.put("subqself-table", new HashSet<>());
        LISTENED_STATEMENTS.put("multikey-nw", new HashSet<>(Arrays.asList("create")));
        LISTENED_STATEMENTS.put("multikey-table", new HashSet<>(Arrays.asList("create")));
    }
    private static final String FAF_INSERT_LABEL = "FafInsert";
    private static final int EXPECTED_STEPS = 92;
    private static final int EXPECTED_RECORDS = 42;

    private InfraNWTableOnUpdateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnUpdateScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportEventWithIntArray.class);
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
     * markers and snapshots can resolve it; the FafInsert label is a
     * fire-and-forget insert (compileExecuteFAFNoResult semantics) and
     * registers nothing.  Listeners attach to the named statements the Java
     * execution listens on for that case.
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Set<String> listened = LISTENED_STATEMENTS.get(caseName);
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
                    if (FAF_INSERT_LABEL.equals(label)) {
                        // Fire-and-forget insert, mirroring
                        // RegressionEnvironmentBase.compileExecuteFAFNoResult:
                        // compiled as a query with the runtime path and
                        // executed on demand; no deployment, no marker.
                        CompilerArguments fafArgs = new CompilerArguments(configuration);
                        fafArgs.getPath().add(runtime.getRuntimePath());
                        EPCompiled query = EPCompilerProvider.getCompiler()
                                .compileQuery(epl, fafArgs);
                        runtime.getFireAndForgetService().executeQuery(query);
                        break;
                    }
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(epl, compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != 1) {
                        throw new IllegalStateException("deployment of " + label + " has "
                                + deployed.length + " statements, want 1");
                    }
                    EPStatement statement = deployed[0];
                    if (listened.contains(statement.getName())) {
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
     * assertPropsIRPair/assertListener: the on-update statement receives the
     * post-update row as new data and the pre-update row as old data, and the
     * named window's own 'create' statement receives the same IR pair (named
     * window only) plus new-rows on inserts.
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

    /** Row projected to exactly the fields the step's assertions read,
     * with property names sorted alphabetically so the canonical "any"-mode
     * ordering matches the Go runner's fields-JSON sort. */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = fields.clone();
        Arrays.sort(names);
        JsonObject values = new JsonObject();
        for (String field : names) {
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
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull()
                        ? null : theString.asString());
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                runtime.getEventService().sendEventBean(new SupportBean_S0(
                        (int) longInteger(payload.get("id"), "id"),
                        string(payload, "p00")), type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(string(payload, "id")), type);
                break;
            }
            case "SupportEventWithIntArray": {
                JsonArray items = array(payload.get("array"), "array");
                int[] values = new int[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    values[index] = (int) longInteger(items.get(index), "array element");
                }
                runtime.getEventService().sendEventBean(new SupportEventWithIntArray(
                        string(payload, "id"), values,
                        (int) longInteger(payload.get("value"), "value")), type);
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
        offset = validateSceneOneCase(steps, offset, "sceneone-nw", EPL_CREATE_SCENEONE_NW, true);
        offset = validateSceneOneCase(steps, offset, "sceneone-table", EPL_CREATE_SCENEONE_TABLE,
                false);
        offset = validateSubqSelfCase(steps, offset, "subqself-nw", EPL_CREATE_SUBQSELF_NW);
        offset = validateSubqSelfCase(steps, offset, "subqself-table", EPL_CREATE_SUBQSELF_TABLE);
        offset = validateMultikeyCase(steps, offset, "multikey-nw", EPL_CREATE_MULTIKEY_NW);
        offset = validateMultikeyCase(steps, offset, "multikey-table", EPL_CREATE_MULTIKEY_TABLE);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of InfraNWTableOnUpdateSceneOne (lines 107-154):
     * create and insert deploy before the two SupportBean sends, the on-update
     * deploys after them, and the named-window case reads the iterator in
     * order after the first trigger (updated row reinserted at the tail) while
     * every later read is any-order, including the repeated read after
     * milestone(3).
     */
    private static int validateSceneOneCase(JsonArray steps, int offset, String caseName,
                                            String createEpl, boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "insert", EPL_INSERT_SCENEONE);
        validateBeanSend(steps.get(offset++), caseName, "A1", 1);
        validateBeanSend(steps.get(offset++), caseName, "B2", 2);
        offset = validateDeployPair(steps, offset, caseName, "update", EPL_UPDATE_SCENEONE);
        validateS0Send(steps.get(offset++), caseName, 1, "X1");
        validateSnapshot(steps.get(offset++), caseName, "create",
                namedWindow ? "ordered" : "any", INFRA_FIELDS);
        validateS0Send(steps.get(offset++), caseName, 2, "X2");
        validateSnapshot(steps.get(offset++), caseName, "create", "any", INFRA_FIELDS);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", INFRA_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraSubquerySelf (lines 218-243): anonymous
     * insert and @Name("Self Update") on-update deploy after 'create', two
     * SupportBean sends then three SupportBean_A triggers, and a single
     * any-order iterator read of 'create' before undeployAll.
     */
    private static int validateSubqSelfCase(JsonArray steps, int offset, String caseName,
                                            String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "Insert", EPL_INSERT_SUBQSELF);
        offset = validateDeployPair(steps, offset, caseName, "Self Update", EPL_UPDATE_SUBQSELF);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 6);
        validateASend(steps.get(offset++), caseName, "E1");
        validateASend(steps.get(offset++), caseName, "E1");
        validateASend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create", "any", INFRA_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraSubqueryMultikeyWArray (lines 57-81): the
     * FafInsert deploy step is a fire-and-forget insert with no deployed
     * marker, the anonymous on-update deploys before the
     * SupportEventWithIntArray sends, and each default SupportBean trigger is
     * followed by an any-order iterator read of 'create'.
     */
    private static int validateMultikeyCase(JsonArray steps, int offset, String caseName,
                                            String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, FAF_INSERT_LABEL, EPL_FAF_INSERT_MULTIKEY);
        offset = validateDeployPair(steps, offset, caseName, "Update", EPL_UPDATE_MULTIKEY);
        validateIntArraySend(steps.get(offset++), caseName, "E1", new int[]{1, 2}, 10);
        validateIntArraySend(steps.get(offset++), caseName, "E2", new int[]{1, 2}, 11);
        validateDefaultBeanSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", VALUE_FIELDS);
        validateIntArraySend(steps.get(offset++), caseName, "E3", new int[]{1, 2}, 12);
        validateDefaultBeanSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", VALUE_FIELDS);
        validateIntArraySend(steps.get(offset++), caseName, "E4", new int[]{1}, 13);
        validateDefaultBeanSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", VALUE_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateDeployPair(JsonArray steps, int offset, String caseName,
                                          String statement, String epl) {
        validateDeploy(steps.get(offset++), caseName, statement, epl);
        validateDeployed(steps.get(offset++), caseName, statement);
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
                                         String expectedMode, String[] expectedFields) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), expectedFields,
                "snapshot fields for " + caseName);
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

    private static void validateDefaultBeanSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        JsonValue theString = payload.get("theString");
        if (theString == null || !theString.isNull()
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != 0) {
            throw new IllegalArgumentException(
                    "default SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, long expectedId,
                                       String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (longInteger(payload.get("id"), "id") != expectedId
                || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    private static void validateASend(JsonValue value, String caseName, String expectedId) {
        JsonObject step = object(value, "SupportBean_A step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_A".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_A step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_A payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException("SupportBean_A payload is not pinned for " + caseName);
        }
    }

    private static void validateIntArraySend(JsonValue value, String caseName, String expectedId,
                                             int[] expectedArray, long expectedValue) {
        JsonObject step = object(value, "SupportEventWithIntArray step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportEventWithIntArray".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportEventWithIntArray payload");
        requireFields(payload, "id", "array", "value");
        if (!expectedId.equals(string(payload, "id"))
                || longInteger(payload.get("value"), "value") != expectedValue) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray payload is not pinned for " + caseName);
        }
        JsonArray items = array(payload.get("array"), "array");
        if (items.size() != expectedArray.length) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray array is not pinned for " + caseName);
        }
        for (int index = 0; index < expectedArray.length; index++) {
            if (longInteger(items.get(index), "array element") != expectedArray[index]) {
                throw new IllegalArgumentException(
                        "SupportEventWithIntArray array is not pinned for " + caseName);
            }
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
     * Local mirror of the regression SupportEventWithIntArray (regression-lib
     * is not on the oracle classpath); id, int[] array and value properties
     * like the suite bean.
     */
    public static class SupportEventWithIntArray {
        private final String id;
        private final int[] array;
        private final int value;

        public SupportEventWithIntArray(String id, int[] array, int value) {
            this.id = id;
            this.array = array;
            this.value = value;
        }

        public String getId() {
            return id;
        }

        public int[] getArray() {
            return array;
        }

        public int getValue() {
            return value;
        }
    }
}
