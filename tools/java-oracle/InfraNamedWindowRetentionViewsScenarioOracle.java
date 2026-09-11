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
import com.espertech.esper.runtime.client.EPUndeployException;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the infra-named-window-retention-views slice of
 * InfraNamedWindowViews.java: the ord-0 InfraKeepAllSimple legacy
 * {@code win:keepall()} window over the SupportBean bean type (lines 159-187),
 * the ord-29 InfraLastEvent key/value map window with the {@code #lastevent}
 * retention view (lines 2641-2683), the ord-30 InfraLastEventSceneTwo
 * three-module projection window with the legacy {@code std:lastevent()}
 * spelling (lines 2685-2742) and the ord-31 InfraFirstEvent {@code #firstevent}
 * window over the same map schema (lines 2744-2787).
 *
 * Replays the four pinned executions on one runtime with undeployAll between
 * cases, mirroring the regression-suite harness: SupportBean from
 * esper-common, SupportMarketDataBean and MySimpleKeyValueMap registered
 * exactly as TestSuiteInfraNamedWindow does (the market trigger as a map type
 * carrying the regression-lib bean's symbol/price/volume/feed/id read surface,
 * since the regression-lib bean is not on the oracle classpath, and the
 * two-column map schema {key: String, value: long} verbatim), the internal
 * timer disabled, and the rethrowing exception handler with
 * UndeployRethrowPolicy.RETHROW_FIRST so statement failures surface to the
 * sender thread.
 *
 * Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 29 and ord 31 compile their four statements
 * (create, insert, s0, delete) as ONE module exactly like the suite's single
 * compileDeploy, ord 30 deploys its three statements as three modules and
 * ord 0 deploys its two statements as two modules (the suite's two
 * compileDeploy calls on the shared RegressionPath).  Joining the pinned step
 * EPLs of one module with ";\\n" reproduces the source literal minus its
 * trailing terminator; for ord 0 the two modules carry the source's capitalised
 * {@code @Name} annotation verbatim.  The insertion statement of the ord-29
 * and ord-31 single modules is unnamed in the Java source, so the engine
 * generates its name; the oracle therefore matches the module's statements by
 * position, and the scenario's "insert" step id is a scenario-level identifier
 * that no listen or snapshot step resolves (see deployModule).  The step schema
 * is pinned by validateScenario: case, deploy {statement, epl},
 * send {eventType, payload}, snapshot {statement, mode}, undeploy {statement}
 * and undeploy-all, so a drifted scenario fails before any Esper object is
 * created.
 *
 * Listener set: create for all four cases plus s0 for ords 29 and 31, attached
 * when the module containing the statement completes, exactly like the
 * source's compileDeploy(...).addListener(...) chaining.  The on-delete
 * statement never gets a trace listener on either side: ords 29-31 attach one
 * in the suite but never assert its payload ({@code
 * assertListenerNotInvoked("s0")} at InfraNamedWindowViews lines 2679/2783 only
 * reads isInvoked() and a no-match delete delivers nothing at all because
 * OnExprViewNamedWindowDelete.handleMatching only updates when matching rows
 * exist), and ord 0's insert module carries no listener.  Deleted rows are
 * observed through the create old rows and the create iterator snapshots.
 *
 * Rows project exactly the fields the Java assertions read: ord 0 asserts
 * {@code fields = "theString"} on a SupportBean-typed window, so its rows carry
 * theString only, while ords 29-31 assert {@code fields = {"key", "value"}} and
 * render the two-column projection (listener rows in delivery order, snapshot
 * rows in engine iterator order).  A Java null aggregate renders as
 * {"state": "null"}, and empty streams are omitted the same way the Go
 * normalizer omits them; an empty iterator emits the snapshot record without a
 * rows array, which is how the suite's iteratorCount == 0 assertions show up.
 *
 * Dispatch order, pinned by this oracle and reported per contract section 6:
 * every wave of the lastevent and firstevent cases delivers the window
 * statement's own listener FIRST and then the s0 consumer - the window
 * statement registers its consumer view at window creation, before s0 is
 * deployed - exactly the order the sibling chain 4.381 measured and pinned by
 * internal/esper/namedwindow_statement_order_test.go.
 */
public final class InfraNamedWindowRetentionViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-retention-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews retention-view slice: the legacy win:keepall() window over the SupportBean bean type, and the MySimpleKeyValueMap lastevent and firstevent windows with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "keepall-simple",
            "lastevent",
            "lastevent-scene-two",
            "firstevent"
    };
    private static final int[] ORDINALS = {0, 29, 30, 31};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-26c44410c8018a34d696",
            "java-runtime-b85cc831b5c23a570cf7",
            "java-runtime-59100403b3c6affd0729",
            "java-runtime-c2f54d9eb104d061950c"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraKeepAllSimple",
            "InfraLastEvent",
            "InfraLastEventSceneTwo",
            "InfraFirstEvent"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "legacy win:keepall() window over SupportBean with a separate insert-into module, two window-statement inserts and the two-statement module teardown",
            "lastevent window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, three delete waves each followed by an empty iterator, and a no-match delete that stays silent",
            "projection lastevent window over theString/intBoxed with three module deployments: replacement IR pair, delete to empty, re-insert after becoming empty",
            "firstevent window over the key/value map schema: a dropped insert emits nothing anywhere, deletes drain the window to empty, and re-inserts are admitted once empty"
    };

    // Transcriptions of InfraNamedWindowViews lines 166/169 (InfraKeepAllSimple):
    // the legacy win:keepall() bean-typed window and its separate insert-into
    // module, with the source's capitalised @Name annotation verbatim.  The
    // source deploys them as two compileDeploy calls on one RegressionPath and
    // tears them down with undeployModuleContaining("insert") followed by
    // undeployModuleContaining("create") (lines 182-183).
    private static final String EPL_KEEPALL_CREATE =
            "@Name('create') @public create window MyWindow.win:keepall() as SupportBean";
    private static final String EPL_KEEPALL_INSERT =
            "@Name('insert') insert into MyWindow select * from SupportBean";

    // Transcriptions of the InfraLastEvent EPL literal (lines 2645-2648),
    // deployed as ONE module exactly like the source's single compileDeploy;
    // joining the four pinned step EPLs with ";\n" reproduces the literal
    // minus its trailing terminator.
    private static final String EPL_LE_CREATE =
            "@name('create') create window MyWindowLE#lastevent as MySimpleKeyValueMap";
    private static final String EPL_LE_INSERT =
            "insert into MyWindowLE select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LE_S0 =
            "@name('s0') select irstream key, value as value from MyWindowLE";
    private static final String EPL_LE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowLE as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 2691/2695/2699
    // (InfraLastEventSceneTwo), deployed as three separate modules exactly like
    // the source's three compileDeploy(stmtTextX, path) calls.  The source text
    // already carries @public on the window create, so no deployment-convention
    // annotation is added and all three are verbatim.
    private static final String EPL_SCENE_CREATE =
            "@name('create') @public create window MyWindow.std:lastevent()"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_SCENE_INSERT =
            "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean";
    private static final String EPL_SCENE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of the InfraFirstEvent EPL literal (lines 2748-2751),
    // deployed as ONE module exactly like the source's single compileDeploy.
    private static final String EPL_FE_CREATE =
            "@name('create') create window MyWindowFE#firstevent as MySimpleKeyValueMap";
    private static final String EPL_FE_INSERT =
            "insert into MyWindowFE select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_FE_S0 =
            "@name('s0') select irstream key, value as value from MyWindowFE";
    private static final String EPL_FE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFE as s1"
                    + " where s0.symbol = s1.key";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_KEEPALL_CREATE, EPL_LE_CREATE, EPL_SCENE_CREATE, EPL_FE_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_KEEPALL_INSERT, EPL_LE_INSERT, EPL_SCENE_INSERT, EPL_FE_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            "", EPL_LE_S0, "", EPL_FE_S0
    };
    private static final String[] CASE_DELETE_EPLS = {
            "", EPL_LE_DELETE, EPL_SCENE_DELETE, EPL_FE_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete"},
            {"create", "insert", "s0", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create"},
            {"create", "s0"},
            {"create"},
            {"create", "s0"}
    };
    private static final int[] CASE_SNAPSHOTS = {0, 6, 3, 6};

    /**
     * Listener discipline: ord 0 listens to the window create statement only
     * (lines 167 and 170: the insert module gets none), ords 29 and 31 listen to create
     * and s0 (lines 2649/2752: {@code addListener("delete").addListener("s0")
     * .addListener("create")}, with the never-asserted delete listener
     * excluded) and ord 30 listens to create only (lines 2692 and 2700: the
     * insert module gets none and the delete listener is excluded).  See
     * the class comment for the exclusion rationale.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out.  Ord 0 deploys create and insert as two modules and ord 30
     * deploys create, insert and delete as three modules; ords 29 and 31
     * compile all four statements as one module (the suite's single
     * compileDeploy).
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: ord 0's window rows are SupportBean events and its Java
     * assertions read "theString" only (line 163); ords 29-31 project the
     * key/value map schema (lines 2643/2687/2746).
     */
    private static final String[][] CASE_FIELDS = {
            {"theString"},
            {"key", "value"},
            {"key", "value"},
            {"key", "value"}
    };

    private static final int[] EXPECTED_CASE_RECORDS = {2, 18, 7, 16};
    private static final int EXPECTED_RECORDS = 43;
    private static final int EXPECTED_STEPS = 58;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> keepAllModules = new HashMap<>();
        keepAllModules.put("create", 0);
        keepAllModules.put("insert", 1);
        modules.put(CASE_NAMES[0], keepAllModules);
        Map<String, Integer> singleModule = new HashMap<>();
        for (String statement : new String[]{"create", "insert", "s0", "delete"}) {
            singleModule.put(statement, 0);
        }
        modules.put(CASE_NAMES[1], singleModule);
        modules.put(CASE_NAMES[3], singleModule);
        Map<String, Integer> sceneModules = new HashMap<>();
        sceneModules.put("create", 0);
        sceneModules.put("insert", 1);
        sceneModules.put("delete", 2);
        modules.put(CASE_NAMES[2], sceneModules);
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
        }
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
    }

    private InfraNamedWindowRetentionViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowRetentionViewsScenarioOracle <scenario.json>");
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
     * Replays one case's steps on the shared runtime; sequences restart per
     * case.  Compiles with CompilerArguments(configuration) plus the runtime
     * path (the sibling-oracle convention): the configuration makes the base
     * event types resolvable to the compiler, and the runtime path carries the
     * prior deployments' public types so the later modules' insert-into and
     * on-delete statements can reference the window created by the first.
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
                    if (!"ordered".equals(string(step, "mode"))) {
                        throw new IllegalStateException("unsupported snapshot mode in case "
                                + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName, fields));
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
     * Resolves the deployment owning the named statement and undeploys it,
     * mirroring RegressionEnvironmentBase.undeployModuleContaining (the
     * ord-0 teardown at InfraNamedWindowViews lines 182-183).  The module's
     * statements are dropped from the by-name index the snapshot steps resolve
     * through; a missing statement is a scenario violation.
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
        // lastevent and firstevent modules unnamed (InfraNamedWindowViews lines
        // 2646/2749), so the engine assigns it a generated name; the scenario's
        // step id "insert" is still the module's second statement and no listen
        // or snapshot step resolves it.  Every statement the source does name
        // must appear at the pinned position under exactly that name.
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
     * the engine iterator order (keepall iteration is insertion order and the
     * lastevent/firstevent views hold at most one row, both orders the Java
     * exact-order iterator assertions pin).  An empty iterator omits the new
     * array, matching the Go normalizer's omitempty rendering and the suite's
     * iteratorCount == 0 assertions.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       String[] fields) {
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            rows.add(projectedRow(iterator.next(), fields));
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
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
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
                    "description", "createEpl", "insertEpl", "s0Epl", "deleteEpl", "deploys",
                    "listened", "iteratorSnapshots");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !CASE_CREATE_EPLS[index].equals(string(definition, "createEpl"))
                    || !CASE_INSERT_EPLS[index].equals(string(definition, "insertEpl"))
                    || !CASE_S0_EPLS[index].equals(string(definition, "s0Epl"))
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
        int offset = validateKeepAllSimple(steps, 0);
        offset = validateLastEvent(steps, offset);
        offset = validateLastEventSceneTwo(steps, offset);
        offset = validateFirstEvent(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraKeepAllSimple step sequence mirroring lines 166-185: the two
     * module deploys (bean-typed window create with the listener, insert-into),
     * the two SupportBean sends that carry only theString, and the two
     * statement-targeted teardown steps undeployModuleContaining("insert")
     * then undeployModuleContaining("create").  The case has no consumer and no
     * iterator snapshot.
     */
    private static int validateKeepAllSimple(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_KEEPALL_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_KEEPALL_INSERT);
        validateStringSend(steps.get(offset++), caseName, "E1");
        validateStringSend(steps.get(offset++), caseName, "E2");
        validateUndeploy(steps.get(offset++), caseName, "insert");
        validateUndeploy(steps.get(offset++), caseName, "create");
        return offset;
    }

    /**
     * Exact InfraLastEvent step sequence mirroring lines 2645-2681: the four
     * leading module deploy steps (create, insert, s0, delete - one step per
     * statement, compiled as one module), the four SupportBean sends and three
     * market deletes with the create iterator snapshots the source asserts
     * between them, and the case-end undeploy-all that mirrors the s0-then-
     * undeploy teardown.
     */
    private static int validateLastEvent(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LE_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_LE_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLastEventSceneTwo step sequence mirroring lines 2691-2727:
     * three module deploys (projection window create with the listener,
     * insert-into, on-trigger delete), the three bean sends and one market
     * delete with the create iterator snapshots the source asserts between
     * them, and the case-end undeploy-all that mirrors the destroy-all
     * teardown.
     */
    private static int validateLastEventSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SCENE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SCENE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SCENE_DELETE);
        validateIntSend(steps.get(offset++), caseName, "G1", 1);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G3", 3);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraFirstEvent step sequence mirroring lines 2748-2785: the four
     * leading module deploy steps (create, insert, s0, delete - one step per
     * statement, compiled as one module), the four SupportBean sends and four
     * market deletes (one matching nothing and one dropped key) with the create
     * iterator snapshots the source asserts between them, and the case-end
     * undeploy-all that mirrors the s0-then-undeploy teardown.
     */
    private static int validateFirstEvent(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_FE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_FE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FE_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_FE_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateMarketSend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E1");
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
     * Ord 0 sends theString only: every other SupportBean property stays at its
     * Java default (intPrimitive 0, boxed values null) and the payload carries
     * exactly one field.
     */
    private static void validateStringSend(JsonValue value, String caseName,
                                           String expectedString) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString");
        if (!expectedString.equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ord 30 sends theString plus intBoxed, the only value field its window projects. */
    private static void validateIntSend(JsonValue value, String caseName, String expectedString,
                                        int expectedIntBoxed) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "intBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intBoxed") != expectedIntBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ords 29 and 31 send theString plus longBoxed, the only value field their windows project. */
    private static void validateLongSend(JsonValue value, String caseName, String expectedString,
                                         long expectedLongBoxed) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "longBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("longBoxed"), "longBoxed") != expectedLongBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
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
