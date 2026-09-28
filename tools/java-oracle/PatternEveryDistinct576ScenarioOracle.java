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
 * Direct Esper 9.0.0 oracle for the pattern-everydistinct-576 bundle:
 * PatternOperatorEveryDistinct ords 0-2 — the single-filter
 * every-distinct slice — replayed as three cases, each against a fresh
 * runtime like the regression suite's per-execution environment.
 *
 * every-distinct-simple (PatternEveryDistinctSimple, ord 0) deploys the
 * byte-exact {@code @Name('s0') select a.theString as c0 from pattern
 * [every-distinct(a.theString) a=SupportBean]} with no clock: the first
 * event per distinct theString key fires c0 and later duplicates are
 * swallowed.
 *
 * every-distinct-w-time (PatternEveryDistinctWTime, ord 1) deploys the
 * same statement with {@code , 5 sec} expiry on the virtual clock:
 * advanceTime(0) before deploy and advanceTime(15000) before the first
 * send, mirroring the Java execution. The boundary is pinned: E1
 * first-seen at t=15000 is still swallowed at t=19999 and fires again
 * at t=20000 — the key expires AT first-seen+5000 — while E2
 * (first-seen 18000) stays swallowed past the boundary.
 *
 * expire-seen-before-key (PatternExpireSeenBeforeKey, ord 2) deploys the
 * byte-exact {@code @name('s0') select * from pattern
 * [every-distinct(a.intPrimitive, 1 sec) a=SupportBean(theString like
 * 'A%')]} — lowercase @name is verbatim Java source, not a typo. The
 * distinct key is intPrimitive; select * projects the captured tag's
 * bean, whose theString is what the Java assertions pin through
 * a.theString. Per-key expiry is measured from each key's first
 * sighting: both keys expire at t=1000 (A4/A5 re-fire, A6 silent),
 * and key 2's window ends at t=2000 (A7@1999 swallowed, A7@2000 fires).
 *
 * Java milestone() savepoints restore identical state for these
 * non-contextual executions and are omitted (573/575 precedent).
 *
 * SupportBean is the real common/internal support bean (theString +
 * intPrimitive ctor) mirroring the regression suite. Tagged-event
 * columns arrive at the listener as the underlying bean, so the trace
 * renders the bean's pinned field projection {theString,intPrimitive}
 * as a kind:row object exactly like the Go NormalizeResults event
 * rendering (EPLOtherPatternEventProperties precedent). Records follow
 * the standard protocol: one listener record per delivered update with
 * a per-case sequence counter starting at 1 and time rendered from the
 * current engine time.
 */
