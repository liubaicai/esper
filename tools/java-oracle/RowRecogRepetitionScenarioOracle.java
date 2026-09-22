import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.annotation.HookType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.compile.stage2.StatementCompileHook;
import com.espertech.esper.common.internal.compile.stage2.StatementSpecCompiled;
import com.espertech.esper.common.internal.epl.rowrecog.core.RowRecogPatternExpandUtil;
import com.espertech.esper.common.internal.epl.rowrecog.expr.RowRecogExprNode;
import com.espertech.esper.common.internal.epl.rowrecog.expr.RowRecogExprNodePrecedenceEnum;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.StringWriter;
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
 * Direct Esper 9.0.0 oracle for all six RowRecogRepetition executions (the
 * match-recognize quantifier repetition family):
 *
 * repeats (ords 0 and 1, RowRecogRepetitionRepeats{soda=false|true}): the
 * two Java executions run the identical 28-assertion sequence over
 * SupportBean partitioned by intPrimitive; soda only switches the
 * compileDeploy path (text vs object model) so the scenario replays the
 * trace once under case "repeats" and tags both runtime ids in the case
 * metadata. Each assertion deploys s0 with the verbatim generated EPL
 * (measures X as x, define X as X.theString like "X%"), sends the pinned
 * match and no-match sequences — one intPrimitive partition per sequence —
 * then undeploys. Matching sequences produce exactly one listener record;
 * no-match sequences produce none (assertListenerNotInvoked).
 *
 * prev (ord 2, RowRecogRepetitionPrev): A{3} with define
 * A as A.intPrimitive > prev(A.intPrimitive); sends A1..A9 (A8 skipped)
 * with intPrimitive 1,4,2,6,5,6,7,8; the milestone(0) after A3 is an
 * ordering no-op; one new row a=[A6,A7,A9].
 *
 * invalid (ord 3, RowRecogRepetitionInvalid, INVALIDITY): deploys
 * 'create variable int myvariable = 0' then nine path-less
 * tryInvalidCompile probes record the pinned Java message prefixes
 * (A{}, A{null}, A{myvariable}, A{prev(A)}, A{-1}, A{,-1}, A{-1,10},
 * A{-1,}, A{5,3}).
 *
 * doc-samples (ord 4, RowRecogRepetitionDocSamples): four deploys of the
 * verbatim doc-sample EPLs over object-array TemperatureSensorEvent
 * {id,device,temp} partitioned by device — A{2}, A{2,} B, A{2,3} B and
 * A{,2} B.
 *
 * equivalent (ord 5, RowRecogRepetitionEquivalent): 62 compile-text
 * probes. The Java execution wraps each statement in an INTERNAL_COMPILE
 * hook (SupportStatementCompileHook, a regression-lib class not on the
 * oracle classpath), compiles and deploys, then expands the captured
 * pattern via RowRecogPatternExpandUtil.expand and asserts the toEPL
 * text. The scenario pins the semantic EPL (without the hook annotation);
 * the oracle prepends its own CaptureHook, mirrors compile+deploy+
 * undeployAll, expands and verifies the pinned expansion text, and emits
 * one "compile-text" record per pair.
 *
 * SupportBean is declared as a map event type (theString string,
 * intPrimitive int) and TemperatureSensorEvent as an object-array type
 * because the regression-lib jar is not on the oracle classpath. The
 * TraceWriter skips null/null listener callbacks.
 */
