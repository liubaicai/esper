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
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.StatementProperty;
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
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableCreateIndex ordinals 2-5: the numeric-widening
 * hash-vs-btree fire-and-forget index surface. Four executions on four
 * runtimes.
 *
 * hashbtree-window/hashbtree-table (ords 2-3, InfraHashBTreeWidening
 * {namedWindow=true/false}): deploys the keepall window or (f1,f2)-keyed
 * MyInfraHBTW(f1 long) fed by `insert into MyInfraHBTW(f1, f2) select
 * longPrimitive, theString from SupportBean`, a late btree index
 * MyInfraHBTWIndex1 on (f1), sends one SupportBean carrying theString=E1 and
 * longPrimitive=10, then runs the compileExecuteFAF
 * `select * from MyInfraHBTW where f1>9` — the int literal 9 widens to the
 * long column and the result is {10,E1}. Two SODA
 * eplToModelCompileDeploy probes then deploy `create index IX1 on
 * MyInfraHBTW(f1, f2 btree)` and `create unique index IX2 on
 * MyInfraHBTW(f1)`; the second leg repeats the fixture on
 * MyInfraHBTWTwo(f1 short) fed by shortPrimitive=2 and read by `f1>=2`.
 *
 * widening-window/widening-table (ords 4-5, InfraWidening
 * {namedWindow=true/false}): the same two-leg fixture over MyInfraW /
 * MyInfraWTwo with plain hash indexes and the equality reads `f1=10` /
 * `f1=2`.
 *
 * The SODA probes ride `unrepresentable` steps: the oracle replays
 * eplToModelCompileDeploy (parse -> toEPL round-trip -> module compile ->
 * deploy) before emitting the pinned note; the Go side has no
 * statement-object boundary and records the plan-only note.
 */
