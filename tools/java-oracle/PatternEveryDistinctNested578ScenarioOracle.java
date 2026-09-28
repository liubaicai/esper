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
import com.espertech.esper.regressionlib.support.bean.SupportEventWithIntArray;
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
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-everydistinct-nested-578
 * bundle: PatternOperatorEveryDistinct ords 4-7 and 16 — every-distinct
 * nested with the repeat and timer:within guards in both orders plus the
 * int-array multi-key execution — replayed as nine cases (ords 4-7 x two
 * expiry legs, ord 16 x one leg), each against a fresh runtime like the
 * regression suite's per-leg deploy.
 *
 * repeat-over-distinct (PatternRepeatOverDistinct, ord 4) deploys
 * {@code [2] every-distinct(a.intPrimitive) a=SupportBean}: the repeat
 * wraps the distinct, so two distinct-key events complete one match —
 * E1(1)+E2(1) are silent (key 1 counted once), E3(2) fires the tag array
 * {a[0]=E1, a[1]=E3}, E4(3)/E5(2) stay silent.
 *
 * timer-within-over-distinct (PatternTimerWithinOverDistinct, ord 5)
 * deploys {@code (every-distinct(a.intPrimitive) a=SupportBean) where
 * timer:within(10 sec)}: the guard wraps the whole distinct, so the
 * statement dies 10 seconds after deploy — E1(1)/E3(2) fire while the
 * guard lives and E4(3)/E5(1) are silent after sendTimer(11000). The
 * `2 days 2 minutes` expiry leg far outlives the guard, so both legs
 * replay the identical sequence.
 *
 * everydistinct-over-repeat (PatternEveryDistinctOverRepeat, ord 6)
 * deploys {@code every-distinct(a[0].intPrimitive) [2] a=SupportBean}:
 * the distinct wraps the repeat, so the key evaluates on the COMPLETED
 * match's first element — E2(1) completes {E1,E2} and fires key 1,
 * E3(1)+E4(2) complete {E3,E4} but the would-be key 1 is a dup, and
 * E5(2)+E6(1) complete {E5,E6} and fire key 2. The expiry leg carries
 * the Java source's doubled key expression verbatim.
 *
 * everydistinct-over-timerwithin (PatternEveryDistinctOverTimerWithin,
 * ord 7) deploys {@code every-distinct(a.intPrimitive) (a=SupportBean
 * where timer:within(10 sec))}: each distinct branch carries its own
 * 10-second window — E1(1)/E3(2) fire, E4-E9 are swallowed while a
 * guarded branch is alive, and after every branch has quit E10(1) fires
 * again on the previously-seen key 1 (EvalEveryDistinctStateNode
 * respawns with a reset key set) and E12(2) fires.
 *
 * multikey-w-array (PatternEveryDistinctMultikeyWArray, ord 16) deploys
 * {@code pattern[every-distinct(a.array) a=SupportEventWithIntArray]} —
 * the Java source carries no space after `pattern` — keyed on int[]
 * content: E1{1,2} fires, E2{1,2} is a content dup, E3{1}/E4{}/E5null
 * fire as new keys (empty and null are their own partitions), and
 * E10-E13 are all swallowed after the omitted milestone(0) savepoint.
 *
 * Java milestone() savepoints restore identical state for these
 * non-contextual executions and are omitted (573/575/576 precedent).
 *
 * SupportBean and SupportEventWithIntArray are the real support beans
 * mirroring the regression suite. Tagged-event columns arrive at the
 * listener as the underlying bean, so the trace renders each bean's
 * pinned field projection ({theString,intPrimitive} or {id,array}) as a
 * kind:row object exactly like the Go NormalizeResults event rendering;
 * repeat legs deliver the tag as an EventBean[] of those rows, and the
 * null int[] key delivers the null marker. Records follow the standard
 * protocol: one listener record per delivered update with a per-case
 * sequence counter starting at 1 and time rendered from the current
 * engine time.
 */
