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
import com.espertech.esper.regressionlib.support.bean.SupportBean_D;
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
 * Direct Esper 9.0.0 oracle for the pattern-followedby-chain-582
 * bundle: PatternOperatorFollowedBy ords 7 and 8 — the pure
 * followed-by chain pair (no timers, no guards, no clock ops) —
 * replayed as two cases each against a fresh runtime like the
 * regression suite's per-execution environment. The internal timer is
 * disabled for determinism and no step ever moves the clock: every
 * listener record reports the initialized epoch time.
 *
 * every-multiple (PatternFollowedEveryMultiple, ord 7) deploys
 * {@code every a=SupportBean_A -> b=SupportBean_B -> c=SupportBean_C ->
 * d=SupportBean_D} — the single space after {@code pattern} is pinned
 * byte-exact. The sends A1, A2, B1, C1, D1 arm TWO branches off the
 * head-leg every that share the B1/C1/D1 tail, so the D1 send completes
 * both in ONE listener invocation carrying TWO rows —
 * [A1,B1,C1,D1] first then [A2,B1,C1,D1] — matching the Java
 * assertPropsPerRowLastNew order.
 *
 * filter-greater-then (PatternFilterGreaterThen, ord 8, ESPER-411) is
 * ONE Java execution carrying TWO sequential deploy/undeployAll cycles
 * under the same runtimeId, modeled as one case with two deploy blocks:
 * phase 1 {@code pattern[every a=SupportBean ->
 * b=SupportBean(b.intPrimitive <= a.intPrimitive)]} — the spaceless
 * {@code pattern[} bracket is pinned byte-exact — and phase 2
 * {@code pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive
 * >= b.intPrimitive)]} — spaced {@code pattern [} and the operand order
 * flipped. Each phase sends E1(10) then E2(11) and fires ZERO times:
 * the correlated second-leg predicate evaluates against the captured
 * first-leg event in either operand order (11<=10 false; 10>=11 false),
 * so ord 8 contributes no records.
 *
 * {@code select *} on both executions projects every bound tag as a
 * fragment property — a/b/c/d (ord 7) and a/b (ord 8, unobservable:
 * both phases are silent). All env.milestone savepoints are harness
 * splits restoring identical state and are unrepresented (ord 7
 * milestones 0/1, ord 8 milestones 0/1). SupportBean renders only the
 * pinned theString/intPrimitive pair and SupportBean_A/B/C/D only their
 * id — the subset the Go beans carry.
 */
public final class PatternFollowedByChain582ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-followedby-chain-582";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java"};

    private static final String DESCRIPTION = "PatternOperatorFollowedBy ords 7 and 8 — the pure "
                    + "followed-by chain pair (no timers, no guards, no clock ops): every-head "
                    + "fan-out over a shared tail plus a correlated second-leg predicate. "
                    + "every-multiple (PatternFollowedEveryMultiple, ord 7) deploys `every "
                    + "a=SupportBean_A -> b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D`: "
                    + "sends A1, A2, B1, C1, D1 deliver ONE listener invocation carrying TWO rows "
                    + "— [A1,B1,C1,D1] first then [A2,B1,C1,D1] — both A-branches pair with the "
                    + "shared B1/C1/D1 tail, select * projecting the a/b/c/d fragments. "
                    + "filter-greater-then (PatternFilterGreaterThen, ord 8, ESPER-411) runs TWO "
                    + "sequential deploy/undeploy-all cycles under ONE runtimeId in a single "
                    + "execution: phase 1 `pattern[every a=SupportBean -> "
                    + "b=SupportBean(b.intPrimitive <= a.intPrimitive)]` (no space after "
                    + "pattern), phase 2 `pattern [every a=SupportBean -> "
                    + "b=SupportBean(a.intPrimitive >= b.intPrimitive)]` (with space) — each "
                    + "phase sends E1(10), E2(11) and fires ZERO times because the correlated "
                    + "second-leg predicate evaluates against the captured a event in either "
                    + "operand order. All env.milestone savepoints are harness splits restoring "
                    + "identical state and are unrepresented.";

    private static final String[] CASES = {
            "every-multiple", "filter-greater-then"};
    private static final int[] CASE_ORDINALS = {7, 8};
    private static final int[] CASE_RUNTIME_INDEX = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-25f948812c392841f1e5",
            "java-runtime-4e9e6ad069be6a8e6074"};
    private static final String[] EXECUTIONS = {
            "PatternFollowedEveryMultiple",
            "PatternFilterGreaterThen"};
    // Deduplicated inventory id: all ten PatternOperatorFollowedBy
    // inventory rows share static id java-089b2086c9945dff918f (NOT the
    // every-distinct file's java-073091a0b42ca8dd974f), pinned once per
    // runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-089b2086c9945dff918f",
            "java-089b2086c9945dff918f"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock; `every a=SupportBean_A -> b=SupportBean_B -> "
                    + "c=SupportBean_C -> d=SupportBean_D`: A1+A2 arm two branches that share "
                    + "the B1/C1/D1 tail, so D1 completes both — ONE listener invocation "
                    + "carrying TWO rows, [A1,B1,C1,D1] first then [A2,B1,C1,D1]; select * "
                    + "projects the a/b/c/d fragments",
            "listener; no clock; ESPER-411 — TWO sequential deploy/undeploy-all cycles under "
                    + "ONE runtimeId: `pattern[every a=SupportBean -> b=SupportBean("
                    + "b.intPrimitive <= a.intPrimitive)]` (no space after pattern) then "
                    + "`pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive >= "
                    + "b.intPrimitive)]` (with space); each phase sends E1(10), E2(11) and "
                    + "fires ZERO times in both operand orders — 0 records"};

    private static final String EPL_EVERY_MULTIPLE =
            "@name('s0') select * from pattern [every a=SupportBean_A -> "
                    + "b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D]";
    private static final String EPL_LESS_OR_EQUAL =
            "@name('s0') select * from pattern[every a=SupportBean -> "
                    + "b=SupportBean(b.intPrimitive <= a.intPrimitive)]";
    private static final String EPL_GREATER_OR_EQUAL =
            "@name('s0') select * from pattern [every a=SupportBean -> "
                    + "b=SupportBean(a.intPrimitive >= b.intPrimitive)]";

    // Pinned EPLs per deploy position: ord 7 deploys once; ord 8's
    // single case deploys twice under the same runtimeId — spaceless
    // pattern[ phase 1, spaced pattern [ phase 2, operand order flipped.
    private static final String[][] CASE_EPLS = {
            {EPL_EVERY_MULTIPLE},
            {EPL_LESS_OR_EQUAL, EPL_GREATER_OR_EQUAL}};

    // Pinned op sequences (after the case marker), in Java source order.
    // env.milestone savepoints are regression-harness splits restoring
    // identical state and carry no scenario op (ord 7 milestones 0/1,
    // ord 8 milestones 0/1). Ord 8's two deploy legs and the undeploy-all
    // between them sit inside the ONE case block exactly like the Java
    // undeployAll inside one execution.
    private static final String[][] CASE_OPS = {
            // every-multiple: A1, A2 arm two branches; B1/C1/D1 shared
            // tail completes both in ONE delivery.
            {"deploy", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // filter-greater-then: phase 1 deploy/sends/undeploy-all,
            // phase 2 deploy/sends/undeploy-all — both silent.
            {"deploy", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "undeploy-all"}
    };

    // Pinned send payloads, in send order per case:
    // "SupportBean_A|id" / "SupportBean_B|id" / "SupportBean_C|id" /
    // "SupportBean_D|id" or "SupportBean|theString|intPrimitive".
    private static final String[][] CASE_SENDS = {
            {"SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|B1",
                    "SupportBean_C|C1", "SupportBean_D|D1"},
            {"SupportBean|E1|10", "SupportBean|E2|11",
                    "SupportBean|E1|10", "SupportBean|E2|11"}
    };

    private static final int EXPECTED_STEPS = 17;
    private static final int EXPECTED_RECORDS = 1;

    private PatternFollowedByChain582ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternFollowedByChain582ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_D.class);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
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
                    case "deploy": {
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                new CompilerArguments(runtime.getRuntimePath()));
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                                + "-" + deployIndex));
                        deployIndex++;
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
            case "SupportBean_D":
                runtime.getEventService().sendEventBean(
                        new SupportBean_D(payload.getString("id", null)), "SupportBean_D");
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
            validateStringArray(definition.get("epls"), CASE_EPLS[index], "epls");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: the case marker is followed by the
     * pinned ops — deploy s0 steps carrying the pinned EPL per deploy
     * position (ord 8's case deploys twice), sends with pinned payloads
     * and undeploy-all. No clock ops exist in this slice. Unknown step
     * fields are rejected.
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
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex][deploys].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deploys++;
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
                                || "SupportBean_C".equals(eventType)
                                || "SupportBean_D".equals(eventType)) {
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
                    || deploys != CASE_EPLS[caseIndex].length) {
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
            // NormalizeResults event rendering.
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
        if (value instanceof SupportBean_D) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_D) value).getId()));
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
