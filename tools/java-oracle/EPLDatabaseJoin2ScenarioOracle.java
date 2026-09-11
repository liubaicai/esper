import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonDBRef;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.epl.historical.database.connection.SupportDatabaseURL;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanTwo;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.util.SupportDatabaseService;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.Iterator;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the database-slices differential chain (work
 * unit 4.378, epl-database-join-2): five later deterministic executions of
 * EPLDatabaseJoin (ordinals 3, 20, 16, 8 and 4) over the mytesttable MySQL
 * fixture. The earlier slice (work unit 4.377, epl-database-join) covered
 * ordinals 1, 19, 15, 17 and 2; the remaining executions of the suite (the
 * connection smoke test, the three time-batch variants, the six
 * invalid-compile cases and the restart loop) are still later slices.
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
 * SupportBeanTwo, SupportBean_A — the InputEvent schema of
 * join-index-null-type is created by the statement text itself) and the
 * 'MyDBWithRetain' database reference as a DriverManagerConnection
 * (DRIVER/FULLURL/SupportDatabaseURL.newProperties()) with
 * ConnectionLifecycleEnum RETAIN and no cache. The suite's
 * setEnableJDBC(true)/setEnableQueryPlan(true) logging flags and the
 * unrelated myvariableOCC/myvariableIPC/myvariableORC variables and other
 * database references are dropped: they are not named by these executions and
 * have no recorded effect. Internal timer off, epoch initialization 0.
 *
 * 2historical-star-inner (ordinal 3) deploys the byte-exact two-historical
 * INNER-join statement (DOUBLE spaces around each "inner join" — the suite
 * segments carry trailing and leading spaces — and a TRAILING space after the
 * final "as s2.myvarchar=s0.theString "), asserts the empty iterator
 * in-process, replays the three negative sends ("E1",1), ("A",1), ("A",10)
 * whose <> conditions exclude every theString-matching row (recorded as one
 * count negative-sends=3 after the in-process listener-not-invoked check),
 * the ("B",3) delivery {a:"B", b:3, c:"B", d:"B"} (asserted in-process,
 * recorded as the listener row), and the ("D",4) negative send (count
 * negative-sends=1). join-index-null-type (ordinal 20) deploys the
 * create-schema + s0 pair (single compile, statements separated by '\n') and
 * sends an EMPTY InputEvent map event: the null-typed fieldTypeNull indexed
 * key matches nothing in the mybigint unique index and the where-clause
 * "s1.mybigint is null" therefore never sees a row — the listener is asserted
 * not invoked in-process (count capture-empty=0). with-pattern (ordinal 16)
 * advances to 0 before the deploy, then advances 5000 (delivery
 * {mychar:"Y"}), 9999 (no delivery — recorded as count silent-advances=1) and
 * 10000 (delivery {mychar:"Y"}), and finally replays the suite's
 * "with variable" part: the @public create variable long VarLastTimestamp = 0
 * path deployment and the '@Name('Poll every 5 seconds')' statement deployed
 * from its parsed object model (in-process, no listener, no records; the
 * internal timer is off so the interval never fires).
 * variables (ordinal 8) builds the RegressionPath analog: the @public create
 * variable int queryvar and "on SupportBean set queryvar=intPrimitive" path
 * deployments, v1 with the historical stream leading (DOUBLE space after
 * "from"), SB(intPrimitive=5) then SupportBean_A("A1") delivering
 * {myint:50}, undeployModuleContaining("s0"), then v2 with the stream order
 * reversed (SINGLE space after "from"), SB(6) then SupportBean_A("A1")
 * delivering {myint:60}. 3stream (ordinal 4) deploys the byte-exact
 * select-* statement (the suite's segments leave THREE spaces before
 * "where"), asserts isStatelessSelect false in-process, and replays:
 * SupportBeanTwo("T1",2) + SupportBean("T1",-1) with no delivery (no myint=2
 * row), SupportBeanTwo("T2",30) + SupportBean("T2",-1) delivering
 * {theString:"T2", stringTwo:"T2", myint:30}, the suite's milestone(0)
 * replayed as the documented no-op it is in RegressionEnvironmentEsper, and
 * SupportBean("T3",-1) + SupportBeanTwo("T3",40) delivering
 * {theString:"T3", stringTwo:"T3", myint:40} — the T3 send order proves the
 * historical is re-polled on the second stream's trigger cycle, which is the
 * semantics Go's unrestricted FromHistorical must reproduce.
 *
 * Record protocol: one record per listener delivery {case,
 * operation:"listener", statement:"s0", sequence, time, new:[rows]} where
 * rows are {kind:"row", fields:{...}} objects projected under the
 * select-clause output names (a/b/c/d for 2historical-star-inner, mychar for
 * with-pattern, myint for variables, theString/stringTwo/myint for 3stream —
 * the exact names the Java assertions read, with the 3stream field paths
 * sb.theString/sbt.stringTwo/s1.myint projected under their short names),
 * with field names sorted alphabetically for human-diffable traces (the
 * canonical comparator is order-insensitive). Value forms, both sides
 * matching because the Go side feeds the identical canonical 10-row fixture
 * through a function-fed HistoricalProvider (no database driver): ints as
 * JSON numbers (b carries the trigger intPrimitive; myint carries the JDBC
 * integer) and strings as JSON strings (MySQL strips CHAR(20) trailing
 * spaces). Count records are {case, operation:"count", statement:"flow",
 * sequence, time, name, count} mirroring the dataflow-oracle convention
 * (negative-sends, capture-empty, silent-advances). Sequence is case-local
 * restarting at 1; the time is the fixed epoch 1970-01-01T00:00:00Z. Each
 * case runs on its own fresh runtime (internal timer off, epoch
 * initialization 0, undeployAll after the case body, destroy in finally) with
 * the runtime URI derived from the pinned java-runtime id. The 11 records
 * (3 + 1 + 3 + 2 + 2) carry the database-join-2 differential chain for the
 * five Go case builders.
 */
