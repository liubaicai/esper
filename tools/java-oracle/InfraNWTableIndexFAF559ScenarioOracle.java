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
 * Java oracle for InfraNWTableCreateIndex ordinals 6-7 and 20-21: the
 * composite-index fire-and-forget probes over a three-column hash index
 * (f2, f3, f1) whose where-clauses lead with the non-leading column f3.
 * Four executions on four runtimes.
 *
 * composite-window/composite-table (ords 6-7, InfraCompositeIndex
 * {namedWindow=true/false}): deploys the keepall window or f1-keyed
 * MyInfraCI(f1 string, f2 int, f3 string, f4 string) fed by `insert into
 * MyInfraCI(f1, f2, f3, f4) select theString, intPrimitive,
 * '>'||theString||'<', '?'||theString||'?' from SupportBean`, the
 * @name('indexOne') index MyInfraCIIndex on (f2, f3, f1), sends one
 * SupportBean("E1", -2), then runs the three compileExecuteFAF probes
 * `f3='>E1<'`, `f3='>E1<' and f2=-2`, `f3='>E1<' and f2=-2 and f1='E1'` —
 * each returning {E1,-2,>E1<,?E1?} — undeploys the indexOne module and
 * replays the SODA eplToModelCompileDeploy of `create index
 * MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1)` before undeployAll.
 *
 * multikey-window/multikey-table (ords 20-21, InfraMultikeyIndexFAF
 * {isNamedWindow=true/false}): the same fixture over MyInfra (the window
 * uses the `.win:keepall()` form) with index MyInfraIndex on (f2, f3, f1)
 * and milestone(0..2) checkpoints between the probes — the milestones are
 * harness no-ops and carry no steps.
 *
 * The SODA-tail steps ride `unrepresentable` records: the oracle performs
 * the real undeployModuleContaining("indexOne") and eplToModelCompileDeploy
 * before emitting the pinned notes; the Go side has no statement-object or
 * module-undeploy boundary for them and records the plan-only notes.
 */
