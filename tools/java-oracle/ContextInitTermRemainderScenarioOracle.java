import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonDBRef;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.epl.historical.database.connection.SupportDatabaseURL;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.util.SupportDatabaseService;
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
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Deterministic Java trace generator for the context-init-term-remainder
 * scenario: the four remaining initiated/terminated-context executions from
 * four different suite files.  The oracle replays the checked-in scenario
 * JSON against the pinned Esper 9.0.0 runtime and emits one JSON record per
 * observable listener delivery plus one compile-error record per invalid
 * probe; the Go parity runner replays the same scenario and the
 * differential comparator requires byte-identical normalized records.
 *
 * <p>db-historical mirrors ContextInitTermTemporalFixed ord 18
 * (ContextStartEndDBHistorical): the MyDB database reference registers as a
 * DriverManagerConnection over the SupportDatabaseService DRIVER/FULLURL
 * constants with SupportDatabaseURL.newProperties() and the POOLED
 * connection lifecycle (a new connection per lookup).  The NineToFive cron
 * context deploys at 2002-05-01T08:00, then s0 joins SupportBean_S0 to
 * sql:MyDB over mytesttable; the S0(2) sends at 08:00 and 17:00 fall
 * outside the 09:00-17:00 window and stay silent, while the in-window
 * 09:00 sends emit the pinned {s1.mychar='Y'} row for mybigint 2 and
 * {s1.mychar='X'} for mybigint 3, mirroring env.assertPropsNew("s0",
 * "s1.mychar", ...).
 *
 * <p>distinct-invalid mirrors ContextInitTermWithDistinct ord 0
 * (ContextInitTermWithDistinctInvalid): five path-less tryInvalidCompile
 * probes pin the missing 'as' stream name, the pattern initiated-by
 * rejection, the sub-select distinct-clause expression, the empty
 * distinct list and the non-overlapping start/end parse rejection.
 *
 * <p>now-invalid mirrors ContextInitTermWithNow ord 2
 * (ContextInitTermWNowInvalid): three path-less tryInvalidCompile probes
 * pin bare @now terminated (expecting 'and'), @now combined with a
 * condition in a non-overlapping context, and @now together with an
 * initiated-by filter stream.
 *
 * <p>hash-invalid mirrors ContextHashSegmented ord 8 (ContextHashInvalid):
 * four path-less probes pin the dummy filter property, the unknown
 * hash_code_xyz function, the bare intPrimitive coalesce expression and
 * the zero-parameter hash_code; the ACtx context and the MyWindow fixture
 * then deploy onto the accumulated module path so the two with-path
 * probes pin the unlisted statement stream type and the named-window
 * partition criteria.
 *
 * <p>The oracle asserts that each caught Java message starts with the
 * verbatim Java-source prefix kept in JAVA_ASSERT_PREFIXES, mirroring
 * SupportMessageAssertUtil.assertMessage startsWith semantics. The
 * recorded compile-error value carries the contract-pinned expectError
 * clause instead: a verbatim substring of the Java assertion text that
 * is a full prefix for most probes but a mid-message clause for
 * hash-bad-func, hash-bare-prop, hash-no-params and
 * statement-stream-type (the Go wording those records pin).
 */
