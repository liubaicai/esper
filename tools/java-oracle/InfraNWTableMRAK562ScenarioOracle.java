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
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindow;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowInstance;
import com.espertech.esper.common.internal.epl.table.core.Table;
import com.espertech.esper.common.internal.epl.table.core.TableInstance;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanRange;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

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
 * Java oracle for InfraNWTableCreateIndex ordinals 0-1: the
 * InfraMultiRangeAndKey multi-range-and-key fire-and-forget probes over the
 * mixed-kind index idx1 (key hash, keyLong hash, rangeStartLong btree,
 * rangeEndLong btree). Two executions on two runtimes.
 *
 * mrak-window/mrak-table (ords 0-1, InfraMultiRangeAndKey
 * {namedWindow=true/false}): deploys the SupportBeanRange keepall window or
 * the (id, rangeStartLong, rangeEndLong)-keyed table MyInfraMRAK fed by the
 * `select *` insert-into (window) or the `merge ... when not matched then
 * insert` feed (table), the mixed-kind create-index idx1 statement, then the
 * four compileExecuteFAF runs of query1 `rangeStartLong > 1 and
 * rangeEndLong > 2 and keyLong=1 and key='K1' order by id asc` (empty, then
 * after E1/E2/E3 makes with milestone(0) between E2 and E3 — the milestone
 * carries no step) plus one run of query2 (same probes minus the key
 * conjunct). The tail asserts getIndexCount("create", "MyInfraMRAK") == 1
 * (window) / 2 (table: the idx1 descriptor plus the implicit primary-key
 * index) before undeployAll.
 *
 * The index-kind step rides an `unrepresentable` record: Java declares idx1
 * with per-column kinds (two hash plus two btree legs) while the Go surface
 * carries a single IndexKind per index and replays it as a creation-time
 * btree composite — the row sets are identical but the plan kind diverges,
 * so the record is plan-only.
 */
