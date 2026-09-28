import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-everydistinct-compound-577
 * bundle: PatternOperatorEveryDistinct ords 8-12 — the compound-operator
 * quintet — replayed as ten cases (five ords x two expiry legs), each
 * against a fresh runtime like the regression suite's per-leg deploy.
 *
 * Every execution runs the identical send sequence twice: first with the
 * no-expiry every-distinct form, then with the timed-expiry form (1 hour
 * for ords 8-11, 2 hours 1 minute for ord 12), undeploying between legs.
 * No clock ops exist in these executions, so the timed key never expires
 * and both legs share the identical expected sequence (ord 3 OverFilter
 * precedent).
 *
 * over-and (PatternEveryDistinctOverAnd, ord 8) deploys
 * {@code every-distinct(a.intPrimitive, b.intPrimitive) (a='A%' and
 * b='B%')}: the composite key is the (a,b) intPrimitive pair, so B1(10)
 * completes A1(1) and fires {A1,B1} while A2(1)+B2(10) is swallowed as a
 * duplicate key.
 *
 * over-or (PatternEveryDistinctOverOr, ord 9) deploys
 * {@code every-distinct(coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0))
 * (a='A%' or b='B%')}: a single expression key sees both branches and the
 * non-firing tag projects null.
 *
 * over-not (PatternEveryDistinctOverNot, ord 10) deploys
 * {@code every-distinct(a.intPrimitive) (a='A%' and not 'B%')}: B1(1)
 * falsifies the attempt and EvalEveryDistinctStateNode.evaluateFalse
 * respawns WITHOUT copying the key set, so A4(1) fires again on the
 * previously seen key — the falsification-respawn discriminant.
 *
 * over-followed-by (PatternEveryDistinctOverFollowedBy, ord 11) deploys
 * {@code every-distinct(a.intPrimitive + b.intPrimitive) (a='A%' -> b='B%')}:
 * identical sums suppress later completions (1+1 and 10+-8 both key 2).
 *
 * within-followed-by (PatternEveryDistinctWithinFollowedBy, ord 12)
 * deploys {@code (every-distinct(a.intPrimitive) a='A%') ->
 * b=SupportBean(intPrimitive=a.intPrimitive)}: each fresh key spawns ONE
 * waiting branch correlated on its own a value — the per-key-branch
 * discriminant — so B4(1) is silent after the A1 branch completed while
 * B5(2) still fires {A2,B5}.
 *
 * Java milestone() savepoints restore identical state for these
 * non-contextual executions and are omitted (573/575/576 precedent).
 *
 * SupportBean is the real common/internal support bean (theString +
 * intPrimitive ctor) mirroring the regression suite. Tagged-event
 * columns arrive at the listener as the underlying bean, so the trace
 * renders the bean's pinned field projection {theString,intPrimitive}
 * as a kind:row object exactly like the Go NormalizeResults event
 * rendering; unbound tags (the ord 9 non-firing side) render the null
 * marker. Records follow the standard protocol: one listener record per
 * delivered update with a per-case sequence counter starting at 1 and
 * time rendered from the current engine time.
 */