public final class RowRecogRepetitionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-repetition";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogRepetition.java";

    private static final String DESCRIPTION =
            "RowRecogRepetition ordinals 0-5 (all six executions, the match-recognize quantifier "
                    + "repetition family): repeats replays the shared 28-assertion trace once for "
                    + "RowRecogRepetitionRepeats{soda=false} and {soda=true} (soda only switches the "
                    + "compileDeploy path); prev covers A{3} with prev(A.intPrimitive); invalid "
                    + "deploys 'create variable int myvariable = 0' then records nine pinned "
                    + "tryInvalidCompile message prefixes; doc-samples runs the four verbatim "
                    + "doc-sample EPLs over object-array TemperatureSensorEvent partitioned by "
                    + "device; equivalent records the 62 pinned pattern-expansion texts as "
                    + "compile-text probes (the oracle substitutes its own INTERNAL_COMPILE hook "
                    + "for the regression-lib SupportStatementCompileHook).";

    private static final String[] CASES = {
            "repeats", "repeats-soda", "prev", "invalid", "doc-samples", "equivalent"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d6d3a12949ef3e346d80",
            "java-runtime-cdf672905648f281ad59",
            "java-runtime-ad8cb0aeafdfcdced0f2",
            "java-runtime-941bbeda7e12013505b3",
            "java-runtime-aa81841e62a62098b578",
            "java-runtime-a47314c7aeffaf6d632c"
    };
    private static final String[] EXECUTIONS = {
            "RowRecogRepetitionRepeats{soda=false}",
            "RowRecogRepetitionRepeats{soda=true}",
            "RowRecogRepetitionPrev",
            "RowRecogRepetitionInvalid",
            "RowRecogRepetitionDocSamples",
            "RowRecogRepetitionEquivalent"
    };
    private static final String[] STATIC_IDS = {
            "java-46702536af8cb29c4f5c"
    };
    private static final String[] JAVA_FLAGS = {
            "INVALIDITY"
    };

    /**
     * Builds the verbatim runAssertion EPL: "@name('s0') select * from
     * SupportBean match_recognize ( partition by intPrimitive measures
     * X as x[, ...] pattern (PATTERN) define X as X.theString like
     * "X%"[, ...])" — byte-exact with the Java makeMeasures/makeDefines
     * concatenation including the double-quoted like literals.
     */
    private static String repeatsEPL(String props, String pattern) {
        StringBuilder measures = new StringBuilder();
        StringBuilder defines = new StringBuilder();
        String delimiter = "";
        for (String prop : props.split(",")) {
            String upper = prop.toUpperCase(java.util.Locale.ENGLISH);
            measures.append(delimiter).append(upper).append(" as ").append(prop);
            defines.append(delimiter).append(upper).append(" as ").append(upper)
                    .append(".theString like \"").append(upper).append("%\"");
            delimiter = ", ";
        }
        return "@name('s0') select * from SupportBean match_recognize ("
                + " partition by intPrimitive measures " + measures
                + " pattern (" + pattern + ")"
                + " define " + defines + ")";
    }

    // Pinned repeats assertions in Java execution order:
    // {props, pattern, sequences in send order}. Each sequence is a
    // comma-separated event-name list sent with intPrimitive equal to its
    // sequence index inside the assertion.
    private static final String[][][] REPEATS_ASSERTIONS = {
            // runAssertionRepeatSingleBound
            {{"a", "A A", "A1,A2", "A3,A4", "A5"}},
            {{"a", "A{2}", "A1,A2", "A3,A4", "A5"}},
            {{"a", "(A{2})", "A1,A2", "A3,A4", "A5"}},
            {{"a,b,c", "A B B C", "A1,B1,B2,C1", "B1,B2,C1", "A1,B1,C1", "A1,B1,B2"}},
            {{"a,b,c", "A B{2} C", "A1,B1,B2,C1", "B1,B2,C1", "A1,B1,C1", "A1,B1,B2"}},
            {{"a,b,c", "A (B B) C", "A1,B1,B2,C1", "B1,B2,C1", "A1,B1,C1", "A1,B1,B2"}},
            {{"a,b,c", "A (B{2}) C", "A1,B1,B2,C1", "B1,B2,C1", "A1,B1,C1", "A1,B1,B2"}},
            {{"a,b,c", "A (B B|C C)", "A1,C1,C2", "A2,B1,B2", "B1,B2", "C1,C2", "A1,B1,C1", "A1,C1,B1"}},
            {{"a,b,c", "A (B{2}|C{2})", "A1,C1,C2", "A2,B1,B2", "B1,B2", "C1,C2", "A1,B1,C1", "A1,C1,B1"}},
            {{"a,b", "A A B B", "A1,A2,B1,B2", "A1,A2,B1", "B1,B2,A1,A2", "A1,B1,A2,B2"}},
            {{"a,b", "A{2} B{2}", "A1,A2,B1,B2", "A1,A2,B1", "B1,B2,A1,A2", "A1,B1,A2,B2"}},
            // runAssertionsRepeatRange
            {{"a,b", "A A A? B", "A1,A2,A3,B1", "A1,A2,B1", "A1,B1", "A1,A2", "B1"}},
            {{"a,b", "A{2,3} B", "A1,A2,A3,B1", "A1,A2,B1", "A1,B1", "A1,A2", "B1"}},
            // runAssertionsUpTo
            {{"a,b", "A? A? B", "B1", "A1,B1", "A1,A2,B1", "A1"}},
            {{"a,b", "A{,2} B", "B1", "A1,B1", "A1,A2,B1", "A1"}},
            // runAssertionsAtLeast
            {{"a,b", "A A A* B", "A1,A2,B1", "A1,A2,A3,B1", "A1,B1", "B1"}},
            {{"a,b", "A{2,} B", "A1,A2,B1", "A1,A2,A3,B1", "A1,B1", "B1"}},
            {{"a,b", "A{2,4} B", "A1,A2,B1", "A1,A2,A3,B1", "A1,B1", "B1"}},
            // runAssertionNestedRepeatSingle
            {{"a,b", "(A B) (A B)", "A1,B1,A2,B2", "A1,A2,B1", "A1,A2,B1,B2", "A1,B1,B2,A2"}},
            {{"a,b", "(A B){2}", "A1,B1,A2,B2", "A1,A2,B1", "A1,A2,B1,B2", "A1,B1,B2,A2"}},
            {{"a,b,c", "A (B C) (B C)", "A1,B1,C1,B2,C2", "A1,B1,C1,B2", "A1,B1,C1,C2", "A1,B1,B2,C1,C2"}},
            {{"a,b,c", "A (B C){2}", "A1,B1,C1,B2,C2", "A1,B1,C1,B2", "A1,B1,C1,C2", "A1,B1,B2,C1,C2"}},
            // runAssertionNestedRepeatRange
            {{"a,b,c", "(A B) (A B)? C", "A1,B1,C1", "A1,B1,A2,B2,C1", "C1", "A1,A2,C2", "B1,A1,C1"}},
            {{"a,b,c", "(A B){1,2} C", "A1,B1,C1", "A1,B1,A2,B2,C1", "C1", "A1,A2,C2", "B1,A1,C1"}},
            // runAssertionsNestedUpTo
            {{"a,b,c", "(A B)? (A B)? C", "C1", "A1,B1,C1", "A1,B1,A2,B2,C1", "A1,B1,A2,B2", "A1,A2"}},
            {{"a,b,c", "(A B){,2} C", "C1", "A1,B1,C1", "A1,B1,A2,B2,C1", "A1,B1,A2,B2", "A1,A2"}},
            // runAssertionsNestedAtLeast
            {{"a,b,c", "(A B) (A B) (A B)* C", "A1,B1,A2,B2,C1", "A1,B1,A2,B2,A3,B3,C1", "A1,B1,C1", "A1,B1,A2,C1", "B1,A1,B2,C1"}},
            {{"a,b,c", "(A B){2,} C", "A1,B1,A2,B2,C1", "A1,B1,A2,B2,A3,B3,C1", "A1,B1,C1", "A1,B1,A2,C1", "B1,A1,B2,C1"}}
    };

    private static final String EPL_PREV =
            "@name('s0') select * from SupportBean match_recognize ("
                    + "  measures A as a"
                    + "  pattern (A{3}) "
                    + "  define "
                    + "    A as A.intPrimitive > prev(A.intPrimitive)"
                    + ")";

    private static final String EPL_CREATE_VARIABLE = "create variable int myvariable = 0";

    private static String invalidEPL(String quantifier) {
        return "select * from SupportBean match_recognize ("
                + "  measures A as a"
                + "  pattern (A" + quantifier + ") "
                + ")";
    }

    // Pinned invalid probes: {statement label, quantifier, message prefix}.
    private static final String[][] INVALID_PROBES = {
            {"empty-quantifier", "{}",
                    "Invalid match-recognize quantifier '{}', expecting an expression"},
            {"null-quantifier", "{null}",
                    "Pattern quantifier 'null' must return an integer-type value"},
            {"variable-quantifier", "{myvariable}",
                    "Pattern quantifier 'myvariable' must return a constant value"},
            {"prev-quantifier", "{prev(A)}",
                    "Invalid match-recognize pattern expression 'prev(A)': Aggregation, sub-select, previous or prior functions are not supported in this context"},
            {"negative-exact", "{-1}",
                    "Invalid pattern quantifier value -1, expecting a minimum of 1"},
            {"negative-upper", "{,-1}",
                    "Invalid pattern quantifier value -1, expecting a minimum of 1"},
            {"negative-lower-range", "{-1,10}",
                    "Invalid pattern quantifier value -1, expecting a minimum of 1"},
            {"negative-lower-open", "{-1,}",
                    "Invalid pattern quantifier value -1, expecting a minimum of 1"},
            {"inverted-range", "{5,3}",
                    "Invalid pattern quantifier value 5, expecting a minimum of 1 and maximum of 3"}
    };

    private static final String EPL_DOC_EXACTLY_N =
            "@name('s0') select * from TemperatureSensorEvent\n"
                    + "match_recognize (\n"
                    + "  partition by device\n"
                    + "  measures A[0].id as a0_id, A[1].id as a1_id\n"
                    + "  pattern (A{2})\n"
                    + "  define \n"
                    + "\tA as A.temp >= 100)";
    private static final String EPL_DOC_N_OR_MORE =
            "@name('s0') select * from TemperatureSensorEvent\n"
                    + "match_recognize (\n"
                    + "  partition by device\n"
                    + "  measures A[0].id as a0_id, A[1].id as a1_id, A[2].id as a2_id, B.id as b_id\n"
                    + "  pattern (A{2,} B)\n"
                    + "  define \n"
                    + "\tA as A.temp >= 100,\n"
                    + "\tB as B.temp >= 102)";
    private static final String EPL_DOC_BETWEEN =
            "@name('s0') select * from TemperatureSensorEvent\n"
                    + "match_recognize (\n"
                    + "  partition by device\n"
                    + "  measures A[0].id as a0_id, A[1].id as a1_id, A[2].id as a2_id, B.id as b_id\n"
                    + "  pattern (A{2,3} B)\n"
                    + "  define \n"
                    + "\tA as A.temp >= 100,\n"
                    + "\tB as B.temp >= 102)";
    private static final String EPL_DOC_UP_TO_N =
            "@name('s0') select * from TemperatureSensorEvent\n"
                    + "match_recognize (\n"
                    + "  partition by device\n"
                    + "  measures A[0].id as a0_id, A[1].id as a1_id, B.id as b_id\n"
                    + "  pattern (A{,2} B)\n"
                    + "  define \n"
                    + "\tA as A.temp >= 100,\n"
                    + "\tB as B.temp >= 102)";

    // Pinned doc-sample deploys in Java execution order:
    // {epl, sends as "id,device,temp" triples}.
    private static final String[][] DOC_SAMPLES = {
            {EPL_DOC_EXACTLY_N, "E1,1,99.0", "E2,1,100.0", "E3,1,100.0", "E4,1,101.0", "E5,1,102.0"},
            {EPL_DOC_N_OR_MORE, "E1,1,99.0", "E2,1,100.0", "E3,1,100.0", "E4,1,101.0", "E5,1,102.0"},
            {EPL_DOC_BETWEEN, "E1,1,99.0", "E2,1,100.0", "E3,1,100.0", "E4,1,101.0", "E5,1,102.0"},
            {EPL_DOC_UP_TO_N, "E1,1,99.0", "E2,1,100.0", "E3,1,100.0", "E4,1,101.0", "E5,1,102.0"}
    };

    /**
     * Builds the verbatim runEquivalent semantic EPL (the Java execution
     * additionally prepends an INTERNAL_COMPILE @Hook annotation naming
     * the regression-lib SupportStatementCompileHook; the oracle
     * substitutes its own CaptureHook at replay time).
     */
    private static String equivalentEPL(String before) {
        return "@name('s0') select * from SupportBean#keepall "
                + "match_recognize ("
                + " measures A as a"
                + " pattern (" + before + ")"
                + " define"
                + " A as A.theString like \"A%\""
                + ")";
    }

    // Pinned runEquivalent before/after pairs in Java execution order.
    private static final String[][] EQUIVALENT_PAIRS = {
            {"A{1}", "A"},
            {"A{2}", "A A"},
            {"A{3}", "A A A"},
            {"A{1} B{2}", "A B B"},
            {"A{1} B{2} C{3}", "A B B C C C"},
            {"(A{2})", "(A A)"},
            {"A?{2}", "A? A?"},
            {"A*{2}", "A* A*"},
            {"A+{2}", "A+ A+"},
            {"A??{2}", "A?? A??"},
            {"A*?{2}", "A*? A*?"},
            {"A+?{2}", "A+? A+?"},
            {"(A B){1}", "(A B)"},
            {"(A B){2}", "(A B) (A B)"},
            {"(A B)?{2}", "(A B)? (A B)?"},
            {"(A B)*{2}", "(A B)* (A B)*"},
            {"(A B)+{2}", "(A B)+ (A B)+"},
            {"A B{2} C", "A B B C"},
            {"A (B{2}) C", "A (B B) C"},
            {"(A{2}) C", "(A A) C"},
            {"A (B{2}|C{2})", "A (B B|C C)"},
            {"A{2} B{2} C{2}", "A A B B C C"},
            {"A{2} B C{2}", "A A B C C"},
            {"A B{2} C{2}", "A B B C C"},
            {"A{1, 3}", "A A? A?"},
            {"A{2, 4}", "A A A? A?"},
            {"A?{1, 3}", "A? A? A?"},
            {"A*{1, 3}", "A* A* A*"},
            {"A+{1, 3}", "A+ A* A*"},
            {"A??{1, 3}", "A?? A?? A??"},
            {"A*?{1, 3}", "A*? A*? A*?"},
            {"A+?{1, 3}", "A+? A*? A*?"},
            {"(A B)?{1, 3}", "(A B)? (A B)? (A B)?"},
            {"(A B)*{1, 3}", "(A B)* (A B)* (A B)*"},
            {"(A B)+{1, 3}", "(A B)+ (A B)* (A B)*"},
            {"A{2,}", "A A A*"},
            {"A?{2,}", "A? A? A*"},
            {"A*{2,}", "A* A* A*"},
            {"A+{2,}", "A+ A+ A*"},
            {"A??{2,}", "A?? A?? A*?"},
            {"A*?{2,}", "A*? A*? A*?"},
            {"A+?{2,}", "A+? A+? A*?"},
            {"(A B)?{2,}", "(A B)? (A B)? (A B)*"},
            {"(A B)*{2,}", "(A B)* (A B)* (A B)*"},
            {"(A B)+{2,}", "(A B)+ (A B)+ (A B)*"},
            {"A{,2}", "A? A?"},
            {"A?{,2}", "A? A?"},
            {"A*{,2}", "A* A*"},
            {"A+{,2}", "A* A*"},
            {"A??{,2}", "A?? A??"},
            {"A*?{,2}", "A*? A*?"},
            {"A+?{,2}", "A*? A*?"},
            {"(A B){,2}", "(A B)? (A B)?"},
            {"(A B)?{,2}", "(A B)? (A B)?"},
            {"(A B)*{,2}", "(A B)* (A B)*"},
            {"(A B)+{,2}", "(A B)* (A B)*"},
            {"(A B){2}", "(A B) (A B)"},
            {"(A){2}", "A A"},
            {"(A B C){3}", "(A B C) (A B C) (A B C)"},
            {"(A B){2} (C D){2}", "(A B) (A B) (C D) (C D)"},
            {"((A B){2} C){2}", "((A B) (A B) C) ((A B) (A B) C)"},
            {"((A|B){2} (C|D){2}){2}", "((A|B) (A|B) (C|D) (C|D)) ((A|B) (A|B) (C|D) (C|D))"}
    };

    private static final String[] CASE_EPLS = {
            repeatsEPL("a", "A A"),
            repeatsEPL("a", "A A"),
            EPL_PREV,
            invalidEPL("{}"),
            EPL_DOC_EXACTLY_N,
            equivalentEPL("A{1}")
    };
    private static final String[] OBSERVATIONS = {
            "listener; replays the shared RowRecogRepetitionRepeats trace once — 28 deploy cycles "
                    + "of the verbatim generated EPLs (partition by intPrimitive, one partition per "
                    + "send sequence); matching sequences produce exactly one listener record, "
                    + "no-match sequences produce none (assertListenerNotInvoked)",
            "shared-trace; RowRecogRepetitionRepeats{soda=true} runs the identical runtime "
                    + "assertions as soda=false — soda only switches the compileDeploy path — so "
                    + "the case carries no steps and maps to the 'repeats' trace records",
            "listener; A{3} with define A as A.intPrimitive > prev(A.intPrimitive); sends A1..A9 "
                    + "(A8 skipped) with intPrimitive 1,4,2,6,5,6,7,8; the milestone(0) after A3 "
                    + "is an ordering no-op; one new row a=[A6,A7,A9]",
            "compile-error; deploys 'create variable int myvariable = 0' then nine path-less "
                    + "tryInvalidCompile probes record the pinned Java message prefixes (A{}, "
                    + "A{null}, A{myvariable}, A{prev(A)}, A{-1}, A{,-1}, A{-1,10}, A{-1,}, A{5,3})",
            "listener; four verbatim doc-sample deploys over object-array TemperatureSensorEvent "
                    + "{id,device,temp} partitioned by device — A{2}, A{2,} B, A{2,3} B and "
                    + "A{,2} B; milestones are ordering no-ops",
            "compile-text; 62 runEquivalent probes pin the RowRecogPatternExpandUtil expansion "
                    + "text — the scenario carries the semantic EPL and the oracle prepends its "
                    + "own INTERNAL_COMPILE hook in place of the regression-lib "
                    + "SupportStatementCompileHook; no events are sent"
    };

    private static final int EXPECTED_STEPS = 542;
    private static final int EXPECTED_RECORDS = 127;

    private RowRecogRepetitionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogRepetitionScenarioOracle <scenario.json>");
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
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        configuration.getCommon().addEventType("TemperatureSensorEvent",
                new String[]{"id", "device", "temp"},
                new Object[]{String.class, Integer.class, Double.class});

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
                switch (operation) {
                    case "deploy": {
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                new CompilerArguments(runtime.getRuntimePath()));
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions().setDeploymentId(
                                        SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                        deployCount++;
                        if ("s0".equals(step.getString("statement", ""))) {
                            writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                            writer.statement.addListener(writer);
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        writer = null;
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "snapshot":
                        if (writer == null) {
                            throw new IllegalStateException("snapshot without a deployed statement");
                        }
                        writer.snapshot();
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "compile-text":
                        compileTextStep(runtime, caseName, step, records);
                        break;
                    default:
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
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            runtime.getEventService().sendEventMap(event, "SupportBean");
            return;
        }
        if ("TemperatureSensorEvent".equals(eventType)) {
            Object[] event = new Object[]{
                    payload.getString("id", null),
                    payload.get("device").asInt(),
                    payload.get("temp").asDouble()};
            runtime.getEventService().sendEventObjectArray(event, "TemperatureSensorEvent");
            return;
        }
        throw new IllegalArgumentException("unsupported event type: " + eventType);
    }

    /**
     * Compiles an expected-invalid probe without the runtime path,
     * mirroring env.tryInvalidCompile's path-less compileWCheckedEx, and
     * emits {"operation":"compile-error"} carrying the pinned expectError
     * prefix after verifying the caught message starts with it
     * (SupportMessageAssertUtil.assertMessage semantics).
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String epl = step.getString("epl", "");
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        records.add(new JsonObject()
                .add("case", caseName)
                .add("operation", "compile-error")
                .add("statement", label)
                .add("sequence", 0)
                .add("value", expected));
    }

    /**
     * Mirrors runEquivalent: prepends the oracle's own INTERNAL_COMPILE
     * hook to the pinned semantic EPL (the Java execution uses
     * SupportStatementCompileHook, a regression-lib class absent from the
     * oracle classpath), compiles and deploys, undeploys, expands the
     * captured pattern via RowRecogPatternExpandUtil.expand and verifies
     * the toEPL text equals the pinned expectExpansion. Emits
     * {"operation":"compile-text"} carrying the verified expansion.
     */
    private static void compileTextStep(EPRuntime runtime, String caseName,
                                        JsonObject step, JsonArray records) throws Exception {
        String label = step.getString("statement", "");
        String expected = step.getString("expectExpansion", "");
        String epl = step.getString("epl", "");
        String hooked = "@Hook(type=" + HookType.class.getName()
                + ".INTERNAL_COMPILE,hook='" + CaptureHook.class.getName() + "')" + epl;
        CaptureHook.reset();
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(hooked,
                new CompilerArguments(runtime.getRuntimePath()));
        runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        runtime.getDeploymentService().undeployAll();
        List<StatementSpecCompiled> specs = CaptureHook.drain();
        if (specs.size() != 1) {
            throw new IllegalStateException("compile-text probe " + label
                    + " captured " + specs.size() + " specs");
        }
        RowRecogExprNode expanded = RowRecogPatternExpandUtil.expand(
                specs.get(0).getRaw().getMatchRecognizeSpec().getPattern(), null);
        StringWriter writer = new StringWriter();
        expanded.toEPL(writer, RowRecogExprNodePrecedenceEnum.MINIMUM);
        String actual = writer.toString();
        if (!expected.equals(actual)) {
            throw new IllegalStateException("expansion drift for " + label
                    + ": expected [" + expected + "] got [" + actual + "]");
        }
        records.add(new JsonObject()
                .add("case", caseName)
                .add("operation", "compile-text")
                .add("statement", label)
                .add("sequence", 0)
                .add("value", expected));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

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
     * case's pinned ops — repeats carries the 28 deploy/send/undeploy-all
     * cycles derived from REPEATS_ASSERTIONS, repeats-soda carries no
     * steps, prev carries deploy plus the eight pinned sends and
     * undeploy-all, invalid carries the create-variable deploy plus nine
     * build-error probes and undeploy-all, doc-samples carries four
     * deploy/send/undeploy-all cycles, and equivalent carries 62
     * compile-text probes. Unknown step fields are rejected.
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
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            switch (CASES[caseIndex]) {
                case "repeats":
                    cursor = validateRepeatsSteps(steps, cursor);
                    break;
                case "repeats-soda":
                    break;
                case "prev":
                    cursor = validatePrevSteps(steps, cursor);
                    break;
                case "invalid":
                    cursor = validateInvalidSteps(steps, cursor);
                    break;
                case "doc-samples":
                    cursor = validateDocSampleSteps(steps, cursor);
                    break;
                case "equivalent":
                    cursor = validateEquivalentSteps(steps, cursor);
                    break;
                default:
                    throw new IllegalStateException("unhandled case " + CASES[caseIndex]);
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateRepeatsSteps(JsonArray steps, int cursor) {
        for (String[][] assertion : REPEATS_ASSERTIONS) {
            String props = assertion[0][0];
            String pattern = assertion[0][1];
            cursor = validateDeployStep(steps, cursor, "repeats", "s0",
                    repeatsEPL(props, pattern));
            for (int sequence = 2; sequence < assertion[0].length; sequence++) {
                String[] events = assertion[0][sequence].split(",");
                for (String event : events) {
                    cursor = validateSendStep(steps, cursor, "repeats",
                            event, sequence - 2);
                }
            }
            cursor = validateUndeployAllStep(steps, cursor, "repeats");
        }
        return cursor;
    }

    private static int validatePrevSteps(JsonArray steps, int cursor) {
        cursor = validateDeployStep(steps, cursor, "prev", "s0", EPL_PREV);
        String[] sends = {"A1:1", "A2:4", "A3:2", "A4:6", "A5:5", "A6:6", "A7:7", "A9:8"};
        for (String send : sends) {
            int separator = send.indexOf(':');
            cursor = validateSendStep(steps, cursor, "prev",
                    send.substring(0, separator),
                    Integer.parseInt(send.substring(separator + 1)));
        }
        return validateUndeployAllStep(steps, cursor, "prev");
    }

    private static int validateInvalidSteps(JsonArray steps, int cursor) {
        cursor = validateDeployStep(steps, cursor, "invalid", "create-variable",
                EPL_CREATE_VARIABLE);
        for (String[] probe : INVALID_PROBES) {
            JsonObject step = object(steps.get(cursor), "step " + cursor);
            requireFields(step, "op", "case", "statement", "epl", "expectError",
                    "compileWithoutPath");
            if (!"build-error".equals(string(step, "op"))
                    || !"invalid".equals(string(step, "case"))
                    || !probe[0].equals(string(step, "statement"))
                    || !invalidEPL(probe[1]).equals(string(step, "epl"))
                    || !probe[2].equals(string(step, "expectError"))
                    || !step.getBoolean("compileWithoutPath", false)) {
                throw new IllegalArgumentException("build-error step " + cursor + " is not pinned");
            }
            cursor++;
        }
        return validateUndeployAllStep(steps, cursor, "invalid");
    }

    private static int validateDocSampleSteps(JsonArray steps, int cursor) {
        for (String[] sample : DOC_SAMPLES) {
            cursor = validateDeployStep(steps, cursor, "doc-samples", "s0", sample[0]);
            for (int send = 1; send < sample.length; send++) {
                String[] fields = sample[send].split(",");
                cursor = validateObjectArraySendStep(steps, cursor, "doc-samples",
                        fields[0], fields[1], fields[2]);
            }
            cursor = validateUndeployAllStep(steps, cursor, "doc-samples");
        }
        return cursor;
    }

    private static int validateEquivalentSteps(JsonArray steps, int cursor) {
        for (int pair = 0; pair < EQUIVALENT_PAIRS.length; pair++) {
            JsonObject step = object(steps.get(cursor), "step " + cursor);
            requireFields(step, "op", "case", "statement", "epl", "expectExpansion");
            String label = "expand-" + (pair + 1);
            if (!"compile-text".equals(string(step, "op"))
                    || !"equivalent".equals(string(step, "case"))
                    || !label.equals(string(step, "statement"))
                    || !equivalentEPL(EQUIVALENT_PAIRS[pair][0]).equals(string(step, "epl"))
                    || !EQUIVALENT_PAIRS[pair][1].equals(string(step, "expectExpansion"))) {
                throw new IllegalArgumentException("compile-text step " + cursor + " is not pinned");
            }
            cursor++;
        }
        return cursor;
    }

    private static int validateDeployStep(JsonArray steps, int cursor, String caseName,
                                          String statement, String epl) {
        JsonObject step = object(steps.get(cursor), "step " + cursor);
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !statement.equals(string(step, "statement"))
                || !epl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
        }
        return cursor + 1;
    }

    private static int validateSendStep(JsonArray steps, int cursor, String caseName,
                                        String theString, int intPrimitive) {
        JsonObject step = object(steps.get(cursor), "step " + cursor);
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != intPrimitive) {
            throw new IllegalArgumentException("send payload " + cursor + " is not pinned");
        }
        return cursor + 1;
    }

    private static int validateObjectArraySendStep(JsonArray steps, int cursor, String caseName,
                                                   String id, String device, String temp) {
        JsonObject step = object(steps.get(cursor), "step " + cursor);
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"TemperatureSensorEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
        requireFields(payload, "id", "device", "temp");
        if (!id.equals(string(payload, "id"))
                || integer(payload, "device") != Integer.parseInt(device)
                || payload.get("temp").asDouble() != Double.parseDouble(temp)) {
            throw new IllegalArgumentException("send payload " + cursor + " is not pinned");
        }
        return cursor + 1;
    }

    private static int validateUndeployAllStep(JsonArray steps, int cursor, String caseName) {
        JsonObject step = object(steps.get(cursor), "step " + cursor);
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step " + cursor + " is not pinned");
        }
        return cursor + 1;
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

    /**
     * Stands in for the regression-lib SupportStatementCompileHook: the
     * compiler instantiates it via the INTERNAL_COMPILE @Hook annotation
     * and it captures each compiled StatementSpecCompiled.
     */
    public static class CaptureHook implements StatementCompileHook {
        private static final List<StatementSpecCompiled> SPECS = new ArrayList<>();

        public static void reset() {
            SPECS.clear();
        }

        public static List<StatementSpecCompiled> drain() {
            List<StatementSpecCompiled> copy = new ArrayList<>(SPECS);
            SPECS.clear();
            return copy;
        }

        @Override
        public void compiled(StatementSpecCompiled compiled) {
            SPECS.add(compiled);
        }
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
            if (value instanceof Map<?, ?> map) {
                JsonObject fields = new JsonObject();
                String[] names = map.keySet().stream().map(Object::toString).sorted()
                        .toArray(String[]::new);
                for (String name : names) {
                    fields.add(name, normalize(map.get(name)));
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
