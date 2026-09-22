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
 * Direct Esper 9.0.0 oracle for the five RowRecogInterval-family executions
 * (match_recognize INTERVAL and INTERVAL OR TERMINATED) replayed as one
 * differential chain of fourteen cases:
 *
 * interval-simple (ord 0, RowRecogIntervalSimple): keepall over pattern
 * (A B*) with interval 10 seconds; completed matches are scheduled and emit
 * only when the interval anchored to the match-begin event expires
 * (inclusive boundary at 11000/23000/35000), while the iterator sees
 * scheduled-but-unemitted matches at 10999. The identical sequence runs
 * twice, mirroring the Java compileDeploy plus eplToModelCompileDeploy
 * cycle as undeploy-all plus redeploy.
 *
 * interval-partitioned (ord 1, RowRecogPartitioned): the same shape
 * partitioned by cat with per-partition interval timers emitting at
 * 11000 (cat C1 and C2), 12000 (C3) and 13000 (C4).
 *
 * interval-multicompleted (ord 2, RowRecogMultiCompleted): multiple
 * completed matches drain across successive interval boundaries at 11000,
 * 15000, 31000 and 32000.
 *
 * interval-monthscoped (ord 3, RowRecogMonthScoped): SupportBean without a
 * keepall window, interval 1 month (calendar semantics); the boundary probe
 * one millisecond before 2002-03-01T09:00:00.000Z emits nothing and the
 * boundary itself emits {A1,B1,null}.
 *
 * orterminated-doc-sample plus nine further sub-scenarios (ord 0,
 * RowRecogIntervalOrTerminated): each sub-scenario is a fresh runtime with
 * the clock reset, mirroring the Java sendTimer(0) plus fresh deploy per
 * sub-assertion. or-terminated emits dead-end end states immediately on a
 * misfit event and drains scheduled end states at interval expiry; the doc
 * sample sends the TemperatureSensorEvent object-array type partitioned by
 * device; orterminated-a-bstar-allmatches carries mode "any" because Java
 * asserts its two end states any-order. sendTimer(Integer.MAX_VALUE)
 * advances to 1970-01-25T20:31:23.647Z.
 *
 * Mirroring SupportEvalRunner, each case deploys "@name('s0') <case EPL>"
 * once per cycle, advances the clock where Java sends timers, sends every
 * assertion event, snapshots the iterator where Java asserts it, then
 * undeploys. The pinned case EPL is the contract text verbatim including
 * its irregular whitespace. SupportRecogBean and SupportBean are declared
 * as map event types and TemperatureSensorEvent as an object-array type
 * (id string, device int, temp double) because the regression-lib jar is
 * not on the oracle classpath. The TraceWriter skips null/null listener
 * callbacks.
 */
