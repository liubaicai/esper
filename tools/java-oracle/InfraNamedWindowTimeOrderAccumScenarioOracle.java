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
 * Java oracle for the infra-named-window-time-order-accum-views slice (S7) of
 * InfraNamedWindowViews.java: the ord-9 {@code InfraTimeOrderWindow}
 * {@code MyWindowTOW#time_order(value, 10 sec)} window over the
 * MySimpleKeyValueMap schema (lines 919-972), the ord-10
 * {@code InfraTimeOrderSceneTwo} legacy
 * {@code MyWindow.ext:time_order(value, 10)} projection window over
 * theString/longBoxed with three module deployments (lines 974-1090), the
 * ord-14 {@code InfraTimeAccum} {@code MyWindowTA#time_accum(10 sec)} window
 * over the same map schema (lines 1301-1417) and the ord-15
 * {@code InfraTimeAccumSceneTwo} legacy
 * {@code MyWindow.win:time_accum(10 sec)} projection window over
 * theString/intBoxed with four module deployments (lines 1419-1586).
 *
 * <p>Every execution of this slice moves the engine clock: the scenario
 * therefore carries {@code advance-time} steps, and the oracle replays them
 * through {@code EPRuntime.getEventService().advanceTime} exactly as the
 * suite's {@code sendTimer} helper does ({@code sendTimer(env,ms)} is literally
 * {@code env.advanceTime(ms)}, INV.java:3618-3620; the internal timer is
 * disabled, so schedules fire only on those calls).  Advancing the clock alone
 * fires the expiry waves with no event sent, and the runtime sets the clock
 * before it processes the schedule, so a wave carries the boundary instant:
 * {@code EPEventServiceImpl.advanceTime} performs {@code setTime(time)} then
 * {@code processSchedule(time)}.  Record {@code time} values are the virtual
 * clock at delivery as rendered by {@code Instant.ofEpochMilli}, which for
 * these cases is the epoch plus the advance offset and never the wall clock.
 *
 * <p>One EPRuntime is created per case, with a distinct runtime URI, because
 * the pinned timelines are not monotonic in sequence: ord 9 runs 5000 to
 * 100000, ord 10 starts at 20000, ords 14 and 15 restart at 1000.
 * {@code SchedulingServiceImpl.setTime} assigns the clock unconditionally with
 * no backwards guard, so sharing one runtime across the four cases (the
 * sibling-oracle style for slices that never advance) would move the clock
 * backwards and corrupt pending schedules.  Each case tears its runtime down
 * with undeployAll + destroy.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ords 9 and 14 compile their four statements (create,
 * insert, s0, delete) as ONE module exactly like the suite's single
 * {@code compileDeploy}, and ords 10 and 15 deploy one single-statement module
 * per step on one shared path (ord 10: create, insert, delete; ord 15: create,
 * insert, consume, delete) exactly like the source's separate compileDeploy
 * calls.  Joining the pinned step EPLs of one module with ";\n" reproduces the
 * source literal minus its trailing terminator.  The window creates of ords 9,
 * 10 and 15 carry {@code @public} because the later modules resolve the window
 * through the runtime path, and the insert statements of ords 9 and 14 plus
 * the insert/consume statements of ords 10 and 15 are unnamed or named exactly
 * as the source declares them.  The oracle matches each module's statements by
 * position and pins the names the source does declare.  Ord 15's four
 * {@code undeployModuleContaining} calls (J:1569-1572) collapse into the
 * case-end {@code undeploy-all} step: each of its four modules holds exactly
 * one statement and the teardown emits no records.
 *
 * <p>Listener set: create and s0 for ords 9 and 14, create only for ords 10 and
 * 15, attached when the module containing the statement completes, exactly like
 * the source's compileDeploy(...).addListener(...) chaining.  Ord 15's consume
 * consumer is deployed and attached by the suite (J:1433-1434) but never
 * asserted, and the on-delete statement of all four cases is attached in the
 * suite but never asserted, so both stay excluded exactly like the sibling
 * slices; deleted rows are observed through the window statement's old rows and
 * the ordered iterator snapshots.
 *
 * <p>Rows project exactly the fields the Java assertions read:
 * {@code fields = {"key", "value"}} for ords 9, 14 and 15 (ord 9 and 14 project
 * longBoxed as Long, ord 15 projects intBoxed as Integer; both render as JSON
 * numbers, so the Go projection must not coerce between them) and
 * {@code {"key"}} for ord 10.  Listener rows render in delivery order, and both
 * streams of one invocation land in one record: ord 10's stale G3 arrives below
 * the window tail ({@code runtimeTime - period + 1}) and is released by
 * {@code TimeOrderView.update} in the same callback as its own new data
 * ({@code child.update(newData, postOldEventsArray)}), so its record carries
 * new and old together, while the flush of {@code TimeAccumViewRStream}
 * delivers the whole batch as one old-data array in insertion order
 * ({@code sendRemoveStream}'s {@code LinkedHashMap} key set), which is how the
 * ord-14 25000/41000 and ord-15 53000 bursts show up as single callbacks of
 * three, two and two rows.
 *
 * <p>Snapshot steps replay the execution's {@code assertPropsPerRowIterator}
 * call sites in source order and emit the engine iterator order: TimeOrderView
 * iterates its TreeMap in ascending timestamp order and TimeAccumViewRStream
 * its batch in insertion order.  Every snapshot of this slice is exact-order
 * (the suite never calls the any-order variant), so the scenario pins mode
 * {@code "ordered"} everywhere and the oracle keeps the engine order verbatim.
 * An empty iterator emits the snapshot record with the {@code new} key omitted,
 * which is how the suite's {@code null} iterator assertions show up: seven of
 * the 37 snapshot records are empty - ord 9 once (J:964), ord 14 three times
 * (J:1354 after the three-row 25000 burst, J:1397 after the two-row 41000 burst
 * and J:1409 after the E8 delete) and ord 15 three times (J:1471 and J:1476
 * after the one-row 11000 burst and J:1506 after the G4 delete); ord 10 never
 * empties its iterator.  Snapshot instants follow the source order, which is
 * the one place this slice corrects the prefetch report's prose: ord 15's
 * J:1484 snapshot sits between the G3 send and its {@code advanceTime(29999)}
 * (the source asserts the iterator before moving the clock at J:1485), so that
 * record carries t=20000 and not t=29999; the count is unaffected.
 *
 * <p>Record counts are pinned to the Java source: ord 9 emits 16 records (6
 * listener waves x 2 listeners + 4 snapshots), ord 10 emits 21 (11 listener
 * callbacks + 10 snapshots), ord 14 emits 34 (13 waves x 2 listeners + 8
 * snapshots) and ord 15 emits 30 (15 listener callbacks + 15 snapshots), 101 in
 * total over 145 scenario steps.  The snapshot counts are the
 * assertPropsPerRowIterator call-site counts of each execution: 4 for ord 9
 * (J:943, J:949, J:955, J:964), 10 for ord 10 (J:999, J:1007, J:1019, J:1027,
 * J:1034, J:1041, J:1049, J:1057, J:1064, J:1071), 8 for ord 14 (J:1331,
 * J:1337, J:1354, J:1375, J:1381, J:1397, J:1403, J:1409) and 15 for ord 15
 * (J:1451, J:1458, J:1461, J:1468, J:1471, J:1476, J:1484, J:1492, J:1499,
 * J:1506, J:1520, J:1528, J:1536, J:1544, J:1551), matching the frozen
 * contract's 4/10/8/15 pins.
 *
 * <p>Dispatch order, pinned by this oracle and reported per contract section 4:
 * the named window's own statement listener always precedes a separately
 * deployed consumer ({@code NamedWindowTailViewInstance.update} calls
 * {@code child.update(...)} before the tail view's {@code addDispatches} to
 * consumers), so every wave of ords 9 and 14 delivers create before s0.
 * TimeOrderView retains rows while {@code value >= engineTime - period + 1} and
 * releases them old-only once the clock passes {@code value + period}: ord 9
 * releases E3 (1000) at exactly 11000 and E1 (3000) at exactly 13000 after the
 * silent 12999 move, ord 10 releases G4 (18000) at exactly 28000, G5 (22000) at
 * exactly 32000 and G6 (25000) at exactly 35000, the last one after the silent
 * 33000 reschedule hidden inside the 34999 move.  Deletes release their row
 * immediately and never re-arm (ord 9's E2 at 11000, ord 10's G2/G1).  A
 * {@code time_accum} arrival is delivered immediately as new data and re-arms
 * the flush to {@code arrival + 10000}; deleting the newest row re-arms to that
 * row's stored arrival plus ten seconds (ord 14's E7 delete at 38000 moves the
 * 48000 flush to 41000; ord 15's G2 delete at 5000 moves the 15000 flush to
 * 11000, and its G8 delete at 44000 moves the 54000 flush to 53000) while
 * deleting a non-newest row leaves the timer alone (ord 14's E2 at 15000, ord
 * 15's G6 at 44000) and deleting the last remaining row cancels it (ord 14's E8
 * at 55000, ord 15's G4 at 29999).  Deletes of unknown keys are silent (ord
 * 14's E4 at 25000).  The ord-10 pass-through assertion of J:1008-1014 checks
 * both streams of the single callback, and ord 15's {@code milestone} calls
 * emit no records.
 */
public final class InfraNamedWindowTimeOrderAccumScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-time-order-accum-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews time-order/accum-slice: the time_order(value, 10 sec) sliding window and its ext:time_order projection variant with pass-through rows, plus the time_accum map and projection windows whose multi-row expiry bursts release in insertion order under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "time-order-window",
            "time-order-scene-two",
            "time-accum",
            "time-accum-scene-two"
    };
    private static final int[] ORDINALS = {9, 10, 14, 15};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-320f1b03eddb24520308",
            "java-runtime-ed5bfc247d088a603224",
            "java-runtime-d74b2c9c614b26973637",
            "java-runtime-8b4e2e7d81bb990c7d72"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraTimeOrderWindow",
            "InfraTimeOrderSceneTwo",
            "InfraTimeAccum",
            "InfraTimeAccumSceneTwo"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "time_order(value, 10 sec) window over the key/value map schema: rows are retained while value >= clock - 9999 and released old-only by timer waves during advance-time, with deletes releasing immediately and ascending-value iterators",
            "legacy ext:time_order(value, 10) projection window over theString/longBoxed with three module deployments and only the window statement listened; a stale arriving row passes through in one callback carrying both streams",
            "time_accum(10 sec) window over the key/value map schema: arrivals deliver immediately and re-arm the flush to their arrival plus ten seconds, whose bursts release all retained rows as old data in insertion order, with newest-row deletes re-arming and last-row deletes cancelling the timer",
            "legacy win:time_accum(10 sec) projection window over theString/intBoxed with four module deployments and only the window statement listened across the delete, re-arm and two-row burst timeline"
    };

    // Transcriptions of InfraNamedWindowViews lines 923-926 (InfraTimeOrderWindow),
    // deployed as ONE module exactly like the source's single compileDeploy:
    // joining the four pinned step EPLs with ";\n" reproduces the literal minus
    // its trailing terminator.  The window create is @public; the map schema's
    // "value" column is the view's own timestamp property (TimeOrderView reads
    // the first view parameter as an event property, not as engine time).
    private static final String EPL_TOW_CREATE =
            "@name('create') @public create window MyWindowTOW#time_order(value, 10 sec)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_TOW_INSERT =
            "insert into MyWindowTOW select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TOW_S0 =
            "@name('s0') select irstream key, value as value from MyWindowTOW";
    private static final String EPL_TOW_DELETE =
            "@name('delete') on SupportMarketDataBean delete from MyWindowTOW where symbol = key";

    // Transcriptions of InfraNamedWindowViews lines 980/984/988
    // (InfraTimeOrderSceneTwo), deployed as three separate modules on one shared
    // RegressionPath exactly like the source's three compileDeploy calls.  The
    // window create carries @public because the insert and delete modules are
    // separate deployments resolving MyWindow by name, and the insert statement
    // is unnamed in the source (its scenario step id is the module position).
    // #time_order and #ext:time_order resolve to the same TimeOrderView: the
    // view registry keys on the view name alone.
    private static final String EPL_TWO_CREATE =
            "@name('create') @public create window MyWindow.ext:time_order(value, 10)"
                    + " as select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TWO_INSERT =
            "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean";
    private static final String EPL_TWO_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 1306-1309 (InfraTimeAccum),
    // deployed as ONE module exactly like the source's single compileDeploy.
    // The window create deliberately has NO @public (the source omits it) and
    // is the only one of the four cases whose window statement is not public;
    // the delete statement carries the "as s0" alias the source spells out.
    private static final String EPL_TA_CREATE =
            "@name('create') create window MyWindowTA#time_accum(10 sec) as MySimpleKeyValueMap";
    private static final String EPL_TA_INSERT =
            "insert into MyWindowTA select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_TA_S0 =
            "@name('s0') select irstream key, value as value from MyWindowTA";
    private static final String EPL_TA_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowTA as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 1425/1429/1433/1437
    // (InfraTimeAccumSceneTwo), deployed as four separate modules on one shared
    // RegressionPath exactly like the source's four compileDeploy calls.  The
    // window create carries @public because the following modules resolve
    // MyWindow by name; the consume statement is the separately deployed
    // irstream consumer the suite attaches but never asserts, so it stays
    // excluded from the trace.
    private static final String EPL_TAS_CREATE =
            "@name('create') @public create window MyWindow.win:time_accum(10 sec)"
                    + " as select theString as key, intBoxed as value from SupportBean";
    private static final String EPL_TAS_INSERT =
            "@name('insert') insert into MyWindow(key, value) select irstream theString, intBoxed"
                    + " from SupportBean";
    private static final String EPL_TAS_CONSUME =
            "@name('consume') select irstream key, value as value from MyWindow";
    private static final String EPL_TAS_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_TOW_CREATE, EPL_TWO_CREATE, EPL_TA_CREATE, EPL_TAS_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_TOW_INSERT, EPL_TWO_INSERT, EPL_TA_INSERT, EPL_TAS_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_TOW_S0, "", EPL_TA_S0, ""
    };
    private static final String[] CASE_CONSUME_EPLS = {
            "", "", "", EPL_TAS_CONSUME
    };
    private static final String[] CASE_DELETE_EPLS = {
            EPL_TOW_DELETE, EPL_TWO_DELETE, EPL_TA_DELETE, EPL_TAS_DELETE
    };
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "delete"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "consume", "delete"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"create"},
            {"create", "s0"},
            {"create"}
    };

    /**
     * Iterator-snapshot count per case, read off the execution's
     * assertPropsPerRowIterator call sites: ord 9 asserts four iterator states
     * (J:943, J:949, J:955, J:964), ord 10 ten (J:999, J:1007, J:1019, J:1027,
     * J:1034, J:1041, J:1049, J:1057, J:1064, J:1071), ord 14 eight (J:1331,
     * J:1337, J:1354, J:1375, J:1381, J:1397, J:1403, J:1409) and ord 15
     * fifteen (J:1451, J:1458, J:1461, J:1468, J:1471, J:1476, J:1484, J:1492,
     * J:1499, J:1506, J:1520, J:1528, J:1536, J:1544, J:1551), matching the
     * frozen contract's 4/10/8/15 pins - twelve of the fifteen ord-15 states
     * precede the send of the wave they describe and three follow a delete.
     */
    private static final int[] CASE_SNAPSHOTS = {4, 10, 8, 15};

    /**
     * Module grouping per deploy position: ords 9 and 14 compile all four
     * statements as one module (0,0,0,0), ord 10 deploys three single-statement
     * modules (0,1,2) and ord 15 four (0,1,2,3) on one shared path.  See the
     * class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 1, 2},
            {0, 0, 0, 0},
            {0, 1, 2, 3}
    };

    /**
     * Listener discipline: ords 9 and 14 listen to create and s0
     * (J:927/J:1310), ords 10 and 15 to create only (J:981/J:1426).  The
     * never-asserted delete listener of all four cases and ord 15's
     * never-observed consume consumer are excluded; see the class comment.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: ords 9, 14 and 15 assert {@code fields = {"key", "value"}}
     * (J:921/J:1303/J:1421) and ord 10 asserts {@code {"key"}} (J:976).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key"},
            {"key", "value"},
            {"key", "value"}
    };

    /**
     * Per-case record totals: 6 waves x 2 listeners + 4 snapshots for ord 9,
     * 11 listener callbacks + 10 snapshots for ord 10, 13 waves x 2 listeners +
     * 8 snapshots for ord 14 and 15 listener callbacks + 15 snapshots for
     * ord 15.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {16, 21, 34, 30};
    private static final int EXPECTED_RECORDS = 101;
    private static final int EXPECTED_STEPS = 145;

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

    private InfraNamedWindowTimeOrderAccumScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowTimeOrderAccumScenarioOracle <scenario.json>");
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
     * (TestSuiteInfraNamedWindow lines 136-139): key string, value long, which
     * is also the ord-9/ord-14 window's own timestamp property.  Ord 14 inserts
     * the Long projection of longBoxed, mirroring the {@code 1L} literals of
     * its assertions.
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
     * Replays one case on its own runtime.  A fresh EPRuntime per case is
     * mandatory for this slice: the four pinned timelines are not monotonic in
     * sequence and the scheduling service assigns the clock unconditionally.
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
     * deployments' public types so the later ord-10/ord-15 modules' insert-into,
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
        // EPL text: the insert statements of the ord-9 and ord-14 modules
        // (lines 924 and 1307) are unnamed in the Java source, and so are the
        // ord-10 create/insert (lines 980/984, the create statement's
        // @public/@name names it) and ord-10 delete statements, so the engine
        // assigns them generated names; the scenario's step ids of those
        // statements are still the module positions and no listen or snapshot
        // step resolves them.  Every statement the source does name must appear
        // at the pinned position under exactly that name.
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
     * invocation that carries neither stream is a contract violation.  A single
     * invocation carrying both streams (ord 10's pass-through row) therefore
     * produces ONE record with both keys.  The record time is the virtual clock
     * at delivery.
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
     * order the suite's exact-order iterator assertions pin (TimeOrderView's
     * ascending timestamp order or TimeAccumViewRStream's insertion order).
     * Every snapshot of this slice is exact-order, so no canonical reordering
     * branch is needed.  An empty iterator omits the new array, matching the Go
     * normalizer's omitempty rendering and the suite's {@code null} iterator
     * assertions.
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
        int offset = validateTimeOrderWindow(steps, 0);
        offset = validateTimeOrderSceneTwo(steps, offset);
        offset = validateTimeAccum(steps, offset);
        offset = validateTimeAccumSceneTwo(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraTimeOrderWindow step sequence mirroring lines 923-970: the
     * single four-statement module deploy, the four sends with their leading
     * clock moves (5000, 6000, 10000), the four ordered create iterator
     * snapshots (J:943/949/955/964), the 11000 expiry wave and the E2 delete
     * that both land at t=11, the silent 12999 move before the 13000 expiry and
     * the final 100000 move that emits nothing.
     */
    private static int validateTimeOrderWindow(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TOW_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TOW_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TOW_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TOW_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 3000L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:06Z");
        validateLongSend(steps.get(offset++), caseName, "E2", 2000L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateLongSend(steps.get(offset++), caseName, "E3", 1000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:12.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:13Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:01:40Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeOrderSceneTwo step sequence mirroring lines 980-1078: the
     * three single-statement module deploys, the clock moves to 20000 before
     * the G1 send (both G1 and G2 arrive at virtual time 20000), the ten
     * ordered create iterator snapshots (J:999/1007/1019/1027/1034/1041/1049/
     * 1057/1064/1071), the stale G3 that passes through at 21000, the G2 and G1
     * deletes, the equal-time 32000 advances around G6, the silent 27999/31999
     * moves before the 28000/32000 releases and the 34999 move that hides the
     * silent 33000 reschedule before the 35000 release of G6.
     */
    private static int validateTimeOrderSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TWO_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TWO_INSERT);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TWO_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:20Z");
        validateLongSend(steps.get(offset++), caseName, "G1", 23000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:20Z");
        validateLongSend(steps.get(offset++), caseName, "G2", 19000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:21Z");
        validateLongSend(steps.get(offset++), caseName, "G3", 10000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:21Z");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:22Z");
        validateLongSend(steps.get(offset++), caseName, "G4", 18000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:23Z");
        validateLongSend(steps.get(offset++), caseName, "G5", 22000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:27.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:28Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:31.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:32Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:32Z");
        validateLongSend(steps.get(offset++), caseName, "G6", 25000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:32Z");
        validateMarketSend(steps.get(offset++), caseName, "G1");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:34.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:35Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeAccum step sequence mirroring lines 1306-1415: the single
     * four-statement module deploy, the four arrivals that each re-arm the
     * flush (1000, 5000, 10000, 15000), the E2 delete at 15000 and the
     * three-row 25000 burst releasing E1/E3/E4 as old data, the silent 24999
     * move, the E4 delete of an already-flushed key, the E5/E6/E7 arrivals, the
     * E7 delete at 38000 that re-arms the 48000 flush to 41000, the two-row
     * 41000 burst, the E8 arrival at 50000 and the 55000 E8 delete that
     * cancels the timer, plus the four empty-iterator snapshots (J:1354/1397/
     * 1409) and the final 100000 move that emits nothing.
     */
    private static int validateTimeAccum(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TA_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TA_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TA_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TA_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10Z");
        validateLongSend(steps.get(offset++), caseName, "E3", 3L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:15Z");
        validateLongSend(steps.get(offset++), caseName, "E4", 4L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:24.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:25Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E4");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:30Z");
        validateLongSend(steps.get(offset++), caseName, "E5", 5L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:31Z");
        validateLongSend(steps.get(offset++), caseName, "E6", 6L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:38Z");
        validateLongSend(steps.get(offset++), caseName, "E7", 7L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E7");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:40.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:41Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:50Z");
        validateLongSend(steps.get(offset++), caseName, "E8", 8L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:55Z");
        validateMarketSend(steps.get(offset++), caseName, "E8");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:01:40Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraTimeAccumSceneTwo step sequence mirroring lines 1425-1572: the
     * four single-statement module deploys, the G1 arrival at 1000, the G2
     * arrival and delete at 5000 (the newest-row delete re-arms the flush from
     * 15000 to 11000), the silent 10999 move before the one-row 11000 burst,
     * the two consecutive empty snapshots of the post-burst state, the G3/G4
     * arrivals at 20000/29999, the G3 and G4 deletes that leave the batch empty
     * and cancel the timer, the silent 40000 move, the G5/G6/G7/G8 arrivals at
     * 41000/42000/43000/44000, the G6 delete that leaves the timer alone and
     * the G8 delete that re-arms it to 53000, the silent 52999 move and the
     * two-row 53000 burst.  The four undeployModuleContaining calls of J:1569-
     * 1572 collapse into the case-end undeploy-all step (one statement per
     * module, no records either way).
     */
    private static int validateTimeAccumSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TAS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_TAS_INSERT);
        validateDeploy(steps.get(offset++), caseName, "consume", EPL_TAS_CONSUME);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_TAS_DELETE);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:01Z");
        validateIntSend(steps.get(offset++), caseName, "G1", 1);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIntSend(steps.get(offset++), caseName, "G2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:10.999Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11Z");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:20Z");
        validateIntSend(steps.get(offset++), caseName, "G3", 3);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:29.999Z");
        validateIntSend(steps.get(offset++), caseName, "G4", 4);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G3");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G4");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:40Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:41Z");
        validateIntSend(steps.get(offset++), caseName, "G5", 5);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:42Z");
        validateIntSend(steps.get(offset++), caseName, "G6", 6);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:43Z");
        validateIntSend(steps.get(offset++), caseName, "G7", 7);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:44Z");
        validateIntSend(steps.get(offset++), caseName, "G8", 8);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G6");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G8");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:52.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:53Z");
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
     * Ord 15 sends theString plus intBoxed, the only value field its projection
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

    /** Ord 9/10/14 send theString plus longBoxed, the value field their windows read. */
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
