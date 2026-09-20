import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

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
 * Java oracle for InfraTableFAFExecuteQuery ordinals 0-3: the fire-and-forget
 * insert/delete/update/select surface over tables. Four executions on four
 * runtimes.
 *
 * InfraFAFInsert (ord 0) deploys the unkeyed MyTableINS(p0 string, p1 int)
 * under statement name 'create', runs the compileExecuteFAF
 * `insert into MyTableINS (p0, p1) select 'a', 1` (empty result array, event
 * type identical to the create statement's row type) and pins the single
 * {a,1} row through an ordered iterator assert.
 *
 * InfraFAFDelete (ord 1) deploys keyed MyTableDEL(p0 string primary key,
 * thesum sum(int)) plus a grouped into-table sum feed, sends ten
 * SupportBean("G0",0)..("G9",9) events, pins iterator count 10, runs the
 * compileExecuteFAF `delete from MyTableDEL` delete-all and pins iterator
 * count 0.
 *
 * InfraFAFUpdate (ord 2) deploys MyTableUPD(p0 string primary key, p1 string,
 * thesum sum(int)) under statement name 'TheTable' plus the same grouped
 * feed, sends SupportBean("E1",1) and ("E2",2), runs the compileExecuteFAF
 * `update MyTableUPD set p1 = 'ABC'` update-all and pins {E1,ABC},{E2,ABC}
 * through an any-order iterator assert.
 *
 * InfraFAFSelect (ord 3) deploys MyTableSEL(p0 string primary key, thesum
 * sum(int)) under statement name 'TheTable' plus the same grouped feed,
 * sends the same two events, runs the compileExecuteFAF
 * `select * from MyTableSEL` and pins the result array projected to p0 as
 * {E1},{E2} in any order.
 */
public final class InfraTableFAFExecuteQueryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-faf-execute-query";
    private static final String DESCRIPTION =
            "InfraTableFAFExecuteQuery ordinals 0-3: InfraFAFInsert deploys the "
                    + "unkeyed MyTableINS and fire-and-forget inserts one "
                    + "('a',1) row pinned by an ordered iterator assert; "
                    + "InfraFAFDelete feeds keyed MyTableDEL through a grouped "
                    + "into-table sum, pins the ten-row count, fire-and-forget "
                    + "deletes every row and pins the empty count; "
                    + "InfraFAFUpdate feeds MyTableUPD (statement name "
                    + "TheTable) two rows then fire-and-forget updates p1 to "
                    + "'ABC' pinned by an any-order iterator assert; "
                    + "InfraFAFSelect feeds MyTableSEL two rows then "
                    + "fire-and-forget selects all rows projected to p0 (Java "
                    + "source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/tbl/"
                    + "InfraTableFAFExecuteQuery.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableFAFExecuteQuery.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-a79e19dc5f135bb8e628",
            "java-runtime-a68109b2bb91de4ce1cd",
            "java-runtime-196ff792f0f739c8d97c",
            "java-runtime-b995c40f3c052bcc277e"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraFAFInsert",
            "InfraFAFDelete",
            "InfraFAFUpdate",
            "InfraFAFSelect"
    };
    private static final String[] STATIC_IDS = {
            "java-1899404b366f6f3c1a31",
            "java-11e9c7fcb6e1617929a9",
            "java-db32f6072d9b36a84471",
            "java-c87f698c077e7dd7ada7"
    };
    private static final String[] CASES = {
            "faf-insert",
            "faf-delete",
            "faf-update",
            "faf-select"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+snapshot; unkeyed MyTableINS: FAF insert into (p0,p1) "
                    + "select 'a',1 returns an empty array with the create "
                    + "statement's row type and the ordered iterator assert "
                    + "pins the single {a,1} row",
            "deploy+snapshot; keyed MyTableDEL fed by a grouped into-table "
                    + "sum: ten SupportBean sends pin iterator count 10, the "
                    + "FAF delete-all empties the table and the count-0 "
                    + "snapshot pins the empty iterator",
            "deploy+snapshot; MyTableUPD under statement name TheTable fed by "
                    + "the same grouped sum: the FAF update-all sets p1='ABC' "
                    + "and the any-order iterator assert pins "
                    + "{E1,ABC},{E2,ABC}",
            "deploy+snapshot; MyTableSEL fed by the same grouped sum: the FAF "
                    + "select * returns both rows and the any-order assert "
                    + "pins p0 {E1},{E2}"
    };

    // Verbatim transcriptions of InfraTableFAFExecuteQuery lines 48, 50 (ord
    // 0); 66-67, 72 (ord 1); 87-88, 91 (ord 2); and 105-106, 109 (ord 3).
    // Each non-Faf deploy step compiles its EPL as one module; Faf* deploy
    // labels are fire-and-forget queries (compileExecuteFAF semantics) and
    // register nothing.
    private static final String EPL_INS_CREATE =
            "@name('create') @public create table MyTableINS as (p0 string, p1 int)";
    private static final String EPL_INS_FAF =
            "insert into MyTableINS (p0, p1) select 'a', 1";

    private static final String EPL_DEL_CREATE =
            "@name('create') @public create table MyTableDEL as (p0 string primary key, thesum sum(int))";
    private static final String EPL_DEL_INTO =
            "into table MyTableDEL select theString, sum(intPrimitive) as thesum from SupportBean group by theString";
    private static final String EPL_DEL_FAF =
            "delete from MyTableDEL";

    private static final String EPL_UPD_CREATE =
            "@Name('TheTable') @public create table MyTableUPD as (p0 string primary key, p1 string, thesum sum(int))";
    private static final String EPL_UPD_INTO =
            "into table MyTableUPD select theString, sum(intPrimitive) as thesum from SupportBean group by theString";
    private static final String EPL_UPD_FAF =
            "update MyTableUPD set p1 = 'ABC'";

    private static final String EPL_SEL_CREATE =
            "@Name('TheTable') @public create table MyTableSEL as (p0 string primary key, thesum sum(int))";
    private static final String EPL_SEL_INTO =
            "into table MyTableSEL select theString, sum(intPrimitive) as thesum from SupportBean group by theString";
    private static final String EPL_SEL_FAF =
            "select * from MyTableSEL";

    private static final String[] SNAPSHOT_INS_FIELDS = {"p0", "p1"};
    private static final String[] SNAPSHOT_DEL_FIELDS = {"p0", "thesum"};
    private static final String[] SNAPSHOT_UPD_FIELDS = {"p0", "p1"};
    private static final String[] SNAPSHOT_SEL_FIELDS = {"p0"};

    private static final int EXPECTED_STEPS = 44;
    private static final int EXPECTED_RECORDS = 12;

    private InfraTableFAFExecuteQueryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableFAFExecuteQueryScenarioOracle <scenario.json>");
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
        for (String caseName : CASES) {
            runCase(caseName, allSteps, records);
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
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). SupportBean is preconfigured and the internal timer
     * is disabled. Every non-FAF deploy registers its deployment by step
     * label and its statements by name so deployed markers and snapshots
     * resolve both; Faf* deploy labels are compileExecuteFAF queries and
     * register nothing. The FafInsert deploy additionally asserts the empty
     * result array and the create-statement event-type identity that
     * assertFAFInsertResult pins.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
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
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        if (label.startsWith("Faf")) {
                            // Fire-and-forget query, mirroring
                            // RegressionEnvironmentBase.compileExecuteFAF:
                            // compiled as a query with the runtime path and
                            // executed on demand; no deployment, no marker.
                            EPFireAndForgetQueryResult result =
                                    executeFaf(runtime, configuration, epl);
                            if ("FafInsert".equals(label)) {
                                // assertFAFInsertResult: the insert's result
                                // array is empty and its event type is the
                                // create statement's row type.
                                if (result.getArray().length != 0) {
                                    throw new IllegalStateException(
                                            "FafInsert returned " + result.getArray().length
                                                    + " rows, expected 0");
                                }
                                EPStatement create = statements.get("create");
                                if (create == null
                                        || result.getEventType() != create.getEventType()) {
                                    throw new IllegalStateException(
                                            "FafInsert event type is not the create statement row type");
                                }
                            }
                            break;
                        }
                        EPDeployment deployment = compileDeploy(runtime, configuration, epl);
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.containsKey(label) && !statements.containsKey(label)) {
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
                        String[] fields = stringArray(step.get("fields"), "fields");
                        JsonArray rows = new JsonArray();
                        JsonValue eplValue = step.get("epl");
                        if (eplValue instanceof JsonString) {
                            // FAF select (compileExecuteFAF): the result array
                            // rows are projected to the pinned fields.
                            EPFireAndForgetQueryResult result =
                                    executeFaf(runtime, configuration, eplValue.asString());
                            for (EventBean event : result.getArray()) {
                                rows.add(projectedRow(event, fields));
                            }
                        } else {
                            EPStatement statement = statements.get(label);
                            if (statement == null) {
                                throw new IllegalStateException("snapshot statement " + label
                                        + " was not deployed in case " + caseName);
                            }
                            for (Iterator<EventBean> iterator = statement.iterator();
                                 iterator.hasNext(); ) {
                                rows.add(projectedRow(iterator.next(), fields));
                            }
                        }
                        if ("any".equals(string(step, "mode"))) {
                            sortRowsCanonical(rows);
                        }
                        // The optional count pin mirrors the iteratorCount
                        // asserts (assertEquals(10L/0L, getTableCount(...))).
                        JsonValue countValue = step.get("count");
                        Long expectedCount = null;
                        if (countValue instanceof JsonNumber) {
                            expectedCount = longInteger(countValue, "count");
                            if (rows.size() != expectedCount) {
                                throw new IllegalStateException("snapshot count drift for "
                                        + caseName + "/" + label + ": expected " + expectedCount
                                        + " got " + rows.size());
                            }
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "snapshot");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        if (rows.size() > 0) {
                            record.add("new", rows);
                        }
                        if (expectedCount != null) {
                            record.add("count", expectedCount);
                        }
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deployments.clear();
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

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * against the runtime path followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration,
                                              String epl) throws EPCompileException,
            EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    /** executeFaf mirrors env.compileExecuteFAF(epl, path): compileQuery with
     * the runtime path followed by executeQuery. */
    private static EPFireAndForgetQueryResult executeFaf(EPRuntime runtime,
                                                         Configuration configuration,
                                                         String epl) throws EPCompileException {
        CompilerArguments fafArgs = new CompilerArguments(configuration);
        fafArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled query = EPCompilerProvider.getCompiler().compileQuery(epl, fafArgs);
        return runtime.getFireAndForgetService().executeQuery(query);
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
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, and Object[]/int[] row underlyings as JSON
     * arrays.
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
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /** Sends one pinned SupportBean event carrying theString/intPrimitive. */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if (!"SupportBean".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        SupportBean event = new SupportBean(
                string(payload, "theString"),
                intField(payload.get("intPrimitive"), "intPrimitive"));
        runtime.getEventService().sendEventBean(event, type);
    }

    private static int intField(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        return (int) longInteger(value, label);
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
        validateStringArray(scenario.get("javaFlags"), new String[]{}, "javaFlags");

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
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
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
            switch (caseName) {
                case "faf-insert":
                    offset = validateInsertCase(steps, offset, caseName);
                    break;
                case "faf-delete":
                    offset = validateDeleteCase(steps, offset, caseName);
                    break;
                case "faf-update":
                    offset = validateUpdateCase(steps, offset, caseName);
                    break;
                default:
                    offset = validateSelectCase(steps, offset, caseName);
                    break;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "faf-insert":
                return String.join("\n", EPL_INS_CREATE, EPL_INS_FAF);
            case "faf-delete":
                return String.join("\n", EPL_DEL_CREATE, EPL_DEL_INTO, EPL_DEL_FAF);
            case "faf-update":
                return String.join("\n", EPL_UPD_CREATE, EPL_UPD_INTO, EPL_UPD_FAF);
            default:
                return String.join("\n", EPL_SEL_CREATE, EPL_SEL_INTO, EPL_SEL_FAF);
        }
    }

    /**
     * Exact step sequence of InfraFAFInsert.run (lines 45-56): the
     * create-table deploy, the compileExecuteFAF insert (a Faf* deploy step
     * with no marker), the ordered iterator assert and undeployAll.
     */
    private static int validateInsertCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_INS_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "FafInsert", EPL_INS_FAF);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered",
                SNAPSHOT_INS_FIELDS, null, null);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraFAFDelete.run (lines 65-75): the
     * create-table and into-table deploys, the ten SupportBean sends, the
     * count-10 iterator pin, the compileExecuteFAF delete-all, the count-0
     * iterator pin and undeployAll.
     */
    private static int validateDeleteCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_DEL_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_DEL_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        for (int index = 0; index < 10; index++) {
            validateBeanSend(steps.get(offset++), caseName, "G" + index, index);
        }
        validateSnapshot(steps.get(offset++), caseName, "create", "any",
                SNAPSHOT_DEL_FIELDS, null, 10L);
        validateDeploy(steps.get(offset++), caseName, "FafDelete", EPL_DEL_FAF);
        validateSnapshot(steps.get(offset++), caseName, "create", "any",
                SNAPSHOT_DEL_FIELDS, null, 0L);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraFAFUpdate.run (lines 85-93): the
     * create-table deploy under statement name TheTable, the into-table
     * deploy, the two SupportBean sends, the compileExecuteFAF update-all
     * and the any-order iterator assert on TheTable.
     */
    private static int validateUpdateCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_UPD_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_UPD_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateDeploy(steps.get(offset++), caseName, "FafUpdate", EPL_UPD_FAF);
        validateSnapshot(steps.get(offset++), caseName, "TheTable", "any",
                SNAPSHOT_UPD_FIELDS, null, null);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraFAFSelect.run (lines 103-111): the
     * create-table deploy under statement name TheTable, the into-table
     * deploy, the two SupportBean sends, and the compileExecuteFAF
     * select-all carried as a snapshot step with the pinned query EPL.
     */
    private static int validateSelectCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SEL_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_SEL_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "FafSelect", "any",
                SNAPSHOT_SEL_FIELDS, EPL_SEL_FAF, null);
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

    /** Pins one snapshot step: op, case, label, mode, the projection fields,
     * the optional FAF-select epl and the optional iterator-count pin. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedMode,
                                         String[] expectedFields, String expectedEpl,
                                         Long expectedCount) {
        JsonObject step = object(value, "snapshot step");
        List<String> names = new ArrayList<>(Arrays.asList(
                "op", "case", "statement", "mode", "fields"));
        if (expectedEpl != null) {
            names.add("epl");
        }
        if (expectedCount != null) {
            names.add("count");
        }
        requireFields(step, names.toArray(new String[0]));
        if (expectedEpl != null && !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("snapshot step EPL is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
        if (expectedCount != null
                && longInteger(step.get("count"), "count") != expectedCount) {
            throw new IllegalArgumentException("snapshot step count is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
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

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String expectedString, int expectedInt) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || intField(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException(
                    "SupportBean payload is not pinned for " + caseName);
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
}
