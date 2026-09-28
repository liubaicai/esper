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
import com.espertech.esper.regressionlib.support.bean.SupportCallEvent;
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
 * Direct Esper 9.0.0 oracle for the pattern-followedby-timer-584 unit:
 * PatternOperatorFollowedBy ord 2 PatternFollowedByTimer — every-A
 * followed-by fanning persistent branches into a per-branch every-B leg
 * carrying TWO correlated filter predicates (dest equality plus the
 * closed range {@code startTime in [A.startTime:A.endTime]}), wrapped by
 * a {@code where timer:within (7200000)} pattern guard, plus the
 * statement-level {@code where B.source != A.source} post-filter outside
 * the pattern brackets. The deployment's EPL is pinned byte-exact:
 * {@code ]} is joined directly to the statement-level {@code where}
 * ({@code ]where}, no space) while {@code timer:within (7200000)} keeps
 * its literal space.
 *
 * The execution performs NO clock calls — no sendTimer, advanceTime or
 * milestone — so the runtime clock stays at epoch and the ~83-day
 * within guard never binds (the longest payload window is ~79s, and
 * startTime/endTime are bean fields, not engine time). The internal
 * timer is disabled for determinism like the regression suite's
 * deterministic runtime.
 *
 * Three SupportCallEvent sends carry RELATIVE millisecond offsets —
 * the Java source's dateToLong resolves the pinned datetimes via
 * SimpleDateFormat in the default timezone, so only the offsets are
 * timezone-stable: e1(callId 2000002601, source "18", 0/41200) arms an
 * every-A branch with no fire, e2(2000002607, "20", 24100/65400)
 * completes it — ONE record {A:e1,B:e2} — and e3(2000002610, "22",
 * 38100/78900) falls inside BOTH armed windows ([0:41200] and
 * [24100:65400]), delivering ONE listener invocation carrying TWO rows
 * [{A:e1,B:e3},{A:e2,B:e3}] — the Java assertion pins
 * getNewDataList().size()==1 AND getLastNewData().length==2. All three
 * dest values are "123456789014795" and all sources are distinct, so the
 * statement-level source-inequality filter is armed but never rejects.
 *
 * {@code select *} projects both bound tags as bean fragments (A and
 * B); the Java asserts assertSame identity on the underlying beans,
 * which this oracle renders as the full five-property SupportCallEvent
 * row shape (callId/source/dest/startTime/endTime).
 */
