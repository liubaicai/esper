import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the six RowRecogPrev-family executions
 * (PREV() inside match_recognize DEFINE) replayed as one differential
 * chain:
 *
 * timewindow-partitioned-simple (ord 0, RowRecogTimeWindowPartitionedSimple):
 * #time(5 sec) partitioned by cat over pattern (A) with
 * PREV(A.value) = (A.value - 1). Runs under virtual time; the pinned
 * timers show prev history surviving time-window eviction (E4 matches at
 * 6200 against E1 sent at 1000) while the iterator shrinks at
 * 11200/11600/16000.
 *
 * partition-by-2-fields-keepall (ord 1, RowRecogPartitionBy2FieldsKeepall):
 * keepall partitioned by theString,cat over pattern (A B) with
 * A.value > PREV(A.value) and B.value > PREV(B.value); the null-cat S1
 * partition produces no output.
 *
 * unpartitioned-keepall (ord 2, RowRecogUnpartitionedKeepAll): two
 * sequential deployments mirroring the Java undeployModuleContaining plus
 * compileDeploy cycle as undeploy-all plus redeploy; stmt1 defines
 * A.value > PREV(A.value), stmt2 defines PREV(A.value, 2) = 5 (indexed
 * prev).
 *
 * timewindow-unpartitioned (ord 3, RowRecogTimeWindowUnpartitioned) and
 * timewindow-partitioned (ord 4, RowRecogTimeWindowPartitioned): #time(5)
 * over pattern (A B) exercising multi-index prev on string and numeric
 * fields, prev(x,0) under Math.abs and IN over prev, under virtual time
 * with iterator expiry at 9500/11500; ord 4 adds partition by cat.
 *
 * example-with-prev (ord 5, RowRecogExampleWithPREV from
 * RowRecogDataSet.java): keepall with 20 measure columns and after match
 * skip to current row over ( A B C* D E* F+ ); A is undefined and matches
 * any row; E7 produces 1 row, E8 3 rows and E9 5 rows.
 *
 * Mirroring SupportEvalRunner, each case deploys "@name('s0') <case EPL>"
 * once per cycle, advances the clock where Java sends timers, sends every
 * assertion event, snapshots the iterator where Java asserts it, then
 * undeploys. The pinned case EPL is the contract text verbatim including
 * its irregular whitespace and ord5's block comment. SupportRecogBean is
 * declared as a map event type (theString string, value int, cat string)
 * because the regression-lib jar is not on the oracle classpath. The
 * TraceWriter skips null/null listener callbacks.
 */