public final class PatternEveryDistinct576ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-everydistinct-576";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "PatternOperatorEveryDistinct ords 0-2 — the single-filter every-distinct slice over SupportBean. "
                    + "every-distinct-simple (PatternEveryDistinctSimple, ord 0) replays "
                    + "`@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString) a=SupportBean]` "
                    + "with no clock: E1 fires c0=E1, two E1 duplicates are swallowed, E2 fires c0=E2, the E1/E2 dup "
                    + "pairs stay silent and E3 fires c0=E3. every-distinct-w-time (PatternEveryDistinctWTime, ord 1) "
                    + "replays the same statement with `, 5 sec` expiry over the virtual clock: advanceTime(0) before "
                    + "deploy, then E1@15000 fires (dup swallowed); at t=18000 E1 stays swallowed and E2 fires; at "
                    + "t=19999 E1 still does not fire — the key expires AT first-seen+5000, so E1@20000 fires while "
                    + "E2 (first-seen 18000) stays swallowed, then the E1/E2 dups are silent and E3 fires c0=E3. "
                    + "expire-seen-before-key (PatternExpireSeenBeforeKey, ord 2) replays `@name('s0') select * from "
                    + "pattern [every-distinct(a.intPrimitive, 1 sec) a=SupportBean(theString like 'A%')]`: the key is "
                    + "intPrimitive, the filter gates on theString like 'A%' and select * delivers the captured bean; "
                    + "A1(1)/A3(2) fire at t=0, A4(1)/A5(2) are swallowed, then at t=1000 both keys expire so "
                    + "A4(1)/A5(2) fire, A6(1) is swallowed, A7(2)@1999 stays swallowed and A7(2)@2000 fires — "
                    + "per-key expiry measured from each key's first sighting. Statement names keep the Java "
                    + "asymmetry verbatim: @Name('s0') for ords 0-1 vs @name('s0') for ord 2; Java milestone() "
                    + "savepoints are omitted (they restore identical state for these non-contextual executions).";

    private static final String[] CASES = {
            "every-distinct-simple", "every-distinct-w-time", "expire-seen-before-key"};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-593adaf9d26cd35ab4c9",
            "java-runtime-666dd3af9524272914d9",
            "java-runtime-904c2139b52a7eaeedfb"};
    private static final String[] EXECUTIONS = {
            "PatternEveryDistinctSimple", "PatternEveryDistinctWTime", "PatternExpireSeenBeforeKey"};
    // Deduplicated inventory id: the three runtime rows share static id
    // java-073091a0b42ca8dd974f with the other PatternOperatorEveryDistinct
    // executions, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f",
            "java-073091a0b42ca8dd974f"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock; the first event per distinct theString key fires c0 and later duplicates are "
                    + "swallowed: E1 fires, E1 x2 silent, E2 fires, E1+E2 and E1+E2 silent, E3 fires",
            "listener; 5-second per-key expiry from first sighting over the virtual clock: E1@15000 fires "
                    + "(dup swallowed), E1@18000 swallowed and E2 fires, E1@19999 still swallowed (the key "
                    + "expires AT first-seen+5000=20000), E1@20000 fires while E2 (first-seen 18000) stays "
                    + "swallowed, E1/E2 dups silent, E3 fires c0=E3",
            "listener; theString like 'A%' filter with the distinct key on intPrimitive and 1-second per-key "
                    + "expiry from each key's first sighting; select * projects the captured tag's bean (the "
                    + "a.theString the Java assertions pin): A1(1)/A3(2) fire at t=0, A4(1)/A5(2) swallowed, "
                    + "at t=1000 both keys expired so A4(1)/A5(2) fire, A6(1) silent, A7(2)@1999 swallowed, "
                    + "A7(2)@2000 fires"};

    private static final String EPL_SIMPLE =
            "@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString) a=SupportBean]";
    private static final String EPL_W_TIME =
            "@Name('s0') select a.theString as c0 from pattern [every-distinct(a.theString, 5 sec) a=SupportBean]";
    private static final String EPL_EXPIRY =
            "@name('s0') select * from pattern [every-distinct(a.intPrimitive, 1 sec) a=SupportBean(theString like 'A%')]";

    private static final String[] CASE_EPLS = {EPL_SIMPLE, EPL_W_TIME, EPL_EXPIRY};

    // Pinned op sequences (after the case marker), in Java source order.
    // Milestones are regression-harness savepoints restoring identical
    // state and carry no scenario op.
    private static final String[][] CASE_OPS = {
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"advance-time", "deploy",
                    "advance-time", "send", "send",
                    "advance-time", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"advance-time", "deploy",
                    "send", "send", "send", "send", "send",
                    "advance-time", "send", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "undeploy-all"}
    };

    // Pinned advance-time targets, in advance order. The 19999/20000 pair
    // pins expiry AT first-seen+5000 for the 5-second key; the 1999/2000
    // pair pins the same boundary for the 1-second key.
    private static final String[][] CASE_TIMES = {
            {},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:15.000Z",
                    "1970-01-01T00:00:18.000Z", "1970-01-01T00:00:19.999Z",
                    "1970-01-01T00:00:20.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:01.999Z", "1970-01-01T00:00:02.000Z"}
    };

    // Pinned send payloads, in send order, encoded
    // "SupportBean|theString|intPrimitive". Ord 0/1 sends carry
    // intPrimitive 0 like the Java sendSupportBean(string) helper
    // (new SupportBean(theString, 0)); ord 2 carries the distinct key.
    private static final String[][] CASE_SENDS = {
            {"SupportBean|E1|0", "SupportBean|E1|0", "SupportBean|E1|0",
                    "SupportBean|E2|0", "SupportBean|E1|0", "SupportBean|E2|0",
                    "SupportBean|E1|0", "SupportBean|E2|0", "SupportBean|E3|0"},
            {"SupportBean|E1|0", "SupportBean|E1|0",
                    "SupportBean|E1|0", "SupportBean|E2|0",
                    "SupportBean|E1|0",
                    "SupportBean|E1|0", "SupportBean|E2|0", "SupportBean|E1|0",
                    "SupportBean|E2|0", "SupportBean|E3|0"},
            {"SupportBean|A1|1", "SupportBean|A2|1", "SupportBean|A3|2",
                    "SupportBean|A4|1", "SupportBean|A5|2",
                    "SupportBean|A4|1", "SupportBean|A5|2", "SupportBean|A6|1",
                    "SupportBean|A7|2", "SupportBean|A7|2"}
    };

    private static final int EXPECTED_STEPS = 47;
    private static final int EXPECTED_RECORDS = 12;

    private PatternEveryDistinct576ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternEveryDistinct576ScenarioOracle <scenario.json>");
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
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
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
     * pinned ops — advance-time at the pinned clock targets, deploy s0
     * with the verbatim EPL (the @Name/@name case asymmetry preserved),
     * sends with pinned payloads and undeploy-all. Unknown step fields
     * are rejected.
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
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_TIMES[caseIndex][advances++].equals(string(step, "at"))) {
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
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != 1
                    || advances != CASE_TIMES[caseIndex].length) {
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
