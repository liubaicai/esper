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
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetQueryTypeRowForAllHaving ordinal 1. */
public final class ResultSetQueryTypeRowForAllHavingSumJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-row-for-all-having-sum-join";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAllHaving.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeRowForAllHaving ordinal 1: time-window join sum with having threshold and listener expiry.";
    private static final String RUNTIME_ID = "java-runtime-dfb97881f562ac33e873";
    private static final String EXECUTION_NAME = "ResultSetQueryTypeRowForAllWHavingSumJoin";
    private static final String STATIC_ID = "java-bf6637bf00f9e0e78f89";
    private static final String CASE = "sum-join";
    private static final String EPL =
            "@name('s0') select irstream sum(longBoxed) as mySum from SupportBeanString#time(10 seconds) as one, SupportBean#time(10 seconds) as two where one.theString = two.theString having sum(longBoxed) > 10";
    private static final String[] SORTED_FIELDS = {"mySum"};

    private ResultSetQueryTypeRowForAllHavingSumJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetQueryTypeRowForAllHavingSumJoinScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBeanString", LocalSupportBeanString.class);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        String runtimeURI = "parity-" + ID + "-" + RUNTIME_ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        JsonArray records = new JsonArray();
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime);
            statement.addListener(writer);
            replay(scenario.get("steps").asArray(), runtime);
            if (writer.sequence != 3) {
                throw new IllegalStateException("expected three listener records, got " + writer.sequence);
            }
        } finally {
            runtime.destroy();
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static void replay(JsonArray steps, EPRuntime runtime) {
        for (JsonValue value : steps) {
            JsonObject step = value.asObject();
            String op = string(step, "op");
            if ("case".equals(op)) {
                continue;
            }
            if ("advance-time".equals(op)) {
                runtime.getEventService().advanceTime(Instant.parse(string(step, "at")).toEpochMilli());
                continue;
            }
            JsonObject payload = object(step.get("payload"), "payload");
            String eventType = string(step, "eventType");
            if ("SupportBeanString".equals(eventType)) {
                requireFields(payload, "theString");
                runtime.getEventService().sendEventBean(
                        new LocalSupportBeanString(string(payload, "theString")), eventType);
            } else if ("SupportBean".equals(eventType)) {
                requireFields(payload, "theString", "intBoxed", "shortBoxed", "longBoxed");
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setIntBoxed(integer(payload, "intBoxed"));
                bean.setShortBoxed((short) integer(payload, "shortBoxed"));
                bean.setLongBoxed(longNumber(payload, "longBoxed"));
                runtime.getEventService().sendEventBean(bean, eventType);
            } else {
                throw new IllegalArgumentException("unexpected event type");
            }
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version")) || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME_ID}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{EXECUTION_NAME}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(string(definition, "case")) || integer(definition, "ordinal") != 1
                || !RUNTIME_ID.equals(string(definition, "runtimeId"))
                || !EXECUTION_NAME.equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 9) {
            throw new IllegalArgumentException("scenario must contain exactly nine steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }
        validateAdvanceTime(steps, 1, "1970-01-01T00:00:00Z");
        validateStringSend(steps, 2);
        validateBeanSend(steps, 3, 10L);
        validateAdvanceTime(steps, 4, "1970-01-01T00:00:05Z");
        validateBeanSend(steps, 5, 15L);
        validateAdvanceTime(steps, 6, "1970-01-01T00:00:08Z");
        validateBeanSend(steps, 7, -5L);
        validateAdvanceTime(steps, 8, "1970-01-01T00:00:10Z");
    }

    private static void validateAdvanceTime(JsonArray steps, int index, String expected) {
        JsonObject step = object(steps.get(index), "advance-time step " + index);
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op")) || !expected.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step " + index + " is not pinned");
        }
    }

    private static void validateStringSend(JsonArray steps, int index) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBeanString".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step " + index + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload");
        requireFields(payload, "theString");
        if (!"KEY".equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("payload step " + index + " is not pinned");
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, long expected) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step " + index + " is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload");
        requireFields(payload, "theString", "intBoxed", "shortBoxed", "longBoxed");
        if (!"KEY".equals(string(payload, "theString"))
                || integer(payload, "intBoxed") != 0
                || integer(payload, "shortBoxed") != 0
                || longNumber(payload, "longBoxed") != expected) {
            throw new IllegalArgumentException("payload step " + index + " is not pinned");
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
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return value.asLong();
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
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
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > 3) {
                throw new IllegalStateException("unexpected listener callback after three records");
            }
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (next == 1 && (!hasNew || newEvents.length != 1 || hasOld)
                    || next == 2 && (!hasNew || newEvents.length != 1 || !hasOld || oldEvents.length != 1)
                    || next == 3 && (hasNew || !hasOld || oldEvents.length != 1)) {
                throw new IllegalStateException("unexpected new/old shape at sequence " + next);
            }
            String expectedTime = next == 1 ? "1970-01-01T00:00:05Z"
                    : next == 2 ? "1970-01-01T00:00:08Z" : "1970-01-01T00:00:10Z";
            long now = runtime.getEventService().getCurrentTime();
            if (!expectedTime.equals(Instant.ofEpochMilli(now).toString())) {
                throw new IllegalStateException("unexpected callback time at sequence " + next);
            }
            long[] expectedNew = {25L, 20L, 0L};
            long[] expectedOld = {0L, 25L, 20L};
            if (hasNew && sum(newEvents) != expectedNew[next - 1]
                    || hasOld && sum(oldEvents) != expectedOld[next - 1]) {
                throw new IllegalStateException("unexpected callback value at sequence " + next);
            }
            sequence = next;
            JsonObject record = new JsonObject().add("case", CASE).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString());
            if (hasNew) {
                record.add("new", rows(newEvents));
            }
            if (hasOld) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private long sum(EventBean[] events) {
            if (events.length != 1) {
                throw new IllegalStateException("expected one aggregate row");
            }
            String[] names = events[0].getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, SORTED_FIELDS)) {
                throw new IllegalStateException("aggregate field metadata is not pinned");
            }
            Object value = events[0].get("mySum");
            if (!(value instanceof Number)) {
                throw new IllegalStateException("mySum is not numeric");
            }
            return ((Number) value).longValue();
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray rows = new JsonArray();
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, SORTED_FIELDS)) {
                    throw new IllegalStateException("aggregate field metadata is not pinned");
                }
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                rows.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return rows;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }

    /** Local mirror of the pinned SupportBeanString regression bean. */
    public static class LocalSupportBeanString {
        private String theString;

        public LocalSupportBeanString(String theString) {
            this.theString = theString;
        }

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }
    }
}
