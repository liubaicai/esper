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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowPerGroup ordinals 1-8
 * (the ResultAssertExecution virtual-time cluster): grouped sum(price) over
 * SupportMarketDataBean#time(5.5 sec).  The "none" scenario (ords 1-4) has no
 * output clause (per-event immediate output); the "default" scenario (ords
 * 5-8) uses output every 1 seconds.  The no-having twins add order by symbol
 * asc; the having twins add having sum(price)>50.  Mirroring
 * ResultAssertExecution, each case runs the EPL twice — the plain istream
 * select then the select irstream twin — over the shared ResultAssertInput
 * schedule, bracketed by deploy/undeploy-all steps.  SupportBean is the shared
 * regression bean; SupportMarketDataBean is a map type because the
 * regression-lib jar is not on the oracle classpath.  The oracle dispatches on
 * the scenario id to the none or default case table; both share the same
 * replay loop and TraceWriter (which skips null/null listener callbacks).
 */
public final class ResultSetOutputLimitRowPerGroupNoneDefaultScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String NONE_ID = "resultset-output-limit-row-per-group-none";
    private static final String DEFAULT_ID = "resultset-output-limit-row-per-group-default";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java";

    private static final String NONE_DESCRIPTION =
            "ResultSetOutputLimitRowPerGroup ordinals 1-4 (ResultAssertExecution virtual-time cluster): grouped sum(price) over SupportMarketDataBean#time(5.5 sec) with no output clause (per-event immediate output). The no-having twins add order by symbol asc; the having twins add having sum(price)>50. Each case runs the EPL twice (plain istream then select irstream) over the shared ResultAssertInput schedule.";
    private static final String DEFAULT_DESCRIPTION =
            "ResultSetOutputLimitRowPerGroup ordinals 5-8 (ResultAssertExecution virtual-time cluster): grouped sum(price) over SupportMarketDataBean#time(5.5 sec) under output every 1 seconds. The no-having twins add order by symbol asc; the having twins add having sum(price)>50 (no order-by). Each case runs the EPL twice (plain istream then select irstream) over the shared ResultAssertInput schedule.";

    private static final String[] NONE_CASES = {
            "none-no-having-no-join", "none-no-having-join", "none-having-no-join", "none-having-join"};
    private static final String[] DEFAULT_CASES = {
            "default-no-having-no-join", "default-no-having-join", "default-having-no-join", "default-having-join"};
    private static final int[] NONE_ORDINALS = {1, 2, 3, 4};
    private static final int[] DEFAULT_ORDINALS = {5, 6, 7, 8};
    private static final String[] NONE_RUNTIME_IDS = {
            "java-runtime-047b01d4e6e6e73101c3",
            "java-runtime-f6dc738e9e1219068421",
            "java-runtime-815886be170eed7aee20",
            "java-runtime-7c4e54256aa53004caa5"
    };
    private static final String[] DEFAULT_RUNTIME_IDS = {
            "java-runtime-b6a1986826894ec12fd1",
            "java-runtime-66eefdbac64b7ab79a9b",
            "java-runtime-3df0124bc42a5e107a8c",
            "java-runtime-56e8b086b5eaf29a9494"
    };
    private static final String[] NONE_EXECUTIONS = {
            "ResultSet1NoneNoHavingNoJoin",
            "ResultSet2NoneNoHavingJoin",
            "ResultSet3NoneHavingNoJoin",
            "ResultSet4NoneHavingJoin"
    };
    private static final String[] DEFAULT_EXECUTIONS = {
            "ResultSet5DefaultNoHavingNoJoin",
            "ResultSet6DefaultNoHavingJoin",
            "ResultSet7DefaultHavingNoJoin",
            "ResultSet8DefaultHavingJoin"
    };
    private static final String[] NONE_STATIC_IDS = {
            "java-17d44b04969031bdfa95",
            "java-3b39afd80726954a1e0c",
            "java-21aef3310b0a2d8dc7a2",
            "java-151b7aae6ccfb43d2917"
    };
    private static final String[] DEFAULT_STATIC_IDS = {
            "java-999488ed531009909b94",
            "java-62a8bd394618820c193e",
            "java-335b33ab42a2888b6cc8",
            "java-20b789466c2490afee52"
    };
    private static final String[] NONE_BASE_EPLS = {
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec)group by symbol order by symbol asc",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol order by symbol asc",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol  having sum(price) > 50",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol having sum(price) > 50"
    };
    private static final String[] DEFAULT_BASE_EPLS = {
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol output every 1 seconds order by symbol asc",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol output every 1 seconds order by symbol asc",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec) \ngroup by symbol having sum(price) > 50output every 1 seconds",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol having sum(price) > 50output every 1 seconds"
    };
    private static final int DEPLOYS_PER_CASE = 2;
    private static final int SENDS_PER_CASE = 24;
    private static final int ADVANCES_PER_CASE = 51;

    private ResultSetOutputLimitRowPerGroupNoneDefaultScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowPerGroupNoneDefaultScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        Spec spec = specFor(scenario);
        validateScenario(scenario, spec);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < spec.cases.length; index++) {
            runCase(steps, spec, spec.cases[index], spec.runtimeIds[index], index, records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", spec.id)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static final class Spec {
        final String id;
        final String description;
        final String[] cases;
        final int[] ordinals;
        final String[] runtimeIds;
        final String[] executions;
        final String[] staticIds;
        final String[] baseEpls;

        Spec(String id, String description, String[] cases, int[] ordinals, String[] runtimeIds,
             String[] executions, String[] staticIds, String[] baseEpls) {
            this.id = id;
            this.description = description;
            this.cases = cases;
            this.ordinals = ordinals;
            this.runtimeIds = runtimeIds;
            this.executions = executions;
            this.staticIds = staticIds;
            this.baseEpls = baseEpls;
        }
    }

    private static Spec specFor(JsonObject scenario) {
        String id = scenario.getString("id", "");
        if (NONE_ID.equals(id)) {
            return new Spec(NONE_ID, NONE_DESCRIPTION, NONE_CASES, NONE_ORDINALS, NONE_RUNTIME_IDS,
                    NONE_EXECUTIONS, NONE_STATIC_IDS, NONE_BASE_EPLS);
        }
        if (DEFAULT_ID.equals(id)) {
            return new Spec(DEFAULT_ID, DEFAULT_DESCRIPTION, DEFAULT_CASES, DEFAULT_ORDINALS,
                    DEFAULT_RUNTIME_IDS, DEFAULT_EXECUTIONS, DEFAULT_STATIC_IDS, DEFAULT_BASE_EPLS);
        }
        throw new IllegalArgumentException("unsupported scenario id: " + id);
    }

    private static void runCase(JsonArray steps, Spec spec, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        marketType.put("volume", Long.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);

        String runtimeURI = "parity-" + spec.id + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
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
                if ("advance-time".equals(operation)) {
                    long at = Instant.parse(step.getString("at", "")).toEpochMilli();
                    runtime.getEventService().advanceTime(at);
                } else if ("deploy".equals(operation)) {
                    String epl = spec.baseEpls[caseIndex];
                    if (deployIndex == 1) {
                        epl = epl.replace("select ", "select irstream ");
                    }
                    deployIndex++;
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(spec.id + "-" + caseIndex + "-" + deployIndex));
                    findStatement(deployment).addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
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
            SupportBean bean = new SupportBean();
            bean.setTheString(payload.getString("theString", null));
            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            event.put("price", payload.getDouble("price", 0));
            event.put("volume", payload.get("volume") == null || payload.get("volume").isNull()
                    ? null : payload.get("volume").asLong());
            event.put("feed", payload.get("feed") == null || payload.get("feed").isNull()
                    ? null : payload.get("feed").asString());
            runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario, Spec spec) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !spec.id.equals(string(scenario, "id"))
                || !spec.description.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), spec.runtimeIds, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), spec.executions, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), spec.staticIds, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != spec.cases.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        for (int index = 0; index < spec.cases.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!spec.cases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != spec.ordinals[index]
                    || !spec.runtimeIds[index].equals(string(definition, "runtimeId"))
                    || !spec.executions[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || !spec.baseEpls[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps, spec);
    }

    private static void validateSteps(JsonArray steps, Spec spec) {
        int expectedTotal = spec.cases.length;
        for (int index = 0; index < spec.cases.length; index++) {
            expectedTotal += ADVANCES_PER_CASE + SENDS_PER_CASE + DEPLOYS_PER_CASE + DEPLOYS_PER_CASE;
        }
        if (steps.size() != expectedTotal) {
            throw new IllegalArgumentException("scenario must contain exactly " + expectedTotal + " steps");
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < spec.cases.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            if (!"case".equals(string(marker, "op")) || !spec.cases[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            int advances = 0;
            int deploys = 0;
            int undeploys = 0;
            while (cursor < steps.size()) {
                JsonObject step = steps.get(cursor).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    break;
                }
                if ("advance-time".equals(operation)) {
                    advances++;
                } else if ("send".equals(operation)) {
                    sends++;
                } else if ("deploy".equals(operation)) {
                    deploys++;
                } else if ("undeploy-all".equals(operation)) {
                    undeploys++;
                } else {
                    throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != SENDS_PER_CASE || advances != ADVANCES_PER_CASE
                    || deploys != DEPLOYS_PER_CASE || undeploys != DEPLOYS_PER_CASE) {
                throw new IllegalArgumentException("case " + spec.cases[caseIndex] + " step counts are not pinned");
            }
        }
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
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
