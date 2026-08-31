import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;

/** Direct Esper oracle for the pinned iterator-window rollup executions. */
public final class ResultSetQueryTypeRollupHavingIteratorScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-rollup-having-iterator";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java";
    private static final String DESCRIPTION = "ResultSetQueryTypeRollupHavingAndOrderBy ordinals 2-3: length-window rollup iterator snapshots with an optional keepall join.";
    private static final String EPL_PREFIX = "@Name('s0') select theString as c0, sum(intPrimitive) as c1 from SupportBean#length(3)";
    private static final String[] CASES = {"iterator-window-no-join", "iterator-window-join"};
    private static final int[] ORDINALS = {2, 3};
    private static final String[] RUNTIMES = {"java-runtime-1ce0d3fc4b6fed76d38d", "java-runtime-7730d794daee6dac92c7"};
    private static final String[] NAMES = {"ResultSetQueryTypeIteratorWindow{join=false}", "ResultSetQueryTypeIteratorWindow{join=true}"};
    private static final String STATIC_ID = "java-efb06d5c38363ea3a5d0";
    private static final String[] BEAN_STRINGS = {"E1", "E2", "E1", "E2"};
    private static final int[] BEAN_INTS = {1, 2, 3, 4};

    private ResultSetQueryTypeRollupHavingIteratorScenarioOracle() {}

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetQueryTypeRollupHavingIteratorScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validate(scenario);
        List<JsonObject> records = new ArrayList<>();
        JsonArray steps = scenario.get("steps").asArray();
        for (int i = 0; i < CASES.length; i++) {
            runCase(steps, records, i);
        }
        JsonArray out = new JsonArray();
        for (JsonObject record : records) {
            out.add(record);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", COMMIT).add("java", System.getProperty("java.version"))
                .add("records", out));
    }

    private static void validate(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !COMMIT.equals(string(scenario, "javaCommit"))
                || !SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario identity is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("expected two cases");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(cases.get(i), "case definition " + i);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[i].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[i]
                    || !RUNTIMES[i].equals(string(definition, "runtimeId"))
                    || !NAMES[i].equals(string(definition, "executionName"))
                    || !"iterator".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 4
                    || !epl(i == 1).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + i);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 20) {
            throw new IllegalArgumentException("expected twenty steps");
        }
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            int base = caseIndex * 10;
            JsonObject marker = object(steps.get(base), "case marker " + base);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker is not pinned at step " + base);
            }
            validateSeed(object(steps.get(base + 1), "seed step " + (base + 1)), base + 1);
            for (int sendIndex = 0; sendIndex < BEAN_STRINGS.length; sendIndex++) {
                int sendStep = base + 2 + sendIndex * 2;
                validateBeanSend(object(steps.get(sendStep), "send step " + sendStep), sendStep,
                        BEAN_STRINGS[sendIndex], BEAN_INTS[sendIndex]);
                int snapshotStep = sendStep + 1;
                JsonObject snapshot = object(steps.get(snapshotStep), "snapshot step " + snapshotStep);
                requireFields(snapshot, "op", "statement", "mode");
                if (!"snapshot".equals(string(snapshot, "op"))
                        || !"s0".equals(string(snapshot, "statement"))
                        || !"any".equals(string(snapshot, "mode"))) {
                    throw new IllegalArgumentException("snapshot is not pinned at step " + snapshotStep);
                }
            }
        }
    }

    private static void validateSeed(JsonObject step, int index) {
        requireFields(step, "op", "eventType", "payload");
        JsonObject payload = object(step.get("payload"), "seed payload " + index);
        requireFields(payload, "id");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean_S0".equals(string(step, "eventType"))
                || integer(payload, "id") != 1) {
            throw new IllegalArgumentException("seed payload is not pinned at step " + index);
        }
    }

    private static void validateBeanSend(JsonObject step, int index, String expectedString, int expectedInt) {
        requireFields(step, "op", "eventType", "payload");
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean".equals(string(step, "eventType"))
                || !expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static void runCase(JsonArray steps, List<JsonObject> records, int index) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetQueryTypeRollupHavingIteratorScenarioOracle-" + RUNTIMES[index], config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    epl(index == 1), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
            EPStatement statement = deployment.getStatements()[0];
            int sequence = 0;
            int snapshots = 0;
            int matchingMarkers = 0;
            boolean active = false;
            for (JsonValue value : steps) {
                JsonObject step = object(value, "step");
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    boolean matches = CASES[index].equals(string(step, "case"));
                    if (matches) {
                        if (++matchingMarkers != 1) {
                            throw new IllegalArgumentException("duplicate active case marker");
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
                    send(runtime, step);
                } else if ("snapshot".equals(operation)) {
                    if (!"s0".equals(string(step, "statement"))
                            || !"any".equals(string(step, "mode"))) {
                        throw new IllegalArgumentException("snapshot metadata is not pinned");
                    }
                    JsonArray rows = new JsonArray();
                    Iterator<EventBean> iterator = statement.iterator();
                    while (iterator.hasNext()) {
                        rows.add(row(iterator.next()));
                    }
                    records.add(new JsonObject().add("case", CASES[index])
                            .add("operation", "snapshot").add("statement", statement.getName())
                            .add("sequence", ++sequence)
                            .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                            .add("new", rows));
                    snapshots++;
                } else {
                    throw new IllegalArgumentException("unsupported operation: " + operation);
                }
            }
            if (matchingMarkers != 1 || snapshots != 4 || sequence != 4) {
                throw new IllegalStateException("case replay count mismatch");
            }
        } finally {
            runtime.destroy();
        }
    }

    private static String epl(boolean join) {
        return EPL_PREFIX + (join ? ", SupportBean_S0#keepall" : "") + " group by rollup(theString)";
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))) {
            throw new IllegalArgumentException("step is not a send");
        }
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        if ("SupportBean_S0".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S0(integer(payload, "id")), eventType);
            return;
        }
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalArgumentException("event type mismatch");
        }
        requireFields(payload, "theString", "intPrimitive");
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        runtime.getEventService().sendEventBean(bean, eventType);
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
        return Json.value(String.valueOf(value));
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
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return value.asInt();
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
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
}
