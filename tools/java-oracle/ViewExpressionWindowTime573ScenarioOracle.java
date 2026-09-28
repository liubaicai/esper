import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
import java.util.Comparator;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the view-expression-window-time-573
 * bundle: ViewExpressionWindow ord 3 ViewExpressionWindowTimeWindow — the
 * keep-form timestamp window — replayed as a single case against a fresh
 * runtime. The bundle's two compile-error-only siblings
 * (ViewExpressionWindowInvalid ord 5, ViewExpressionBatchInvalid ord 4)
 * carry no replayable steps and are intentionally-different manifest
 * rows, so this oracle emits records for the time-window case only.
 *
 * time-window replays ViewExpressionWindowTimeWindow (ord 3):
 * {@code @name('s0') select irstream * from
 * SupportBean#expr(oldest_timestamp > newest_timestamp - 2000)}. The keep
 * predicate reads the retained rows' arrival timestamps (the KEEP form —
 * the expression-batch trigger form newest-oldest spread is a different
 * predicate). Expiry is lazy: the view re-evaluates only when new data
 * arrives, so advanceTime(10000) before the E8 send evicts nothing by
 * itself. The Java execution sends E3 with NO advanceTime after E2, so
 * E2 and E3 share the t=1500 arrival timestamp: E7@3500 evicts that tied
 * pair {E2,E3} in one delivery (1500 > 1500 is false — the double-old
 * discriminant) and E8@10000 mass-expires {E4,E5,E6,E7}.
 *
 * The Java execution asserts the iterator after E1 and E3..E8; this
 * oracle additionally snapshots after E2 (a stronger pin than the Java
 * assertion set). Java's milestone(0)/milestone(1) calls are
 * regression-harness savepoints restoring identical state and the
 * post-E4 listenerReset gates the assert-helper only — the raw listener
 * records every delivery — so the scenario omits all of them.
 *
 * SupportBean is the real common/internal support bean (theString +
 * intPrimitive ctor). Records follow the standard protocol: one listener
 * record per delivered update with a per-case sequence counter starting
 * at 1 and time rendered from the current engine time; the deployed
 * marker carries the deploy ordinal; snapshots carry sequence 0, the
 * pinned theString projection and "new" only when rows exist.
 */
