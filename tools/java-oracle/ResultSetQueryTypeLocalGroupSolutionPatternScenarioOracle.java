import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinal 12,
 * ResultSetLocalGroupedSolutionPattern: the grouped solution-pattern ratio
 * {@code count(*) / count(*, group_by:())} over a 30-second time window with an
 * output snapshot every 10 seconds, driven entirely by virtual time.
 *
 * <p>The Java execution advances to zero, compiles and deploys one statement
 * named s0, attaches the listener, sends six {@code new SupportBean(theString,
 * 0)} per batch (A,B,C,B,B,C / A,B,B,B,B,A / C,A,A,A,B,A), advances by 10000ms
 * after each batch and undeploys.  Because {@code output snapshot every 10
 * seconds} is the only output policy and the internal timer is disabled, the
 * listener fires exactly once per advance - never for a send - and each
 * callback carries exactly three new rows and no old rows.
 *
 * <p>The window is {@code #time(30 sec)} and expires events strictly before
 * {@code current - delta + 1}, so the advance to 30000 expires the six events
 * sent at zero while the events sent at 10000 and 20000 remain.  The first
 * snapshot therefore divides by six, and the second and third by twelve; the
 * second batch sends no C, so the second snapshot keeps C at two of twelve.
 *
 * <p>The Java execution asserts with
 * {@code assertPropsPerRowLastNewAnyOrder}, so the row order inside a snapshot
 * is not a contract.  The oracle matches each batch as a multiset against the
 * values the Java assertion pins and records it in the canonical ascending
 * {@code theString} order (A, B, C) so both sides pin the same order.
 *
 * <p>The ratio is Esper's default floating-point division
 * ({@code MathArithTypeEnum.DivideDouble}), so {@code pct} is a Java
 * {@code Double} and the trace emits it as a JSON number using the shortest
 * round-trip spelling of {@code Double.toString}.  Nothing in this execution
 * carries a remove stream, so no record has an {@code old} key.
 */
public final class ResultSetQueryTypeLocalGroupSolutionPatternScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-solution-pattern";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinal 12: the grouped solution-pattern ratio, count(*) divided by"
                    + " the statement-wide count(*, group_by:()) over a 30-second time window with an output snapshot"
                    + " every 10 seconds, driven entirely by virtual time (0/10/20/30s) so the final boundary expires"
                    + " the batch sent at zero and the third snapshot's denominator is 12.";
    private static final String CASE = "grouped-solution-pattern";
    private static final int ORDINAL = 12;
    private static final String RUNTIME = "java-runtime-ae96db5ed464e562e6d7";
    private static final String NAME = "ResultSetLocalGroupedSolutionPattern";
    private static final String OBSERVATION = "listener";
    private static final int ITERATOR_SNAPSHOTS = 0;
    private static final String STATIC_ID = "java-13f0da7834ee65870fb0";
    private static final String EPL =
            "@name('s0') select theString, count(*) / count(*, group_by:()) as pct"
                    + " from SupportBean#time(30 sec)"
                    + " group by theString"
                    + " output snapshot every 10 seconds";
    private static final String[] STEP_OPS = {
            "case", "advance-time",
            "send", "send", "send", "send", "send", "send", "advance-time",
            "send", "send", "send", "send", "send", "send", "advance-time",
            "send", "send", "send", "send", "send", "send", "advance-time"
    };
    private static final String[] ADVANCE_ATS = {
            "1970-01-01T00:00:00Z", "1970-01-01T00:00:10Z",
            "1970-01-01T00:00:20Z", "1970-01-01T00:00:30Z"
    };
    private static final String[] SEND_STRINGS = {
            "A", "B", "C", "B", "B", "C",
            "A", "B", "B", "B", "B", "A",
            "C", "A", "A", "A", "B", "A"
    };
    private static final int CASE_SENDS = 18;
    private static final int CASE_ADVANCES = 4;
    private static final int CASE_RECORDS = 3;
    // Result metadata of the one statement, sorted by the runtime.
    private static final String[] RESULT_FIELDS = {"pct", "theString"};
    // The canonical ascending-theString row order of every snapshot, and the
    // values the Java assertPropsPerRowLastNewAnyOrder calls pin.  The second
    // snapshot divides A/B/C by twelve because the second batch sends no C, so
    // C stays at two events.
    private static final String[] SNAPSHOT_ROWS = {"A", "B", "C"};
    private static final double[][] SNAPSHOT_PCT = {
            {1 / 6d, 3 / 6d, 2 / 6d},
            {3 / 12d, 7 / 12d, 2 / 12d},
            {6 / 12d, 5 / 12d, 1 / 12d}
    };

    private ResultSetQueryTypeLocalGroupSolutionPatternScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupSolutionPatternScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIME, configuration);
        runtime.getEventService().advanceTime(0L);

        JsonArray records = new JsonArray();
        try {
            EPStatement statement = deploy(runtime);
            ListenerWriter writer = new ListenerWriter(records, statement, runtime);
            statement.addListener(writer);

            JsonArray steps = scenario.get("steps").asArray();
            boolean active = false;
            int markers = 0;
            int sends = 0;
            int advances = 0;
            for (int stepIndex = 0; stepIndex < steps.size(); stepIndex++) {
                JsonObject step = object(steps.get(stepIndex), "step " + stepIndex);
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    if (CASE.equals(string(step, "case"))) {
                        if (++markers != 1) {
                            throw new IllegalStateException("duplicate active case marker");
                        }
                        active = true;
                    } else if (active) {
                        active = false;
                    }
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("send".equals(operation)) {
                    if (sends >= CASE_SENDS) {
                        throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                    }
                    SupportBean bean = bean(step, stepIndex, SEND_STRINGS[sends]);
                    sends++;
                    runtime.getEventService().sendEventBean(bean, "SupportBean");
                } else if ("advance-time".equals(operation)) {
                    advances++;
                    advanceTime(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASE);
            }
            if (sends != CASE_SENDS || advances != CASE_ADVANCES) {
                throw new IllegalStateException("expected " + CASE_SENDS + " sends and " + CASE_ADVANCES
                        + " advance-time steps for " + CASE + ", got " + sends + " and " + advances);
            }
            if (writer.sequence != CASE_RECORDS) {
                throw new IllegalStateException("expected " + CASE_RECORDS + " listener records for " + CASE
                        + ", got " + writer.sequence);
            }
            if (records.size() != CASE_RECORDS) {
                throw new IllegalStateException("expected " + CASE_RECORDS + " trace records, got " + records.size());
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static EPStatement deploy(EPRuntime runtime) throws Exception {
        EPDeployment deployment = runtime.getDeploymentService().deploy(
                EPCompilerProvider.getCompiler().compile(EPL, new CompilerArguments(runtime.getRuntimePath())),
                new DeploymentOptions().setDeploymentId(ID + "-" + CASE));
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static SupportBean bean(JsonObject step, int stepIndex, String expectedString) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != 0
                || longNumber(payload, "longPrimitive") != 0L) {
            throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
        }
        SupportBean bean = new SupportBean(expectedString, 0);
        bean.setLongPrimitive(0L);
        return bean;
    }

    private static void advanceTime(EPRuntime runtime, JsonObject step) {
        requireFields(step, "op", "at");
        runtime.getEventService().advanceTime(Instant.parse(string(step, "at")).toEpochMilli());
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{NAME}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != 1) {
            throw new IllegalArgumentException("expected exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition 0");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(string(definition, "case"))
                || integer(definition, "ordinal") != ORDINAL
                || !RUNTIME.equals(string(definition, "runtimeId"))
                || !NAME.equals(string(definition, "executionName"))
                || !OBSERVATION.equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly " + STEP_OPS.length + " steps");
        }
        int sendIndex = 0;
        int advanceIndex = 0;
        for (int index = 0; index < STEP_OPS.length; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if (!STEP_OPS[index].equals(operation)) {
                throw new IllegalArgumentException("step " + index + " operation is " + operation + ", want "
                        + STEP_OPS[index]);
            }
            if ("case".equals(operation)) {
                requireFields(step, "op", "case");
                if (!CASE.equals(string(step, "case"))) {
                    throw new IllegalArgumentException("case marker is not pinned at step " + index);
                }
            } else if ("advance-time".equals(operation)) {
                requireFields(step, "op", "at");
                if (advanceIndex >= ADVANCE_ATS.length || !ADVANCE_ATS[advanceIndex].equals(string(step, "at"))) {
                    throw new IllegalArgumentException("advance-time step is not pinned at step " + index);
                }
                advanceIndex++;
            } else {
                validateSend(step, index, SEND_STRINGS[sendIndex]);
                sendIndex++;
            }
        }
        if (sendIndex != SEND_STRINGS.length || advanceIndex != ADVANCE_ATS.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, String expectedString) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != 0
                || longNumber(payload, "longPrimitive") != 0L) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static JsonObject row(EventBean event) {
        String[] properties = event.getEventType().getPropertyNames().clone();
        Arrays.sort(properties);
        JsonObject fields = new JsonObject();
        for (String property : properties) {
            fields.add(property, normalize(event.get(property)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray output = new JsonArray();
        for (EventBean event : events) {
            output.add(row(event));
        }
        return output;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (com.espertech.esper.common.client.json.minimaljson.Member member : value.asObject()) {
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
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
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

    private static final class ListenerWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int sequence;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            try {
                if (next > CASE_RECORDS) {
                    throw new IllegalStateException("unexpected listener callback " + next + " for " + CASE);
                }
                if (newEvents == null || newEvents.length != SNAPSHOT_ROWS.length) {
                    throw new IllegalStateException(CASE + " callback " + next + " must carry exactly "
                            + SNAPSHOT_ROWS.length + " new rows, got "
                            + (newEvents == null ? "no new data" : Integer.toString(newEvents.length)));
                }
                long now = runtime.getEventService().getCurrentTime();
                long expectedTime = next * 10000L;
                if (now != expectedTime) {
                    throw new IllegalStateException(CASE + " callback " + next + " is at " + now
                            + ", want " + expectedTime);
                }
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(CASE + " callback " + next + " delivered " + oldEvents.length
                            + " old rows");
                }
                assertSnapshotBatch(newEvents, next);
                sequence = next;
                records.add(new JsonObject().add("case", CASE).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString())
                        .add("new", rows(canonical(newEvents))));
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        /**
         * Mirrors the Java assertPropsPerRowLastNewAnyOrder of the execution:
         * every row carries the pinned result metadata and the batch contains
         * exactly the pinned (theString, pct) pairs in any order.
         */
        private void assertSnapshotBatch(EventBean[] batch, int snapshot) {
            for (EventBean row : batch) {
                String[] names = row.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, RESULT_FIELDS)) {
                    throw new IllegalStateException(CASE + " snapshot " + snapshot
                            + " result field metadata is not pinned: " + Arrays.toString(names));
                }
            }
            boolean[] matched = new boolean[batch.length];
            for (int index = 0; index < SNAPSHOT_ROWS.length; index++) {
                String wantString = SNAPSHOT_ROWS[index];
                double wantPct = SNAPSHOT_PCT[snapshot - 1][index];
                boolean found = false;
                for (int candidate = 0; candidate < batch.length; candidate++) {
                    if (!matched[candidate] && matchesRow(batch[candidate], wantString, wantPct)) {
                        matched[candidate] = true;
                        found = true;
                        break;
                    }
                }
                if (!found) {
                    throw new IllegalStateException(CASE + " snapshot " + snapshot + " has no row for "
                            + wantString + " = " + wantPct);
                }
            }
        }

        private static boolean matchesRow(EventBean row, String wantString, double wantPct) {
            Object value = row.get("pct");
            if (!(value instanceof Double)) {
                throw new IllegalStateException("pct = " + value + " ("
                        + (value == null ? "null" : value.getClass().getSimpleName()) + "), want Double");
            }
            return wantString.equals(row.get("theString"))
                    && Double.compare(((Double) value).doubleValue(), wantPct) == 0;
        }

        /**
         * The Java execution matches each snapshot in any order, so the trace
         * keeps the canonical ascending-theString order (A, B, C).
         */
        private static EventBean[] canonical(EventBean[] batch) {
            EventBean[] ordered = batch.clone();
            Arrays.sort(ordered, (left, right) -> ((String) left.get("theString"))
                    .compareTo((String) right.get("theString")));
            return ordered;
        }
    }
}
