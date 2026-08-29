import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregateMinMax ordinals 2 and 3.
 *
 * <p>Each case uses a fresh runtime and mirrors the pinned setup: a public
 * length-two named window receives SupportBean inserts, then a min/max query
 * observes the current and ever aggregates. The SODA case compiles the same
 * query through the pinned eplToModel/serialization-copy path; listener
 * semantics are intentionally shared by both representations.</p>
 */
public final class ResultSetAggregateMinMaxNamedWindowWEverScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-minmax-named-window-wever";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMinMax.java";
    private static final String STATIC_ID = "java-09115f6e876ce88a42f9";
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7d0a94525b0038b397fd",
            "java-runtime-840f1ca5610dd00d3ec0"
    };
    private static final String[] CASES = {
            "named-window-wever-soda-false",
            "named-window-wever-soda-true"
    };
    private static final int[] ORDINALS = {2, 3};
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateMinMaxNamedWindowWEver{soda=false}",
            "ResultSetAggregateMinMaxNamedWindowWEver{soda=true}"
    };
    private static final String[] FLAGS = {"EXCLUDEWHENINSTRUMENTED"};
    private static final String DESCRIPTION =
            "ResultSetAggregateMinMax ordinals 2-3 NamedWindowWEver: public length(2) named window min/max current plus minever/maxever; four SupportBean sends produce new-only rows and length eviction changes current extrema while ever values retain the first event.";
    private static final String SETUP_EPL =
            "@public create window NamedWindow5m#length(2) as select * from SupportBean;\n" +
                    "insert into NamedWindow5m select * from SupportBean;";
    private static final String QUERY_EPL =
            "@name('s0') select min(intPrimitive) as lower, " +
                    "max(intPrimitive) as upper, " +
                    "minever(intPrimitive) as lowerever, " +
                    "maxever(intPrimitive) as upperever from NamedWindow5m";
    private static final int[][] EXPECTED = {
            {1, 1, 1, 1},
            {1, 5, 1, 5},
            {3, 5, 1, 5},
            {3, 6, 1, 6}
    };
    private static final String[] SORTED_FIELDS = {"lower", "lowerever", "upper", "upperever"};

    private ResultSetAggregateMinMaxNamedWindowWEverScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetAggregateMinMaxNamedWindowWEverScenarioOracle <scenario.json>");
            System.exit(2);
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
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }
        if (records.size() != 8) {
            throw new IllegalStateException("expected eight listener records, got " + records.size());
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", ID)
                .add("javaCommit", COMMIT)
                .add("java", System.getProperty("java.version"))
                .add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !COMMIT.equals(string(scenario, "javaCommit"))
                || !SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), FLAGS, "javaFlags");

        JsonArray caseDefinitions = array(scenario.get("cases"), "cases");
        if (caseDefinitions.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation", "epl");
            if (!CASES[i].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[i].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || !QUERY_EPL.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != CASES.length * 5) {
            throw new IllegalArgumentException("scenario must contain exactly ten steps");
        }
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            int offset = caseIndex * 5;
            JsonObject marker = object(steps.get(offset), "case marker " + caseIndex);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker mismatch at index " + offset);
            }
            validateBeanSend(steps, offset + 1, 1);
            validateBeanSend(steps, offset + 2, 5);
            validateBeanSend(steps, offset + 3, 3);
            validateBeanSend(steps, offset + 4, 6);
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, int expectedValue) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "payload at step " + index);
        requireFields(payload, "theString", "intPrimitive");
        JsonValue text = payload.get("theString");
        if (text == null || !text.isNull()) {
            throw new IllegalArgumentException("step " + index + " theString must be JSON null");
        }
        if (integer(payload, "intPrimitive") != expectedValue) {
            throw new IllegalArgumentException("step " + index + " intPrimitive mismatch");
        }
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled setup = EPCompilerProvider.getCompiler().compile(
                    SETUP_EPL, new CompilerArguments(runtime.getRuntimePath()));
            runtime.getDeploymentService().deploy(setup,
                    new DeploymentOptions().setDeploymentId(runtimeURI + "-setup"));

            EPCompiled query;
            if (caseIndex == 1) {
                EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(QUERY_EPL, configuration);
                model = SerializableObjectCopier.copyMayFail(model);
                Module module = new Module();
                module.getItems().add(new ModuleItem(model));
                module.setModuleText(model.toEPL());
                query = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                query = EPCompilerProvider.getCompiler().compile(
                        QUERY_EPL, new CompilerArguments(runtime.getRuntimePath()));
            }
            EPDeployment queryDeployment = runtime.getDeploymentService().deploy(query,
                    new DeploymentOptions().setDeploymentId(runtimeURI + "-query"));
            EPStatement statement = findStatement(queryDeployment);
            TraceWriter writer = new TraceWriter(records, CASES[caseIndex], statement, runtime);
            statement.addListener(writer);

            replayCase(steps, caseIndex, runtime);
            if (writer.sequence != EXPECTED.length) {
                throw new IllegalStateException("case " + CASES[caseIndex]
                        + " expected four callbacks, got " + writer.sequence);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replayCase(JsonArray steps, int caseIndex, EPRuntime runtime) {
        int offset = caseIndex * 5;
        for (int i = 1; i <= 4; i++) {
            JsonObject step = steps.get(offset + i).asObject();
            JsonObject payload = step.get("payload").asObject();
            Map<String, Object> event = new HashMap<>();
            event.put("theString", null);
            event.put("intPrimitive", integer(payload, "intPrimitive"));
            runtime.getEventService().sendEventMap(event, "SupportBean");
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private int sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            if (newEvents == null || newEvents.length != 1
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("callback must contain exactly one new-only row");
            }
            if (sequence >= EXPECTED.length) {
                throw new IllegalStateException("unexpected callback after four events");
            }
            int[] expected = EXPECTED[sequence];
            EventBean event = newEvents[0];
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, SORTED_FIELDS)) {
                throw new IllegalStateException("field metadata mismatch: " + Arrays.toString(names));
            }
            if (!same(event.get("lower"), expected[0])
                    || !same(event.get("upper"), expected[1])
                    || !same(event.get("lowerever"), expected[2])
                    || !same(event.get("upperever"), expected[3])) {
                throw new IllegalStateException("callback values mismatch at sequence " + (sequence + 1));
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("callback time must remain at epoch, got " + now);
            }
            sequence++;
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", rows(newEvents));
            records.add(record);
        }

        private boolean same(Object actual, int expected) {
            return actual instanceof Number
                    && Double.isFinite(((Number) actual).doubleValue())
                    && ((Number) actual).doubleValue() == expected;
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, SORTED_FIELDS)) {
                    throw new IllegalStateException("field metadata mismatch while rendering row");
                }
                JsonObject fields = new JsonObject();
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
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
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
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            double number = value.asDouble();
            long integral = value.asLong();
            if (!Double.isFinite(number) || number != integral
                    || integral < Integer.MIN_VALUE || integral > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            }
            return (int) integral;
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        for (int i = 0; i < expected.length; i++) {
            JsonValue item = actual.get(i);
            if (item == null || !item.isString() || !expected[i].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
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
}
