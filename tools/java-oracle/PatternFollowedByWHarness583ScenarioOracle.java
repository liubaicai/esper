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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-followedby-wharness-583
 * unit: PatternOperatorFollowedBy ord 0 PatternOpWHarness — the shared
 * sixteen-leg followed-by W-harness over
 * EventCollectionFactory.getEventSetOne(0, 1000), replayed as ONE case
 * against a fresh runtime like the regression suite's per-execution
 * environment.
 *
 * The PatternTestHarness USE_EPL style compiles all sixteen statements
 * FIRST — text {@code @name("name--<atom>") @Audit('pattern')
 * @Audit('pattern-instances') select * from pattern [<atom>]} where
 * name--&lt;atom&gt; is nameOfStatement (StringEscapeUtils.escapeJava is
 * an identity on these atoms) — then replays the external clock:
 * advanceTime(ON_START=0) precedes the deployments and each event
 * advances the clock to its pinned instant (+1000ms in makeMixedSet
 * order) before sendEventBean. Every send's fires arrive per leg in one
 * listener invocation per completing delivery; the harness compares
 * each leg's expected EventDescriptors as a MULTISET (compareLists), so
 * the duplicated rows of legs 10/15/16 — the retained done-sequence
 * re-fires of the nested/outer every compositions — are normative
 * multiplicity and are preserved verbatim.
 *
 * Legs 1/2/5/6 fire AT B1 with a vacant d cell (the or-not/not right
 * side completes at spawn, d renders null); leg 12
 * {@code c=SupportBean_C() -> d=SupportBean_D -> a=SupportBean_A} is a
 * normative zero-fire leg; the {@code -[1000]>}/{@code -[10]>}
 * FollowedByMax edges never bind on this clock. The COMPILE_TO_MODEL,
 * COMPILE_TO_EPL and USE_EPL_AND_CONSUME_NOCHECK replay styles, all
 * env.milestone savepoints and the silent post-undeploy resend are
 * harness machinery outside the replayed surface and are
 * unrepresented. The internal timer is disabled for determinism; every
 * listener record reports the virtual time at the triggering send.
 *
 * {@code select *} projects every named tag's bean fragment —
 * b/d, a_1/b/a_2, c/d/a or a/b per leg — and unbound tags render null,
 * matching the Go PatternEvent projection that renders
 * {"state":"null"}.
 */
