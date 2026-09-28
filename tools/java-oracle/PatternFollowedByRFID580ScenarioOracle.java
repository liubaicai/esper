import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportRFIDEvent;
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
 * Direct Esper 9.0.0 oracle for the pattern-followedby-rfid-580 bundle:
 * PatternOperatorFollowedBy ords 3, 4 and 5 — the RFID trio —
 * every-then followed-by with a per-branch correlated {@code and not}
 * terminator over SupportRFIDEvent, replayed as three cases each against
 * a fresh runtime like the regression suite's per-execution environment.
 *
 * memory-rfid (PatternMemoryRFIDEvent, ord 3) deploys
 * {@code every tagMayBeBroken=SupportRFIDEvent -> (timer:interval(10 sec)
 * and not SupportRFIDEvent(mac=tagMayBeBroken.mac))} and sends ten
 * identical ("a","111") pairs: each repeat cancels the pending same-mac
 * branch before the 10-second timer can elapse (no clock ops exist), so
 * the missing-heartbeat alert NEVER fires — the case contributes ZERO
 * listener records. Silence is the whole assertion: the Java execution
 * performs no assertListenerNotInvoked at all, it simply relies on the
 * listener counter staying empty.
 *
 * zone-exit (PatternRFIDZoneExit, ord 4) deploys {@code every
 * a=SupportRFIDEvent(zoneID='1') -> (b=SupportRFIDEvent(mac=a.mac,
 * zoneID!='1') and not SupportRFIDEvent(mac=a.mac,zoneID='1'))}: (a,1)
 * arms silently, (a,2) fires, (b,1) arms, the duplicate (b,1) cancels
 * AND re-arms only its own mac branch (assertListenerNotInvoked on both
 * (b,1) sends), (b,2) fires — 2 rows.
 *
 * zone-enter (PatternRFIDZoneEnter, ord 5) deploys {@code every
 * a=SupportRFIDEvent(zoneID!='1') -> (b=SupportRFIDEvent(mac=a.mac,
 * zoneID='1') and not SupportRFIDEvent(mac=a.mac,zoneID=a.zoneID))} —
 * the not-conjunct zoneID is the captured a.zoneID tag value, not a
 * constant: (a,2) arms, (a,1) fires, (b,2) arms, the repeat (b,2)
 * cancels the (b,2)-armed entry watch (the discriminant — a constant '1'
 * not-branch would not cancel here), (b,1) fires — 2 rows.
 *
 * {@code select *} on ords 4/5 projects every bound tag as a fragment
 * property, so each delivered row carries BOTH the {@code a} and
 * {@code b} SupportRFIDEvent beans (the Java assertions pin the
 * b-tagged row's identity via assertEqualsNew(s0,b,theEvent)). All
 * env.milestone savepoints are harness splits restoring identical state
 * and are unrepresented. SupportRFIDEvent is the real regression-lib
 * bean: the two-argument constructor sends (mac, zoneID) with
 * locationReportId null, rendered as the null marker in the trace.
 */
public final class PatternFollowedByRFID580ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-followedby-rfid-580";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportRFIDEvent.java"};

    private static final String DESCRIPTION =
            "PatternOperatorFollowedBy ords 3, 4 and 5 — the RFID trio: every-then followed-by "
                    + "with a per-branch correlated `and not` terminator over "
                    + "SupportRFIDEvent(mac,zoneID). memory-rfid (PatternMemoryRFIDEvent, ord 3) "
                    + "sends ten identical (a,111) pairs: each repeat cancels the pending same-mac "
                    + "branch before timer:interval(10 sec) can elapse, so the missing-heartbeat "
                    + "alert NEVER fires — zero listener records. zone-exit (PatternRFIDZoneExit, "
                    + "ord 4) arms an exit watch on zone-1 reports: (a,1) arms silently, (a,2) "
                    + "fires, (b,1) arms, the duplicate (b,1) cancels and re-arms only its own mac "
                    + "branch, (b,2) fires — 2 rows with per-mac branch isolation. zone-enter "
                    + "(PatternRFIDZoneEnter, ord 5) mirrors it with the not-conjunct keyed on "
                    + "TagField(a,zoneID) rather than a constant: (a,2) arms, (a,1) fires, (b,2) "
                    + "arms, the repeat (b,2) cancels the (b,2)-armed watch, (b,1) fires — 2 rows. "
                    + "select * on ords 4/5 projects every bound tag — both the a and b "
                    + "SupportRFIDEvent beans per Java fragment semantics. All milestone() "
                    + "savepoints are harness splits restoring identical state and are "
                    + "unrepresented; no clock ops exist (the 10-second timer never elapses).";

    private static final String[] CASES = {
            "memory-rfid", "zone-exit", "zone-enter"};
    private static final int[] CASE_ORDINALS = {3, 4, 5};
    private static final int[] CASE_RUNTIME_INDEX = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-a477964502f64fe1b368",
            "java-runtime-f8ac45e337f93e276ac3",
            "java-runtime-6a045f5ae813471b32e8"};
    private static final String[] EXECUTIONS = {
            "PatternMemoryRFIDEvent",
            "PatternRFIDZoneExit",
            "PatternRFIDZoneEnter"};
    // Deduplicated inventory id: all ten PatternOperatorFollowedBy
    // inventory rows share static id java-089b2086c9945dff918f (NOT the
    // every-distinct file's java-073091a0b42ca8dd974f), pinned once per
    // runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-089b2086c9945dff918f",
            "java-089b2086c9945dff918f",
            "java-089b2086c9945dff918f"};
    private static final String[] OBSERVATIONS = {
            "listener never invoked; no clock ops; `every tagMayBeBroken -> "
                    + "(timer:interval(10 sec) and not "
                    + "SupportRFIDEvent(mac=tagMayBeBroken.mac))`: ten identical (a,111) pairs — "
                    + "each repeat cancels the pending same-mac branch before the timer elapses — "
                    + "ZERO listener records",
            "listener; no clock; `every a=(zoneID='1') -> (b=(mac=a.mac,zoneID!='1') and not "
                    + "(mac=a.mac,zoneID='1'))`: (a,1) arms silently, (a,2) fires, (b,1) arms, "
                    + "duplicate (b,1) cancels and re-arms only its own mac branch, (b,2) fires — "
                    + "2 rows, select * delivers both a and b RFID fragments",
            "listener; no clock; `every a=(zoneID!='1') -> (b=(mac=a.mac,zoneID='1') and not "
                    + "(mac=a.mac,zoneID=a.zoneID))` — the not-conjunct is TagField(a,zoneID), "
                    + "not a constant: (a,2) arms, (a,1) fires, (b,2) arms, repeat (b,2) cancels "
                    + "the armed watch, (b,1) fires — 2 rows, select * delivers both a and b "
                    + "RFID fragments"};

    private static final String EPL_MEMORY_RFID =
            "@name('s0') select 'Tag May Be Broken' as alert, tagMayBeBroken.mac, "
                    + "tagMayBeBroken.zoneID from pattern [every tagMayBeBroken=SupportRFIDEvent "
                    + "-> (timer:interval(10 sec) and not "
                    + "SupportRFIDEvent(mac=tagMayBeBroken.mac))]";
    private static final String EPL_ZONE_EXIT =
            "@name('s0') select * from pattern [every a=SupportRFIDEvent(zoneID='1') -> "
                    + "(b=SupportRFIDEvent(mac=a.mac,zoneID!='1') and not "
                    + "SupportRFIDEvent(mac=a.mac,zoneID='1'))]";
    private static final String EPL_ZONE_ENTER =
            "@name('s0') select * from pattern [every a=SupportRFIDEvent(zoneID!='1') -> "
                    + "(b=SupportRFIDEvent(mac=a.mac,zoneID='1') and not "
                    + "SupportRFIDEvent(mac=a.mac,zoneID=a.zoneID))]";

    private static final String[] CASE_EPLS = {
            EPL_MEMORY_RFID, EPL_ZONE_EXIT, EPL_ZONE_ENTER};

    // Pinned op sequences (after the case marker), in Java source order.
    // env.milestone savepoints are regression-harness splits restoring
    // identical state and carry no scenario op (ord 4 after sends 1/3,
    // ord 5 after sends 1/4). No advance-time ops exist: the ord 3 timer
    // is never given time to elapse.
    private static final String[][] CASE_OPS = {
            // memory-rfid: ten identical ("a","111") pairs — 20 sends.
            {"deploy",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send",
                    "undeploy-all"},
            // zone-exit: (a,1) silent, (a,2) fires, (b,1) silent,
            // (b,1) silent dup-cancel, (b,2) fires.
            {"deploy",
                    "send", "send", "send", "send", "send",
                    "undeploy-all"},
            // zone-enter: (a,2) silent, (a,1) fires, (b,2) silent,
            // (b,2) silent repeat-cancel, (b,1) fires.
            {"deploy",
                    "send", "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads, in send order: "SupportRFIDEvent|mac|zoneID".
    private static final String[][] CASE_SENDS = {
            {"SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111",
                    "SupportRFIDEvent|a|111", "SupportRFIDEvent|a|111"},
            {"SupportRFIDEvent|a|1", "SupportRFIDEvent|a|2",
                    "SupportRFIDEvent|b|1", "SupportRFIDEvent|b|1",
                    "SupportRFIDEvent|b|2"},
            {"SupportRFIDEvent|a|2", "SupportRFIDEvent|a|1",
                    "SupportRFIDEvent|b|2", "SupportRFIDEvent|b|2",
                    "SupportRFIDEvent|b|1"}
    };

    private static final int EXPECTED_STEPS = 39;
    private static final int EXPECTED_RECORDS = 4;

    private PatternFollowedByRFID580ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternFollowedByRFID580ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportRFIDEvent.class);

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
        if ("SupportRFIDEvent".equals(eventType)) {
            // Two-argument constructor like the regression executions:
            // locationReportId stays null.
            runtime.getEventService().sendEventBean(
                    new SupportRFIDEvent(payload.getString("mac", null),
                            payload.getString("zoneID", null)),
                    "SupportRFIDEvent");
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
     * pinned ops — deploy s0 with the verbatim EPL, sends with pinned
     * payloads and undeploy-all. Unknown step fields are rejected.
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
                        if (!"SupportRFIDEvent".equals(eventType)) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        requireFields(payload, "mac", "zoneID");
                        String actual = eventType + "|"
                                + string(payload, "mac") + "|"
                                + string(payload, "zoneID");
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
            if (sends != CASE_SENDS[caseIndex].length || deploys != 1) {
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
        if (value instanceof SupportRFIDEvent) {
            // Tagged-event columns arrive as the underlying bean; render
            // the pinned three-field projection exactly like the Go
            // NormalizeResults event rendering (locationReportId is null
            // under the two-argument constructor -> the null marker).
            SupportRFIDEvent bean = (SupportRFIDEvent) value;
            JsonObject fields = new JsonObject();
            fields.add("locationReportId", normalize(bean.getLocationReportId()));
            fields.add("mac", normalize(bean.getMac()));
            fields.add("zoneID", normalize(bean.getZoneID()));
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
