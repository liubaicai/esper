import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonDBRef;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.historical.database.connection.SupportDatabaseURL;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
 * unit 4.380, epl-database-timebatch): the EPLDatabaseTimeBatch execution of
 * EPLDatabaseJoin (ordinal 5, the first of the three time-batch variants) over
 * the mytesttable MySQL fixture. The sibling EPLDatabaseTimeBatchOM and
 * EPLDatabaseTimeBatchCompile executions replay the identical shared runner
 * over a SODA object model and an eplToModel round-trip respectively and stay
 * implemented-not-differential pending the Go compile surface; this slice
 * opens the time-batch differential that draft 4.379 deferred for engine-level
 * investigation of the window-release/poll interaction.
 *
 * The oracle requires the established esper-mysql mysql:8.0 Docker fixture
 * (started by the operator per docs/integration/external-services.md, loaded
 * from common/etc/regression/create_testdb.sql with the '//'-comment lines
 * stripped; the load tolerates the pre-existing mytesttable_large and
 * mytestupsert re-create errors so it is idempotent). The connection goes
 * through SupportDatabaseService's constants: DRIVER com.mysql.cj.jdbc.Driver,
 * FULLURL jdbc:mysql://localhost/test?user=root&password=password&useSSL=false.
 * Session configuration mirrors TestSuiteEPLDatabase.configure restricted to
 * this execution: the SupportBean POJO event type (the only event type it
 * names) and the 'MyDBWithRetain' database reference as a
 * DriverManagerConnection (DRIVER/FULLURL/SupportDatabaseURL.newProperties())
 * with ConnectionLifecycleEnum RETAIN and no cache. The suite's
 * setEnableJDBC(true)/setEnableQueryPlan(true) logging flags are dropped: they
 * alter log observability only and have no recorded effect. Internal timer
 * off, epoch initialization 0.
 *
 * The statement is byte-exact from EPLDatabaseTimeBatch (suite lines
 * 443-446), assembled with the suite's own ALL_FIELDS concatenation: the
 * select-leading 'from ' plus the segment-start ' sql:' leave a DOUBLE space
 * after "from"; the SQL carries a literal LF+CR ("\n\r") pair between
 * "mytesttable " and "where"; and the second stream segment appends with NO
 * space after "as s0," — "as s0,SupportBean#time_batch(10 sec) as s1".
 *
 * Replay of the shared runtestTimeBatch runner (suite lines 476-517),
 * advanced-time semantics per RegressionEnvironmentBase.advanceTime ->
 * runtime.getEventService().advanceTime: the env.advanceTime(0) after the
 * deploy; the initial iterator read over myint is empty (in-process, no
 * record); sendEvent SB(intPrimitive=10) — the historical poll fires at send
 * time and the joined row (mybigint 10, myint 100) is ITERATOR-VISIBLE
 * immediately while the triggering event sits in the time_batch window (count
 * record iterator-rows=1, in-process ordered compare [100]); sendEvent SB(5)
 * (count iterator-rows=2, in-process [100,50]); the suite's milestone(0)
 * replayed as the documented no-op it is in RegressionEnvironmentBase;
 * sendEvent SB(2) (count iterator-rows=3, in-process [100,50,20]);
 * advanceTime(10000) — TimeBatchView.sendBatch releases the batch of three
 * trigger events into the join and the flush delivers exactly the three
 * joined rows [100,50,20] to the listener as ONE new-data delivery recorded
 * as the listener record (the first release carries a NULL old-data array
 * because TimeBatchView posts the prior batch as old data and lastBatch is
 * still empty — asserted in-process); each flushed row is asserted in full
 * against the seed values with BigDecimal equals (row mybigint 10:
 * 10,100,"J","P",true,null,1000,10.2,10.3 — the NULL mynumeric stays
 * Java-null in-process; row mybigint 5: 5,50,"E","V",false,500,500,5.2,5.3;
 * row mybigint 2: 2,20,"B","Y",false,100,200,2.2,2.3); the iterator is then
 * EMPTY because the released window cleared (count iterator-rows=0, in-process
 * empty assert); sendEvent SB(9) re-fires the poll and the joined row is
 * iterator-visible again (count iterator-rows=1, in-process [90]); sendEvent
 * SB(8) (count iterator-rows=2, in-process [90,80]) — no further release
 * happens because the internal timer is off and time never advances again, so
 * the two sends stay batched and the listener stays silent.
 *
 * Record protocol: one record per listener delivery {case,
 * operation:"listener", statement:"s0", sequence, time, new:[rows]} where
 * rows are {kind:"row", fields:{...}} objects projected under the canonical
 * output names (mybigint..myreal) with field names sorted alphabetically for
 * human-diffable traces (the canonical comparator is order-insensitive).
 * Value forms, both sides matching because the Go side feeds the identical
 * canonical 10-row fixture through a function-fed HistoricalProvider (no
 * database driver): mybigint as JSON number (int64), myint as JSON number,
 * myvarchar/mychar as JSON strings (MySQL strips CHAR(20) trailing spaces),
 * mybool as JSON true/false, mynumeric/mydecimal as JSON STRINGS of the
 * scale-0 BigDecimal (asserted scale 0 in-process, e.g. "1000"/"500"/"100" and
 * "500"/"200"), mydouble/myreal as JSON numbers, and NULL column values as the
 * differential protocol null marker object {"state":"null"} — the canonical
 * null form both the committed Java oracle convention and the Go
 * NormalizeResults renderer emit. The NULL mynumeric columns belong to seed
 * rows 7-10 and row mybigint 10 is among the flushed rows, so the listener
 * record carries exactly one null marker (row 10's mynumeric); rows 5 and 2
 * carry the non-null scale-0 strings. Count records are {case,
 * operation:"count", statement:"flow", sequence, time, name, count} mirroring
 * the dataflow-oracle convention (iterator-rows). Sequence is case-local
 * restarting at 1; the time is the fixed epoch 1970-01-01T00:00:00Z. The case
 * runs on its own fresh runtime (internal timer off, epoch initialization 0,
 * undeployAll after the case body, destroy in finally) with the runtime URI
 * derived from the pinned java-runtime id. The 7 records (6 counts + 1
 * listener delivery of 3 rows) carry the database-timebatch differential
 * chain for the Go case builder.
 */
public final class EPLDatabaseTimeBatchScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-database-timebatch";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java";
    private static final String CASE_TIMEBATCH = "timebatch";
    private static final int ORDINAL = 5;
    private static final String RUNTIME_ID = "java-runtime-fb0cea6fe1e469ee8237";
    private static final String EXECUTION_NAME = "EPLDatabaseTimeBatch";
    private static final int TOTAL_RECORDS = 7;
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // The suite's shared ALL_FIELDS projection list (suite line 39).
    private static final String ALL_FIELDS =
            "mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal";

    // Byte-exact statement from EPLDatabaseTimeBatch (suite lines 443-446),
    // assembled with the suite's own concatenations: DOUBLE space after
    // "from" ("from " + " sql:"), a literal LF+CR pair inside the SQL, and NO
    // space after "as s0," before the SupportBean segment.
    private static final String TIME_BATCH_EPL =
            "@name('s0') select " + ALL_FIELDS + " from " +
            " sql:MyDBWithRetain ['select " + ALL_FIELDS + " from mytesttable \n\r where ${intPrimitive} = mytesttable.mybigint'] as s0," +
            "SupportBean#time_batch(10 sec) as s1";

    // Canonical output names in select-clause order; the projection sorts
    // them alphabetically when serializing.
    private static final String[] FULL_ROW_COLUMNS = {
            "mybigint", "myint", "myvarchar", "mychar", "mybool",
            "mynumeric", "mydecimal", "mydouble", "myreal"};

    // The differential protocol null marker: the canonical form both the
    // committed Java oracle convention and the Go NormalizeResults renderer
    // emit for null property values.
    private static final JsonValue NULL_MARKER = new JsonObject().add("state", "null");

    private EPLDatabaseTimeBatchScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLDatabaseTimeBatchScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        runCase(records);
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

        JsonValue runtimeValue = scenario.get("javaRuntimes");
        if (runtimeValue == null || !runtimeValue.isArray() || runtimeValue.asArray().size() != 1
                || !RUNTIME_ID.equals(runtimeValue.asArray().get(0).asString())) {
            throw new IllegalArgumentException("javaRuntimes must contain exactly " + RUNTIME_ID);
        }

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly the timebatch case");
        }
        JsonObject definition = object(caseValue.asArray().get(0), "case definition 0");
        if (!CASE_TIMEBATCH.equals(definition.getString("case", ""))
                || definition.getInt("ordinal", -1) != ORDINAL
                || !RUNTIME_ID.equals(definition.getString("runtimeId", ""))
                || !EXECUTION_NAME.equals(definition.getString("executionName", ""))) {
            throw new IllegalArgumentException("case metadata mismatch");
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != 2) {
            throw new IllegalArgumentException("scenario must contain exactly 2 steps");
        }
        JsonArray steps = stepValue.asArray();
        JsonObject marker = object(steps.get(0), "case marker");
        if (!"case".equals(marker.getString("op", "")) || !CASE_TIMEBATCH.equals(marker.getString("case", ""))) {
            throw new IllegalArgumentException("steps must open with the timebatch case marker");
        }
        JsonObject advance = object(steps.get(1), "advance-time");
        if (!"advance-time".equals(advance.getString("op", "")) || !EPOCH.equals(advance.getString("at", ""))) {
            throw new IllegalArgumentException("the scenario must pin the epoch advance-time step");
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
     * this execution: the SupportBean POJO event type and the MyDBWithRetain
     * database reference as a DriverManagerConnection over the
     * SupportDatabaseService constants with the RETAIN connection lifecycle
     * and no cache (the suite's setEnableJDBC/setEnableQueryPlan logging
     * flags have no recorded effect). Internal timer off.
     */
    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        ConfigurationCommonDBRef dbWithRetain = new ConfigurationCommonDBRef();
        dbWithRetain.setDriverManagerConnection(
                SupportDatabaseService.DRIVER, SupportDatabaseService.FULLURL, SupportDatabaseURL.newProperties());
        dbWithRetain.setConnectionLifecycleEnum(ConfigurationCommonDBRef.ConnectionLifecycleEnum.RETAIN);
        configuration.getCommon().addDatabaseReference("MyDBWithRetain", dbWithRetain);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        return configuration;
    }

    private static void runCase(JsonArray records) throws Exception {
        Configuration configuration = configuration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-epl-database-timebatch-" + RUNTIME_ID, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, CASE_TIMEBATCH);
            runTimeBatch(runtime, configuration, writer);
            if (writer.count() != TOTAL_RECORDS) {
                throw new IllegalStateException("case " + CASE_TIMEBATCH + " produced " + writer.count()
                        + " records, expected " + TOTAL_RECORDS);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * EPLDatabaseJoin EPLDatabaseTimeBatch (ordinal 5) over the shared
     * runtestTimeBatch runner: the historical poll fires per trigger send and
     * the joined rows are iterator-visible while the triggering events sit in
     * the time_batch window; the advanceTime(10000) boundary releases the
     * batch and delivers the three joined rows to the listener in trigger
     * order, clearing the window.
     */
    private static void runTimeBatch(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 443-448: deploy and add the listener.
        EPStatement statement = compileDeploy(runtime, configuration, TIME_BATCH_EPL);
        RowListener listener = new RowListener(writer, FULL_ROW_COLUMNS);
        statement.addListener(listener);

        // suite line 478: env.advanceTime(0) after the deploy.
        runtime.getEventService().advanceTime(0);

        // suite line 479: the iterator over myint is empty before the first
        // trigger (in-process; no record).
        assertIteratorMyInt(statement, new int[0]);

        // suite lines 481-482: SB(10) polls the mybigint-10 row; the joined
        // row is iterator-visible immediately while the trigger sits in the
        // batch window.
        sendSupportBean(runtime, 10);
        assertIteratorMyInt(statement, new int[]{100});
        writer.addCount("flow", "iterator-rows", 1);

        // suite lines 484-485: SB(5) adds the mybigint-5 row.
        sendSupportBean(runtime, 5);
        assertIteratorMyInt(statement, new int[]{100, 50});
        writer.addCount("flow", "iterator-rows", 2);

        // suite line 487: milestone(0) — RegressionEnvironmentBase.milestone
        // is a documented no-op, replayed as one.

        // suite lines 489-490: SB(2) adds the mybigint-2 row.
        sendSupportBean(runtime, 2);
        assertIteratorMyInt(statement, new int[]{100, 50, 20});
        writer.addCount("flow", "iterator-rows", 3);

        // suite lines 492-499: advanceTime(10000) releases the batch and the
        // join delivers exactly the three joined rows in trigger order — one
        // new-data delivery, recorded as the listener record above; each row
        // is asserted in full against the seed values (BigDecimal equals; the
        // NULL mynumeric of row 10 stays Java-null here and is recorded as
        // the protocol null marker).
        runtime.getEventService().advanceTime(10000);
        if (listener.deliveries() != 1) {
            throw new IllegalStateException("listener was invoked " + listener.deliveries()
                    + " times after the advanceTime(10000), expected 1");
        }
        if (listener.oldDeliveries() != 0) {
            throw new IllegalStateException("the flush delivered an old-data batch "
                    + listener.oldDeliveries() + " times, expected 0 (first release, empty lastBatch)");
        }
        EventBean[] flushed = listener.lastNew();
        if (flushed == null || flushed.length != 3) {
            throw new IllegalStateException("flush delivered "
                    + (flushed == null ? "nothing" : flushed.length + " rows") + ", expected 3 rows");
        }
        assertReceived(flushed[0], 10L, 100, "J", "P", true, null,
                new BigDecimal(1000), 10.2, 10.3);
        assertReceived(flushed[1], 5L, 50, "E", "V", false,
                new BigDecimal(500), new BigDecimal(500), 5.2, 5.3);
        assertReceived(flushed[2], 2L, 20, "B", "Y", false,
                new BigDecimal(100), new BigDecimal(200), 2.2, 2.3);

        // suite line 501: the released window is empty (in-process; the count
        // record pins the size).
        assertIteratorMyInt(statement, new int[0]);
        writer.addCount("flow", "iterator-rows", 0);

        // suite lines 503-504: SB(9) re-fires the poll; the joined row is
        // iterator-visible while batched (no release happens — the internal
        // timer is off and time never advances again).
        sendSupportBean(runtime, 9);
        assertIteratorMyInt(statement, new int[]{90});
        writer.addCount("flow", "iterator-rows", 1);

        // suite lines 506-507: SB(8) adds the mybigint-8 row.
        sendSupportBean(runtime, 8);
        assertIteratorMyInt(statement, new int[]{90, 80});
        writer.addCount("flow", "iterator-rows", 2);

        // suite line 509: undeployAll happens at the case boundary.
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
     * Mirrors runtestTimeBatch's env.assertPropsPerRowIterator("s0", myint)
     * reads: the ordered iterator rows over myint are compared in full. Never
     * recorded (the count records pin the sizes).
     */
    private static void assertIteratorMyInt(EPStatement statement, int[] expected) throws Exception {
        List<Integer> actual = new ArrayList<Integer>();
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            EventBean event = it.next();
            Object value = event.get("myint");
            if (!(value instanceof Integer)) {
                throw new IllegalStateException("iterator myint carried "
                        + (value == null ? "null" : value.getClass().getName()) + ", expected Integer");
            }
            actual.add((Integer) value);
        }
        if (actual.size() != expected.length) {
            throw new IllegalStateException("iterator held " + actual.size() + " rows, expected " + expected.length);
        }
        for (int i = 0; i < expected.length; i++) {
            if (actual.get(i).intValue() != expected[i]) {
                throw new IllegalStateException("iterator row " + i + " myint was " + actual.get(i)
                        + ", expected " + expected[i]);
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
     * the iterator observables (statement 'flow', the dataflow-oracle
     * convention); the sequence is case-local.
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
     * Records one trace record per new-data listener delivery and keeps the
     * delivery counters plus the last new-data array for the flush asserts
     * (the single time_batch release posts new data only — the first release
     * has an empty lastBatch, asserted via the old-delivery counter).
     */
    private static final class RowListener implements UpdateListener {
        private final TraceWriter writer;
        private final String[] columns;
        private int deliveries;
        private int oldDeliveries;
        private EventBean[] lastNew;

        private RowListener(TraceWriter writer, String[] columns) {
            this.writer = writer;
            this.columns = columns;
        }

        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
            if (oldEvents != null && oldEvents.length > 0) {
                oldDeliveries++;
            }
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            deliveries++;
            lastNew = newEvents;
            writer.addListenerRecord(newEvents, statement.getName(), columns);
        }

        private int deliveries() {
            return deliveries;
        }

        private int oldDeliveries() {
            return oldDeliveries;
        }

        private EventBean[] lastNew() {
            return lastNew;
        }
    }
}
