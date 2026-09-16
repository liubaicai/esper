import com.espertech.esper.common.client.EPCompiled;
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
 * Direct Esper 9.0.0 oracle for ResultSetAggregateFirstLastWindow ordinals
 * 0 (ResultSetAggregateStar), 2 (ResultSetAggregateUnboundedStream) and 20
 * (ResultSetAggregateLastMaxMixedOnSelect).  The star execution runs
 * compileDeploy then eplToModelCompileDeploy under one runtime id; the
 * compile path is not observable, so the oracle replays a single pass.
 * Suite milestones are persistence round-trips with no observable engine
 * output and stay out of the trace.
 */
public final class ResultSetAggregateFirstLastWindowStarScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-firstlastwindow-star";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstLastWindow.java";
    private static final String DESCRIPTION =
            "ResultSetAggregateFirstLastWindow ordinals 0/2/20: star-form first/last/window/firstever/lastever over a length-2 window, unbounded-stream first/last behaving as ever aggregates, and a keep-all named window observed by a constant-key grouped on-select emitting one aggregate row per B% trigger.";

    private static final String STAR = "star";
    private static final String UNBOUNDED = "unbounded-stream";
    private static final String ON_SELECT = "last-max-on-select";
    private static final String[] CASES = {STAR, UNBOUNDED, ON_SELECT};
    private static final int[] ORDINALS = {0, 2, 20};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-00be68da7736fcafb968",
            "java-runtime-da477a833deb82cf0225",
            "java-runtime-edb70b7eb3a9cd4217e8"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetAggregateStar",
            "ResultSetAggregateUnboundedStream",
            "ResultSetAggregateLastMaxMixedOnSelect"
    };
    private static final String[] STATIC_IDS = {
            "java-c867ed972dfaa61eaf41",
            "java-459d25127c6b2a8d35c2",
            "java-f1ef7fe95770df930d5f"
    };
    private static final String STAR_EPL =
            "@name('s0') select first(*) as firststar, first(sb.*) as firststarsb, last(*) as laststar, last(sb.*) as laststarsb, window(*) as windowstar, window(sb.*) as windowstarsb, firstever(*) as firsteverstar, lastever(*) as lasteverstar from SupportBean#length(2) as sb";
    private static final String UNBOUNDED_EPL =
            "@name('s0') select first(theString) as f1, first(sb.*) as f2, first(*) as f3, last(theString) as l1, last(sb.*) as l2, last(*) as l3 from SupportBean as sb";
    private static final String ON_SELECT_EPL =
            "create window MyWindowOne#keepall as SupportBean;\n" +
            "insert into MyWindowOne select * from SupportBean(theString like 'A%');\n" +
            "@name('s0') on SupportBean(theString like 'B%') select last(mw.intPrimitive) as li, max(mw.intPrimitive) as mi from MyWindowOne mw;";
    private static final String[] EPLS = {STAR_EPL, UNBOUNDED_EPL, ON_SELECT_EPL};

    private ResultSetAggregateFirstLastWindowStarScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateFirstLastWindowStarScenarioOracle <scenario.json>");
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
            runCase(steps, CASES[index], RUNTIME_IDS[index], EPLS[index], records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId, String epl,
                                JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replay(steps, caseName, runtime);
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

    private static void replay(JsonArray steps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = steps.get(index).asObject();
            String operation = step.getString("op", "");
            if ("case".equals(operation)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (active && "send".equals(operation)) {
                JsonObject payload = step.get("payload").asObject();
                SupportBean bean = new SupportBean(payload.getString("theString", null),
                        payload.getInt("intPrimitive", 0));
                if (payload.get("doublePrimitive") != null && !payload.get("doublePrimitive").isNull()) {
                    bean.setDoublePrimitive(payload.get("doublePrimitive").asDouble());
                }
                runtime.getEventService().sendEventBean(bean, "SupportBean");
            }
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
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    private static void validateSteps(JsonArray steps) {
        String[][] expectedStrings = {
                {"E1", "E2", "E3"},
                {"E1", "E2", "E3"},
                onSelectStrings()
        };
        int[][] expectedInts = {
                {10, 20, 30},
                {1, 2, 3},
                onSelectInts()
        };
        double[][] expectedDoubles = {
                null,
                {1.0, 2.0, 3.0},
                null
        };
        int expectedTotal = CASES.length;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            expectedTotal += expectedStrings[caseIndex].length;
        }
        if (steps.size() != expectedTotal) {
            throw new IllegalArgumentException("scenario must contain exactly " + expectedTotal + " steps");
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            for (int send = 0; send < expectedStrings[caseIndex].length; send++) {
                JsonObject step = object(steps.get(cursor), "send step " + cursor);
                requireFields(step, "op", "eventType", "payload");
                if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
                    throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                }
                JsonObject payload = object(step.get("payload"), "send step " + cursor + " payload");
                String[] payloadFields = expectedDoubles[caseIndex] == null
                        ? new String[]{"theString", "intPrimitive"}
                        : new String[]{"theString", "intPrimitive", "doublePrimitive"};
                requireFields(payload, payloadFields);
                if (!expectedStrings[caseIndex][send].equals(string(payload, "theString"))
                        || integer(payload, "intPrimitive") != expectedInts[caseIndex][send]) {
                    throw new IllegalArgumentException("send step " + cursor + " payload is not pinned");
                }
                if (expectedDoubles[caseIndex] != null
                        && doubleNumber(payload, "doublePrimitive") != expectedDoubles[caseIndex][send]) {
                    throw new IllegalArgumentException("send step " + cursor + " doublePrimitive is not pinned");
                }
                cursor++;
            }
        }
    }

    private static String[] onSelectStrings() {
        String[] strings = new String[24];
        int cursor = 0;
        strings[cursor++] = "A1";
        strings[cursor++] = "B1";
        for (int i = 11; i < 20; i++) {
            strings[cursor++] = "A1";
            strings[cursor++] = "Bx";
        }
        strings[cursor++] = "A1";
        strings[cursor++] = "B1";
        strings[cursor++] = "A1";
        strings[cursor] = "B1";
        return strings;
    }

    private static int[] onSelectInts() {
        int[] ints = new int[24];
        int cursor = 0;
        ints[cursor++] = 10;
        ints[cursor++] = -1;
        for (int i = 11; i < 20; i++) {
            ints[cursor++] = i;
            ints[cursor++] = -1;
        }
        ints[cursor++] = 1;
        ints[cursor++] = -1;
        ints[cursor++] = 2;
        ints[cursor] = -1;
        return ints;
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

    private static double doubleNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON number");
        }
        return value.asDouble();
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

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
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
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean[] events) {
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    array.add(normalize(event));
                }
                return array;
            }
            if (value instanceof EventBean event) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
            }
            if (value instanceof SupportBean bean) {
                JsonObject fields = new JsonObject();
                fields.add("bigDecimal", normalize(bean.getBigDecimal()));
                fields.add("bigInteger", normalize(bean.getBigInteger()));
                fields.add("boolBoxed", normalize(bean.getBoolBoxed()));
                fields.add("boolPrimitive", normalize(bean.isBoolPrimitive()));
                fields.add("byteBoxed", normalize(bean.getByteBoxed()));
                fields.add("bytePrimitive", normalize(bean.getBytePrimitive()));
                fields.add("charBoxed", normalize(bean.getCharBoxed()));
                fields.add("charPrimitive", normalize(bean.getCharPrimitive()));
                fields.add("doubleBoxed", normalize(bean.getDoubleBoxed()));
                fields.add("doublePrimitive", normalize(bean.getDoublePrimitive()));
                fields.add("enumValue", normalize(bean.getEnumValue()));
                fields.add("floatBoxed", normalize(bean.getFloatBoxed()));
                fields.add("floatPrimitive", normalize(bean.getFloatPrimitive()));
                fields.add("intBoxed", normalize(bean.getIntBoxed()));
                fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
                fields.add("longBoxed", normalize(bean.getLongBoxed()));
                fields.add("longPrimitive", normalize(bean.getLongPrimitive()));
                fields.add("shortBoxed", normalize(bean.getShortBoxed()));
                fields.add("shortPrimitive", normalize(bean.getShortPrimitive()));
                fields.add("theString", normalize(bean.getTheString()));
                return new JsonObject().add("kind", "row").add("fields", fields);
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
    }
}