public final class PatternEveryDistinctNested578ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-everydistinct-nested-578";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportEventWithIntArray.java"};

    private static final String DESCRIPTION =
            "PatternOperatorEveryDistinct ords 4-7 and 16 — every-distinct nested with the repeat and "
                    + "timer:within guards in both orders plus the int-array multi-key execution, over "
                    + "SupportBean and SupportEventWithIntArray. repeat-over-distinct "
                    + "(PatternRepeatOverDistinct, ord 4) wraps every-distinct inside [2], so two "
                    + "distinct-key events make one match: E1(1)+E2(1) silent (key 1 counted once), E3(2) "
                    + "fires {a[0]=E1, a[1]=E3}, E4(3)+E5(2) silent — 1 fire. timer-within-over-distinct "
                    + "(PatternTimerWithinOverDistinct, ord 5) wraps every-distinct inside a 10-second "
                    + "guard starting at deploy (t=0): E1(1) fires, E2(1) dup, E3(2) fires, then "
                    + "sendTimer(11000) kills the guard so E4(3)/E5(1) stay silent — 2 fires. "
                    + "everydistinct-over-repeat (PatternEveryDistinctOverRepeat, ord 6) keys the "
                    + "COMPLETED two-event match on a[0].intPrimitive: E1(1) silent, E2(1) fires {E1,E2} "
                    + "key 1, E3(1)+E4(2) silent (would-be key 1 dup), E5(2)+E6(1) fires {E5,E6} key 2 — "
                    + "2 fires; the expiry leg lists the key expression twice before `1 hour`, pinned "
                    + "byte-exact. everydistinct-over-timerwithin (PatternEveryDistinctOverTimerWithin, "
                    + "ord 7) guards each distinct branch with its own 10-second window: E1(1)/E3(2) "
                    + "fire, E4-E9 are swallowed while any guarded branch is alive, and after every "
                    + "branch has quit E10(1) fires again on the previously-seen key 1 (keyset reset on "
                    + "respawn), E12(2) fires, E11(1)/E13(2) dup — 4 fires. multikey-w-array "
                    + "(PatternEveryDistinctMultikeyWArray, ord 16) keys on int[] content: "
                    + "E1{1,2}/E3{1}/E4{}/E5null fire, E2{1,2} is a content dup and E10-E13 are all "
                    + "swallowed — 4 fires; the mid-run milestone(0) savepoint is omitted (it restores "
                    + "identical state). Each of ords 4-7 replays a no-expiry leg then a timed-expiry leg "
                    + "(1 hour; 2 days 2 minutes for ord 5 — both far beyond the send window) with "
                    + "undeployAll between; ord 16 is a single leg.";

    private static final String[] CASES = {
            "repeat-over-distinct", "repeat-over-distinct-expiry",
            "timer-within-over-distinct", "timer-within-over-distinct-expiry",
            "everydistinct-over-repeat", "everydistinct-over-repeat-expiry",
            "everydistinct-over-timerwithin", "everydistinct-over-timerwithin-expiry",
            "multikey-w-array"};
    private static final int[] CASE_ORDINALS = {4, 4, 5, 5, 6, 6, 7, 7, 16};
    private static final int[] CASE_RUNTIME_INDEX = {0, 0, 1, 1, 2, 2, 3, 3, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9ad19ee9b29674b08443",
            "java-runtime-56116a3e1351bcc4dc23",
            "java-runtime-d09d0786090e8a2e387e",
            "java-runtime-a0ae611c1bd46c4407d9",
            "java-runtime-1c2ea40ffffe4e58a323"};
    private static final String[] EXECUTIONS = {
            "PatternRepeatOverDistinct",
            "PatternTimerWithinOverDistinct",
            "PatternEveryDistinctOverRepeat",
            "PatternEveryDistinctOverTimerWithin",
            "PatternEveryDistinctMultikeyWArray"};
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
            "listener; no clock; `[2] every-distinct` nests the distinct inside the repeat: E1(1)+E2(1) "
                    + "silent (key 1 counted once so the match never completes), E3(2) fires "
                    + "{a[0]=E1, a[1]=E3}, E4(3) starts a fresh match and E5(2) is a dup — 1 fire",
            "listener; no clock; `, 1 hour` per-key expiry leg — the clock never advances so the key "
                    + "never expires and the identical sequence replays: E1(1)+E2(1) silent, E3(2) "
                    + "fires {a[0]=E1, a[1]=E3}, E4(3)+E5(2) silent",
            "listener; virtual clock; the 10-second guard wraps the whole every-distinct starting at "
                    + "deploy t=0: E1(1) fires, E2(1) dup, E3(2) fires; sendTimer(11000) kills the "
                    + "guard so E4(3)/E5(1) stay silent — guard-death discriminant, 2 fires",
            "listener; virtual clock; `, 2 days 2 minutes` per-key expiry leg — the expiry far "
                    + "outlives the 10-second guard so the identical sequence replays: E1(1)/E3(2) "
                    + "fire, E4(3)/E5(1) silent after sendTimer(11000)",
            "listener; no clock; `every-distinct [2]` keys the COMPLETED match on a[0].intPrimitive: "
                    + "E1(1) silent, E2(1) fires {E1,E2} key 1, E3(1)+E4(2) silent (would-be key 1 "
                    + "dup), E5(2)+E6(1) fires {E5,E6} key 2 — post-completion key discriminant, "
                    + "2 fires",
            "listener; no clock; `a[0].intPrimitive, a[0].intPrimitive, 1 hour` expiry leg — the "
                    + "Java source lists the key expression twice (pinned byte-exact) and the clock "
                    + "never advances so the identical sequence replays: E2 fires {E1,E2} key 1, E6 "
                    + "fires {E5,E6} key 2",
            "listener; virtual clock; `every-distinct (a where timer:within)` gives each branch its "
                    + "own 10-second window: E1(1)/E3(2) fire, E4-E9 swallowed while a branch lives, "
                    + "then E10(1)@50000 fires again on previously-seen key 1 (keyset reset after the "
                    + "last guarded branch quits) and E12(2) fires, E11(1)/E13(2) dup — within-quit "
                    + "keyset-reset discriminant, 4 fires",
            "listener; virtual clock; `, 1 hour` per-key expiry leg — the key never expires inside "
                    + "the 50-second replay so the identical sequence replays: E1(1)/E3(2)/E10(1)/"
                    + "E12(2) fire, E4-E9 and E11(1)/E13(2) silent",
            "listener; no clock; int[] content key over SupportEventWithIntArray: E1{1,2} fires, "
                    + "E2{1,2} content-dup silent, E3{1} fires, E4{} fires (empty key), E5null fires "
                    + "(null key — its own partition); milestone(0) savepoint omitted; "
                    + "E10{1,2}/E11{1}/E12{}/E13null all swallowed — 4 fires"};

    private static final String EPL_REPEAT_OVER_DISTINCT =
            "@name('s0') select * from pattern [[2] every-distinct(a.intPrimitive) a=SupportBean]";
    private static final String EPL_REPEAT_OVER_DISTINCT_EXPIRY =
            "@name('s0') select * from pattern [[2] every-distinct(a.intPrimitive, 1 hour) a=SupportBean]";
    private static final String EPL_TIMER_WITHIN_OVER_DISTINCT =
            "@name('s0') select * from pattern [(every-distinct(a.intPrimitive) a=SupportBean) "
                    + "where timer:within(10 sec)]";
    private static final String EPL_TIMER_WITHIN_OVER_DISTINCT_EXPIRY =
            "@name('s0') select * from pattern [(every-distinct(a.intPrimitive, 2 days 2 minutes) "
                    + "a=SupportBean) where timer:within(10 sec)]";
    private static final String EPL_EVERYDISTINCT_OVER_REPEAT =
            "@name('s0') select * from pattern [every-distinct(a[0].intPrimitive) [2] a=SupportBean]";
    private static final String EPL_EVERYDISTINCT_OVER_REPEAT_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a[0].intPrimitive, a[0].intPrimitive, "
                    + "1 hour) [2] a=SupportBean]";
    private static final String EPL_EVERYDISTINCT_OVER_TIMER_WITHIN =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive) (a=SupportBean "
                    + "where timer:within(10 sec))]";
    private static final String EPL_EVERYDISTINCT_OVER_TIMER_WITHIN_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 hour) "
                    + "(a=SupportBean where timer:within(10 sec))]";
    private static final String EPL_MULTIKEY_W_ARRAY =
            "@name('s0') select * from pattern[every-distinct(a.array) a=SupportEventWithIntArray]";

    private static final String[] CASE_EPLS = {
            EPL_REPEAT_OVER_DISTINCT, EPL_REPEAT_OVER_DISTINCT_EXPIRY,
            EPL_TIMER_WITHIN_OVER_DISTINCT, EPL_TIMER_WITHIN_OVER_DISTINCT_EXPIRY,
            EPL_EVERYDISTINCT_OVER_REPEAT, EPL_EVERYDISTINCT_OVER_REPEAT_EXPIRY,
            EPL_EVERYDISTINCT_OVER_TIMER_WITHIN, EPL_EVERYDISTINCT_OVER_TIMER_WITHIN_EXPIRY,
            EPL_MULTIKEY_W_ARRAY};

    private static final String AT_0 = "1970-01-01T00:00:00.000Z";
    private static final String AT_5 = "1970-01-01T00:00:05.000Z";
    private static final String AT_10 = "1970-01-01T00:00:10.000Z";
    private static final String AT_11 = "1970-01-01T00:00:11.000Z";
    private static final String AT_15 = "1970-01-01T00:00:15.000Z";
    private static final String AT_20 = "1970-01-01T00:00:20.000Z";
    private static final String AT_25 = "1970-01-01T00:00:25.000Z";
    private static final String AT_50 = "1970-01-01T00:00:50.000Z";

    // Pinned op sequences (after the case marker), in Java source order.
    // Milestones are regression-harness savepoints restoring identical
    // state and carry no scenario op (ord 16's mid-run milestone(0)
    // included). Ords 5/7 pin the Java sendTimer calls as advance-time
    // steps at the absolute instants CASE_ATS lists in order.
    private static final String[][] CASE_OPS = {
            // repeat-over-distinct: E1(1) E2(1) E3(2) E4(3) E5(2)
            {"deploy",
                    "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // repeat-over-distinct-expiry: identical sends
            {"deploy",
                    "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // timer-within-over-distinct: sendTimer(0), deploy,
            // E1(1) E2(1) E3(2), sendTimer(11000), E4(3) E5(1)
            {"advance-time", "deploy",
                    "send", "send", "send",
                    "advance-time",
                    "send", "send",
                    "undeploy-all"},
            // timer-within-over-distinct-expiry: identical sequence
            {"advance-time", "deploy",
                    "send", "send", "send",
                    "advance-time",
                    "send", "send",
                    "undeploy-all"},
            // everydistinct-over-repeat: E1(1) E2(1) E3(1) E4(2) E5(2)
            // E6(1)
            {"deploy",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // everydistinct-over-repeat-expiry: identical sends
            {"deploy",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // everydistinct-over-timerwithin: sendTimer(0), deploy,
            // E1(1) E2(1); @5s E3(2); @10s E4(1) E5(1) E6(2); @15s E7(2);
            // @20s E8(2); @25s E9(1); @50s E10(1) E11(1) E12(2) E13(2)
            {"advance-time", "deploy",
                    "send", "send",
                    "advance-time", "send",
                    "advance-time", "send", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time",
                    "send", "send", "send", "send",
                    "undeploy-all"},
            // everydistinct-over-timerwithin-expiry: identical sequence
            {"advance-time", "deploy",
                    "send", "send",
                    "advance-time", "send",
                    "advance-time", "send", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time",
                    "send", "send", "send", "send",
                    "undeploy-all"},
            // multikey-w-array: E1{1,2} E2{1,2} E3{1} E4{} E5null
            // (milestone(0) omitted) E10{1,2} E11{1} E12{} E13null
            {"deploy",
                    "send", "send", "send", "send", "send",
                    "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned advance-time instants per case, in op order.
    private static final String[][] CASE_ATS = {
            {}, {},
            {AT_0, AT_11}, {AT_0, AT_11},
            {}, {},
            {AT_0, AT_5, AT_10, AT_15, AT_20, AT_25, AT_50},
            {AT_0, AT_5, AT_10, AT_15, AT_20, AT_25, AT_50},
            {}
    };

    // Pinned send payloads, in send order: SupportBean sends encode
    // "SupportBean|theString|intPrimitive" and SupportEventWithIntArray
    // sends encode "SupportEventWithIntArray|id|csv" with "null" for the
    // Java null int[].
    private static final String[][] CASE_SENDS = {
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|2"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|2"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|1"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|1"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|1",
                    "SupportBean|E4|2", "SupportBean|E5|2", "SupportBean|E6|1"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|1",
                    "SupportBean|E4|2", "SupportBean|E5|2", "SupportBean|E6|1"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|1", "SupportBean|E5|1", "SupportBean|E6|2",
                    "SupportBean|E7|2", "SupportBean|E8|2", "SupportBean|E9|1",
                    "SupportBean|E10|1", "SupportBean|E11|1", "SupportBean|E12|2",
                    "SupportBean|E13|2"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|1", "SupportBean|E5|1", "SupportBean|E6|2",
                    "SupportBean|E7|2", "SupportBean|E8|2", "SupportBean|E9|1",
                    "SupportBean|E10|1", "SupportBean|E11|1", "SupportBean|E12|2",
                    "SupportBean|E13|2"},
            {"SupportEventWithIntArray|E1|1,2", "SupportEventWithIntArray|E2|1,2",
                    "SupportEventWithIntArray|E3|1", "SupportEventWithIntArray|E4|",
                    "SupportEventWithIntArray|E5|null", "SupportEventWithIntArray|E10|1,2",
                    "SupportEventWithIntArray|E11|1", "SupportEventWithIntArray|E12|",
                    "SupportEventWithIntArray|E13|null"}
    };

    private static final int EXPECTED_STEPS = 112;
    private static final int EXPECTED_RECORDS = 22;

    private PatternEveryDistinctNested578ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternEveryDistinctNested578ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportEventWithIntArray.class);

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
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            JsonValue intPrimitiveValue = payload.get("intPrimitive");
            int intPrimitive = intPrimitiveValue instanceof JsonNumber
                    ? ((JsonNumber) intPrimitiveValue).asInt() : 0;
            runtime.getEventService().sendEventBean(
                    new SupportBean(payload.getString("theString", null), intPrimitive),
                    "SupportBean");
            return;
        }
        if ("SupportEventWithIntArray".equals(eventType)) {
            String id = payload.getString("id", null);
            JsonValue arrayValue = payload.get("array");
            int[] array = null;
            if (arrayValue != null && arrayValue.isArray()) {
                JsonArray items = arrayValue.asArray();
                array = new int[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    array[index] = items.get(index).asInt();
                }
            }
            runtime.getEventService().sendEventBean(
                    new SupportEventWithIntArray(id, array),
                    "SupportEventWithIntArray");
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
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
     * pinned ops — advance-time steps at the pinned instants, deploy s0
     * with the verbatim EPL, sends with pinned payloads and undeploy-all.
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
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            int advances = 0;
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
                    case "send": {
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        String eventType = string(step, "eventType");
                        String actual;
                        if ("SupportBean".equals(eventType)) {
                            requireFields(payload, "theString", "intPrimitive");
                            actual = eventType + "|"
                                    + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive");
                        } else if ("SupportEventWithIntArray".equals(eventType)) {
                            requireFields(payload, "id", "array");
                            StringBuilder content = new StringBuilder();
                            JsonValue arrayValue = payload.get("array");
                            if (arrayValue.isArray()) {
                                JsonArray items = arrayValue.asArray();
                                for (int index = 0; index < items.size(); index++) {
                                    if (index > 0) {
                                        content.append(',');
                                    }
                                    content.append(items.get(index).asInt());
                                }
                            } else {
                                content.append("null");
                            }
                            actual = eventType + "|"
                                    + string(payload, "id") + "|"
                                    + content;
                        } else {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_ATS[caseIndex][advances++].equals(string(step, "at"))) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " is not pinned");
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
            if (sends != CASE_SENDS[caseIndex].length || advances != CASE_ATS[caseIndex].length
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
                array.add(normalize(event.getUnderlying()));
            }
            return array;
        }
        if (value instanceof EventBean) {
            return normalize(((EventBean) value).getUnderlying());
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
        if (value instanceof SupportEventWithIntArray) {
            // Same pinned-projection protocol for the ord 16 bean: the id
            // plus the int[] rendered element-wise (a null array delivers
            // the null marker), matching the Go []int rendering.
            SupportEventWithIntArray bean = (SupportEventWithIntArray) value;
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            fields.add("array", intArray(bean.getArray()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof int[]) {
            return intArray((int[]) value);
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Iterable<?>) {
            JsonArray array = new JsonArray();
            for (Object item : (Iterable<?>) value) {
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
    private static JsonValue intArray(int[] value) {
        if (value == null) {
            // The Go event getter surfaces a nil []int as JSON null
            // (not the null-marker object), so the oracle renders the
            // Java null int[] identically.
            return Json.NULL;
        }
        JsonArray array = new JsonArray();
        for (int item : value) {
            array.add(item);
        }
        return array;
    }
}
