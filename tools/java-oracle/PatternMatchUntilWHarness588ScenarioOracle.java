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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
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
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-matchuntil-wharness-588
 * unit: PatternOperatorMatchUntil ord 1 PatternOp — the shared 52-leg
 * match-until W-harness over EventCollectionFactory.getEventSetOne(0,
 * 1000), replayed as ONE case against a fresh runtime like the
 * regression suite's per-execution environment.
 *
 * The PatternTestHarness deploys all fifty-two EventExpressionCase
 * statements BEFORE replaying the event set; the scenario encodes that
 * phase as ONE deploy step (statement "all", the fifty-two leg texts
 * pinned in case.epls) which this oracle expands into the harness's
 * fifty-two individual compile+deploy+listener registrations —
 * statement names S0..S51 standing in for the harness's
 * name--&lt;atom&gt; labels (StringEscapeUtils.escapeJava is an
 * identity on these atoms). Each leg deploys as
 * {@code @name("S<i>") select * from pattern [<atom>]} — the harness's
 * @Audit('pattern')/@Audit('pattern-instances') annotations carry no
 * observable listener payload. Every send step then performs the
 * harness's advanceTime(currentTime) BEFORE sendEventBean — the
 * {@code at} instant advances the clock first so timer legs whose
 * expirations land inside an advance (S39 d until timer:interval(7
 * sec) at E1; S43 `a until every(timer:interval(6 sec) and not A)` at
 * G1; S44 `A until every(timer:interval(7 sec) and not A)` at D3)
 * attribute to the upcoming event's bucket exactly like the harness's
 * checkResults-after-sendEventBean ordering. Every send's fires arrive
 * per leg in one listener invocation per completing delivery; the
 * harness compares each leg's expected EventDescriptors as a MULTISET
 * (compareLists), so per-statement bucketing and multiset semantics —
 * not global cross-statement ordering — carry the contract. Leg S51
 * `SupportBean_B until not SupportBean_B` can fire on start; the
 * harness ignores start fires by design (the start event carries no
 * information) and the EPL statement surface does not deliver them to
 * listeners, so S51 contributes zero records like the other silent
 * legs S7/S12/S15/S16/S24/S26/S27/S28.
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
 * {@code select *} projects every named tag's fragment — the repeated
 * tag as an EventBean[] (empty when no match collected) and each
 * single tag as its bean fragment, rendering null when unbound —
 * matching the Go TagEvents/PatternEvent projection.
 */
