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
 * Java oracle for the infra-named-window-time-views slice of
 * InfraNamedWindowViews.java: the ord-4 InfraTimeWindowSceneTwo legacy
 * {@code MyWindow#time(10 sec)} projection window over theString/intBoxed with
 * a separately deployed irstream consumer (lines 528-648), the ord-5
 * InfraTimeFirstWindow {@code MyWindowTFW#firsttime(10 sec)} window over the
 * MySimpleKeyValueMap schema (lines 650-704) and the ord-8
 * InfraExtTimeWindowSceneThree bean-backed {@code ABCWin.win:time(10 sec)}
 * window over SupportBean with a SupportBean_A delete trigger and a wildcard
 * irstream consumer (lines 859-917).
 *
 * <p>Unlike the sibling slices every execution of this slice moves the engine
 * clock: the scenario therefore carries {@code advance-time} steps, and the
 * oracle replays them through {@code EPRuntime.getEventService().advanceTime}
 * exactly as the suite's {@code sendTimer} helper does (the internal timer is
 * disabled, so schedules fire only on those calls).  Record {@code time} values
 * are the virtual clock at delivery as rendered by
 * {@code Instant.ofEpochMilli}, which for these cases is the epoch plus the
 * advance offset and not the wall clock.
 *
 * <p>One EPRuntime is created per case, with a distinct runtime URI, because
 * the pinned timelines are not monotonic in sequence: ord 4 runs 0 to 36000,
 * ord 5 requires a deploy at clock 1000 and runs to 100000, and ord 8 restarts
 * at 0.  {@code SchedulingServiceImpl.setTime} assigns the clock unconditionally
 * with no backwards guard, so sharing one runtime across the three cases (the
 * sibling-oracle style) would move the clock backwards and corrupt pending
 * schedules.  Each case tears its runtime down with undeployAll + destroy.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 5 compiles its four statements (create, insert, s0,
 * delete) as ONE module exactly like the suite's single compileDeploy, and ords
 * 4 and 8 deploy four modules on one shared path (ord 4: create, insert,
 * consume, delete; ord 8: create, insert, s0, delete).  Joining the pinned step
 * EPLs of one module with ";\n" reproduces the source literal minus its
 * trailing terminator.  The create/consume statements of ord 4, the
 * insert/s0/delete statements of ord 8 and the insert statement of ord 5 are
 * named in the source, and the window create of ord 4/8 carries {@code @public}
 * because the later modules of those cases resolve it through the runtime path.
 * The oracle matches each module's statements by position and pins the names
 * the source does declare.  The step schema is pinned by validateScenario:
 * case, deploy {statement, epl}, send {eventType, payload}, snapshot
 * {statement, mode}, advance-time {at} and undeploy-all, so a drifted scenario
 * fails before any Esper object is created.
 *
 * <p>Listener set: create and consume for ord 4, create and s0 for ord 5, s0
 * for ord 8, attached when the module containing the statement completes,
 * exactly like the source's compileDeploy(...).addListener(...) chaining.  The
 * on-delete statement never gets a trace listener on either side: ords 4 and 5
 * attach one in the suite ({@code addListener("delete")}) but never assert its
 * payload, while ord 8 attaches no listener at all.  Matching deletes DO
 * deliver the deleted rows to the delete statement's own listener as new data
 * (the synthetic path of OnExprViewNamedWindowDelete), so the exclusion
 * rationale is exactly the never-asserted payload, not silence.  Deleted rows
 * are observed through the window statement's old rows and the ordered iterator
 * snapshots.
 *
 * <p>Rows project exactly the fields the Java assertions read:
 * {@code fields = {"key", "value"}} for ords 4 and 5 (ord 4 projects intBoxed
 * as Integer while ord 5 projects longBoxed as Long; both render as JSON
 * numbers, so the Go projection must not coerce between them) and
 * {@code {"theString"}} for ord 8's bean-typed window.  Listener rows render in
 * delivery order.  Every snapshot step of this slice is exact-order (the suite
 * calls assertPropsPerRowIterator only, never the any-order variant) and keeps
 * the engine iterator order; the canonical any-mode branch of snapshot() is the
 * sibling oracle's shared protocol code and is unreached here.  An empty
 * iterator emits the snapshot record without a rows array, which is how the
 * suite's {@code null} / {@code new Object[0][]} iterator assertions show up -
 * ord 4 reaches it twice (lines 623 and 627), ord 5 once (line 661) and ord 8
 * three times (lines 872, 886 and 913).
 *
 * <p>Record counts are pinned to the Java source: ord 4 emits 30 records (10
 * create listener + 10 consume listener + 10 snapshots), ord 5 emits 15 (4
 * create + 4 s0 + 7 snapshots) and ord 8 emits 12 (6 s0 listener + 6
 * snapshots), 57 in total.  The ord-8 figure is the one place where this oracle
 * deliberately departs from the frozen contract's header arithmetic: the
 * contract's §4.3 row table lists the same six snapshots this oracle replays,
 * but its summary line (and the slice total of 58) counts seven.  The only
 * assertPropsPerRowIterator call sites of the execution are J:872, J:879,
 * J:886, J:898, J:904 and J:913 (six), and the source sends five events (E1,
 * SupportBean_A E1, E2, E3, SupportBean_A E3) while advancing the clock seven
 * times (0, 1000, 2000, 3000, 3000, 12999, 13000), so the execution is 24
 * scenario steps and 12 records.  The 58-record total is corrected to 57.
 *
 * <p>Dispatch order, pinned by this oracle and reported per contract section 4:
 * every wave of ord 4 delivers the window statement's own listener FIRST and
 * then the consume consumer (the window statement registers its consumer view
 * at window creation, before consume is deployed).  Ord 4/8 expire strictly
 * older-than rows at the exact boundary: G1 (ts 0) expires when the clock
 * reaches 10000, G4 (ts 26000) at 36000, and the silent 35000 reschedule keeps
 * 35999 quiet; ord 8's E2 (ts 3000) expires at 13000 while 12999 stays quiet.
 * Ord 5's firsttime close is deploy-anchored (deploy at 1000, close at 11000):
 * E3 at exactly 10000 is admitted, E4 at 12000 is dropped without a callback,
 * and the retained rows survive until the explicit delete.
 */
public final class InfraNamedWindowTimeViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-time-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews time-slice: the projection time(10 sec) window with a separately deployed irstream consumer, the firsttime(10 sec) map window and the bean-backed time(10 sec) window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "time-window-scene-two",
            "time-first-window",
            "ext-time-window-scene-three"
    };
    private static final int[] ORDINALS = {4, 5, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-879d6ad8aee378657a02",
            "java-runtime-5f5a65dd5f72c8e24619",
            "java-runtime-a6bfdd3389c077127861"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraTimeWindowSceneTwo",
            "InfraTimeFirstWindow",
            "InfraExtTimeWindowSceneThree"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "projection time(10 sec) window over theString/intBoxed with four module deployments; the window statement and the consume consumer both carry irstream, expiry at t=10000 and t=36000 delivers old-only rows and equal-time advances stay silent",
            "firsttime(10 sec) window over the key/value map schema anchored at deploy time (clock 1000): it closes silently at 11000, later sends are dropped while retained rows survive until deleted",
            "bean-backed win:time(10 sec) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; expiry at the 13000 boundary releases the retained row"
    };

    // Transcriptions of InfraNamedWindowViews lines 534/540/546/552
    // (InfraTimeWindowSceneTwo), deployed as four separate modules on one
    // shared RegressionPath exactly like the source's four compileDeploy calls.
    // The window create carries @public because the insert, consume and delete
    // modules are separate deployments resolving MyWindow by name; the consume
    // statement is the separately deployed irstream consumer this slice traces
    // in addition to the window statement itself.  The delete module's "s0" is
    // a stream alias, unrelated to the ord-5 statement named "s0".
    private static final String EPL_T2_CREATE =
            "@name('create') @public create window MyWindow#time(10 sec)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_T2_INSERT =
            "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed"
                    + " from SupportBean";
    private static final String EPL_T2_CONSUME =
            "@name('consume') select irstream key, value as value from MyWindow";
    private static final String EPL_T2_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of the InfraTimeFirstWindow EPL literal (lines 656-659),
    // deployed as ONE module exactly like the source's single compileDeploy:
    // joining the four pinned step EPLs with ";\n" reproduces the literal minus
    // its trailing terminator.  The window is not @public (all four statements
    // are in the same module).  firsttime(10 sec) anchors its close at the
    // deployment clock (1000), so the scenario must advance to
    // 1970-01-01T00:00:01Z BEFORE the deploy steps.
    private static final String EPL_TFW_CREATE =
            "@name('create') create window MyWindowTFW#firsttime(10 sec) as MySimpleKeyValueMap";
    private static final String EPL_TFW_INSERT =
            "insert into MyWindowTFW select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TFW_S0 =
            "@name('s0') select irstream key, value as value from MyWindowTFW";
    private static final String EPL_TFW_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTFW as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 865-868
    // (InfraExtTimeWindowSceneThree), deployed as four separate modules in the
    // source's compileDeploy order create -> insert -> s0 -> delete (which
    // differs from the length-window twin's create/insert/delete/s0 order).
    // The window create is @public because the following modules are separate
    // deployments; the consumer carries the capital @Name('s0') spelling
    // verbatim.  Despite the class name the window is win:time, not ext_timed:
    // retention is engine-clock driven exactly like ord 4.
    private static final String EPL_ABC_CREATE =
            "@public create window ABCWin.win:time(10 sec) as SupportBean";
    private static final String EPL_ABC_INSERT =
            "insert into ABCWin select * from SupportBean";
    private static final String EPL_ABC_S0 =
            "@Name('s0') select irstream * from ABCWin";
    private static final String EPL_ABC_DELETE =
            "on SupportBean_A delete from ABCWin where theString = id";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_T2_CREATE, EPL_TFW_CREATE, EPL_ABC_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_T2_INSERT, EPL_TFW_INSERT, EPL_ABC_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            "", EPL_TFW_S0, EPL_ABC_S0
    };
    private static final String[] CASE_CONSUME_EPLS = {
            EPL_T2_CONSUME, "", ""
    };
    private static final String[] CASE_DELETE_EPLS = {
            EPL_T2_DELETE, EPL_TFW_DELETE, EPL_ABC_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "consume", "delete"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "s0", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "consume"},
            {"create", "s0"},
            {"s0"}
    };

    /**
     * Iterator-snapshot count per case, read off the execution's
     * assertPropsPerRowIterator call sites: ord 4 asserts ten iterator states
     * (J:566, J:573, J:576, J:581, J:604, J:607, J:611, J:620, J:623, J:627),
     * ord 5 seven (J:661, J:666, J:672, J:678, J:684, J:689, J:695) and ord 8
     * six (J:872, J:879, J:886, J:898, J:904, J:913 - see the class comment on
     * the corrected ord-8 count).
     */
    private static final int[] CASE_SNAPSHOTS = {10, 7, 6};

    /**
     * Module grouping per deploy position: ord 4 and ord 8 deploy four separate
     * modules (one statement each) on one shared path, ord 5 compiles all four
     * statements as one module (0,0,0,0).  See the class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 1, 2, 3},
            {0, 0, 0, 0},
            {0, 1, 2, 3}
    };

    /**
     * Listener discipline: ord 4 listens to create and consume (J:535/J:547),
     * ord 5 to create and s0 (J:660), ord 8 to s0 only (J:869).  The
     * never-asserted delete listener of ords 4/5 is attached in the suite but
     * excluded here; see the class comment.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: ords 4 and 5 assert {@code fields = {"key", "value"}}
     * (J:531/J:652) and ord 8 asserts {@code {"theString"}} (J:861).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {"theString"}
    };

    private static final int[] EXPECTED_CASE_RECORDS = {30, 15, 12};
    private static final int EXPECTED_RECORDS = 57;
    private static final int EXPECTED_STEPS = 80;

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

    private InfraNamedWindowTimeViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowTimeViewsScenarioOracle <scenario.json>");
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

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            int before = records.size();
            runCaseOnFreshRuntime(CASE_NAMES[index], CASE_FIELDS[index], configuration, allSteps,
                    records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASE_NAMES[index] + " emitted "
                        + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
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
     * (TestSuiteInfraNamedWindow lines 136-139): key string, value long.  Ord 5
     * inserts the Long projection of longBoxed, mirroring the {@code 1L}
     * literals of its assertions.
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
     * Map event type for the ord-8 delete trigger: SupportBean_A carries the
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
     * Replays one case on its own runtime.  A fresh EPRuntime per case is
     * mandatory for this slice: the pinned timelines are not monotonic in
     * sequence, and the scheduling service assigns the clock unconditionally.
     */
    private static void runCaseOnFreshRuntime(String caseName, String[] fields,
                                              Configuration configuration, JsonArray allSteps,
                                              JsonArray records)
            throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        try {
            runCase(caseName, fields, configuration, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Replays one case's steps on its runtime; sequences restart per case.
     * Compiles with CompilerArguments(configuration) plus the runtime path (the
     * sibling-oracle convention): the configuration makes the base event types
     * resolvable to the compiler, and the runtime path carries the prior
     * deployments' public types so the later ord-4/ord-8 modules' insert-into,
     * consumer and on-delete statements can reference the window created by the
     * first module.
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
                case "advance-time":
                    // Exactly one advanceTime per step, mirroring sendTimer in
                    // the suite (RegressionEnvironmentBase.advanceTime); the
                    // scenario pins the absolute virtual times byte-exactly.
                    runtime.getEventService().advanceTime(
                            Instant.parse(string(step, "at")).toEpochMilli());
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
        // EPL text: the insertion statement of the ord-5 module
        // (InfraNamedWindowViews line 657), the create/insert/delete statements
        // of the ord-4 modules (lines 534/540/552) and the create/insert/delete
        // statements of the ord-8 modules (lines 865/866/868) are unnamed in
        // the Java source, so the engine assigns them generated names; the
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
     * invocation that carries neither stream is a contract violation.  The
     * record time is the virtual clock at delivery.
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
     * Snapshot of the statement iterator at the current virtual clock: one
     * record with sequence 0 holding either the engine iterator order (ordered
     * steps, the order the suite's exact-order iterator assertions pin) or, for
     * any-mode steps, the rows sorted canonically by serialized fields.  Every
     * step of this slice is ordered, so the any-mode branch is the sibling
     * oracle's shared protocol code and is unreached here.  An empty iterator
     * omits the new array, matching the Go normalizer's omitempty rendering and
     * the suite's {@code null} / {@code new Object[0][]} iterator assertions.
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

    /** Projected row rendering projected to exactly the fields the Java assertions read. */
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
        int offset = validateTimeWindowSceneTwo(steps, 0);
        offset = validateTimeFirstWindow(steps, offset);
        offset = validateExtTimeWindowSceneThree(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraTimeWindowSceneTwo step sequence mirroring lines 534-633: the
     * four module deploys (create, insert, consume, delete - one step per
     * statement), the leading advanceTime(0) and eight further clock moves with
     * the eight bean/market sends and all ten ordered create iterator snapshots
     * the source asserts between them, and the case-end undeploy-all that
     * mirrors the source teardown.  The two 25000 advances are the pinned
     * equal-time pair (J:587 and J:593); 26000 and 27000 carry the G4 and G5
     * sends; 35999 stays silent while 36000 expires G4.
     */
    private static int validateTimeWindowSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_T2_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_T2_INSERT);
        validateDeploy(steps.get(offset++), caseName, "consume", EPL_T2_CONSUME);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_T2_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateIntSend(steps.get(offset++), caseName, "G1", 10);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 20);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:25Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:25Z");
        validateIntSend(steps.get(offset++), caseName, "G3", 30);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:26Z");
        validateIntSend(steps.get(offset++), caseName, "G4", 40);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:27Z");
        validateIntSend(steps.get(offset++), caseName, "G5", 50);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G3");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:35.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:36Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G5");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeFirstWindow step sequence mirroring lines 654-702: the
     * clock move to 1000 BEFORE the four deploy steps (the source's sendTimer
     * precedes the single compileDeploy and anchors the firsttime close at
     * 11000), the four module deploy steps compiled as one module, the leading
     * empty create iterator snapshot, the four SupportBean sends plus the
     * market delete with all seven ordered create iterator snapshots the source
     * asserts between them (E4 is dropped after the close and produces no
     * record), and the case-end undeploy-all that mirrors the source teardown.
     */
    private static int validateTimeFirstWindow(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TFW_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TFW_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TFW_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TFW_DELETE);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:12Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:01:40Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraExtTimeWindowSceneThree step sequence mirroring lines 864-915:
     * the leading advanceTime(0) and the four module deploys in the source's
     * create -> insert -> s0 -> delete order, the four SupportBean sends and two
     * SupportBean_A deletes with the six ordered s0 iterator snapshots the
     * source asserts between them (three of them empty), the two 3000 advances
     * (the pinned equal-time pair, J:888 and J:892), the silent 12999 move and
     * the 13000 expiry, and the case-end undeploy-all that mirrors the source
     * teardown.
     */
    private static int validateExtTimeWindowSceneThree(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), caseName, "create", EPL_ABC_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ABC_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_ABC_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_ABC_DELETE);
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateBeanSend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:02Z");
        validateSupportBeanASend(steps.get(offset++), caseName, "E1");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:03Z");
        validateBeanSend(steps.get(offset++), caseName, "E2");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:03Z");
        validateBeanSend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateSupportBeanASend(steps.get(offset++), caseName, "E3");
        validateSnapshot(steps.get(offset++), caseName, "s0");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:12.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:13Z");
        validateSnapshot(steps.get(offset++), caseName, "s0");
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
     * Ord 4 sends theString plus intBoxed, the only value field its projection
     * window reads; every other SupportBean property stays at its Java default.
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

    /** Ord 5 sends theString plus longBoxed, the only value field its window projects. */
    private static void validateLongSend(JsonValue value, String caseName, String expectedString,
                                         long expectedLongBoxed) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "longBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("longBoxed"), "longBoxed") != expectedLongBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ord 8's bean-typed window reads theString only (sendSupportBean(env, theString)). */
    private static void validateBeanSend(JsonValue value, String caseName, String expectedString) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString");
        if (!expectedString.equals(string(payload, "theString"))) {
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

    /** Ord 8's on-delete trigger sends new SupportBean_A(id) with the single id property. */
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

    /**
     * Every clock move is pinned byte-exactly as the RFC3339 rendering of the
     * source's advanceTime argument, one step per advanceTime call.
     */
    private static void validateAdvanceTime(JsonValue value, String caseName, String expectedAt) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "case", "at");
        if (!"advance-time".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedAt.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step is not pinned for " + caseName
                    + " at " + expectedAt);
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
