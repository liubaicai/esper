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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the infra-named-window-keepall-delete slice of
 * InfraNamedWindowViews.java: the ord-1 InfraKeepAllSceneTwo three-module
 * scene (a projection keep-all window, an irstream insert-into into it, and an
 * on-trigger delete) plus the four ord-39-42 on-delete alias spellings
 * (InfraWithDeleteUseAs, InfraWithDeleteFirstAs, InfraWithDeleteSecondAs,
 * InfraWithDeleteNoAs) that share the tryCreateWindow matrix: a projection
 * window, an irstream insert-into, and the s0 value*2, s2 grouped sum and s3
 * value &gt;= 10 consumers over it.
 *
 * Replays the five pinned executions on one runtime with undeployAll between
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
 * onto one plan), while the four tryCreateWindow cases compile their five
 * leading statements (create, insert, s0, s2, s3) as ONE module exactly like
 * the suite's single compileDeploy, with the on-delete statement as the second
 * module; ordinal 1 deploys its three statements as three modules.  Joining
 * the pinned step EPLs of one module with ";\\n" reproduces the source literal
 * minus its trailing terminator.  The step schema is pinned by
 * validateScenario: case, deploy {statement, epl}, send {eventType, payload},
 * snapshot {statement, mode} and undeploy-all, so a drifted scenario fails
 * before any Esper object is created.
 *
 * Listener set: create, s0, s2 and s3 only, attached when the module
 * containing the statement completes, exactly like the source's
 * compileDeploy(...).addListener(...) chaining.  The on-delete statement never
 * gets a trace listener on either side: the Java suite never asserts its
 * payload ({@code assertListenerInvoked("delete")} at InfraNamedWindowViews
 * lines 3567/3584 only reads getIsInvokedAndReset(), which is still set from
 * the previous successful delete, and a no-match delete delivers nothing at
 * all because OnExprViewNamedWindowDelete.handleMatching only updates when
 * matching rows exist).  Deleted rows are observed through the create old
 * rows, the s0/s2/s3 remove-stream rows and the create iterator snapshots.
 *
 * Rows project exactly the two Java-asserted fields key and value (listener
 * rows in delivery order, snapshot rows in engine iterator order); a Java null
 * aggregate renders as {"state": "null"}, and empty streams are omitted the
 * same way the Go normalizer omits them.
 *
 * Dispatch order, pinned by this oracle and reported per contract section 6:
 * every insert and delete wave of the four tryCreateWindow cases delivers the
 * window statement's own listener FIRST and then the consumers in registration
 * order (s0, s2, s3) - the window statement registers its consumer view at
 * window creation, before s0/s2/s3 are deployed.  That order was confirmed
 * against the running Java engine in two structurally different deployments
 * (the suite's single five-statement module AND one module per statement, both
 * byte-identical traces) and it contradicts contract section 5.1, which
 * predicted s0/s2/s3 followed by the window statement from the sibling-chain
 * precedent; the trace pins the Java runtime, per contract section 6.
 */