public class ContextInitTermRemainderScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-init-term-remainder";
    private static final String DESCRIPTION =
            "Context initiated/terminated remainder across four suite files: ContextStartEndDBHistorical (ContextInitTermTemporalFixed ord 18) deploys the NineToFive cron context and an s0 join of SupportBean_S0 against sql:MyDB over mytesttable via the MySQL DriverManagerConnection fixture with POOLED lifecycle — S0(2) at 08:00 and 17:00 emits nothing outside the window while the two in-window 09:00 sends emit {s1.mychar='Y'} for id 2 and {s1.mychar='X'} for id 3; ContextInitTermWithDistinctInvalid (ContextInitTermWithDistinct ord 0) pins five distinct-clause rejection probes (missing 'as' stream name, pattern stream, sub-select expression, empty list, non-overlapping start/end); ContextInitTermWNowInvalid (ContextInitTermWithNow ord 2) pins three @now misuse rejections (bare @now terminated, @now with condition in a non-overlapping context, @now with a filter stream); ContextHashInvalid (ContextHashSegmented ord 8) pins four path-less coalesce-clause probes (bad filter property, unknown/bare hash function, no parameters) then deploys the ACtx context and the MyWindow fixture to pin the statement stream-type and named-window partition rejections with the module path. listener records carry the pinned s1.mychar field; compile-error records carry the pinned value while the oracle asserts the Java prefix (Java sources regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java, ContextInitTermWithDistinct.java, ContextInitTermWithNow.java, ContextHashSegmented.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithDistinct.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithNow.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextHashSegmented.java"
    };
    private static final String[] CASES = {
            "db-historical",
            "distinct-invalid",
            "now-invalid",
            "hash-invalid"
    };
    private static final int[] ORDINALS = {18, 0, 2, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-bc152186877c0a641b3d",
            "java-runtime-19cc6b63d1614c49dfbf",
            "java-runtime-6b3caa8f5e3490b05507",
            "java-runtime-25a58a30d6cbd02f00e7"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextStartEndDBHistorical",
            "ContextInitTermWithDistinctInvalid",
            "ContextInitTermWNowInvalid",
            "ContextHashInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-06954b45a1979f495425",
            "java-1db75f8dcee67079871d",
            "java-21fe1b2ee6da1a4f412c",
            "java-0564864de64ece6e7772"
    };
    private static final String[] JAVA_FLAGS = {};
    // The statement name each case listens on (env.addListener); only the
    // db-historical execution attaches a listener.
    private static final String[] LISTENERS = {"s0", null, null, null};

    // Verbatim EPL texts transcribed from the Java executions.
    private static final String EPL_NINETOFIVE =
            "@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)";
    private static final String EPL_DB_S0 =
            "@name('s0') context NineToFive select * from SupportBean_S0 as s0, sql:MyDB ['select * from mytesttable where ${id} = mytesttable.mybigint'] as s1";
    private static final String EPL_DISTINCT_NO_AS =
            "create context MyContext initiated by distinct(theString) SupportBean terminated after 15 seconds";
    private static final String EPL_DISTINCT_PATTERN =
            "create context MyContext initiated by distinct(a.theString) pattern [a=SupportBean] terminated after 15 seconds";
    private static final String EPL_DISTINCT_SUBSELECT =
            "create context MyContext initiated by distinct((select * from MyWindow)) SupportBean as sb terminated after 15 seconds";
    private static final String EPL_DISTINCT_EMPTY =
            "create context MyContext initiated by distinct() SupportBean terminated after 15 seconds";
    private static final String EPL_DISTINCT_START =
            "create context MyContext start distinct(theString) SupportBean end after 15 seconds";
    private static final String EPL_NOW_ALONE =
            "create context TimedImmediate initiated @now terminated after 10 seconds";
    private static final String EPL_NOW_AND =
            "create context TimedImmediate start @now and after 5 seconds end after 10 seconds";
    private static final String EPL_NOW_FILTER =
            "create context TimedImmediate initiated @now and SupportBean terminated after 10 seconds";
    private static final String EPL_HASH_DUMMY =
            "create context ACtx coalesce hash_code(intPrimitive) from SupportBean(dummy = 1) granularity 10";
    private static final String EPL_HASH_BAD_FUNC =
            "create context ACtx coalesce hash_code_xyz(intPrimitive) from SupportBean granularity 10";
    private static final String EPL_HASH_BARE_PROP =
            "create context ACtx coalesce intPrimitive from SupportBean granularity 10";
    private static final String EPL_HASH_NO_PARAMS =
            "create context ACtx coalesce hash_code() from SupportBean granularity 10";
    private static final String EPL_HASH_CTX =
            "@public create context ACtx coalesce hash_code(intPrimitive) from SupportBean granularity 10";
    private static final String EPL_HASH_STREAM_TYPE =
            "context ACtx select * from SupportBean_S0";
    private static final String EPL_HASH_WINDOW =
            "@public create window MyWindow#keepall as SupportBean";
    private static final String EPL_HASH_PARTITION =
            "@public create context SegmentedByWhat partition by theString from MyWindow";

    // Pinned record values (the Go wording carried by compile-error records).
    private static final String ERR_DISTINCT_NO_AS =
            "Distinct-expressions require that a stream name is assigned to the stream using 'as'";
    private static final String ERR_DISTINCT_PATTERN =
            "Distinct-expressions require a stream as the initiated-by condition";
    private static final String ERR_DISTINCT_SUBSELECT =
            "Invalid context distinct-clause expression 'subselect_0': Aggregation, sub-select, previous or prior functions are not supported in this context";
    private static final String ERR_DISTINCT_EMPTY =
            "Distinct-expressions have not been provided";
    private static final String ERR_DISTINCT_START =
            "Incorrect syntax near 'distinct' (a reserved keyword)";
    private static final String ERR_NOW_ALONE =
            "Incorrect syntax near 'terminated' (a reserved keyword) expecting 'and'";
    private static final String ERR_NOW_AND =
            "Incorrect syntax near 'and' (a reserved keyword)";
    private static final String ERR_NOW_FILTER =
            "Invalid use of 'now' with initiated-by stream";
    private static final String ERR_HASH_DUMMY =
            "Failed to validate filter expression 'dummy=1': Property named 'dummy' is not valid in any stream";
    private static final String ERR_HASH_FUNC =
            "expected a hash function that is any of {consistent_hash_crc32, hash_code}";
    private static final String ERR_HASH_NO_PARAMS =
            "expected one or more parameters to the hash function";
    private static final String ERR_HASH_STREAM_TYPE =
            "requires that any of the event types that are listed in the segmented context also appear in any of the filter expressions of the statement, type 'SupportBean_S0' is not one of the types listed";
    private static final String ERR_HASH_PARTITION =
            "Partition criteria may not include named windows";

    /**
     * The verbatim Java-source assertion text per probe label: the full
     * expected message passed to env.tryInvalidCompile, which the oracle
     * asserts with startsWith (SupportMessageAssertUtil.assertMessage
     * semantics).  These include the trailing " [epl-text]" suffixes the
     * Java assertions pin; the emitted record value carries only the
     * pinned expectError clause.
     */
    private static final Map<String, String> JAVA_ASSERT_PREFIXES = new HashMap<>();
    static {
        JAVA_ASSERT_PREFIXES.put("distinct-no-as",
                ERR_DISTINCT_NO_AS + " [" + EPL_DISTINCT_NO_AS + "]");
        JAVA_ASSERT_PREFIXES.put("distinct-pattern",
                ERR_DISTINCT_PATTERN + " [" + EPL_DISTINCT_PATTERN + "]");
        JAVA_ASSERT_PREFIXES.put("distinct-subselect",
                ERR_DISTINCT_SUBSELECT + " [" + EPL_DISTINCT_SUBSELECT + "]");
        JAVA_ASSERT_PREFIXES.put("distinct-empty",
                ERR_DISTINCT_EMPTY + " [" + EPL_DISTINCT_EMPTY + "]");
        JAVA_ASSERT_PREFIXES.put("start-distinct",
                "Incorrect syntax near 'distinct' (a reserved keyword) at line 1 column 31 ["
                        + EPL_DISTINCT_START + "]");
        JAVA_ASSERT_PREFIXES.put("now-alone-terminated",
                "Incorrect syntax near 'terminated' (a reserved keyword) expecting 'and' but found 'terminated' at line 1 column 45 ["
                        + EPL_NOW_ALONE + "]");
        JAVA_ASSERT_PREFIXES.put("now-and-nonoverlapping",
                "Incorrect syntax near 'and' (a reserved keyword) at line 1 column 41 ["
                        + EPL_NOW_AND + "]");
        JAVA_ASSERT_PREFIXES.put("now-with-filter",
                "Invalid use of 'now' with initiated-by stream, this combination is not supported ["
                        + EPL_NOW_FILTER + "]");
        JAVA_ASSERT_PREFIXES.put("hash-dummy-filter",
                ERR_HASH_DUMMY + " [");
        JAVA_ASSERT_PREFIXES.put("hash-bad-func",
                "For context 'ACtx' expected a hash function that is any of {consistent_hash_crc32, hash_code} or a plug-in single-row function or script but received 'hash_code_xyz' [");
        JAVA_ASSERT_PREFIXES.put("hash-bare-prop",
                "For context 'ACtx' expected a hash function that is any of {consistent_hash_crc32, hash_code} or a plug-in single-row function or script but received 'intPrimitive' [");
        JAVA_ASSERT_PREFIXES.put("hash-no-params",
                "For context 'ACtx' expected one or more parameters to the hash function, but found no parameter list [");
        JAVA_ASSERT_PREFIXES.put("statement-stream-type",
                "Segmented context 'ACtx' requires that any of the event types that are listed in the segmented context also appear in any of the filter expressions of the statement, type 'SupportBean_S0' is not one of the types listed [");
        JAVA_ASSERT_PREFIXES.put("partition-named-window",
                ERR_HASH_PARTITION + " [" + EPL_HASH_PARTITION + "]");
    }

    private static final String[] CASE_OBSERVATIONS = {
            "listener; the NineToFive cron context bounds an s0 join of SupportBean_S0 to sql:MyDB over mytesttable (DriverManagerConnection, POOLED lifecycle): S0(2) at 08:00 and 17:00 stays silent out of window, the in-window 09:00 send emits {s1.mychar='Y'} for mybigint 2 and the next-day 09:00 send emits {s1.mychar='X'} for mybigint 3",
            "compile-error; five tryInvalidCompile probes pin the missing 'as' stream name, the pattern-as-initiated-by rejection, the sub-select distinct-clause expression, the empty distinct list, and the non-overlapping start/end distinct parse rejection",
            "compile-error; three tryInvalidCompile probes pin bare @now terminated (expecting 'and'), @now combined with a condition in a non-overlapping context, and @now together with an initiated-by filter stream",
            "compile-error; four path-less probes pin the dummy filter property, the unknown hash_code_xyz function, the bare intPrimitive coalesce expression, and the zero-parameter hash_code; after deploying the ACtx context and the MyWindow fixture the with-path probes pin the unlisted statement stream type and the named-window partition criteria"
    };
    private static final String[] CASE_EPLS = {
            EPL_DB_S0,
            EPL_DISTINCT_NO_AS,
            EPL_NOW_ALONE,
            EPL_HASH_DUMMY
    };

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|at with the payload compacted.  Deploy steps
     * carry the byte-exact EPL text; build-error steps carry the pinned
     * expectError record value plus the compileWithoutPath marker on the
     * probes whose Java execution used the path-less tryInvalidCompile
     * overload; advance-time carries the at instant.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        String db = "db-historical";
        CASE_STEPS.put(db, new String[]{
                "advance-time|" + db + "|||||||||||2002-05-01T08:00:00.000Z",
                "deploy|" + db + "|ctx||" + EPL_NINETOFIVE + "||||||||",
                "deploy|" + db + "|s0||" + EPL_DB_S0 + "||||||||",
                "send|" + db + "||SupportBean_S0||{\"id\":2,\"p00\":null}|||||||",
                "advance-time|" + db + "|||||||||||2002-05-01T09:00:00.000Z",
                "send|" + db + "||SupportBean_S0||{\"id\":2,\"p00\":null}|||||||",
                "advance-time|" + db + "|||||||||||2002-05-01T17:00:00.000Z",
                "send|" + db + "||SupportBean_S0||{\"id\":2,\"p00\":null}|||||||",
                "advance-time|" + db + "|||||||||||2002-05-02T09:00:00.000Z",
                "send|" + db + "||SupportBean_S0||{\"id\":3,\"p00\":null}|||||||",
                "undeploy-all|" + db + "|||||||||||"
        });
        String distinct = "distinct-invalid";
        CASE_STEPS.put(distinct, new String[]{
                "build-error|" + distinct + "|distinct-no-as||" + EPL_DISTINCT_NO_AS
                        + "||" + ERR_DISTINCT_NO_AS + "|1|||||",
                "build-error|" + distinct + "|distinct-pattern||" + EPL_DISTINCT_PATTERN
                        + "||" + ERR_DISTINCT_PATTERN + "|1|||||",
                "build-error|" + distinct + "|distinct-subselect||" + EPL_DISTINCT_SUBSELECT
                        + "||" + ERR_DISTINCT_SUBSELECT + "|1|||||",
                "build-error|" + distinct + "|distinct-empty||" + EPL_DISTINCT_EMPTY
                        + "||" + ERR_DISTINCT_EMPTY + "|1|||||",
                "build-error|" + distinct + "|start-distinct||" + EPL_DISTINCT_START
                        + "||" + ERR_DISTINCT_START + "|1|||||",
                "undeploy-all|" + distinct + "|||||||||||"
        });
        String now = "now-invalid";
        CASE_STEPS.put(now, new String[]{
                "build-error|" + now + "|now-alone-terminated||" + EPL_NOW_ALONE
                        + "||" + ERR_NOW_ALONE + "|1|||||",
                "build-error|" + now + "|now-and-nonoverlapping||" + EPL_NOW_AND
                        + "||" + ERR_NOW_AND + "|1|||||",
                "build-error|" + now + "|now-with-filter||" + EPL_NOW_FILTER
                        + "||" + ERR_NOW_FILTER + "|1|||||",
                "undeploy-all|" + now + "|||||||||||"
        });
        String hash = "hash-invalid";
        CASE_STEPS.put(hash, new String[]{
                "build-error|" + hash + "|hash-dummy-filter||" + EPL_HASH_DUMMY
                        + "||" + ERR_HASH_DUMMY + "|1|||||",
                "build-error|" + hash + "|hash-bad-func||" + EPL_HASH_BAD_FUNC
                        + "||" + ERR_HASH_FUNC + "|1|||||",
                "build-error|" + hash + "|hash-bare-prop||" + EPL_HASH_BARE_PROP
                        + "||" + ERR_HASH_FUNC + "|1|||||",
                "build-error|" + hash + "|hash-no-params||" + EPL_HASH_NO_PARAMS
                        + "||" + ERR_HASH_NO_PARAMS + "|1|||||",
                "deploy|" + hash + "|ctx||" + EPL_HASH_CTX + "||||||||",
                "build-error|" + hash + "|statement-stream-type||" + EPL_HASH_STREAM_TYPE
                        + "||" + ERR_HASH_STREAM_TYPE + "||||||",
                "deploy|" + hash + "|window||" + EPL_HASH_WINDOW + "||||||||",
                "build-error|" + hash + "|partition-named-window||" + EPL_HASH_PARTITION
                        + "||" + ERR_HASH_PARTITION + "||||||",
                "undeploy-all|" + hash + "|||||||||||"
        });
    }

    private static final int EXPECTED_STEPS = 34;
    private static final int EXPECTED_RECORDS = 16;

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextInitTermRemainderScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
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
     * Replays the case's steps on a fresh runtime with the internal timer
     * disabled and the clock initialized at epoch.  Every case registers
     * SupportBean and SupportBean_S0 as bean event types plus the MyDB
     * DriverManagerConnection (SupportDatabaseService DRIVER/FULLURL,
     * SupportDatabaseURL.newProperties(), POOLED connection lifecycle),
     * mirroring TestSuiteContext.configure restricted to this scenario.
     * Deploy steps compile against the accumulated module path (the Java
     * execution's shared RegressionPath) and attach the listener to the
     * case's traced statement, mirroring env.addListener; build-error
     * steps compile with or without the path per their compileWithoutPath
     * marker, mirroring the two tryInvalidCompile overloads.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        ConfigurationCommonDBRef configDB = new ConfigurationCommonDBRef();
        configDB.setDriverManagerConnection(SupportDatabaseService.DRIVER,
                SupportDatabaseService.FULLURL, SupportDatabaseURL.newProperties());
        configDB.setConnectionLifecycleEnum(ConfigurationCommonDBRef.ConnectionLifecycleEnum.POOLED);
        configuration.getCommon().addDatabaseReference("MyDB", configDB);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(string(step, "at")).toEpochMilli());
                        break;
                    case "deploy": {
                        String epl = string(step, "epl");
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        compilerArgs.getPath().getCompileds().addAll(deployedModules);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if (LISTENERS[caseIndex] != null
                                    && LISTENERS[caseIndex].equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime, statement));
                            }
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "build-error":
                        buildErrorStep(configuration, deployedModules, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployedModules.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Compiles an expected-invalid probe, mirroring env.tryInvalidCompile:
     * compileWithoutPath steps compile without the accumulated module
     * path (the path-less overload used by every probe except the two
     * with-path hash probes), other steps compile against the path.  The
     * compile must fail and the caught message must start with the
     * verbatim Java-source prefix pinned in JAVA_ASSERT_PREFIXES
     * (assertMessage startsWith semantics); the emitted compile-error
     * record carries the pinned expectError value.
     */
    private static void buildErrorStep(Configuration configuration, List<EPCompiled> deployedModules,
                                       String caseName, JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String javaPrefix = JAVA_ASSERT_PREFIXES.get(label);
        if (javaPrefix == null) {
            throw new IllegalStateException("no pinned Java assert prefix for probe " + label);
        }
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().getCompileds().addAll(deployedModules);
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(javaPrefix)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + javaPrefix + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * The db-historical execution pins only s1.mychar per delivery
     * (env.assertPropsNew("s0", "s1.mychar", ...)), so each row carries
     * exactly that field.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           EPStatement statement) {
        return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            if (newEvents != null && newEvents.length > 0) {
                JsonArray rows = new JsonArray();
                for (EventBean event : newEvents) {
                    rows.add(pinnedRow(event));
                }
                record.add("new", rows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                JsonArray rows = new JsonArray();
                for (EventBean event : oldEvents) {
                    rows.add(pinnedRow(event));
                }
                record.add("old", rows);
            }
            records.add(record);
        };
    }

    private static JsonObject pinnedRow(EventBean event) {
        JsonObject fields = new JsonObject();
        fields.add("s1.mychar", normalize(event.get("s1.mychar")));
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    /**
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * integral numbers as JSON numbers, other numbers as doubles,
     * booleans passthrough, everything else stringified.
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
            case "SupportBean_S0": {
                requirePayloadFields(payload, "id", "p00");
                JsonValue p00 = payload.get("p00");
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(payload.get("id").asInt(),
                                p00.isNull() ? null : p00.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unsupported event type " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|at with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields", "at"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText + "|" + string(step, "at");
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
    }

    private static void validateStringArray(JsonValue value, String[] expected, String field) {
        JsonArray actual = array(value, field);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(field + " length " + actual.size()
                    + " != " + expected.length);
        }
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(actual.get(i).asString())) {
                throw new IllegalArgumentException(field + "[" + i + "] "
                        + actual.get(i).asString() + " != " + expected[i]);
            }
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
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
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static void requirePayloadFields(JsonObject payload, String... names) {
        for (String name : names) {
            if (payload.get(name) == null) {
                throw new IllegalStateException("payload missing field " + name);
            }
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
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String field) {
        return object.get(field).asInt();
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
