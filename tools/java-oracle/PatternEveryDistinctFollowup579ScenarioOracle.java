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
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-everydistinct-followup-579
 * bundle: PatternOperatorEveryDistinct ords 3, 13 and 15 — every-distinct
 * over a bare filter with an unqualified key, dual every-distinct on
 * both followed-by sides with per-branch right keysets, and
 * calendar-month scoped expiry — replayed as five cases (ords 3/13 x two
 * legs, ord 15 x one leg), each against a fresh runtime like the
 * regression suite's per-leg deploy.
 *
 * over-filter (PatternEveryDistinctOverFilter, ord 3) deploys
 * {@code every-distinct(intPrimitive) a=SupportBean} — the key is
 * UNQUALIFIED (no a. prefix) — then the {@code every-distinct
 * (intPrimitive,2 minutes)} expiry leg (no space after the comma,
 * pinned byte-exact): E1(1) fires, E2(1) is a dup, E3(2)/E4(3) fire,
 * E5(2)/E6(3)/E7(1) stay silent and E8(0) fires. No clock ops exist, so
 * both legs replay the identical sequence. The Java leg's trailing
 * eplToModelCompileDeploy(expression) is a SODA/object-model front-end
 * round-trip and is excluded (compile-text precedent).
 *
 * followedby-with-distinct (PatternFollowedByWithDistinct, ord 13)
 * deploys {@code every-distinct(a.intPrimitive) a=SupportBean(theString
 * like 'A%') -> every-distinct(b.intPrimitive) b=SupportBean(theString
 * like 'B%')}: each retained left branch owns an INDEPENDENT right-side
 * keyset — B1(0) and B2(1) both fire on the A1 branch, B3(0) is a
 * right-key dup, A2(1) is a swallowed LEFT dup (spawns no branch),
 * B4(2) fires {A1,B4}, A3(2) arms a second branch and B5(1) fires
 * {A3,B5}, B6(1) is a dup on the A3 branch, and B7(3) completes BOTH
 * retained branches delivering TWO rows {A1,B7},{A3,B7} in one listener
 * update — the fan-out discriminant. The expiry leg expires the LEFT
 * every-distinct only ({@code every-distinct(a.intPrimitive, 1 day)} —
 * the right side stays bare in the Java source), pinned asymmetry; no
 * clock, so it replays the identical sequence.
 *
 * month-scoped (PatternMonthScoped, ord 15) deploys {@code
 * every-distinct(theString, 1 month) a=SupportBean} with the clock at
 * 2002-02-01T09:00:00.000 before deploy (sendCurrentTime): E1(1) fires,
 * E1(2) is a dup, one millisecond before the 2002-03-01T09:00:00.000
 * month mark the key is still alive so E1(3) stays silent (boundary
 * exclusive), and at exactly the mark E1(4) fires — the calendar-month
 * boundary discriminant. The mid-run milestone(0) savepoint is omitted
 * (restores identical state, 573/575/576 precedent).
 *
 * SupportBean is the real support bean mirroring the regression suite.
 * Tagged-event columns arrive at the listener as the underlying bean,
 * so the trace renders each bean's pinned {theString,intPrimitive}
 * projection as a kind:row object exactly like the Go NormalizeResults
 * event rendering. Records follow the standard protocol: one listener
 * record per delivered update with a per-case sequence counter starting
 * at 1 and time rendered from the current engine time.
 */
