import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonDBRef;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.historical.database.connection.SupportDatabaseURL;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;

/**
 * Direct Esper 9.0.0 oracle for the database-slices differential chain (work
 * unit 4.393, epl-database-restart): the EPLDatabaseRestartStatement
 * execution of EPLDatabaseJoin (ordinal 18) over the mytesttable MySQL
 * fixture — the restart loop that draft 4.379 deferred pending the
 * connection-lifecycle accounting on the real fixture. The earlier slices
 * covered ordinals 1, 19, 15, 17 and 2 (work unit 4.377, epl-database-join),
 * 3, 20, 16, 8 and 4 (work unit 4.378, epl-database-join-2) and the first
 * time-batch variant (work unit 4.380, epl-database-timebatch); the remaining
 * executions of the suite are covered by separate dispositions (the raw JDBC
 * connection smoke has no Esper observables, the invalid-compile family is
 * diagnostic-surface only, and the two remaining time-batch variants need the
 * Go SODA compile surface).
 *
 * The oracle requires the established esper-mysql mysql:8.0 Docker fixture
 * (started by the operator per docs/integration/external-services.md, loaded
 * from common/etc/regression/create_testdb.sql with the '//'-comment lines
 * stripped; the load tolerates the pre-existing mytesttable_large and
 * mytestupsert re-create errors so it is idempotent). The connection goes
 * through SupportDatabaseService's constants: DRIVER com.mysql.cj.jdbc.Driver,
 * FULLURL jdbc:mysql://localhost/test?user=root&password=password&useSSL=false.
 * Session configuration mirrors TestSuiteEPLDatabase.configure restricted to
 * this execution: the SupportBean_S0 POJO event type (the only event type it
 * names) and the 'MyDBWithRetain' database reference as a
 * DriverManagerConnection (DRIVER/FULLURL/SupportDatabaseURL.newProperties())
 * with ConnectionLifecycleEnum RETAIN and no cache. The suite's
 * setEnableJDBC(true)/setEnableQueryPlan(true) logging flags are dropped: they
 * alter log observability only and have no recorded effect. Internal timer
 * off, epoch initialization 0.
 *
 * The statement is byte-exact from EPLDatabaseRestartStatement (suite lines
 * 397-399, assembled with the suite's own concatenations): the S0 trigger
 * stream leads (SINGLE space after "s0," — the " sql:" segment starts with one
 * space) and the ${id}-keyed single-column mychar lookup follows.
 *
 * Replay of suite lines 400-414: the statement is compiled ONCE and the same
 * EPCompiled object is deployed once before the loop (no listener — the suite
 * deploys the initial deployment listener-less), then the 100-cycle loop runs:
 * undeployModuleContaining("s0"), sendEventS0(1) with NO statement deployed
 * (no listener can observe it and no record is possible), re-deploy the SAME
 * compiled module and addListener("s0"), sendEventS0(1) again, and the suite's
 * assertEqualsNew("s0", "mychar", "Z") replayed in-process as exactly one
 * delivery whose first new event carries mychar "Z" (a fresh listener per
 * cycle mirrors the fresh deployment, so a double delivery fails loudly).
 * Suite line 414's undeployAll happens at the case boundary. The 100-cycle
 * loop is the suite's connection-leak probe ("Too many connections unless the
 * stop actually relieves them"): undeploy must release the RETAIN-lifecycle
 * connection each cycle or cycle ~N fails on MySQL's connection ceiling — the
 * uniform 100-record trace is itself the evidence that every cycle delivered.
 *
 * Record protocol: exactly one record per listener delivery {case,
 * operation:"listener", statement:"s0", sequence, time, new:[rows]} where rows
 * are {kind:"row", fields:{mychar:"Z"}} objects with field names sorted
 * alphabetically for human-diffable traces (the canonical comparator is
 * order-insensitive; mychar is the sole output column). Value form: mychar as
 * a JSON string (MySQL strips CHAR(20) trailing spaces; "Z" is the
 * mybigint-1 row), both sides matching because the Go side feeds the identical
 * canonical 10-row fixture through a function-fed HistoricalProvider (no
 * database driver). There are deliberately NO count records (unlike the
 * sibling database slices): the undeployed send between the cycles has no
 * statement and no listener to observe it, and the fixed record count of 100
 * — one per cycle — is the leak-probe observable the Go runner pairs with.
 * Sequence is case-local restarting at 1 and runs 1..100; the time is the
 * fixed epoch 1970-01-01T00:00:00Z. The case runs on its own fresh runtime
 * (internal timer off, epoch initialization 0, undeployAll after the case
 * body, destroy in finally) with the runtime URI derived from the pinned
 * java-runtime id. The 100 records carry the database-restart differential
 * chain for the Go case builder.
 */
