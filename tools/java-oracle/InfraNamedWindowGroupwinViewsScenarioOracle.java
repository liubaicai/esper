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
 * Java oracle for the infra-named-window-groupwin-views slice of
 * InfraNamedWindowViews.java: the ord-26 {@code InfraLengthWindowPerGroup}
 * {@code MyWindowWPG#groupwin(value)#length(2)} window over the
 * MySimpleKeyValueMap schema (lines 2506-2579), the ord-27
 * {@code InfraTimeBatchPerGroup}
 * {@code MyWindowTBPG#groupwin(value)#time_batch(10 sec)} window over the same
 * schema (lines 2581-2616), the ord-44 {@code InfraSelectGroupedViewLateStart}
 * legacy grouped-window consumer started after the window was filled
 * (lines 2997-3044) and the ord-55
 * {@code InfraSelectGroupedViewLateStartVariableIterate} variant whose having
 * clause reads a variable set by an on-set trigger (lines 3046-3118).
 *
 * <p>Only the ord-27 execution moves the engine clock: the scenario carries
 * {@code advance-time} steps for its three {@code sendTimer} calls (0 before
 * the deploy, 1000 before the four arrivals and 11000 for the flush), and the
 * oracle replays them through {@code EPRuntime.getEventService().advanceTime}
 * exactly as the suite's helper does ({@code sendTimer(env,ms)} is literally
 * {@code env.advanceTime(ms)}, INV.java:3618-3620; the internal timer is
 * disabled, so schedules fire only on those calls).  {@code EPEventServiceImpl
 * .advanceTime} performs {@code setTime(time)} then {@code processSchedule
 * (time)}, so the per-group time_batch callbacks fire at the pinned 11000
 * instant.  The other three executions never move the clock, so their records
 * carry the pinned start instant; one EPRuntime is created per case with the
 * clock pinned at 0 before any statement is deployed, which keeps those
 * instants deterministic instead of inheriting the wall clock or the previous
 * case's position ({@code SchedulingServiceImpl.setTime} assigns
 * unconditionally and the timelines restart at 0).
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 26 compiles its four statements (create, insert,
 * s0, delete; INV:2510-2514), ord 27 its three (create, insert, s0;
 * INV:2586-2589) and ord 44 its first two (create, insert; INV:3001-3003) as
 * one module each, ord 44 deploys its {@code s0} consumer as a second module
 * mid-timeline (INV:3021-3022, after the twelve fills) and ord 55 deploys five
 * single-statement modules on one shared path (create INV:3051-3052, insert
 * INV:3055-3056, the variable INV:3059, the on-set trigger INV:3060 and the
 * late {@code s0} INV:3083-3085).  Joining the pinned step EPLs of one module
 * with ";\n" reproduces the source literal minus its trailing terminator.  The
 * window creates of ords 44 and 55 carry {@code @public} because the later
 * modules resolve the window (and, for ord 55, the variable) through the
 * runtime path; the create statements of ords 26 and 27 deliberately carry
 * none, exactly like the source.
 *
 * <p>Listener set: create and s0 for ord 26 (INV:2514) and ord 27 (INV:2589);
 * ords 44 and 55 deploy no listener at all, so their traces contain only
 * snapshot records.  The on-delete statement of ord 26 is attached in the suite
 * but never asserted and therefore stays excluded, exactly like the sibling
 * slices; its effect is observed through the old-only rows of the create and s0
 * listeners and through the ordered window snapshots.
 *
 * <p>Snapshot steps replay the execution's iterator assertions in source order
 * and carry their own projection list.  Ord 26 asserts the window content twice
 * through {@code assertPropsPerRowIterator} (INV:2527 with three rows and
 * INV:2533 with two rows, both exact-order, fields key/value); ord 27 asserts
 * no iterator at all.  Ord 44 asserts the window content once as a bare count
 * ({@code assertIterator("create", …)} with {@code received.length == 12},
 * INV:3015-3018) and the consumer once with an explicit count plus ordered rows
 * (INV:3023-3039: ten rows, fields theString/intPrimitive/count(*)); ord 55
 * asserts the window content as a bare count of ten (INV:3077-3080) and the
 * consumer twice with a count of three plus ordered rows (INV:3091-3100 and
 * INV:3105-3114, fields theString/intPrimitive/avgLong/cntBool).  The two
 * count-only sites are encoded by the smallest honest protocol extension the
 * primary pinned: a snapshot step whose {@code fields} array is EMPTY and whose
 * mode is {@code "any"}.  The oracle then emits one record per window row with
 * an empty field map, so the row count is compared while the order Java leaves
 * unasserted stays unconstrained; no row Java does not assert is invented and
 * no asserted row is dropped.  All other snapshots keep mode {@code "ordered"}
 * and their exact field list.
 *
 * <p>Record counts are pinned to the Java source: ord 26 emits 22 records (20
 * listener callbacks + 2 snapshots), ord 27 emits 2 (one 4-row batch callback
 * per listener), ord 44 emits 2 (both snapshots, no listeners) and ord 55 emits
 * 3 (all snapshots, no listeners), 29 in total over 72 scenario steps.  The
 * snapshot counts are 2/0/2/3 in case order.
 *
 * <p>Semantics this oracle observes and the Go side must reproduce:
 * {@code groupwin(value)#length(2)} keeps two rows per group, so the third
 * arrival of a group expels that group's oldest row in the SAME callback that
 * carries the new row (ord 26's E5 and E8 waves), a delete removes its row
 * without expiring anything and is delivered old-only to the irstream
 * listeners, and the window iterator walks the groups in creation order and the
 * rows in insertion order within a group (g1=[E1,E2] before g2=[E3] at
 * INV:2527).  {@code groupwin(value)#time_batch(10 sec)} arms one boundary per
 * group at that group's first arrival, so ord 27's four arrivals at t=1000 all
 * flush at t=11000 in ONE callback per listener carrying the group batches
 * concatenated in group-creation order, [E1,E4] then [E2,E3] (INV:2598-2612).
 * The late-started grouped consumers of ords 44 and 55 preload the window
 * snapshot when they are deployed (INV:3003/3022 and INV:3052/3085), so their
 * aggregation state covers the rows that arrived before the deployment; the
 * ord-55 {@code having theString = var_1_1_1} clause is evaluated at iterate
 * time against the variable's current value, which is why the same consumer
 * reports the c0 groups, then the c1 groups after the on-set trigger repoints
 * the variable (INV:3088/3103).  The window of ords 44/55 keeps nine rows per
 * group and therefore all twelve and ten events respectively.
 */
public final class InfraNamedWindowGroupwinViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-groupwin-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews groupwin-slice: the per-group #groupwin(value)#length(2) map window whose iterator walks group-creation order and whose delete removes without expiring, the per-group #groupwin(value)#time_batch(10 sec) map window whose boundary flush concatenates the group batches as [E1,E4,E2,E3], and the #groupwin(theString, intPrimitive)#length(9) projection windows whose late grouped count(*)/avg/count consumers preload the populated window, the second one having-filtered on a runtime variable (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "length-per-group",
            "time-batch-per-group",
            "select-grouped-view-late-start",
            "select-grouped-view-late-start-variable-iterate"
    };
    private static final int[] ORDINALS = {26, 27, 44, 55};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-083a289ee5f82dd87ad3",
            "java-runtime-accaf82c3c846493832b",
            "java-runtime-3326973240f20b92dced",
            "java-runtime-0ec49d4098c9796540b0"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraLengthWindowPerGroup",
            "InfraTimeBatchPerGroup",
            "InfraSelectGroupedViewLateStart",
            "InfraSelectGroupedViewLateStartVariableIterate"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "per-group #groupwin(value)#length(2) window over the key/value map schema: each group keeps its newest two rows, an over-full group expels its oldest row as old data in the same delta as the triggering insert, a delete removes without expiring, and the iterator walks group-creation order",
            "per-group #groupwin(value)#time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, each group anchors its boundary at its first arrival, and the 11000 flush concatenates the completed group batches as [E1,E4,E2,E3] for both the window stream and the plain consumer",
            "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose count(*) consumer is deployed after the window is populated: the late preload builds the per-group counts, the window iterator keeps the twelve rows of the ten groups, and the consumer iterates one sorted row per group",
            "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose variable-filtered avg/count consumer is deployed after the window is populated: the late preload builds the group state including both rows of the non-uniform (c1,1) group, and the having clause reads the runtime variable at iterate time so the on-set trigger switches the visible theString group"
    };

    // Transcriptions of InfraNamedWindowViews lines 2510-2513
    // (InfraLengthWindowPerGroup), deployed as ONE module exactly like the
    // source's single compileDeploy: joining the four pinned step EPLs with
    // ";\n" reproduces the literal minus its trailing terminator.  The window
    // create carries no @public (the source omits it); the delete trigger has
    // no stream alias because it compares the market symbol with the window's
    // own key column.
    private static final String EPL_LPG_CREATE =
            "@name('create') create window MyWindowWPG#groupwin(value)#length(2)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_LPG_INSERT =
            "insert into MyWindowWPG select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LPG_SELECT =
            "@name('s0') select irstream key, value as value from MyWindowWPG";
    private static final String EPL_LPG_DELETE =
            "@name('delete') on SupportMarketDataBean delete from MyWindowWPG where symbol = key";

    // Transcriptions of InfraNamedWindowViews lines 2586-2588
    // (InfraTimeBatchPerGroup), deployed as ONE module exactly like the
    // source's single compileDeploy.  The s0 select is istream-only, so the
    // first flush's old data (empty by construction) never reaches it.
    private static final String EPL_TBPG_CREATE =
            "@name('create') create window MyWindowTBPG#groupwin(value)#time_batch(10 sec)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_TBPG_INSERT =
            "insert into MyWindowTBPG select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TBPG_SELECT =
            "@name('s0') select key, value as value from MyWindowTBPG";

    // Transcriptions of InfraNamedWindowViews lines 3001-3002 and 3021
    // (InfraSelectGroupedViewLateStart): the create and insert statements form
    // the first module, the grouped count consumer is a second, separately
    // deployed module that the scenario replays MID-TIMELINE at its source
    // position (after the twelve fills and the window snapshot, before the
    // consumer snapshot).  The window create is @public because the consumer
    // resolves the window by name; the consumer's order-by props are all
    // select props, so the engine compiles a RowPerGroup result set.
    private static final String EPL_SGVS_CREATE =
            "@name('create') @public create window MyWindowSGVS#groupwin(theString, intPrimitive)"
                    + "#length(9) as select theString, intPrimitive from SupportBean";
    private static final String EPL_SGVS_INSERT =
            "@name('insert') insert into MyWindowSGVS select theString, intPrimitive from SupportBean";
    private static final String EPL_SGVS_SELECT =
            "@name('s0') select theString, intPrimitive, count(*) from MyWindowSGVS"
                    + " group by theString, intPrimitive order by theString, intPrimitive";

    // Transcriptions of InfraNamedWindowViews lines 3051, 3055, 3059, 3060 and
    // 3083-3084 (InfraSelectGroupedViewLateStartVariableIterate), deployed as
    // FIVE single-statement modules on one shared path exactly like the
    // source's five compileDeploy calls.  The insert statement is unnamed and
    // its scenario step id is the module position; the variable and the on-set
    // trigger are the "var" and "on-set" steps.  The window and the variable
    // carry @public because the late consumer resolves both by name, and the
    // having clause reads the variable at iterate time.
    private static final String EPL_SGVLS_CREATE =
            "@name('create') @public create window MyWindowSGVLS#groupwin(theString, intPrimitive)"
                    + "#length(9) as select theString, intPrimitive, longPrimitive, boolPrimitive"
                    + " from SupportBean";
    private static final String EPL_SGVLS_INSERT =
            "insert into MyWindowSGVLS select theString, intPrimitive, longPrimitive, boolPrimitive"
                    + " from SupportBean";
    private static final String EPL_SGVLS_VAR =
            "@public create variable string var_1_1_1";
    private static final String EPL_SGVLS_ONSET =
            "on SupportVariableSetEvent(variableName='var_1_1_1') set var_1_1_1 = value";
    private static final String EPL_SGVLS_SELECT =
            "@name('s0') select theString, intPrimitive, avg(longPrimitive) as avgLong,"
                    + " count(boolPrimitive) as cntBool from MyWindowSGVLS"
                    + " group by theString, intPrimitive having theString = var_1_1_1"
                    + " order by theString, intPrimitive";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_LPG_CREATE, EPL_TBPG_CREATE, EPL_SGVS_CREATE, EPL_SGVLS_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_LPG_INSERT, EPL_TBPG_INSERT, EPL_SGVS_INSERT, EPL_SGVLS_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_LPG_SELECT, EPL_TBPG_SELECT, EPL_SGVS_SELECT, EPL_SGVLS_SELECT
    };
    private static final String[] CASE_CONSUME_EPLS = {"", "", "", ""};
    private static final String[] CASE_DELETE_EPLS = {
            EPL_LPG_DELETE, "", "", ""
    };
    private static final String[] CASE_VAR_EPLS = {"", "", "", EPL_SGVLS_VAR};
    private static final String[] CASE_ONSET_EPLS = {"", "", "", EPL_SGVLS_ONSET};
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "s0"},
            {"create", "insert", "s0"},
            {"create", "insert", "var", "on-set", "s0"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"create", "s0"},
            {},
            {}
    };

    /**
     * Snapshot-step count per case: ord 26 asserts the window content twice
     * (INV:2527, INV:2533), ord 27 never asserts an iterator, ord 44 asserts
     * the window content once as a bare count and the consumer once with rows
     * (INV:3015-3018, INV:3023-3039) and ord 55 asserts the window content
     * once as a bare count and the consumer twice with rows (INV:3077-3080,
     * INV:3091-3100, INV:3105-3114).
     */
    private static final int[] CASE_SNAPSHOTS = {2, 0, 2, 3};

    /**
     * Module grouping per deploy position: ords 26 and 27 compile all their
     * statements as one module (all-zero keys), ord 44 deploys create+insert as
     * module 0 and its late consumer as module 1, and ord 55 deploys five
     * single-statement modules 0..4.  See the class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 0, 0},
            {0, 0, 1},
            {0, 1, 2, 3, 4}
    };

    /**
     * Listener discipline: ords 26 and 27 listen to create and s0
     * (INV:2514/INV:2589); ords 44 and 55 deploy no listener at all.  Ord 26's
     * never-asserted delete listener is excluded; see the class comment.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Listener row projection: ords 26 and 27 assert {@code {"key", "value"}}
     * (INV:2508/INV:2583); the two listener-less cases project nothing through
     * listeners, so their entry is empty.  Snapshot projections live on the
     * snapshot steps.
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {},
            {}
    };

    /**
     * Per-case record totals: 20 listener callbacks + 2 snapshots for ord 26,
     * 2 + 0 for ord 27, 0 + 2 for ord 44 and 0 + 3 for ord 55.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {22, 2, 2, 3};
    private static final int EXPECTED_RECORDS = 29;
    private static final int EXPECTED_STEPS = 72;

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

    private InfraNamedWindowGroupwinViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowGroupwinViewsScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportVariableSetEvent", variableSetSchema());
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
     * (TestSuiteInfraNamedWindow lines 136-139): key string, value long, which
     * is also the group key of the two per-group windows.  Ord 26/27 insert the
     * Long projection of longBoxed, mirroring the {@code 1L} literals of their
     * assertions.
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
     * EPL, where ord 26's delete trigger compares it with the window's key
     * column.  Sends mirror new SupportMarketDataBean(symbol, 0, 0L, "").
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
     * Map event type carrying the regression-lib SupportVariableSetEvent read
     * surface (regressionlib/support/bean/SupportVariableSetEvent.java:23-32:
     * a String variableName and a String value); the regression-lib bean is not
     * on the oracle classpath, so ord 55's on-set trigger reads the same two
     * String columns from a map.  Sends mirror
     * new SupportVariableSetEvent("var_1_1_1", "c0").
     */
    private static Map<String, Object> variableSetSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("variableName", String.class);
        schema.put("value", String.class);
        return schema;
    }

    /**
     * Replays one case on its own runtime with the clock pinned at 0.  A fresh
     * EPRuntime per case is mandatory for this slice: only ord 27 moves the
     * clock (0 to 1000 to 11000) while the other three executions never advance
     * it, and the scheduling service assigns the clock unconditionally.  The
     * explicit {@code advanceTime(0)} before any statement is deployed makes
     * that pin a property of the oracle itself rather than of the runtime's
     * initial clock; it cannot fire anything because no statement exists yet,
     * and ord 27's source spells the same pin out as {@code sendTimer(env, 0)}
     * at the head of its timeline.
     */
    private static void runCaseOnFreshRuntime(String caseName, String[] fields,
                                              Configuration configuration, JsonArray allSteps,
                                              JsonArray records)
            throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        try {
            runtime.getEventService().advanceTime(0);
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
     * deployments' public types so ord 44's late consumer and ord 55's insert,
     * variable, on-set and late consumer can resolve the window and the
     * variable created by the first modules.  A pending module is deployed when
     * the next non-deploy step of the same case arrives, which is what lets
     * ord 44's and ord 55's consumers be compiled mid-timeline at their source
     * position (INV:3021-3022 after the twelve fills and the window snapshot;
     * INV:3083-3085 after the window snapshot and before the variable sends).
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
                    records.add(snapshot(runtime, statement, caseName, fieldsOf(step)));
                    break;
                }
                case "advance-time":
                    // Exactly one advanceTime per step, mirroring sendTimer in
                    // the suite (RegressionEnvironmentBase.advanceTime); the
                    // scenario pins the absolute virtual times byte-exactly.
                    runtime.getEventService().advanceTime(
                            Instant.parse(string(step, "at")).toEpochMilli());
                    break;
                case "undeploy": {
                    // Mirrors undeployModuleContaining in the suite: the
                    // deployment holding the statement is removed, so every
                    // statement of that module leaves the registry.
                    String statementName = string(step, "statement");
                    EPStatement statement = statementsByName.get(statementName);
                    if (statement == null) {
                        throw new IllegalStateException("undeploy targets unknown statement "
                                + statementName + " in case " + caseName);
                    }
                    String deploymentId = statement.getDeploymentId();
                    runtime.getDeploymentService().undeploy(deploymentId);
                    statementsByName.values().removeIf(
                            registered -> deploymentId.equals(registered.getDeploymentId()));
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
        // EPL text: the insert statement of every module (lines 2511/2587/3002/
        // 3055) is unnamed in the Java source, together with ord 55's variable
        // and on-set statements (lines 3059/3060), so the engine assigns them
        // generated names; the scenario's step ids of those statements are the
        // module positions and no listen or snapshot step resolves them.  Every
        // statement the source does name (create, s0 and delete) must appear at
        // the pinned position under exactly that name.
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
     * shapes this slice pins are the single new row of an ordinary arrival, the
     * new-plus-old pair of a per-group length expiry (ord 26's E5 and E8),
     * the old-only delta of a delete and the four-row new array of ord 27's
     * per-group time_batch flush, which the engine coalesces into one
     * invocation carrying the group batches concatenated.  The record time is
     * the virtual clock at delivery.
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
     * record with sequence 0 holding the engine iterator order projected onto
     * the step's field list, which is the order the suite's exact-order iterator
     * assertions pin (ord 26's window snapshots and the consumer snapshots of
     * ords 44/55).  A snapshot step whose fields array is EMPTY encodes the
     * suite's count-only iterator assertions (ord 44's window and ord 55's
     * window): every row then renders an empty field map, so the row count is
     * compared while the order Java leaves unasserted stays unconstrained by
     * the step's mode.  No canonical reordering happens here; the mode is
     * carried by the scenario step, not by the record.  An empty iterator omits
     * the new array, matching the Go normalizer's omitempty rendering and the
     * suite's {@code null} iterator assertions.
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
     * The snapshot step's projection list.  An empty array is the count-only
     * encoding: it projects no field, so each row renders {@code {}} and only
     * the row count is pinned.
     */
    private static String[] fieldsOf(JsonObject step) {
        JsonArray fields = array(step.get("fields"), "fields");
        String[] names = new String[fields.size()];
        for (int index = 0; index < fields.size(); index++) {
            JsonValue value = fields.get(index);
            if (!(value instanceof JsonString)) {
                throw new IllegalArgumentException("snapshot fields must be strings");
            }
            names[index] = value.asString();
        }
        return names;
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

    /**
     * Sends one scenario event.  SupportBean carries theString plus whichever
     * of intPrimitive/longBoxed/longPrimitive/boolPrimitive the case's Java
     * send helper sets (ord 26/27 longBoxed, ord 44 intPrimitive, ord 55
     * intPrimitive + longPrimitive + boolPrimitive); the map schemas mirror the
     * regression-lib beans the suite sends.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null) {
                    bean.setIntPrimitive(integer(payload, "intPrimitive"));
                }
                JsonValue intBoxed = payload.get("intBoxed");
                if (intBoxed != null) {
                    bean.setIntBoxed(integer(payload, "intBoxed"));
                }
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null) {
                    bean.setLongBoxed(longInteger(longBoxed, "longBoxed"));
                }
                JsonValue longPrimitive = payload.get("longPrimitive");
                if (longPrimitive != null) {
                    bean.setLongPrimitive(longInteger(longPrimitive, "longPrimitive"));
                }
                JsonValue boolPrimitive = payload.get("boolPrimitive");
                if (boolPrimitive != null) {
                    bean.setBoolPrimitive(bool(payload, "boolPrimitive"));
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
            case "SupportVariableSetEvent": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("variableName", string(payload, "variableName"));
                event.put("value", string(payload, "value"));
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
                    "varEpl", "onSetEpl", "deploys", "listened", "iteratorSnapshots");
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
                    || !CASE_VAR_EPLS[index].equals(string(definition, "varEpl"))
                    || !CASE_ONSET_EPLS[index].equals(string(definition, "onSetEpl"))
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
        int offset = validateLengthPerGroup(steps, 0);
        offset = validateTimeBatchPerGroup(steps, offset);
        offset = validateSelectGroupedViewLateStart(steps, offset);
        offset = validateSelectGroupedViewLateStartVariableIterate(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }


    /**
     * Exact InfraLengthWindowPerGroup step sequence mirroring lines 2506-2577:
     * the single four-statement module deploy, the eight SupportBean arrivals
     * and the two market deletes, with the two ordered window snapshots of
     * INV:2527 (E1/E2/E3) and INV:2533 (E1/E3) in source position.  The E5 and
     * E8 arrivals are the per-group expiries whose new and old rows the suite
     * reads from one raw listener state.
     */
    private static int validateLengthPerGroup(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LPG_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LPG_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LPG_SELECT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_LPG_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 1L);
        validateLongSend(steps.get(offset++), caseName, "E3", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", "key", "value");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", "key", "value");
        validateLongSend(steps.get(offset++), caseName, "E4", 1L);
        validateLongSend(steps.get(offset++), caseName, "E5", 1L);
        validateLongSend(steps.get(offset++), caseName, "E6", 2L);
        validateMarketSend(steps.get(offset++), caseName, "E6");
        validateLongSend(steps.get(offset++), caseName, "E7", 2L);
        validateLongSend(steps.get(offset++), caseName, "E8", 2L);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeBatchPerGroup step sequence mirroring lines 2585-2614: the
     * leading sendTimer(0) clock pin before the deploy, the three-statement
     * module, the move to 1000 with the four arrivals (E1/E4 in group 10L,
     * E2/E3 in group 20L) and the 11000 move whose per-group flushes deliver
     * one concatenated four-row batch per listener.
     */
    private static int validateTimeBatchPerGroup(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TBPG_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TBPG_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TBPG_SELECT);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 10L);
        validateLongSend(steps.get(offset++), caseName, "E2", 20L);
        validateLongSend(steps.get(offset++), caseName, "E3", 20L);
        validateLongSend(steps.get(offset++), caseName, "E4", 10L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraSelectGroupedViewLateStart step sequence mirroring lines
     * 3001-3042: the two-statement first module, the twelve fills, the
     * count-only window snapshot of INV:3015-3018 (empty projection, mode any),
     * the MID-TIMELINE deploy of the grouped count consumer at its source
     * position (INV:3021-3022), its ordered ten-row snapshot and the two
     * module-scoped undeploys of INV:3041-3042.
     */
    private static int validateSelectGroupedViewLateStart(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SGVS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SGVS_INSERT);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c0", 0);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c0", 1);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c0", 2);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c1", 0);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c1", 1);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c1", 2);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c2", 0);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c2", 1);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c2", 2);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c0", 1);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c1", 2);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "c3", 3);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SGVS_SELECT);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered",
                "theString", "intPrimitive", "count(*)");
        validateUndeploy(steps.get(offset++), caseName, "s0");
        validateUndeploy(steps.get(offset++), caseName, "create");
        return offset;
    }

    /**
     * Exact InfraSelectGroupedViewLateStartVariableIterate step sequence
     * mirroring lines 3051-3116: the five single-statement modules, the nine
     * uniform fills plus the non-uniform (c1,1) fill, the count-only window
     * snapshot of INV:3077-3080 (empty projection, mode any), the MID-TIMELINE
     * deploy of the grouped avg/count consumer, the variable set to c0 with its
     * ordered three-row snapshot, the variable repointed to c1 with its second
     * ordered three-row snapshot and the final undeployAll.
     */
    private static int validateSelectGroupedViewLateStartVariableIterate(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SGVLS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SGVLS_INSERT);
        validateDeploy(steps.get(offset++), caseName, "var", EPL_SGVLS_VAR);
        validateDeploy(steps.get(offset++), caseName, "on-set", EPL_SGVLS_ONSET);
        validateVariableBeanSend(steps.get(offset++), caseName, "c0", 0, 0L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c0", 1, 1L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c0", 2, 2L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c1", 0, 0L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c1", 1, 1L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c1", 2, 2L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c2", 0, 0L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c2", 1, 1L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c2", 2, 2L, true);
        validateVariableBeanSend(steps.get(offset++), caseName, "c1", 1, 10L, true);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SGVLS_SELECT);
        validateVariableSetSend(steps.get(offset++), caseName, "var_1_1_1", "c0");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered",
                "theString", "intPrimitive", "avgLong", "cntBool");
        validateVariableSetSend(steps.get(offset++), caseName, "var_1_1_1", "c1");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered",
                "theString", "intPrimitive", "avgLong", "cntBool");
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
     * Ord 44 sends theString plus intPrimitive (the SupportBean(String,int)
     * constructor); every other SupportBean property stays at its Java default.
     */
    private static void validateIntPrimitiveSend(JsonValue value, String caseName,
                                                 String expectedString, int expectedIntPrimitive) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /**
     * Ord 55 sends theString plus intPrimitive, longPrimitive and boolPrimitive
     * (the SupportBean(String,int) constructor followed by setLongPrimitive and
     * setBoolPrimitive); every unset property stays at its Java default.
     */
    private static void validateVariableBeanSend(JsonValue value, String caseName,
                                                 String expectedString, int expectedIntPrimitive,
                                                 long expectedLongPrimitive,
                                                 boolean expectedBoolPrimitive) {
        JsonObject payload = validateBeanStep(value, caseName);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive", "boolPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedIntPrimitive
                || longInteger(payload.get("longPrimitive"), "longPrimitive")
                        != expectedLongPrimitive
                || bool(payload, "boolPrimitive") != expectedBoolPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Ords 26 and 27 send theString plus longBoxed, the value their windows read. */
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

    /** Ord 55's on-set trigger is fired by SupportVariableSetEvent(variableName, value). */
    private static void validateVariableSetSend(JsonValue value, String caseName,
                                                String expectedName, String expectedValue) {
        JsonObject step = object(value, "SupportVariableSetEvent step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportVariableSetEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportVariableSetEvent step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportVariableSetEvent payload");
        requireFields(payload, "variableName", "value");
        if (!expectedName.equals(string(payload, "variableName"))
                || !expectedValue.equals(string(payload, "value"))) {
            throw new IllegalArgumentException("SupportVariableSetEvent payload is not pinned for "
                    + caseName);
        }
    }

    /**
     * Every clock move is pinned byte-exactly as the RFC3339 rendering of the
     * source's advanceTime argument, one step per advanceTime call.  The
     * advance moves the clock before the runtime processes the schedule, so the
     * expiry waves this slice observes fire at the pinned boundary instant.
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

    /**
     * Every snapshot step pins its statement, its mode and its projection list.
     * The ordered snapshots carry the exact fields their iterator assertions
     * read; the count-only snapshots of ords 44/55 carry an EMPTY field list
     * with mode "any", which is the pinned encoding of the suite's bare length
     * assertions (the row count is compared, the order Java leaves unasserted
     * is not).
     */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedMode,
                                         String... expectedFields) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), expectedFields,
                "snapshot fields for " + caseName + "/" + expectedStatement);
    }

    /** Ord 44's module-scoped undeploys (undeployModuleContaining in the suite). */
    private static void validateUndeploy(JsonValue value, String caseName, String expectedStatement) {
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

    private static boolean bool(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || (!value.toString().equals("true") && !value.toString().equals("false"))) {
            throw new IllegalArgumentException(name + " must be a JSON boolean");
        }
        return value.toString().equals("true");
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