public final class InfraNWTableMRAK562ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-mrak-562";
    private static final String DESCRIPTION =
            "InfraNWTableCreateIndex ordinals 0-1: InfraMultiRangeAndKey "
                    + "{namedWindow=true/false} deploys a SupportBeanRange keepall "
                    + "window or the (id,rangeStartLong,rangeEndLong)-keyed table "
                    + "MyInfraMRAK fed by the select-* insert-into (window) or the "
                    + "merge-not-matched insert (table) plus the mixed-kind index "
                    + "idx1 (key hash, keyLong hash, rangeStartLong btree, "
                    + "rangeEndLong btree), then runs query1 (rangeStartLong>1, "
                    + "rangeEndLong>2, keyLong=1, key='K1', order by id asc) four "
                    + "times around the E1/E2/E3 sends with milestone(0) between "
                    + "E2 and E3 (a harness no-op carrying no step) and query2 "
                    + "(query1 minus key='K1') once, returning {}, {E1}, "
                    + "{E1,E2}, {E1,E2,E3} and {E1,E2,E3}; the mixed index kind "
                    + "has no Go single-Kind form and rides a plan-only record, "
                    + "and the tail asserts getIndexCount == 1 (window) / 2 "
                    + "(table). (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableCreateIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-7242c335a588f1d6fa15",
            "java-runtime-4d509bfd0e311be23afe"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraMultiRangeAndKey{namedWindow=true}",
            "InfraMultiRangeAndKey{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};
    private static final String[] CASES = {
            "mrak-window",
            "mrak-table"
    };
    private static final int[] ORDINALS = {0, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+send+snapshot+index-count+unrepresentable; SupportBeanRange "
                    + "MyInfraMRAK#keepall window fed by the select-* insert-into "
                    + "with mixed-kind index idx1 (key hash, keyLong hash, "
                    + "rangeStartLong btree, rangeEndLong btree) replayed as a "
                    + "single btree composite (plan-only record): the keyed FAF "
                    + "probe resolves idx1's equality prefix plus the "
                    + "rangeStartLong range leg (rangeEndLong post-filters) while "
                    + "the key-less probe full-scans; both return {E1},{E1,E2},"
                    + "{E1,E2,E3}; index-count 1",
            "deploy+send+snapshot+index-count+unrepresentable; "
                    + "(id,rangeStartLong,rangeEndLong)-keyed MyInfraMRAK table fed "
                    + "by the merge-not-matched insert with mixed-kind index idx1 "
                    + "replayed as a single btree composite (plan-only record): "
                    + "the keyed FAF probe resolves idx1's equality prefix plus "
                    + "the rangeStartLong range leg while the key-less probe "
                    + "full-scans; both return {E1},{E1,E2},{E1,E2,E3}; "
                    + "index-count 2 (idx1 plus the implicit primary-key "
                    + "descriptor)"
    };

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 586-622
    // (ords 0-1, InfraMultiRangeAndKey.run).
    private static final String EPL_CREATE_NW =
            "@name('create') @public create window MyInfraMRAK#keepall as SupportBeanRange";
    private static final String EPL_CREATE_TABLE =
            "@name('create') @public create table MyInfraMRAK(id string primary key, "
                    + "key string, keyLong long, rangeStartLong long primary key, "
                    + "rangeEndLong long primary key)";
    private static final String EPL_INSERT_NW =
            "insert into MyInfraMRAK select * from SupportBeanRange";
    private static final String EPL_INSERT_TABLE =
            "on SupportBeanRange t0 merge MyInfraMRAK t1 where t0.id = t1.id "
                    + "when not matched then insert select id, key, keyLong, "
                    + "rangeStartLong, rangeEndLong";
    private static final String EPL_INDEX =
            "create index idx1 on MyInfraMRAK(key hash, keyLong hash, "
                    + "rangeStartLong btree, rangeEndLong btree)";
    private static final String EPL_QUERY1 =
            "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 "
                    + "and keyLong=1 and key='K1' order by id asc";
    private static final String EPL_QUERY2 =
            "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 "
                    + "and keyLong=1 order by id asc";

    // Plan-divergence note pinned by the index-kind unrepresentable step:
    // Java's idx1 carries per-column kinds while Go indexes carry a single
    // Kind, so the replay declares the composite as one btree index.
    private static final String INDEX_KIND_NOTE =
            "create index idx1 on MyInfraMRAK(key hash, keyLong hash, "
                    + "rangeStartLong btree, rangeEndLong btree): Java declares "
                    + "per-column kinds (two hash plus two btree legs) while Go "
                    + "indexes carry a single Kind; the replay declares idx1 as a "
                    + "creation-time btree composite (key,keyLong,rangeStartLong,"
                    + "rangeEndLong) - identical rows (Go consumes the key+keyLong "
                    + "equality prefix and the rangeStartLong range leg; "
                    + "rangeEndLong>2 post-filters) so the kind split is "
                    + "plan-only";

    private static final String[] SNAPSHOT_FIELDS = {"id"};

    private static final int EXPECTED_STEPS = 36;
    private static final int EXPECTED_RECORDS = 20;

    private InfraNWTableMRAK562ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableMRAK562ScenarioOracle <scenario.json>");
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
     * its own runtime). SupportBeanRange is preconfigured, the internal
     * timer is disabled and the rethrowing exception handler surfaces
     * statement failures to the sender thread. Every deploy registers its
     * deployment by step label and its statements by name so deployed
     * markers and the index-count's "create" lookup resolve both; snapshot
     * steps run compileExecuteFAF selects (mode "ordered" — the `order by id
     * asc` clause fixes row order positionally); the index-kind
     * unrepresentable step emits the pinned plan-divergence note after the
     * mixed-kind index has been deployed; the index-count step resolves the
     * infra through the create statement's deployment exactly like
     * SupportInfraUtil.getIndexCountNoContext.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBeanRange.class);
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
        boolean namedWindow = caseName.endsWith("-window");
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
                        // projected to the pinned id field; `order by id asc`
                        // fixes the row order positionally (mode "ordered").
                        String label = string(step, "statement");
                        String[] fields = stringArray(step.get("fields"), "fields");
                        if (!"ordered".equals(string(step, "mode"))) {
                            throw new IllegalStateException(
                                    "snapshot mode is not ordered for " + label);
                        }
                        EPFireAndForgetQueryResult result =
                                executeFaf(runtime, configuration, string(step, "epl"));
                        JsonArray rows = new JsonArray();
                        for (EventBean event : result.getArray()) {
                            rows.add(projectedRow(event, fields));
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
                    case "index-count": {
                        // SupportInfraUtil.getIndexCountNoContext: the infra's
                        // index-descriptor count via the create statement's
                        // deployment.
                        String infraName = string(step, "statement");
                        String createLabel = string(step, "create");
                        String of = string(step, "of");
                        if (!"indexes".equals(of)) {
                            throw new IllegalArgumentException("unknown index-count kind: " + of);
                        }
                        EPStatement create = statements.get(createLabel);
                        if (create == null) {
                            throw new IllegalStateException(
                                    "index-count targets unknown create statement " + createLabel);
                        }
                        String deploymentId = create.getDeploymentId();
                        long actual;
                        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
                        if (namedWindow) {
                            NamedWindow window = spi.getServicesContext()
                                    .getNamedWindowManagementService()
                                    .getNamedWindow(deploymentId, infraName);
                            NamedWindowInstance instance = window.getNamedWindowInstance(null);
                            actual = instance.getIndexDescriptors().length;
                        } else {
                            Table table = spi.getServicesContext()
                                    .getTableManagementService()
                                    .getTable(deploymentId, infraName);
                            TableInstance instance = table.getTableInstance(-1);
                            actual = instance.getIndexRepository().getIndexDescriptors().length;
                        }
                        long expected = longInteger(step.get("count"), "count");
                        if (actual != expected) {
                            throw new IllegalStateException("index-count mismatch for " + infraName
                                    + " (indexes): expected " + expected + ", got " + actual);
                        }
                        int sequence = sequences.merge(infraName + ":index-count", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "index-count");
                        record.add("statement", infraName);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("count", actual);
                        records.add(record);
                        break;
                    }
                    case "unrepresentable": {
                        // The mixed-kind index declaration has no Go
                        // single-Kind form; the real `create index` module was
                        // already deployed by the index step, so the record is
                        // plan-only.
                        String label = string(step, "statement");
                        if (!"index-kind".equals(label)) {
                            throw new IllegalStateException(
                                    "unknown unrepresentable label " + label);
                        }
                        String epl = string(step, "epl");
                        String note = string(step, "expectError");
                        if (!EPL_INDEX.equals(epl) || !INDEX_KIND_NOTE.equals(note)) {
                            throw new IllegalStateException("unrepresentable step "
                                    + label + " is not pinned");
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

    /** Row projected to exactly the fields the step's assertions read. */
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
     * Sends one pinned SupportBeanRange event built with the five-arg
     * makeLong factory the Java execution uses
     * (sendEventBean(SupportBeanRange.makeLong(id, key, keyLong,
     * rangeStartLong, rangeEndLong))).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if (!"SupportBeanRange".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        String id = string(payload, "id");
        String key = string(payload, "key");
        Long keyLong = nullableLong(payload.get("keyLong"), "keyLong");
        Long rangeStartLong = nullableLong(payload.get("rangeStartLong"), "rangeStartLong");
        Long rangeEndLong = nullableLong(payload.get("rangeEndLong"), "rangeEndLong");
        runtime.getEventService().sendEventBean(
                SupportBeanRange.makeLong(id, key, keyLong, rangeStartLong, rangeEndLong), type);
    }

    private static Long nullableLong(JsonValue value, String label) {
        if (value == null || value.isNull()) {
            return null;
        }
        return longInteger(value, label);
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
            offset = validateMrakCase(steps, offset, caseName);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order; the index deploy and the index-kind
     * unrepresentable step both carry the create-index text, so idx1's EPL
     * appears twice. */
    private static String caseEpl(String caseName) {
        String create = caseName.endsWith("-window") ? EPL_CREATE_NW : EPL_CREATE_TABLE;
        String insert = caseName.endsWith("-window") ? EPL_INSERT_NW : EPL_INSERT_TABLE;
        return String.join("\n", create, insert, EPL_INDEX, EPL_INDEX,
                EPL_QUERY1, EPL_QUERY1, EPL_QUERY1, EPL_QUERY1, EPL_QUERY2);
    }

    /**
     * Exact step sequence of InfraMultiRangeAndKey.run
     * (InfraNWTableCreateIndex.java lines 586-626): the create/insert/index
     * deploys, the plan-only index-kind record, the query1 probes around the
     * three makeLong sends (milestone(0) carries no step), the query2 probe,
     * the getIndexCount assert and undeployAll.
     */
    private static int validateMrakCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create",
                window ? EPL_CREATE_NW : EPL_CREATE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert",
                window ? EPL_INSERT_NW : EPL_INSERT_TABLE);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateUnrepresentable(steps.get(offset++), caseName, "index-kind",
                EPL_INDEX, INDEX_KIND_NOTE);
        validateSnapshot(steps.get(offset++), caseName, "select-q1-empty", EPL_QUERY1);
        validateSend(steps.get(offset++), caseName, "E1", "K1", 1, 2, 3);
        validateSnapshot(steps.get(offset++), caseName, "select-q1-e1", EPL_QUERY1);
        validateSend(steps.get(offset++), caseName, "E2", "K1", 1, 2, 4);
        validateSnapshot(steps.get(offset++), caseName, "select-q1-e2", EPL_QUERY1);
        validateSend(steps.get(offset++), caseName, "E3", "K1", 1, 3, 3);
        validateSnapshot(steps.get(offset++), caseName, "select-q1-e3", EPL_QUERY1);
        validateSnapshot(steps.get(offset++), caseName, "select-q2", EPL_QUERY2);
        validateIndexCount(steps.get(offset++), caseName, window ? 1 : 2);
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

    /** Pins one snapshot step: op, case, label, mode "ordered", the id
     * projection field and the compileExecuteFAF query text. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields", "epl");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"ordered".equals(string(step, "mode"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), SNAPSHOT_FIELDS,
                "snapshot fields for " + caseName);
    }

    /** Pins one sendEventBean(SupportBeanRange.makeLong(...)) step. */
    private static void validateSend(JsonValue value, String caseName, String id, String key,
                                     long keyLong, long rangeStartLong, long rangeEndLong) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBeanRange".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/SupportBeanRange");
        }
        JsonObject payload = object(step.get("payload"), "SupportBeanRange payload");
        requireFields(payload, "id", "key", "keyLong", "rangeStartLong", "rangeEndLong");
        if (!id.equals(string(payload, "id"))
                || !key.equals(string(payload, "key"))
                || longInteger(payload.get("keyLong"), "keyLong") != keyLong
                || longInteger(payload.get("rangeStartLong"), "rangeStartLong") != rangeStartLong
                || longInteger(payload.get("rangeEndLong"), "rangeEndLong") != rangeEndLong) {
            throw new IllegalArgumentException(
                    "SupportBeanRange payload is not pinned for " + caseName);
        }
    }

    /** Pins the index-kind plan-divergence step: op, case, label, the
     * create-index EPL and the pinned note. */
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

    /** Pins the getIndexCount(env, namedWindow, "create", "MyInfraMRAK")
     * assert: statement names the infra, create names the @name('create')
     * deployment statement, of selects the indexes kind. */
    private static void validateIndexCount(JsonValue value, String caseName, long expected) {
        JsonObject step = object(value, "index-count step");
        requireFields(step, "op", "case", "statement", "create", "of", "count");
        if (!"index-count".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyInfraMRAK".equals(string(step, "statement"))
                || !"create".equals(string(step, "create"))
                || !"indexes".equals(string(step, "of"))
                || longInteger(step.get("count"), "count") != expected) {
            throw new IllegalArgumentException("index-count step is not pinned for " + caseName);
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