public final class PatternEveryDistinctFollowup579ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-everydistinct-followup-579";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "PatternOperatorEveryDistinct ords 3, 13 and 15 — the follow-up triplet: every-distinct "
                    + "over a bare filter with an unqualified key, dual every-distinct on both "
                    + "followed-by sides with per-branch right keysets, and calendar-month scoped "
                    + "expiry over SupportBean. over-filter (PatternEveryDistinctOverFilter, ord 3) "
                    + "keys on the unqualified intPrimitive: E1(1) fires, E2(1) dup, E3(2)/E4(3) "
                    + "fire, E5(2)/E6(3)/E7(1) silent, E8(0) fires — 4 fires; the `2 minutes` "
                    + "expiry leg replays the identical sequence with no clock and the trailing "
                    + "eplToModelCompileDeploy SODA round-trip is excluded (compile-text "
                    + "precedent). followedby-with-distinct (PatternFollowedByWithDistinct, ord 13) "
                    + "keys each followed-by side on its own tag's intPrimitive over a='A%' -> "
                    + "b='B%': B1(0)/B2(1) both fire on the A1 branch (per-branch right keyset), "
                    + "B3(0) is a right-key dup, A2(1) is a swallowed left dup spawning no branch, "
                    + "B4(2) fires {A1,B4}, A3(2) arms a second branch and B5(1) fires {A3,B5}, "
                    + "B6(1) is a dup on the A3 branch, and B7(3) fans out TWO rows "
                    + "{A1,B7},{A3,B7} inside one listener update — 5 listener calls, "
                    + "6 rows; the `1 day` expiry leg applies to "
                    + "the LEFT distinct only (right side bare — pinned asymmetry) and replays "
                    + "identically. month-scoped (PatternMonthScoped, ord 15) keys on theString "
                    + "with a 1-month calendar expiry starting at 2002-02-01T09:00:00: E1(1) fires "
                    + "{E1,1}, E1(2) is a dup, E1(3) at one millisecond before the 2002-03-01 mark "
                    + "stays silent (the milestone(0) savepoint is omitted — restores identical "
                    + "state), and E1(4) at exactly the month mark fires {E1,4} — 2 fires.";

    private static final String[] CASES = {
            "over-filter", "over-filter-expiry",
            "followedby-with-distinct", "followedby-with-distinct-expiry",
            "month-scoped"};
    private static final int[] CASE_ORDINALS = {3, 3, 13, 13, 15};
    private static final int[] CASE_RUNTIME_INDEX = {0, 0, 1, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7d52c9e689e3eeca65d7",
            "java-runtime-0fe853af533719599512",
            "java-runtime-d211795c3ecad71c297a"};
    private static final String[] EXECUTIONS = {
            "PatternEveryDistinctOverFilter",
            "PatternFollowedByWithDistinct",
            "PatternMonthScoped"};
    // Deduplicated inventory id: the three runtime rows share static id
    // java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
    // executions, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock; unqualified `every-distinct(intPrimitive)` key over a bare "
                    + "a=SupportBean: E1(1) fires, E2(1) dup, E3(2)/E4(3) fire, "
                    + "E5(2)/E6(3)/E7(1) silent, E8(0) fires — 4 fires; trailing "
                    + "eplToModelCompileDeploy SODA round-trip excluded (compile-text precedent)",
            "listener; no clock; `every-distinct(intPrimitive,2 minutes)` per-key expiry leg (no "
                    + "space after the comma — pinned byte-exact) — the clock never advances so "
                    + "the identical sequence replays: E1(1)/E3(2)/E4(3)/E8(0) fire, "
                    + "E2(1)/E5(2)/E6(3)/E7(1) silent; SODA tail excluded",
            "listener; no clock; dual every-distinct over a='A%' -> b='B%' with per-branch right "
                    + "keysets: A1(1) arms the left, B1(0)/B2(1) fire on the A1 branch, B3(0) "
                    + "right-dup silent, A2(1) left-dup swallowed, B4(2) fires {A1,B4}, "
                    + "A3(2)+B5(1) fire {A3,B5}, B6(1) A3-branch dup, B7(3) TWO rows "
                    + "{A1,B7},{A3,B7} any-order — 5 fires, 6 rows",
            "listener; no clock; `1 day` expiry applies to the LEFT every-distinct only (the "
                    + "right side stays bare — pinned asymmetry); the key never expires in-run so "
                    + "the identical sequence replays including the two-row B7 fan-out "
                    + "{A1,B7},{A3,B7} — 5 fires, 6 rows",
            "listener; virtual clock 2002-02-01T09:00:00; `every-distinct(theString, 1 month)` "
                    + "calendar expiry: E1(1) fires {E1,1}, E1(2) dup, advance to "
                    + "2002-03-01T09:00:00 minus 1ms keeps E1(3) silent (boundary exclusive — key "
                    + "still alive), milestone(0) savepoint omitted, advance to the exact month "
                    + "mark lets E1(4) fire {E1,4} — 2 fires"};

    private static final String EPL_OVER_FILTER =
            "@name('s0') select * from pattern [every-distinct(intPrimitive) a=SupportBean]";
    private static final String EPL_OVER_FILTER_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(intPrimitive,2 minutes) a=SupportBean]";
    private static final String EPL_FOLLOWEDBY_WITH_DISTINCT =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive) "
                    + "a=SupportBean(theString like 'A%') -> every-distinct(b.intPrimitive) "
                    + "b=SupportBean(theString like 'B%')]";
    private static final String EPL_FOLLOWEDBY_WITH_DISTINCT_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 day) "
                    + "a=SupportBean(theString like 'A%') -> every-distinct(b.intPrimitive) "
                    + "b=SupportBean(theString like 'B%')]";
    private static final String EPL_MONTH_SCOPED =
            "@name('s0') select * from pattern [every-distinct(theString, 1 month) a=SupportBean]";

    private static final String[] CASE_EPLS = {
            EPL_OVER_FILTER, EPL_OVER_FILTER_EXPIRY,
            EPL_FOLLOWEDBY_WITH_DISTINCT, EPL_FOLLOWEDBY_WITH_DISTINCT_EXPIRY,
            EPL_MONTH_SCOPED};

    private static final String AT_START = "2002-02-01T09:00:00.000Z";
    private static final String AT_MINUS_1MS = "2002-03-01T08:59:59.999Z";
    private static final String AT_MONTH_MARK = "2002-03-01T09:00:00.000Z";

    // Pinned op sequences (after the case marker), in Java source order.
    // Milestones are regression-harness savepoints restoring identical
    // state and carry no scenario op (ord 15's mid-run milestone(0)
    // included). Ord 15 pins the Java sendCurrentTime calls as
    // advance-time steps at the absolute instants CASE_ATS lists in
    // order; the timed-expiry legs replay the identical send sequence.
    private static final String[][] CASE_OPS = {
            // over-filter: E1(1) E2(1) E3(2) E4(3) E5(2) E6(3) E7(1) E8(0)
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // over-filter-expiry: identical sends
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // followedby-with-distinct: A1(1), B1(0), B2(1), B3(0),
            // A2(1), B4(2), A3(2), B5(1), B6(1), B7(3)
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send",
                    "undeploy-all"},
            // followedby-with-distinct-expiry: identical sends
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send",
                    "undeploy-all"},
            // month-scoped: sendCurrentTime(2002-02-01T09:00:00.000),
            // deploy, E1(1), E1(2), sendCurrentTimeWithMinus(mark,1),
            // E1(3), milestone(0) omitted, sendCurrentTime(mark), E1(4)
            {"advance-time", "deploy",
                    "send", "send",
                    "advance-time",
                    "send",
                    "advance-time",
                    "send",
                    "undeploy-all"}
    };

    // Pinned advance-time instants per case, in op order.
    private static final String[][] CASE_ATS = {
            {}, {}, {}, {},
            {AT_START, AT_MINUS_1MS, AT_MONTH_MARK}
    };

    // Pinned send payloads, in send order: "SupportBean|theString|
    // intPrimitive".
    private static final String[][] CASE_SENDS = {
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|2", "SupportBean|E6|3",
                    "SupportBean|E7|1", "SupportBean|E8|0"},
            {"SupportBean|E1|1", "SupportBean|E2|1", "SupportBean|E3|2",
                    "SupportBean|E4|3", "SupportBean|E5|2", "SupportBean|E6|3",
                    "SupportBean|E7|1", "SupportBean|E8|0"},
            {"SupportBean|A1|1", "SupportBean|B1|0", "SupportBean|B2|1",
                    "SupportBean|B3|0", "SupportBean|A2|1", "SupportBean|B4|2",
                    "SupportBean|A3|2", "SupportBean|B5|1", "SupportBean|B6|1",
                    "SupportBean|B7|3"},
            {"SupportBean|A1|1", "SupportBean|B1|0", "SupportBean|B2|1",
                    "SupportBean|B3|0", "SupportBean|A2|1", "SupportBean|B4|2",
                    "SupportBean|A3|2", "SupportBean|B5|1", "SupportBean|B6|1",
                    "SupportBean|B7|3"},
            {"SupportBean|E1|1", "SupportBean|E1|2", "SupportBean|E1|3",
                    "SupportBean|E1|4"}
    };

    private static final int EXPECTED_STEPS = 58;
    private static final int EXPECTED_RECORDS = 20;

    private PatternEveryDistinctFollowup579ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternEveryDistinctFollowup579ScenarioOracle <scenario.json>");
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
                        if (!"SupportBean".equals(eventType)) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        requireFields(payload, "theString", "intPrimitive");
                        String actual = eventType + "|"
                                + string(payload, "theString") + "|"
                                + integer(payload, "intPrimitive");
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
}