public final class ViewExpressionWindowTime573ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-expression-window-time-573";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "ViewExpressionWindow ord 3 — the keep-form timestamp window. "
                    + "time-window (ViewExpressionWindowTimeWindow, ord 3) "
                    + "replays `@name('s0') select irstream * from "
                    + "SupportBean#expr(oldest_timestamp > "
                    + "newest_timestamp - 2000)` over virtual time: "
                    + "E1@1000, E2@1500, E3@1500 (the Java execution sends "
                    + "E3 with no advanceTime, so E2 and E3 share the 1500 "
                    + "arrival timestamp) and E4@2500 accumulate; E5@3000 "
                    + "evicts {E1} (oldest 1000 is not > newest 3000 - "
                    + "2000); E6@3499 is retained (window {E2,E3,E4,E5,E6}); "
                    + "E7@3500 evicts the tied pair {E2,E3} (1500 is not "
                    + "> 1500) — the double-old discriminant; E8@10000 "
                    + "mass-expires {E4,E5,E6,E7}. Expiry is lazy on send "
                    + "— the advanceTime(10000) before E8 evicts nothing "
                    + "by itself. Every send posts an in-order iterator "
                    + "snapshot (the post-E2 pin is stronger than the Java "
                    + "assertions, which skip the iterator there). Java "
                    + "milestone() savepoints and the post-E4 "
                    + "listenerReset are omitted: milestones restore "
                    + "identical state for this non-contextual execution "
                    + "and the reset gates Java's assert helper only — "
                    + "the raw listener records every delivery. The "
                    + "sibling compile-error executions "
                    + "ViewExpressionWindowInvalid (ord 5) and "
                    + "ViewExpressionBatchInvalid (ord 4) carry no "
                    + "replayable steps and join the manifest as "
                    + "intentionally-different rows.";

    private static final String[] CASES = {"time-window"};
    private static final int[] ORDINALS = {3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-5d04258cf2eb6e0025fd"};
    private static final String[] EXECUTIONS = {"ViewExpressionWindowTimeWindow"};
    // Deduplicated inventory id: the runtime row shares static id
    // java-06e6b1f6c905b8f12b82 with the other ViewExpressionWindow
    // executions, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-06e6b1f6c905b8f12b82"};
    private static final String[] OBSERVATIONS = {
            "deployed+listener+snapshot; keep-form timestamp window "
                    + "(oldest_timestamp > newest_timestamp - 2000 over "
                    + "row arrival timestamps; expiry is lazy — it "
                    + "re-evaluates on send, never on a bare advance): "
                    + "E1@1000, E2@1500, E3@1500 (tied arrival with E2), "
                    + "E4@2500 accumulate, E5@3000 evicts {E1}, E6@3499 "
                    + "is retained {E2..E6}, E7@3500 evicts the "
                    + "t=1500-tied pair {E2,E3} (double-old), E8@10000 "
                    + "mass-expires {E4,E5,E6,E7}; per-send in-order "
                    + "iterator snapshots project theString"};

    private static final String EPL_TIME_WINDOW =
            "@name('s0') select irstream * from SupportBean#expr(oldest_timestamp > newest_timestamp - 2000)";

    private static final String[] CASE_EPLS = {EPL_TIME_WINDOW};

    // Pinned row projection: the theString field the Java assertions pin.
    private static final String[][] CASE_FIELDS = {{"theString"}};

    // Pinned op sequence (after the case marker). Every send posts an
    // in-order iterator snapshot — including after E2, a stronger pin
    // than the Java assertions — and NO send/advance sits between the
    // 1500 advance and E3, so E3 inherits E2's t=1500 arrival timestamp.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "undeploy-all"}
    };

    // Pinned advance-time targets, in advance order.
    private static final String[][] CASE_TIMES = {
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:01.500Z", "1970-01-01T00:00:02.500Z",
                    "1970-01-01T00:00:03.000Z", "1970-01-01T00:00:03.499Z",
                    "1970-01-01T00:00:03.500Z", "1970-01-01T00:00:10.000Z"}
    };

    // Pinned send payloads, in send order, encoded
    // "SupportBean|theString|intPrimitive". E3 follows E2 with no
    // intervening advance-time: both rows carry the t=1500 arrival
    // timestamp that drives the E7 double-old eviction.
    private static final String[][] CASE_SENDS = {
            {"SupportBean|E1|1", "SupportBean|E2|2", "SupportBean|E3|3",
                    "SupportBean|E4|4", "SupportBean|E5|5", "SupportBean|E6|6",
                    "SupportBean|E7|7", "SupportBean|E8|8"}
    };

    private static final int EXPECTED_STEPS = 28;
    private static final int EXPECTED_RECORDS = 17;

    private ViewExpressionWindowTime573ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewExpressionWindowTime573ScenarioOracle <scenario.json>");
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
            int deployCount = 0;
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
                                new com.espertech.esper.runtime.client.DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                        deployCount++;
                        writer = new TraceWriter(records, caseName, findStatement(deployment),
                                runtime, CASE_FIELDS[caseIndex]);
                        writer.statement.addListener(writer);
                        break;
                    }
                    case "deployed": {
                        String label = step.getString("statement", "");
                        if (writer == null || !label.equals(writer.statement.getName())) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        writer.deployed(label);
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
                    case "snapshot": {
                        if (writer == null) {
                            throw new IllegalStateException("snapshot without a deployed statement");
                        }
                        writer.snapshot();
                        break;
                    }
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
            String theString = payload.getString("theString", null);
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            int intPrimitive = intPrimitiveVal instanceof JsonNumber
                    ? ((JsonNumber) intPrimitiveVal).asInt() : 0;
            runtime.getEventService().sendEventBean(
                    new SupportBean(theString, intPrimitive), "SupportBean");
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
                    "observation", "epl", "deploys");
            JsonArray deploys = array(definition.get("deploys"), "deploys");
            JsonValue deployLabel = deploys.size() == 1 ? deploys.get(0) : null;
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))
                    || deployLabel == null || !deployLabel.isString()
                    || !"s0".equals(deployLabel.asString())) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: the case marker is followed by the
     * pinned ops — advance-time at the pinned clock targets, deploy s0
     * with the verbatim EPL, a deployed marker, sends with pinned
     * payloads, per-send snapshots and undeploy-all. Unknown step fields
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
                    case "deployed":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deployed step " + cursor + " is not pinned");
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
                    case "snapshot":
                        requireFields(step, "op", "case", "statement", "mode");
                        if (!"s0".equals(string(step, "statement"))
                                || !"ordered".equals(string(step, "mode"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
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
        private final String[] fields;
        private long sequence;
        private long deployedSequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime, String[] fields) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.fields = fields;
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

        private void deployed(String label) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "deployed")
                    .add("statement", label)
                    .add("sequence", ++deployedSequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            records.add(record);
        }

        private void snapshot() {
            JsonArray rows = new JsonArray();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                rows.add(row(iterator.next(), fields));
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "snapshot")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (rows.size() > 0) {
                record.add("new", rows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(row(event, fields));
            }
            return output;
        }
    }

    private static JsonObject row(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
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
            for (String name : new java.util.TreeSet<>(
                    Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(name, normalize(event.get(name)));
            }
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
            java.util.TreeSet<String> keys = new java.util.TreeSet<>();
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
