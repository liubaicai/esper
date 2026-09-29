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
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.regressionlib.support.client.SupportPortableDeploySubstitutionParams;
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
 * Direct Esper 9.0.0 oracle for the pattern-guard-timerwithin-forms-595
 * unit: PatternGuardTimerWithin ordinals 1-6 — the six remaining
 * timer:within executions after the ord-0 W-harness, each replayed as
 * its own case on a fresh engine like the regression suite's
 * per-execution environment.
 *
 * All six executions run under the external clock: sendTimer(0) (or
 * sendCurrentTime("2002-02-01T09:00:00.000") for the month-scoped
 * rounds) establishes the arm instant before deploy, and every send
 * step performs advanceTime(at) BEFORE sendEventBean so guard-expiry
 * callbacks inside an advance run at the advance instant BEFORE the
 * upcoming event — an event landing exactly at arm+period loses (the
 * deadline is exclusive) while a branch respawned inside an expiry
 * callback is live for the same-tick event.
 *
 * (interval-10-min) PatternInterval10Min arms
 * `(every SupportBean) where timer:within(1 days 2 hours 3 minutes 4
 * seconds 5 milliseconds)` — a 93784005ms guard. tryAssertion sends the
 * default-ctor SupportBean at t=0 (fire), advances to 93784004 and
 * sends again (fire), then advances to the exact 93784005 deadline
 * where the third send is killed silently.
 * (interval-10-min-variable) PatternInterval10MinVariable reads the
 * same period once at arm time from the suite variables
 * D/H/M/S/MS=1/2/3/4/5 — TestSuitePattern.configure's
 * addVariable(name, double.class, n) declarations, registered on the
 * Configuration for EVERY case (inert elsewhere). The execution's
 * stmtText == eplToModel(stmtText).toEPL() assertion is mirrored as a
 * compile-path invariant check after the sends.
 * (interval-prepared) PatternIntervalPrepared composes the same period
 * from five positional `?::int` substitution parameters bound at deploy
 * via DeploymentOptions.setStatementSubstitutionParameter(
 * SupportPortableDeploySubstitutionParams().add(1,1).add(2,2).add(3,3)
 * .add(4,4).add(5,5)).
 * (within-from-expression) PatternWithinFromExpression deploys the
 * event-correlated guard `a=SupportBean -> (every b=SupportBean)
 * where timer:within(a.intPrimitive seconds)`: SupportBean("E1",3)@t=0
 * completes a and arms the every-b guard to t=3000, E2@2000 and
 * E3@2999 emit `b.theString as id` rows, the advance to 3000 expires
 * the guard so E4@3000 emits nothing.
 * (pattern-not-followed-by) PatternPatternNotFollowedBy deploys
 * `every(SupportBean -> (SupportMarketDataBean where
 * timer:within(5 sec)))`: E1@t=0 and E2@t=0 each spawn a branch whose
 * guarded RHS expires at its own +5000ms deadline; the advance to 6000
 * kills both branches and the outer every respawns at the advance
 * instant, so E4@6000 starts a fresh branch that the
 * SupportMarketDataBean("E5","M1",1) send at the same instant completes
 * — exactly one empty-payload emission.
 * (may-max-month) PatternWithinMayMaxMonthScoped runs two sequential
 * rounds in one execution: `(every SupportBean) where timer:within(
 * 1 month)` then `timer:withinmax(1 month, 10)`, each preceded by
 * sendCurrentTime(2002-02-01T09:00:00.000) — the deploy step's `at`
 * pre-advance — so the calendar-month boundary is
 * 2002-03-01T09:00:00Z: E1 fires at arm, E2 fires at boundary-1ms and
 * E3 at the exact boundary is silent; each round ends in undeployAll.
 *
 * `select *` over untagged pattern atoms projects the empty row;
 * milestone()/assertRuntime calls are harness machinery carrying no
 * observable listener payload. The internal timer is disabled for
 * determinism; every listener record reports the virtual time at the
 * triggering send. Steps not belonging to the active case are skipped;
 * the JSON validation below pins every case block byte-for-byte.
 */
