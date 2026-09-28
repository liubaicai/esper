import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportBean_C;
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
 * Direct Esper 9.0.0 oracle for the pattern-followedby-timernot-581
 * bundle: PatternOperatorFollowedBy ords 1, 6 and 9 — the timer+not
 * trio — every-then followed-by whose second leg is
 * {@code timer:interval(...) and not ...}, replayed as three cases each
 * against a fresh runtime like the regression suite's per-execution
 * environment. The internal timer is disabled and every clock move is an
 * explicit advance-time step (external timer semantics), so timer
 * expirations surface exactly at the pinned instants.
 *
 * with-not (PatternFollowedByWithNot, ord 1) deploys {@code every
 * a=SupportBean_A -> (timer:interval(10 seconds) and not
 * (SupportBean_B(id=a.id) or SupportBean_C(id=a.id)))} — the EPL keeps
 * the leading space after {@code [} and the trailing space after
 * {@code ]} verbatim. Each A arms a per-branch 10-second timer cancelled
 * only by a same-id B or C: A1 fires at t=10000, B(A2) at t=29999
 * cancels A2's branch (t=30000 silent), C(A3) at t=30000 cancels A3's
 * branch (t=40000 silent), and non-matching B(B4)/C(A5) leave A4 firing
 * at t=50000 — 2 listener records, each the {@code a} fragment under
 * select *.
 *
 * not-every (PatternFollowedNotEvery, ord 6) deploys {@code every
 * A=SupportBean -> (timer:interval(1 seconds) and not SupportBean_A)}.
 * The terminator is UNCORRELATED — any SupportBean_A would quiesce every
 * armed branch, but none is sent. Two default SupportBean sends at t=0
 * (theString null) arm two branches whose timers expire together at
 * t=1000: the Java assertion pins getNewDataList().size()==1 AND
 * get(0).length==2 — ONE listener invocation carrying TWO rows, the
 * batching discriminant of this slice.
 *
 * or-perm-false (PatternFollowedOrPermFalse, ord 9) deploys {@code every
 * s=SupportBean(theString='E') -> (timer:interval(10) and not
 * SupportBean(theString='C1'))or(SupportBean(theString='C2') and not
 * timer:interval(10))} — the literal {@code )or(} (no spaces) is pinned
 * byte-exact. The bare numeric timer:interval(10) is SECONDS under the
 * pattern observer's time abacus (deltaForSeconds; the external clock
 * still advances in millis): the right alternative's
 * {@code not timer:interval(10)} arms at the E match (t=1000) and dies
 * permanently at t=11000 without a
 * C2, so the E sent at t=1000 stays silent at t=10999 and fires alone
 * at t=11000 — 1 record.
 *
 * {@code select *} on all three executions projects every bound tag as a
 * fragment property — {@code a}, {@code A} and {@code s} respectively;
 * the unnamed not-branches project nothing. All env.milestone savepoints
 * are harness splits restoring identical state and are unrepresented
 * (ord 1 milestones 0/1/2, ord 6 milestone 1, ord 9 milestone 0).
 * SupportBean renders only the pinned theString/intPrimitive pair and
 * SupportBean_A/B/C only their id — the subset the Go beans carry.
 */
