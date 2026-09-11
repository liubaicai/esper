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
 * Java oracle for the infra-named-window-consumer-views slice of
 * InfraNamedWindowViews.java: the ord-43 {@code InfraFilteringConsumer}
 * {@code MyWindowFC#unique(key)} window with its filtered consumer
 * (lines 2947-2995), the ord-45 {@code InfraFilteringConsumerLateStart}
 * {@code MyWindowFCLS#keepall} window whose filtered aggregate consumer is
 * deployed after the window was filled (lines 3120-3166), the ord-49
 * {@code InfraPriorStats} {@code MyWindowPS#keepall} window with its
 * prior-values consumer and the {@code #uni(value)} statistics consumer
 * (lines 3221-3260), the ord-50 {@code InfraLateConsumer}
 * {@code MyWindowLCL#keepall} window with its late {@code #uni} and
 * {@code count(*)} consumers (lines 3262-3311) and the ord-51
 * {@code InfraLateConsumerJoin} window whose left-outer join consumer against
 * {@code SupportMarketDataBean#keepall} is deployed after the window was filled
 * (lines 3313-3370).
 *
 * <p>None of the five executions moves the engine clock, so every record
 * carries the pinned start instant; one EPRuntime is created per case with the
 * clock pinned at 0 before any statement is deployed, which keeps that instant
 * deterministic instead of inheriting the wall clock or the previous case's
 * position ({@code SchedulingServiceImpl.setTime} assigns unconditionally).
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: the
 * scenario lists one deploy step per statement (the Go runner maps each step
 * onto one plan), while ord 43 (INV:2951-2955) and ord 49 (INV:3226-3230)
 * compile all their statements as one module, ord 45 deploys module A
 * (create+insert, INV:3125-3127), its filtered consumer (INV:3135) and the
 * delete trigger (INV:3152) separately, ord 50 deploys four single-statement
 * modules (INV:3269-3270, 3277-3278, 3287-3288, 3299-3300) and ord 51 three
 * (INV:3319-3320, 3327-3328, 3338-3341).  Joining the pinned step EPLs of one
 * module with ";\n" reproduces the source literal minus its trailing
 * terminator; the window creates of the multi-module cases carry {@code @public}
 * because the later modules resolve the window through the runtime path.
 *
 * <p>Listener set: create and s0 for ord 43, s0 only for ord 45, s0 and s3 for
 * ord 49, create and s0 for ord 50 and create and s2 for ord 51.  Never-asserted
 * listeners stay excluded exactly like the sibling slices (ord 43's and ord 45's
 * delete triggers, ord 49's create, ord 50's s2), while every invocation of an
 * attached listener is recorded - including the two ord-43 waves the suite never
 * asserts (INV:2989-2990).
 *
 * <p>Snapshot steps replay the execution's iterator assertions in source order
 * and carry their own projection list and comparison mode.  The suite reads
 * order-sensitive and order-insensitive iterators side by side: ord 43's
 * {@code #unique(key)} window iterator and its filtered consumer iterate a
 * HashMap (INV:2969/2981/2991 are the any-order assertions while INV:2970 and
 * the empty INV:2982 stay ordered), ord 45's {@code sum} iterator is a single
 * RowForAll row, ord 49's {@code #uni(value)} statistics view exposes exactly
 * one row at every site, ord 50 reads one {@code #uni} row and one
 * {@code count(*)} row and ord 51's join iterator is recomputed from the join
 * repositories - ordered while only unmatched rows exist (INV:3343) and
 * any-order afterwards (INV:3357/3361/3366).  The ord-51 case marker carries
 * {@code mode: "any"} because that execution's two-row join invocation order is
 * deliberately unpinned by the suite; all other case markers carry no mode.
 * No snapshot of this slice is count-only, so no step uses the empty-field
 * encoding.  An empty iterator emits the snapshot record with the {@code new}
 * key omitted, which is how INV:2982's exhausted-iterator assertion shows up.
 *
 * <p>Record counts are pinned to the Java source and the attached listeners:
 * ord 43 emits 19 records (14 listener callbacks + 5 snapshots), ord 45 emits 9
 * (3 + 6), ord 49 emits 12 (8 + 4), ord 50 emits 15 (8 + 7, because its create
 * listener keeps firing on the three arrivals that follow the statistics
 * consumer's deploy even though the suite asserts it only twice) and ord 51
 * emits 9 (5 + 4), 64 in total over 87 scenario steps.
 *
 * <p>Semantics this oracle observes and the Go side must reproduce: the
 * {@code #unique(key)} window replaces a row in ONE callback carrying the new
 * and the replaced row (ord 43's G1 waves), the consumer filter is evaluated on
 * BOTH streams so a filtered-out new row still delivers the passing old row and
 * a filtered-out pair is fully silent, {@code #uni(value)} is the univariate
 * statistics view (single-row iterators, {@code average} as a Double) rather
 * than a dedupe view, the late-start preload pushes the window snapshot into the
 * aggregation or the join before the statement is dispatchable (ord 45's 7, ord
 * 50's 1.5 and 4, ord 51's null-padded rows), ord 49's {@code prior(1|2, key)}
 * reads the statement's own arrival history, and ord 51's join iterator
 * recomputes the full left-outer join so an unmatched row disappears from the
 * iterator without a remove-stream record once a matching market event exists.
 */
public final class InfraNamedWindowConsumerViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-consumer-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews consumer slice: the unique-key window with its filtered consumer and the keepall window with a late filtered aggregate, plus the prior-value, late univariate-statistics and late left-outer-join consumers whose preload and iterator shapes differ from their listener deltas (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "filtering-consumer",
            "filtering-consumer-late-start",
            "prior-stats",
            "late-consumer",
            "late-consumer-join"
    };
    private static final int[] ORDINALS = {43, 45, 49, 50, 51};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-a7827f22ee0c135e84d2",
            "java-runtime-502dd5b0e84f28fb2c68",
            "java-runtime-5ccc9535c9241efda4cc",
            "java-runtime-5118f72a4d8684d593d0",
            "java-runtime-c49a6a43a1efd3fa0729"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraFilteringConsumer",
            "InfraFilteringConsumerLateStart",
            "InfraPriorStats",
            "InfraLateConsumer",
            "InfraLateConsumerJoin"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "unique(key) window over the theString/intPrimitive projection: a same-key arrival replaces the stored row in one callback carrying the new and the replaced row, and the filtered consumer evaluates its filter on both streams so the replaced row still passes while the filtered-out new row is dropped",
            "keepall window over the theString/intPrimitive projection with a late filtered aggregate consumer: the preload pushes the two matching window rows as one update so the sum starts at 7, and a filtered-out arrival never enters the aggregation, not even for its later delete",
            "keepall map window with a prior-values consumer and a univariate-statistics consumer: prior(1|2, key) reads the statement's own arrival history while the uni(value) view exposes the running average as a single-row iterator on every update",
            "keepall map window whose univariate-statistics consumer and count(*) consumer are both deployed late: the two independent preloads start the average at 1.5 and the count at 4, and the statistics consumer is irstream so the previous average is delivered as old data",
            "keepall map window whose left-outer join consumer against the market-data window is deployed late: the replayed window arrives in the join before the first iterator read, the market send matches both rows in either order, and the join iterator recomputes the full result so an unmatched row vanishes without a remove-stream record"
    };

    // Transcriptions of InfraNamedWindowViews lines 2951-2954
    // (InfraFilteringConsumer), deployed as ONE module exactly like the source's
    // single compileDeploy.  The window create carries no @public (the source
    // omits it) and projects intPrimitive as value, so the window column is an
    // Integer; the consumer filter is part of the from-clause.
    private static final String EPL_FC_CREATE =
            "@name('create') create window MyWindowFC#unique(key)"
                    + " as select theString as key, intPrimitive as value from SupportBean";
    private static final String EPL_FC_INSERT =
            "insert into MyWindowFC select theString as key, intPrimitive as value from SupportBean";
    private static final String EPL_FC_S0 =
            "@name('s0') select irstream key, value as value from MyWindowFC(value > 0, value < 10)";
    private static final String EPL_FC_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFC as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 3125-3126, 3135 and 3152
    // (InfraFilteringConsumerLateStart): module A holds the window and its
    // insert, the filtered aggregate consumer is a second module deployed after
    // the three fills and the delete trigger is a third one deployed after the
    // consumer snapshots.  The window create is @public because the later
    // modules resolve it by name, and the consumer filter sits in the
    // from-clause so the preload only sees the matching rows.
    private static final String EPL_FCLS_CREATE =
            "@name('create') @public create window MyWindowFCLS#keepall"
                    + " as select theString as key, intPrimitive as value from SupportBean";
    private static final String EPL_FCLS_INSERT =
            "insert into MyWindowFCLS select theString as key, intPrimitive as value from SupportBean";
    private static final String EPL_FCLS_S0 =
            "@name('s0') select irstream sum(value) as sumvalue from MyWindowFCLS(value > 0, value < 10)";
    private static final String EPL_FCLS_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFCLS as s1"
                    + " where s0.symbol = s1.key";

    // Transcriptions of InfraNamedWindowViews lines 3226-3229 (InfraPriorStats),
    // deployed as ONE module exactly like the source's single compileDeploy.
    // The window create deliberately has NO @public; the univariate-statistics
    // derived view lives on the window spec of the s3 statement and adds the
    // "average" column the suite reads.
    private static final String EPL_PS_CREATE =
            "@name('create') create window MyWindowPS#keepall as MySimpleKeyValueMap";
    private static final String EPL_PS_INSERT =
            "insert into MyWindowPS select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_PS_S0 =
            "@name('s0') select prior(1, key) as priorKeyOne, prior(2, key) as priorKeyTwo"
                    + " from MyWindowPS";
    private static final String EPL_PS_S3 =
            "@name('s3') select average from MyWindowPS#uni(value)";

    // Transcriptions of InfraNamedWindowViews lines 3269-3270, 3277-3278,
    // 3287-3288 and 3299-3300 (InfraLateConsumer), deployed as FOUR
    // single-statement modules on one shared path exactly like the source's
    // four compileDeploy calls.  The statistics consumer preloads the two
    // filled rows, the count consumer preloads the four rows that exist by its
    // own deploy time and neither has a listener on the count statement.
    private static final String EPL_LCL_CREATE =
            "@name('create') @public create window MyWindowLCL#keepall as MySimpleKeyValueMap";
    private static final String EPL_LCL_INSERT =
            "insert into MyWindowLCL select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LCL_S0 =
            "@name('s0') select irstream average from MyWindowLCL#uni(value)";
    private static final String EPL_LCL_S2 =
            "@name('s2') select count(*) as cnt from MyWindowLCL";

    // Transcriptions of InfraNamedWindowViews lines 3319-3320, 3327-3328 and
    // 3338-3341 (InfraLateConsumerJoin), deployed as THREE single-statement
    // modules.  The join consumer selects from the window and the market-data
    // window with a left outer join on the shared Long key and is deployed
    // after the window was filled, which is what makes the replay into the join
    // observable: without it the first iterator read would be empty.
    private static final String EPL_LCJ_CREATE =
            "@name('create') @public create window MyWindowLCJ#keepall as MySimpleKeyValueMap";
    private static final String EPL_LCJ_INSERT =
            "insert into MyWindowLCJ select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_LCJ_S2 =
            "@name('s2') select key, value, symbol from MyWindowLCJ as s0"
                    + " left outer join SupportMarketDataBean#keepall as s1"
                    + " on s0.value = s1.volume";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_FC_CREATE, EPL_FCLS_CREATE, EPL_PS_CREATE, EPL_LCL_CREATE, EPL_LCJ_CREATE
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_FC_INSERT, EPL_FCLS_INSERT, EPL_PS_INSERT, EPL_LCL_INSERT, EPL_LCJ_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_FC_S0, EPL_FCLS_S0, EPL_PS_S0, EPL_LCL_S0, ""
    };
    private static final String[] CASE_S2_EPLS = {
            "", "", "", EPL_LCL_S2, EPL_LCJ_S2
    };
    private static final String[] CASE_S3_EPLS = {
            "", "", EPL_PS_S3, "", ""
    };
    private static final String[] CASE_CONSUME_EPLS = {"", "", "", "", ""};
    private static final String[] CASE_DELETE_EPLS = {
            EPL_FC_DELETE, EPL_FCLS_DELETE, "", "", ""
    };
    private static final String[] CASE_VAR_EPLS = {"", "", "", "", ""};
    private static final String[] CASE_ONSET_EPLS = {"", "", "", "", ""};
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "s0", "delete"},
            {"create", "insert", "s0", "s3"},
            {"create", "insert", "s0", "s2"},
            {"create", "insert", "s2"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0"},
            {"s0"},
            {"s0", "s3"},
            {"create", "s0"},
            {"create", "s2"}
    };

    /**
     * Listener row projection per case and deploy position (parallel to
     * CASE_DEPLOYS; unused slots are empty): ord 43 and ord 51 project the
     * window's key/value (and the join's symbol), ord 45 projects the aggregate
     * sumvalue, ord 49's prior consumer projects its two key columns while its
     * statistics consumer projects average, and ord 50's statistics consumer
     * projects average.
     */
    private static final String[][][] CASE_LISTENER_FIELDS = {
            {
                    {"key", "value"}, {"key", "value"}, {"key", "value"}, {}
            },
            {
                    {"sumvalue"}, {"sumvalue"}, {"sumvalue"}, {}
            },
            {
                    {"key", "value"}, {}, {"priorKeyOne", "priorKeyTwo"}, {"average"}
            },
            {
                    {"key", "value"}, {}, {"average"}, {}
            },
            {
                    {"key", "value"}, {}, {"key", "value", "symbol"}
            }
    };

    /**
     * Snapshot-step count per case: ord 43 asserts five iterator states
     * (INV:2969/2970/2981/2982/2991), ord 45 six (INV:3136/3140/3144/3148/3156/
     * 3160), ord 49 four (INV:3241/3246/3251/3256), ord 50 seven (INV:3289/
     * 3293/3297/3301/3302/3306/3307) and ord 51 four (INV:3343/3357/3361/3366).
     */
    private static final int[] CASE_SNAPSHOTS = {5, 6, 4, 7, 4};

    /**
     * Module grouping per deploy position: ord 43 and ord 49 compile all their
     * statements as one module, ord 45 deploys module A (create+insert), the
     * consumer and the delete trigger as three modules, ord 50 four
     * single-statement modules and ord 51 three.  See the class comment.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 0, 0, 0},
            {0, 0, 1, 2},
            {0, 0, 0, 0},
            {0, 1, 2, 3},
            {0, 1, 2}
    };

    /**
     * Orders 49, 50 and 51 assert the window event type's property types
     * (String key, Long value) through {@code assertStatement} at
     * INV:3232-3235, INV:3272-3275 and INV:3322-3325; the two intPrimitive
     * projections of ords 43/45 are not covered by that assertion.
     */
    private static final boolean[] CASE_WINDOW_PROPERTY_PINS = {false, false, true, true, true};

    /**
     * Listener discipline: create and s0 for ord 43, s0 only for ord 45, s0 and
     * s3 for ord 49, create and s0 for ord 50 and create and s2 for ord 51.
     * The never-asserted delete listeners of ords 43/45 and ord 49's create
     * listener are excluded; ord 50's s2 has no listener in the suite at all.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    private static final Map<String, Map<String, String[]>> LISTENER_FIELDS;

    /**
     * Per-case record totals: 14 listener callbacks + 5 snapshots for ord 43,
     * 3 + 6 for ord 45, 8 + 4 for ord 49, 8 + 7 for ord 50 and 5 + 4 for
     * ord 51.  Ord 50's create listener keeps firing on the three arrivals that
     * follow the statistics consumer's deploy (the suite asserts it only at the
     * first two), so that case emits eight listener records, not the five its
     * assertion sites alone suggest.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {19, 9, 12, 15, 9};
    private static final int EXPECTED_RECORDS = 64;
    private static final int EXPECTED_STEPS = 87;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Map<String, String[]>> listenerFields = new HashMap<>();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
            Map<String, Integer> moduleKeys = new HashMap<>();
            Map<String, String[]> fieldsByStatement = new HashMap<>();
            for (int position = 0; position < CASE_DEPLOYS[index].length; position++) {
                String statement = CASE_DEPLOYS[index][position];
                moduleKeys.put(statement, CASE_MODULE_KEYS[index][position]);
                fieldsByStatement.put(statement, CASE_LISTENER_FIELDS[index][position]);
            }
            modules.put(CASE_NAMES[index], moduleKeys);
            listenerFields.put(CASE_NAMES[index], fieldsByStatement);
        }
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
        LISTENER_FIELDS = Collections.unmodifiableMap(listenerFields);
    }

    private InfraNamedWindowConsumerViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowConsumerViewsScenarioOracle <scenario.json>");
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
            runCaseOnFreshRuntime(CASE_NAMES[index], configuration, allSteps, records);
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
     * is also the join key of ord 51.  Ord 49/50/51 insert the Long projection
     * of longBoxed, mirroring the {@code 1L} literals of their assertions and
     * the {@code Long.class} property-type pins at INV:3232-3235,
     * INV:3272-3275 and INV:3322-3325.
     */
    private static Map<String, Object> simpleKeyValueSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("key", String.class);
        schema.put("value", long.class);
        return schema;
    }

    /**
     * Map event type carrying the regression-lib SupportMarketDataBean read
     * surface (symbol, price, volume, feed, id); the regression-lib bean is not
     * on the oracle classpath.  Ord 43's and ord 45's delete triggers compare
     * symbol with the window's key column, and ord 51's join matches the
     * boxed Long volume of a market event against the window's Long value, so
     * the sender sets volume from the payload (defaulting to 0L) exactly like
     * sendMarketBean(env, symbol, volume) and new SupportMarketDataBean(symbol,
     * 0, volume, "").
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
     * EPRuntime per case keeps that instant deterministic: none of the five
     * executions moves the clock, so without the pin the records would inherit
     * the wall clock or the previous case's position, and the scheduling
     * service assigns the clock unconditionally.  The explicit
     * {@code advanceTime(0)} before any statement is deployed cannot fire
     * anything because no statement exists yet.
     */
    private static void runCaseOnFreshRuntime(String caseName, Configuration configuration,
                                              JsonArray allSteps, JsonArray records)
            throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        try {
            runtime.getEventService().advanceTime(0);
            runCase(caseName, configuration, runtime, allSteps, records);
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
     * deployments' public types so the later modules of ords 45, 50 and 51 can
     * resolve the window created by the first module.  A pending module is
     * deployed when the next non-deploy step of the same case arrives, which is
     * what lets the late consumers be compiled mid-timeline at their source
     * positions (ord 45 INV:3135, ord 50 INV:3288/3300, ord 51 INV:3341).
     */
    private static void runCase(String caseName, Configuration configuration,
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
                    deployModule(configuration, runtime, caseName, pendingStatements,
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
            deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
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
        // EPL text: the insert statement of every module (lines 2952/3126/3227/
        // 3278/3328) is unnamed in the Java source, so the engine assigns it a
        // generated name and the scenario's step id of that statement is the
        // module position; no listen or snapshot step resolves it.  Every
        // statement the source does name (create, s0, s2, s3 and delete) must
        // appear at the pinned position under exactly that name.
        Set<String> scenarioNames = new HashSet<>(statementNames);
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        Map<String, String[]> listenerFields =
                LISTENER_FIELDS.getOrDefault(caseName, Collections.emptyMap());
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
            if ("create".equals(name) && windowPropertyPin(caseName)) {
                // The suite's assertStatement blocks of INV:3232-3235,
                // INV:3272-3275 and INV:3322-3325 pin the window event type's
                // property types; this harness-internal pin keeps them checked
                // without adding a record the suite does not assert.
                if (statement.getEventType().getPropertyType("key") != String.class
                        || statement.getEventType().getPropertyType("value") != Long.class) {
                    throw new IllegalStateException("case " + caseName + " window event type = key "
                            + statement.getEventType().getPropertyType("key") + ", value "
                            + statement.getEventType().getPropertyType("value")
                            + ", want String and Long");
                }
            }
            if (listened.contains(name)) {
                statement.addListener(listener(caseName, listenerFields.get(name), sequences, records,
                        runtime));
            }
        }
    }

    /** Whether the case asserts the window event type's property types. */
    private static boolean windowPropertyPin(String caseName) {
        for (int index = 0; index < CASE_NAMES.length; index++) {
            if (CASE_NAMES[index].equals(caseName)) {
                return CASE_WINDOW_PROPERTY_PINS[index];
            }
        }
        throw new IllegalStateException("unknown case " + caseName);
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.  The
     * shapes this slice pins are the single new row of an ordinary arrival, the
     * new-plus-old pair of a unique-key replacement and of every statistics
     * update, the old-only delta of a delete whose passing row the filter lets
     * through, the new-only delta of a statistics update on an istream-free
     * select and the two-row new array of ord 51's join replay.  The record
     * time is the virtual clock at delivery.
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
     * the step's field list; the scenario's mode says whether the differential
     * compares that order positionally.  The suite mixes the two: ord 43's
     * unique-view window and its filtered consumer iterate a HashMap, so those
     * states are any-order, while ord 45's RowForAll sum, ord 49's statistics
     * row, ord 50's statistics and count rows and ord 51's recomputed join
     * result are ordered (INV:3343) except where the suite switched to the
     * any-order variant.  No snapshot of this slice is count-only, so no step
     * uses the empty-field encoding; an empty iterator omits the new array,
     * which is how the exhausted consumer iterator of INV:2982 shows up.  No
     * canonical reordering happens here; the mode is carried by the scenario
     * step, not by the record.
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
     * Sends one scenario event.  SupportBean carries theString plus intPrimitive
     * for ords 43/45 and theString plus longBoxed for ords 49/50/51; the market
     * map carries symbol (plus the optional volume that ord 51's join matches)
     * and the delete triggers of ords 43/45 only use symbol.
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
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportMarketDataBean": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("symbol", string(payload, "symbol"));
                event.put("price", 0.0d);
                JsonValue volume = payload.get("volume");
                event.put("volume", volume == null ? 0L : longInteger(volume, "volume"));
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
                    "description", "createEpl", "insertEpl", "s0Epl", "s2Epl", "s3Epl",
                    "consumeEpl", "deleteEpl", "varEpl", "onSetEpl", "deploys", "listened",
                    "iteratorSnapshots");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !CASE_CREATE_EPLS[index].equals(string(definition, "createEpl"))
                    || !CASE_INSERT_EPLS[index].equals(string(definition, "insertEpl"))
                    || !CASE_S0_EPLS[index].equals(string(definition, "s0Epl"))
                    || !CASE_S2_EPLS[index].equals(string(definition, "s2Epl"))
                    || !CASE_S3_EPLS[index].equals(string(definition, "s3Epl"))
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
        int offset = validateFilteringConsumer(steps, 0);
        offset = validateFilteringConsumerLateStart(steps, offset);
        offset = validatePriorStats(steps, offset);
        offset = validateLateConsumer(steps, offset);
        offset = validateLateConsumerJoin(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }



    /**
     * Exact InfraFilteringConsumer step sequence mirroring lines 2951-2993: the
     * single four-statement module deploy, the two same-key G1 arrivals whose
     * replacement is one new-plus-old delta, the G2 arrival with its any-order
     * window and ordered consumer snapshots (INV:2969/2970), the G2 delete, the
     * filtered-out G3 arrival and delete with the exhausted consumer iterator of
     * INV:2982, and the two closing arrivals whose consumer state is read
     * any-order (INV:2991).  No clock moves.
     */
    private static int validateFilteringConsumer(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_FC_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_FC_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FC_S0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_FC_DELETE);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G1", 5);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G1", 15);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G2", 8);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", "key", "value");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "key", "value");
        validateMarketSend(steps.get(offset++), caseName, "G2");
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G3", -1);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", "key", "value");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "key", "value");
        validateMarketSend(steps.get(offset++), caseName, "G3");
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G1", 6);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G2", 7);
        validateSnapshot(steps.get(offset++), caseName, "s0", "any", "key", "value");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraFilteringConsumerLateStart step sequence mirroring lines
     * 3125-3164: the two-statement module A, the three fills that arrive before
     * any listener exists, the MID-TIMELINE deploy of the filtered aggregate
     * consumer at its source position (INV:3135) whose preload the first
     * ordered snapshot reads, the four filtered/unfiltered arrivals and their
     * snapshots, the late delete module (INV:3152), the two market deletes and
     * the three module-scoped undeploys in source order.  No clock moves.
     */
    private static int validateFilteringConsumerLateStart(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_FCLS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_FCLS_INSERT);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G1", 5);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G2", 15);
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G3", 2);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FCLS_S0);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G4", 1);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G5", 20);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateIntPrimitiveSend(steps.get(offset++), caseName, "G6", 9);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_FCLS_DELETE);
        validateMarketSend(steps.get(offset++), caseName, "G4");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateMarketSend(steps.get(offset++), caseName, "G5");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "sumvalue");
        validateUndeploy(steps.get(offset++), caseName, "s0");
        validateUndeploy(steps.get(offset++), caseName, "delete");
        validateUndeploy(steps.get(offset++), caseName, "create");
        return offset;
    }

    /**
     * Exact InfraPriorStats step sequence mirroring lines 3226-3258: the single
     * four-statement module deploy and the four arrivals, each followed by the
     * ordered statistics snapshot of INV:3241/3246/3251/3256.  The prior-values
     * consumer and the statistics consumer are asserted through their listener
     * records only for the new stream; no clock moves.
     */
    private static int validatePriorStats(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_PS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_PS_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_PS_S0);
        validateDeploy(steps.get(offset++), caseName, "s3", EPL_PS_S3);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "s3", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s3", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E3", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s3", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E4", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s3", "ordered", "average");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLateConsumer step sequence mirroring lines 3269-3309: the four
     * single-statement modules, the two fills that exist before the statistics
     * consumer is deployed mid-timeline (INV:3288, first ordered snapshot
     * INV:3289), the two updates, the second late consumer at INV:3300 with the
     * count preload (INV:3301) and the final update with both consumer
     * snapshots of INV:3306/3307.  No clock moves.
     */
    private static int validateLateConsumer(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LCL_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LCL_INSERT);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 2L);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LCL_S0);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E3", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E4", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "average");
        validateDeploy(steps.get(offset++), caseName, "s2", EPL_LCL_S2);
        validateSnapshot(steps.get(offset++), caseName, "s2", "ordered", "cnt");
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "average");
        validateLongSend(steps.get(offset++), caseName, "E5", 3L);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", "average");
        validateSnapshot(steps.get(offset++), caseName, "s2", "ordered", "cnt");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraLateConsumerJoin step sequence mirroring lines 3319-3368: the
     * three single-statement modules, the two fills, the MID-TIMELINE deploy of
     * the left-outer join consumer at INV:3341 whose replayed window the ordered
     * null-padded snapshot of INV:3343 reads, the two market sends (S1 matching
     * both rows in the any-order invocation order the suite leaves open, S2
     * matching nothing) with their any-order snapshots, the third window arrival
     * that joins S2 and its any-order snapshot.  The case marker carries the
     * mode "any" the suite's ambiguity requires; no clock moves.
     */
    private static int validateLateConsumerJoin(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[4];
        validateCaseMarkerWithMode(steps.get(offset++), caseName, "any");
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LCJ_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LCJ_INSERT);
        validateLongSend(steps.get(offset++), caseName, "E1", 1L);
        validateLongSend(steps.get(offset++), caseName, "E2", 1L);
        validateDeploy(steps.get(offset++), caseName, "s2", EPL_LCJ_S2);
        validateSnapshot(steps.get(offset++), caseName, "s2", "ordered",
                "key", "value", "symbol");
        validateMarketVolumeSend(steps.get(offset++), caseName, "S1", 1L);
        validateSnapshot(steps.get(offset++), caseName, "s2", "any", "key", "value", "symbol");
        validateMarketVolumeSend(steps.get(offset++), caseName, "S2", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s2", "any", "key", "value", "symbol");
        validateLongSend(steps.get(offset++), caseName, "E3", 2L);
        validateSnapshot(steps.get(offset++), caseName, "s2", "any", "key", "value", "symbol");
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

    /**
     * Ord 51's case marker carries the any-order mode that tells the
     * differential to sort that case's listener rows; every other case marker
     * has no mode key at all.
     */
    private static void validateCaseMarkerWithMode(JsonValue value, String expectedCase,
                                                   String expectedMode) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case", "mode");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))
                || !expectedMode.equals(string(marker, "mode"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase
                    + " with mode " + expectedMode);
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
     * Ords 43 and 45 send theString plus intPrimitive (the
     * sendSupportBeanInt helper, which never touches longBoxed); every other
     * SupportBean property stays at its Java default.
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

    /** Ords 49, 50 and 51 send theString plus longBoxed, the value their windows read. */
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

    /** Ord 51 sends a market event whose boxed Long volume is the join key. */
    private static void validateMarketVolumeSend(JsonValue value, String caseName,
                                                 String expectedSymbol, long expectedVolume) {
        JsonObject step = object(value, "SupportMarketDataBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportMarketDataBean step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload");
        requireFields(payload, "symbol", "volume");
        if (!expectedSymbol.equals(string(payload, "symbol"))
                || longInteger(payload.get("volume"), "volume") != expectedVolume) {
            throw new IllegalArgumentException("SupportMarketDataBean payload is not pinned for "
                    + caseName);
        }
    }

    /**
     * Every snapshot step pins its statement, its mode and its projection
     * list. No snapshot of this slice is count-only, so every step carries the
     * non-empty field list its iterator assertion reads; ordered steps pin
     * exact order and any-mode steps leave the order the Java suite does not
     * assert unconstrained.
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

    /** Ord 45's module-scoped undeploys (undeployModuleContaining in the suite). */
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
