import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonDBRef;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.historical.database.connection.SupportDatabaseURL;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.util.SupportDatabaseService;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Iterator;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the database-slices differential chain (work
 * unit 4.377, epl-database-join): the first five deterministic executions of
 * EPLDatabaseJoin (ordinals 1, 19, 15, 17 and 2) over the mytesttable MySQL
 * fixture. The remaining executions of the suite (the connection smoke test,
 * the 2-historical inner join, the three time-batch variants, the variables
 * case, the six invalid-compile cases, the pattern case, the restart loop and
 * the null-typed index case) are later slices.
 *
 * The oracle requires the established esper-mysql mysql:8.0 Docker fixture
 * (started by the operator per docs/integration/external-services.md, loaded
 * from common/etc/regression/create_testdb.sql with the '//'-comment lines
 * stripped; the load tolerates the pre-existing mytesttable_large and
 * mytestupsert re-create errors so it is idempotent). The connection goes
 * through SupportDatabaseService's constants: DRIVER com.mysql.cj.jdbc.Driver,
 * FULLURL jdbc:mysql://localhost/test?user=root&password=password&useSSL=false.
 * Session configuration mirrors TestSuiteEPLDatabase.configure restricted to
 * these executions: the POJO event types they name (SupportBean,
 * SupportBean_S0, SupportBeanComplexProps) and the 'MyDBWithRetain' database
 * reference as a DriverManagerConnection (DRIVER/FULLURL/
 * SupportDatabaseURL.newProperties()) with ConnectionLifecycleEnum RETAIN and
 * no cache. The suite's setEnableJDBC(true)/setEnableQueryPlan(true) logging
 * flags are dropped: they alter log observability only and have no recorded
 * effect. Internal timer off, epoch initialization 0.
 *
 * simple-join-left (ordinal 1) deploys the byte-exact 9-column projection
 * built with the suite's ALL_FIELDS concatenation — the S0 trigger stream
 * leads and the historical ${id}-keyed lookup follows — and sends
 * SupportBean_S0(1). simple-join-right (ordinal 19) reverses the stream order
 * so the historical stream leads ("from  sql:" carries a DOUBLE space and
 * "as s0,SupportBean_S0" no space); before the send it FIRST pins the
 * statement event-type property types in-process (mybigint Long, myint
 * Integer, myvarchar String, mychar String, mybool Boolean, mynumeric
 * BigDecimal, mydecimal BigDecimal, mydouble Double, myreal Double — never
 * recorded). stream-names-and-rename (ordinal 15) deploys the aliased
 * statement whose SQL renames the columns to a..i (double space after
 * "as a," before "myint") and whose select renames them back under the
 * canonical names (double spaces after "mybigint," and after "myreal");
 * the output property names pin the rename mapping in-process.
 * property-resolution (ordinal 17) deploys the ${s1.arrayProperty[0]}-keyed
 * statement joined against SupportBeanComplexProps as s1 and sends
 * SupportBeanComplexProps.makeDefaultBean() whose arrayProperty is
 * {10,20,30}, selecting the mybigint-10 row whose mynumeric column is NULL.
 * Cases 0-3 assert the single delivered row in-process against the suite's
 * assertReceived values (the BigDecimal values are asserted with equals, so a
 * scale or value deviation from the fixture fails loudly) and each deliver
 * exactly one listener record. 2historical-star (ordinal 2) deploys the
 * two-historical keepall statement (double space after "s0," and a TRAILING
 * space after "as s2 ") and replays: iterator count 0 (in-process); send
 * SB(intPrimitive=6) delivering one row {6,60,"F"}; iterator snapshot count 1
 * (count record iterator-rows=1, in-process full compare); send SB(9)
 * delivering one row {9,90,"I"}; iterator snapshot count 2 (count record
 * iterator-rows=2, in-process ordered compare [[6,60,"F"],[9,90,"I"]]); the
 * suite's milestone(0) replayed as the documented no-op it is in
 * RegressionEnvironmentEsper; send SB(20) with no matching row (mybigint
 * stops at 10) — the listener is asserted not invoked in-process (count
 * record listener-not-invoked=0) and the iterator is asserted unchanged
 * in-process.
 *
 * Record protocol: one record per listener delivery {case,
 * operation:"listener", statement:"s0", sequence, time, new:[rows]} where
 * rows are {kind:"row", fields:{...}} objects projected under the canonical
 * output names (mybigint..myreal for the four single-delivery cases,
 * intPrimitive/myint/myvarchar for 2historical-star) with field names sorted
 * alphabetically for human-diffable traces (the canonical comparator is
 * order-insensitive). Value forms, both sides matching because the Go side
 * feeds the identical canonical 10-row fixture through a function-fed
 * HistoricalProvider (no database driver): mybigint as JSON number (int64),
 * myint as JSON number, myvarchar/mychar as JSON strings (MySQL strips
 * CHAR(20) trailing spaces), mybool as JSON true/false, mynumeric/mydecimal
 * as JSON STRINGS of the scale-0 BigDecimal (asserted scale 0 in-process,
 * e.g. "5000" and "1000"), mydouble/myreal as JSON numbers (1.2..10.2 /
 * 1.3..10.3), and NULL column values as the differential protocol null marker
 * object {"state":"null"} — the canonical null form both the committed Java
 * oracle convention and the Go NormalizeResults renderer emit, so the NULL
 * mynumeric columns compare zero-difference under the structural trace diff
 * (the in-process assertion stays Java-null). Count records are {case,
 * operation:"count", statement:"flow", sequence, time, name, count} mirroring
 * the dataflow-oracle convention (iterator-rows and listener-not-invoked).
 * Sequence is case-local restarting at 1; the time is the fixed epoch
 * 1970-01-01T00:00:00Z. Each case runs on its own fresh runtime (internal
 * timer off, epoch initialization 0, undeployAll after the case body,
 * destroy in finally) with the runtime URI derived from the pinned
 * java-runtime id. The 9 records (1 + 1 + 1 + 1 + 5) carry the
 * database-join differential chain for the five Go case builders.
 */