public final class InfraNWTableIndexFAF559ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-index-faf-559";
    private static final String DESCRIPTION =
            "InfraNWTableCreateIndex ordinals 6-7 and 20-21: InfraCompositeIndex "
                    + "(ords 6-7) deploys a keepall window or f1-keyed MyInfraCI(f1 "
                    + "string, f2 int, f3 string, f4 string) fed by a concat "
                    + "insert-into over SupportBean, a composite hash index "
                    + "MyInfraCIIndex on (f2, f3, f1) and three f3-leading FAF probes "
                    + "each returning {E1,-2,>E1<,?E1?}, then the plan-only SODA tail "
                    + "(undeployModuleContaining indexOne + eplToModelCompileDeploy "
                    + "IX2); InfraMultikeyIndexFAF (ords 20-21) runs the same fixture "
                    + "over MyInfra (.win:keepall() window form) with MyInfraIndex and "
                    + "milestone(0..2) between probes (harness no-ops carrying no "
                    + "steps). The f3-leading and f3+f2 probes resolve to a full scan "
                    + "on both engines (a composite hash index requires its full "
                    + "column set); only the complete {f2,f3,f1} predicate uses the "
                    + "declared index. Expected row {E1,-2,>E1<,?E1?} (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableCreateIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableCreateIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-78145a646789e9674a38",
            "java-runtime-694aacd1c09bee51ffda",
            "java-runtime-4c0e49ebf2ae523ef825",
            "java-runtime-dbe715ce8e9fbede1285"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraCompositeIndex{namedWindow=true}",
            "InfraCompositeIndex{namedWindow=false}",
            "InfraMultikeyIndexFAF{isNamedWindow=true}",
            "InfraMultikeyIndexFAF{isNamedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};
    private static final String[] CASES = {
            "composite-window",
            "composite-table",
            "multikey-window",
            "multikey-table"
    };
    private static final int[] ORDINALS = {6, 7, 20, 21};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+snapshot+unrepresentable; MyInfraCI#keepall window with composite "
                    + "hash index (f2,f3,f1): the f3-leading and f3+f2 FAF probes "
                    + "resolve to a full scan (Java's hash index requires its full "
                    + "column set) while the complete {f2,f3,f1} predicate uses the "
                    + "declared index; each returns {E1,-2,>E1<,?E1?}; "
                    + "undeployModuleContaining(indexOne) and the SODA IX2 deploy are "
                    + "plan-only",
            "deploy+snapshot+unrepresentable; f1-keyed MyInfraCI table with composite "
                    + "hash index (f2,f3,f1): the f3-leading and f3+f2 FAF probes "
                    + "resolve to a full scan (Java's hash index requires its full "
                    + "column set) while the complete {f2,f3,f1} predicate uses the "
                    + "declared index; each returns {E1,-2,>E1<,?E1?}; "
                    + "undeployModuleContaining(indexOne) and the SODA IX2 deploy are "
                    + "plan-only",
            "deploy+snapshot; MyInfra.win:keepall() window with composite hash index "
                    + "MyInfraIndex (f2,f3,f1): the f3-leading and f3+f2 FAF probes "
                    + "resolve to a full scan while the complete {f2,f3,f1} predicate "
                    + "uses the declared index; each returns {E1,-2,>E1<,?E1?}; "
                    + "milestone(0..2) checkpoints carry no steps",
            "deploy+snapshot; f1-keyed MyInfra table with composite hash index "
                    + "MyInfraIndex (f2,f3,f1): the f3-leading and f3+f2 FAF probes "
                    + "resolve to a full scan while the complete {f2,f3,f1} predicate "
                    + "uses the declared index; each returns {E1,-2,>E1<,?E1?}; "
                    + "milestone(0..2) checkpoints carry no steps"
    };

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 430-457
    // (ords 6-7, InfraCompositeIndex.run) and lines 645-672 (ords 20-21,
    // InfraMultikeyIndexFAF.run).
    private static final String EPL_CI_CREATE_NW =
            "@public create window MyInfraCI#keepall as (f1 string, f2 int, f3 string, f4 string)";
    private static final String EPL_CI_CREATE_TABLE =
            "@public create table MyInfraCI as (f1 string primary key, f2 int, f3 string, f4 string)";
    private static final String EPL_CI_INSERT =
            "insert into MyInfraCI(f1, f2, f3, f4) select theString, intPrimitive, "
                    + "'>'||theString||'<', '?'||theString||'?' from SupportBean";
    private static final String EPL_CI_INDEX =
            "@name('indexOne') create index MyInfraCIIndex on MyInfraCI(f2, f3, f1)";
    private static final String EPL_CI_SELECT_F3 = "select * from MyInfraCI where f3='>E1<'";
    private static final String EPL_CI_SELECT_F3F2 =
            "select * from MyInfraCI where f3='>E1<' and f2=-2";
    private static final String EPL_CI_SELECT_FULL =
            "select * from MyInfraCI where f3='>E1<' and f2=-2 and f1='E1'";
    private static final String EPL_CI_SODA_TWO =
            "create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1)";

    private static final String EPL_MK_CREATE_NW =
            "@public create window MyInfra.win:keepall() as (f1 string, f2 int, f3 string, f4 string)";
    private static final String EPL_MK_CREATE_TABLE =
            "@public create table MyInfra as (f1 string primary key, f2 int, f3 string, f4 string)";
    private static final String EPL_MK_INSERT =
            "insert into MyInfra(f1, f2, f3, f4) select theString, intPrimitive, "
                    + "'>'||theString||'<', '?'||theString||'?' from SupportBean";
    private static final String EPL_MK_INDEX =
            "create index MyInfraIndex on MyInfra(f2, f3, f1)";
    private static final String EPL_MK_SELECT_F3 = "select * from MyInfra where f3='>E1<'";
    private static final String EPL_MK_SELECT_F3F2 =
            "select * from MyInfra where f3='>E1<' and f2=-2";
    private static final String EPL_MK_SELECT_FULL =
            "select * from MyInfra where f3='>E1<' and f2=-2 and f1='E1'";

    // The statement-name argument the Java composite tail passes to
    // undeployModuleContaining (not EPL); carried in the step's epl field.
    private static final String UNDEPLOY_INDEX_KEY = "indexOne";

    // SODA-tail notes pinned by the unrepresentable steps; the oracle
    // performs the real module undeploy and eplToModelCompileDeploy before
    // emitting the records.
    private static final String UNDEPLOY_INDEX_NOTE =
            "undeployModuleContaining(indexOne): the Java execution retires the "
                    + "@name('indexOne') index module before the SODA probe; the Go "
                    + "secondary index is declared at creation time and the deploy "
                    + "step is a catalog check, so no deployment exists to undeploy "
                    + "and the record is plan-only";
    private static final String SODA_INDEX_TWO_NOTE =
            "SODA create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1): the Java "
                    + "execution replays eplToModelCompileDeploy (parse -> toEPL -> "
                    + "module compile -> deploy) then undeployAll; EPL-object-model "
                    + "deploy has no Go boundary so the record is plan-only";

    private static final String[] SNAPSHOT_FIELDS = {"f1", "f2", "f3", "f4"};

    private static final int EXPECTED_STEPS = 52;
    private static final int EXPECTED_RECORDS = 28;

    private InfraNWTableIndexFAF559ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableIndexFAF559ScenarioOracle <scenario.json>");
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
     * steps perform the undeployModuleContaining and SODA
     * eplToModelCompileDeploy tails.
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
                        // projected to the pinned f1..f4 fields the way
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
                        if ("undeploy-index-one".equals(label)) {
                            if (!UNDEPLOY_INDEX_KEY.equals(epl)
                                    || !UNDEPLOY_INDEX_NOTE.equals(note)) {
                                throw new IllegalStateException("unrepresentable step "
                                        + label + " is not pinned");
                            }
                            // undeployModuleContaining("indexOne"): retire the
                            // deployment that owns the index statement.
                            EPDeployment indexModule = deployments.remove("index");
                            if (indexModule == null) {
                                throw new IllegalStateException(
                                        "no deployment registered for index label");
                            }
                            runtime.getDeploymentService().undeploy(indexModule.getDeploymentId());
                            statements.values().removeIf(
                                    statement -> "indexOne".equals(statement.getName()));
                        } else if ("soda-index-two".equals(label)) {
                            if (!EPL_CI_SODA_TWO.equals(epl)
                                    || !SODA_INDEX_TWO_NOTE.equals(note)) {
                                throw new IllegalStateException("unrepresentable step "
                                        + label + " is not pinned");
                            }
                            EPDeployment soda = sodaDeploy(runtime, configuration, epl);
                            // The Java execution calls .undeployAll() on the
                            // SODA deployment immediately.
                            runtime.getDeploymentService().undeploy(soda.getDeploymentId());
                        } else {
                            throw new IllegalStateException(
                                    "unknown unrepresentable label " + label);
                        }
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
    private static EPDeployment sodaDeploy(EPRuntime runtime, Configuration configuration,
                                           String epl) throws EPCompileException, EPDeployException {
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
        return deployment;
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
     * numbers, other numbers as doubles, boolean, null as the tagged
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
     * Sends one pinned SupportBean event: the payload carries theString
     * plus intPrimitive, mirroring sendEventBean(new SupportBean("E1", -2)).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if (!"SupportBean".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        String theString = string(payload, "theString");
        int intPrimitive = (int) longInteger(payload.get("intPrimitive"), "intPrimitive");
        runtime.getEventService().sendEventBean(new SupportBean(theString, intPrimitive), type);
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
            if (caseName.startsWith("composite")) {
                offset = validateCompositeCase(steps, offset, caseName);
            } else {
                offset = validateMultikeyCase(steps, offset, caseName);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order; the undeploy-index-one step carries
     * a statement-name key rather than EPL and is excluded. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "composite-window":
                return String.join("\n", EPL_CI_CREATE_NW, EPL_CI_INSERT, EPL_CI_INDEX,
                        EPL_CI_SELECT_F3, EPL_CI_SELECT_F3F2, EPL_CI_SELECT_FULL,
                        EPL_CI_SODA_TWO);
            case "composite-table":
                return String.join("\n", EPL_CI_CREATE_TABLE, EPL_CI_INSERT, EPL_CI_INDEX,
                        EPL_CI_SELECT_F3, EPL_CI_SELECT_F3F2, EPL_CI_SELECT_FULL,
                        EPL_CI_SODA_TWO);
            case "multikey-window":
                return String.join("\n", EPL_MK_CREATE_NW, EPL_MK_INSERT, EPL_MK_INDEX,
                        EPL_MK_SELECT_F3, EPL_MK_SELECT_F3F2, EPL_MK_SELECT_FULL);
            default:
                return String.join("\n", EPL_MK_CREATE_TABLE, EPL_MK_INSERT, EPL_MK_INDEX,
                        EPL_MK_SELECT_F3, EPL_MK_SELECT_F3F2, EPL_MK_SELECT_FULL);
        }
    }

    /**
     * Exact step sequence of InfraCompositeIndex.run
     * (InfraNWTableCreateIndex.java lines 430-457): the create/insert/index
     * deploys, the SupportBean("E1",-2) send, the three f3-leading FAF
     * probes, the plan-only undeployModuleContaining("indexOne") and SODA
     * IX2 records, then the undeployAll terminator.
     */
    private static int validateCompositeCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create",
                window ? EPL_CI_CREATE_NW : EPL_CI_CREATE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_CI_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_CI_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateBeanSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "select-f3", EPL_CI_SELECT_F3);
        validateSnapshot(steps.get(offset++), caseName, "select-f3-f2", EPL_CI_SELECT_F3F2);
        validateSnapshot(steps.get(offset++), caseName, "select-full", EPL_CI_SELECT_FULL);
        validateUnrepresentable(steps.get(offset++), caseName, "undeploy-index-one",
                UNDEPLOY_INDEX_KEY, UNDEPLOY_INDEX_NOTE);
        validateUnrepresentable(steps.get(offset++), caseName, "soda-index-two",
                EPL_CI_SODA_TWO, SODA_INDEX_TWO_NOTE);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraMultikeyIndexFAF.run
     * (InfraNWTableCreateIndex.java lines 645-672): the create/insert/index
     * deploys, the SupportBean("E1",-2) send, the three f3-leading FAF
     * probes (milestone(0..2) checkpoints carry no steps) and undeployAll.
     */
    private static int validateMultikeyCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create",
                window ? EPL_MK_CREATE_NW : EPL_MK_CREATE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_MK_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_MK_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateBeanSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "select-f3", EPL_MK_SELECT_F3);
        validateSnapshot(steps.get(offset++), caseName, "select-f3-f2", EPL_MK_SELECT_F3F2);
        validateSnapshot(steps.get(offset++), caseName, "select-full", EPL_MK_SELECT_FULL);
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

    /** Pins one snapshot step: op, case, label, mode "any", the f1..f4
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

    /** Pins one unrepresentable step: op, case, label, the carried key or
     * SODA EPL the oracle replays and the plan-only note. */
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

    /** Pins the single sendEventBean send: SupportBean("E1", -2) carries
     * theString plus intPrimitive. */
    private static void validateBeanSend(JsonValue value, String caseName) {
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
        if (!"E1".equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != -2) {
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