public final class PatternFollowedByTimerNot581ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-followedby-timernot-581";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java"};

    private static final String DESCRIPTION = "PatternOperatorFollowedBy ords 1, 6 and 9 — the timer+not trio: every-then followed-by "
                    + "whose second leg is `timer:interval(...) and not ...`, all driven by the external clock "
                    + "(advance-time ops; engine timer threading off). with-not (PatternFollowedByWithNot, ord "
                    + "1) arms a 10-second timer per SupportBean_A cancelled by a correlated B/C: A1 fires at "
                    + "t=10000, correlated B(A2)/C(A3) cancels keep A2/A3 silent, non-correlated B(B4)/C(A5) "
                    + "leave A4 firing at t=50000 — 2 listener records, select * projecting the SupportBean_A "
                    + "fragment. not-every (PatternFollowedNotEvery, ord 6) arms a 1-second timer per "
                    + "uncorrelated SupportBean_A terminator: two SupportBean sends at t=0 deliver ONE listener "
                    + "invocation carrying TWO rows at t=1000 — the batching pin. or-perm-false "
                    + "(PatternFollowedOrPermFalse, ord 9) ORs the timer leg against `(theString='C2' and not "
                    + "timer:interval(10))`: bare numeric timer parameters are seconds under the pattern "
                    + "observer's time abacus (external clock still advances in millis); the `)or(` sits inside "
                    + "the followed-by's second leg, so the right alternative's not-timer arms at the E match "
                    + "(t=1000) and dies permanently at t=11000 without C2 — the same instant the left branch's "
                    + "timer fires the E send at t=1000 — 1 record, the literal `)or(` kept byte-exact. All "
                    + "env.milestone savepoints are harness splits restoring identical state and are "
                    + "unrepresented.";

    private static final String[] CASES = {
            "with-not", "not-every", "or-perm-false"};
    private static final int[] CASE_ORDINALS = {1, 6, 9};
    private static final int[] CASE_RUNTIME_INDEX = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-df0604ce4c4e286b3e6e",
            "java-runtime-8631344fa90a6c397c84",
            "java-runtime-7682cbc4f27c52c33cce"};
    private static final String[] EXECUTIONS = {
            "PatternFollowedByWithNot",
            "PatternFollowedNotEvery",
            "PatternFollowedOrPermFalse"};
    // Deduplicated inventory id: all ten PatternOperatorFollowedBy
    // inventory rows share static id java-089b2086c9945dff918f (NOT the
    // every-distinct file's java-073091a0b42ca8dd974f), pinned once per
    // runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-089b2086c9945dff918f",
            "java-089b2086c9945dff918f",
            "java-089b2086c9945dff918f"};
    private static final String[] OBSERVATIONS = {
            "listener; advance-time drives the 10-second timers; `every a=SupportBean_A -> "
                    + "(timer:interval(10 seconds) and not (SupportBean_B(id=a.id) or "
                    + "SupportBean_C(id=a.id)))`: A1 fires at t=10000, B(A2)/C(A3) correlated "
                    + "cancels stay silent, B(B4)/C(A5) non-matching cancels let A4 fire at "
                    + "t=50000 — 2 records, select * projects the a fragment",
            "listener; `every A=SupportBean -> (timer:interval(1 seconds) and not "
                    + "SupportBean_A)` — the terminator is UNCORRELATED: two SupportBean sends "
                    + "at t=0 arm both branches, advance to t=1000 delivers ONE listener "
                    + "invocation with TWO rows (Java asserts getNewDataList().size()==1 and "
                    + "get(0).length==2)",
            "listener; `every s=SupportBean(theString='E') -> (timer:interval(10) and not "
                    + "SupportBean(theString='C1'))or(SupportBean(theString='C2') and not "
                    + "timer:interval(10))` — literal )or( no spaces; bare timer:interval(10) "
                    + "is SECONDS: right alternative's not-timer arms at the E match and dies permanently at t=11000, "
                    + "E sent at t=1000 stays silent at t=10999 and fires at t=11000 — 1 record"};

    private static final String EPL_WITH_NOT =
            "@name('s0') select * from pattern [ every a=SupportBean_A -> "
                    + "(timer:interval(10 seconds) and not (SupportBean_B(id=a.id) or "
                    + "SupportBean_C(id=a.id)))] ";
    private static final String EPL_NOT_EVERY =
            "@name('s0') select * from pattern [every A=SupportBean -> "
                    + "(timer:interval(1 seconds) and not SupportBean_A)]";
    private static final String EPL_OR_PERM_FALSE =
            "@name('s0') select * from pattern [every s=SupportBean(theString='E') -> "
                    + "(timer:interval(10) and not SupportBean(theString='C1'))"
                    + "or"
                    + "(SupportBean(theString='C2') and not timer:interval(10))]";

    private static final String[] CASE_EPLS = {
            EPL_WITH_NOT, EPL_NOT_EVERY, EPL_OR_PERM_FALSE};

    // Pinned op sequences (after the case marker), in Java source order.
    // env.milestone savepoints are regression-harness splits restoring
    // identical state and carry no scenario op (ord 1 milestones 0/1/2,
    // ord 6 milestone 1, ord 9 milestone 0). Every sendTimer/advanceTime
    // call is a pinned advance-time step, including ord 1's repeated
    // t=30000 advances around the A3/C(A3) pair.
    private static final String[][] CASE_OPS = {
            // with-not: A1 fires at 10000; B(A2) cancels (30000 silent);
            // C(A3) cancels (40000 silent); B(B4)/C(A5) non-correlated,
            // A4 fires at 50000.
            {"advance-time", "deploy", "send",
                    "advance-time", "advance-time",
                    "advance-time", "send",
                    "advance-time", "send", "advance-time",
                    "advance-time", "send",
                    "advance-time", "send", "advance-time",
                    "send", "send", "send", "advance-time",
                    "undeploy-all"},
            // not-every: two default SupportBean sends at t=0, advance to
            // t=1000 delivers ONE invocation with TWO rows.
            {"advance-time", "deploy", "send", "send",
                    "advance-time",
                    "undeploy-all"},
            // or-perm-false: deploy at t=0, advance to t=1000, send E,
            // silent at t=10999, fires at t=11000.
            {"advance-time", "deploy",
                    "advance-time", "send",
                    "advance-time", "advance-time",
                    "undeploy-all"}
    };

    // Pinned send payloads, in send order per case:
    // "SupportBean_A|id" / "SupportBean_B|id" / "SupportBean_C|id" or
    // "SupportBean|theString|intPrimitive" (null renders as "~").
    private static final String[][] CASE_SENDS = {
            {"SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|A2",
                    "SupportBean_A|A3", "SupportBean_C|A3",
                    "SupportBean_A|A4", "SupportBean_B|B4", "SupportBean_C|A5"},
            {"SupportBean|~|0", "SupportBean|~|0"},
            {"SupportBean|E|0"}
    };

    // Pinned advance-time instants (epoch millis), in advance order per
    // case. Ord 1 repeats t=30000 three times verbatim like the Java
    // sendTimer calls.
    private static final long[][] CASE_ADVANCES = {
            {0L, 9999L, 10000L, 20000L, 29999L, 30000L, 30000L, 30000L, 40000L, 50000L},
            {0L, 1000L},
            {0L, 1000L, 10999L, 11000L}
    };

    private static final int EXPECTED_STEPS = 36;
    private static final int EXPECTED_RECORDS = 4;

    private PatternFollowedByTimerNot581ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternFollowedByTimerNot581ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_C.class);

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
        switch (eventType) {
            case "SupportBean": {
                JsonValue theString = payload.get("theString");
                runtime.getEventService().sendEventBean(
                        new SupportBean(theString == null || theString.isNull()
                                        ? null : theString.asString(),
                                payload.getInt("intPrimitive", 0)),
                        "SupportBean");
                return;
            }
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(payload.getString("id", null)), "SupportBean_A");
                return;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(payload.getString("id", null)), "SupportBean_B");
                return;
            case "SupportBean_C":
                runtime.getEventService().sendEventBean(
                        new SupportBean_C(payload.getString("id", null)), "SupportBean_C");
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
     * pinned ops — advance-time steps with pinned instants, deploy s0
     * with the verbatim EPL, sends with pinned payloads and
     * undeploy-all. Unknown step fields are rejected.
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
                            JsonValue theString = payload.get("theString");
                            actual = eventType + "|"
                                    + (theString == null || theString.isNull()
                                            ? "~" : theString.asString())
                                    + "|" + integer(payload, "intPrimitive");
                        } else if ("SupportBean_A".equals(eventType)
                                || "SupportBean_B".equals(eventType)
                                || "SupportBean_C".equals(eventType)) {
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
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length || deploys != 1
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
        if (value instanceof SupportBean) {
            // Tagged-event columns arrive as the underlying bean; render
            // the pinned two-property subset exactly like the Go
            // NormalizeResults event rendering (the default SupportBean()
            // leaves theString null -> the null marker).
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            return new JsonObject().add("kind", "row").add("fields", fields);
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