public final class RowRecogPrevScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-prev";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogPrev.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogDataSet.java";

    private static final String DESCRIPTION =
            "RowRecogPrev ordinals 0-4 plus RowRecogDataSet "
                    + "RowRecogExampleWithPREV (six executions, the PREV() family inside "
                    + "match_recognize DEFINE): timewindow-partitioned-simple runs under "
                    + "virtual time and shows prev history surviving time-window "
                    + "eviction; partition-by-2-fields-keepall partitions by "
                    + "theString,cat including a null-cat partition that produces no "
                    + "output; unpartitioned-keepall replays two sequential deployments "
                    + "(undeployModuleContaining plus redeploy) covering plain and "
                    + "indexed prev; timewindow-unpartitioned and timewindow-partitioned "
                    + "exercise multi-index prev on string and numeric fields, "
                    + "prev(x,0) under Math.abs and IN over prev under virtual time; "
                    + "example-with-prev replays the 20-measure-column dataset example "
                    + "with after match skip to current row and an undefined A "
                    + "variable. Each case deploys s0 with the verbatim Java EPL, "
                    + "advances the clock where Java sends timers, sends "
                    + "SupportRecogBean events and snapshots the statement iterator "
                    + "where Java asserts it.";

    private static final String[] CASES = {
            "timewindow-partitioned-simple", "partition-by-2-fields-keepall",
            "unpartitioned-keepall", "timewindow-unpartitioned",
            "timewindow-partitioned", "example-with-prev"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-c096e55f89e15fcc54b7",
            "java-runtime-9c42dd39d70d409a64d7",
            "java-runtime-1200bc1d6899155bac4a",
            "java-runtime-241336e6d46c14fbcf06",
            "java-runtime-59f7afa17be17f57fbd5",
            "java-runtime-41c5b0a59540aec34895"
    };
    private static final String[] EXECUTIONS = {
            "RowRecogTimeWindowPartitionedSimple",
            "RowRecogPartitionBy2FieldsKeepall",
            "RowRecogUnpartitionedKeepAll",
            "RowRecogTimeWindowUnpartitioned",
            "RowRecogTimeWindowPartitioned",
            "RowRecogExampleWithPREV"
    };
    private static final String[] STATIC_IDS = {
            "java-0c4fd7a54738a6e8389b",
            "java-afe8f36bb6325baadb99"
    };
    private static final String[] OBSERVATIONS = {
            "listener+iterator; virtual time; prev history survives time-window "
                    + "eviction (E4 matches at 6200 against E1 sent at 1000); time-window "
                    + "expiry shrinks the iterator at 11200/11600/16000",
            "listener+iterator; keepall partitioned by theString,cat; the null-cat "
                    + "S1 partition produces no output",
            "listener+iterator; two sequential deployments mirroring "
                    + "undeployModuleContaining plus redeploy: stmt1 defines "
                    + "A.value > PREV(A.value), stmt2 defines PREV(A.value, 2) = 5 "
                    + "(indexed prev)",
            "listener+iterator; virtual time; multi-index prev on string and "
                    + "numeric fields, prev(x,0) under Math.abs, IN over prev; iterator "
                    + "expiry at 9500/11500",
            "listener+iterator; virtual time; the timewindow-unpartitioned shape "
                    + "partitioned by cat with per-partition prev history and per-partition "
                    + "time expiry",
            "listener; keepall with 20 measure columns and after match skip to "
                    + "current row over ( A B C* D E* F+ ); A is undefined (matches any "
                    + "row); E7 produces 1 row, E8 3 rows, E9 5 rows"
    };

    private static final String EPL_TIMEWINDOW_PARTITIONED_SIMPLE =
            "@name('s0') select * from SupportRecogBean#time(5 sec) "
                    + "match_recognize ("
                    + "  partition by cat "
                    + "  measures A.cat as cat, A.theString as a_string"
                    + "  all matches pattern (A) "
                    + "  define "
                    + "    A as PREV(A.value) = (A.value - 1)"
                    + ") order by a_string";
    private static final String EPL_PARTITION_BY_2_FIELDS_KEEPALL =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + "  partition by theString, cat"
                    + "  measures A.theString as a_string, A.cat as a_cat, A.value as a_value, B.value as b_value "
                    + "  all matches pattern (A B) "
                    + "  define "
                    + "    A as (A.value > PREV(A.value)),"
                    + "    B as (B.value > PREV(B.value))"
                    + ") order by a_string, a_cat";
    private static final String EPL_UNPARTITIONED_KEEPALL_FIRST =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + "  measures A.theString as a_string"
                    + "  all matches pattern (A) "
                    + "  define A as (A.value > PREV(A.value))"
                    + ") "
                    + "order by a_string";
    private static final String EPL_UNPARTITIONED_KEEPALL_SECOND =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + "  measures A.theString as a_string"
                    + "  all matches pattern (A) "
                    + "  define A as (PREV(A.value, 2) = 5)"
                    + ") "
                    + "order by a_string";
    private static final String EPL_TIMEWINDOW_UNPARTITIONED =
            "@name('s0') select * from SupportRecogBean#time(5) "
                    + "match_recognize ("
                    + "  measures A.theString as a_string, B.theString as b_string"
                    + "  all matches pattern (A B) "
                    + "  define "
                    + "    A as PREV(A.theString, 3) = 'P3' and PREV(A.theString, 2) = 'P2' and PREV(A.theString, 4) = 'P4' and Math.abs(prev(A.value, 0)) >= 0,"
                    + "    B as B.value in (PREV(B.value, 4), PREV(B.value, 2))"
                    + ")";
    private static final String EPL_TIMEWINDOW_PARTITIONED =
            "@name('s0') select * from SupportRecogBean#time(5) "
                    + "match_recognize ("
                    + "  partition by cat"
                    + "  measures A.cat as cat, A.theString as a_string, B.theString as b_string"
                    + "  all matches pattern (A B) "
                    + "  define "
                    + "    A as PREV(A.theString, 3) = 'P3' and PREV(A.theString, 2) = 'P2' and PREV(A.theString, 4) = 'P4',"
                    + "    B as B.value in (PREV(B.value, 4), PREV(B.value, 2))"
                    + ") order by cat";
    private static final String EPL_EXAMPLE_WITH_PREV =
            "@name('s0') SELECT * "
                    + "FROM SupportRecogBean#keepall"
                    + "   MATCH_RECOGNIZE ("
                    + "       MEASURES A.theString AS a_string,"
                    + "         A.value AS a_value,"
                    + "         B.theString AS b_string,"
                    + "         B.value AS b_value,"
                    + "         C[0].theString AS c0_string,"
                    + "         C[0].value AS c0_value,"
                    + "         C[1].theString AS c1_string,"
                    + "         C[1].value AS c1_value,"
                    + "         C[2].theString AS c2_string,"
                    + "         C[2].value AS c2_value,"
                    + "         D.theString AS d_string,"
                    + "         D.value AS d_value,"
                    + "         E[0].theString AS e0_string,"
                    + "         E[0].value AS e0_value,"
                    + "         E[1].theString AS e1_string,"
                    + "         E[1].value AS e1_value,"
                    + "         F[0].theString AS f0_string,"
                    + "         F[0].value AS f0_value,"
                    + "         F[1].theString AS f1_string,"
                    + "         F[1].value AS f1_value"
                    + "       ALL MATCHES"
                    + "       after match skip to current row"
                    + "       PATTERN ( A B C* D E* F+ )"
                    + "       DEFINE /* A is unspecified, defaults to TRUE, matches any row */"
                    + "            B AS (B.value < PREV (B.value)),"
                    + "            C AS (C.value <= PREV (C.value)),"
                    + "            D AS (D.value < PREV (D.value)),"
                    + "            E AS (E.value >= PREV (E.value)),"
                    + "            F AS (F.value >= PREV (F.value) and F.value > A.value)"
                    + ")";

    // The case-metadata EPL pins the first deployment of each case; ord2's
    // second deployment EPL is pinned by CASE_DEPLOY_EPLS below.
    private static final String[] CASE_EPLS = {
            EPL_TIMEWINDOW_PARTITIONED_SIMPLE,
            EPL_PARTITION_BY_2_FIELDS_KEEPALL,
            EPL_UNPARTITIONED_KEEPALL_FIRST,
            EPL_TIMEWINDOW_UNPARTITIONED,
            EPL_TIMEWINDOW_PARTITIONED,
            EPL_EXAMPLE_WITH_PREV
    };

    // Pinned deploy EPLs per case, in deploy order. ord2 deploys two
    // different statements (undeployModuleContaining plus redeploy).
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_TIMEWINDOW_PARTITIONED_SIMPLE},
            {EPL_PARTITION_BY_2_FIELDS_KEEPALL},
            {EPL_UNPARTITIONED_KEEPALL_FIRST, EPL_UNPARTITIONED_KEEPALL_SECOND},
            {EPL_TIMEWINDOW_UNPARTITIONED},
            {EPL_TIMEWINDOW_PARTITIONED},
            {EPL_EXAMPLE_WITH_PREV}
    };

    // Pinned op sequences per case (after the case marker). ord2 replays
    // deploy -> sends -> undeploy-all -> deploy -> sends -> undeploy-all.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "advance-time", "send", "advance-time",
                    "send", "advance-time", "send", "advance-time", "send",
                    "snapshot", "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot", "advance-time", "send",
                    "snapshot", "send", "send", "send", "send", "advance-time",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "advance-time", "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "snapshot", "send", "snapshot",
                    "send", "send", "send", "snapshot", "send", "send",
                    "snapshot", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "snapshot", "send", "snapshot", "send",
                    "snapshot", "send", "snapshot", "send", "snapshot", "send",
                    "send", "snapshot", "undeploy-all", "deploy", "send",
                    "send", "snapshot", "send", "snapshot", "send", "send",
                    "send", "send", "snapshot", "send", "snapshot", "send",
                    "snapshot", "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "send", "send", "send",
                    "send", "advance-time", "send", "send", "snapshot",
                    "advance-time", "send", "send", "send", "advance-time",
                    "send", "send", "send", "snapshot", "advance-time", "send",
                    "send", "advance-time", "send", "send", "send", "send",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "send", "send", "send",
                    "send", "advance-time", "send", "send", "snapshot",
                    "advance-time", "send", "send", "send", "advance-time",
                    "send", "send", "send", "snapshot", "advance-time", "send",
                    "send", "advance-time", "send", "send", "send", "send",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "undeploy-all"}
    };

    // Pinned advance-time targets per case, in advance order.
    private static final String[][] CASE_TIMES = {
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:02.000Z", "1970-01-01T00:00:02.500Z",
                    "1970-01-01T00:00:06.200Z", "1970-01-01T00:00:06.500Z",
                    "1970-01-01T00:00:07.000Z", "1970-01-01T00:00:10.000Z",
                    "1970-01-01T00:00:11.199Z", "1970-01-01T00:00:11.200Z",
                    "1970-01-01T00:00:11.600Z", "1970-01-01T00:00:16.000Z"},
            {},
            {},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:02.000Z", "1970-01-01T00:00:03.000Z",
                    "1970-01-01T00:00:04.000Z", "1970-01-01T00:00:05.000Z",
                    "1970-01-01T00:00:06.000Z", "1970-01-01T00:00:08.500Z",
                    "1970-01-01T00:00:09.500Z", "1970-01-01T00:00:10.500Z",
                    "1970-01-01T00:00:11.500Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:02.000Z", "1970-01-01T00:00:03.000Z",
                    "1970-01-01T00:00:04.000Z", "1970-01-01T00:00:05.000Z",
                    "1970-01-01T00:00:06.000Z", "1970-01-01T00:00:08.500Z",
                    "1970-01-01T00:00:09.500Z", "1970-01-01T00:00:10.500Z",
                    "1970-01-01T00:00:11.500Z"},
            {}
    };

    // Pinned send payloads per case, in send order, encoded
    // "theString|cat|value" with <null> for a JSON-null cat.
    private static final String[][] CASE_SENDS = {
            {"E1|S1|100", "E2|S3|100", "E3|S2|102", "E4|S1|101", "E5|S3|101",
                    "E6|S1|102", "E7|S2|103", "E8|S2|102", "E8|S1|101",
                    "E8|S2|104", "E8|S1|105"},
            {"S1|T1|5", "S2|T1|110", "S1|T2|21", "S1|T1|7", "S2|T1|111",
                    "S1|T2|20", "S2|T1|110", "S2|T2|1000", "S2|T2|1001",
                    "S1|<null>|9", "S1|T1|9", "S2|T2|1001", "S2|T1|109",
                    "S1|T2|25", "S2|T2|1002", "S2|T2|1003", "S1|T2|28"},
            {"E1|<null>|5", "E2|<null>|3", "E3|<null>|6", "E4|<null>|4",
                    "E5|<null>|6", "E6|<null>|10", "E7|<null>|9",
                    "E8|<null>|4", "E1|<null>|5", "E2|<null>|4",
                    "E3|<null>|6", "E4|<null>|3", "E5|<null>|3",
                    "E5|<null>|5", "E6|<null>|5", "E7|<null>|6",
                    "E8|<null>|6"},
            {"P2|<null>|1", "P1|<null>|2", "P3|<null>|3", "P4|<null>|4",
                    "P2|<null>|1", "E1|<null>|3", "P4|<null>|11",
                    "P3|<null>|12", "P2|<null>|13", "xx|<null>|4",
                    "E2|<null>|-1", "E3|<null>|12", "P4|<null>|21",
                    "P3|<null>|22", "P2|<null>|23", "xx|<null>|-2",
                    "E5|<null>|-1", "E6|<null>|-2"},
            {"P4|c2|1", "P3|c1|2", "P2|c2|3", "xx|c1|4", "P2|c1|1",
                    "E1|c1|3", "P4|c1|11", "P3|c1|12", "P2|c1|13",
                    "xx|c1|4", "E2|c1|-1", "E3|c1|12", "P4|c2|21",
                    "P3|c2|22", "P2|c2|23", "xx|c2|-2", "E5|c2|-1",
                    "E6|c2|-2"},
            {"E1|<null>|100", "E2|<null>|98", "E3|<null>|75", "E4|<null>|61",
                    "E5|<null>|50", "E6|<null>|49", "E7|<null>|64",
                    "E8|<null>|78", "E9|<null>|84"}
    };

    private static final int EXPECTED_STEPS = 182;
    private static final int EXPECTED_RECORDS = 58;

    private RowRecogPrevScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogPrevScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("value", Integer.class);
        beanType.put("cat", String.class);
        configuration.getCommon().addEventType("SupportRecogBean", beanType);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
            int deployCount = 0;
            boolean active = false;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    String epl = step.getString("epl", "");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(
                                    SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                    deployCount++;
                    writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                    writer.statement.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    writer = null;
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("advance-time".equals(operation)) {
                    runtime.getEventService().advanceTime(
                            Instant.parse(step.getString("at", "")).toEpochMilli());
                } else if ("snapshot".equals(operation)) {
                    if (writer == null) {
                        throw new IllegalStateException("snapshot without a deployed statement");
                    }
                    writer.snapshot();
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportRecogBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("value", payload.get("value").asInt());
        event.put("cat", nullableString(payload, "cat"));
        runtime.getEventService().sendEventMap(event, "SupportRecogBean");
    }

    private static String nullableString(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned ops — deploy s0 with the verbatim EPL, advance-time
     * steps at the pinned clock targets, the assertion sends with pinned
     * payloads, iterator snapshots and undeploy-all. ord2 carries two
     * deploy cycles with different EPLs. Unknown step fields are
     * rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            int deploys = 0;
            int advances = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_DEPLOY_EPLS[caseIndex][deploys++].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!"SupportRecogBean".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "value", "cat");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String cat = nullableString(payload, "cat");
                        String actual = string(payload, "theString") + "|"
                                + (cat == null ? "<null>" : cat) + "|" + integer(payload, "value");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_TIMES[caseIndex][advances++].equals(string(step, "at"))) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "snapshot":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != CASE_DEPLOY_EPLS[caseIndex].length
                    || advances != CASE_TIMES[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " step counts are not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            append("listener", ++sequence, newEvents, oldEvents);
        }

        private void snapshot() {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append("snapshot", 0, events.toArray(new EventBean[0]), null);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
            }
            if (value instanceof EventBean[] events) {
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    array.add(normalize(event));
                }
                return array;
            }
            if (value instanceof EventBean event) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