public final class PatternEveryDistinctCompound577ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-everydistinct-compound-577";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "PatternOperatorEveryDistinct ords 8-12 — every-distinct over compound pattern operators "
                    + "(and/or/and-not/followed-by roots plus left-nested distinct inside followed-by) over "
                    + "SupportBean, each replayed twice: no-expiry leg and timed-expiry leg (1 hour; 2 hours 1 "
                    + "minute for within-followed-by) with undeployAll between and no clock, so each leg pair "
                    + "shares the identical expected sequence (ord 3 OverFilter precedent). over-and "
                    + "(PatternEveryDistinctOverAnd, ord 8) keys on the (a.intPrimitive, b.intPrimitive) pair: "
                    + "A1(1) silent, B1(10) fires {A1,B1}, A2(1)+B2(10)+A3(2) silent (dup key 1+10), B3(10) fires "
                    + "{A3,B3}, A4(1)+B4(20) fires {A4,B4}, A5(2)+B5(10) silent (dup 2+10), A6(2)+B6(20) fires "
                    + "{A6,B6}, A7(2)+B7(20) silent — 4 fires. over-or (PatternEveryDistinctOverOr, ord 9) keys "
                    + "on coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0): A1(1) fires {A1,null} key 1, "
                    + "B1(2) fires {null,B1} key 2, B2(1)+A2(2)+A3(2)+B3(1) silent (keys 1 and 2 seen), B4(3) and "
                    + "B5(4) fire, B6(3)+A4(3)+A5(4) silent — 4 fires. over-not (PatternEveryDistinctOverNot, "
                    + "ord 10) keys on a.intPrimitive: A1(1)/A3(2) fire, A2(1) swallowed, B1(1) falsifies the "
                    + "attempt and the respawned attempt carries an EMPTY key set (EvalEveryDistinctStateNode "
                    + "evaluateFalse) so A4(1) fires again on the previously seen key, A5(1) silent — 3 fires. "
                    + "over-followed-by (PatternEveryDistinctOverFollowedBy, ord 11) keys on "
                    + "a.intPrimitive+b.intPrimitive: A1(1)->B1(1) fires key 2, A2(1)->B2(1) and A3(10)->B3(-8) "
                    + "silent (both sum to 2), A4(2)->B4(1) fires key 3, A5(3)->B5(0) silent (dup 3) — 2 fires. "
                    + "within-followed-by (PatternEveryDistinctWithinFollowedBy, ord 12) nests every-distinct "
                    + "on the left of followed-by so each fresh a.intPrimitive key spawns one waiting branch "
                    + "correlating b on intPrimitive=a.intPrimitive: B1(0) matches nothing, B2(1) fires "
                    + "{A1,B2}, A4(1) dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent (A1 branch consumed), "
                    + "B5(2) fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires — 4 fires. Java milestone() "
                    + "savepoints are omitted (they restore identical state for these non-contextual "
                    + "executions).";

    private static final String[] CASES = {
            "over-and", "over-and-expiry",
            "over-or", "over-or-expiry",
            "over-not", "over-not-expiry",
            "over-followed-by", "over-followed-by-expiry",
            "within-followed-by", "within-followed-by-expiry"};
    private static final int[] CASE_ORDINALS = {8, 8, 9, 9, 10, 10, 11, 11, 12, 12};
    private static final int[] CASE_RUNTIME_INDEX = {0, 0, 1, 1, 2, 2, 3, 3, 4, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-fa2df3aeba01a5eda1d2",
            "java-runtime-6aab27f9a29d3ad7188f",
            "java-runtime-c23dddb4d0d001809ffd",
            "java-runtime-c28f82ea566795672292",
            "java-runtime-58fcea53e80425896c2d"};
    private static final String[] EXECUTIONS = {
            "PatternEveryDistinctOverAnd",
            "PatternEveryDistinctOverOr",
            "PatternEveryDistinctOverNot",
            "PatternEveryDistinctOverFollowedBy",
            "PatternEveryDistinctWithinFollowedBy"};
    // Deduplicated inventory id: the five runtime rows share static id
    // java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
    // executions, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock; composite distinct key (a.intPrimitive, b.intPrimitive) over a='A%' and "
                    + "b='B%': A1(1) silent, B1(10) fires {A1,B1} key 1+10, A2(1)+B2(10)+A3(2) silent "
                    + "(dup 1+10), B3(10) fires {A3,B3} key 2+10, A4(1)+B4(20) fires {A4,B4} key 1+20, "
                    + "A5(2)+B5(10) silent (dup 2+10), A6(2)+B6(20) fires {A6,B6} key 2+20, A7(2)+B7(20) "
                    + "silent (dup 2+20)",
            "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key "
                    + "never expires and the identical sequence replays: A1(1) silent, B1(10) fires "
                    + "{A1,B1}, A2(1)+B2(10)+A3(2) silent, B3(10) fires {A3,B3}, A4(1)+B4(20) fires "
                    + "{A4,B4}, A5(2)+B5(10) silent, A6(2)+B6(20) fires {A6,B6}, A7(2)+B7(20) silent",
            "listener; no clock; single expression key coalesce(a.intPrimitive,0)+coalesce(b.intPrimitive,0) "
                    + "over a='A%' or b='B%': A1(1) fires {A1,null} key 1, B1(2) fires {null,B1} key 2, "
                    + "B2(1)+A2(2)+A3(2)+B3(1) silent (keys 1 and 2 seen), B4(3) fires {null,B4} key 3, "
                    + "B5(4) fires {null,B5} key 4, B6(3)+A4(3)+A5(4) silent",
            "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key "
                    + "never expires and the identical sequence replays: A1(1) fires {A1,null}, B1(2) "
                    + "fires {null,B1}, B2(1)+A2(2)+A3(2)+B3(1) silent, B4(3) fires {null,B4}, B5(4) "
                    + "fires {null,B5}, B6(3)+A4(3)+A5(4) silent",
            "listener; no clock; key a.intPrimitive over a='A%' and not 'B%': A1(1) fires, A2(1) "
                    + "swallowed (key 1 seen), A3(2) fires, B1(1) falsifies the attempt and the respawned "
                    + "attempt carries an EMPTY key set so A4(1) fires again on previously-seen key 1, "
                    + "A5(1) silent",
            "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key "
                    + "never expires and the identical sequence replays: A1(1) fires, A2(1) swallowed, "
                    + "A3(2) fires, B1(1) falsifies and resets the key set so A4(1) fires again, A5(1) "
                    + "silent",
            "listener; no clock; expression key a.intPrimitive+b.intPrimitive over a='A%' -> b='B%': "
                    + "A1(1)->B1(1) fires {A1,B1} key 2, A2(1)->B2(1) silent (dup 2), A3(10)->B3(-8) "
                    + "silent (sum 2 again), A4(2) waits alone then B4(1) fires {A4,B4} key 3, "
                    + "A5(3)->B5(0) silent (dup 3)",
            "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key "
                    + "never expires and the identical sequence replays: A1(1)->B1(1) fires {A1,B1}, "
                    + "A2(1)->B2(1) and A3(10)->B3(-8) silent (dup key 2), A4(2)+B4(1) fires {A4,B4}, "
                    + "A5(3)->B5(0) silent",
            "listener; no clock; left-nested every-distinct spawns one waiting branch per fresh "
                    + "a.intPrimitive key, each correlating b on intPrimitive=a.intPrimitive: A1(1) then "
                    + "B1(0) matches nothing, B2(1) fires {A1,B2}, A2(2)/A3(3) spawn branches and A4(1) "
                    + "is dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent (A1 branch consumed), B5(2) "
                    + "fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires {A6,B7}",
            "listener; no clock; `, 2 hours 1 minute` per-key expiry leg — the clock never advances so "
                    + "the key never expires and the identical sequence replays: B1(0) matches nothing, "
                    + "B2(1) fires {A1,B2}, A4(1) dup-swallowed, B3(3) fires {A3,B3}, B4(1) silent, "
                    + "B5(2) fires {A2,B5}, A5(2)+B6(2) silent, A6(4)+B7(4) fires {A6,B7}"};

    private static final String EPL_OVER_AND =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, b.intPrimitive) "
                    + "(a=SupportBean(theString like 'A%') and b=SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_AND_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, b.intPrimitive, 1 hour) "
                    + "(a=SupportBean(theString like 'A%') and b=SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_OR =
            "@name('s0') select * from pattern [every-distinct(coalesce(a.intPrimitive, 0) + "
                    + "coalesce(b.intPrimitive, 0)) (a=SupportBean(theString like 'A%') or "
                    + "b=SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_OR_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(coalesce(a.intPrimitive, 0) + "
                    + "coalesce(b.intPrimitive, 0), 1 hour) (a=SupportBean(theString like 'A%') or "
                    + "b=SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_NOT =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive) "
                    + "(a=SupportBean(theString like 'A%') and not SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_NOT_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 hour) "
                    + "(a=SupportBean(theString like 'A%') and not SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_FOLLOWED_BY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive + b.intPrimitive) "
                    + "(a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%'))]";
    private static final String EPL_OVER_FOLLOWED_BY_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive + b.intPrimitive, 1 hour) "
                    + "(a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%'))]";
    private static final String EPL_WITHIN_FOLLOWED_BY =
            "@name('s0') select * from pattern [(every-distinct(a.intPrimitive) "
                    + "a=SupportBean(theString like 'A%')) -> b=SupportBean(intPrimitive=a.intPrimitive)]";
    private static final String EPL_WITHIN_FOLLOWED_BY_EXPIRY =
            "@name('s0') select * from pattern [(every-distinct(a.intPrimitive, 2 hours 1 minute) "
                    + "a=SupportBean(theString like 'A%')) -> b=SupportBean(intPrimitive=a.intPrimitive)]";

    private static final String[] CASE_EPLS = {
            EPL_OVER_AND, EPL_OVER_AND_EXPIRY,
            EPL_OVER_OR, EPL_OVER_OR_EXPIRY,
            EPL_OVER_NOT, EPL_OVER_NOT_EXPIRY,
            EPL_OVER_FOLLOWED_BY, EPL_OVER_FOLLOWED_BY_EXPIRY,
            EPL_WITHIN_FOLLOWED_BY, EPL_WITHIN_FOLLOWED_BY_EXPIRY};

    // Pinned op sequences (after the case marker), in Java source order.
    // Milestones are regression-harness savepoints restoring identical
    // state and carry no scenario op. No case carries clock ops: ords
    // 8-12 never advance the clock, so each timed-expiry leg replays the
    // identical send sequence as its no-expiry sibling.
    private static final String[][] CASE_OPS = {
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send",
                    "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads, in send order, encoded
    // "SupportBean|theString|intPrimitive".
    private static final String[][] CASE_SENDS = {
            {"SupportBean|A1|1", "SupportBean|B1|10", "SupportBean|A2|1",
                    "SupportBean|B2|10", "SupportBean|A3|2", "SupportBean|B3|10",
                    "SupportBean|A4|1", "SupportBean|B4|20", "SupportBean|A5|2",
                    "SupportBean|B5|10", "SupportBean|A6|2", "SupportBean|B6|20",
                    "SupportBean|A7|2", "SupportBean|B7|20"},
            {"SupportBean|A1|1", "SupportBean|B1|10", "SupportBean|A2|1",
                    "SupportBean|B2|10", "SupportBean|A3|2", "SupportBean|B3|10",
                    "SupportBean|A4|1", "SupportBean|B4|20", "SupportBean|A5|2",
                    "SupportBean|B5|10", "SupportBean|A6|2", "SupportBean|B6|20",
                    "SupportBean|A7|2", "SupportBean|B7|20"},
            {"SupportBean|A1|1", "SupportBean|B1|2", "SupportBean|B2|1",
                    "SupportBean|A2|2", "SupportBean|A3|2", "SupportBean|B3|1",
                    "SupportBean|B4|3", "SupportBean|B5|4", "SupportBean|B6|3",
                    "SupportBean|A4|3", "SupportBean|A5|4"},
            {"SupportBean|A1|1", "SupportBean|B1|2", "SupportBean|B2|1",
                    "SupportBean|A2|2", "SupportBean|A3|2", "SupportBean|B3|1",
                    "SupportBean|B4|3", "SupportBean|B5|4", "SupportBean|B6|3",
                    "SupportBean|A4|3", "SupportBean|A5|4"},
            {"SupportBean|A1|1", "SupportBean|A2|1", "SupportBean|A3|2",
                    "SupportBean|B1|1", "SupportBean|A4|1", "SupportBean|A5|1"},
            {"SupportBean|A1|1", "SupportBean|A2|1", "SupportBean|A3|2",
                    "SupportBean|B1|1", "SupportBean|A4|1", "SupportBean|A5|1"},
            {"SupportBean|A1|1", "SupportBean|B1|1", "SupportBean|A2|1",
                    "SupportBean|B2|1", "SupportBean|A3|10", "SupportBean|B3|-8",
                    "SupportBean|A4|2", "SupportBean|B4|1", "SupportBean|A5|3",
                    "SupportBean|B5|0"},
            {"SupportBean|A1|1", "SupportBean|B1|1", "SupportBean|A2|1",
                    "SupportBean|B2|1", "SupportBean|A3|10", "SupportBean|B3|-8",
                    "SupportBean|A4|2", "SupportBean|B4|1", "SupportBean|A5|3",
                    "SupportBean|B5|0"},
            {"SupportBean|A1|1", "SupportBean|B1|0", "SupportBean|B2|1",
                    "SupportBean|A2|2", "SupportBean|A3|3", "SupportBean|A4|1",
                    "SupportBean|B3|3", "SupportBean|B4|1", "SupportBean|B5|2",
                    "SupportBean|A5|2", "SupportBean|B6|2", "SupportBean|A6|4",
                    "SupportBean|B7|4"},
            {"SupportBean|A1|1", "SupportBean|B1|0", "SupportBean|B2|1",
                    "SupportBean|A2|2", "SupportBean|A3|3", "SupportBean|A4|1",
                    "SupportBean|B3|3", "SupportBean|B4|1", "SupportBean|B5|2",
                    "SupportBean|A5|2", "SupportBean|B6|2", "SupportBean|A6|4",
                    "SupportBean|B7|4"}
    };

    private static final int EXPECTED_STEPS = 138;
    private static final int EXPECTED_RECORDS = 34;

    private PatternEveryDistinctCompound577ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternEveryDistinctCompound577ScenarioOracle <scenario.json>");
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
            runCase(steps, CASES[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
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
                                new DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                        writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                        writer.statement.addListener(writer);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        writer = null;
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
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
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalStateException("unknown eventType: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        JsonValue intPrimitiveValue = payload.get("intPrimitive");
        int intPrimitive = intPrimitiveValue instanceof JsonNumber
                ? ((JsonNumber) intPrimitiveValue).asInt() : 0;
        runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null), intPrimitive),
                "SupportBean");
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds",
                "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
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
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "runtimeId"))
                    || !EXECUTIONS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: the case marker is followed by the
     * pinned ops — deploy s0 with the verbatim EPL, sends with pinned
     * payloads and undeploy-all. Unknown step fields are rejected.
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
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        deploys++;
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String eventType = string(step, "eventType");
                        if (!"SupportBean".equals(eventType)) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "intPrimitive");
                        String actual = eventType + "|"
                                + string(payload, "theString") + "|"
                                + integer(payload, "intPrimitive");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
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
                    || deploys != 1) {
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

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(row(event));
            }
            return output;
        }
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String name : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            values.add(name, normalize(event.get(name)));
        }
        item.add("fields", values);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Double && ((Double) value).isNaN()
                || value instanceof Float && ((Float) value).isNaN()) {
            return new JsonObject().add("state", "nan");
        }
        if (value instanceof BigDecimal) {
            return Json.value(((BigDecimal) value).toPlainString());
        }
        if (value instanceof BigInteger) {
            return Json.value(value.toString());
        }
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean event : (EventBean[]) value) {
                array.add(normalize(event));
            }
            return array;
        }
        if (value instanceof EventBean) {
            JsonObject fields = new JsonObject();
            EventBean event = (EventBean) value;
            for (String name : new TreeSet<>(
                    Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean) {
            // Tagged-event columns arrive as the underlying bean; render
            // the pinned two-field projection exactly like the Go
            // NormalizeResults event rendering.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            TreeSet<String> keys = new TreeSet<>();
            Map<?, ?> mapValue = (Map<?, ?>) value;
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject object = new JsonObject();
            for (String key : keys) {
                object.add(key, normalize(mapValue.get(key)));
            }
            return object;
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        return Json.value(String.valueOf(value));
    }
}
