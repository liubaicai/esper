import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expression;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.Filter;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.PatternExpr;
import com.espertech.esper.common.client.soda.PatternStream;
import com.espertech.esper.common.client.soda.Patterns;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportBean_C;
import com.espertech.esper.regressionlib.support.bean.SupportBean_D;
import com.espertech.esper.regressionlib.support.bean.SupportBean_E;
import com.espertech.esper.regressionlib.support.bean.SupportBean_F;
import com.espertech.esper.regressionlib.support.bean.SupportBean_G;
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
import java.util.Collections;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-guard-timerwithin-wharness-594
 * unit: PatternGuardTimerWithin ord 0 PatternOp — the
 * single-execution 34-leg timer:within W-harness over
 * EventCollectionFactory.getEventSetOne(0, 1000), replayed as ONE case
 * against a fresh runtime like the regression suite's per-execution
 * environment.
 *
 * The PatternTestHarness deploys all thirty-four EventExpressionCase
 * statements BEFORE replaying the event set; the scenario encodes that
 * phase as ONE deploy step (statement "all", the leg texts pinned in
 * case.epls) which this oracle expands into the harness's thirty-four
 * individual compile+deploy+listener registrations — statement names
 * S0..S33 standing in for the harness's name--&lt;atom&gt; labels
 * (StringEscapeUtils.escapeJava is an identity on these atoms). Legs
 * other than S3 deploy as {@code @name("S<i>") select * from pattern
 * [<atom>]}; S3 is the harness's SODA-model leg and deploys through the
 * same model path (a compiled EPStatementObjectModel built as
 * Patterns.timerWithin(10.001, Patterns.filter(Filter.create(...), "b"))
 * whose toEPL is the pinned
 * `b=SupportBean_B(id="B3") where timer:within(10.001d)` text — the `d`
 * suffix is the double literal marker, so the guard period is 10.001
 * seconds, the same 10001ms deadline as S4). The harness's
 * @Audit('pattern')/@Audit('pattern-instances') annotations carry no
 * observable listener payload. Every send step performs the harness's
 * advanceTime(currentTime) BEFORE sendEventBean — the {@code at}
 * instant advances the clock first so guard expiry callbacks inside an
 * advance run at the advance instant BEFORE the upcoming event: an
 * event landing exactly at arm+period loses (the deadline is
 * exclusive — S0's B1 at 2000 and S5's B3 at 10000 hit the exact
 * boundary) while a child spawned inside an expiry callback is live
 * for the same-tick event (S23's respawn-at-6000 sees D1). Every send's
 * fires arrive per leg in one listener invocation carrying all rows of
 * that delivery; the harness compares each leg's expected
 * EventDescriptors as a MULTISET (compareLists), so per-statement
 * bucketing and multiset semantics — not global cross-statement or
 * within-batch ordering — carry the contract.
 * `every X where G` parses as every(X where G): the guarded child
 * serially retries after each expiry or match (S9/S10/S15 deliver
 * B1/B2/B3). `(every X) where G` guards the whole every, so expiry
 * kills the loop (S7 dies at 2001 after B1; S14 dies at 4001 after B2).
 * `every ((every X) where G)` accumulates: the inner every's filter
 * child never quits so the guarded instance stays live and the outer
 * every spawns a sibling per match — B1×1, B2×2, B3×4 (S11/S12). The
 * `and` legs die when either side's guard expires (S20/S21/S33) but a
 * live and retains the completed side for later pairing — S18/S29
 * deliver {b:B3, d:D2} into the B3 bucket because D2 was retained from
 * its own send. The `or` legs complete on the first match and kill the
 * sibling (S30's {b:B1}) but survive the OTHER side's expiry to
 * deliver a single-tag row (S31's {d:D1} with b unbound). Legs
 * S0/S2/S5/S6/S17/S20/S21/S24/S27/S28/S33 are pinned silence legs: the
 * guard quits at or before the would-be trigger (S0/S5 hit the exact
 * arm+period deadline; S6's 9.999 guard quits inside the B3 advance;
 * S17's 4 sec sequence guard dies at D1's exact deadline; S19's twin
 * with the 6000 msec d-guard dies inside the D1 advance as S20;
 * S24's 3999 msec sequence guard quits before D1; the nested or of
 * S27/S28 and the guarded-every-and of S33 quit inside their first
 * advance).
 *
 * After undeploy-all the harness resends the whole event set and
 * asserts listener silence plus statement resolution returning null —
 * mirrored verbatim. The COMPILE_TO_MODEL, COMPILE_TO_EPL and
 * USE_EPL_AND_CONSUME_NOCHECK replay styles and all env.milestone
 * savepoints are harness machinery outside the replayed surface and
 * are unrepresented. The internal timer is disabled for determinism;
 * every listener record reports the virtual time at the triggering
 * send.
 *
 * {@code select *} projects every named tag's fragment as the single
 * bean that fired it (an unbound or-branch tag projects null),
 * matching the Go PatternEvent projection.
 */
public final class PatternGuardTimerWithinWHarness594ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-guard-timerwithin-wharness-594";
    private static final String DESCRIPTION =
            "PatternGuardTimerWithin ord 0 PatternOp — the shared 34-leg timer:within "
                    + "W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed as "
                    + "ONE case: ONE deploy-all step (statement \"all\") expands to the harness's "
                    + "thirty-four per-leg deployments of `@name(\"S<i>\") select * from pattern "
                    + "[<atom>]` (S0..S33 stand in for the name--<atom> labels, S3 carrying the "
                    + "SODA model's toEPL text `b=SupportBean_B(id=\"B3\") where "
                    + "timer:within(10.001d)`), then twelve "
                    + "advance-before-send steps each advance the external clock to the event's "
                    + "pinned instant (+1000ms per send) BEFORE sendEventBean — guard expiry "
                    + "callbacks inside an advance run at the advance instant BEFORE the event, "
                    + "so an event at exactly arm+period loses (the deadline is exclusive) and a "
                    + "child respawned inside an expiry callback is live for the same-tick event "
                    + "(S23's respawn-at-6000 sees D1). Expected: 64 listener records — buckets "
                    + "B1=11, B2=8, D1=12, D2=4, B3=18, D3=9; silent legs "
                    + "S0,S2,S5,S6,S17,S20,S21,S24,S27,S28,S33 produce zero rows. env.milestone "
                    + "savepoints, the three other replay styles and the ON_START advance-time "
                    + "carry no scenario op; per leg+trigger the fires compare as a multiset like "
                    + "compareLists.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternGuardTimerWithin.java";
    private static final String[] JAVA_SOURCE_FILES = {
            JAVA_SOURCE,
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "EventCollectionFactory.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "EventExpressionCase.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "PatternTestHarness.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_E.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_F.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_G.java"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-bb8113cb979826cff927"
    };
    private static final String[] EXECUTIONS = {
            "PatternOp"
    };
    private static final String[] STATIC_IDS = {
            "java-0dec801a426fed297402"
    };

    // Verbatim transcription of PatternGuardTimerWithin ord 0
    // (lines 52-223): the thirty-four EventExpressionCase atoms in
    // case-list order. Spacing is pinned byte-exact — the
    // `sec`/`msec`/`milliseconds` unit spellings, the bare decimal
    // seconds (`2.001`, `4.001`, `6.001`, `9.999`, `11.001`), the
    // `()` empty-filter markers, `every` placement, `or`/`and`/`->`
    // punctuation and the spaced `timer:within (...)` forms of
    // S18/S19/S20/S21/S27/S28/S29/S30/S31/S32/S33 keep the Java source
    // text. S3's atom is the model leg's toEPL text (the harness pins
    // the `10.001d` double-literal suffix that way, the same 10001ms
    // guard as S4's `10001 msec`).
    private static final String[] ATOMS = {
            "b=SupportBean_B(id='B1') where timer:within(2 sec)",
            "b=SupportBean_B(id='B1') where timer:within(2001 msec)",
            "b=SupportBean_B(id='B1') where timer:within(1999 msec)",
            "b=SupportBean_B(id=\"B3\") where timer:within(10.001d)",
            "b=SupportBean_B(id='B3') where timer:within(10001 msec)",
            "b=SupportBean_B(id='B3') where timer:within(10 sec)",
            "b=SupportBean_B(id='B3') where timer:within(9.999)",
            "(every b=SupportBean_B) where timer:within(2.001)",
            "(every b=SupportBean_B) where timer:within(4.001)",
            "every b=SupportBean_B where timer:within(2.001)",
            "every (b=SupportBean_B where timer:within(2001 msec))",
            "every ((every b=SupportBean_B) where timer:within(2.001))",
            "every ((every b=SupportBean_B) where timer:within(6.001))",
            "(every b=SupportBean_B) where timer:within(11.001)",
            "(every b=SupportBean_B) where timer:within(4001 milliseconds)",
            "every (b=SupportBean_B) where timer:within(6.001)",
            "b=SupportBean_B -> d=SupportBean_D where timer:within(4001 milliseconds)",
            "b=SupportBean_B() -> d=SupportBean_D() where timer:within(4 sec)",
            "every (b=SupportBean_B() where timer:within (4.001) and d=SupportBean_D() where timer:within(6.001))",
            "b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6001 msec)",
            "b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6000 msec)",
            "b=SupportBean_B() where timer:within (2000 msec) and d=SupportBean_D() where timer:within(6001 msec)",
            "every b=SupportBean_B -> d=SupportBean_D where timer:within(4000 msec)",
            "every b=SupportBean_B() -> every d=SupportBean_D where timer:within(4000 msec)",
            "b=SupportBean_B() -> d=SupportBean_D() where timer:within(3999 msec)",
            "every b=SupportBean_B() -> (every d=SupportBean_D) where timer:within(2001 msec)",
            "every (b=SupportBean_B() -> d=SupportBean_D()) where timer:within(6001 msec)",
            "b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6000 msec)",
            "(b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6000 msec)) where timer:within (1999 msec)",
            "every (b=SupportBean_B() where timer:within (2001 msec) and d=SupportBean_D() where timer:within(6001 msec))",
            "b=SupportBean_B() where timer:within (2001 msec) or d=SupportBean_D() where timer:within(6001 msec)",
            "b=SupportBean_B() where timer:within (2000 msec) or d=SupportBean_D() where timer:within(6001 msec)",
            "every b=SupportBean_B() where timer:within (2001 msec) and every d=SupportBean_D() where timer:within(6001 msec)",
            "(every b=SupportBean_B) where timer:within (2000 msec) and every d=SupportBean_D() where timer:within(6001 msec)"
    };

    // Index of the SODA-model leg: the harness builds S3's case from an
    // EPStatementObjectModel rather than EPL text, so the oracle deploys
    // it through the same module path.
    private static final int SODA_LEG = 3;

    // Pinned deployed statement text per leg: `@name("S<i>") select *
    // from pattern [<atom>]` — S0..S33 replace the harness's
    // name--<atom> labels (escapeJava is an identity on these atoms)
    // and the pattern-audit annotations are listener-invisible.
    private static final String[] CASE_EPLS;
    private static final String[] LEG_NAMES;

    static {
        CASE_EPLS = new String[ATOMS.length];
        LEG_NAMES = new String[ATOMS.length];
        for (int index = 0; index < ATOMS.length; index++) {
            LEG_NAMES[index] = "S" + index;
            CASE_EPLS[index] = "@name(\"" + LEG_NAMES[index]
                    + "\") select * from pattern [" + ATOMS[index] + "]";
        }
    }

    private static final String[] CASES = {"w-harness"};
    private static final int[] CASE_ORDINALS = {0};
    private static final int[] CASE_RUNTIME_INDEX = {0};
    private static final String[] OBSERVATIONS = {
            "listener; external clock +1000ms per send; all 34 legs deploy in ONE step "
                    + "before any send (harness deploy-all), then twelve advance-before-send "
                    + "replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes "
                    + "sendEventBean so guard-expiry callbacks run at the advance instant — "
                    + "the exclusive deadline kills a leg whose event lands exactly at "
                    + "arm+period, and a child respawned inside an expiry callback is live "
                    + "for the same-tick event): S0/S2/S5/S6 silent boundary pairs, S1 B1, "
                    + "S3/S4 B3 (SODA 10.001d == 10001 msec), S7 B1 only (guarded every dies "
                    + "at 2001), S8 B1+B2 (dies at 4001), S9/S10/S15 B1+B2+B3 (serial "
                    + "every(guarded b) retry), S11/S12 accumulation @B1×1,@B2×2,@B3×4 "
                    + "(nested every over guarded every), S13 B1+B2+B3, S14 B1+B2 (dies at "
                    + "4001), S16 D1{b:B1,d:D1} (d arm at B1+0), S17/S20/S21/S24/S27/S28/S33 "
                    + "silent, S18/S29 @D1 {B1,D1} + @B3 {B3,D2} (and-side expiry-respawn "
                    + "retains D2 for B3's pairing), S19 @D1 {B1,D1} once, S22 @D1 {B2,D1} + "
                    + "@D3 {B3,D3} (per-B serial every LHS), S23 seven rows (per-branch every "
                    + "d retry; respawn at 6000 sees D1), S25 @D1 {B2,D1} + @D3 {B3,D3} "
                    + "(guarded inner every dies permanently at expiry), S26 @D1 {B1,D1} + "
                    + "@D3 {B3,D3} (every over guarded sequence), S30 @B1 {b:B1} (or wins on "
                    + "first match), S31 @D1 {d:D1} (or survives the b side's expiry with b "
                    + "unbound), S32 nine rows (every×every and cross product); per leg+trigger "
                    + "the fires compare as a multiset like compareLists"
    };

    // Pinned op sequence (after the case marker): ONE deploy-all step
    // expands to the thirty-four per-leg deployments, then twelve fused
    // advance-before-send steps (the harness advances the clock to the
    // event's instant before sendEventBean) and the closing
    // undeploy-all whose post-undeploy resend is a silence check.
    private static final String[][] CASE_OPS = {{
            "deploy",
            "send", "send", "send", "send", "send", "send",
            "send", "send", "send", "send", "send", "send",
            "undeploy-all"
    }};

    // Pinned advance instants (epoch millis) inside each send step:
    // the harness's sendEventCollection.getTime(eventId) advance —
    // A1=1000 .. D3=12000.
    private static final long[][] CASE_ADVANCES = {{
            1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000, 11000, 12000
    }};

    // Pinned sends ("<eventType>|<id>") in send order — the
    // EventCollectionFactory.makeMixedSet insertion order.
    private static final String[][] CASE_SENDS = {{
            "SupportBean_A|A1", "SupportBean_B|B1", "SupportBean_C|C1",
            "SupportBean_B|B2", "SupportBean_A|A2", "SupportBean_D|D1",
            "SupportBean_E|E1", "SupportBean_F|F1", "SupportBean_D|D2",
            "SupportBean_B|B3", "SupportBean_G|G1", "SupportBean_D|D3"
    }};

    private static final int EXPECTED_STEPS = 15;
    // The harness pins sixty-four testCase.add(...) EventDescriptors
    // (S1=1, S3=1, S4=1, S7=1, S8=2, S9=3, S10=3, S11=7, S12=7, S13=3,
    // S14=2, S15=3, S16=1, S18=2, S19=1, S22=2, S23=7, S25=2, S26=2,
    // S29=2, S30=1, S31=1, S32=9) as expected rows. Adds under the same
    // (statement, trigger) arrive in ONE listener invocation — the
    // harness's checkResults compares SupportListener.getLastNewData,
    // the whole newEvents array of that delivery — so the trace's
    // records are per-statement-per-trigger invocations: S11/S12's
    // B2×2 and B3×4 adds collapse to two and one record, S23's
    // D1×2/D2×2/D3×3 to three, S32's D1×2/D2×2/B3×2/D3×3 to four,
    // leaving 47 records carrying the 64 rows.
    private static final int EXPECTED_RECORDS = 47;

    private PatternGuardTimerWithinWHarness594ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternGuardTimerWithinWHarness594ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportBean_B.class);
        configuration.getCommon().addEventType(SupportBean_C.class);
        configuration.getCommon().addEventType(SupportBean_D.class);
        configuration.getCommon().addEventType(SupportBean_E.class);
        configuration.getCommon().addEventType(SupportBean_F.class);
        configuration.getCommon().addEventType(SupportBean_G.class);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            boolean active = false;
            int deployIndex = 0;
            List<String> deploymentIds = new ArrayList<>();
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
                        // ONE deploy-all step expands to the harness's
                        // thirty-four individual deployments — statement
                        // names S0..S33, one listener each, like
                        // env.deploy(unit).addListener(nameOfStatement).
                        for (int leg = 0; leg < CASE_EPLS.length; leg++) {
                            EPCompiled compiled = compileLeg(runtime, leg);
                            EPDeployment deployment = runtime.getDeploymentService().deploy(
                                    compiled, new DeploymentOptions()
                                            .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                                    + "-" + (deployIndex++)));
                            deploymentIds.add(deployment.getDeploymentId());
                            EPStatement statement = findStatement(deployment, LEG_NAMES[leg]);
                            statement.addListener(
                                    new TraceWriter(records, caseName, statement, runtime));
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        // The harness resends the whole set after
                        // undeployAll to prove silence and asserts the
                        // statements resolve to null.
                        for (String send : CASE_SENDS[caseIndex]) {
                            resendSilently(runtime, send);
                        }
                        for (int leg = 0; leg < LEG_NAMES.length; leg++) {
                            if (runtime.getDeploymentService().getStatement(
                                    deploymentIds.get(leg), LEG_NAMES[leg]) != null) {
                                throw new IllegalStateException(
                                        "statement " + LEG_NAMES[leg] + " survived undeploy-all");
                            }
                        }
                        break;
                    case "send":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        sendEvent(runtime, step);
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

    // compileLeg mirrors the harness's per-case deployment: legs other
    // than S3 compile the pinned `@name("S<i>") select * from pattern
    // [<atom>]` text while S3 compiles the SODA model leg — the harness
    // pins `select * from pattern [b=SupportBean_B(id="B3") where
    // timer:within(10.001d)]` as the model's toEPL output (the `d` is a
    // double-literal suffix, so the guard is the same 10001ms deadline
    // as S4). The model construction order — select clause,
    // serialization copy, then the timer:within-over-filter from
    // clause — follows PatternGuardTimerWithin.java lines 62-69
    // verbatim.
    private static EPCompiled compileLeg(EPRuntime runtime, int leg) throws Exception {
        if (leg == SODA_LEG) {
            String text = "select * from pattern [" + ATOMS[SODA_LEG] + "]";
            EPStatementObjectModel model = new EPStatementObjectModel();
            model.setSelectClause(SelectClause.createWildcard());
            model = SerializableObjectCopier.copyMayFail(model);
            Expression filter = Expressions.eq("id", "B3");
            PatternExpr pattern = Patterns.timerWithin(10.001,
                    Patterns.filter(Filter.create("SupportBean_B", filter), "b"));
            model.setFromClause(FromClause.create(PatternStream.create(pattern)));
            if (!text.equals(model.toEPL())) {
                throw new IllegalStateException(
                        "SODA leg toEPL is not pinned: " + model.toEPL());
            }
            model.setAnnotations(Collections.singletonList(
                    AnnotationPart.nameAnnotation(LEG_NAMES[SODA_LEG])));
            Module module = new Module();
            module.getItems().add(new ModuleItem(model));
            module.setModuleText(model.toEPL());
            return EPCompilerProvider.getCompiler().compile(
                    module, new CompilerArguments(runtime.getRuntimePath()));
        }
        return EPCompilerProvider.getCompiler().compile(
                CASE_EPLS[leg], new CompilerArguments(runtime.getRuntimePath()));
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static void resendSilently(EPRuntime runtime, String pinned) {
        String eventType = pinned.substring(0, pinned.indexOf('|'));
        String id = pinned.substring(pinned.indexOf('|') + 1);
        sendTyped(runtime, eventType, id);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        String id = payload.getString("id", null);
        sendTyped(runtime, eventType, id);
    }

    private static void sendTyped(EPRuntime runtime, String eventType, String id) {
        switch (eventType) {
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(new SupportBean_A(id), "SupportBean_A");
                return;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(new SupportBean_B(id), "SupportBean_B");
                return;
            case "SupportBean_C":
                runtime.getEventService().sendEventBean(new SupportBean_C(id), "SupportBean_C");
                return;
            case "SupportBean_D":
                runtime.getEventService().sendEventBean(new SupportBean_D(id), "SupportBean_D");
                return;
            case "SupportBean_E":
                runtime.getEventService().sendEventBean(new SupportBean_E(id), "SupportBean_E");
                return;
            case "SupportBean_F":
                runtime.getEventService().sendEventBean(new SupportBean_F(id), "SupportBean_F");
                return;
            case "SupportBean_G":
                runtime.getEventService().sendEventBean(new SupportBean_G(id), "SupportBean_G");
                return;
            default:
                throw new IllegalStateException("unknown eventType: " + eventType);
        }
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
                    "observation", "epls");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "runtimeId"))
                    || !EXECUTIONS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
            validateStringArray(definition.get("epls"), CASE_EPLS, "epls");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: the case marker is followed by the
     * pinned ops — ONE deploy-all step (statement "all"), the twelve
     * fused advance-before-send steps with pinned instants/payloads
     * and the closing undeploy-all. Unknown step fields are rejected.
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
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement");
                        if (!"all".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send": {
                        requireFields(step, "op", "case", "at", "eventType", "payload");
                        long expected = CASE_ADVANCES[caseIndex][sends];
                        long actual = Instant.parse(string(step, "at")).toEpochMilli();
                        if (actual != expected) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " advance instant is not pinned: expected " + expected
                                    + " got " + actual);
                        }
                        String expectedSend = CASE_SENDS[caseIndex][sends++];
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        String eventType = string(step, "eventType");
                        switch (eventType) {
                            case "SupportBean_A":
                            case "SupportBean_B":
                            case "SupportBean_C":
                            case "SupportBean_D":
                            case "SupportBean_E":
                            case "SupportBean_F":
                            case "SupportBean_G":
                                requireFields(payload, "id");
                                break;
                            default:
                                throw new IllegalArgumentException("send step " + cursor
                                        + " is not pinned");
                        }
                        String actualSend = eventType + "|" + string(payload, "id");
                        if (!expectedSend.equals(actualSend)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expectedSend
                                    + " got " + actualSend);
                        }
                        break;
                    }
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length) {
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
        JsonValue value = object.get(name);
        if (!(value instanceof com.espertech.esper.common.client.json.minimaljson.JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            long parsed = Long.parseLong(text, 10);
            if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " is outside the Java int range");
            }
            return (int) parsed;
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
        if (value instanceof SupportBean_A) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_A) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_B) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_B) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_C) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_C) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_D) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_D) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_E) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_E) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_F) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_F) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_G) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_G) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
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