public final class PatternGuardTimerWithinForms595ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-guard-timerwithin-forms-595";
    private static final String DESCRIPTION =
            "PatternGuardTimerWithin ordinals 1-6 — the six remaining timer:within "
                    + "executions after the ord-0 W-harness, each replayed as its own "
                    + "case on a fresh engine: ord 1 `within(1 days 2 hours 3 minutes 4 "
                    + "seconds 5 milliseconds)` (93784005ms, tryAssertion "
                    + "fire@0/fire@period-1/silent@period), ord 2 the same period read "
                    + "from variables D/H/M/S/MS=1..5, ord 3 the same via five positional "
                    + "`?::int` substitution params (1..5), ord 4 `a=SupportBean -> "
                    + "(every b=SupportBean) where timer:within(a.intPrimitive seconds)` "
                    + "(E1(3)@0 arms 3000ms, {id:E2}@2000, {id:E3}@2999, expiry@3000 "
                    + "silent), ord 5 `every(SupportBean -> (SupportMarketDataBean where "
                    + "timer:within(5 sec)))` (branch expiry + every respawn at the 6000 "
                    + "advance, ONE emission for E4+E5), ord 6 two rounds `(every "
                    + "SupportBean) where timer:within(1 month)` and `timer:withinmax(1 "
                    + "month, 10)` (fire E1 + E2 at boundary-1ms, silent at boundary). "
                    + "Java milestone()/assertRuntime calls are oracle-internal and "
                    + "carry no observable listener payload.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternGuardTimerWithin.java";
    private static final String[] JAVA_SOURCE_FILES = {
            JAVA_SOURCE,
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/"
                    + "SupportMarketDataBean.java"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f0649272cfb528ce731b",
            "java-runtime-2fe5370a7c1599bfb424",
            "java-runtime-66d65309509fac58bbfc",
            "java-runtime-36ef6f15f14a0e86ab93",
            "java-runtime-6a5e8128ae184e8a7249",
            "java-runtime-34555c4a9823a346d710"
    };
    private static final String[] EXECUTIONS = {
            "PatternInterval10Min",
            "PatternInterval10MinVariable",
            "PatternIntervalPrepared",
            "PatternWithinFromExpression",
            "PatternPatternNotFollowedBy",
            "PatternWithinMayMaxMonthScoped"
    };
    private static final String[] STATIC_IDS = {
            "java-0dec801a426fed297402"
    };

    private static final String[] CASES = {
            "interval-10-min", "interval-10-min-variable", "interval-prepared",
            "within-from-expression", "pattern-not-followed-by", "may-max-month"
    };
    private static final int[] CASE_ORDINALS = {1, 2, 3, 4, 5, 6};

    // Byte-exact statement texts (PatternGuardTimerWithin.java
    // verbatim). Each case's epls array is consumed in order across its
    // deploy steps — only may-max-month has two (the within round then
    // the withinmax round).
    private static final String[][] CASE_EPLS = {
            {"@name('s0') select * from pattern [(every SupportBean) where "
                    + "timer:within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)]"},
            {"@name('s0') select * from pattern [(every SupportBean) where "
                    + "timer:within(D days H hours M minutes S seconds MS milliseconds)]"},
            {"@name('s0') select * from pattern [(every SupportBean) where "
                    + "timer:within(?::int days ?::int hours ?::int minutes ?::int seconds "
                    + "?::int milliseconds)]"},
            {"@name('s0') select b.theString as id from pattern[a=SupportBean -> "
                    + "(every b=SupportBean) where timer:within(a.intPrimitive seconds)]"},
            {"@name('s0') select * from pattern [ every(SupportBean -> "
                    + "(SupportMarketDataBean where timer:within(5 sec))) ]"},
            {"@name('s0') select * from pattern [(every SupportBean) where "
                    + "timer:within(1 month)]",
                    "@name('s0') select * from pattern [(every SupportBean) where "
                            + "timer:withinmax(1 month, 10)]"}
    };

    private static final String[] OBSERVATIONS = {
            "listener; external clock; guard arms at deploy t=0 with period 93784005ms "
                    + "(1d2h3m4s5ms): tryAssertion sends the default-ctor SupportBean at "
                    + "t=0 (fire 1), advances to 93784004 and sends again (fire 2), then "
                    + "advances to 93784005 and sends a third SupportBean which the "
                    + "exclusive deadline kills — two empty-payload rows (untagged "
                    + "atoms project nothing).",
            "listener; identical tryAssertion schedule; the guard period is the same "
                    + "93784005ms read once at arm time from variables D/H/M/S/MS=1/2/3/4/5 "
                    + "(suite-level declarations like TestSuitePattern.configure's "
                    + "addVariable doubles). Java also asserts stmtText == "
                    + "eplToModel(stmtText).toEPL() — a compile-path invariant with no "
                    + "Go counterpart (approved-difference precedent), not an observable "
                    + "payload.",
            "listener; identical tryAssertion schedule; the guard period is the same "
                    + "93784005ms composed at deploy from five positional `?::int` "
                    + "substitution params (1,2,3,4,5) — Go deploys via "
                    + "DeployWithPositionalParameters(1..5) instead of "
                    + "DeploymentOptions.setStatementSubstitutionParameter.",
            "listener; event-correlated guard: `a=SupportBean -> (every b=SupportBean) "
                    + "where timer:within(a.intPrimitive seconds)` — "
                    + "SupportBean(\"E1\",3)@t=0 completes a and arms the every-b guard "
                    + "to t=3000; SupportBean(\"E2\",-1)@2000 emits {id:E2}, "
                    + "SupportBean(\"E3\",-1)@2999 emits {id:E3}, advance to 3000 expires "
                    + "the guard so the post-deadline SupportBean(\"E4\",-1) emits "
                    + "nothing.",
            "listener; `every(SupportBean -> (SupportMarketDataBean where "
                    + "timer:within(5 sec)))` — E1@t=0 and E2@t=0 each spawn a branch "
                    + "whose guarded RHS expires at its own +5000ms deadline; advance to "
                    + "6000 kills both branches and the outer every respawns at the "
                    + "advance instant (594 respawn semantics), so SupportBean(\"E4\",1)"
                    + "@6000 starts a fresh branch that SupportMarketDataBean "
                    + "(\"E5\",\"M1\",1)@6000 completes — exactly one empty-payload "
                    + "emission.",
            "listener; two sequential rounds under one execution (deploy/undeploy cycle "
                    + "per round): `(every SupportBean) where timer:within(1 month)` then "
                    + "`timer:withinmax(1 month, 10)` — each arms 2002-02-01T09:00:00Z, "
                    + "fires E1 at arm and E2 at 2002-03-01T09:00:00Z-1ms (calendar "
                    + "AddDate keeps the month boundary), and stays silent for E3 at the "
                    + "exact boundary; two empty-payload rows per round."
    };

    // Pinned op sequence per case (after the case marker). Only the two
    // may-max-month deploys carry an `at` pre-advance
    // (sendCurrentTime("2002-02-01T09:00:00.000") before each round's
    // compileDeploy); every other case deploys at the initialized clock.
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "send", "undeploy-all"}
    };

    // Pinned `at` instant per deploy step (null = absent). The
    // may-max-month rounds pre-advance to 2002-02-01T09:00:00Z so each
    // guard arms at the February anchor.
    private static final String[][] CASE_DEPLOY_AT = {
            {null},
            {null},
            {null},
            {null},
            {null},
            {"2002-02-01T09:00:00Z", "2002-02-01T09:00:00Z"}
    };

    // Pinned `at` instants per send step: the fixed-period cases walk
    // t=0, deadline-1ms and the exact deadline; may-max-month walks the
    // calendar boundary.
    private static final String[][] CASE_SEND_AT = {
            {"1970-01-01T00:00:00Z", "1970-01-02T02:03:04.004Z",
                    "1970-01-02T02:03:04.005Z"},
            {"1970-01-01T00:00:00Z", "1970-01-02T02:03:04.004Z",
                    "1970-01-02T02:03:04.005Z"},
            {"1970-01-01T00:00:00Z", "1970-01-02T02:03:04.004Z",
                    "1970-01-02T02:03:04.005Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:00:02Z",
                    "1970-01-01T00:00:02.999Z", "1970-01-01T00:00:03Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:00:00Z",
                    "1970-01-01T00:00:06Z", "1970-01-01T00:00:06Z"},
            {"2002-02-01T09:00:00Z", "2002-03-01T08:59:59.999Z",
                    "2002-03-01T09:00:00Z", "2002-02-01T09:00:00Z",
                    "2002-03-01T08:59:59.999Z", "2002-03-01T09:00:00Z"}
    };

    // Pinned sends in ctor-call notation: SupportBean(theString,
    // intPrimitive) (theString "null" = JSON null, the default-ctor
    // payload of tryAssertion's sendEvent) and
    // SupportMarketDataBean(symbol, id, price).
    private static final String[][] CASE_SENDS = {
            {"SupportBean(null,0)", "SupportBean(null,0)", "SupportBean(null,0)"},
            {"SupportBean(null,0)", "SupportBean(null,0)", "SupportBean(null,0)"},
            {"SupportBean(null,0)", "SupportBean(null,0)", "SupportBean(null,0)"},
            {"SupportBean(E1,3)", "SupportBean(E2,-1)", "SupportBean(E3,-1)",
                    "SupportBean(E4,-1)"},
            {"SupportBean(E1,1)", "SupportBean(E2,2)", "SupportBean(E4,1)",
                    "SupportMarketDataBean(E5,M1,1)"},
            {"SupportBean(E1,0)", "SupportBean(E2,0)", "SupportBean(E3,0)",
                    "SupportBean(E1,0)", "SupportBean(E2,0)", "SupportBean(E3,0)"}
    };

    // Index of the prepared-statement case: its single deploy binds the
    // five positional `?::int` substitution parameters (1..5) via
    // DeploymentOptions.setStatementSubstitutionParameter, mirroring
    // PatternIntervalPrepared verbatim.
    private static final int PREPARED_CASE = 2;
    private static final int[] PREPARED_PARAM_VALUES = {1, 2, 3, 4, 5};

    // Index of the variables case: after tryAssertion the execution
    // asserts stmtText == eplToModel(stmtText).toEPL(), mirrored as a
    // compile-path invariant (no listener payload).
    private static final int VARIABLE_CASE = 1;

    private static final int EXPECTED_STEPS = 43;
    // Thirteen listener invocations: the three fixed-period cases fire
    // at t=0 and deadline-1ms (2 each), within-from-expression emits
    // {id:E2} and {id:E3} (2), pattern-not-followed-by emits once for
    // the E4+M1 pair (1) and may-max-month fires twice per round (4).
    private static final int EXPECTED_RECORDS = 13;

    private PatternGuardTimerWithinForms595ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternGuardTimerWithinForms595ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        // TestSuitePattern.configure registers the suite variables
        // D/H/M/S/MS as doubles 1/2/3/4/5; they arm ord 2's
        // `timer:within(D days H hours M minutes S seconds MS
        // milliseconds)` to the same 93784005ms period and are inert
        // for every other case.
        configuration.getCommon().addVariable("D", double.class, 1);
        configuration.getCommon().addVariable("H", double.class, 2);
        configuration.getCommon().addVariable("M", double.class, 3);
        configuration.getCommon().addVariable("S", double.class, 4);
        configuration.getCommon().addVariable("MS", double.class, 5);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        EPRuntime runtime = EPRuntimeProvider.getDefaultRuntime(configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            boolean active = false;
            int deployIndex = 0;
            int eplIndex = 0;
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
                        JsonValue at = step.get("at");
                        if (at != null) {
                            // sendCurrentTime: only the two may-max-month
                            // round deploys pre-advance the clock to the
                            // February anchor before arming the month guard.
                            runtime.getEventService().advanceTime(
                                    Instant.parse(at.asString()).toEpochMilli());
                        }
                        String epl = CASE_EPLS[caseIndex][eplIndex++];
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                                epl, new CompilerArguments(configuration));
                        DeploymentOptions options = new DeploymentOptions()
                                .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                        + "-" + (deployIndex++));
                        if (caseIndex == PREPARED_CASE) {
                            // PatternIntervalPrepared binds the five
                            // positional `?::int` substitution parameters
                            // add(1,1).add(2,2).add(3,3).add(4,4).add(5,5).
                            SupportPortableDeploySubstitutionParams params =
                                    new SupportPortableDeploySubstitutionParams();
                            for (int position = 0;
                                 position < PREPARED_PARAM_VALUES.length; position++) {
                                params.add(position + 1, PREPARED_PARAM_VALUES[position]);
                            }
                            options.setStatementSubstitutionParameter(params);
                        }
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, options);
                        EPStatement statement = requireStatement(
                                deployment, step.getString("statement", ""));
                        statement.addListener(new TraceWriter(
                                records, caseName, statement, runtime));
                        break;
                    }
                    case "send":
                        // Every send advances the external clock to the
                        // pinned instant BEFORE sendEventBean: guard
                        // expiry callbacks run at the advance instant so
                        // an event at exactly arm+period loses.
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        sendEvent(runtime, step);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            if (eplIndex != CASE_EPLS[caseIndex].length) {
                throw new IllegalStateException("case " + caseName
                        + " did not consume its pinned epls");
            }
            if (caseIndex == VARIABLE_CASE) {
                // PatternInterval10MinVariable asserts
                // stmtText == eplToModel(stmtText).toEPL(): a compile-path
                // invariant, mirrored verbatim (env.eplToModel compiles
                // the model against the configuration and copies it).
                EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(
                        EPCompilerProvider.getCompiler().eplToModel(
                                CASE_EPLS[caseIndex][0], configuration));
                if (!CASE_EPLS[caseIndex][0].equals(model.toEPL())) {
                    throw new IllegalStateException(
                            "eplToModel round-trip is not pinned: " + model.toEPL());
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement requireStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean": {
                JsonValue theString = payload.get("theString");
                runtime.getEventService().sendEventBean(new SupportBean(
                        theString.isNull() ? null : theString.asString(),
                        payload.get("intPrimitive").asInt()), "SupportBean");
                return;
            }
            case "SupportMarketDataBean":
                runtime.getEventService().sendEventBean(new SupportMarketDataBean(
                        payload.get("symbol").asString(), payload.get("id").asString(),
                        payload.get("price").asDouble()), "SupportMarketDataBean");
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
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
            validateStringArray(definition.get("epls"), CASE_EPLS[index], "epls");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * pinned ops — deploy steps labeled s0 (only the two may-max-month
     * round deploys carry the `at` pre-advance), the fused
     * advance-before-send steps with pinned instants and ctor payloads,
     * and the per-round undeploy-all. Unknown step fields are rejected.
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
            int deploys = 0;
            int sends = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy": {
                        String expectedAt = CASE_DEPLOY_AT[caseIndex][deploys++];
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException(
                                    "deploy step " + cursor + " is not pinned");
                        }
                        if (expectedAt == null) {
                            requireFields(step, "op", "case", "statement");
                        } else {
                            requireFields(step, "op", "case", "statement", "at");
                            if (!expectedAt.equals(string(step, "at"))) {
                                throw new IllegalArgumentException("deploy step " + cursor
                                        + " advance instant is not pinned: expected "
                                        + expectedAt + " got " + string(step, "at"));
                            }
                        }
                        break;
                    }
                    case "send": {
                        requireFields(step, "op", "case", "at", "eventType", "payload");
                        if (!CASE_SEND_AT[caseIndex][sends].equals(string(step, "at"))) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " advance instant is not pinned: expected "
                                    + CASE_SEND_AT[caseIndex][sends]
                                    + " got " + string(step, "at"));
                        }
                        JsonObject payload = object(step.get("payload"),
                                "send payload " + cursor);
                        String eventType = string(step, "eventType");
                        String actualSend;
                        switch (eventType) {
                            case "SupportBean":
                                requireFields(payload, "theString", "intPrimitive");
                                JsonValue theString = payload.get("theString");
                                if (!theString.isNull() && !theString.isString()) {
                                    throw new IllegalArgumentException("send payload "
                                            + cursor + " theString is not pinned");
                                }
                                actualSend = "SupportBean("
                                        + (theString.isNull() ? "null" : theString.asString())
                                        + "," + integer(payload, "intPrimitive") + ")";
                                break;
                            case "SupportMarketDataBean":
                                requireFields(payload, "symbol", "id", "price");
                                actualSend = "SupportMarketDataBean("
                                        + string(payload, "symbol") + ","
                                        + string(payload, "id") + ","
                                        + integer(payload, "price") + ")";
                                break;
                            default:
                                throw new IllegalArgumentException("send step " + cursor
                                        + " is not pinned");
                        }
                        if (!CASE_SENDS[caseIndex][sends].equals(actualSend)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected "
                                    + CASE_SENDS[caseIndex][sends] + " got " + actualSend);
                        }
                        sends++;
                        break;
                    }
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException(
                                "unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (deploys != CASE_DEPLOY_AT[caseIndex].length
                    || sends != CASE_SENDS[caseIndex].length) {
                throw new IllegalArgumentException(
                        "case " + caseIndex + " step counts are not pinned");
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
                    throw new IllegalArgumentException(
                            "duplicate JSON object key: " + member.getName());
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
        if (value instanceof SupportBean) {
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(((SupportBean) value).getTheString()));
            fields.add("intPrimitive", normalize(((SupportBean) value).getIntPrimitive()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportMarketDataBean) {
            JsonObject fields = new JsonObject();
            fields.add("symbol", normalize(((SupportMarketDataBean) value).getSymbol()));
            fields.add("id", normalize(((SupportMarketDataBean) value).getId()));
            fields.add("price", normalize(((SupportMarketDataBean) value).getPrice()));
            fields.add("volume", normalize(((SupportMarketDataBean) value).getVolume()));
            fields.add("feed", normalize(((SupportMarketDataBean) value).getFeed()));
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