public final class PatternMatchUntilWHarness588ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-matchuntil-wharness-588";
    private static final String DESCRIPTION =
            "PatternOperatorMatchUntil ord 1 PatternOp — the shared 52-leg match-until "
                    + "W-harness over EventCollectionFactory.getEventSetOne(0,1000), replayed "
                    + "as ONE case: ONE deploy-all step (statement \"all\") expands to the "
                    + "harness's fifty-two per-leg deployments of `@name(\"S<i>\") select * "
                    + "from pattern [<atom>]` (S0..S51 stand in for the name--<atom> labels), "
                    + "then twelve advance-before-send steps each advance the external clock "
                    + "to the event's pinned instant (+1000ms per send) BEFORE sendEventBean — "
                    + "timer expirations inside an advance attribute to the upcoming event's "
                    + "bucket (S39 d until timer:interval(7 sec) -> E1, S43 a until "
                    + "every(timer:interval(6 sec) and not A) -> G1, S44 A until "
                    + "every(timer:interval(7 sec) and not A) -> D3) — and every send's fires "
                    + "are grouped per leg+trigger as multiset rows. Expected fires: S0 "
                    + "D1{a:A2}, S1 D1{a:A1,A2}, S2 A1{b:empty,a:A1}, S3 D3{b:B1,B2,B3}, S4 "
                    + "D3{a:A1,A2;b:B1,B2,B3;d:D3}, S5 D1{a:A1,A2;b:B1,B2;d:D1}, S6 A1{a:A1}, "
                    + "S7 silent, S8/S9 A2{a:A1,A2}, S10/S11 A1{a:A1}, S12 silent, S13 "
                    + "B3{b:B1,B2,B3}, S14 A2{a:A1,A2;b:B1,B2}, S15/S16 silent (until kills the "
                    + "sub-minimum repetition permanently), S17 B2{b:B1,B2} (tight [2:2] fires "
                    + "at the bound), S18/S19 G1{b:B1,B2,B3;g:G1}, S20 G1{b:B1,B2;g:G1}, S21 "
                    + "G1{b:B1;g:G1}, S22 A1{b:empty,a:A1}, S23 G1{b:B1,B2,B3;g:G1}, S24 silent "
                    + "(terminator below minimum), S25 A2{b:B1,B2;a:A2}, S26 silent, S27/S28 "
                    + "silent (until wins on a shared event), S29 A2{b:B1,B2;a:A2}, S30 "
                    + "G1{b:B1,B2,B3}, S31 G1{b:B1,B2}, S32 F1{b:B1,B2}, S33/S34 C1{b:B1}, S35 "
                    + "D3{c:C1;b:B2,B3;d:D3}, S36 B3{b:B1,B2,B3}, S37 D3{d:D1,D2,D3}, S38 "
                    + "D2{b:B1,B2;d:D1,D2}, S39 E1{d:D1}, S40 B1{b:B1}/B2{b:B2}/B3{b:B3;d:D1,D2}, "
                    + "S41/S42 B1{b:B1} (every binds tighter than until), S43 G1{a:A1,A2}, S44 "
                    + "D3{} (fire, no tags), S45 B1{a:A1;b:B1}, S46 A2{a:A1,A2}, S47 "
                    + "D1{a:A1,A2;d:D1} (ESPER-339 every precedence), S48 B2{a:A1;b:B1,B2}, S49 "
                    + "C1{a:A1;b:B1;c:C1}, S50 G1{a:A1,A2;b:B1,B2,B3;g:G1}, S51 silent "
                    + "(start-fire ignored by the harness). The COMPILE_TO_MODEL/"
                    + "COMPILE_TO_EPL/consume+suppress harness styles, env.milestone savepoints "
                    + "and the post-undeploy resend silence check are unrepresented harness "
                    + "machinery.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternOperatorMatchUntil.java";
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
            "java-runtime-0383244f373c8a0ffc8b"
    };
    private static final String[] EXECUTIONS = {
            "PatternOp"
    };
    private static final String[] STATIC_IDS = {
            "java-d4cb4534ca508be87595"
    };

    // Verbatim transcription of PatternOperatorMatchUntil ord 1
    // PatternOp (lines 91-315): the fifty-two EventExpressionCase atoms
    // in case-list order. Spacing is pinned byte-exact — `until`,
    // `[N]`, `[N:M]`, `[N:]`, `[:M]` bounds, `->` arrows, parentheses
    // and `not`/`or`/`and`/`every` keep the Java source spelling.
    private static final String[] ATOMS = {
            "a=SupportBean_A(id='A2') until SupportBean_D",
            "a=SupportBean_A until SupportBean_D",
            "b=SupportBean_B until a=SupportBean_A",
            "b=SupportBean_B until SupportBean_D(id='D3')",
            "(a=SupportBean_A or b=SupportBean_B) until d=SupportBean_D(id='D3')",
            "(a=SupportBean_A or b=SupportBean_B) until (g=SupportBean_G or d=SupportBean_D)",
            "(d=SupportBean_D) until a=SupportBean_A(id='A1')",
            "a=SupportBean_A until SupportBean_G(id='GX')",
            "[2] a=SupportBean_A",
            "[2:2] a=SupportBean_A",
            "[1] a=SupportBean_A",
            "[1:1] a=SupportBean_A",
            "[3] a=SupportBean_A",
            "[3] b=SupportBean_B",
            "[4] (a=SupportBean_A or b=SupportBean_B)",
            "[2] b=SupportBean_B until a=SupportBean_A(id='A1')",
            "[2] b=SupportBean_B until c=SupportBean_C",
            "[2:2] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[:4] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[:3] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[:2] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[:1] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[:1] b=SupportBean_B until a=SupportBean_A(id='A1')",
            "[1:] b=SupportBean_B until g=SupportBean_G(id='G1')",
            "[1:] b=SupportBean_B until a=SupportBean_A",
            "[2:] b=SupportBean_B until a=SupportBean_A(id='A2')",
            "[2:] b=SupportBean_B until c=SupportBean_C",
            "[2:] b=SupportBean_B until e=SupportBean_B(id='B2')",
            "[1:] b=SupportBean_B until e=SupportBean_B(id='B1')",
            "[1:2] b=SupportBean_B until a=SupportBean_A(id='A2')",
            "[1:3] b=SupportBean_B until SupportBean_G",
            "[1:2] b=SupportBean_B until SupportBean_G",
            "[1:10] b=SupportBean_B until SupportBean_F",
            "[1:10] b=SupportBean_B until SupportBean_C",
            "[0:1] b=SupportBean_B until SupportBean_C",
            "c=SupportBean_C -> [2] b=SupportBean_B -> d=SupportBean_D",
            "[3] d=SupportBean_D or [3] b=SupportBean_B",
            "[3] d=SupportBean_D or [4] b=SupportBean_B",
            "[2] d=SupportBean_D and [2] b=SupportBean_B",
            "d=SupportBean_D until timer:interval(7 sec)",
            "every (d=SupportBean_D until b=SupportBean_B)",
            "every d=SupportBean_D until b=SupportBean_B",
            "(every d=SupportBean_D) until b=SupportBean_B",
            "a=SupportBean_A until (every (timer:interval(6 sec) and not SupportBean_A))",
            "SupportBean_A until (every (timer:interval(7 sec) and not SupportBean_A))",
            "[2] (a=SupportBean_A or b=SupportBean_B)",
            "every [2] a=SupportBean_A",
            "every [2] a=SupportBean_A until d=SupportBean_D",
            "[3] (a=SupportBean_A or b=SupportBean_B)",
            "(a=SupportBean_A until b=SupportBean_B) until c=SupportBean_C",
            "(a=SupportBean_A until b=SupportBean_B) until g=SupportBean_G",
            "SupportBean_B until not SupportBean_B"
    };

    // Pinned deployed statement text per leg: `@name("S<i>") select *
    // from pattern [<atom>]` — S0..S51 replace the harness's
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
    private static final int[] CASE_ORDINALS = {1};
    private static final int[] CASE_RUNTIME_INDEX = {0};
    private static final String[] OBSERVATIONS = {
            "listener; external clock +1000ms per send; all 52 legs deploy in ONE step "
                    + "before any send (harness deploy-all), then twelve advance-before-send "
                    + "replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes "
                    + "sendEventBean so timer expiries attribute to the upcoming bucket): S0 "
                    + "D1{a:A2}, S1 D1{a:A1,A2}, S2 A1{b empty,a:A1}, S3 D3{b:B1,B2,B3}, S4 "
                    + "D3{a:A1,A2;b:B1,B2,B3;d:D3}, S5 D1{a:A1,A2;b:B1,B2;d:D1}, S6 A1{a:A1}, "
                    + "S7 silent, S8/S9 A2{a:A1,A2}, S10/S11 A1{a:A1}, S12 silent, S13 "
                    + "B3{b:B1,B2,B3}, S14 A2{a:A1,A2;b:B1,B2}, S15/S16 silent, S17 B2{b:B1,B2}, "
                    + "S18/S19 G1{b:B1,B2,B3;g:G1}, S20 G1{b:B1,B2;g:G1}, S21 G1{b:B1;g:G1}, "
                    + "S22 A1{b empty,a:A1}, S23 G1{b:B1,B2,B3;g:G1}, S24 silent, S25 "
                    + "A2{b:B1,B2;a:A2}, S26/S27/S28 silent, S29 A2{b:B1,B2;a:A2}, S30 "
                    + "G1{b:B1,B2,B3}, S31 G1{b:B1,B2}, S32 F1{b:B1,B2}, S33/S34 C1{b:B1}, S35 "
                    + "D3{c:C1;b:B2,B3;d:D3}, S36 B3{b:B1,B2,B3}, S37 D3{d:D1,D2,D3}, S38 "
                    + "D2{b:B1,B2;d:D1,D2}, S39 E1{d:D1} (timer fires inside the advance to "
                    + "7000), S40 B1/B2/B3 (d:D1,D2 collected on B3), S41/S42 B1 only (every "
                    + "precedence), S43 G1{a:A1,A2} (6s timer+not-A terminator), S44 D3 fire "
                    + "with no tags (7s every-timer), S45 B1{a:A1;b:B1}, S46 A2{a:A1,A2}, S47 "
                    + "D1{a:A1,A2;d:D1} (ESPER-339), S48 B2{a:A1;b:B1,B2}, S49 C1{a:A1;b:B1;"
                    + "c:C1}, S50 G1{a:A1,A2;b:B1,B2,B3;g:G1}, S51 silent (start-fire ignored); "
                    + "per leg+trigger the fires compare as a multiset like compareLists"
    };

    // Pinned op sequence (after the case marker): ONE deploy-all step
    // expands to the fifty-two per-leg deployments, then twelve fused
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
    private static final int EXPECTED_RECORDS = 45;

    private PatternMatchUntilWHarness588ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternMatchUntilWHarness588ScenarioOracle <scenario.json>");
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
                        // fifty-two individual deployments — statement
                        // names S0..S51, one listener each, like
                        // env.deploy(unit).addListener(nameOfStatement).
                        for (int leg = 0; leg < CASE_EPLS.length; leg++) {
                            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                                    CASE_EPLS[leg],
                                    new CompilerArguments(runtime.getRuntimePath()));
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