public final class RowRecogIntervalScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-interval";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogInterval.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogIntervalOrTerminated.java";

    private static final String DESCRIPTION =
            "RowRecogInterval ordinals 0-3 plus RowRecogIntervalOrTerminated "
                    + "(five executions, the match_recognize INTERVAL family): "
                    + "interval-simple replays the identical sequence across two "
                    + "deploy cycles mirroring compileDeploy plus "
                    + "eplToModelCompileDeploy, emitting completed matches only "
                    + "when the 10-second interval anchored to the match-begin "
                    + "event expires; interval-partitioned partitions by cat "
                    + "with per-partition interval timers; "
                    + "interval-multicompleted drains multiple completed matches "
                    + "across successive interval boundaries; "
                    + "interval-monthscoped uses a calendar month interval over "
                    + "SupportBean with a boundary probe one millisecond before "
                    + "and at 2002-03-01T09:00:00.000Z; the ten or-terminated "
                    + "sub-scenarios each run a fresh runtime and emit "
                    + "immediately on misfit termination or at interval expiry, "
                    + "including the TemperatureSensorEvent object-array doc "
                    + "sample and the all-matches variant asserted any-order. "
                    + "Each case deploys s0 with the verbatim Java EPL, "
                    + "advances the clock where Java sends timers, sends events "
                    + "and snapshots the statement iterator where Java asserts "
                    + "it.";

    private static final String[] CASES = {
            "interval-simple", "interval-partitioned", "interval-multicompleted",
            "interval-monthscoped", "orterminated-doc-sample",
            "orterminated-a-b", "orterminated-a-bstar",
            "orterminated-a-bstar-allmatches", "orterminated-a-bstar-or-c",
            "orterminated-a-bstar-or-cstar", "orterminated-a-b-cstar",
            "orterminated-a-bplus", "orterminated-astar",
            "orterminated-a-parens-bstar"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-02e57a7f151323edc4be",
            "java-runtime-b9a2b0afe61a2bfabcd0",
            "java-runtime-1361f7530aa059720913",
            "java-runtime-373cb011a639d99148f0",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3",
            "java-runtime-bd18c2cfd8ff34a0b5a3"
    };
    private static final String[] EXECUTIONS = {
            "RowRecogIntervalSimple",
            "RowRecogPartitioned",
            "RowRecogMultiCompleted",
            "RowRecogMonthScoped",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated",
            "RowRecogIntervalOrTerminated"
    };
    private static final String[] STATIC_IDS = {
            "java-4f72d1e9bd1e9e0e91c3",
            "java-2b80de56360cd5f89976",
            "java-ecc5ea69ce7531fba427",
            "java-a3bc77ab51a1c2e468a8",
            "java-9c1e0161596faa9a387e"
    };
    private static final String[] OBSERVATIONS = {
            "listener+iterator; two deploy cycles mirroring compileDeploy plus "
                    + "eplToModelCompileDeploy; completed matches emit at the "
                    + "inclusive 10-second interval boundary anchored to the "
                    + "match-begin event (11000/23000/35000); the iterator sees "
                    + "scheduled-but-unemitted matches at 10999",
            "listener+iterator; partition by cat; per-partition interval "
                    + "emission at 11000 (cat C1 and C2), 12000 (C3) and 13000 "
                    + "(C4)",
            "listener+iterator; multiple completed matches drain across "
                    + "successive interval boundaries at 11000, 15000, 31000 "
                    + "and 32000",
            "listener; SupportBean without a keepall window; calendar month "
                    + "interval emits at 2002-03-01T09:00:00.000Z but not one "
                    + "millisecond earlier",
            "listener; TemperatureSensorEvent object-array partitioned by "
                    + "device; the E5 send at temp 100 terminates the match "
                    + "emitting {E2,2,E3,E4}; the interval timer at MAX_VALUE "
                    + "emits nothing further",
            "listener; pattern (A B) emits immediately on B so the interval is "
                    + "not effective; the A2 strand dies on the A3 misfit",
            "listener; misfit X1 terminates the A1 match emitting "
                    + "{A1,B1,null,null}; timer expiry at 30000 emits "
                    + "{A2,B2,null,null}",
            "listener; all-matches variant emits both the A-only and A,B end "
                    + "states per match, asserted any-order in Java",
            "listener; pattern (A (B* | C)); C1 terminates A1 emitting the C "
                    + "branch; X1 terminates A2 emitting {A2,B1,B2,null,null}; "
                    + "A3 emits at the 10000 boundary",
            "listener; pattern (A (B* | C*)); X1 terminates A1 immediately; "
                    + "the B1 misfit terminates the C branch emitting "
                    + "{A2,null,null,C1,null}",
            "listener; pattern (A B C*); the B2 misfit emits "
                    + "{A1,B1,C1,C2,null}; X3 emits {A3,B4,null,null,null}; "
                    + "the A4 match emits at the 30000 boundary",
            "listener; pattern (A B+); X2 emits {A2,B2,null,null}; X3 emits "
                    + "{A4,B3,B4,null}",
            "listener; pattern (A*); the B1 misfit emits "
                    + "{A1,A2,null,null,null}; interval expiry at 12000 emits "
                    + "{A3,A4,A5,null,null}; B2 emits {A3,A4,A5,A6,null}",
            "listener; pattern (A (B)*) mirrors a-bstar with a parenthesized "
                    + "repeat; X1 emits {A1,B1,null,null}; timer expiry at "
                    + "30000 emits {A2,B2,null,null}"
    };

    // orterminated-a-bstar-allmatches asserts its two end states any-order;
    // its case marker carries "mode":"any".
    private static final boolean[] CASE_ANY_MODE = {
            false, false, false, false, false, false, false, true, false,
            false, false, false, false, false};

    private static final String EPL_SIMPLE =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb"
                    + " pattern (A B*)"
                    + " interval 10 seconds"
                    + " define"
                    + " A as A.theString like \"A%\","
                    + " B as B.theString like \"B%\""
                    + ") order by a, b0, b1, lastb";
    private static final String EPL_PARTITIONED =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + "  partition by cat "
                    + "  measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb"
                    + "  pattern (A B*) "
                    + "  INTERVAL 10 seconds "
                    + "  define "
                    + "    A as A.theString like 'A%',"
                    + "    B as B.theString like 'B%'"
                    + ") order by a, b0, b1, lastb";
    private static final String EPL_MULTICOMPLETED =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + "  measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb"
                    + "  pattern (A B*) "
                    + "  interval 10 seconds "
                    + "  define "
                    + "    A as A.theString like 'A%',"
                    + "    B as B.theString like 'B%'"
                    + ") order by a, b0, b1, lastb";
    private static final String EPL_MONTHSCOPED =
            "@name('s0') select * from SupportBean "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1 "
                    + " pattern (A B*)"
                    + " interval 1 month"
                    + " define"
                    + " A as A.theString like \"A%\","
                    + " B as B.theString like \"B%\""
                    + ")";
    private static final String EPL_DOC_SAMPLE =
            "@name('s0') select * from TemperatureSensorEvent\n"
                    + "match_recognize (\n"
                    + "  partition by device\n"
                    + "  measures A.id as a_id, count(B.id) as count_b, first(B.id) as first_b, last(B.id) as last_b\n"
                    + "  pattern (A B*)\n"
                    + "  interval 5 seconds or terminated\n"
                    + "  define\n"
                    + "    A as A.temp > 100,\n"
                    + "    B as B.temp > 100)";
    private static final String EPL_A_B =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B.theString as b"
                    + " pattern (A B)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like 'A%',"
                    + " B as B.theString like 'B%'"
                    + ")";
    private static final String EPL_A_BSTAR =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2"
                    + " pattern (A B*)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like \"A%\","
                    + " B as B.theString like \"B%\""
                    + ")";
    private static final String EPL_A_BSTAR_ALLMATCHES =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2"
                    + " all matches"
                    + " pattern (A B*)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like \"A%\","
                    + " B as B.theString like \"B%\""
                    + ")";
    private static final String EPL_A_BSTAR_OR_C =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2, C.theString as c "
                    + " pattern (A (B* | C))"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like 'A%',"
                    + " B as B.theString like 'B%',"
                    + " C as C.theString like 'C%'"
                    + ")";
    private static final String EPL_A_BSTAR_OR_CSTAR =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, "
                    + "B[0].theString as b0, B[1].theString as b1, "
                    + "C[0].theString as c0, C[1].theString as c1 "
                    + " pattern (A (B* | C*))"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like 'A%',"
                    + " B as B.theString like 'B%',"
                    + " C as C.theString like 'C%'"
                    + ")";
    private static final String EPL_A_B_CSTAR =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B.theString as b, "
                    + "C[0].theString as c0, C[1].theString as c1, C[2].theString as c2 "
                    + " pattern (A B C*)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like 'A%',"
                    + " B as B.theString like 'B%',"
                    + " C as C.theString like 'C%'"
                    + ")";
    private static final String EPL_A_BPLUS =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2"
                    + " pattern (A B+)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like 'A%',"
                    + " B as B.theString like 'B%'"
                    + ")";
    private static final String EPL_ASTAR =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, A[3].theString as a3, A[4].theString as a4"
                    + " pattern (A*)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as theString like 'A%'"
                    + ")";
    private static final String EPL_A_PARENS_BSTAR =
            "@name('s0') select * from SupportRecogBean#keepall "
                    + "match_recognize ("
                    + " measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2"
                    + " pattern (A (B)*)"
                    + " interval 10 seconds or terminated"
                    + " define"
                    + " A as A.theString like \"A%\","
                    + " B as B.theString like \"B%\""
                    + ")";

    // The case-metadata EPL pins the first deployment of each case;
    // interval-simple's second deploy cycle reuses the identical EPL.
    private static final String[] CASE_EPLS = {
            EPL_SIMPLE,
            EPL_PARTITIONED,
            EPL_MULTICOMPLETED,
            EPL_MONTHSCOPED,
            EPL_DOC_SAMPLE,
            EPL_A_B,
            EPL_A_BSTAR,
            EPL_A_BSTAR_ALLMATCHES,
            EPL_A_BSTAR_OR_C,
            EPL_A_BSTAR_OR_CSTAR,
            EPL_A_B_CSTAR,
            EPL_A_BPLUS,
            EPL_ASTAR,
            EPL_A_PARENS_BSTAR
    };

    // Pinned deploy EPLs per case, in deploy order. interval-simple deploys
    // the identical EPL twice (compileDeploy plus eplToModelCompileDeploy).
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_SIMPLE, EPL_SIMPLE},
            {EPL_PARTITIONED},
            {EPL_MULTICOMPLETED},
            {EPL_MONTHSCOPED},
            {EPL_DOC_SAMPLE},
            {EPL_A_B},
            {EPL_A_BSTAR},
            {EPL_A_BSTAR_ALLMATCHES},
            {EPL_A_BSTAR_OR_C},
            {EPL_A_BSTAR_OR_CSTAR},
            {EPL_A_B_CSTAR},
            {EPL_A_BPLUS},
            {EPL_ASTAR},
            {EPL_A_PARENS_BSTAR}
    };

    // Pinned op sequences per case (after the case marker). interval-simple
    // replays deploy -> sends/advances/snapshots -> undeploy-all twice.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "advance-time", "send", "advance-time",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "send", "advance-time", "send", "advance-time",
                    "advance-time", "snapshot", "advance-time", "send",
                    "advance-time", "send", "advance-time", "send",
                    "advance-time", "send", "advance-time", "snapshot",
                    "undeploy-all", "deploy", "advance-time", "send",
                    "advance-time", "snapshot", "advance-time", "snapshot",
                    "advance-time", "send", "advance-time", "send",
                    "advance-time", "advance-time", "snapshot", "advance-time",
                    "send", "advance-time", "send", "advance-time", "send",
                    "advance-time", "send", "advance-time", "snapshot",
                    "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "send", "advance-time",
                    "send", "advance-time", "send", "advance-time", "send",
                    "send", "send", "send", "send", "snapshot", "advance-time",
                    "advance-time", "advance-time", "advance-time",
                    "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "send", "advance-time",
                    "send", "advance-time", "snapshot", "advance-time",
                    "advance-time", "advance-time", "send", "advance-time",
                    "send", "advance-time", "send", "send", "send", "send",
                    "advance-time", "snapshot", "advance-time",
                    "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "advance-time",
                    "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "advance-time",
                    "send", "send", "advance-time", "advance-time",
                    "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "advance-time",
                    "send", "send", "advance-time", "advance-time",
                    "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "send", "send", "advance-time", "advance-time",
                    "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "send", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send", "send",
                    "advance-time", "send", "send", "send", "advance-time",
                    "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "advance-time",
                    "send", "send", "send", "advance-time", "send", "send",
                    "send", "undeploy-all"},
            {"advance-time", "deploy", "send", "send", "send", "advance-time",
                    "send", "send", "advance-time", "advance-time",
                    "undeploy-all"}
    };

    // Pinned advance-time targets per case, in advance order.
    private static final String[][] CASE_TIMES = {
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11.000Z",
                    "1970-01-01T00:00:13.000Z", "1970-01-01T00:00:15.000Z",
                    "1970-01-01T00:00:22.999Z", "1970-01-01T00:00:23.000Z",
                    "1970-01-01T00:00:25.000Z", "1970-01-01T00:00:26.000Z",
                    "1970-01-01T00:00:29.000Z", "1970-01-01T00:00:34.999Z",
                    "1970-01-01T00:00:35.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11.000Z",
                    "1970-01-01T00:00:13.000Z", "1970-01-01T00:00:15.000Z",
                    "1970-01-01T00:00:22.999Z", "1970-01-01T00:00:23.000Z",
                    "1970-01-01T00:00:25.000Z", "1970-01-01T00:00:26.000Z",
                    "1970-01-01T00:00:29.000Z", "1970-01-01T00:00:34.999Z",
                    "1970-01-01T00:00:35.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:01.000Z", "1970-01-01T00:00:02.000Z",
                    "1970-01-01T00:00:03.000Z", "1970-01-01T00:00:10.999Z",
                    "1970-01-01T00:00:11.000Z", "1970-01-01T00:00:11.999Z",
                    "1970-01-01T00:00:12.000Z", "1970-01-01T00:00:13.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:05.000Z", "1970-01-01T00:00:10.999Z",
                    "1970-01-01T00:00:11.000Z", "1970-01-01T00:00:15.000Z",
                    "1970-01-01T00:00:21.000Z", "1970-01-01T00:00:22.000Z",
                    "1970-01-01T00:00:23.000Z", "1970-01-01T00:00:31.000Z",
                    "1970-01-01T00:00:32.000Z"},
            {"2002-02-01T09:00:00.000Z", "2002-03-01T08:59:59.999Z",
                    "2002-03-01T09:00:00.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-25T20:31:23.647Z"},
            {"1970-01-01T00:00:00.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:20.000Z",
                    "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:30.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:20.000Z",
                    "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:30.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:10.000Z",
                    "1970-01-25T20:31:23.647Z"},
            {"1970-01-01T00:00:00.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:20.000Z",
                    "1970-01-01T00:00:30.000Z", "1970-01-25T20:31:23.647Z"},
            {"1970-01-01T00:00:00.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:02.000Z",
                    "1970-01-01T00:00:12.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:20.000Z",
                    "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:30.000Z"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "eventType|field|field|..." with <null> for a JSON-null cat.
    // SupportRecogBean sends encode theString|value|cat, SupportBean sends
    // encode theString|intPrimitive and TemperatureSensorEvent sends encode
    // id|device|temp.
    private static final String[][] CASE_SENDS = {
            {"SupportRecogBean|A1|1|<null>", "SupportRecogBean|A2|2|<null>",
                    "SupportRecogBean|B1|3|<null>", "SupportRecogBean|A3|4|<null>",
                    "SupportRecogBean|B2|5|<null>", "SupportRecogBean|B3|6|<null>",
                    "SupportRecogBean|B4|7|<null>", "SupportRecogBean|A1|1|<null>",
                    "SupportRecogBean|A2|2|<null>", "SupportRecogBean|B1|3|<null>",
                    "SupportRecogBean|A3|4|<null>", "SupportRecogBean|B2|5|<null>",
                    "SupportRecogBean|B3|6|<null>", "SupportRecogBean|B4|7|<null>"},
            {"SupportRecogBean|A1|1|C1", "SupportRecogBean|A2|2|C2",
                    "SupportRecogBean|A3|3|C3", "SupportRecogBean|A4|4|C4",
                    "SupportRecogBean|B1|5|C3", "SupportRecogBean|B2|6|C1",
                    "SupportRecogBean|B3|7|C1", "SupportRecogBean|B4|7|C4"},
            {"SupportRecogBean|A1|1|<null>", "SupportRecogBean|A2|2|<null>",
                    "SupportRecogBean|A3|3|<null>", "SupportRecogBean|A4|4|<null>",
                    "SupportRecogBean|B1|5|<null>", "SupportRecogBean|B2|6|<null>",
                    "SupportRecogBean|B3|7|<null>", "SupportRecogBean|B4|8|<null>"},
            {"SupportBean|A1|0", "SupportBean|B1|0"},
            {"TemperatureSensorEvent|E1|1|98", "TemperatureSensorEvent|E2|1|101",
                    "TemperatureSensorEvent|E3|1|102",
                    "TemperatureSensorEvent|E4|1|101",
                    "TemperatureSensorEvent|E5|1|100"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|A2|0|<null>", "SupportRecogBean|A3|0|<null>",
                    "SupportRecogBean|B2|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|X1|0|<null>", "SupportRecogBean|A2|0|<null>",
                    "SupportRecogBean|B2|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|X1|0|<null>", "SupportRecogBean|A2|0|<null>",
                    "SupportRecogBean|B2|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|C1|0|<null>",
                    "SupportRecogBean|A2|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|B2|0|<null>", "SupportRecogBean|X1|0|<null>",
                    "SupportRecogBean|A3|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|X1|0|<null>",
                    "SupportRecogBean|A2|0|<null>", "SupportRecogBean|C1|0|<null>",
                    "SupportRecogBean|B1|0|<null>", "SupportRecogBean|C2|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|C1|0|<null>", "SupportRecogBean|C2|0|<null>",
                    "SupportRecogBean|B2|0|<null>", "SupportRecogBean|A2|0|<null>",
                    "SupportRecogBean|X1|0|<null>", "SupportRecogBean|B3|0|<null>",
                    "SupportRecogBean|X2|0|<null>", "SupportRecogBean|A3|0|<null>",
                    "SupportRecogBean|B4|0|<null>", "SupportRecogBean|X3|0|<null>",
                    "SupportRecogBean|A4|0|<null>", "SupportRecogBean|B5|0|<null>",
                    "SupportRecogBean|C3|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|X1|0|<null>",
                    "SupportRecogBean|A2|0|<null>", "SupportRecogBean|B2|0|<null>",
                    "SupportRecogBean|X2|0|<null>", "SupportRecogBean|A3|0|<null>",
                    "SupportRecogBean|A4|0|<null>", "SupportRecogBean|B3|0|<null>",
                    "SupportRecogBean|B4|0|<null>", "SupportRecogBean|X3|-1|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|A2|0|<null>",
                    "SupportRecogBean|B1|0|<null>", "SupportRecogBean|A3|0|<null>",
                    "SupportRecogBean|A4|0|<null>", "SupportRecogBean|A5|0|<null>",
                    "SupportRecogBean|A6|0|<null>", "SupportRecogBean|B2|0|<null>",
                    "SupportRecogBean|B3|0|<null>"},
            {"SupportRecogBean|A1|0|<null>", "SupportRecogBean|B1|0|<null>",
                    "SupportRecogBean|X1|0|<null>", "SupportRecogBean|A2|0|<null>",
                    "SupportRecogBean|B2|0|<null>"}
    };

    private static final int EXPECTED_STEPS = 235;
    private static final int EXPECTED_RECORDS = 49;

    private RowRecogIntervalScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogIntervalScenarioOracle <scenario.json>");
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
        Map<String, Object> recogType = new HashMap<>();
        recogType.put("theString", String.class);
        recogType.put("value", Integer.class);
        recogType.put("cat", String.class);
        configuration.getCommon().addEventType("SupportRecogBean", recogType);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        configuration.getCommon().addEventType("TemperatureSensorEvent",
                "id,device,temp".split(","),
                new Object[]{String.class, int.class, double.class});

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
        if ("SupportRecogBean".equals(eventType)) {
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("value", payload.get("value").asInt());
            event.put("cat", nullableString(payload, "cat"));
            runtime.getEventService().sendEventMap(event, "SupportRecogBean");
        } else if ("SupportBean".equals(eventType)) {
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            runtime.getEventService().sendEventMap(event, "SupportBean");
        } else if ("TemperatureSensorEvent".equals(eventType)) {
            JsonArray payload = step.get("payload").asArray();
            Object[] event = new Object[]{
                    payload.get(0).asString(),
                    (int) longNumber(payload.get(1)),
                    payload.get(2).asDouble()};
            runtime.getEventService().sendEventObjectArray(event, "TemperatureSensorEvent");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{
                "java-runtime-02e57a7f151323edc4be",
                "java-runtime-b9a2b0afe61a2bfabcd0",
                "java-runtime-1361f7530aa059720913",
                "java-runtime-373cb011a639d99148f0",
                "java-runtime-bd18c2cfd8ff34a0b5a3"}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{
                "RowRecogIntervalSimple", "RowRecogPartitioned",
                "RowRecogMultiCompleted", "RowRecogMonthScoped",
                "RowRecogIntervalOrTerminated"}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
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
     * payloads, iterator snapshots and undeploy-all. interval-simple
     * carries two deploy cycles with the identical EPL and
     * orterminated-a-bstar-allmatches carries a "mode":"any" marker field.
     * Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            if (CASE_ANY_MODE[caseIndex]) {
                requireFields(marker, "op", "case", "mode");
                if (!"any".equals(string(marker, "mode"))) {
                    throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
                }
            } else {
                requireFields(marker, "op", "case");
            }
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
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String eventType = string(step, "eventType");
                        String actual;
                        if ("SupportRecogBean".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "theString", "value", "cat");
                            String cat = nullableString(payload, "cat");
                            actual = eventType + "|" + string(payload, "theString") + "|"
                                    + integer(payload, "value") + "|"
                                    + (cat == null ? "<null>" : cat);
                        } else if ("SupportBean".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "theString", "intPrimitive");
                            actual = eventType + "|" + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive");
                        } else if ("TemperatureSensorEvent".equals(eventType)) {
                            JsonArray payload = array(step.get("payload"), "send payload " + cursor);
                            if (payload.size() != 3) {
                                throw new IllegalArgumentException("send payload " + cursor
                                        + " is not pinned");
                            }
                            actual = eventType + "|" + string(payload.get(0)) + "|"
                                    + longNumber(payload.get(1)) + "|"
                                    + longNumber(payload.get(2));
                        } else {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
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

    private static String string(JsonValue value) {
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException("value must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object.get(name));
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonValue value) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException("value is outside the Java long range", ex);
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
