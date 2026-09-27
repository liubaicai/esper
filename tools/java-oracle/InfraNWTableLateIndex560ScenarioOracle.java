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
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableCreateIndex ordinals 8-11: the late-create
 * index executions where rows land in the named window/table before the
 * `create index` statement deploys and a fire-and-forget select then reads
 * them. Four executions on four runtimes.
 *
 * late-window/late-table (ords 8-9, InfraLateCreate{namedWindow=
 * true/false}): deploys the SupportBean keepall window or the
 * (theString,intPrimitive)-keyed table MyInfra fed by `insert into MyInfra
 * select theString, intPrimitive from SupportBean`, sends
 * SupportBean("A1",1), ("B2",2), ("B2",1), deploys
 * `create index MyInfra_IDX on MyInfra(theString)`, then runs the
 * compileExecuteFAF probe `select * from MyInfra where theString = 'B2'
 * order by intPrimitive asc` returning {B2,1},{B2,2}. The Java
 * `.addListener("Create")` registration has no observable output — a
 * create-infra statement never delivers events — so no listener record is
 * emitted.
 *
 * scene-two-window/scene-two-table (ords 10-11, InfraLateCreateSceneTwo
 * {namedWindow=true/false}): deploys the keepall window or fully-keyed
 * table MyInfraLC(f1 string, f2 int, f3 string, f4 string) fed by
 * `insert into MyInfraLC(f1, f2, f3, f4) select theString, intPrimitive,
 * '>'||theString||'<', '?'||theString||'?' from SupportBean`, sends
 * SupportBean("E1",-4), ("E1",-2), ("E1",-3), deploys
 * `create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)`, then runs the
 * compileExecuteFAF probe `select * from MyInfraLC where f3='>E1<' order by
 * f2 asc` returning the three E1 rows in f2 order {-4,-3,-2}.
 *
 * env.milestone(0)/milestone(1) checkpoints are harness no-ops and carry no
 * steps. The snapshot steps pin mode "ordered": the Java `order by` clause
 * fixes the row order positionally, so rows are emitted in engine order
 * without canonicalization.
 */
