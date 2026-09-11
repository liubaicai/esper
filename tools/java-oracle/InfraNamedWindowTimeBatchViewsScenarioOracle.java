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
 * Java oracle for the infra-named-window-time-batch-views slice (S8) of
 * InfraNamedWindowViews.java: the ord-16 {@code InfraTimeBatch}
 * {@code MyWindowTB#time_batch(10 sec)} window over the MySimpleKeyValueMap
 * schema (lines 1589-1654), the ord-17 {@code InfraTimeBatchSceneTwo} legacy
 * {@code MyWindow.win:time_batch(10)} projection window over
 * theString/intBoxed with three module deployments (lines 1656-1790), the
 * ord-18 {@code InfraTimeBatchLateConsumer} map window whose aggregate consumer
 * is deployed mid-timeline (lines 1792-1821), the ord-23
 * {@code InfraTimeLengthBatch} {@code MyWindowTLB#time_length_batch(10 sec, 3)}
 * window over the same map schema (lines 2241-2312) and the ord-24
 * {@code InfraTimeLengthBatchSceneTwo} legacy
 * {@code MyWindow.win:time_length_batch(10 sec, 4)} projection window over
 * theString/intBoxed with three module deployments (lines 2314-2457).
 *
 * <p>Every execution of this slice moves the engine clock and the batch
 * flushes fire inside {@code advanceTime} with no event sent: the scenario
 * therefore carries {@code advance-time} steps, and the oracle replays them
 * through {@code EPRuntime.getEventService().advanceTime} exactly as the
 * suite's {@code sendTimer} helper does ({@code sendTimer(env,ms)} is literally
 * {@code env.advanceTime(ms)}, INV.java:3618-3620; the internal timer is
 * disabled, so schedules fire only on those calls).  {@code EPEventServiceImpl}
 * performs {@code setTime(time)} then {@code processSchedule(time)}, so a wave
 * fires at the pinned boundary instant (10999 stays silent, 11000 fires; 24999
 * stays silent, 25000 fires; 34999 stays silent, 35000 fires).  Record
 * {@code time} values are the virtual clock at delivery as rendered by
 * {@code Instant.ofEpochMilli}, which for these cases is the epoch plus the
 * advance offset and never the wall clock.
 *
 * <p>One EPRuntime is created per case, with a distinct runtime URI, for the
 * same reason as the sibling virtual-time slices: {@code SchedulingServiceImpl
 * .setTime} assigns the clock unconditionally with no backwards guard, and the
 * five pinned timelines restart at 0 (ord 18) or 1000 (the others) after the
 * previous case ended at 11000 to 35000.  Each runtime is created with the
 * clock pinned at 0 before any statement is deployed - ord 18's source spells
 * that pin out as {@code sendTimer(env, 0)} (INV:1795) and repeats it at
 * INV:1801, while the other four cases rely on their first absolute move - and
 * each case tears its runtime down with undeployAll + destroy.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ords 16 and 23 compile their four statements (create,
 * insert, s0, delete) as ONE module exactly like the suite's single
 * {@code compileDeploy} (INV:1593-1597/2245-2249), ord 18 compiles module 1
 * (create + insert, INV:1797-1799) and then its late consumer as a second
 * single-statement module (INV:1808-1809), and ords 17 and 24 deploy three
 * single-statement modules on one shared path (ord 17: create, insert, delete;
 * ord 24: create, insert, delete).  Joining the pinned step EPLs of one module
 * with ";\n" reproduces the source literal minus its trailing terminator.  The
 * window creates of the multi-module cases carry {@code @public} because the
 * later modules resolve the window through the runtime path (ord 18 included),
 * and the insert statements of ords 17, 18 and 24 plus the s0 statement of
 * ord 18 are named or unnamed exactly as the source declares them.  The oracle
 * matches each module's statements by position and pins the names the source
 * does declare.  Ord 18's mid-timeline deploy is what makes its flush prove the
 * preload skip: the consumer is attached at clock 5000, after E1 and E2 are
 * already buffered, and the definition of {@code create} is untouched by it.
 *
 * <p>Listener set: create and s0 for ords 16 and 23 (INV:1597/2249), create
 * only for ords 17 and 24 (INV:1663/2321) and s0 only for ord 18 (INV:1809),
 * attached when the module containing the statement completes, exactly like the
 * source's compileDeploy(...).addListener(...) chaining.  The on-delete
 * statement of ords 16, 17, 23 and 24 is attached in the suite but never
 * asserted, so it stays excluded exactly like the sibling slices; deleted rows
 * are observed through the window statement's old rows and the ordered iterator
 * snapshots.  Ord 18 has no create listener at all, yet its single snapshot
 * targets the create statement, so the oracle still registers every deployed
 * statement by its step id.
 *
 * <p>Rows project exactly the fields the Java assertions read:
 * {@code fields = {"key", "value"}} for ords 16, 17, 23 and 24 (ords 16 and 23
 * project longBoxed as Long, ords 17 and 24 project intBoxed as Integer; both
 * render as JSON numbers, so the Go projection must not coerce between them)
 * and {@code {"value"}} for ord 18's null-iterator snapshot of the create
 * statement.  Listener rows render in delivery order and both streams of one
 * invocation land in one record, which is how the batch flush shows up: the
 * boundary callback carries the pending batch as new data and, from the second
 * callback on, the batch emitted at the previous flush as old data in insertion
 * order (ord 16's 11000 new-only and 21000 old-only flushes, ord 17's 21000
 * new-only and 31000 new-plus-old flushes, ord 23's 1000 size-triggered flush
 * and 11000 new-plus-old flush, ord 24's 25000 new-only and 35000 new-plus-old
 * flushes).  A flush whose pending batch and prior batch are both empty emits
 * no callback at all (ord 16 at 31000, ord 17 at 11000, ord 24 at 11000), and a
 * delta carrying only old rows produces no callback for the plain-istream s0
 * consumers (ord 16 at 21000, ord 23's 11000 flush yields the new row only).
 * Dispatch order is pinned by the tail view: the create statement's own
 * listener precedes the separately deployed s0 consumer inside one flush wave.
 *
 * <p>Snapshot steps replay the execution's {@code assertPropsPerRowIterator}
 * call sites in source order and emit the engine iterator order, which is the
 * pending batch only - rows already flushed are invisible even though they come
 * back as old data at the next flush (ord 16's S3/S5, ord 17's S6/S8, ord 23's
 * S4 and ord 24's S8/S11 are the empty states right after a flush).  Every
 * snapshot of this slice is exact-order (the suite never calls the any-order
 * variant), so the scenario pins mode {@code "ordered"} everywhere and the
 * oracle keeps the engine order verbatim.  An empty iterator emits the snapshot
 * record with the {@code new} key omitted, which is how the suite's
 * {@code null} iterator assertions show up: ten of the 31 snapshot records are
 * empty - ord 16 twice (J:1628 after the 11000 flush and J:1646 after the E4
 * delete), ord 17 three times (J:1702, J:1740 and J:1772), ord 18 once (J:1817),
 * ord 23 once (J:2280) and ord 24 three times (J:2360, J:2406 and J:2439).
 *
 * <p>Record counts are pinned to the Java source: ord 16 emits 8 records (3
 * listener callbacks + 5 snapshots), ord 17 emits 10 (2 + 8), ord 18 emits 2
 * (1 + 1), ord 23 emits 10 (4 + 6) and ord 24 emits 13 (2 + 11), 43 in total
 * over 138 scenario steps.  The snapshot counts are the
 * assertPropsPerRowIterator call-site counts of each execution: 5 for ord 16
 * (J:1607, J:1611, J:1628, J:1643, J:1646), 8 for ord 17 (J:1682, J:1689,
 * J:1696, J:1702, J:1723, J:1740, J:1760, J:1772), 1 for ord 18 (J:1817), 6 for
 * ord 23 (J:2256, J:2262, J:2267, J:2280, J:2285, J:2290) and 11 for ord 24
 * (J:2340, J:2347, J:2354, J:2360, J:2373, J:2380, J:2387, J:2406, J:2419,
 * J:2429, J:2439), matching the frozen contract's 5/8/1/6/11 pins.
 *
 * <p>Batch semantics this oracle observes and the Go implementation must
 * reproduce: arrivals buffer silently (no callback, no visibility to
 * consumers), a delete removes the pending row by identity and is fully silent
 * (no callback anywhere, immediate iterator effect, no effect on the prior
 * batch, no timer re-arm), the {@code time_batch} anchor is the first arrival
 * and survives an all-empty flush (ord 17's 15000 arrivals flush at 21000, not
 * 25000), {@code time_length_batch} arms its timer on the first insert and
 * fires the size trigger immediately on the third or fourth arrival
 * (ord 23's E4 at t=1000) before re-arming the boundary from the flush instant,
 * and a late consumer of a batch named window skips its preload so ord 18's
 * first flush reports the sum of the whole batch (1+2+3 = 6L) including the
 * rows that arrived before the consumer existed.
 */
public final class InfraNamedWindowTimeBatchViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-time-batch-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews time-batch-slice: the time_batch(10 sec) map and projection windows with buffered arrivals and boundary flushes, the late aggregate consumer whose preload is skipped, and the time_length_batch(10 sec, 3|4) windows with their size-or-time dual trigger under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "time-batch",
            "time-batch-scene-two",
            "time-batch-late-consumer",
            "time-length-batch",
            "time-length-batch-scene-two"
    };
    private static final int[] ORDINALS = {16, 17, 18, 23, 24};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-1ad42a8ed8025c4a0730",
            "java-runtime-1eb11f5cf275069ef6b5",
            "java-runtime-bb926d092e7110db203f",
            "java-runtime-22bf6b3644a24862df7d",
            "java-runtime-dd924e1b7e500df135f8"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraTimeBatch",
            "InfraTimeBatchSceneTwo",
            "InfraTimeBatchLateConsumer",
            "InfraTimeLengthBatch",
            "InfraTimeLengthBatchSceneTwo"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, deletes remove pending rows without a callback, and boundary flushes deliver the buffered rows as one new-data batch followed by an old-only flush of the previous batch",
            "legacy win:time_batch(10) projection window over theString/intBoxed with three module deployments and only the window statement listened; the anchor stays at the first arrival even across a silent empty flush",
            "time_batch(10 sec) window over the key/value map schema with a late-deployed aggregate consumer whose batch preload is skipped, so the first flush reports the sum of the whole batch including rows that arrived before the consumer existed",
            "time_length_batch(10 sec, 3) window over the key/value map schema: the size trigger flushes immediately on the third arrival and re-arms the boundary, after which deletes stay silent and the time trigger flushes the remaining row with the prior batch as old data",
            "legacy win:time_length_batch(10 sec, 4) projection window over theString/intBoxed with three module deployments and only the window statement listened across the silent-delete, size and time trigger timeline"
    };

    // Transcriptions of InfraNamedWindowViews lines 1593-1596 (InfraTimeBatch),
    // deployed as ONE module exactly like the source's single compileDeploy:
    // joining the four pinned step EPLs with ";\n" reproduces the literal minus
    // its trailing terminator.  The window create deliberately has NO @public
    // (the source omits it) and the s0 select is istream-only, so it sees the
    // new rows of a flush and stays silent on an old-only delta.
    private static final String EPL_TB_CREATE =
            "@name('create') create window MyWindowTB#time_batch(10 sec) as MySimpleKeyValueMap";
    private static final String EPL_TB_INSERT =
            "insert into MyWindowTB select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TB_S0 =
            "@name('s0') select key, value as value from MyWindowTB";
    private static final String EPL_TB_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTB as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 1662/1666/1670
    // (InfraTimeBatchSceneTwo), deployed as three separate modules on one
    // shared RegressionPath exactly like the source's three compileDeploy
    // calls.  The window create carries @public because the insert and delete
    // modules are separate deployments resolving MyWindow by name, and the
    // insert statement is unnamed in the source (its scenario step id is the
    // module position).  The legacy "10" second spelling is kept verbatim; the
    // window statement is the only listener of this case.
    private static final String EPL_TB2_CREATE =
            "@name('create') @public create window MyWindow.win:time_batch(10)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_TB2_INSERT =
            "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean";
    private static final String EPL_TB2_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 1797-1798 and 1808
    // (InfraTimeBatchLateConsumer): module 1 joins the create and insert
    // statements exactly like the source's single compileDeploy, and the
    // aggregate consumer is a second, separately deployed module that the
    // scenario replays MID-TIMELINE at the source position (line 1808, after
    // the 5000 advance and its send, before the 8000 advance).  The create
    // statement is @public because the late consumer resolves the window by
    // name; the sum over the map schema's long value yields a boxed Long.
    private static final String EPL_TBLC_CREATE =
            "@name('create') @public create window MyWindowTBLC#time_batch(10 sec)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_TBLC_INSERT =
            "insert into MyWindowTBLC select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TBLC_S0 =
            "@name('s0') select sum(value) as value from MyWindowTBLC";

    // Transcriptions of InfraNamedWindowViews lines 2245-2248
    // (InfraTimeLengthBatch), deployed as ONE module exactly like the source's
    // single compileDeploy.  The size trigger is 3 and the map schema's value
    // column is not a view timestamp here: time_length_batch takes the interval
    // and the size, no event property.
    private static final String EPL_TLB_CREATE =
            "@name('create') create window MyWindowTLB#time_length_batch(10 sec, 3)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_TLB_INSERT =
            "insert into MyWindowTLB select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TLB_S0 =
            "@name('s0') select key, value as value from MyWindowTLB";
    private static final String EPL_TLB_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTLB as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 2320/2324/2328
    // (InfraTimeLengthBatchSceneTwo), deployed as three separate modules on one
    // shared RegressionPath exactly like the source's three compileDeploy
    // calls.  The window create carries @public because the insert and delete
    // modules resolve MyWindow by name, the insert statement is unnamed in the
    // source and the size trigger is 4, the only execution of this slice whose
    // size trigger never fires (the 25000 and 35000 flushes are time-triggered).
    private static final String EPL_TLB2_CREATE =
            "@name('create') @public create window MyWindow.win:time_length_batch(10 sec, 4)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_TLB2_INSERT =
            "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean";
    private static final String EPL_TLB2_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_TB_CREATE, EPL_TB2_CREATE, EPL_TBLC_CREATE, EPL_TLB_CREATE, EPL_TLB2_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_TB_INSERT, EPL_TB2_INSERT, EPL_TBLC_INSERT, EPL_TLB_INSERT, EPL_TLB2_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_TB_S0, "", EPL_TBLC_S0, EPL_TLB_S0, ""
    };
    private static final String[] CASE_CONSUME_EPLS = {"", "", "", "", ""};
    private static final String[] CASE_DELETE_EPLS = {
            EPL_TB_DELETE, EPL_TB2_DELETE, "", EPL_TLB_DELETE, EPL_TLB2_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete"},
            {"create", "insert", "s0"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"create"},
            {"s0"},
            {"create", "s0"},
            {"create"}
    };

    /**
     * Iterator-snapshot count per case, read off the execution's
     * assertPropsPerRowIterator call sites: ord 16 asserts five iterator states
     * (J:1607, J:1611, J:1628, J:1643, J:1646), ord 17 eight (J:1682, J:1689,
     * J:1696, J:1702, J:1723, J:1740, J:1760, J:1772), ord 18 one (J:1817),
     * ord 23 six (J:2256, J:2262, J:2267, J:2280, J:2285, J:2290) and ord 24
     * eleven (J:2340, J:2347, J:2354, J:2360, J:2373, J:2380, J:2387, J:2406,
     * J:2419, J:2429, J:2439), matching the frozen contract's 5/8/1/6/11 pins.
     * Ten of the 31 states are the empty iterator right after a flush or after
     * a delete emptied the pending batch.
     */
    private static final int[] CASE_SNAPSHOTS = {5, 8, 1, 6, 11};

    /**
     * Module grouping per deploy position: ords 16 and 23 compile all four
     * statements as one module (0,0,0,0), ords 17 and 24 deploy three
     * single-statement modules (0,1,2) and ord 18 deploys module 1 (create and
     * insert, key 0) plus its mid-timeline consumer as module 2 (key 1).  See
     * the class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 1, 2},
            {0, 0, 1},
            {0, 0, 0, 0},
            {0, 1, 2}
    };

    /**
     * Listener discipline: ords 16 and 23 listen to create and s0
     * (J:1597/J:2249), ords 17 and 24 to create only (J:1663/J:2321) and
     * ord 18 to its late s0 consumer only (J:1809).  The never-asserted delete
     * listener of the four delete-bearing cases is excluded; see the class
     * comment.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: ords 16, 17, 23 and 24 assert
     * {@code fields = {"key", "value"}} (J:1591/1658/2243/2316) and ord 18's
     * single snapshot asserts {@code {"value"}} (J:1817).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {"value"},
            {"key", "value"},
            {"key", "value"}
    };

    /**
     * Per-case record totals: 3 listener callbacks + 5 snapshots for ord 16,
     * 2 + 8 for ord 17, 1 + 1 for ord 18, 4 + 6 for ord 23 and 2 + 11 for
     * ord 24.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {8, 10, 2, 10, 13};
    private static final int EXPECTED_RECORDS = 43;
    private static final int EXPECTED_STEPS = 138;

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

    private InfraNamedWindowTimeBatchViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowTimeBatchViewsScenarioOracle <scenario.json>");
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
     * (TestSuiteInfraNamedWindow lines 136-139): key string, value long.
     * Ord 16/18/23 insert the Long projection of longBoxed, mirroring the
     * {@code 1L} literals of their assertions; time_batch and
     * time_length_batch read no event property, so the column is plain data.
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
     * Replays one case on its own runtime with the clock pinned at 0.  A fresh
     * EPRuntime per case is mandatory for this slice: the five pinned timelines
     * restart at 0 (ord 18) or 1000 (the other four) after the previous case
     * ended at 11000 to 35000, and the scheduling service assigns the clock
     * unconditionally.  The explicit {@code advanceTime(0)} before any
     * statement is deployed makes that pin a property of the oracle itself
     * rather than of the runtime's initial clock; it cannot fire anything
     * because no statement exists yet, and ord 18's source spells the same pin
     * out as {@code sendTimer(env, 0)} at the head of its timeline.
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
     * deployments' public types so the later ord-17/ord-24 modules' insert-into
     * and on-delete statements and ord 18's late consumer can reference the
     * window created by the first module.  A pending module is deployed when
     * the next non-deploy step of the same case arrives, which is what lets
     * ord 18's consumer be compiled and attached mid-timeline at its source
     * position (INV:1808-1809, after the 5000 advance and its send).
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
                    records.add(snapshot(runtime, statement, caseName, fields));
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
        // EPL text: the insert statement of every module (lines 1594/1666/1798/
        // 2246/2324) is unnamed in the Java source, so the engine assigns it a
        // generated name; the scenario's step id of that statement is still the
        // module position and no listen or snapshot step resolves it.  Every
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
     * invocation that carries neither stream is a contract violation.  A batch
     * flush is exactly one invocation of the window statement's listener
     * carrying the pending batch as new data and, from the second flush on, the
     * previously flushed batch as old data, so one record holds both keys; a
     * flush whose two batches are both empty invokes nothing at all.  The
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
     * record with sequence 0 holding the engine iterator order, which is the
     * order the suite's exact-order iterator assertions pin.  The batch
     * window's iterator exposes the pending batch only, in insertion order,
     * so a snapshot right after a flush is empty even though the flushed rows
     * return as old data at the next flush.  Every snapshot of this slice is
     * exact-order, so no canonical reordering branch is needed.  An empty
     * iterator omits the new array, matching the Go normalizer's omitempty
     * rendering and the suite's {@code null} iterator assertions.
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
        int offset = validateTimeBatch(steps, 0);
        offset = validateTimeBatchSceneTwo(steps, offset);
        offset = validateTimeBatchLateConsumer(steps, offset);
        offset = validateTimeLengthBatch(steps, offset);
        offset = validateTimeLengthBatchSceneTwo(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraTimeBatch step sequence mirroring lines 1593-1652: the single
     * four-statement module deploy, the three arrivals that buffer silently
     * with their leading clock moves (1000, 5000, 10000), the five ordered
     * create iterator snapshots (J:1607/1611/1628/1643/1646), the E2 delete
     * that removes a pending row at t=10000 without any callback and the E4
     * send/delete pair at t=21000, the silent 10999 move before the 11000
     * new-only flush and the 21000 old-only flush of that same batch, and the
     * 31000 move whose empty flush emits nothing.
     */
    private static int validateTimeBatch(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TB_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TB_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TB_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TB_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:21Z");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E4");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:31Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeBatchSceneTwo step sequence mirroring lines 1662-1775: the
     * three single-statement module deploys, the G1 arrival at 1000, the clock
     * move to 5000 before the G1 snapshot and the G2 arrival, the G2 and G1
     * deletes that empty the pending batch silently, the 11000 empty flush that
     * emits nothing and does not re-arm, the three G3/G4/G5 arrivals at 15000
     * (the batch flushes at 21000 because the anchor stays at 1000), the G5
     * delete and the G6 arrival at 18000, the 21000 new-only flush of
     * G3/G4/G6, the G7/G8/G9 arrivals at 22000 with the G7 and G9 deletes,
     * and the 31000 flush whose old rows are exactly the 21000 batch.
     */
    private static int validateTimeBatchSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TB2_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TB2_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TB2_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateIntSend(steps.get(offset++), caseName, "G1", 1);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:15Z");
        validateIntSend(steps.get(offset++), caseName, "G3", 3);
        validateIntSend(steps.get(offset++), caseName, "G4", 4);
        validateIntSend(steps.get(offset++), caseName, "G5", 5);
        validateMarketSend(steps.get(offset++), caseName, "G5");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:18Z");
        validateIntSend(steps.get(offset++), caseName, "G6", 6);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:21Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:22Z");
        validateIntSend(steps.get(offset++), caseName, "G7", 7);
        validateIntSend(steps.get(offset++), caseName, "G8", 8);
        validateIntSend(steps.get(offset++), caseName, "G9", 9);
        validateMarketSend(steps.get(offset++), caseName, "G7");
        validateMarketSend(steps.get(offset++), caseName, "G9");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:31Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeBatchLateConsumer step sequence mirroring lines 1795-1819:
     * the leading sendTimer(0) clock pin, the two-statement module 1 deploy,
     * the repeated sendTimer(0) before the E1 arrival, the E2 arrival at 5000
     * followed by the MID-TIMELINE deploy of the aggregate consumer (the
     * scenario places it exactly where line 1808 compiles and attaches it,
     * after the 5000 move and its send and before the 8000 move), the E3
     * arrival at 8000, the 10000 boundary flush of the whole batch and the
     * empty create-iterator snapshot of line 1817.
     */
    private static int validateTimeBatchLateConsumer(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TBLC_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TBLC_INSERT);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TBLC_S0);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:08Z");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeLengthBatch step sequence mirroring lines 2245-2310: the
     * single four-statement module deploy, the E1/E2 arrivals at 1000 that arm
     * the timer to 11000, the E2 delete and the E3 arrival that leave the
     * pending batch at two rows, the immediate size-triggered flush on the E4
     * arrival at t=1000 (new = E1/E3/E4 in insertion order, no old data, the
     * boundary re-armed to 11000), the E5/E6 arrivals at 5000, the E5 delete,
     * the silent 10999 move before the 11000 time-triggered flush of the
     * remaining E6 with the size-flushed batch as old data, and the six ordered
     * create snapshots of J:2256/2262/2267/2280/2285/2290 (the one after the
     * size flush is empty).
     */
    private static int validateTimeLengthBatch(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TLB_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TLB_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TLB_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TLB_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E5", 5L);
        validateLongSend(steps.get(offset++), caseName, "E6", 6L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E5");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeLengthBatchSceneTwo step sequence mirroring lines
     * 2320-2442: the three single-statement module deploys, the G1 arrival at
     * 1000 that arms the timer to 11000, the G1/G2 snapshot pair at 5000, the
     * G2 and G1 deletes that empty the pending batch so the 11000 move is a
     * silent empty flush, the G3/G4 arrivals at 15000, the G5 arrival at 16000
     * and its delete at the same instant, the G6 arrival at 18000, the silent
     * 24999 move before the 25000 time-triggered flush of G3/G4/G6 (the size
     * trigger of 4 never fires), the G7/G8/G9 arrivals at 28000 with their
     * snapshot before the G7 and G9 deletes, the silent 34999 move and its
     * G8 snapshot at that instant, the 35000 flush of the remaining G8 with the
     * 25000 batch as old data, and the eleven ordered create snapshots of
     * J:2340/2347/2354/2360/2373/2380/2387/2406/2419/2429/2439.
     */
    private static int validateTimeLengthBatchSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[4];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TLB2_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TLB2_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TLB2_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateIntSend(steps.get(offset++), caseName, "G1", 1);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:15Z");
        validateIntSend(steps.get(offset++), caseName, "G3", 3);
        validateIntSend(steps.get(offset++), caseName, "G4", 4);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:16Z");
        validateIntSend(steps.get(offset++), caseName, "G5", 5);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G5");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:18Z");
        validateIntSend(steps.get(offset++), caseName, "G6", 6);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:24.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:25Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:28Z");
        validateIntSend(steps.get(offset++), caseName, "G7", 7);
        validateIntSend(steps.get(offset++), caseName, "G8", 8);
        validateIntSend(steps.get(offset++), caseName, "G9", 9);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G7");
        validateMarketSend(steps.get(offset++), caseName, "G9");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:34.999Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:35Z");
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
     * Ords 17 and 24 send theString plus intBoxed, the only value field their
     * projection windows read; every other SupportBean property stays at its
     * Java default.
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

    /** Ords 16, 18 and 23 send theString plus longBoxed, the value field their windows read. */
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