public final class InfraNamedWindowKeepAllDeleteScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-keepall-delete";
    private static final String DESCRIPTION =
            "InfraNamedWindowViews keep-all named-window view slice: the legacy win:keepall() window"
                    + " over a theString/intBoxed projection with insert-into(intBoxed) and the four"
                    + " on-delete alias spellings over the key/value map schema with the shared"
                    + " tryCreateWindow consumer matrix (s0 value*2, s2 grouped sum with Esper"
                    + " first-row insert/remove pairs, s3 value >= 10 filter), captured from listener"
                    + " callbacks and ordered window or consumer iterator snapshots (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/"
                    + "namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "keepall-scene-two",
            "with-delete-use-as",
            "with-delete-first-as",
            "with-delete-second-as",
            "with-delete-no-as"
    };
    private static final int[] ORDINALS = {1, 39, 40, 41, 42};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-14f3c7199cf587fea29a",
            "java-runtime-6d4a6e1168f100add522",
            "java-runtime-8fe8d92928db5eba5fc3",
            "java-runtime-b47bdbdf2ac26de6cb0d",
            "java-runtime-594ea469769e1990e60f"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraKeepAllSceneTwo",
            "InfraWithDeleteUseAs",
            "InfraWithDeleteFirstAs",
            "InfraWithDeleteSecondAs",
            "InfraWithDeleteNoAs"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "keep-all projection window over theString/intBoxed with the legacy win:keepall()"
                    + " spelling; insert-into column list with irstream, on-delete alias on both"
                    + " sides, window listener new/old rows across seven inserts and three deletes"
                    + " with eleven ordered iterator snapshots",
            "map-schema window with aliases on both trigger and window (as s0 / as s1) in the"
                    + " on-delete statement",
            "projection window (select key, value from the map type) with the window-side alias"
                    + " only (as s1) and an unqualified trigger property",
            "map-schema window with the trigger-side alias only (as s0) and an unqualified window"
                    + " property",
            "projection window with the value alias preserved and no aliases in the on-delete"
                    + " statement"
    };

    // Transcriptions of InfraNamedWindowViews lines 195, 199 and 203
    // (InfraKeepAllSceneTwo), deployed as three separate modules exactly like
    // the source's three compileDeploy(stmtTextX, path) calls.  The source
    // text already carries @public on the window create, so no
    // deployment-convention annotation is added and all three are verbatim.
    private static final String EPL_SCENE_CREATE =
            "@name('create') @public create window MyWindow.win:keepall()"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_SCENE_INSERT =
            "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed"
                    + " from SupportBean";
    private static final String EPL_SCENE_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of the tryCreateWindow module (InfraNamedWindowViews
    // lines 3516-3520) and of the four per-case window creates (lines
    // 413/420/427/434).  The scenario lists one deploy step per statement
    // (the Go runner deploys one plan per step) while this oracle compiles the
    // five leading steps as ONE module, exactly like the suite's single
    // compileDeploy: joining the five pinned EPLs with ";\n" reproduces the
    // source literal minus its trailing ";\n".  The create statements carry
    // the source's own @public annotation.
    private static final String[] EPL_CREATES = {
            "@name('create') @public create window MyWindow#keepall as MySimpleKeyValueMap",
            "@name('create') @public create window MyWindow#keepall"
                    + " as select key, value from MySimpleKeyValueMap",
            "@name('create') @public create window MyWindow#keepall as MySimpleKeyValueMap",
            "@name('create') @public create window MyWindow#keepall"
                    + " as select key as key, value as value from MySimpleKeyValueMap"
    };
    private static final String EPL_INSERT =
            "@name('insert') insert into MyWindow select theString as key, longBoxed as value"
                    + " from SupportBean";
    private static final String EPL_S0 =
            "@name('s0') select irstream key, value*2 as value from MyWindow";
    private static final String EPL_S2 =
            "@name('s2') select irstream key, sum(value) as value from MyWindow group by key";
    private static final String EPL_S3 =
            "@name('s3') select irstream key, value from MyWindow where value >= 10";

    // Transcriptions of the tryCreateWindow delete argument (call sites at
    // lines 414/421/428/435) with the "@name('delete') " prefix the helper
    // prepends at line 3561, deployed as the second module.  The name is what
    // the source's undeployModuleContaining("delete") teardown resolves and
    // carries no semantics of its own.
    private static final String[] EPL_DELETES = {
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key",
            "@name('delete') on SupportMarketDataBean delete from MyWindow as s1"
                    + " where symbol = s1.key",
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow"
                    + " where s0.symbol = key",
            "@name('delete') on SupportMarketDataBean delete from MyWindow where symbol = key"
    };

    private static final String[] CASE_CREATE_EPLS = {
            EPL_SCENE_CREATE,
            EPL_CREATES[0],
            EPL_CREATES[1],
            EPL_CREATES[2],
            EPL_CREATES[3]
    };
    private static final String[] CASE_DELETE_EPLS = {
            EPL_SCENE_DELETE,
            EPL_DELETES[0],
            EPL_DELETES[1],
            EPL_DELETES[2],
            EPL_DELETES[3]
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "delete"},
            {"create", "insert", "s0", "s2", "s3", "delete"},
            {"create", "insert", "s0", "s2", "s3", "delete"},
            {"create", "insert", "s0", "s2", "s3", "delete"},
            {"create", "insert", "s0", "s2", "s3", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create"},
            {"create", "s0", "s2", "s3"},
            {"create", "s0", "s2", "s3"},
            {"create", "s0", "s2", "s3"},
            {"create", "s0", "s2", "s3"}
    };
    private static final int[] CASE_SNAPSHOTS = {11, 9, 9, 9, 9};

    /**
     * Listener discipline: keepall-scene-two listens to the window create
     * statement only (InfraNamedWindowViews line 196: the insert and delete
     * modules get none), and the four tryCreateWindow cases listen to create,
     * s0, s2 and s3 (line 3521: compileDeploy(epl, path).addListener("create")
     * .addListener("s0").addListener("s2").addListener("s3")).  The on-delete
     * statement gets no listener on either side; see the class comment for the
     * exclusion rationale.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out.  Ordinal 1 deploys create, insert and delete as three separate
     * modules; the four tryCreateWindow cases compile create, insert, s0, s2
     * and s3 as one module (the suite's single compileDeploy) and the on-delete
     * statement as a second module.
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> sceneModules = new HashMap<>();
        sceneModules.put("create", 0);
        sceneModules.put("insert", 1);
        sceneModules.put("delete", 2);
        modules.put(CASE_NAMES[0], sceneModules);
        Map<String, Integer> withDeleteModules = new HashMap<>();
        for (String statement : new String[]{"create", "insert", "s0", "s2", "s3"}) {
            withDeleteModules.put(statement, 0);
        }
        withDeleteModules.put("delete", 1);
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
            if (index > 0) {
                modules.put(CASE_NAMES[index], withDeleteModules);
            }
        }
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
    }

    /**
     * Every statement projects the same two fields; the Java assertions of
     * both execution shapes read "key" and "value" only.
     */
    private static final String[] FIELDS = {"key", "value"};

    private static final int[] EXPECTED_CASE_RECORDS = {20, 31, 31, 31, 31};
    private static final int EXPECTED_RECORDS = 144;
    private static final int EXPECTED_STEPS = 121;

    private InfraNamedWindowKeepAllDeleteScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowKeepAllDeleteScenarioOracle <scenario.json>");
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
                runCase(CASE_NAMES[index], configuration, runtime, allSteps, records);
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
     * prior deployments' public types so the second module's on-delete
     * statement can reference the window created by the first.  No plugin
     * function is needed for these executions.
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
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
                    deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                            statementsByName, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                pendingModule = module.intValue();
                pendingStatements.add(statementName);
                pendingEpls.add(string(step, "epl"));
                continue;
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
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
                    records.add(snapshot(runtime, statement, caseName));
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
            deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                    statementsByName, sequences, records);
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (exactly the Java source text of the suite's module), then
     * registers the statements by name and attaches the case's listeners in
     * statement order.
     */
    private static void deployModule(Configuration configuration, EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
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
        Map<String, EPStatement> deployed = new HashMap<>();
        for (EPStatement statement : deployment.getStatements()) {
            deployed.put(statement.getName(), statement);
        }
        if (deployed.size() != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.size() + " statements");
        }
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        for (String name : statementNames) {
            EPStatement statement = deployed.get(name);
            if (statement == null) {
                throw new IllegalStateException("module of case " + caseName
                        + " does not contain statement " + name);
            }
            statementsByName.put(name, statement);
            if (listened.contains(name)) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
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
     * the engine iterator order (keepall iteration is insertion order, the
     * order the Java exact-order iterator asserts pin).  An empty iterator
     * omits the new array, matching the Go normalizer's omitempty rendering.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            rows.add(projectedRow(iterator.next()));
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
    private static JsonObject projectedRow(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : FIELDS) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(projectedRow(event));
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
                    "description", "createEpl", "deleteEpl", "deploys", "listened",
                    "iteratorSnapshots");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !CASE_CREATE_EPLS[index].equals(string(definition, "createEpl"))
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
        int offset = validateKeepAllSceneTwoCase(steps, 0);
        for (int index = 1; index < CASE_NAMES.length; index++) {
            offset = validateWithDeleteCase(steps, offset, index);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact keepall-scene-two step sequence mirroring InfraKeepAllSceneTwo
     * lines 195-266: three module deploys (window create with the listener,
     * insert-into, on-trigger delete), the six bean sends and three market
     * deletes with the iterator snapshots the source asserts between them, and
     * the case-end undeploy-all that mirrors the insert/delete/create teardown.
     */
    private static int validateKeepAllSceneTwoCase(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SCENE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SCENE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SCENE_DELETE);
        validateBeanSend(steps.get(offset++), caseName, "G1", 10, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "G2", 20, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "G3", 30, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "G4", 40, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "G5", 50, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "G6", 60, null);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G6");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact tryCreateWindow step sequence mirroring lines 3516-3588: the five
     * leading module deploy steps (create, insert, s0, s2, s3 - one step per
     * statement, compiled as one module), the three irstream insert-into sends
     * with the create/s0 iterator snapshots the source asserts, the second
     * module's on-delete deploy at the contract's row-9 position, the four
     * market sends (one matching nothing), and the case-end undeploy-all that
     * mirrors the delete-then-s0 teardown.
     */
    private static int validateWithDeleteCase(JsonArray steps, int offset, int caseIndex) {
        String caseName = CASE_NAMES[caseIndex];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", CASE_CREATE_EPLS[caseIndex]);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_S0);
        validateDeploy(steps.get(offset++), caseName, "s2", EPL_S2);
        validateDeploy(steps.get(offset++), caseName, "s3", EPL_S3);
        validateBeanSend(steps.get(offset++), caseName, "E1", null, 10L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E2", null, 20L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E3", null, 5L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "delete", CASE_DELETE_EPLS[caseIndex]);
        validateMarketSend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "create");
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
     * Ordinal 1 sends intBoxed (intBoxed is the only value field the scene
     * window projects) and the four tryCreateWindow cases send longBoxed;
     * whatever the case, the payload carries exactly theString plus that one
     * value field.
     */
    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         Integer expectedIntBoxed, Long expectedLongBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (expectedIntBoxed != null) {
            requireFields(payload, "theString", "intBoxed");
            if (!expectedString.equals(string(payload, "theString"))
                    || integer(payload, "intBoxed") != expectedIntBoxed.intValue()) {
                throw new IllegalArgumentException("SupportBean payload is not pinned for "
                        + caseName);
            }
        } else {
            requireFields(payload, "theString", "longBoxed");
            if (!expectedString.equals(string(payload, "theString"))
                    || longInteger(payload.get("longBoxed"), "longBoxed")
                    != expectedLongBoxed.longValue()) {
                throw new IllegalArgumentException("SupportBean payload is not pinned for "
                        + caseName);
            }
        }
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
