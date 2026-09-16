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
 * Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowPerGroup ordinals 9-12
 * (the ResultAssertExecution virtual-time cluster): grouped sum(price) over
 * SupportMarketDataBean#time(5.5 sec) under output all every 1s.  The
 * no-having twins add order by symbol; the having twins add having
 * sum(price)>50 across the three output-limit-opt hint variants (no order-by —
 * the ENABLE hint is a compile error with order-by).  Mirroring
 * ResultAssertExecution, each variant runs the EPL twice — the plain istream
 * select then the select irstream twin — over the shared ResultAssertInput
 * schedule, bracketed by deploy/undeploy-all steps.  SupportBean is the shared
 * regression bean; SupportMarketDataBean is a map type because the
 * regression-lib jar is not on the oracle classpath.  OutputConditionTime
 * force-dispatches a listener callback even for a null/null pair at quiet
 * boundaries; like the sibling oracles this trace skips those empty
 * callbacks and only numbers payload-carrying ones.
 */
public final class ResultSetOutputLimitRowPerGroupAllScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-per-group-all";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowPerGroup ordinals 9-12 (ResultAssertExecution virtual-time cluster): grouped sum(price) over SupportMarketDataBean#time(5.5 sec) under output all every 1s. The no-having twins add order by symbol; the having twins add having sum(price)>50 across the three output-limit-opt hint variants (no order-by — the ENABLE hint is a compile error with order-by). Each variant runs the EPL twice (plain istream then select irstream) over the shared ResultAssertInput schedule.";

    private static final String ALL_NOJOIN = "all-no-having-no-join";
    private static final String ALL_JOIN = "all-no-having-join";
    private static final String HAVING_NOJOIN = "all-having-no-join";
    private static final String HAVING_JOIN = "all-having-join";
    private static final String[] CASES = {ALL_NOJOIN, ALL_JOIN, HAVING_NOJOIN, HAVING_JOIN};
    private static final int[] ORDINALS = {9, 10, 11, 12};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-3cb45ffbbce3a1039f72",
            "java-runtime-2cfe8200a666f421582d",
            "java-runtime-2a425da1594958f77a88",
            "java-runtime-073783d29500f840e025"
    };
    private static final String[] EXECUTIONS = {
            "ResultSet9AllNoHavingNoJoin",
            "ResultSet10AllNoHavingJoin",
            "ResultSet11AllHavingNoJoin",
            "ResultSet12AllHavingJoin"
    };
    private static final String[] STATIC_IDS = {
            "java-e4b9e4ed53a43cd360b1",
            "java-c2f74c2589bf0414d654",
            "java-62c24905b76ff3fc0c5d",
            "java-826ccfb25fb656367980"
    };
    private static final String[] BASE_EPLS = {
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol output all every 1 seconds order by symbol",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol output all every 1 seconds order by symbol",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol having sum(price) > 50 output all every 1 seconds",
            "@name('s0') select symbol, sum(price) from SupportMarketDataBean#time(5.5 sec), SupportBean#keepall where theString=symbol group by symbol having sum(price) > 50 output all every 1 seconds"
    };
    private static final String[] HINTS = {
            "",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')"
    };
    private static final int[] HINT_VARIANTS_PER_CASE = {1, 1, 3, 3};
    private static final int[] DEPLOYS_PER_CASE = {2, 2, 6, 6};
    private static final int[] SENDS_PER_CASE = {24, 24, 72, 72};
    private static final int[] ADVANCES_PER_CASE = {51, 51, 153, 153};

    private ResultSetOutputLimitRowPerGroupAllScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowPerGroupAllScenarioOracle <scenario.json>");
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
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId, int caseIndex,
                                JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        marketType.put("volume", Long.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
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
                    String epl = eplFor(caseIndex, deployIndex);
                    deployIndex++;
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
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

    // eplFor maps the deploy index to the hint x istream variant: for the
    // having twins deployIndex/2 selects the hint and deployIndex%2 the
    // istream twin; for the no-having cases deployIndex is the istream twin
    // directly (no hint variants).
    private static String eplFor(int caseIndex, int deployIndex) {
        int hintIndex = deployIndex / 2;
        boolean irstream = deployIndex % 2 == 1;
        if (HINT_VARIANTS_PER_CASE[caseIndex] == 1) {
            hintIndex = 0;
            irstream = deployIndex == 1;
        }
        String epl = HINTS[hintIndex] + BASE_EPLS[caseIndex];
        if (irstream) {
            epl = epl.replace("select ", "select irstream ");
        }
        return epl;
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
            throw new IllegalArgumentException("scenario must contain exactly four cases");
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
                    || !BASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    private static void validateSteps(JsonArray steps) {
        int expectedTotal = CASES.length;
        for (int index = 0; index < CASES.length; index++) {
            expectedTotal += ADVANCES_PER_CASE[index] + SENDS_PER_CASE[index]
                    + DEPLOYS_PER_CASE[index] + DEPLOYS_PER_CASE[index];
        }
        if (steps.size() != expectedTotal) {
            throw new IllegalArgumentException("scenario must contain exactly " + expectedTotal + " steps");
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
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
            if (sends != SENDS_PER_CASE[caseIndex] || advances != ADVANCES_PER_CASE[caseIndex]
                    || deploys != DEPLOYS_PER_CASE[caseIndex] || undeploys != DEPLOYS_PER_CASE[caseIndex]) {
                throw new IllegalArgumentException("case " + CASES[caseIndex] + " step counts are not pinned");
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
            // OutputConditionTime force-dispatches the listener callback even
            // for a null/null pair at quiet boundaries; the parity trace
            // convention records only payload-carrying callbacks, matching
            // the sibling output-last/first/snapshot oracles.
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