public final class EPLDatabaseRestartScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-database-restart";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java";
    private static final String CASE_RESTART_STATEMENT = "restart-statement";
    private static final int ORDINAL = 18;
    private static final String RUNTIME_ID = "java-runtime-0d41625df368897ca97b";
    private static final String EXECUTION_NAME = "EPLDatabaseRestartStatement";
    private static final int RESTART_CYCLES = 100;
    private static final int TOTAL_RECORDS = 100;
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    // Byte-exact statement from EPLDatabaseRestartStatement (suite lines
    // 397-399), assembled with the suite's own concatenations: SINGLE space
    // after "s0," (the " sql:" segment starts with one space).
    private static final String RESTART_EPL =
            "@name('s0') select mychar from " +
            "SupportBean_S0 as s0," +
            " sql:MyDBWithRetain ['select mychar from mytesttable where ${id} = mytesttable.mybigint'] as s1";

    // Canonical output names in select-clause order; the projection sorts
    // them alphabetically when serializing.
    private static final String[] MYCHAR_COLUMNS = {"mychar"};

    private EPLDatabaseRestartScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLDatabaseRestartScenarioOracle <scenario.json>");
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME_ID}, "javaRuntimes");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != 1) {
            throw new IllegalArgumentException("scenario must contain the restart-statement case");
        }
        JsonObject definition = object(caseValue.asArray().get(0), "case definition 0");
        if (!CASE_RESTART_STATEMENT.equals(definition.getString("case", ""))
                || definition.getInt("ordinal", -1) != ORDINAL
                || !RUNTIME_ID.equals(definition.getString("runtimeId", ""))
                || !EXECUTION_NAME.equals(definition.getString("executionName", ""))) {
            throw new IllegalArgumentException("case metadata mismatch at index 0");
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != 2) {
            throw new IllegalArgumentException("scenario must contain exactly 2 steps");
        }
        JsonArray steps = stepValue.asArray();
        JsonObject marker = object(steps.get(0), "case marker 0");
        if (!"case".equals(marker.getString("op", ""))
                || !CASE_RESTART_STATEMENT.equals(marker.getString("case", ""))) {
            throw new IllegalArgumentException("the restart-statement case must appear once");
        }
        JsonObject advance = object(steps.get(1), "advance-time 0");
        if (!"advance-time".equals(advance.getString("op", "")) || !EPOCH.equals(advance.getString("at", ""))) {
            throw new IllegalArgumentException("the case must pin the epoch advance-time step");
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
     * this execution: the SupportBean_S0 POJO event type and the
     * MyDBWithRetain database reference as a DriverManagerConnection over the
     * SupportDatabaseService constants with the RETAIN connection lifecycle
     * and no cache (the suite's setEnableJDBC/setEnableQueryPlan logging
     * flags have no recorded effect). Internal timer off.
     */
    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean_S0.class);
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
                "parity-epl-database-restart-" + RUNTIME_ID, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, CASE_RESTART_STATEMENT);
            runRestartStatement(runtime, configuration, writer);
            if (writer.count() != TOTAL_RECORDS) {
                throw new IllegalStateException("case " + CASE_RESTART_STATEMENT + " produced " + writer.count()
                        + " records, expected " + TOTAL_RECORDS);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * EPLDatabaseJoin EPLDatabaseRestartStatement (ordinal 18): compile once,
     * deploy listener-less, then the 100-cycle stop/redeploy loop — the
     * suite's connection-leak probe. Each cycle undeploys, sends to the
     * deployment-less runtime (unobservable, unrecorded), redeploys the SAME
     * compiled module with a fresh listener, sends again and asserts exactly
     * one delivery carrying mychar "Z".
     */
    private static void runRestartStatement(EPRuntime runtime, Configuration configuration, TraceWriter writer)
            throws Exception {
        // suite lines 400-401: compile ONCE and deploy the same EPCompiled
        // the loop redeploys; the initial deployment carries no listener.
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(RESTART_EPL, compilerArgs);
        runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

        // suite lines 404-412: "Too many connections unless the stop
        // actually relieves them" — undeploy, unobserved send, redeploy +
        // listener, send, assertEqualsNew("s0", "mychar", "Z").
        for (int i = 0; i < RESTART_CYCLES; i++) {
            undeployModuleContaining(runtime, "s0");

            // no statement is deployed: nothing can observe this send.
            runtime.getEventService().sendEventBean(new SupportBean_S0(1), "SupportBean_S0");

            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = requireStatement(deployment, "s0");
            RowListener listener = new RowListener(writer, MYCHAR_COLUMNS);
            statement.addListener(listener);
            runtime.getEventService().sendEventBean(new SupportBean_S0(1), "SupportBean_S0");

            // suite line 411: assertEqualsNew("s0", "mychar", "Z") — exactly
            // one delivery whose row carries "Z" (a fresh listener per cycle
            // mirrors the fresh deployment, so a double delivery fails).
            if (listener.deliveries() != 1) {
                throw new IllegalStateException("cycle " + i + " produced " + listener.deliveries()
                        + " deliveries, expected 1");
            }
            assertEquals("Z", listener.lastFirstNew().get("mychar"));
        }

        // suite line 414: undeployAll happens at the case boundary.
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
            return new JsonObject().add("state", "null");
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
        if (value instanceof java.math.BigDecimal) {
            java.math.BigDecimal decimal = (java.math.BigDecimal) value;
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
     * new:[rows]} records for the listener deliveries; the sequence is
     * case-local and, with no count records in this slice, equals the
     * cycle-and-delivery index 1..100.
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

        private long count() {
            return sequence;
        }
    }

    /**
     * Records one trace record per listener delivery (new-data batches only —
     * the execution never delivers an old stream) and keeps the delivery
     * count plus the first new event of the last delivery for the in-process
     * per-cycle assertEqualsNew check.
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