public final class PatternFollowedByTimer584ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-followedby-timer-584";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorFollowedBy.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportCallEvent.java"};

    private static final String DESCRIPTION =
            "PatternOperatorFollowedBy ord 2 PatternFollowedByTimer — `every A=SupportCallEvent -> "
                    + "every B=SupportCallEvent(dest=A.dest, startTime in [A.startTime:A.endTime]) where "
                    + "timer:within (7200000)` plus the statement-level `where B.source != A.source` "
                    + "post-filter (byte-exact: `)]where` has NO space, `timer:within (7200000)` keeps "
                    + "its space). Three SupportCallEvent sends carry RELATIVE millisecond offsets "
                    + "(Java's dateToLong is timezone-dependent): e1(callId 2000002601, source 18, "
                    + "0/41200) arms an every-A branch with no fire, e2(2000002607, source 20, "
                    + "24100/65400) completes it — one record {A:e1,B:e2} — and e3(2000002610, "
                    + "source 22, 38100/78900) falls inside BOTH armed windows ([0:41200] and "
                    + "[24100:65400]), delivering ONE listener invocation carrying TWO rows "
                    + "[{A:e1,B:e3},{A:e2,B:e3}] (Java pins getNewDataList().size()==1 and "
                    + "getLastNewData().length==2). No clock ops: the timer:within guard never binds. "
                    + "select * projects both the A and B bean fragments (assertSame identity in "
                    + "Java; the five-property SupportCallEvent row shape in the trace).";

    private static final String[] CASES = {"timer"};
    private static final int[] CASE_ORDINALS = {2};
    private static final int[] CASE_RUNTIME_INDEX = {0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-4759bc801b8c0be6c10a"};
    private static final String[] EXECUTIONS = {
            "PatternFollowedByTimer"};
    // Static-manifest id for the ord 2 execution (discovery
    // static-candidate). The deduplicated inventory id
    // java-089b2086c9945dff918f — shared by all ten
    // PatternOperatorFollowedBy rows — is NOT the static id here: where
    // a per-execution static id exists the per-runtime row carries it
    // (pattern-followedby-wharness-583 precedent).
    private static final String[] STATIC_IDS = {
            "java-b96c718a6895cd0d80de"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock ops (timer:within(7200000) never binds over the ~79s payload span); "
                    + "`every A -> every B(dest=A.dest, startTime in [A.startTime:A.endTime]) "
                    + "where timer:within (7200000)` + statement `where B.source != A.source`: e1 arms "
                    + "a branch, e2 delivers ONE record {A:e1,B:e2}, e3 falls inside both windows and "
                    + "delivers ONE invocation carrying TWO rows [{A:e1,B:e3},{A:e2,B:e3}] (Java pins "
                    + "getNewDataList().size()==1 and getLastNewData().length==2); select * projects "
                    + "both A and B fragments"};

    private static final String EPL_TIMER =
            "@name('s0') select * from pattern "
                    + "[every A=SupportCallEvent -> "
                    + "every B=SupportCallEvent(dest=A.dest, "
                    + "startTime in [A.startTime:A.endTime]) "
                    + "where timer:within (7200000)]"
                    + "where B.source != A.source";

    private static final String[] CASE_EPLS = {EPL_TIMER};

    // Pinned op sequence (after the case marker), in Java source order:
    // one s0 deploy, three SupportCallEvent sends, undeploy-all. No
    // clock ops — the execution performs no sendTimer/advanceTime calls.
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "undeploy-all"}
    };

    // Pinned send payloads, in send order per case:
    // "callId|source|dest|startTime|endTime" (relative ms offsets).
    private static final String[][] CASE_SENDS = {
            {"2000002601|18|123456789014795|0|41200",
                    "2000002607|20|123456789014795|24100|65400",
                    "2000002610|22|123456789014795|38100|78900"}
    };

    private static final int EXPECTED_STEPS = 6;
    private static final int EXPECTED_RECORDS = 2;

    private PatternFollowedByTimer584ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternFollowedByTimer584ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportCallEvent.class);

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
        if (!"SupportCallEvent".equals(eventType)) {
            throw new IllegalStateException("unknown eventType: " + eventType);
        }
        runtime.getEventService().sendEventBean(
                new SupportCallEvent(longInteger(payload, "callId"),
                        payload.getString("source", null),
                        payload.getString("dest", null),
                        longInteger(payload, "startTime"),
                        longInteger(payload, "endTime")),
                "SupportCallEvent");
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
     * pinned ops — deploy s0 with the verbatim EPL ("]where" no space,
     * "timer:within (7200000)" spaced), three five-field
     * SupportCallEvent sends and undeploy-all. Unknown step fields are
     * rejected. There are no advance-time steps because the Java
     * execution performs no clock calls.
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
                        if (!"SupportCallEvent".equals(eventType)) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        requireFields(payload, "callId", "source", "dest", "startTime", "endTime");
                        String actual = longInteger(payload, "callId") + "|"
                                + string(payload, "source") + "|"
                                + string(payload, "dest") + "|"
                                + longInteger(payload, "startTime") + "|"
                                + longInteger(payload, "endTime");
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
        long parsed = longInteger(object, name);
        if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) parsed;
    }

    private static long longInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof com.espertech.esper.common.client.json.minimaljson.JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
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
        if (value instanceof SupportCallEvent) {
            // Tagged-event columns arrive as the underlying bean; render
            // the full five-property bean exactly like the Go
            // NormalizeResults event rendering.
            SupportCallEvent bean = (SupportCallEvent) value;
            JsonObject fields = new JsonObject();
            fields.add("callId", normalize(bean.getCallId()));
            fields.add("dest", normalize(bean.getDest()));
            fields.add("endTime", normalize(bean.getEndTime()));
            fields.add("source", normalize(bean.getSource()));
            fields.add("startTime", normalize(bean.getStartTime()));
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
