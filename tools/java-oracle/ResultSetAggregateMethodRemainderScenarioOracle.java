import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.EventBean;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.441
 * 'resultset-aggregate-method-remainder' unit: ResultSetAggregateRate
 * ordinal 0 (ResultSetAggregateRateDataNonWindowed, constant rate(10) over
 * virtual-time ever-points), ResultSetAggregateRate ordinal 1
 * (ResultSetAggregateRateDataWindowed, timestamp-property
 * RATE(longPrimitive)/RATE(longPrimitive, intPrimitive) over #length(3)) and
 * ResultSetAggregateLeaving ordinal 0 (sticky leaving() over #length(3)).
 * Each Java execution is one scenario case inside its own runtime.
 * ResultSetAggregateRateDataNonWindowed and ResultSetAggregateLeaving each
 * run their assertion twice (compileDeploy plus eplToModelCompileDeploy);
 * the scenario replays the assertion once because the second pass only
 * changes the compile path, not listener output.
 *
 * The rate-ever case is virtual-time driven: the Java execution calls
 * env.advanceTime (an absolute setTime on the external timer) before each
 * send, so the scenario carries an advance-time step before every send and
 * the oracle applies it through EPEventService.advanceTime.  The
 * rate-windowed case takes its timestamps from the longPrimitive event
 * property and never advances the clock; the leaving case sends
 * SupportBean("E", n) events and records the sticky boolean.
 *
 * All projected columns are scalars (Double rate values, Boolean leaving
 * flag), so the only normalization rule is the shared null marker: a null
 * column value emits {"state":"null"}.
 */
public final class ResultSetAggregateMethodRemainderScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-method-remainder";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateRate.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateLeaving.java";
    private static final String DESCRIPTION =
            "ResultSetAggregateRate ordinals 0 and 1 (constant rate(10) over virtual-time ever-points; timestamp-property RATE(longPrimitive)/RATE(longPrimitive, intPrimitive) over length(3)) and ResultSetAggregateLeaving ordinal 0 (sticky leaving() flag over length(3))";

    private static final String[] CASES = {
            "rate-ever", "rate-windowed", "leaving"};
    private static final int[] ORDINALS = {0, 1, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d0424628aa4aad2d80bc",
            "java-runtime-b90c3df2e444e89e8cb6",
            "java-runtime-276a60a1f8e8298c32ec"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetAggregateRateDataNonWindowed",
            "ResultSetAggregateRateDataWindowed",
            "ResultSetAggregateLeaving"
    };
    private static final String[] STATIC_IDS = {
            "java-5d1774a76d730ac199fb",
            "java-5d1774a76d730ac199fb",
            "java-f24caa115a51453bcad3"
    };
    private static final String[] OBSERVATIONS = {
            "listener; Java runs the assertion twice (compileDeploy + eplToModelCompileDeploy), one scenario pass covers both",
            "listener",
            "listener; Java runs the assertion twice (compileDeploy + eplToModelCompileDeploy), one scenario pass covers both"
    };

    // ResultSetAggregateRateDataNonWindowed EPL, byte-exact from
    // ResultSetAggregateRate.
    private static final String RATE_EVER_EPL =
            "@name('s0') select rate(10) as myrate from SupportBean";

    // ResultSetAggregateRateDataWindowed EPL, byte-exact from
    // ResultSetAggregateRate: uppercase RATE, single space after the
    // "as myrate," comma and after the intPrimitive parameter comma.
    private static final String RATE_WINDOWED_EPL =
            "@name('s0') select RATE(longPrimitive) as myrate, RATE(longPrimitive, intPrimitive) as myqtyrate from SupportBean#length(3)";

    // ResultSetAggregateLeaving EPL, byte-exact.
    private static final String LEAVING_EPL =
            "@name('s0') select leaving() as val from SupportBean#length(3)";

    private ResultSetAggregateMethodRemainderScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateMethodRemainderScenarioOracle <scenario.json>");
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

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
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
                if ("deploy".equals(operation)) {
                    String statement = step.getString("statement", "");
                    if (!"s0".equals(statement)) {
                        throw new IllegalArgumentException("unexpected deploy statement " + statement);
                    }
                    deployIndex++;
                    String epl = s0Epl(caseName);
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
                    EPStatement s0 = findStatement(deployment);
                    if (s0 == null) {
                        throw new IllegalStateException("deployment has no statement named s0");
                    }
                    s0.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("advance-time".equals(operation)) {
                    // env.advanceTime is an absolute setTime on the external
                    // timer, so the step's instant is applied verbatim.
                    runtime.getEventService().advanceTime(
                            Instant.parse(step.getString("at", "")).toEpochMilli());
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String s0Epl(String caseName) {
        switch (caseName) {
            case "rate-ever":
                return RATE_EVER_EPL;
            case "rate-windowed":
                return RATE_WINDOWED_EPL;
            case "leaving":
                return LEAVING_EPL;
            default:
                throw new IllegalArgumentException("unexpected case " + caseName);
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        return null;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setTheString(payload.getString("theString", null));
            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
            bean.setLongPrimitive(payload.getLong("longPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
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
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !s0Epl(CASES[index]).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
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

        private long nextSequence() {
            return ++sequence;
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
                    .add("sequence", nextSequence())
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
    }
}
