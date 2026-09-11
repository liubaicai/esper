import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the infra-named-window-length-views slice of
 * InfraNamedWindowViews.java: the ord-11 InfraLengthWindow {@code #length(3)}
 * window over the MySimpleKeyValueMap schema (lines 1095-1143), the ord-12
 * InfraLengthWindowSceneTwo legacy {@code win:length(3)} projection window
 * over theString/intBoxed (lines 1145-1257), the ord-13
 * InfraLengthFirstWindow {@code #firstlength(2)} window over the same map
 * schema (lines 1259-1298) and the ord-25 InfraLengthWindowSceneThree
 * bean-backed {@code #length(2)} window over SupportBean with a SupportBean_A
 * delete trigger (lines 2459-2503).
 *
 * <p>Replays the four pinned executions on one runtime with undeployAll
 * between cases, mirroring the regression-suite harness: SupportBean from
 * esper-common, SupportMarketDataBean registered exactly as the sibling oracle
 * does (a map type carrying the regression-lib bean's symbol/price/volume/feed
 * read surface, since the regression-lib bean is not on the oracle classpath),
 * MySimpleKeyValueMap as the two-column map schema {key: String, value: long}
 * verbatim, and SupportBean_A as the {id: String} map type the ord-25 delete
 * trigger reads.  The internal timer is disabled and the rethrowing exception
 * handler with UndeployRethrowPolicy.RETHROW_FIRST surfaces statement failures
 * to the sender thread.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 11 and ord 13 compile their four statements
 * (create, insert, s0, delete) as ONE module exactly like the suite's single
 * compileDeploy, ord 12 deploys three modules on one shared RegressionPath
 * (create, insert, delete - the case has no consumer statement) and ord 25
 * deploys four modules on one shared RegressionPath (create, insert, delete,
 * s0).  Joining the pinned step EPLs of one module with ";\n" reproduces the
 * source literal minus its trailing terminator.  The insertion statement of
 * the ord-11/ord-13 single modules and the create/insert/delete statements of
 * the ord-12/ord-25 modules are unnamed in the Java source, so the engine
 * generates their names; the oracle therefore matches the module's statements
 * by position, and the scenario's step ids of those statements are
 * scenario-level identifiers that no listen or snapshot step resolves (see
 * deployModule).  The step schema is pinned by validateScenario: case, deploy
 * {statement, epl}, send {eventType, payload}, snapshot {statement, mode} and
 * undeploy-all, so a drifted scenario fails before any Esper object is
 * created.
 *
 * <p>Listener set: create and s0 for ords 11 and 13, create for ord 12, s0 for
 * ord 25, attached when the module containing the statement completes, exactly
 * like the source's compileDeploy(...).addListener(...) chaining.  The on-delete
 * statement never gets a trace listener on either side: ords 11/12/13 attach
 * one in the suite ({@code addListener("delete")}) but never assert its
 * payload, while ord 25 attaches no listener at all.  Matching deletes DO
 * deliver the deleted rows to the delete statement's own listener as new data
 * (the synthetic path of OnExprViewNamedWindowDelete), so the exclusion
 * rationale is exactly the never-asserted payload, not silence.  Deleted rows
 * are observed through the window statement's old rows and the ordered
 * iterator snapshots.
 *
 * <p>Rows project exactly the fields the Java assertions read: the key/value
 * windows assert {@code fields = {"key", "value"}} (ord 12's value is the
 * Integer projection of intBoxed while ords 11/13 project the Long longBoxed;
 * both render as JSON numbers) and the ord-25 bean window asserts
 * {@code {"theString"}}.  Listener rows render in delivery order.  Snapshot
 * rows carry the step's mode: every step of this slice is "ordered" and keeps
 * the engine iterator order the suite's assertPropsPerRowIterator pins; the
 * canonical any-mode branch of snapshot() is kept identical to the sibling
 * oracle's shared protocol but no step here uses it.  An empty iterator emits
 * the snapshot record without a rows array, which is how the suite's
 * {@code null} / {@code new Object[0][]} iterator assertions show up - ord 25
 * reaches it twice, at lines 2469 and 2481.
 *
 * <p>Dispatch order, pinned by this oracle and reported per contract section 6:
 * every wave of the ord-11/ord-13 cases delivers the window statement's own
 * listener FIRST and then the s0 consumer - the window statement registers its
 * consumer view at window creation, before s0 is deployed.  The delete waves
 * deliver old only, the length-window replacements deliver one invocation
 * carrying the new row and the evicted row (assertPropsIRPair), and the
 * ord-13 inserts into a full window deliver nothing anywhere because
 * FirstLengthWindowView only updates its child when something changed.
 */
public final class InfraNamedWindowLengthViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-length-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews length-slice: the MySimpleKeyValueMap length and firstlength windows, the projection length window and the bean-backed length window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "length-window",
            "length-window-scene-two",
            "length-first-window",
            "length-window-scene-three"
    };
    private static final int[] ORDINALS = {11, 12, 13, 25};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d931bfbbbaa3ea632c2b",
            "java-runtime-793f6ec7608e25556803",
            "java-runtime-b53dbf8f541e79537f32",
            "java-runtime-c656dd0eeae6618a4162"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraLengthWindow",
            "InfraLengthWindowSceneTwo",
            "InfraLengthFirstWindow",
            "InfraLengthWindowSceneThree"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "length(3) window over the key/value map schema: FIFO eviction delivers one invocation carrying the new row and the evicted row, with ordered snapshots around every delete and eviction",
            "legacy win:length(3) projection window over theString/intBoxed with three module deployments and only the window statement listened across ten delete and eviction waves",
            "firstlength(2) window over the key/value map schema: inserts into the full window are dropped without any callback, deletes free a slot, and the freed slot is filled at the tail by the next arrival",
            "bean-backed length(2) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; the first two snapshots are empty"
    };

    // Transcriptions of the InfraLengthWindow EPL literal (lines 1099-1102),
    // deployed as ONE module exactly like the source's single compileDeploy;
    // joining the four pinned step EPLs with ";\n" reproduces the literal
    // minus its trailing terminator.
    private static final String EPL_LW_CREATE =
            "@name('create') create window MyWindowLW#length(3) as MySimpleKeyValueMap";
    private static final String EPL_LW_INSERT =
            "insert into MyWindowLW select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LW_S0 =
            "@name('s0') select irstream key, value as value from MyWindowLW";
    private static final String EPL_LW_DELETE =
            "@name('delete') on SupportMarketDataBean delete from MyWindowLW where symbol = key";

    // Transcriptions of InfraNamedWindowViews lines 1151/1155/1159
    // (InfraLengthWindowSceneTwo), deployed as three separate modules exactly
    // like the source's three compileDeploy(stmtTextX, path) calls on one
    // shared RegressionPath.  The window create keeps the legacy
    // "MyWindow.win:length(3)" spelling and the @public marker verbatim; the
    // case has no consumer statement, so only the window statement is
    // listened.  The ord-12 insert-into carries an explicit column list and
    // an inert irstream (a filter source has no remove stream).
    private static final String EPL_SCENE_CREATE =
            "@name('create') @public create window MyWindow.win:length(3)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_SCENE_INSERT =
            "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean";
    private static final String EPL_SCENE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of the InfraLengthFirstWindow EPL literal (lines
    // 1263-1266), deployed as ONE module exactly like the source's single
    // compileDeploy.
    private static final String EPL_LFW_CREATE =
            "@name('create') create window MyWindowLFW#firstlength(2) as MySimpleKeyValueMap";
    private static final String EPL_LFW_INSERT =
            "insert into MyWindowLFW select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LFW_S0 =
            "@name('s0') select irstream key, value as value from MyWindowLFW";
    private static final String EPL_LFW_DELETE =
            "@name('delete') on SupportMarketDataBean delete from MyWindowLFW where symbol = key";

    // Transcriptions of InfraNamedWindowViews lines 2463-2466
    // (InfraLengthWindowSceneThree), deployed as four separate modules exactly
    // like the source's four compileDeploy(stmtText, path) calls on one shared
    // RegressionPath.  The bean-backed window, the insert-into and the
    // SupportBean_A on-delete trigger are unnamed in the source; the consumer
    // carries the capital @Name('s0') verbatim and is the only listened
    // statement.
    private static final String EPL_ABC_CREATE =
            "@public create window ABCWin#length(2) as SupportBean";
    private static final String EPL_ABC_INSERT =
            "insert into ABCWin select * from SupportBean";
    private static final String EPL_ABC_DELETE =
            "on SupportBean_A delete from ABCWin where theString = id";
    private static final String EPL_ABC_S0 =
            "@Name('s0') select irstream * from ABCWin";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_LW_CREATE, EPL_SCENE_CREATE, EPL_LFW_CREATE, EPL_ABC_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_LW_INSERT, EPL_SCENE_INSERT, EPL_LFW_INSERT, EPL_ABC_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_LW_S0, "", EPL_LFW_S0, EPL_ABC_S0
    };
    private static final String[] CASE_CONSUME_EPLS = {"", "", "", ""};
    private static final String[] CASE_DELETE_EPLS = {
            EPL_LW_DELETE, EPL_SCENE_DELETE, EPL_LFW_DELETE, EPL_ABC_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete", "s0"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"create"},
            {"create", "s0"},
            {"s0"}
    };
    private static final int[] CASE_SNAPSHOTS = {4, 13, 4, 4};

    /**
     * Module grouping per deploy position: ords 11 and 13 compile all four
     * statements as one module (0,0,0,0), ord 12 deploys three modules on one
     * shared path (0,1,2) and ord 25 four modules (0,1,2,3).  See the class
     * comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 1, 2},
            {0, 0, 0, 0},
            {0, 1, 2, 3}
    };

    /**
     * Listener discipline: ords 11 and 13 listen to create and s0 (lines 1103
     * and 1267: {@code addListener("delete").addListener("s0")
     * .addListener("create")}, with the never-asserted delete listener
     * excluded), ord 12 listens to create only (line 1152: the window
     * module's listener; the insert and delete modules get none) and ord 25
     * listens to s0 only (line 2466).  See the class comment for the
     * exclusion rationale.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: the key/value windows assert
     * {@code fields = {"key", "value"}} (lines 1097/1147/1261) and the ord-25
     * bean window asserts {@code "theString".split(",")} (line 2462).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {"key", "value"},
            {"theString"}
    };

    private static final int[] EXPECTED_CASE_RECORDS = {18, 24, 12, 11};
    private static final int EXPECTED_RECORDS = 65;
    private static final int EXPECTED_STEPS = 79;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
            Map<String, Integer> moduleKeys = new HashMap<>();
            for (int position = 0; position < CASE_DEPLOYS[index].length; position++) {
                moduleKeys.put(CASE_DEPLOYS[index][position], CASE_MODULE_KEYS[index][position]);
            }
            modules.put(CASE_NAMES[index], moduleKeys);
        }
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
    }

    private InfraNamedWindowLengthViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowLengthViewsScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportMarketDataBean", marketDataSchema());
        configuration.getCommon().addEventType("MySimpleKeyValueMap", simpleKeyValueSchema());
        configuration.getCommon().addEventType("SupportBean_A", supportBeanASchema());
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (int index = 0; index < CASE_NAMES.length; index++) {
                int before = records.size();
                runCase(CASE_NAMES[index], CASE_FIELDS[index], configuration, runtime, allSteps,
                        records);
                int emitted = records.size() - before;
                if (emitted != EXPECTED_CASE_RECORDS[index]) {
                    throw new IllegalStateException("case " + CASE_NAMES[index] + " emitted "
                            + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
                }
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
     * The map event type the suite registers as MySimpleKeyValueMap
     * (TestSuiteInfraNamedWindow lines 136-139): key string, value long.
     */
    private static Map<String, Object> simpleKeyValueSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("key", String.class);
        schema.put("value", long.class);
        return schema;
    }

    /**
     * Map event type carrying the regression-lib SupportMarketDataBean read
     * surface (symbol, price, volume, feed, id); the regression-lib bean is
     * not on the oracle classpath and only symbol participates in the pinned
     * EPL.  Sends mirror new SupportMarketDataBean(symbol, 0, 0L, "").
     */
    private static Map<String, Object> marketDataSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("symbol", String.class);
        schema.put("price", double.class);
        schema.put("volume", Long.class);
        schema.put("feed", String.class);
        schema.put("id", String.class);
        return schema;
    }

    /**
     * Map event type for the ord-25 delete trigger: SupportBean_A carries the
     * single String property id (support/bean/SupportBeanAtoFBase), and the
     * regression-lib bean is not on the oracle classpath.  Sends mirror
     * sendSupportBean_A(env, id) -> new SupportBean_A(id).
     */
    private static Map<String, Object> supportBeanASchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("id", String.class);
        return schema;
    }

    /**
     * Replays one case's steps on the shared runtime; sequences restart per
     * case.  Compiles with CompilerArguments(configuration) plus the runtime
     * path (the sibling-oracle convention): the configuration makes the base
     * event types resolvable to the compiler, and the runtime path carries the
     * prior deployments' public types so the later ord-12/ord-25 modules'
     * insert-into and on-delete statements can reference the window created by
     * the first.
     */
    private static void runCase(String caseName, String[] fields, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        int pendingModule = -1;
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
            String operation = string(step, "op");
            if ("deploy".equals(operation)) {
                String statementName = string(step, "statement");
                Integer module = MODULE_KEYS.get(caseName).get(statementName);
                if (module == null) {
                    throw new IllegalStateException("case " + caseName + " deploys unknown statement "
                            + statementName);
                }
                if (!pendingStatements.isEmpty() && pendingModule != module.intValue()) {
                    deployModule(configuration, runtime, caseName, fields, pendingStatements,
                            pendingEpls, statementsByName, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                pendingModule = module.intValue();
                pendingStatements.add(statementName);
                pendingEpls.add(string(step, "epl"));
                continue;
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(configuration, runtime, caseName, fields, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
                pendingStatements.clear();
                pendingEpls.clear();
            }
            switch (operation) {
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
                    String mode = string(step, "mode");
                    if (!"ordered".equals(mode) && !"any".equals(mode)) {
                        throw new IllegalStateException("unsupported snapshot mode " + mode
                                + " in case " + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName, fields, mode));
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        if (!pendingStatements.isEmpty()) {
            deployModule(configuration, runtime, caseName, fields, pendingStatements, pendingEpls,
                    statementsByName, sequences, records);
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (exactly the Java source text of the suite's module), then
     * registers the statements in module order under the scenario's step ids
     * and attaches the case's listeners in statement order.
     */
    private static void deployModule(Configuration configuration, EPRuntime runtime, String caseName,
                                     String[] fields, List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        List<EPStatement> deployed = new ArrayList<>();
        Set<String> deployedNames = new HashSet<>();
        for (EPStatement statement : deployment.getStatements()) {
            if (!deployedNames.add(statement.getName())) {
                throw new IllegalStateException("module of case " + caseName
                        + " deployed two statements named " + statement.getName());
            }
            deployed.add(statement);
        }
        if (deployed.size() != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.size() + " statements");
        }
        // Statements are matched by position, which is the order of the module
        // EPL text: the Java source leaves the insertion statement of the
        // length and firstlength modules unnamed (InfraNamedWindowViews lines
        // 1100/1264) and the create/insert/delete statements of the scene-two
        // and scene-three modules unnamed (lines 1151/1155/1159 and
        // 2463/2464/2465), so the engine assigns them generated names; the
        // scenario's step ids of those statements are still the module
        // positions and no listen or snapshot step resolves them.  Every
        // statement the source does name must appear at the pinned position
        // under exactly that name.
        Set<String> scenarioNames = new HashSet<>(statementNames);
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        for (int index = 0; index < statementNames.size(); index++) {
            String name = statementNames.get(index);
            EPStatement statement = deployed.get(index);
            if (scenarioNames.contains(statement.getName()) && !statement.getName().equals(name)) {
                throw new IllegalStateException("module of case " + caseName + " statement "
                        + statement.getName() + " is not the pinned statement at position "
                        + index + " (" + name + ")");
            }
            if (!scenarioNames.contains(statement.getName()) && deployedNames.contains(name)) {
                throw new IllegalStateException("module of case " + caseName + " pins the script "
                        + "name " + name + " for an unnamed statement while the module also names "
                        + "another statement " + name);
            }
            statementsByName.put(name, statement);
            if (listened.contains(name)) {
                statement.addListener(listener(caseName, fields, sequences, records, runtime));
            }
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.
     */
    private static UpdateListener listener(String caseName, String[] fields,
                                           Map<String, Integer> sequences, JsonArray records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents, fields);
            JsonArray oldRows = rows(oldEvents, fields);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
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
     * either the engine iterator order (ordered steps, the order the suite's
     * exact-order iterator assertions pin) or, for any-mode steps, the rows
     * sorted canonically by serialized fields.  Every step of this slice is
     * ordered, so the any-mode branch is the sibling oracle's shared protocol
     * code and is unreached here.  An empty iterator omits the new array,
     * matching the Go normalizer's omitempty rendering and the suite's
     * {@code null} / {@code new Object[0][]} assertions.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       String[] fields, String mode) {
        List<JsonObject> fieldRows = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            fieldRows.add(projectedFields(iterator.next(), fields));
        }
        if ("any".equals(mode)) {
            fieldRows.sort(Comparator.comparing(JsonObject::toString));
        }
        JsonArray rows = new JsonArray();
        for (JsonObject fieldRow : fieldRows) {
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fieldRow);
            rows.add(row);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        return record;
    }

    /**
     * Row rendering projected to exactly the fields the Java assertions read;
     * the window row carries no other property any execution observes.
     */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", projectedFields(event, fields));
        return item;
    }

    /** The projected field object of one event in the case's declared field order. */
    private static JsonObject projectedFields(EventBean event, String[] fields) {
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        return values;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(projectedRow(event, fields));
        }
        return array;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object the Go normalizer also emits.
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
                JsonValue intBoxed = payload.get("intBoxed");
                if (intBoxed != null) {
                    bean.setIntBoxed(integer(payload, "intBoxed"));
                }
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null) {
                    bean.setLongBoxed(longInteger(longBoxed, "longBoxed"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportMarketDataBean": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("symbol", string(payload, "symbol"));
                event.put("price", 0.0d);
                event.put("volume", 0L);
                event.put("feed", "");
                event.put("id", null);
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "SupportBean_A": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", string(payload, "id"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASE_NAMES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "description", "createEpl", "insertEpl", "s0Epl", "consumeEpl", "deleteEpl",
                    "deploys", "listened", "iteratorSnapshots");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !CASE_CREATE_EPLS[index].equals(string(definition, "createEpl"))
                    || !CASE_INSERT_EPLS[index].equals(string(definition, "insertEpl"))
                    || !CASE_S0_EPLS[index].equals(string(definition, "s0Epl"))
                    || !CASE_CONSUME_EPLS[index].equals(string(definition, "consumeEpl"))
                    || !CASE_DELETE_EPLS[index].equals(string(definition, "deleteEpl"))
                    || integer(definition, "iteratorSnapshots") != CASE_SNAPSHOTS[index]) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateStringArray(definition.get("deploys"), CASE_DEPLOYS[index],
                    "deploys for " + CASE_NAMES[index]);
            validateStringArray(definition.get("listened"), CASE_LISTENED[index],
                    "listened for " + CASE_NAMES[index]);
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = validateLengthWindow(steps, 0);
        offset = validateLengthWindowSceneTwo(steps, offset);
        offset = validateLengthFirstWindow(steps, offset);
        offset = validateLengthWindowSceneThree(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraLengthWindow step sequence mirroring lines 1099-1141: the
     * four leading module deploy steps (create, insert, s0, delete - one step
     * per statement, compiled as one module), the six SupportBean sends and the
     * E2 market delete with the four ordered create iterator snapshots the
     * source asserts between them, and the case-end undeploy-all that mirrors
     * the source teardown.  The E5 and E6 inserts are the FIFO evictions whose
     * single invocation carries the new and the evicted row.
     */
    private static int validateLengthWindow(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LW_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LW_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LW_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_LW_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E5", 5L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E6", 6L);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLengthWindowSceneTwo step sequence mirroring lines
     * 1151-1255: the three module deploys (projection window create with the
     * listener, insert-into, on-trigger delete), the eight bean sends and
     * three market deletes with all thirteen ordered create iterator snapshots
     * the source asserts between them - including the four pairs that re-assert
     * the same window across a milestone - and the case-end undeploy-all that
     * mirrors the source teardown.  G6 and G8 are the FIFO evictions.
     */
    private static int validateLengthWindowSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SCENE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SCENE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SCENE_DELETE);
        validateIntSend(steps.get(offset++), caseName, "G1", 10);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 20);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G3", 30);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G4", 40);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G5", 50);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G6", 60);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G6");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G7", 70);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G8", 80);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLengthFirstWindow step sequence mirroring lines 1263-1297:
     * the four leading module deploy steps (create, insert, s0, delete - one
     * step per statement, compiled as one module), the four SupportBean sends
     * (E3 and E5 arrive at the full window and are dropped) and the E2 market
     * delete with the four ordered create iterator snapshots the source
     * asserts between them, and the case-end undeploy-all that mirrors the
     * source teardown.
     */
    private static int validateLengthFirstWindow(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LFW_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LFW_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LFW_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_LFW_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E5", 5L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLengthWindowSceneThree step sequence mirroring lines
     * 2463-2502: the four module deploys (bean-backed window create, insert,
     * SupportBean_A on-delete trigger, @Name('s0') consumer with the listener)
     * in the source's compileDeploy order, the five SupportBean sends and two
     * SupportBean_A deletes with the four ordered s0 iterator snapshots the
     * source asserts between them, and the case-end undeploy-all that mirrors
     * the source teardown.  The first two snapshots are the slice's only empty
     * iterators (lines 2469 and 2481), so their records carry no rows.  The
     * one-argument sendSupportBean helper leaves intBoxed/longBoxed null, so
     * the SupportBean payloads carry theString only.
     */
    private static int validateLengthWindowSceneThree(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_ABC_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ABC_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_ABC_DELETE);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_ABC_S0);
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateStringSend(steps.get(offset++), caseName, "E1");
        validateSupportBeanASend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateStringSend(steps.get(offset++), caseName, "E2");
        validateStringSend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateSupportBeanASend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateStringSend(steps.get(offset++), caseName, "E4");
        validateStringSend(steps.get(offset++), caseName, "E5");
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

    /**
     * Ord 12 sends theString plus intBoxed, the only value field its window
     * projects; every other SupportBean property stays at its Java default.
     */
    private static void validateIntSend(JsonValue value, String caseName, String expectedString,
                                        int expectedIntBoxed) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "intBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intBoxed") != expectedIntBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ords 11 and 13 send theString plus longBoxed, the only value field their windows project. */
    private static void validateLongSend(JsonValue value, String caseName, String expectedString,
                                         long expectedLongBoxed) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "longBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("longBoxed"), "longBoxed") != expectedLongBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /**
     * Ord 25 sends theString only: the source calls the one-argument
     * sendSupportBean helper (line 3471), leaving intBoxed and longBoxed at
     * their null defaults.
     */
    private static void validateStringSend(JsonValue value, String caseName, String expectedString) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString");
        if (!expectedString.equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ord 25's on-delete trigger sends new SupportBean_A(id) with the single id property. */
    private static void validateSupportBeanASend(JsonValue value, String caseName,
                                                 String expectedId) {
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

    private static JsonObject validateBeanStep(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        return object(step.get("payload"), "SupportBean payload");
    }

    private static void validateMarketSend(JsonValue value, String caseName, String expectedSymbol) {
        JsonObject step = object(value, "SupportMarketDataBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportMarketDataBean step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload");
        requireFields(payload, "symbol");
        if (!expectedSymbol.equals(string(payload, "symbol"))) {
            throw new IllegalArgumentException("SupportMarketDataBean payload is not pinned for "
                    + caseName);
        }
    }

    /** Every snapshot step of this slice is exact-order. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"ordered".equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
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
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
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
                || !new HashSet<>(object.names()).equals(
                        new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields "
                    + (object == null ? "<null>" : object.names()) + ", expected "
                    + Arrays.toString(expectedNames));
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