public final class EPLDatabaseJoin2ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-database-join-2";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java";
    private static final String CASE_2HISTORICAL_STAR_INNER = "2historical-star-inner";
    private static final String CASE_JOIN_INDEX_NULL_TYPE = "join-index-null-type";
    private static final String CASE_WITH_PATTERN = "with-pattern";
    private static final String CASE_VARIABLES = "variables";
    private static final String CASE_3STREAM = "3stream";
    private static final String[] CASES = {
            CASE_2HISTORICAL_STAR_INNER, CASE_JOIN_INDEX_NULL_TYPE, CASE_WITH_PATTERN,
            CASE_VARIABLES, CASE_3STREAM};
    private static final int[] ORDINALS = {3, 20, 16, 8, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-8961178ee540998a99f3",
            "java-runtime-a8180e298e71e8135b38",
            "java-runtime-3ddf3346fbe67a639057",
            "java-runtime-0a048d0e0005df5f6279",
            "java-runtime-fc8c20664fc4787fc987"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDatabase2HistoricalStarInner",
            "EPLDatabaseJoinIndexNullType",
            "EPLDatabaseWithPattern",
            "EPLDatabaseVariables",
            "EPLDatabase3Stream"};
    private static final int[] RECORD_COUNTS = {3, 1, 3, 2, 2};
    private static final int TOTAL_RECORDS = 11;
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact statements from EPLDatabaseJoin, assembled with the suite's
    // own concatenations.

    // EPLDatabase2HistoricalStarInner (suite lines 134-141): the DOUBLE
    // spaces around each "inner join" come from the segment trailing/leading
    // spaces and the statement ends with a TRAILING space after "s0.theString ".
    private static final String TWO_HISTORICAL_STAR_INNER_EPL =
            "@name('s0') select theString as a, intPrimitive as b, s1.myvarchar as c, s2.myvarchar as d from " +
            "SupportBean#keepall as s0 " +
            " inner join " +
            " sql:MyDBWithRetain ['select myvarchar from mytesttable where ${intPrimitive} <> mytesttable.mybigint'] as s1 " +
            " on s1.myvarchar=s0.theString " +
            " inner join " +
            " sql:MyDBWithRetain ['select myvarchar from mytesttable where ${intPrimitive} <> mytesttable.myint'] as s2 " +
            " on s2.myvarchar=s0.theString ";

    // EPLDatabaseJoinIndexNullType (suite lines 368-371): one compile with
    // the create-schema statement and s0 separated by '\n'.
    private static final String JOIN_INDEX_NULL_TYPE_EPL =
            "@public @buseventtype create schema InputEvent(id string, fieldTypeNull null);\n" +
            "@name('s0') select mybigint from InputEvent#unique(id) as s0," +
            " sql:MyDBWithRetain ['select mybigint from mytesttable where ${fieldTypeNull} = mytesttable.mybigint'] as s1" +
            " where s1.mybigint is null";

    // EPLDatabaseWithPattern (suite lines 325-327): DOUBLE space after
    // "from" ("from " + " sql:") and a SINGLE space after "as s0,"
    // (the pattern segment starts with one space).
    private static final String WITH_PATTERN_EPL =
            "@name('s0') select mychar from " +
            " sql:MyDBWithRetain ['select mychar from mytesttable where mytesttable.mybigint = 2'] as s0," +
            " pattern [every timer:interval(5 sec) ]";

    // EPLDatabaseWithPattern's "with variable" part (suite lines 340-344):
    // the path variable and the poll statement deployed from its model.
    private static final String WITH_PATTERN_VARIABLE_CREATE =
            "@public create variable long VarLastTimestamp = 0";
    private static final String WITH_PATTERN_VARIABLE_EPL =
            "@Name('Poll every 5 seconds') insert into PollStream" +
            " select * from pattern[every timer:interval(5 sec)]," +
            " sql:MyDBWithRetain ['select mychar from mytesttable where mytesttable.mybigint > ${VarLastTimestamp}'] as s0";

    // EPLDatabaseVariables (suite lines 164-168 and 177-179): v1 has the
    // historical stream leading (DOUBLE space after "from"); v2 reverses the
    // stream order (SINGLE space after "from").
    private static final String VARIABLES_VARIABLE_CREATE = "@public create variable int queryvar";
    private static final String VARIABLES_ON_SET = "on SupportBean set queryvar=intPrimitive";
    private static final String VARIABLES_EPL_V1 =
            "@name('s0') select myint from " +
            " sql:MyDBWithRetain ['select myint from mytesttable where ${queryvar} = mytesttable.mybigint'] as s0, " +
            "SupportBean_A#keepall as s1";
    private static final String VARIABLES_EPL_V2 =
            "@name('s0') select myint from " +
            "SupportBean_A#keepall as s1, " +
            "sql:MyDBWithRetain ['select myint from mytesttable where ${queryvar} = mytesttable.mybigint'] as s0";

    // EPLDatabase3Stream (suite lines 69-71): the segments leave THREE
    // spaces before "where" ("as s1 " + "  where").
    private static final String THREE_STREAM_EPL =
            "@name('s0') select * from SupportBean#lastevent sb, SupportBeanTwo#lastevent sbt, " +
            "sql:MyDBWithRetain ['select myint from mytesttable'] as s1 " +
            "  where sb.theString = sbt.stringTwo and s1.myint = sbt.intPrimitiveTwo";

    // Output projection per case: {record field name, event property path}
    // pairs. The 3stream select-* row is read through the stream-qualified
    // paths the Java assertions use and recorded under the short names.
    private static final Column[] INNER_COLUMNS = {
            new Column("a", "a"), new Column("b", "b"), new Column("c", "c"), new Column("d", "d")};
    private static final Column[] MYCHAR_COLUMNS = {new Column("mychar", "mychar")};
    private static final Column[] MYINT_COLUMNS = {new Column("myint", "myint")};
    private static final Column[] THREE_STREAM_COLUMNS = {
            new Column("theString", "sb.theString"),
            new Column("stringTwo", "sbt.stringTwo"),
            new Column("myint", "s1.myint")};

    // The differential protocol null marker: the canonical form both the
    // committed Java oracle convention and the Go NormalizeResults renderer
    // emit for null property values (kept for completeness; the five cases
    // deliver no NULL columns).
    private static final JsonValue NULL_MARKER = new JsonObject().add("state", "null");

    private EPLDatabaseJoin2ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLDatabaseJoin2ScenarioOracle <scenario.json>");
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
     * flags, its unrelated variables and its other database references are
     * not named by these executions and have no recorded effect). Internal
     * timer off.
     */
    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanTwo.class);
        configuration.getCommon().addEventType(SupportBean_A.class);
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
                "parity-epl-database-join-2-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            switch (caseIndex) {
                case 0:
                    runTwoHistoricalStarInner(runtime, configuration, writer);
                    break;
                case 1:
                    runJoinIndexNullType(runtime, configuration, writer);
                    break;
                case 2:
                    runWithPattern(runtime, configuration, writer);
                    break;
                case 3:
                    runVariables(runtime, configuration, writer);
                    break;
                default:
                    runThreeStream(runtime, configuration, writer);
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
     * EPLDatabaseJoin EPLDatabase2HistoricalStarInner (ordinal 3): two
     * historical sides INNER-joined against a keepall trigger with <> lookup
     * conditions — the three negative sends match nothing, the ("B",3) send
     * delivers one row and the ("D",4) send matches nothing.
     */
    private static void runTwoHistoricalStarInner(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 134-142: deploy and add the listener.
        EPStatement statement = requireStatement(
                compileDeploy(runtime, configuration, TWO_HISTORICAL_STAR_INNER_EPL, null), "s0");
        RowListener listener = new RowListener(writer, INNER_COLUMNS);
        statement.addListener(listener);

        // suite line 144: the empty iterator before the first trigger
        // (in-process; no record).
        assertIteratorRows(statement, INNER_COLUMNS, new Object[0][]);

        // suite lines 146-149: the three negative sends — the <> conditions
        // exclude every theString-matching row ("E1" never matches any
        // myvarchar; intPrimitive=1 excludes the mybigint-1 "A" row from s1;
        // intPrimitive=10 excludes the myint-10 "A" row from s2), so each
        // inner join is empty — asserted not-invoked in-process, recorded as
        // one count.
        sendSupportBean(runtime, "E1", 1);
        sendSupportBean(runtime, "A", 1);
        sendSupportBean(runtime, "A", 10);
        if (listener.deliveries() != 0) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the three negative sends, expected 0");
        }
        writer.addCount("flow", "negative-sends", 3);

        // suite lines 151-152: ("B",3) delivers one row (s1 keeps the
        // mybigint-2 "B" row, s2 keeps every row) asserted in-process and
        // recorded as the listener row.
        sendSupportBean(runtime, "B", 3);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the (\"B\",3) send, expected 1");
        }
        assertEquals("B", listener.lastFirstNew().get("a"));
        assertEquals(3, listener.lastFirstNew().get("b"));
        assertEquals("B", listener.lastFirstNew().get("c"));
        assertEquals("B", listener.lastFirstNew().get("d"));

        // suite lines 154-155: ("D",4) excludes the mybigint-4 "D" row from
        // s1 — asserted not-invoked in-process, recorded as one count.
        sendSupportBean(runtime, "D", 4);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the (\"D\",4) send, expected 1");
        }
        writer.addCount("flow", "negative-sends", 1);

        // suite line 157: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabaseJoinIndexNullType (ordinal 20): the
     * null-typed indexed key ${fieldTypeNull} of an EMPTY InputEvent map
     * event matches nothing, so the "s1.mybigint is null" filter never sees a
     * row and the listener is not invoked.
     */
    private static void runJoinIndexNullType(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 368-372: deploy the create-schema + s0 pair and add
        // the listener.
        EPStatement statement = requireStatement(
                compileDeploy(runtime, configuration, JOIN_INDEX_NULL_TYPE_EPL, null), "s0");
        RowListener listener = new RowListener(writer, MYINT_COLUMNS);
        statement.addListener(listener);

        // suite line 374: send the empty map event — the null key matches
        // nothing — and assert the listener not invoked in-process.
        runtime.getEventService().sendEventMap(Collections.<String, Object>emptyMap(), "InputEvent");
        if (listener.deliveries() != 0) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the empty InputEvent send, expected 0");
        }
        writer.addCount("flow", "capture-empty", 0);

        // suite line 377: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabaseWithPattern (ordinal 16): the pattern
     * timer:interval(5 sec) trigger polls the fixed mybigint-2 lookup at
     * 5000 and 10000, with the 9999 advance silent, followed by the suite's
     * "with variable" part replayed in-process (no records).
     */
    private static void runWithPattern(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite line 323: the pre-deploy advanceTime(0).
        runtime.getEventService().advanceTime(0);

        // suite lines 325-328: deploy and add the listener.
        EPStatement statement = requireStatement(
                compileDeploy(runtime, configuration, WITH_PATTERN_EPL, null), "s0");
        RowListener listener = new RowListener(writer, MYCHAR_COLUMNS);
        statement.addListener(listener);

        // suite lines 330-331: advanceTime(5000) fires the interval and
        // polls the mybigint-2 row (mychar "Y").
        runtime.getEventService().advanceTime(5000);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the advanceTime(5000), expected 1");
        }
        assertEquals("Y", listener.lastFirstNew().get("mychar"));

        // suite lines 333-334: advanceTime(9999) is silent — asserted
        // in-process, recorded as one count.
        runtime.getEventService().advanceTime(9999);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the advanceTime(9999), expected 1");
        }
        writer.addCount("flow", "silent-advances", 1);

        // suite lines 336-337: advanceTime(10000) fires the second interval.
        runtime.getEventService().advanceTime(10000);
        if (listener.deliveries() != 2) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the advanceTime(10000), expected 2");
        }
        assertEquals("Y", listener.lastFirstNew().get("mychar"));

        // suite lines 339-347: the "with variable" part — the path variable
        // and the poll statement deployed from its parsed object model
        // (in-process, no listener, no records; the internal timer is off so
        // the interval never fires). suite line 347's env.undeployAll()
        // happens at the case boundary.
        List<EPCompiled> path = new ArrayList<EPCompiled>();
        compileDeploy(runtime, configuration, WITH_PATTERN_VARIABLE_CREATE, path);
        EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(
                EPCompilerProvider.getCompiler().eplToModel(WITH_PATTERN_VARIABLE_EPL, configuration));
        compileDeployModel(runtime, configuration, model, path);
    }

    /**
     * EPLDatabaseJoin EPLDatabaseVariables (ordinal 8): the on-set variable
     * feeds the ${queryvar} lookup; v1 polls myint 50 with the historical
     * stream leading, then the s0 deployment is undeployed and v2 with the
     * reversed stream order polls myint 60.
     */
    private static void runVariables(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 163-165: the RegressionPath analog — the variable and
        // the on-set statement as path deployments.
        List<EPCompiled> path = new ArrayList<EPCompiled>();
        compileDeploy(runtime, configuration, VARIABLES_VARIABLE_CREATE, path);
        compileDeploy(runtime, configuration, VARIABLES_ON_SET, path);

        // suite lines 166-169: v1 with the historical stream leading.
        EPDeployment firstDeployment = compileDeploy(runtime, configuration, VARIABLES_EPL_V1, path);
        RowListener firstListener = new RowListener(writer, MYINT_COLUMNS);
        requireStatement(firstDeployment, "s0").addListener(firstListener);

        // suite lines 171-174: SB(intPrimitive=5) updates the variable,
        // SupportBean_A("A1") triggers the join polling myint 50.
        sendSupportBeanInt(runtime, 5);
        sendSupportBeanA(runtime, "A1");
        if (firstListener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + firstListener.deliveries()
                    + " times after the first SupportBean_A send, expected 1");
        }
        assertEquals(50, firstListener.lastFirstNew().get("myint"));

        // suite line 175: undeployModuleContaining("s0").
        undeployModuleContaining(runtime, "s0");

        // suite lines 177-180: v2 with the stream order reversed.
        EPDeployment secondDeployment = compileDeploy(runtime, configuration, VARIABLES_EPL_V2, path);
        RowListener secondListener = new RowListener(writer, MYINT_COLUMNS);
        requireStatement(secondDeployment, "s0").addListener(secondListener);

        // suite lines 182-185: SB(6) updates the variable,
        // SupportBean_A("A1") triggers the join polling myint 60.
        sendSupportBeanInt(runtime, 6);
        sendSupportBeanA(runtime, "A1");
        if (secondListener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + secondListener.deliveries()
                    + " times after the second SupportBean_A send, expected 1");
        }
        assertEquals(60, secondListener.lastFirstNew().get("myint"));

        // suite line 187: undeployAll happens at the case boundary.
    }

    /**
     * EPLDatabaseJoin EPLDatabase3Stream (ordinal 4): two lastevent streams
     * joined against the unrestricted myint historical; the T3 send order
     * proves the historical is re-polled on the second stream's trigger
     * cycle.
     */
    private static void runThreeStream(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 69-72: deploy and add the listener.
        EPStatement statement = requireStatement(
                compileDeploy(runtime, configuration, THREE_STREAM_EPL, null), "s0");

        // suite line 73: assertStatelessStmt(env, "s0", false).
        assertStatelessSelect(statement, false);

        RowListener listener = new RowListener(writer, THREE_STREAM_COLUMNS);
        statement.addListener(listener);

        // suite lines 75-76: SupportBeanTwo("T1",2) + SupportBean("T1",-1)
        // — no myint=2 row exists, so no delivery.
        sendSupportBeanTwo(runtime, "T1", 2);
        sendSupportBean(runtime, "T1", -1);
        if (listener.deliveries() != 0) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the T1 sends, expected 0");
        }

        // suite lines 78-80: SupportBeanTwo("T2",30) + SupportBean("T2",-1)
        // delivers the myint-30 row.
        sendSupportBeanTwo(runtime, "T2", 30);
        sendSupportBean(runtime, "T2", -1);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the T2 sends, expected 1");
        }
        assertEquals("T2", listener.lastFirstNew().get("sb.theString"));
        assertEquals("T2", listener.lastFirstNew().get("sbt.stringTwo"));
        assertEquals(30, listener.lastFirstNew().get("s1.myint"));

        // suite line 82: milestone(0) — RegressionEnvironmentEsper.milestone
        // is a documented no-op, replayed as one.

        // suite lines 84-86: SupportBean("T3",-1) first (the join still sees
        // sbt=T2 and delivers nothing), then SupportBeanTwo("T3",40) — the
        // trigger cycle re-polls the unrestricted historical and delivers
        // the myint-40 row.
        sendSupportBean(runtime, "T3", -1);
        sendSupportBeanTwo(runtime, "T3", 40);
        if (listener.deliveries() != 2) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the T3 sends, expected 2");
        }
        assertEquals("T3", listener.lastFirstNew().get("sb.theString"));
        assertEquals("T3", listener.lastFirstNew().get("sbt.stringTwo"));
        assertEquals(40, listener.lastFirstNew().get("s1.myint"));

        // suite line 88: undeployAll happens at the case boundary.
    }

    /**
     * Mirrors SupportAdminUtil.assertStatelessStmt: the statement context's
     * isStatelessSelect must equal the expected flag. Never recorded.
     */
    private static void assertStatelessSelect(EPStatement statement, boolean expected) {
        EPStatementSPI spi = (EPStatementSPI) statement;
        if (spi.getStatementContext().isStatelessSelect() != expected) {
            throw new IllegalStateException("isStatelessSelect was "
                    + spi.getStatementContext().isStatelessSelect() + ", expected " + expected);
        }
    }

    /**
     * Mirrors EPLDatabaseJoin's env.assertPropsPerRowIterator reads: the
     * ordered iterator rows over the case columns are compared in full.
     * Never recorded.
     */
    private static void assertIteratorRows(EPStatement statement, Column[] columns, Object[][] expected)
            throws Exception {
        List<Object[]> actual = new ArrayList<Object[]>();
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            EventBean event = it.next();
            Object[] row = new Object[columns.length];
            for (int i = 0; i < columns.length; i++) {
                row[i] = event.get(columns[i].path);
            }
            actual.add(row);
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

    private static void sendSupportBean(EPRuntime runtime, String theString, int intPrimitive) {
        SupportBean bean = new SupportBean(theString, intPrimitive);
        runtime.getEventService().sendEventBean(bean, SupportBean.class.getSimpleName());
    }

    private static void sendSupportBeanInt(EPRuntime runtime, int intPrimitive) {
        SupportBean bean = new SupportBean();
        bean.setIntPrimitive(intPrimitive);
        runtime.getEventService().sendEventBean(bean, SupportBean.class.getSimpleName());
    }

    private static void sendSupportBeanA(EPRuntime runtime, String id) {
        runtime.getEventService().sendEventBean(new SupportBean_A(id), SupportBean_A.class.getSimpleName());
    }

    private static void sendSupportBeanTwo(EPRuntime runtime, String stringTwo, int intPrimitiveTwo) {
        SupportBeanTwo bean = new SupportBeanTwo(stringTwo, intPrimitiveTwo);
        runtime.getEventService().sendEventBean(bean, SupportBeanTwo.class.getSimpleName());
    }

    /**
     * Mirrors RegressionEnvironment.compileDeploy(epl, path) without the
     * listener: compile (with the path's compiled modules exported), deploy,
     * and add the compiled module to the path when one is given.
     */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration, String epl,
                                              List<EPCompiled> path) throws Exception {
        CompilerArguments args = new CompilerArguments(configuration);
        if (path != null) {
            args.getPath().getCompileds().addAll(path);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, args);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        if (path != null) {
            path.add(compiled);
        }
        return deployment;
    }

    /**
     * Mirrors RegressionEnvironment.compileDeploy(model, path): the model is
     * compiled as a single-item module over the path and deployed.
     */
    private static EPDeployment compileDeployModel(EPRuntime runtime, Configuration configuration,
                                                   EPStatementObjectModel model, List<EPCompiled> path)
            throws Exception {
        CompilerArguments args = new CompilerArguments(configuration);
        if (path != null) {
            args.getPath().getCompileds().addAll(path);
        }
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, args);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        if (path != null) {
            path.add(compiled);
        }
        return deployment;
    }

    /**
     * Mirrors RegressionEnvironment.undeployModuleContaining: undeploys the
     * deployment whose statement list contains the named statement.
     */
    private static void undeployModuleContaining(EPRuntime runtime, String statementName) throws Exception {
        for (String deploymentId : runtime.getDeploymentService().getDeployments()) {
            EPDeployment deployment = runtime.getDeploymentService().getDeployment(deploymentId);
            for (EPStatement candidate : deployment.getStatements()) {
                if (candidate.getName().equals(statementName)) {
                    runtime.getDeploymentService().undeploy(deploymentId);
                    return;
                }
            }
        }
        throw new IllegalStateException("Failed to find deployment with statement '" + statementName + "'");
    }

    private static EPStatement requireStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (candidate.getName().equals(name)) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " not found");
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
     * Projects one delivered row under the record field names (read through
     * the event property paths) with the fields sorted alphabetically (the
     * canonical comparator is order-insensitive; sorted keys keep the traces
     * human-diffable).
     */
    private static JsonObject row(Column[] columns, EventBean event) {
        Column[] sorted = columns.clone();
        Arrays.sort(sorted);
        JsonObject fields = new JsonObject();
        for (Column column : sorted) {
            fields.add(column.name, canonicalValue(column.name, event.get(column.path)));
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
     * the not-invoked/silent observables (statement 'flow', the
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

        private void addListenerRecord(EventBean[] newEvents, String statementName, Column[] columns) {
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
     * One projected output field: the record field name and the event
     * property path it is read from (equal for the named projections,
     * stream-qualified for the 3stream select-* row).
     */
    private static final class Column implements Comparable<Column> {
        private final String name;
        private final String path;

        private Column(String name, String path) {
            this.name = name;
            this.path = path;
        }

        public int compareTo(Column other) {
            return name.compareTo(other.name);
        }
    }

    /**
     * Records one trace record per listener delivery (new-data batches only —
     * the five executions never deliver an old stream) and keeps the
     * delivery count plus the first new event of the last delivery for the
     * in-process value checks.
     */
    private static final class RowListener implements UpdateListener {
        private final TraceWriter writer;
        private final Column[] columns;
        private int deliveries;
        private EventBean lastFirstNew;

        private RowListener(TraceWriter writer, Column[] columns) {
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
