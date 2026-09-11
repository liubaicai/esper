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
 * Java oracle for the infra-named-window-ext-time-views slice (S6) of
 * InfraNamedWindowViews.java: the ord-6 {@code InfraExtTimeWindow} sliding
 * {@code MyWindowETW#ext_timed(value, 10 sec)} window over the
 * MySimpleKeyValueMap schema (lines 706-752), the ord-7
 * {@code InfraExtTimeWindowSceneTwo} legacy
 * {@code MyWindow.win:ext_timed(value, 10 sec)} projection window over
 * theString/longBoxed with a separately deployed irstream consumer
 * (lines 755-843) and the ord-53 {@code InfraExternallyTimedBatch}
 * epoch-referenced {@code MyWindowETB#ext_timed_batch(value, 10 sec, 0L)}
 * window over the same map schema (lines 3399-3456).
 *
 * <p>Unlike the sibling time-slice every execution of this slice is driven
 * purely by the timestamp property of the arriving event: the sliding
 * {@code ExternallyTimedWindowView} "does not use the schedule service time"
 * (its own Javadoc) and the batch view reads only the arriving event's
 * timestamp, no timer is ever scheduled and the suite never calls
 * {@code advanceTime}.  The scenario therefore carries NO advance-time steps;
 * the oracle pins the runtime clock to the epoch itself (one
 * {@code getEventService().advanceTime(0)} per case, mirroring the regression
 * harness requirement) so every record's {@code time} is
 * {@code 1970-01-01T00:00:00Z} and never the wall clock.  The Go runner pins
 * the same epoch at engine construction.
 *
 * <p>Dispatch order, pinned by the engine and reported per contract section 4:
 * the window statement's own listener always precedes its separately deployed
 * consumer (ords 6 and 53 deliver {@code create} before {@code s0} in every
 * wave; ord 6's single module, ord 53's single module and ord 7's four modules
 * on one path all register the window statement first).  Ord 6's sliding
 * threshold is {@code newest - 10000 + 1}: E4 (11000) releases E1 (1000) as
 * old data together with the arriving row in one invocation, the delete of E2
 * releases an old-only row, and when E5 (15000) arrives the head is the E2
 * tombstone, which is swept silently (no old row).  Ord 7 behaves the same on
 * the projection window: G3 (10000) releases G1 (0), the G2/G3 deletes release
 * old-only rows and the swept tombstones never emit.  Ord 53's batch is
 * anchored at the epoch reference 0L: arrivals emit nothing until the 10
 * second boundary is crossed (E3 9999 stays silent, E4 10000 flushes the
 * buffered {E1, E3} as new data with no old data because no batch was replaced
 * yet), deletes release rows without touching the pending batch (E4's delete
 * emits old-only and leaves the replaced batch intact, and an empty iterator
 * snapshot follows), and the E6 flush (21000) releases the new batch {E5} as
 * new data with the replaced batch {E1, E3} as old data.
 *
 * <p>One EPRuntime is created per case (distinct runtime URI), mirroring the
 * Go runner's fresh environment and engine per case and keeping the epoch pin
 * independent per case; each case tears its runtime down with undeployAll +
 * destroy.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: ords 6
 * and 53 compile their four statements (create, insert, s0, delete) as ONE
 * module exactly like the suite's single {@code compileDeploy}, while ord 7
 * deploys four single-statement modules on one shared path (create, insert,
 * consume, delete).  Joining the pinned step EPLs of one module with ";\n"
 * reproduces the source literal minus its trailing terminator.  The window
 * create of ord 7 carries {@code @public} because the later modules resolve
 * {@code MyWindow} through the runtime path, and the statements the source
 * leaves unnamed (both insert statements and the ord-7 delete) take their
 * scenario step ids from module position; the oracle matches each module's
 * statements by position and pins the names the source does declare.
 *
 * <p>Listener set: create and s0 for ord 6 and ord 53, create only for ord 7.
 * The on-delete statement of all three cases is attached in the suite but its
 * payload is never asserted, so it is excluded from the trace exactly like the
 * sibling slices; ord 7's consume consumer is attached by the suite
 * ({@code addListener("consume")}) but never observed, so it is excluded too.
 * Ord 7's tombstone behaviour is still fully pinned by the create listener and
 * the iterator snapshots.  Rows project the fields the Java assertions read
 * ({@code {"key", "value"}} for all three cases).
 *
 * <p>Snapshot steps replay the execution's {@code assertPropsPerRowIterator}
 * call sites in source order and emit the engine iterator order; every
 * snapshot of this slice is exact-order, so the scenario pins mode
 * {@code "ordered"} everywhere.  An empty iterator emits the snapshot record
 * with the {@code new} key omitted, which is how the suite's {@code null}
 * iterator assertion of ord 53 (line 3442) shows up.  The ord-53 E6 wave is
 * emitted listener-first: the flush is delivered during the E6 send (line
 * 3451) and the suite's iterator assertion follows it (line 3452), so the
 * oracle records the two listener callbacks at send time and the snapshot at
 * its scenario position.
 *
 * <p>Record counts are pinned to the Java source: ord 6 emits 16 records
 * (6 waves x 2 listeners + 4 snapshots), ord 7 emits 19 (7 listener waves + 12
 * snapshots) and ord 53 emits 14 (4 waves x 2 listeners + 6 snapshots), 49 in
 * total.  The ord-7 snapshot count is the corrected one: the frozen contract's
 * header pins 7, but the execution has twelve {@code assertPropsPerRowIterator}
 * call sites (J:791, J:798, J:801, J:806, J:813, J:818, J:821, J:825, J:828,
 * J:832, J:835 and J:839); the 7 counts only the post-send sites and drops the
 * five milestone re-checks that the sibling slice keeps (see the S5 unit's
 * documented ord-8 correction).  The chain's {@code iteratorSnapshots} pin is
 * therefore 12 for ord 7 and the slice total is 49 records, not 44 (the
 * 7-snapshot reading would give ord 7 14 records).
 */
public final class InfraNamedWindowExtTimeViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-ext-time-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews ext-time-slice: the ext_timed(value, 10 sec) sliding window, its projection variant with a separately deployed consumer and the epoch-referenced ext_timed_batch window, all driven purely by event timestamps with the clock pinned at the epoch (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "ext-time-window",
            "ext-time-window-scene-two",
            "ext-timed-batch"
    };
    private static final int[] ORDINALS = {6, 7, 53};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-c960628819cddf06d0bd",
            "java-runtime-aa7fe5003096f5bf1cf8",
            "java-runtime-3af75eefff14ce16ae81"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraExtTimeWindow",
            "InfraExtTimeWindowSceneTwo",
            "InfraExternallyTimedBatch"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "ext_timed(value, 10 sec) window over the key/value map schema: the sliding threshold newest-9999 releases expired rows as old data together with the arriving row in one invocation, and a delete releases old-only rows",
            "legacy win:ext_timed(value, 10 sec) projection window over theString/longBoxed with four module deployments and only the window statement listened across the expiry, delete and reinsert waves",
            "epoch-referenced ext_timed_batch(value, 10 sec, 0L) window over the key/value map schema: arrivals stay silent until the 10 second boundary, the flush releases the batch as new data with the replaced batch as old data, and deletes never touch the pending batch"
    };

    // Transcriptions of InfraNamedWindowViews lines 710-713 (InfraExtTimeWindow),
    // deployed as ONE module exactly like the source's single compileDeploy:
    // joining the four pinned step EPLs with ";\n" reproduces the literal minus
    // its trailing terminator.  The window is not @public (all four statements
    // are in the same module); the map schema's "value" column is the window's
    // ext_timed timestamp property.
    private static final String EPL_ETW_CREATE =
            "@name('create') create window MyWindowETW#ext_timed(value, 10 sec)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_ETW_INSERT =
            "insert into MyWindowETW select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_ETW_S0 =
            "@name('s0') select irstream key, value as value from MyWindowETW";
    private static final String EPL_ETW_DELETE =
            "@name('delete') on SupportMarketDataBean delete from MyWindowETW where symbol = key";

    // Transcriptions of InfraNamedWindowViews lines 761/767/773/779
    // (InfraExtTimeWindowSceneTwo), deployed as four separate modules on one
    // shared RegressionPath exactly like the source's four compileDeploy calls.
    // The window create carries @public because the insert, consume and delete
    // modules are separate deployments resolving MyWindow by name; the consume
    // statement is the separately deployed irstream consumer the suite attaches
    // but never asserts, so it stays excluded from the trace.
    private static final String EPL_ETW2_CREATE =
            "@name('create') @public create window MyWindow.win:ext_timed(value, 10 sec)"
                    + " as select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_ETW2_INSERT =
            "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean";
    private static final String EPL_ETW2_CONSUME =
            "@name('consume') select irstream key, value as value from MyWindow";
    private static final String EPL_ETW2_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 3403-3406
    // (InfraExternallyTimedBatch), deployed as ONE module exactly like the
    // source's single compileDeploy (the source literal ends with ";\n"; the
    // module join reproduces it minus the trailing terminator).  The third view
    // parameter 0L is the batch reference point in msec: the named-window batch
    // path anchors at the epoch, so the flush boundaries are the multiples of
    // 10000.
    private static final String EPL_ETB_CREATE =
            "@name('create') create window MyWindowETB#ext_timed_batch(value, 10 sec, 0L)"
                    + " as MySimpleKeyValueMap";
    private static final String EPL_ETB_INSERT =
            "insert into MyWindowETB select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_ETB_S0 =
            "@name('s0') select irstream key, value as value from MyWindowETB";
    private static final String EPL_ETB_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowETB as s1"
                    + " where s0.symbol = s1.key";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_ETW_CREATE, EPL_ETW2_CREATE, EPL_ETB_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_ETW_INSERT, EPL_ETW2_INSERT, EPL_ETB_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_ETW_S0, "", EPL_ETB_S0
    };
    private static final String[] CASE_CONSUME_EPLS = {
            "", EPL_ETW2_CONSUME, ""
    };
    private static final String[] CASE_DELETE_EPLS = {
            EPL_ETW_DELETE, EPL_ETW2_DELETE, EPL_ETB_DELETE
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

    /**
     * Iterator-snapshot count per case, read off the execution's
     * assertPropsPerRowIterator call sites: ord 6 asserts four iterator states
     * (J:719, J:728, J:738, J:744), ord 7 twelve (J:791, J:798, J:801, J:806,
     * J:813, J:818, J:821, J:825, J:828, J:832, J:835, J:839) and ord 53 six
     * (J:3417, J:3425, J:3434, J:3442, J:3447, J:3452).  The ord-7 figure is
     * the corrected one - see the class comment.
     */
    private static final int[] CASE_SNAPSHOTS = {4, 12, 6};

    /**
     * Module grouping per deploy position: ords 6 and 53 compile all four
     * statements as one module (0,0,0,0), ord 7 deploys four single-statement
     * modules on one shared path (0,1,2,3).  See the class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 1, 2, 3},
            {0, 0, 0, 0}
    };

    /**
     * Listener discipline: ords 6 and 53 listen to create and s0 (J:714/J:3407),
     * ord 7 to create only (J:762).  The never-asserted delete listener of all
     * three cases and ord 7's never-observed consume consumer are excluded;
     * see the class comment.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    /**
     * Row projection: all three cases assert {@code fields = {"key", "value"}}
     * (J:708/J:757/J:3401).
     */
    private static final String[][] CASE_FIELDS = {
            {"key", "value"},
            {"key", "value"},
            {"key", "value"}
    };

    /**
     * Per-case record totals: 6 waves x 2 listeners + 4 snapshots for ord 6,
     * 7 listener waves + 12 snapshots for ord 7 and 4 waves x 2 listeners +
     * 6 snapshots for ord 53.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {16, 19, 14};
    private static final int EXPECTED_RECORDS = 49;
    private static final int EXPECTED_STEPS = 61;

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

    private InfraNamedWindowExtTimeViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowExtTimeViewsScenarioOracle <scenario.json>");
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
     * is also the window's ext_timed timestamp property.
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
     * Replays one case on its own runtime, mirroring the Go runner's fresh
     * environment and engine per case; the epoch pin below is therefore
     * independent per case.
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
     * deployments' public types so ord 7's insert, consume and delete modules
     * can reference the window created by the first module.
     */
    private static void runCase(String caseName, String[] fields, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        // Harness clock pin, not a scenario step: these executions never
        // advance the engine clock (the ext_timed and ext_timed_batch views
        // expire off the event's timestamp property), but the runtime would
        // otherwise start at the wall clock, so the case pins it to the epoch
        // before its first step and every record carries
        // 1970-01-01T00:00:00Z.
        runtime.getEventService().advanceTime(0);
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
        // EPL text: the insert statement of the ord-6/ord-53 modules (lines 711
        // and 3404), the insert and delete statements of the ord-7 modules
        // (lines 767 and 779) are unnamed in the Java source, so the engine
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
     * invocation that carries neither stream is a contract violation.  The
     * record time is the virtual clock at delivery, which this slice pins to
     * the epoch.
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
     * order the suite's exact-order iterator assertions pin.  Every snapshot of
     * this slice is ordered, so no canonical reordering branch is needed.  An
     * empty iterator omits the new array, matching the Go normalizer's
     * omitempty rendering and the suite's {@code null} iterator assertion of
     * ord 53 (line 3442).
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
                bean.setLongBoxed(longInteger(payload.get("longBoxed"), "longBoxed"));
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
        int offset = validateExtTimeWindow(steps, 0);
        offset = validateExtTimeWindowSceneTwo(steps, offset);
        offset = validateExtTimedBatch(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact InfraExtTimeWindow step sequence mirroring lines 706-752: the four
     * module deploy steps (create, insert, s0, delete - one step per statement
     * of the single module), the five SupportBean sends plus the market delete
     * with all four ordered create iterator snapshots the source asserts
     * between them, and the case-end undeploy-all that mirrors the source
     * teardown.  There is no advance-time step: the window expires off the
     * event's value timestamp (E4 11000 releases E1 1000).
     */
    private static int validateExtTimeWindow(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_ETW_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ETW_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_ETW_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_ETW_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E2", 5000L);
        validateLongSend(steps.get(offset++), caseName, "E3", 10000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 11000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E5", 15000L);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraExtTimeWindowSceneTwo step sequence mirroring lines 755-843:
     * the four single-statement module deploys (create, insert, consume,
     * delete), the five SupportBean sends plus the two market deletes with all
     * twelve ordered create iterator snapshots the source asserts between them
     * (J:791, J:798, J:801, J:806, J:813, J:818, J:821, J:825, J:828, J:832,
     * J:835 and J:839 - the five milestone re-checks included), and the
     * case-end undeploy-all.  G3 (10000) expires G1 (0); no advance-time step
     * exists because the window expires off the event's value timestamp.
     */
    private static int validateExtTimeWindowSceneTwo(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_ETW2_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ETW2_INSERT);
        validateDeploy(steps.get(offset++), caseName, "consume", EPL_ETW2_CONSUME);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_ETW2_DELETE);
        validateLongSend(steps.get(offset++), caseName, "G1", 0L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "G2", 5000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "G3", 10000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "G4", 15000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "G3");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "G5", 21000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraExternallyTimedBatch step sequence mirroring lines 3399-3456:
     * the four module deploy steps of the single module, the six SupportBean
     * sends plus the two market deletes with all six ordered create iterator
     * snapshots the source asserts between them (one of them empty at J:3442),
     * and the case-end undeploy-all.  The E6 flush is delivered during the E6
     * send (J:3451) and the suite's iterator assertion (J:3452) precedes its
     * IR-pair assertion (J:3453), so the oracle emits the listener records at
     * send time and the snapshot at the step position below.
     */
    private static int validateExtTimedBatch(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_ETB_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ETB_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_ETB_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_ETB_DELETE);
        validateLongSend(steps.get(offset++), caseName, "E1", 1000L);
        validateLongSend(steps.get(offset++), caseName, "E2", 8000L);
        validateLongSend(steps.get(offset++), caseName, "E3", 9999L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E4", 10000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateMarketSend(steps.get(offset++), caseName, "E4");
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E5", 14000L);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateLongSend(steps.get(offset++), caseName, "E6", 21000L);
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
     * Every SupportBean send of this slice sets theString plus longBoxed, the
     * only value field the windows project (sendSupportBean(env, string, Long)
     * at J:3459-3466 and sendBeanLong at J:845-851); every other SupportBean
     * property stays at its Java default.
     */
    private static void validateLongSend(JsonValue value, String caseName, String expectedString,
                                         long expectedLongBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "longBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("longBoxed"), "longBoxed") != expectedLongBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    /** The deletes send new SupportMarketDataBean(symbol, 0, 0L, "") (J:3493-3497). */
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