public final class InfraNWTableWidening558ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-widening-558";
    private static final String DESCRIPTION =
            "InfraNWTableCreateIndex ordinals 2-5: InfraHashBTreeWidening (ords 2-3) "
                    + "deploys keepall window or (f1,f2)-keyed MyInfraHBTW(f1 long) fed "
                    + "by insert-into over SupportBean.longPrimitive, a late btree index "
                    + "MyInfraHBTWIndex1 on (f1), and the FAF range read `select * from "
                    + "MyInfraHBTW where f1>9` — the int literal 9 must widen to the "
                    + "long column — then deploys two SODA probes (`create index IX1 on "
                    + "MyInfraHBTW(f1, f2 btree)`, `create unique index IX2 on "
                    + "MyInfraHBTW(f1)`) and repeats the fixture on short-column "
                    + "MyInfraHBTWTwo with `f1>=2`; InfraWidening (ords 4-5) runs the "
                    + "same two-leg fixture over MyInfraW/MyInfraWTwo with plain hash "
                    + "indexes and the equality reads `f1=10`/`f1=2`. SODA probes are "
                    + "plan-only: pinned unrepresentable records. Expected rows {10,E1} "
                    + "and {2,E1} (Java source regression-lib/src/main/java/com/espertech/"
                    + "esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableCreateIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-ea8f73c9bd9fc31f1312",
            "java-runtime-c29b248899a0388e0768",
            "java-runtime-f4b0d37d40e38914df36",
            "java-runtime-2bffd7c0683d9195c556"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraHashBTreeWidening{namedWindow=true}",
            "InfraHashBTreeWidening{namedWindow=false}",
            "InfraWidening{namedWindow=true}",
            "InfraWidening{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b"
    };
    private static final String[] CASES = {
            "hashbtree-window",
            "hashbtree-table",
            "widening-window",
            "widening-table"
    };
    private static final int[] ORDINALS = {2, 3, 4, 5};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+snapshot; MyInfraHBTW#keepall window with late btree index on "
                    + "long f1: FAF `where f1>9` widens int 9 to long and returns "
                    + "{10,E1}; SODA IX1/IX2 probes are plan-only; MyInfraHBTWTwo short "
                    + "leg answers `f1>=2` with {2,E1}",
            "deploy+snapshot; (f1,f2)-keyed MyInfraHBTW table with late btree index on "
                    + "long f1: FAF `where f1>9` widens int 9 to long and returns "
                    + "{10,E1}; SODA IX1/IX2 probes are plan-only; MyInfraHBTWTwo short "
                    + "leg answers `f1>=2` with {2,E1}",
            "deploy+snapshot; MyInfraW#keepall window with late hash index on long f1: "
                    + "FAF `where f1=10` widens int 10 to long and returns {10,E1}; "
                    + "MyInfraWTwo short leg answers `f1=2` with {2,E1}",
            "deploy+snapshot; (f1,f2)-keyed MyInfraW table with late hash index on long "
                    + "f1: FAF `where f1=10` widens int 10 to long and returns {10,E1}; "
                    + "MyInfraWTwo short leg answers `f1=2` with {2,E1}"
    };

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 481-509
    // (ords 4-5, InfraWidening.run) and lines 527-571 (ords 2-3,
    // InfraHashBTreeWidening.run).
    private static final String EPL_W_CREATE_ONE_NW =
            "@public create window MyInfraW#keepall as (f1 long, f2 string)";
    private static final String EPL_W_CREATE_ONE_TABLE =
            "@public create table MyInfraW as (f1 long primary key, f2 string primary key)";
    private static final String EPL_W_INSERT_ONE =
            "insert into MyInfraW(f1, f2) select longPrimitive, theString from SupportBean";
    private static final String EPL_W_INDEX_ONE =
            "create index MyInfraWIndex1 on MyInfraW(f1)";
    private static final String EPL_W_SELECT_ONE = "select * from MyInfraW where f1=10";
    private static final String EPL_W_CREATE_TWO_NW =
            "@public create window MyInfraWTwo#keepall as (f1 short, f2 string)";
    private static final String EPL_W_CREATE_TWO_TABLE =
            "@public create table MyInfraWTwo as (f1 short primary key, f2 string primary key)";
    private static final String EPL_W_INSERT_TWO =
            "insert into MyInfraWTwo(f1, f2) select shortPrimitive, theString from SupportBean";
    private static final String EPL_W_INDEX_TWO =
            "create index MyInfraWTwoIndex1 on MyInfraWTwo(f1)";
    private static final String EPL_W_SELECT_TWO = "select * from MyInfraWTwo where f1=2";

    private static final String EPL_HBT_CREATE_ONE_NW =
            "@public create window MyInfraHBTW#keepall as (f1 long, f2 string)";
    private static final String EPL_HBT_CREATE_ONE_TABLE =
            "@public create table MyInfraHBTW as (f1 long primary key, f2 string primary key)";
    private static final String EPL_HBT_INSERT_ONE =
            "insert into MyInfraHBTW(f1, f2) select longPrimitive, theString from SupportBean";
    private static final String EPL_HBT_INDEX_ONE =
            "create index MyInfraHBTWIndex1 on MyInfraHBTW(f1 btree)";
    private static final String EPL_HBT_SELECT_ONE = "select * from MyInfraHBTW where f1>9";
    private static final String EPL_HBT_SODA_IX1 =
            "create index IX1 on MyInfraHBTW(f1, f2 btree)";
    private static final String EPL_HBT_SODA_IX2 =
            "create unique index IX2 on MyInfraHBTW(f1)";
    private static final String EPL_HBT_CREATE_TWO_NW =
            "@public create window MyInfraHBTWTwo#keepall as (f1 short, f2 string)";
    private static final String EPL_HBT_CREATE_TWO_TABLE =
            "@public create table MyInfraHBTWTwo as (f1 short primary key, f2 string primary key)";
    private static final String EPL_HBT_INSERT_TWO =
            "insert into MyInfraHBTWTwo(f1, f2) select shortPrimitive, theString from SupportBean";
    private static final String EPL_HBT_INDEX_TWO =
            "create index MyInfraHBTWTwoIndex1 on MyInfraHBTWTwo(f1 btree)";
    private static final String EPL_HBT_SELECT_TWO =
            "select * from MyInfraHBTWTwo where f1>=2";

    // SODA probe notes pinned by the unrepresentable steps; the oracle
    // replays eplToModelCompileDeploy before emitting the record.
    private static final String SODA_IX1_NOTE =
            "SODA create index IX1 on MyInfraHBTW(f1, f2 btree): the Java execution "
                    + "deploys the statement-object-model index over the long leg; "
                    + "EPL-object-model deploy has no Go boundary so the record is "
                    + "plan-only";
    private static final String SODA_IX2_NOTE =
            "SODA create unique index IX2 on MyInfraHBTW(f1): the Java execution "
                    + "deploys the statement-object-model unique index over the long "
                    + "leg; EPL-object-model deploy has no Go boundary so the record "
                    + "is plan-only";

    private static final String[] SNAPSHOT_FIELDS = {"f1", "f2"};

    private static final int EXPECTED_STEPS = 76;
    private static final int EXPECTED_RECORDS = 36;

    private InfraNWTableWidening558ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableWidening558ScenarioOracle <scenario.json>");
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
     * its own runtime). SupportBean is preconfigured, the internal timer is
     * disabled and the rethrowing exception handler surfaces statement
     * failures to the sender thread. Every deploy registers its deployment
     * by step label and its statements by name so deployed markers resolve
     * both; snapshot steps run compileExecuteFAF selects; unrepresentable
     * steps replay the SODA eplToModelCompileDeploy probes.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                com.espertech.esper.common.client.util.UndeployRethrowPolicy.RETHROW_FIRST);
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
                        EPDeployment deployment = compileDeploy(runtime, epl);
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
                        // compileExecuteFAF: the result array rows are
                        // projected to the pinned f1,f2 fields the way
                        // assertPropsPerRow reads them.
                        String label = string(step, "statement");
                        String[] fields = stringArray(step.get("fields"), "fields");
                        EPFireAndForgetQueryResult result =
                                executeFaf(runtime, configuration, string(step, "epl"));
                        JsonArray rows = new JsonArray();
                        for (EventBean event : result.getArray()) {
                            rows.add(projectedRow(event, fields));
                        }
                        if ("any".equals(string(step, "mode"))) {
                            sortRowsCanonical(rows);
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
                        records.add(record);
                        break;
                    }
                    case "unrepresentable": {
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String note = string(step, "expectError");
                        String expectedEpl;
                        String expectedNote;
                        if ("soda-ix1".equals(label)) {
                            expectedEpl = EPL_HBT_SODA_IX1;
                            expectedNote = SODA_IX1_NOTE;
                        } else if ("soda-ix2".equals(label)) {
                            expectedEpl = EPL_HBT_SODA_IX2;
                            expectedNote = SODA_IX2_NOTE;
                        } else {
                            throw new IllegalStateException(
                                    "unknown unrepresentable label " + label);
                        }
                        if (!expectedEpl.equals(epl) || !expectedNote.equals(note)) {
                            throw new IllegalStateException("unrepresentable step " + label
                                    + " is not pinned");
                        }
                        sodaDeploy(runtime, configuration, epl);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "unrepresentable");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", note);
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
    private static EPDeployment compileDeploy(EPRuntime runtime, String epl)
            throws EPCompileException, EPDeployException {
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

    /** sodaDeploy mirrors env.eplToModelCompileDeploy(epl, path): parse to
     * the statement object model, verify the toEPL round-trip, compile the
     * model as a module against the runtime path and deploy. */
    private static void sodaDeploy(EPRuntime runtime, Configuration configuration, String epl)
            throws EPCompileException, EPDeployException {
        EPStatementObjectModel model =
                EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
        if (!epl.trim().equals(model.toEPL())) {
            throw new IllegalStateException("SODA toEPL round-trip drift for " + epl);
        }
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        CompilerArguments args = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, args);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        if (!epl.trim().equals(deployment.getStatements()[0]
                .getProperty(StatementProperty.EPL))) {
            throw new IllegalStateException("SODA statement EPL drift for " + epl);
        }
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
     * runner's sortRowsCanonical freeze of Java's assertPropsPerRow: rows
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
     * numbers (Long/Short included), other numbers as doubles, boolean,
     * null as the tagged {"state":"null"} object.
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
     * Sends one pinned SupportBean event: the payload carries theString
     * plus exactly one numeric property, mirroring sendEventLong /
     * sendEventShort which populate only that setter.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if (!"SupportBean".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        SupportBean event = new SupportBean();
        event.setTheString(string(payload, "theString"));
        JsonValue longValue = payload.get("longPrimitive");
        JsonValue shortValue = payload.get("shortPrimitive");
        if (longValue != null && shortValue != null) {
            throw new IllegalArgumentException(
                    "SupportBean payload carries both numeric legs");
        }
        if (longValue != null) {
            event.setLongPrimitive(longInteger(longValue, "longPrimitive"));
        } else if (shortValue != null) {
            long parsed = longInteger(shortValue, "shortPrimitive");
            event.setShortPrimitive((short) parsed);
        } else {
            throw new IllegalArgumentException(
                    "SupportBean payload carries neither longPrimitive nor shortPrimitive");
        }
        runtime.getEventService().sendEventBean(event, type);
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
            if (caseName.startsWith("hashbtree")) {
                offset = validateHashBTreeCase(steps, offset, caseName);
            } else {
                offset = validateWideningCase(steps, offset, caseName);
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
            case "hashbtree-window":
                return String.join("\n", EPL_HBT_CREATE_ONE_NW, EPL_HBT_INSERT_ONE,
                        EPL_HBT_INDEX_ONE, EPL_HBT_SELECT_ONE, EPL_HBT_SODA_IX1,
                        EPL_HBT_SODA_IX2, EPL_HBT_CREATE_TWO_NW, EPL_HBT_INSERT_TWO,
                        EPL_HBT_INDEX_TWO, EPL_HBT_SELECT_TWO);
            case "hashbtree-table":
                return String.join("\n", EPL_HBT_CREATE_ONE_TABLE, EPL_HBT_INSERT_ONE,
                        EPL_HBT_INDEX_ONE, EPL_HBT_SELECT_ONE, EPL_HBT_SODA_IX1,
                        EPL_HBT_SODA_IX2, EPL_HBT_CREATE_TWO_TABLE, EPL_HBT_INSERT_TWO,
                        EPL_HBT_INDEX_TWO, EPL_HBT_SELECT_TWO);
            case "widening-window":
                return String.join("\n", EPL_W_CREATE_ONE_NW, EPL_W_INSERT_ONE,
                        EPL_W_INDEX_ONE, EPL_W_SELECT_ONE, EPL_W_CREATE_TWO_NW,
                        EPL_W_INSERT_TWO, EPL_W_INDEX_TWO, EPL_W_SELECT_TWO);
            default:
                return String.join("\n", EPL_W_CREATE_ONE_TABLE, EPL_W_INSERT_ONE,
                        EPL_W_INDEX_ONE, EPL_W_SELECT_ONE, EPL_W_CREATE_TWO_TABLE,
                        EPL_W_INSERT_TWO, EPL_W_INDEX_TWO, EPL_W_SELECT_TWO);
        }
    }

    /**
     * Exact step sequence of InfraHashBTreeWidening.run
     * (InfraNWTableCreateIndex.java lines 525-571): the long-leg
     * create/insert/btree-index deploys, the load send and the `f1>9` FAF
     * read, the two SODA index probes, then the short-leg
     * create/insert/btree-index deploys, the load send, the `f1>=2` FAF
     * read and undeployAll.
     */
    private static int validateHashBTreeCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create-one",
                window ? EPL_HBT_CREATE_ONE_NW : EPL_HBT_CREATE_ONE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create-one");
        validateDeploy(steps.get(offset++), caseName, "insert-one", EPL_HBT_INSERT_ONE);
        validateDeployed(steps.get(offset++), caseName, "insert-one");
        validateDeploy(steps.get(offset++), caseName, "index-one", EPL_HBT_INDEX_ONE);
        validateDeployed(steps.get(offset++), caseName, "index-one");
        validateBeanSend(steps.get(offset++), caseName, "E1", "longPrimitive", 10);
        validateSnapshot(steps.get(offset++), caseName, "select-gt-long", EPL_HBT_SELECT_ONE);
        validateUnrepresentable(steps.get(offset++), caseName, "soda-ix1",
                EPL_HBT_SODA_IX1, SODA_IX1_NOTE);
        validateUnrepresentable(steps.get(offset++), caseName, "soda-ix2",
                EPL_HBT_SODA_IX2, SODA_IX2_NOTE);
        validateDeploy(steps.get(offset++), caseName, "create-two",
                window ? EPL_HBT_CREATE_TWO_NW : EPL_HBT_CREATE_TWO_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create-two");
        validateDeploy(steps.get(offset++), caseName, "insert-two", EPL_HBT_INSERT_TWO);
        validateDeployed(steps.get(offset++), caseName, "insert-two");
        validateDeploy(steps.get(offset++), caseName, "index-two", EPL_HBT_INDEX_TWO);
        validateDeployed(steps.get(offset++), caseName, "index-two");
        validateBeanSend(steps.get(offset++), caseName, "E1", "shortPrimitive", 2);
        validateSnapshot(steps.get(offset++), caseName, "select-gte-short", EPL_HBT_SELECT_TWO);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraWidening.run (InfraNWTableCreateIndex.java
     * lines 478-510): the long-leg create/insert/hash-index deploys, the
     * load send and the `f1=10` FAF read, then the short-leg
     * create/insert/hash-index deploys, the load send, the `f1=2` FAF read
     * and undeployAll.
     */
    private static int validateWideningCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create-one",
                window ? EPL_W_CREATE_ONE_NW : EPL_W_CREATE_ONE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create-one");
        validateDeploy(steps.get(offset++), caseName, "insert-one", EPL_W_INSERT_ONE);
        validateDeployed(steps.get(offset++), caseName, "insert-one");
        validateDeploy(steps.get(offset++), caseName, "index-one", EPL_W_INDEX_ONE);
        validateDeployed(steps.get(offset++), caseName, "index-one");
        validateBeanSend(steps.get(offset++), caseName, "E1", "longPrimitive", 10);
        validateSnapshot(steps.get(offset++), caseName, "select-eq-long", EPL_W_SELECT_ONE);
        validateDeploy(steps.get(offset++), caseName, "create-two",
                window ? EPL_W_CREATE_TWO_NW : EPL_W_CREATE_TWO_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create-two");
        validateDeploy(steps.get(offset++), caseName, "insert-two", EPL_W_INSERT_TWO);
        validateDeployed(steps.get(offset++), caseName, "insert-two");
        validateDeploy(steps.get(offset++), caseName, "index-two", EPL_W_INDEX_TWO);
        validateDeployed(steps.get(offset++), caseName, "index-two");
        validateBeanSend(steps.get(offset++), caseName, "E1", "shortPrimitive", 2);
        validateSnapshot(steps.get(offset++), caseName, "select-eq-short", EPL_W_SELECT_TWO);
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

    /** Pins one snapshot step: op, case, label, mode "any", the f1,f2
     * projection fields and the compileExecuteFAF query text. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields", "epl");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"any".equals(string(step, "mode"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), SNAPSHOT_FIELDS,
                "snapshot fields for " + caseName);
    }

    /** Pins one unrepresentable step: op, case, label, the SODA EPL the
     * oracle replays and the plan-only note. */
    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedStatement, String expectedEpl,
                                                String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
    }

    /** Pins one sendEventLong/sendEventShort send: theString plus exactly
     * the numeric property the Java helper sets. */
    private static void validateBeanSend(JsonValue value, String caseName,
                                         String expectedString, String numericField,
                                         long expectedNumber) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", numericField);
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get(numericField), numericField) != expectedNumber) {
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

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory
            implements com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory {
        @Override
        public com.espertech.esper.common.client.hook.exception.ExceptionHandler getHandler(
                com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