public final class EPLDatabaseJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-database-join";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java";
    private static final String CASE_SIMPLE_JOIN_LEFT = "simple-join-left";
    private static final String CASE_SIMPLE_JOIN_RIGHT = "simple-join-right";
    private static final String CASE_STREAM_NAMES_AND_RENAME = "stream-names-and-rename";
    private static final String CASE_PROPERTY_RESOLUTION = "property-resolution";
    private static final String CASE_TWO_HISTORICAL_STAR = "2historical-star";
    private static final String[] CASES = {
            CASE_SIMPLE_JOIN_LEFT, CASE_SIMPLE_JOIN_RIGHT, CASE_STREAM_NAMES_AND_RENAME,
            CASE_PROPERTY_RESOLUTION, CASE_TWO_HISTORICAL_STAR};
    private static final int[] ORDINALS = {1, 19, 15, 17, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-67745eb75f864ea17fed",
            "java-runtime-45e68d7cf756824c19eb",
            "java-runtime-4675598d7044c43fd088",
            "java-runtime-33cc6c0610b2c801c1bb",
            "java-runtime-a6682a71c8c463babcff"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDatabaseSimpleJoinLeft",
            "EPLDatabaseSimpleJoinRight",
            "EPLDatabaseStreamNamesAndRename",
            "EPLDatabasePropertyResolution",
            "EPLDatabase2HistoricalStar"};
    private static final int[] RECORD_COUNTS = {1, 1, 1, 1, 5};
    private static final int TOTAL_RECORDS = 9;
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // The suite's shared ALL_FIELDS projection list (suite line 39).
    private static final String ALL_FIELDS =
            "mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal";

    // Byte-exact statements from EPLDatabaseJoin, assembled with the suite's
    // own concatenations. EPLDatabaseSimpleJoinLeft (suite lines 383-385).
    private static final String SIMPLE_JOIN_LEFT_EPL =
            "@name('s0') select " + ALL_FIELDS + " from " +
            "SupportBean_S0 as s0," +
            " sql:MyDBWithRetain ['select " + ALL_FIELDS + " from mytesttable where ${id} = mytesttable.mybigint'] as s1";

    // EPLDatabaseSimpleJoinRight (suite lines 420-422): the historical stream
    // leads — "from  sql:" carries a DOUBLE space and "as s0,SupportBean_S0"
    // no space.
    private static final String SIMPLE_JOIN_RIGHT_EPL =
            "@name('s0') select " + ALL_FIELDS + " from " +
            " sql:MyDBWithRetain ['select " + ALL_FIELDS + " from mytesttable where ${id} = mytesttable.mybigint'] as s0," +
            "SupportBean_S0 as s1";

    // EPLDatabaseStreamNamesAndRename (suite lines 292-311): the SQL renames
    // the columns to a..i (double space after "as a," before "myint") and the
    // select renames them back under the canonical names (double spaces after
    // "mybigint," and after "myreal").
    private static final String STREAM_NAMES_AND_RENAME_EPL =
            "@name('s0') select s1.a as mybigint, " +
            " s1.b as myint," +
            " s1.c as myvarchar," +
            " s1.d as mychar," +
            " s1.e as mybool," +
            " s1.f as mynumeric," +
            " s1.g as mydecimal," +
            " s1.h as mydouble," +
            " s1.i as myreal " +
            " from SupportBean_S0 as s0," +
            " sql:MyDBWithRetain ['select mybigint as a, " +
            " myint as b," +
            " myvarchar as c," +
            " mychar as d," +
            " mybool as e," +
            " mynumeric as f," +
            " mydecimal as g," +
            " mydouble as h," +
            " myreal as i " +
            "from mytesttable where ${id} = mytesttable.mybigint'] as s1";

    // EPLDatabasePropertyResolution (suite lines 353-355): the nested-indexed
    // trigger property ${s1.arrayProperty[0]} keys the historical lookup.
    private static final String PROPERTY_RESOLUTION_EPL =
            "@name('s0') select " + ALL_FIELDS + " from " +
            " sql:MyDBWithRetain ['select " + ALL_FIELDS + " from mytesttable where ${s1.arrayProperty[0]} = mytesttable.mybigint'] as s0," +
            "SupportBeanComplexProps as s1";

    // EPLDatabase2HistoricalStar (suite lines 105-108): two historical sides
    // keyed on the same trigger — a DOUBLE space after "s0," and a TRAILING
    // space after "as s2 ".
    private static final String TWO_HISTORICAL_STAR_EPL =
            "@name('s0') select intPrimitive, myint, myvarchar from " +
            "SupportBean#keepall as s0, " +
            " sql:MyDBWithRetain ['select myint from mytesttable where ${intPrimitive} = mytesttable.mybigint'] as s1," +
            " sql:MyDBWithRetain ['select myvarchar from mytesttable where ${intPrimitive} = mytesttable.mybigint'] as s2 ";

    // Canonical output names in select-clause order; the projection sorts
    // them alphabetically when serializing.
    private static final String[] FULL_ROW_COLUMNS = {
            "mybigint", "myint", "myvarchar", "mychar", "mybool",
            "mynumeric", "mydecimal", "mydouble", "myreal"};
    private static final String[] STAR_COLUMNS = {"intPrimitive", "myint", "myvarchar"};

    // The differential protocol null marker: the canonical form both the
    // committed Java oracle convention and the Go NormalizeResults renderer
    // emit for null property values.
    private static final JsonValue NULL_MARKER = new JsonObject().add("state", "null");

    private EPLDatabaseJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLDatabaseJoinScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, records);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " records, got " + records.size());
        }
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT).add("java", System.getProperty("java.version")).add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(scenario.getString("javaSource", JAVA_SOURCE))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain the five selected cases");
        }
        JsonArray caseDefinitions = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[i].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != CASES.length * 2) {
            throw new IllegalArgumentException("scenario must contain exactly " + (CASES.length * 2) + " steps");
        }
        JsonArray steps = stepValue.asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(caseIndex * 2), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            JsonObject advance = object(steps.get(caseIndex * 2 + 1), "advance-time " + caseIndex);
            if (!"advance-time".equals(advance.getString("op", ""))
                    || !EPOCH.equals(advance.getString("at", ""))) {
                throw new IllegalArgumentException("each case must pin the epoch advance-time step");
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    /**
     * Session configuration per TestSuiteEPLDatabase.configure restricted to
     * these executions: the POJO event types they name and the MyDBWithRetain
     * database reference as a DriverManagerConnection over the
     * SupportDatabaseService constants with the RETAIN connection lifecycle
     * and no cache (the suite's setEnableJDBC/setEnableQueryPlan logging
     * flags have no recorded effect). Internal timer off.
     */
    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBeanComplexProps.class);
        ConfigurationCommonDBRef dbWithRetain = new ConfigurationCommonDBRef();
        dbWithRetain.setDriverManagerConnection(
                SupportDatabaseService.DRIVER, SupportDatabaseService.FULLURL, SupportDatabaseURL.newProperties());
        dbWithRetain.setConnectionLifecycleEnum(ConfigurationCommonDBRef.ConnectionLifecycleEnum.RETAIN);
        configuration.getCommon().addDatabaseReference("MyDBWithRetain", dbWithRetain);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        return configuration;
    }

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = configuration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-epl-database-join-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            switch (caseIndex) {
                case 0:
                    runSimpleJoinLeft(runtime, configuration, writer);
                    break;
                case 1:
                    runSimpleJoinRight(runtime, configuration, writer);
                    break;
                case 2:
                    runStreamNamesAndRename(runtime, configuration, writer);
                    break;
                case 3:
                    runPropertyResolution(runtime, configuration, writer);
                    break;
                default:
                    runTwoHistoricalStar(runtime, configuration, writer);
                    break;
            }
            if (writer.count() != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.count()
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * EPLDatabaseJoin EPLDatabaseSimpleJoinLeft (ordinal 1): the 9-column row
     * projection over a ${id}-keyed historical lookup — one delivery for
     * SupportBean_S0(1).
     */
    private static void runSimpleJoinLeft(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 383-390: deploy, add the listener, send S0(1).
        EPStatement statement = compileDeploy(runtime, configuration, SIMPLE_JOIN_LEFT_EPL);
        RowListener listener = new RowListener(writer, FULL_ROW_COLUMNS);
        statement.addListener(listener);
        runtime.getEventService().sendEventBean(new SupportBean_S0(1), "SupportBean_S0");

        // suite line 389: assertReceived(1, 10, "A", "Z", true, 5000, 100,
        // 1.2, 1.3) — in-process, never recorded (the listener record above
        // carries the canonical forms).
        assertReceived(listener.lastFirstNew(), 1L, 10, "A", "Z", true,
                new BigDecimal(5000), new BigDecimal(100), 1.2, 1.3);

        // suite line 391: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabaseSimpleJoinRight (ordinal 19): the stream
     * order reversed with the historical stream leading; the statement event
     * type FIRST pins the JDBC-derived property types (in-process, never
     * recorded) before the send.
     */
    private static void runSimpleJoinRight(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 420-423: deploy.
        EPStatement statement = compileDeploy(runtime, configuration, SIMPLE_JOIN_RIGHT_EPL);

        // suite lines 425-436: the event-type property types, asserted with
        // equals against the JDBC-derived classes.
        EventType eventType = statement.getEventType();
        assertEquals(Long.class, eventType.getPropertyType("mybigint"));
        assertEquals(Integer.class, eventType.getPropertyType("myint"));
        assertEquals(String.class, eventType.getPropertyType("myvarchar"));
        assertEquals(String.class, eventType.getPropertyType("mychar"));
        assertEquals(Boolean.class, eventType.getPropertyType("mybool"));
        assertEquals(BigDecimal.class, eventType.getPropertyType("mynumeric"));
        assertEquals(BigDecimal.class, eventType.getPropertyType("mydecimal"));
        assertEquals(Double.class, eventType.getPropertyType("mydouble"));
        assertEquals(Double.class, eventType.getPropertyType("myreal"));

        // suite lines 438-439: send S0(1) and assertReceived.
        RowListener listener = new RowListener(writer, FULL_ROW_COLUMNS);
        statement.addListener(listener);
        runtime.getEventService().sendEventBean(new SupportBean_S0(1), "SupportBean_S0");
        assertReceived(listener.lastFirstNew(), 1L, 10, "A", "Z", true,
                new BigDecimal(5000), new BigDecimal(100), 1.2, 1.3);

        // suite line 441: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabaseStreamNamesAndRename (ordinal 15): the
     * alias-keyed rename projects the a..i SQL columns back under the
     * canonical names; the output property names pin the rename mapping
     * in-process.
     */
    private static void runStreamNamesAndRename(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 292-312: deploy.
        EPStatement statement = compileDeploy(runtime, configuration, STREAM_NAMES_AND_RENAME_EPL);

        // the rename mapping pinned in-process: the output property names are
        // exactly the nine canonical names.
        String[] propertyNames = statement.getEventType().getPropertyNames().clone();
        Arrays.sort(propertyNames);
        String[] canonicalNames = FULL_ROW_COLUMNS.clone();
        Arrays.sort(canonicalNames);
        if (!Arrays.equals(propertyNames, canonicalNames)) {
            throw new IllegalStateException("output property names were " + Arrays.toString(propertyNames)
                    + ", expected the canonical rename mapping " + Arrays.toString(canonicalNames));
        }

        // suite lines 314-315: send S0(1) and assertReceived.
        RowListener listener = new RowListener(writer, FULL_ROW_COLUMNS);
        statement.addListener(listener);
        runtime.getEventService().sendEventBean(new SupportBean_S0(1), "SupportBean_S0");
        assertReceived(listener.lastFirstNew(), 1L, 10, "A", "Z", true,
                new BigDecimal(5000), new BigDecimal(100), 1.2, 1.3);

        // suite line 317: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabasePropertyResolution (ordinal 17): the
     * nested-indexed trigger property ${s1.arrayProperty[0]} resolves the
     * default bean's arrayProperty {10,20,30} to the mybigint-10 row whose
     * mynumeric column is NULL (recorded as the null marker, asserted
     * Java-null in-process).
     */
    private static void runPropertyResolution(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 353-359: deploy and send the default bean.
        EPStatement statement = compileDeploy(runtime, configuration, PROPERTY_RESOLUTION_EPL);
        RowListener listener = new RowListener(writer, FULL_ROW_COLUMNS);
        statement.addListener(listener);
        runtime.getEventService().sendEventBean(SupportBeanComplexProps.makeDefaultBean(), "SupportBeanComplexProps");

        // suite line 360: assertReceived(10, 100, "J", "P", true, null, 1000,
        // 10.2, 10.3) — the NULL mynumeric stays Java-null in-process.
        assertReceived(listener.lastFirstNew(), 10L, 100, "J", "P", true,
                null, new BigDecimal(1000), 10.2, 10.3);

        // suite line 362: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabase2HistoricalStar (ordinal 2): two historical
     * sides keyed on the same trigger over a keepall window, with per-trigger
     * lineage and the negative no-match send.
     */
    private static void runTwoHistoricalStar(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 105-109: deploy and add the listener.
        EPStatement statement = compileDeploy(runtime, configuration, TWO_HISTORICAL_STAR_EPL);
        RowListener listener = new RowListener(writer, STAR_COLUMNS);
        statement.addListener(listener);

        // suite line 111: the empty iterator before the first trigger
        // (in-process; no record — the count records below pin the sizes).
        assertIteratorRows(statement, new Object[0][]);

        // suite lines 113-115: SB(6) delivers {6,60,"F"} and the iterator
        // holds exactly that row.
        sendSupportBean(runtime, 6);
        assertIteratorRows(statement, new Object[][]{{6, 60, "F"}});
        writer.addCount("flow", "iterator-rows", 1);

        // suite lines 117-119: SB(9) delivers {9,90,"I"} and the iterator
        // holds both rows in order.
        sendSupportBean(runtime, 9);
        assertIteratorRows(statement, new Object[][]{{6, 60, "F"}, {9, 90, "I"}});
        writer.addCount("flow", "iterator-rows", 2);

        // suite line 121: milestone(0) — RegressionEnvironmentEsper.milestone
        // is a documented no-op, replayed as one.

        // suite lines 123-125: SB(20) has no matching row (mybigint stops at
        // 10) — the listener is asserted not invoked in-process and the
        // iterator is asserted unchanged.
        sendSupportBean(runtime, 20);
        if (listener.deliveries() != 2) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the SB(20) send, expected 2");
        }
        writer.addCount("flow", "listener-not-invoked", 0);
        assertIteratorRows(statement, new Object[][]{{6, 60, "F"}, {9, 90, "I"}});

        // suite line 127: undeployAll happens at the case boundary.
    }

    /**
     * Mirrors EPLDatabaseJoin.assertReceived: the delivered row values are
     * asserted with equals against the fixture values (the NULL mynumeric
     * stays Java-null); a scale or value deviation from the fixture fails
     * loudly. Never recorded.
     */
    private static void assertReceived(EventBean received, Long mybigint, Integer myint, String myvarchar,
                                       String mychar, Boolean mybool, BigDecimal mynumeric, BigDecimal mydecimal,
                                       Double mydouble, Double myreal) {
        if (received == null) {
            throw new IllegalStateException("no delivered row to assert");
        }
        assertEquals(mybigint, received.get("mybigint"));
        assertEquals(myint, received.get("myint"));
        assertEquals(myvarchar, received.get("myvarchar"));
        assertEquals(mychar, received.get("mychar"));
        assertEquals(mybool, received.get("mybool"));
        assertEquals(mynumeric, received.get("mynumeric"));
        assertEquals(mydecimal, received.get("mydecimal"));
        assertEquals(mydouble, received.get("mydouble"));
        assertEquals(myreal, received.get("myreal"));
    }

    /**
     * Mirrors EPLDatabaseJoin's env.assertPropsPerRowIterator reads: the
     * ordered iterator rows over intPrimitive/myint/myvarchar are compared in
     * full. Never recorded.
     */
    private static void assertIteratorRows(EPStatement statement, Object[][] expected) throws Exception {
        List<Object[]> actual = new ArrayList<Object[]>();
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            EventBean event = it.next();
            actual.add(new Object[]{event.get("intPrimitive"), event.get("myint"), event.get("myvarchar")});
        }
        if (actual.size() != expected.length) {
            throw new IllegalStateException("iterator held " + actual.size() + " rows, expected " + expected.length);
        }
        for (int i = 0; i < expected.length; i++) {
            Object[] expectedRow = expected[i];
            Object[] actualRow = actual.get(i);
            for (int j = 0; j < expectedRow.length; j++) {
                assertEquals(expectedRow[j], actualRow[j]);
            }
        }
    }

    private static void sendSupportBean(EPRuntime runtime, int intPrimitive) {
        SupportBean bean = new SupportBean();
        bean.setIntPrimitive(intPrimitive);
        runtime.getEventService().sendEventBean(bean, SupportBean.class.getSimpleName());
    }

    /**
     * Compiles and deploys the byte-exact EPL and returns the named
     * statement (RegressionEnvironment.compileDeploy().addListener analog
     * without the listener — callers attach their own).
     */
    private static EPStatement compileDeploy(EPRuntime runtime, Configuration configuration, String epl)
            throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement candidate : deployment.getStatements()) {
            if (candidate.getName().equals("s0")) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 not found");
    }

    private static void assertEquals(Object expected, Object actual) {
        if (expected == null ? actual != null : !expected.equals(actual)) {
            throw new IllegalStateException("value was " + describe(actual) + ", expected " + describe(expected));
        }
    }

    private static String describe(Object value) {
        if (value == null) {
            return "null";
        }
        return value + " (" + value.getClass().getName() + ")";
    }

    /**
     * Projects one delivered row under the canonical output names with the
     * fields sorted alphabetically (the canonical comparator is
     * order-insensitive; sorted keys keep the traces human-diffable).
     */
    private static JsonObject row(String[] columns, EventBean event) {
        String[] sorted = columns.clone();
        Arrays.sort(sorted);
        JsonObject fields = new JsonObject();
        for (String column : sorted) {
            fields.add(column, canonicalValue(column, event.get(column)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    /**
     * Canonical value forms shared with the Go function-fed provider:
     * int64/int JSON numbers, boolean JSON true/false, double JSON numbers,
     * CHAR-stripped strings, scale-0 BigDecimal as its plain string, and NULL
     * columns as the protocol null marker {"state":"null"}.
     */
    private static JsonValue canonicalValue(String column, Object value) {
        if (value == null) {
            return NULL_MARKER;
        }
        if (value instanceof Long) {
            return Json.value(((Long) value).longValue());
        }
        if (value instanceof Integer) {
            return Json.value(((Integer) value).intValue());
        }
        if (value instanceof Double) {
            return Json.value(((Double) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value(((Boolean) value).booleanValue());
        }
        if (value instanceof String) {
            return Json.value((String) value);
        }
        if (value instanceof BigDecimal) {
            BigDecimal decimal = (BigDecimal) value;
            if (decimal.scale() != 0) {
                throw new IllegalStateException("column " + column + " carried scale-" + decimal.scale()
                        + " decimal " + decimal + ", expected the scale-0 fixture decimal");
            }
            return Json.value(decimal.toPlainString());
        }
        throw new IllegalStateException("column " + column + " carried unexpected value class "
                + value.getClass().getName() + " value " + value);
    }

    /**
     * Emits {case, operation:"listener", statement, sequence, time,
     * new:[rows]} records for the listener deliveries and {case,
     * operation:"count", statement, sequence, time, name, count} records for
     * the iterator/not-invoked observables (statement 'flow', the
     * dataflow-oracle convention); the sequence is case-local.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        private void addListenerRecord(EventBean[] newEvents, String statementName, String[] columns) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statementName)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            JsonArray rows = new JsonArray();
            for (EventBean event : newEvents) {
                rows.add(row(columns, event));
            }
            record.add("new", rows);
            records.add(record);
        }

        private void addCount(String statement, String name, int count) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "count")
                    .add("statement", statement)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("count", count);
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }

    /**
     * Records one trace record per listener delivery (new-data batches only —
     * the five executions never deliver an old stream) and keeps the
     * delivery count plus the first new event of the last delivery for the
     * in-process assertReceived checks.
     */
    private static final class RowListener implements UpdateListener {
        private final TraceWriter writer;
        private final String[] columns;
        private int deliveries;
        private EventBean lastFirstNew;

        private RowListener(TraceWriter writer, String[] columns) {
            this.writer = writer;
            this.columns = columns;
        }

        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            deliveries++;
            lastFirstNew = newEvents[0];
            writer.addListenerRecord(newEvents, statement.getName(), columns);
        }

        private int deliveries() {
            return deliveries;
        }

        private EventBean lastFirstNew() {
            return lastFirstNew;
        }
    }
}
