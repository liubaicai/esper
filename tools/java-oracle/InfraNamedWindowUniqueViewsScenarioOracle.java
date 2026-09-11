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
 * Java oracle for the infra-named-window-unique-views slice of
 * InfraNamedWindowViews.java: the ord-32 InfraUnique {@code #unique(key)}
 * window over the MySimpleKeyValueMap schema (lines 2789-2829), the ord-33
 * InfraUniqueSceneTwo four-module projection {@code #unique(key)} window over
 * theString/intBoxed (lines 2831-2902) and the ord-34 InfraFirstUnique
 * {@code #firstunique(key)} window over the same map schema (lines 2904-2945).
 *
 * <p>Replays the three pinned executions on one runtime with undeployAll
 * between cases, mirroring the regression-suite harness: SupportBean from
 * esper-common, SupportMarketDataBean and MySimpleKeyValueMap registered
 * exactly as TestSuiteInfraNamedWindow does (the market trigger as a map type
 * carrying the regression-lib bean's symbol/price/volume/feed/id read surface,
 * since the regression-lib bean is not on the oracle classpath, and the
 * two-column map schema {key: String, value: long} verbatim), the internal
 * timer disabled, and the rethrowing exception handler with
 * UndeployRethrowPolicy.RETHROW_FIRST so statement failures surface to the
 * sender thread.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 32 and ord 34 compile their four statements
 * (create, insert, s0, delete) as ONE module exactly like the suite's single
 * compileDeploy, and ord 33 deploys its four statements as four modules (the
 * suite's four compileDeploy(stmtTextX, path) calls on one shared
 * RegressionPath).  Joining the pinned step EPLs of one module with ";\n"
 * reproduces the source literal minus its trailing terminator.  The insertion
 * statement of the ord-32/ord-34 single modules is unnamed in the Java source,
 * so the engine generates its name; the oracle therefore matches the module's
 * statements by position, and the scenario's "insert" step id is a
 * scenario-level identifier that no listen or snapshot step resolves (see
 * deployModule).  The step schema is pinned by validateScenario: case,
 * deploy {statement, epl}, send {eventType, payload}, snapshot {statement,
 * mode} and undeploy-all, so a drifted scenario fails before any Esper object
 * is created.
 *
 * <p>Listener set: create for all three cases plus s0 for ords 32 and 34,
 * attached when the module containing the statement completes, exactly like
 * the source's compileDeploy(...).addListener(...) chaining.  The on-delete
 * statement never gets a trace listener on either side: ords 32/34 attach one
 * in the suite but never assert its payload (InfraNamedWindowViews lines
 * 2797/2912 chain addListener("delete") and no assertion in the three
 * executions reads it), while ord 33 attaches no listener at all.  Matching
 * deletes DO deliver the deleted rows to the delete statement's own listener
 * as new data (the synthetic path of
 * OnExprViewNamedWindowDelete.handleMatching), so the exclusion rationale is
 * exactly the never-asserted payload, not silence.  Deleted rows are observed
 * through the create old rows and the create iterator snapshots.
 *
 * <p>Rows project exactly the fields the Java assertions read: every case
 * asserts {@code fields = {"key", "value"}} on its key/value window (ord 33's
 * value is the Integer projection of intBoxed while ords 32/34 project the
 * Long longBoxed; both render as JSON numbers).  Listener rows render in
 * delivery order.  Snapshot rows carry the step's mode: "ordered" steps keep
 * the engine iterator order the suite's assertPropsPerRowIterator pins, while
 * "any" steps render in canonical sorted-field order because the underlying
 * #unique/#firstunique view iterates a plain HashMap whose bucket order is not
 * contractual (the suite asserts assertPropsPerRowIteratorAnyOrder and the Go
 * chain sorts the same way before emitting).  A Java null aggregate renders as
 * {"state": "null"}, and empty streams are omitted the same way the Go
 * normalizer omits them; an empty iterator emits the snapshot record without a
 * rows array, which is how the suite's iteratorCount == 0 assertions show up
 * (no execution in this slice ever reaches an empty iterator).
 *
 * <p>Dispatch order, pinned by this oracle and reported per contract section 6:
 * every wave of the ord-32/ord-34 cases delivers the window statement's own
 * listener FIRST and then the s0 consumer - the window statement registers its
 * consumer view at window creation, before s0 is deployed - exactly the order
 * the sibling chain 4.381 measured and pinned by
 * internal/esper/namedwindow_statement_order_test.go.  The delete waves deliver
 * old only, the ord-32/ord-34 replacements deliver one invocation carrying the
 * new row and the replaced row (assertPropsIRPair), and the ord-34 swallowed
 * duplicates deliver nothing anywhere (assertListenerNotInvoked).
 */
public final class InfraNamedWindowUniqueViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-unique-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews unique-slice: the MySimpleKeyValueMap unique and firstunique windows and the projection unique window with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered or canonical any-mode window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "unique",
            "unique-scene-two",
            "firstunique"
    };
    private static final int[] ORDINALS = {32, 33, 34};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-63daa6cfa27a786c0480",
            "java-runtime-1ee31df7428f7c8e85eb",
            "java-runtime-bb670bc21e9d4333cefd"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraUnique",
            "InfraUniqueSceneTwo",
            "InfraFirstUnique"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "unique window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, re-admitted keys after delete, and any-mode two-row snapshots",
            "projection unique window over theString/intBoxed with four module deployments and only the window statement listened; every snapshot is any-mode",
            "firstunique window over the key/value map schema: duplicate keys are swallowed without any callback, deletes free the key for re-admission, and any-mode snapshots cover the two-row phases"
    };

    // Transcriptions of the InfraUnique EPL literal (lines 2793-2796), deployed
    // as ONE module exactly like the source's single compileDeploy; joining the
    // four pinned step EPLs with ";\n" reproduces the literal minus its
    // trailing terminator.
    private static final String EPL_UN_CREATE =
            "@name('create') create window MyWindowUN#unique(key) as MySimpleKeyValueMap";
    private static final String EPL_UN_INSERT =
            "insert into MyWindowUN select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_UN_S0 =
            "@name('s0') select irstream key, value as value from MyWindowUN";
    private static final String EPL_UN_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowUN as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 2837/2843/2849/2855
    // (InfraUniqueSceneTwo), deployed as four separate modules exactly like the
    // source's four compileDeploy(stmtTextX, path) calls.  The source text
    // already carries @public on the window create, so no deployment-convention
    // annotation is added and all four are verbatim; the consumer is named
    // "consume" (NOT "s0").
    private static final String EPL_SCENE_CREATE =
            "@name('create') @public create window MyWindow#unique(key)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_SCENE_INSERT =
            "@name('insert') insert into MyWindow(key, value)"
                    + " select irstream theString, intBoxed from SupportBean";
    private static final String EPL_SCENE_CONSUME =
            "@name('consume') select irstream key, value as value from MyWindow";
    private static final String EPL_SCENE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of the InfraFirstUnique EPL literal (lines 2908-2911),
    // deployed as ONE module exactly like the source's single compileDeploy.
    private static final String EPL_FU_CREATE =
            "@name('create') create window MyWindowFU#firstunique(key) as MySimpleKeyValueMap";
    private static final String EPL_FU_INSERT =
            "insert into MyWindowFU select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_FU_S0 =
            "@name('s0') select irstream key, value as value from MyWindowFU";
    private static final String EPL_FU_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFU as s1"
                    + " where s0.symbol = s1.key";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_UN_CREATE, EPL_SCENE_CREATE, EPL_FU_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_UN_INSERT, EPL_SCENE_INSERT, EPL_FU_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_UN_S0, "", EPL_FU_S0
    };
    private static final String[] CASE_CONSUME_EPLS = {
            "", EPL_SCENE_CONSUME, ""
    };
    private static final String[] CASE_DELETE_EPLS = {
            EPL_UN_DELETE, EPL_SCENE_DELETE, EPL_FU_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "consume", "delete"},
            {"create", "insert", "s0", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"create"},
            {"create", "s0"}
    };
    private static final int[] CASE_SNAPSHOTS = {6, 5, 7};

    /**
     * Listener discipline: ords 32 and 34 listen to create and s0 (lines 2797
     * and 2912: {@code addListener("delete").addListener("s0")
     * .addListener("create")}, with the never-asserted delete listener
     * excluded) and ord 33 listens to create only (lines 2838: the window
     * module's listener; the insert, consume and delete modules get none).
     * See the class comment for the exclusion rationale.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out.  Ord 33 deploys create, insert, consume and delete as four
     * modules (the suite's four compileDeploy calls on one RegressionPath);
     * ords 32 and 34 compile all four statements as one module (the suite's
     * single compileDeploy).
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: every case's window holds key/value rows and its Java
     * assertions read {@code fields = {"key", "value"}} (lines
     * 2801/2867/2916).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {"key", "value"}
    };

    private static final int[] EXPECTED_CASE_RECORDS = {20, 9, 17};
    private static final int EXPECTED_RECORDS = 46;
    private static final int EXPECTED_STEPS = 54;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
        }
        Map<String, Integer> singleModule = new HashMap<>();
        for (String statement : new String[]{"create", "insert", "s0", "delete"}) {
            singleModule.put(statement, 0);
        }
        modules.put(CASE_NAMES[0], singleModule);
        modules.put(CASE_NAMES[2], singleModule);
        Map<String, Integer> sceneModules = new HashMap<>();
        sceneModules.put("create", 0);
        sceneModules.put("insert", 1);
        sceneModules.put("consume", 2);
        sceneModules.put("delete", 3);
        modules.put(CASE_NAMES[1], sceneModules);
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
    }

    private InfraNamedWindowUniqueViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowUniqueViewsScenarioOracle <scenario.json>");
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
     * prior deployments' public types so the later ord-33 modules' insert-into
     * and on-delete statements can reference the window created by the first.
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
        // unique and firstunique modules unnamed (InfraNamedWindowViews lines
        // 2794/2909), so the engine assigns it a generated name; the scenario's
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
     * either the engine iterator order (ordered steps, the order the suite's
     * exact-order iterator assertions pin) or, for any-mode steps, the rows
     * sorted canonically by serialized fields because the #unique/#firstunique
     * view iterates a plain HashMap whose bucket order is not contractual.
     * An empty iterator omits the new array, matching the Go normalizer's
     * omitempty rendering and the suite's iteratorCount == 0 assertions.
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
        int offset = validateUnique(steps, 0);
        offset = validateUniqueSceneTwo(steps, offset);
        offset = validateFirstUnique(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraUnique step sequence mirroring lines 2793-2827: the four
     * leading module deploy steps (create, insert, s0, delete - one step per
     * statement, compiled as one module), the five SupportBean sends and two
     * market deletes with the six create iterator snapshots the source asserts
     * between them (ordered, any, ordered, any, any, ordered), and the
     * case-end undeploy-all that mirrors the source teardown.
     */
    private static int validateUnique(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_UN_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_UN_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_UN_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_UN_DELETE);
        validateLongSend(steps.get(offset++), caseName, "G1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateLongSend(steps.get(offset++), caseName, "G2", 20L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateLongSend(steps.get(offset++), caseName, "G1", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateLongSend(steps.get(offset++), caseName, "G2", 21L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateLongSend(steps.get(offset++), caseName, "G2", 22L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraUniqueSceneTwo step sequence mirroring lines 2837-2887: the
     * four module deploys (projection window create with the listener, named
     * insert-into, named consume consumer, on-trigger delete), the two bean
     * sends and two market deletes with all five create iterator snapshots in
     * any mode, and the case-end undeploy-all that mirrors the source teardown.
     */
    private static int validateUniqueSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SCENE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SCENE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "consume", EPL_SCENE_CONSUME);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SCENE_DELETE);
        validateIntSend(steps.get(offset++), caseName, "G1", 10);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateIntSend(steps.get(offset++), caseName, "G2", 20);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraFirstUnique step sequence mirroring lines 2908-2943: the four
     * leading module deploy steps (create, insert, s0, delete - one step per
     * statement, compiled as one module), the five SupportBean sends (two of
     * them swallowed duplicates) and two market deletes with the seven create
     * iterator snapshots the source asserts between them (ordered, any,
     * ordered, ordered, any, any, ordered), and the case-end undeploy-all that
     * mirrors the source teardown.
     */
    private static int validateFirstUnique(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_FU_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_FU_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FU_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_FU_DELETE);
        validateLongSend(steps.get(offset++), caseName, "G1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateLongSend(steps.get(offset++), caseName, "G2", 20L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateLongSend(steps.get(offset++), caseName, "G1", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
        validateLongSend(steps.get(offset++), caseName, "G2", 21L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateLongSend(steps.get(offset++), caseName, "G2", 22L);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered");
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
     * Ord 33 sends theString plus intBoxed, the only value field its window
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

    /** Ords 32 and 34 send theString plus longBoxed, the only value field their windows project. */
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
                                         String expectedStatement, String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
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
