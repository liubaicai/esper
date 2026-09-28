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
import com.espertech.esper.regressionlib.support.bean.SupportBeanAtoFBase;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
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
 * Direct Esper 9.0.0 oracle for the pattern-matchuntil-repeattags-587
 * unit: PatternOperatorMatchUntil ord 4 PatternRepeatUseTags — ONE
 * execution replayed as THREE sequential deploy/undeploy-all legs
 * inside a single case under one runtimeId, mirroring the Java
 * execution's undeployAll calls inside one runtime (the same
 * ONE-case/multi-deploy convention as pattern-followedby-chain-582).
 * env.milestone() savepoints restore identical state and carry no
 * steps; env.advanceTime calls become advance-time steps. The internal
 * timer is disabled for determinism like the regression suite's
 * deterministic runtime.
 *
 *   - leg 1 {@code every [2] (a=SupportBean_A() -> b=SupportBean_B(
 *     id=a.id))}: sends A(id=A1), B(id=A1), A(id=A2), B(id=A2) and
 *     fires ONCE — the correlated b filter reads the `a` that opened
 *     THAT repeat iteration (per-iteration scope is the discriminant).
 *     select * delivers {@code {a:[A1,A2], b:[B(A1),B(A2)]}}.
 *   - leg 2 {@code every ([2:]e1=SupportBean(theString='2') until
 *     timer:interval(5))->([2:]e2=SupportBean(theString='3') until
 *     timer:interval(2))} under the external clock: advanceTime(0)
 *     precedes the deploy, then sends 2,2 / advance 5000 / sends
 *     3,3,3,3 / advance 10000 / sends 2,2 / advance 15000. Esper's
 *     until is a SUCCESSFUL terminator: the e1 until-timer at t=5000
 *     completes its repeat delivering the two collected '2's into the
 *     sequence and arms e2, whose own until-timer expires at t=7000
 *     completing with all four collected '3's — the firing lands
 *     INSIDE advanceTime(10000): advanceTime sets the clock to the
 *     target before processing due schedules, so the listener records
 *     at t=10000. Java asserts nothing on this leg, so the oracle
 *     trace is the sole arbiter; the record MUST be reproduced
 *     verbatim by the Go replay.
 *   - leg 3 {@code pattern [ every [2] A=SupportBean(theString='1') ->
 *     [2] B=SupportBean(theString='2' and intPrimitive=A[0].
 *     intPrimitive)-> [2] C=SupportBean(theString='3' and intPrimitive
 *     =A[0].intPrimitive)]} (double space after {@code pattern [} from
 *     source concatenation): sends (1,10), (1,20), (2,10)x2, (3,10)x2
 *     and fires ONCE — B and C read {@code A[0].intPrimitive}, the
 *     FIRST element of the completed A-repeat (10), deliberately
 *     disambiguated from the nearest A value 20.
 *
 * Listener sequence numbers are per deployment, like the Go runner's
 * per-deploy counter reset on each undeploy-all.
 */
public final class PatternMatchUntilRepeatTags587ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-matchuntil-repeattags-587";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java"};

    private static final String DESCRIPTION =
            "PatternOperatorMatchUntil ord 4 PatternRepeatUseTags — ONE "
                    + "execution replayed as THREE sequential deploy/undeploy-all legs under "
                    + "one runtimeId. Leg 1 `every [2] (a=SupportBean_A() -> "
                    + "b=SupportBean_B(id=a.id))` sends A1/B(A1)/A2/B(A2) and fires ONCE: "
                    + "the correlated b filter reads the a that opened THAT repeat "
                    + "iteration (per-iteration scope is the discriminant — a "
                    + "non-matching B must not cross-complete another iteration). Leg 2 "
                    + "`every ([2:]e1=SupportBean(theString='2') until timer:interval(5))"
                    + "->([2:]e2=SupportBean(theString='3') until timer:interval(2))` runs "
                    + "under the external clock — advanceTime(0) before the deploy, sends "
                    + "2,2 / advance 5000 / sends 3,3,3,3 / advance 10000 / sends 2,2 / "
                    + "advance 15000: Esper's until is a SUCCESSFUL terminator, so the e1 "
                    + "repeat completes at t=5000 delivering its two '2's into the sequence "
                    + "and the e2 repeat completes when its until-timer expires at t=7000 "
                    + "with the four collected '3's — the firing lands inside "
                    + "advanceTime(10000), which sets the clock to the target before "
                    + "processing due schedules, so ONE listener record at t=10000 (Java "
                    + "asserts nothing for this leg, so any delivery must match the oracle "
                    + "verbatim). Leg 3 `pattern [ every [2] A=SupportBean(theString='1') -> "
                    + "[2] B=SupportBean(theString='2' and intPrimitive=A[0].intPrimitive)"
                    + "-> [2] C=SupportBean(theString='3' and intPrimitive=A[0].intPrimitive)]` "
                    + "(double space after `pattern [` from source concatenation) sends "
                    + "(1,10)/(1,20)/(2,10)x2/(3,10)x2 and fires ONCE: B and C read "
                    + "A[0].intPrimitive — the FIRST repeat element 10, not the nearest A "
                    + "value 20. milestone() savepoints carry no steps.";

    private static final String[] CASES = {"repeat-use-tags"};
    private static final int[] CASE_ORDINALS = {4};
    private static final int[] CASE_RUNTIME_INDEX = {0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-eb238a96331acf6fc30b"};
    private static final String[] EXECUTIONS = {
            "PatternRepeatUseTags"};
    // Static-manifest ids per execution (discovery static-candidate).
    private static final String[] STATIC_IDS = {
            "java-b2c644fc2f9603bd8568"};
    private static final String[] OBSERVATIONS = {
            "listener x3; ONE execution, THREE sequential deploy/undeploy-all legs under "
                    + "one runtimeId: leg 1 `every [2] (a=SupportBean_A() -> "
                    + "b=SupportBean_B(id=a.id))` fires ONCE at t=0 — sends A(id=A1), "
                    + "B(id=A1), A(id=A2), B(id=A2) deliver {a:[A1,A2], b:[B(A1),B(A2)]}; "
                    + "the b predicate `id=a.id` reads the a that opened that same repeat "
                    + "iteration (per-iteration correlation scope); leg 2 `every "
                    + "([2:]e1='2' until timer:interval(5))->([2:]e2='3' until "
                    + "timer:interval(2))` under advance-time 0/5000/10000/15000 — the e1 "
                    + "until-timer completes its repeat at t=5000 (two collected '2's), "
                    + "arming e2 whose own until-timer expires at t=7000 with all four "
                    + "collected '3's — the firing lands inside advanceTime(10000) so ONE "
                    + "record at t=10000 (Java has no listener assert "
                    + "on this leg); leg 3 `every [2] A=SupportBean('1') -> [2] "
                    + "B=SupportBean('2' and intPrimitive=A[0].intPrimitive)"
                    + "-> [2] "
                    + "C=SupportBean('3' and intPrimitive=A[0].intPrimitive)` fires ONCE "
                    + "at t=15000 — B and C correlate on A[0].intPrimitive, the first "
                    + "repeat element (10), deliberately distinguishable from A[1]=20; "
                    + "milestone 0-3 savepoints carry no steps"};

    // Byte-exact EPL pins (PatternOperatorMatchUntil.java verbatim):
    // leg 1 keeps the empty parens and the unquoted id=a.id correlation;
    // leg 2 keeps the [2:] open ranges and parenthesized until-branches;
    // leg 3 keeps the double space after `pattern [` plus the A[0]
    // first-element reads.
    private static final String EPL_LEG1 =
            "@name('s0') select * from pattern [every [2] (a=SupportBean_A() -> "
                    + "b=SupportBean_B(id=a.id))]";
    private static final String EPL_LEG2 =
            "@name('s0') select * from pattern [every ([2:]e1=SupportBean("
                    + "theString='2') until timer:interval(5))->([2:]e2=SupportBean("
                    + "theString='3') until timer:interval(2))]";
    private static final String EPL_LEG3 =
            "@name('s0') select * from pattern [ every [2] A=SupportBean("
                    + "theString='1') -> [2] B=SupportBean(theString='2' and intPrimitive="
                    + "A[0].intPrimitive)-> [2] C=SupportBean(theString='3' and "
                    + "intPrimitive=A[0].intPrimitive)]";

    // Pinned EPLs per deploy position: the single case deploys s0 three
    // times — one per leg in Java source order.
    private static final String[][] CASE_EPLS = {
            {EPL_LEG1, EPL_LEG2, EPL_LEG3}};

    // Pinned op sequences (after the case marker), in Java source order.
    // env.milestone savepoints are regression-harness splits restoring
    // identical state and carry no scenario op (milestones 0-3). The
    // three deploy legs and the undeploy-all separators sit inside the
    // ONE case block exactly like the Java undeployAll inside one
    // execution; leg 2's advanceTime(0) precedes its deploy.
    private static final String[][] CASE_OPS = {
            // leg 1: every [2] (a -> b correlated) fires once.
            {"deploy", "send", "send", "send", "send", "undeploy-all",
                    // leg 2: advance 0 pre-deploy, then sends and the
                    // pinned advances driving the until-timers.
                    "advance-time", "deploy", "send", "send", "advance-time",
                    "send", "send", "send", "send", "advance-time", "send",
                    "send", "advance-time", "undeploy-all",
                    // leg 3: three-stream repeat with A[0] correlation.
                    "deploy", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads, in send order per case:
    // "SupportBean_A|id" / "SupportBean_B|id" (leg 1) or
    // "SupportBean|theString|intPrimitive" (legs 2/3).
    private static final String[][] CASE_SENDS = {
            {"SupportBean_A|A1", "SupportBean_B|A1",
                    "SupportBean_A|A2", "SupportBean_B|A2",
                    "SupportBean|2|0", "SupportBean|2|0",
                    "SupportBean|3|0", "SupportBean|3|0",
                    "SupportBean|3|0", "SupportBean|3|0",
                    "SupportBean|2|0", "SupportBean|2|0",
                    "SupportBean|1|10", "SupportBean|1|20",
                    "SupportBean|2|10", "SupportBean|2|10",
                    "SupportBean|3|10", "SupportBean|3|10"}
    };

    // Pinned advance-time instants (epoch millis), in order:
    // advanceTime(0) pre-deploy, then 5000 / 10000 / 15000.
    private static final long[][] CASE_ADVANCES = {
            {0L, 5000L, 10000L, 15000L}
    };

    private static final int EXPECTED_STEPS = 29;
    private static final int EXPECTED_RECORDS = 3;

    private PatternMatchUntilRepeatTags587ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternMatchUntilRepeatTags587ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportBean_B.class);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            int deployIndex = 0;
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
        switch (eventType) {
            case "SupportBean":
                runtime.getEventService().sendEventBean(
                        new SupportBean(payload.getString("theString", null),
                                integer(payload, "intPrimitive")),
                        "SupportBean");
                return;
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(payload.getString("id", null)),
                        "SupportBean_A");
                return;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(payload.getString("id", null)),
                        "SupportBean_B");
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
        for (int index = 0; index < cases.size(); index++) {
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
            validateStringArray(definition.get("epls"), CASE_EPLS[index], "epls");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: the case marker is followed by the
     * pinned ops — each leg's deploy s0 step carrying the pinned EPL per
     * deploy position (the case deploys three times), the pinned sends
     * and advance-time instants, and the undeploy-all separators. The
     * leg 2 advanceTime(0) precedes its deploy like the Java source.
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
                                || !CASE_EPLS[caseIndex][deploys].equals(string(step, "epl"))) {
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
                        String actual;
                        if ("SupportBean".equals(eventType)) {
                            requireFields(payload, "theString", "intPrimitive");
                            actual = eventType + "|" + string(payload, "theString")
                                    + "|" + integer(payload, "intPrimitive");
                        } else if ("SupportBean_A".equals(eventType)
                                || "SupportBean_B".equals(eventType)) {
                            requireFields(payload, "id");
                            actual = eventType + "|" + string(payload, "id");
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
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != CASE_EPLS[caseIndex].length
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
        long parsed = longInteger(object, name);
        if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) parsed;
    }

    private static long longInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        return longInteger(value, name);
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(label + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
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
            // Render the asserted-field projection {theString,intPrimitive}
            // so the trace carries no unasserted engine defaults.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBeanAtoFBase) {
            // SupportBean_A/B share the id-only SupportBeanAtoFBase:
            // tagged-event columns arrive as the underlying bean and
            // render the single {id} property like the Go NormalizeResults
            // event rendering.
            SupportBeanAtoFBase bean = (SupportBeanAtoFBase) value;
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
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
