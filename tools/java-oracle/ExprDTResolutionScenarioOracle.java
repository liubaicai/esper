import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeObjectArray;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportDateTime;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.TimeUnit;

/**
 * Java oracle for ExprDTResolution's two executions under both engine time
 * resolutions.  Four cases replay on fresh runtimes:
 *
 * event-time-ms / event-time-us (ordinal 0, ExprDTResolutionEventTime): the
 * object-array MyEvent(id,sts,ets) type declares sts/ets as the start/end
 * timestamp properties, so a.withDate(2002, 4, 30) rewrites the event's
 * interval bounds (start -> 2002-05-30 preserving time-of-day, end shifted by
 * the same delta) and before(b) is the strict leftEnd &lt; rightStart.  B@t
 * seeds #lastevent without evaluating (unidirectional), A@flip-1 fires,
 * A@flip does not — the boundary is exactly one engine time unit (1 ms vs
 * 1 µs), which is what proves the resolution.  Every send emits one trace
 * record: a listener row on fire, a listener-not-invoked count otherwise.
 *
 * long-property-ms / long-property-us (ordinal 1, ExprDTLongProperty): one
 * SupportDateTime send emits c0..c7.  Long-returning calendar ops preserve
 * the sub-millisecond µs remainder (c0 keeps +123), toCalendar()/toDate()
 * truncate to ms (c1/c5/c6 are byte-identical across resolutions), and
 * minus(1) is one MILLISECOND regardless of resolution (c7 = t-1 ms /
 * t*1000-1000 µs).
 *
 * The -us cases configure the microsecond time source via
 * configuration.getCommon().getTimeSource().setTimeUnit(MICROSECONDS),
 * mirroring TestSuiteExprDateTimeWConfig.testExprDTMicrosecondResolution.
 */