public final class InfraNWTableLateIndex560ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-late-index-560";
    private static final String DESCRIPTION =
            "InfraNWTableCreateIndex ordinals 8-11: InfraLateCreate (ords 8-9) "
                    + "deploys a SupportBean keepall window or (theString,"
                    + "intPrimitive)-keyed table MyInfra fed by `insert into MyInfra "
                    + "select theString, intPrimitive from SupportBean`, sends A1/1, "
                    + "B2/2, B2/1, then deploys `create index MyInfra_IDX on "
                    + "MyInfra(theString)` before the FAF probe `select * from "
                    + "MyInfra where theString = 'B2' order by intPrimitive asc` "
                    + "returns {B2,1},{B2,2} through the declared hash index; "
                    + "InfraLateCreateSceneTwo (ords 10-11) deploys a keepall window "
                    + "or fully-keyed table MyInfraLC(f1 string, f2 int, f3 string, "
                    + "f4 string) fed by the concat insert-into over SupportBean, "
                    + "sends E1/-4, E1/-2, E1/-3, then deploys `create index "
                    + "MyInfraLCIndex on MyInfraLC(f2, f3, f1)` before the FAF probe "
                    + "`select * from MyInfraLC where f3='>E1<' order by f2 asc` "
                    + "returns the three E1 rows in f2 order {-4,-3,-2}. The "
                    + "f3-leading probe resolves to a full scan on both engines (a "
                    + "composite hash index requires predicates on its leading "
                    + "columns); the late-create equality probe uses the declared "
                    + "MyInfra_IDX index. Java milestone checkpoints carry no steps "
                    + "(Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableCreateIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-ae0b11741e8c17c55271",
            "java-runtime-31d2b97272df49845355",
            "java-runtime-ba5fc9c488e24b9bae78",
            "java-runtime-386fcd9642ae114b58af"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraLateCreate{namedWindow=true}",
            "InfraLateCreate{namedWindow=false}",
            "InfraLateCreateSceneTwo{namedWindow=true}",
            "InfraLateCreateSceneTwo{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "late-window",
            "late-table",
            "scene-two-window",
            "scene-two-table"
    };
    private static final int[] ORDINALS = {8, 9, 10, 11};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+send+snapshot; MyInfra.win:keepall() SupportBean window fed by "
                    + "insert-into, three beans pre-index then the late "
                    + "MyInfra_IDX(theString) hash index: the ordered FAF probe "
                    + "`theString = 'B2'` resolves to the declared index and returns "
                    + "{B2,1},{B2,2} in intPrimitive order; milestone(0..1) "
                    + "checkpoints carry no steps",
            "deploy+send+snapshot; (theString,intPrimitive)-keyed MyInfra table fed "
                    + "by insert-into, three beans pre-index then the late "
                    + "MyInfra_IDX(theString) hash index: the ordered FAF probe "
                    + "`theString = 'B2'` resolves to the declared index and returns "
                    + "{B2,1},{B2,2} in intPrimitive order; milestone(0..1) "
                    + "checkpoints carry no steps",
            "deploy+send+snapshot; MyInfraLC#keepall window fed by the concat "
                    + "insert-into, three E1 beans pre-index then the late "
                    + "MyInfraLCIndex(f2,f3,f1) composite hash index: the ordered "
                    + "FAF probe `f3='>E1<'` leads with the non-leading column so "
                    + "both engines full-scan and return the three E1 rows in f2 "
                    + "order {-4,-3,-2}; milestone(0..1) checkpoints carry no steps",
            "deploy+send+snapshot; fully-keyed MyInfraLC table fed by the concat "
                    + "insert-into, three E1 beans pre-index then the late "
                    + "MyInfraLCIndex(f2,f3,f1) composite hash index: the ordered "
                    + "FAF probe `f3='>E1<'` leads with the non-leading column so "
                    + "both engines full-scan and return the three E1 rows in f2 "
                    + "order {-4,-3,-2}; milestone(0..1) checkpoints carry no steps"
    };

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 342-364
    // (ords 8-9, InfraLateCreate.run) and lines 389-410 (ords 10-11,
    // InfraLateCreateSceneTwo.run).
    private static final String EPL_LC_CREATE_NW =
            "@Name('Create') @public create window MyInfra.win:keepall() as SupportBean";
    private static final String EPL_LC_CREATE_TABLE =
            "@Name('Create') @public create table MyInfra(theString string primary key, intPrimitive int primary key)";
    private static final String EPL_LC_INSERT =
            "@Name('Insert') insert into MyInfra select theString, intPrimitive from SupportBean";
    private static final String EPL_LC_INDEX =
            "@Name('Index') create index MyInfra_IDX on MyInfra(theString)";
    private static final String EPL_LC_SELECT =
            "select * from MyInfra where theString = 'B2' order by intPrimitive asc";

    private static final String EPL_TWO_CREATE_NW =
            "@public create window MyInfraLC#keepall as (f1 string, f2 int, f3 string, f4 string)";
    private static final String EPL_TWO_CREATE_TABLE =
            "@public create table MyInfraLC as (f1 string primary key, f2 int primary key, f3 string primary key, f4 string primary key)";
    private static final String EPL_TWO_INSERT =
            "insert into MyInfraLC(f1, f2, f3, f4) select theString, intPrimitive, "
                    + "'>'||theString||'<', '?'||theString||'?' from SupportBean";
    private static final String EPL_TWO_INDEX =
            "create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)";
    private static final String EPL_TWO_SELECT =
            "select * from MyInfraLC where f3='>E1<' order by f2 asc";

    private static final String[] LC_SNAPSHOT_FIELDS = {"theString", "intPrimitive"};
    private static final String[] TWO_SNAPSHOT_FIELDS = {"f1", "f2", "f3", "f4"};

    private static final int EXPECTED_STEPS = 48;
    private static final int EXPECTED_RECORDS = 16;

    private InfraNWTableLateIndex560ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableLateIndex560ScenarioOracle <scenario.json>");
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
     * both; snapshot steps run the pinned compileExecuteFAF selects whose
     * `order by` clause fixes row order positionally.
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
                        // projected to the pinned fields the way
                        // assertPropsPerRow reads them; mode "ordered" keeps
                        // the engine's order-by ordering positional.
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

    /** Row projected to exactly the fields the step's assertions read,
     * with property names sorted alphabetically so the fields JSON matches
     * the Go runner's projected map. */
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
     * sort by their fields JSON (keys already sorted alphabetically). The
     * pinned probes use mode "ordered" so this never runs for this unit.
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
     * plus intPrimitive, mirroring sendEventBean(new SupportBean(...)).
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
            if (caseName.startsWith("scene-two")) {
                offset = validateSceneTwoCase(steps, offset, caseName);
            } else {
                offset = validateLateCase(steps, offset, caseName);
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
            case "late-window":
                return String.join("\n", EPL_LC_CREATE_NW, EPL_LC_INSERT, EPL_LC_INDEX,
                        EPL_LC_SELECT);
            case "late-table":
                return String.join("\n", EPL_LC_CREATE_TABLE, EPL_LC_INSERT, EPL_LC_INDEX,
                        EPL_LC_SELECT);
            case "scene-two-window":
                return String.join("\n", EPL_TWO_CREATE_NW, EPL_TWO_INSERT, EPL_TWO_INDEX,
                        EPL_TWO_SELECT);
            default:
                return String.join("\n", EPL_TWO_CREATE_TABLE, EPL_TWO_INSERT, EPL_TWO_INDEX,
                        EPL_TWO_SELECT);
        }
    }

    /**
     * Exact step sequence of InfraLateCreate.run
     * (InfraNWTableCreateIndex.java lines 337-370): the create/insert
     * deploys, the three pre-index SupportBean sends, the late index
     * deploy, the ordered FAF probe (milestone(0..1) carry no steps) and
     * undeployAll.
     */
    private static int validateLateCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create",
                window ? EPL_LC_CREATE_NW : EPL_LC_CREATE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LC_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "A1", 1);
        validateBeanSend(steps.get(offset++), caseName, "B2", 2);
        validateBeanSend(steps.get(offset++), caseName, "B2", 1);
        validateDeploy(steps.get(offset++), caseName, "index", EPL_LC_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateSnapshot(steps.get(offset++), caseName, "select", LC_SNAPSHOT_FIELDS,
                EPL_LC_SELECT);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraLateCreateSceneTwo.run
     * (InfraNWTableCreateIndex.java lines 387-413): the create/insert
     * deploys, the three pre-index SupportBean sends, the late composite
     * index deploy, the ordered FAF probe and undeployAll.
     */
    private static int validateSceneTwoCase(JsonArray steps, int offset, String caseName) {
        boolean window = caseName.endsWith("-window");
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create",
                window ? EPL_TWO_CREATE_NW : EPL_TWO_CREATE_TABLE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TWO_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", -4);
        validateBeanSend(steps.get(offset++), caseName, "E1", -2);
        validateBeanSend(steps.get(offset++), caseName, "E1", -3);
        validateDeploy(steps.get(offset++), caseName, "index", EPL_TWO_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateSnapshot(steps.get(offset++), caseName, "select", TWO_SNAPSHOT_FIELDS,
                EPL_TWO_SELECT);
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

    /** Pins one snapshot step: op, case, label, mode "ordered" (the Java
     * `order by` clause fixes row order positionally), the projection
     * fields and the compileExecuteFAF query text. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String[] expectedFields,
                                         String expectedEpl) {
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
        validateStringArray(step.get("fields"), expectedFields,
                "snapshot fields for " + caseName);
    }

    /** Pins one sendEventBean send: the SupportBean payload carries
     * theString plus intPrimitive. */
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
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
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