public final class PatternFollowedByWHarness583ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-followedby-wharness-583";
    private static final String DESCRIPTION =
            "PatternOperatorFollowedBy ord 0 PatternOpWHarness — the shared sixteen-leg "
                    + "followed-by W-harness over EventCollectionFactory.getEventSetOne(0,1000), "
                    + "replayed as ONE case: all sixteen legs deploy up front as "
                    + "`@name(\"name--<atom>\") @Audit('pattern') @Audit('pattern-instances') "
                    + "select * from pattern [<atom>]`, the external clock advances to ON_START=0 "
                    + "before the deployments and to each event's pinned instant (+1000ms per send) "
                    + "before sendEventBean, and every send's fires are grouped per leg+trigger as "
                    + "multiset rows — including the DUPLICATED rows of legs 10/15/16 "
                    + "(`every (b -> every d)` re-fires {B3,D3} twice at D3; `every ( every a -> every b)` "
                    + "re-fires {A2,B3} three times; `every (a -> every b)` twice). Legs 1/2/5/6 fire "
                    + "AT B1 with a vacant d tag rendered {\"state\":\"null\"} (spawn-satisfied "
                    + "or-not/not right side, not a timeout); leg 12 `c=SupportBean_C() -> "
                    + "d=SupportBean_D -> a=SupportBean_A` is a normative 0-fire leg; the "
                    + "`-[1000]>`/`-[10]>` FollowedByMax edges never bind on this clock. Expected "
                    + "rows (fire event -> tag rows): leg1 B1{b,B1;d,null}+D1{b,B1;d,D1}, leg2 same, "
                    + "leg3 D1/D2/D3 {b,B1;d,Dn}, leg4 D1{b,B1;d,D1}, leg5/6 B1{b,B1;d,null}, leg7 "
                    + "D1/D2 2rows+D3 3rows, leg8 D1 2rows+D3 {b,B3}, leg9 same, leg10 D1/D2 "
                    + "{b,B1}+D3 {b,B1;d,D3},{b,B3;d,D3},{b,B3;d,D3}, leg11 A2{a_1,A1;b,B1;a_2,A2}, "
                    + "leg12 none, leg13/14 same as 11, leg15 B1/B2 {a,A1}+B3 "
                    + "{a,A1;b,B3},{a,A2;b,B3}x3, leg16 same with x2. The "
                    + "COMPILE_TO_MODEL/COMPILE_TO_EPL and consume+suppress harness styles plus all "
                    + "env.milestone savepoints are unrepresented harness machinery.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternOperatorFollowedBy.java";
    private static final String[] JAVA_SOURCE_FILES = {
            JAVA_SOURCE,
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "EventCollectionFactory.java",
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
            "java-runtime-d896ea164e3cebd5c27e"
    };
    private static final String[] EXECUTIONS = {
            "PatternOpWHarness"
    };
    private static final String[] STATIC_IDS = {
            "java-971bf7dfac84756cdd0a"
    };

    // Verbatim transcription of PatternOperatorFollowedBy ord 0: the
    // sixteen EventExpressionCase atoms inside the PatternTestHarness
    // USE_EPL statement text. `()` after type names and `-[N]>` edges
    // are pinned exactly as written — including leg 11's lone
    // `a_1=SupportBean_A()` (b and a_2 are bare), leg 12's lone
    // `c=SupportBean_C()`, leg 15's `every ( every a=...` with the
    // space after the opening parenthesis, and legs 13/14/16 carrying
    // `()` on every atom.
    private static final String[] ATOMS = {
            "b=SupportBean_B -> (d=SupportBean_D or not d=SupportBean_D)",
            "b=SupportBean_B -[1000]> (d=SupportBean_D or not d=SupportBean_D)",
            "b=SupportBean_B -> every d=SupportBean_D",
            "b=SupportBean_B -> d=SupportBean_D",
            "b=SupportBean_B -> not d=SupportBean_D",
            "b=SupportBean_B -[1000]> not d=SupportBean_D",
            "every b=SupportBean_B -> every d=SupportBean_D",
            "every b=SupportBean_B -> d=SupportBean_D",
            "every b=SupportBean_B -[10]> d=SupportBean_D",
            "every (b=SupportBean_B -> every d=SupportBean_D)",
            "every (a_1=SupportBean_A() -> b=SupportBean_B -> a_2=SupportBean_A)",
            "c=SupportBean_C() -> d=SupportBean_D -> a=SupportBean_A",
            "every (a_1=SupportBean_A() -> b=SupportBean_B() -> a_2=SupportBean_A())",
            "every (a_1=SupportBean_A() -[10]> b=SupportBean_B() -[10]> a_2=SupportBean_A())",
            "every ( every a=SupportBean_A -> every b=SupportBean_B)",
            "every (a=SupportBean_A() -> every b=SupportBean_B())"
    };

    // USE_EPL statement text per leg: nameOfStatement is
    // `name--` + escapeJava(atom) — an identity on these atoms — and the
    // annotation carries double quotes.
    private static final String[] CASE_EPLS;
    private static final String[] LEG_NAMES;

    static {
        CASE_EPLS = new String[ATOMS.length];
        LEG_NAMES = new String[ATOMS.length];
        for (int index = 0; index < ATOMS.length; index++) {
            LEG_NAMES[index] = "name--" + ATOMS[index];
            CASE_EPLS[index] = "@name(\"" + LEG_NAMES[index]
                    + "\") @Audit('pattern') @Audit('pattern-instances') select * from pattern ["
                    + ATOMS[index] + "]";
        }
    }

    private static final String[] CASES = {"w-harness"};
    private static final int[] CASE_ORDINALS = {0};
    private static final int[] CASE_RUNTIME_INDEX = {0};
    private static final String[] OBSERVATIONS = {
            "listener; external clock +1000ms per send; sixteen legs deploy before any send "
                    + "(harness deploys all statements, then replays the event set): leg1/2 "
                    + "B1{b,B1;d,null}+D1{b,B1;d,D1}, leg3 D1/D2/D3 {b,B1;d,Dn}, leg4 "
                    + "D1{b,B1;d,D1}, leg5/6 B1{b,B1;d,null}, leg7 D1 2rows/D2 2rows/D3 3rows, "
                    + "leg8 D1 2rows/D3 {b,B3;d,D3}, leg9 same, leg10 D3 fires "
                    + "{b,B1;d,D3},{b,B3;d,D3},{b,B3;d,D3} (dup tail), leg11 "
                    + "A2{a_1,A1;b,B1;a_2,A2}, leg12 silent, leg13/14 same as 11, leg15 B3 fires "
                    + "{a,A1;b,B3},{a,A2;b,B3}x3 (dup tail), leg16 x2; each fire event delivers its "
                    + "rows in ONE listener invocation per leg (Java getLastNewData multiset pin)"
    };

    // Pinned op sequence (after the case marker) in harness order: the
    // ON_START advance-time precedes the sixteen deployments (the
    // USE_EPL style compiles all statements before the event loop),
    // then each event's advance-time+send pair, then undeploy-all.
    // env.milestone savepoints, the three other replay styles and the
    // post-undeploy resend carry no scenario op.
    private static final String[][] CASE_OPS = {{
            "advance-time",
            "deploy", "deploy", "deploy", "deploy", "deploy", "deploy", "deploy", "deploy",
            "deploy", "deploy", "deploy", "deploy", "deploy", "deploy", "deploy", "deploy",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "advance-time", "send",
            "undeploy-all"
    }};

    // Pinned advance-time instants (epoch millis): ON_START=0 then
    // +1000ms per event in makeMixedSet insertion order.
    private static final long[][] CASE_ADVANCES = {{
            0, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000, 11000, 12000
    }};

    // Pinned sends ("<eventType>|<id>") in send order.
    private static final String[][] CASE_SENDS = {{
            "SupportBean_A|A1", "SupportBean_B|B1", "SupportBean_C|C1",
            "SupportBean_B|B2", "SupportBean_A|A2", "SupportBean_D|D1",
            "SupportBean_E|E1", "SupportBean_F|F1", "SupportBean_D|D2",
            "SupportBean_B|B3", "SupportBean_G|G1", "SupportBean_D|D3"
    }};

    private static final int EXPECTED_STEPS = 43;
    private static final int EXPECTED_RECORDS = 29;

    private PatternFollowedByWHarness583ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternFollowedByWHarness583ScenarioOracle <scenario.json>");
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
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "deploy": {
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                new CompilerArguments(runtime.getRuntimePath()));
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                                + "-" + deployIndex));
                        deployIndex++;
                        EPStatement statement = findStatement(deployment,
                                step.getString("statement", ""));
                        statement.addListener(
                                new TraceWriter(records, caseName, statement, runtime));
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    case "send":
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

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        String id = payload.getString("id", null);
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
     * pinned ops — the ON_START advance-time, the sixteen deploy steps
     * carrying the pinned USE_EPL statement text and name--&lt;atom&gt;
     * statement label per leg, the twelve advance-time+send pairs with
     * pinned instants/payloads and the closing undeploy-all. Unknown
     * step fields are rejected.
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
                        if (!LEG_NAMES[deploys].equals(string(step, "statement"))
                                || !CASE_EPLS[deploys].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deploys++;
                        break;
                    case "advance-time": {
                        requireFields(step, "op", "case", "at");
                        if (advances >= CASE_ADVANCES[caseIndex].length) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " exceeds the pinned advances");
                        }
                        long expected = CASE_ADVANCES[caseIndex][advances++];
                        long actual = Instant.parse(string(step, "at")).toEpochMilli();
                        if (actual != expected) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "send": {
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
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
                        String actual = eventType + "|" + string(payload, "id");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
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
            if (sends != CASE_SENDS[caseIndex].length || deploys != CASE_EPLS.length
                    || advances != CASE_ADVANCES[caseIndex].length) {
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