public final class ExprDTResolutionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-resolution";
    private static final String DESCRIPTION = "ExprDTResolution executions(isMicrosecond) under both engine time "
            + "resolutions: event-time-ms/us replay ExprDTResolutionEventTime (object-array MyEvent(id,sts,ets) "
            + "with sts/ets timestamp properties; a.withDate(2002,4,30).before(b) fires iff the shifted end is "
            + "strictly before b's start — the boundary is one engine unit, 1 ms vs 1 µs), long-property-ms/us "
            + "replay ExprDTLongProperty (c0..c7 over longdate and current_timestamp: µs long ops preserve the "
            + "sub-ms remainder, toCalendar/toDate truncate to ms, minus(1) is one millisecond in both modes).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/"
                    + "ExprDTResolution.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-28a96cc89bb103f3750a",
            "java-runtime-deee6eb6aef5c6bfa2a7",
            "java-runtime-35ef58e0d7badc88b6a8",
            "java-runtime-81d2223362062b76b0a5"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTResolutionEventTime",
            "ExprDTResolutionEventTime",
            "ExprDTLongProperty",
            "ExprDTLongProperty"
    };
    private static final String[] STATIC_IDS = {
            "java-f30ce9b974beba2c58e8",
            "java-f30ce9b974beba2c58e8",
            "java-55fcf32b326f5e318713",
            "java-55fcf32b326f5e318713"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "event-time-ms",
            "event-time-us",
            "long-property-ms",
            "long-property-us"
    };
    private static final int[] ORDINALS = {0, 0, 1, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+count; advanceTime(0), B@t seeds #lastevent without evaluating (unidirectional), "
            + "A@flip-1 fires the join row (a,b fragments), A@flip does not — the strict before boundary "
            + "is 1 ms",
            "listener+count; identical shape under the microsecond time source with all timestamps x1000: "
            + "A@flip-1µs fires, A@flip does not — the strict before boundary is 1 µs",
            "listener; advanceTime(t) then one SupportDateTime(longdate=t) send emits c0..c7 = "
            + "[calModMillis, calMod, 4, 4, 5, calTime, calTime, t-1]",
            "listener; advanceTime(t*1000) then SupportDateTime(longdate=t*1000+123) emits "
            + "c0=calModMillis*1000+123 (µs remainder preserved), c1/c5/c6 truncated to the same ms values "
            + "as the ms run, c7=t*1000-1000 (minus(1) is one millisecond)"
    };
    private static final String EVENT_TIME_EPL =
            "@name('s0') select * from MyEvent(id='A') as a unidirectional, "
            + "MyEvent(id='B')#lastevent as b where a.withDate(2002, 4, 30).before(b)";
    private static final String LONG_PROPERTY_EPL =
            "@name('s0') select "
            + "longdate.withTime(1, 2, 3, 4) as c0,"
            + "longdate.set('hour', 1).set('minute', 2).set('second', 3).set('millisecond', 4).toCalendar() as c1,"
            + "longdate.get('month') as c2,"
            + "current_timestamp.get('month') as c3,"
            + "current_timestamp.getMinuteOfHour() as c4,"
            + "current_timestamp.toDate() as c5,"
            + "current_timestamp.toCalendar() as c6,"
            + "current_timestamp.minus(1) as c7 "
            + "from SupportDateTime";
    private static final String[] CASE_EPLS = {
            EVENT_TIME_EPL,
            EVENT_TIME_EPL,
            LONG_PROPERTY_EPL,
            LONG_PROPERTY_EPL
    };

    private static final int EXPECTED_STEPS = 24;
    private static final int EXPECTED_RECORDS = 8;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|at.  Deploy steps
     * carry the byte-exact EPL text; send steps carry the compacted payload
     * including the expected flag; advance-time steps carry the RFC3339
     * instant.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("event-time-ms", new String[]{
                "advance-time|event-time-ms||||||1970-01-01T00:00:00Z",
                "deploy|event-time-ms|s0||" + EVENT_TIME_EPL + "|||",
                "send|event-time-ms||MyEvent||{\"id\":\"B\",\"sts\":1022749200000,\"ets\":1022749200000,\"expected\":false}||",
                "send|event-time-ms||MyEvent||{\"id\":\"A\",\"sts\":1022749199999,\"ets\":1022749199999,\"expected\":true}||",
                "send|event-time-ms||MyEvent||{\"id\":\"A\",\"sts\":1022749200000,\"ets\":1022749200000,\"expected\":false}||",
                "undeploy-all|event-time-ms||||||",
        });
        CASE_STEPS.put("event-time-us", new String[]{
                "advance-time|event-time-us||||||1970-01-01T00:00:00Z",
                "deploy|event-time-us|s0||" + EVENT_TIME_EPL + "|||",
                "send|event-time-us||MyEvent||{\"id\":\"B\",\"sts\":1022749200000000,\"ets\":1022749200000000,\"expected\":false}||",
                "send|event-time-us||MyEvent||{\"id\":\"A\",\"sts\":1022749199999999,\"ets\":1022749199999999,\"expected\":true}||",
                "send|event-time-us||MyEvent||{\"id\":\"A\",\"sts\":1022749200000000,\"ets\":1022749200000000,\"expected\":false}||",
                "undeploy-all|event-time-us||||||",
        });
        CASE_STEPS.put("long-property-ms", new String[]{
                "advance-time|long-property-ms||||||2002-05-30T09:05:06.007Z",
                "deploy|long-property-ms|s0||" + LONG_PROPERTY_EPL + "|||",
                "send|long-property-ms||SupportDateTime||{\"longdate\":1022749506007,\"expected\":true}||",
                "undeploy-all|long-property-ms||||||",
        });
        CASE_STEPS.put("long-property-us", new String[]{
                "advance-time|long-property-us||||||2002-05-30T09:05:06.007Z",
                "deploy|long-property-us|s0||" + LONG_PROPERTY_EPL + "|||",
                "send|long-property-us||SupportDateTime||{\"longdate\":1022749506007123,\"expected\":true}||",
                "undeploy-all|long-property-us||||||",
        });
    }

    private ExprDTResolutionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTResolutionScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays the case's steps on a fresh runtime.  The -us cases configure
     * the microsecond time source so advanceTime and every long timestamp
     * are engine µs units; advance-time steps carry the RFC3339 instant and
     * are scaled to engine units here.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        boolean microsecond = caseName.endsWith("-us");
        Configuration configuration = configure(caseName);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        try {
            ListenerRecorder listener = new ListenerRecorder(caseName, runtime, microsecond);
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "advance-time": {
                        long atMillis = Instant.parse(string(step, "at")).toEpochMilli();
                        runtime.getEventService().advanceTime(microsecond ? atMillis * 1000 : atMillis);
                        break;
                    }
                    case "deploy": {
                        runtime.getDeploymentService().undeployAll();
                        listener.reset();
                        String epl = string(step, "epl");
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, new CompilerArguments(configuration));
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        boolean found = false;
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(listener);
                                found = true;
                            }
                        }
                        if (!found) {
                            throw new IllegalStateException(
                                    "statement s0 was not deployed for " + string(step, "statement"));
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, listener, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        listener.reset();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Per-case event-type registration mirroring the suite: the event-time
     * cases register the object-array MyEvent(id,sts,ets) type with sts/ets
     * as the start/end timestamp properties (addIdStsEtsEvent); the
     * long-property cases register the SupportDateTime bean.  The -us cases
     * additionally select the microsecond time source.
     */
    private static Configuration configure(String caseName) {
        Configuration configuration = new Configuration();
        if (caseName.startsWith("event-time")) {
            ConfigurationCommonEventTypeObjectArray objectArray =
                    new ConfigurationCommonEventTypeObjectArray();
            objectArray.setStartTimestampPropertyName("sts");
            objectArray.setEndTimestampPropertyName("ets");
            configuration.getCommon().addEventType("MyEvent",
                    "id,sts,ets".split(","),
                    new Object[]{String.class, long.class, long.class},
                    objectArray);
        } else {
            configuration.getCommon().addEventType(SupportDateTime.class);
        }
        if (caseName.endsWith("-us")) {
            configuration.getCommon().getTimeSource().setTimeUnit(TimeUnit.MICROSECONDS);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    /**
     * Sends one event, drains the deliveries it produced, and emits exactly
     * one record: the listener row when the send fires or a
     * listener-not-invoked count when it does not — mirroring
     * assertListenerInvoked/assertListenerNotInvoked.  A delivery that
     * contradicts the pinned expected flag is a replay error.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        boolean expected = payload.getBoolean("expected", false);
        if ("MyEvent".equals(eventType)) {
            Object[] event = new Object[]{
                    string(payload, "id"),
                    longInteger(payload.get("sts"), "sts"),
                    longInteger(payload.get("ets"), "ets")};
            runtime.getEventService().sendEventObjectArray(event, eventType);
        } else if ("SupportDateTime".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportDateTime(longInteger(payload.get("longdate"), "longdate"),
                            null, null, null, null),
                    "SupportDateTime");
        } else {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
        List<JsonObject> delivered = listener.drain();
        if (expected) {
            if (delivered.size() != 1) {
                throw new IllegalStateException("expected one delivery for " + eventType
                        + ", got " + delivered.size());
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", listener.nextSequence());
            record.add("time", listener.currentTime());
            JsonArray newArray = new JsonArray();
            newArray.add(delivered.get(0));
            record.add("new", newArray);
            records.add(record);
            return;
        }
        if (!delivered.isEmpty()) {
            throw new IllegalStateException("unexpected delivery for " + eventType);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "count");
        record.add("statement", "s0");
        record.add("sequence", listener.nextSequence());
        record.add("time", listener.currentTime());
        record.add("name", "listener-not-invoked");
        record.add("count", 0);
        records.add(record);
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(fragmentOrValue(event, prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    /**
     * Reads a join fragment column as its EventBean when the property is a
     * stream fragment (select * over the unidirectional join exposes the a/b
     * events as fragments), falling back to the plain property value.
     */
    private static Object fragmentOrValue(EventBean event, String prop) {
        try {
            Object fragment = event.getFragment(prop);
            if (fragment != null) {
                return fragment;
            }
        } catch (RuntimeException ignored) {
            // Not a fragment property; read the plain value.
        }
        return event.get(prop);
    }

    /** Canonical cell rendering: instants collapse to epoch-millis numbers. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return renderRow((EventBean) value);
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
        if (value instanceof java.util.Date) {
            return Json.value(((java.util.Date) value).getTime());
        }
        if (value instanceof java.util.Calendar) {
            return Json.value(((java.util.Calendar) value).getTimeInMillis());
        }
        if (value instanceof java.time.LocalDateTime) {
            java.time.LocalDateTime ldt = (java.time.LocalDateTime) value;
            return Json.value(ldt.atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli());
        }
        if (value instanceof java.time.ZonedDateTime) {
            return Json.value(((java.time.ZonedDateTime) value).toInstant().toEpochMilli());
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object element : (Object[]) value) {
                array.add(normalize(element));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            JsonObject fields = new JsonObject();
            for (Object key : new TreeSet<>(((Map<?, ?>) value).keySet())) {
                fields.add(String.valueOf(key), normalize(((Map<?, ?>) value).get(key)));
            }
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            item.add("fields", fields);
            return item;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Buffers rendered rows per delivery so the send step can verify the
     * pinned expected flag before emitting the listener record, and renders
     * record times as instants regardless of the engine time unit.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final boolean microsecond;
        private final List<JsonObject> pending = new ArrayList<>();
        private long sequence;

        private ListenerRecorder(String caseName, EPRuntime runtime, boolean microsecond) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.microsecond = microsecond;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            for (EventBean event : newEvents) {
                pending.add(renderRow(event));
            }
        }

        private List<JsonObject> drain() {
            List<JsonObject> rows = new ArrayList<>(pending);
            pending.clear();
            return rows;
        }

        private long nextSequence() {
            return ++sequence;
        }

        private void reset() {
            pending.clear();
        }

        /** Engine current time rendered as an instant: µs runtimes divide. */
        private String currentTime() {
            long now = runtime.getEventService().getCurrentTime();
            return Instant.ofEpochMilli(microsecond ? now / 1000 : now).toString();
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|at with the payload
     * compacted.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload", "expectError", "at"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + string(step, "at");
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

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
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
