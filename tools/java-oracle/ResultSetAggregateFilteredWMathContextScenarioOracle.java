import com.espertech.esper.regressionlib.support.bean.SupportBeanNumeric;
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
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.math.MathContext;
import java.math.RoundingMode;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Set;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for ResultSetAggregateFilteredWMathContext. */
public final class ResultSetAggregateFilteredWMathContextScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-filtered-w-math-context";
    private static final String DESCRIPTION =
            "ResultSetAggregateFilteredWMathContext ordinal 0: compiler MathContext precision 2 HALF_UP rounds an unbounded BigDecimal average; scale-discarding inputs 0, 0, and 1 produce new-only listener rows 0, 0, and 0.33.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilteredWMathContext.java";
    private static final String RUNTIME_ID = "java-runtime-fa8b6d5d6fb58a905f23";
    private static final String STATIC_ID = "java-aba2cfbf41a3be9809f1";
    private static final String EXECUTION_NAME = "ResultSetAggregateFilteredWMathContext";
    private static final String CASE = ID;
    private static final String EPL = "@name('s0') select avg(bigdec) as c0 from SupportBeanNumeric";
    private static final String[] INPUT_VALUES = {"0", "0", "1"};
    private static final String[] EXPECTED_VALUES = {"0", "0", "0.33"};

    private ResultSetAggregateFilteredWMathContextScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateFilteredWMathContextScenarioOracle <scenario.json>");
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
        configuration.getCompiler().getExpression().setMathContext(new MathContext(2, RoundingMode.HALF_UP));
        configuration.getCommon().addEventType(SupportBeanNumeric.class);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        JsonArray records = new JsonArray();
        try {
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL, compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime);
            statement.addListener(writer);
            replay(scenario.get("steps").asArray(), runtime);
            if (writer.sequence != EXPECTED_VALUES.length) {
                throw new IllegalStateException("expected three listener records, got " + writer.sequence);
            }
            runtime.getDeploymentService().undeployAll();
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
        boolean foundCase = false;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                foundCase = true;
                continue;
            }
            if (!foundCase || !"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation at step " + index);
            }
            requireFields(step, "op", "eventType", "payload");
            if (!"SupportBeanNumeric".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("unexpected event type at step " + index);
            }
            JsonObject payload = object(step.get("payload"), "payload step " + index);
            requireFields(payload, "bigint", "bigdec");
            JsonValue bigint = payload.get("bigint");
            if (bigint == null || !bigint.isNull()) {
                throw new IllegalArgumentException("bigint must be JSON null at step " + index);
            }
            String bigdec = string(payload, "bigdec");
            runtime.getEventService().sendEventBean(new SupportBeanNumeric(null, new BigDecimal(bigdec)), "SupportBeanNumeric");
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
        if (!CASE.equals(string(definition, "case"))
                || integer(definition, "ordinal") != 0
                || !RUNTIME_ID.equals(string(definition, "runtimeId"))
                || !EXECUTION_NAME.equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPL.equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_VALUES.length + 1) {
            throw new IllegalArgumentException("scenario must contain exactly four steps");
        }
        JsonObject marker = object(steps.get(0), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned");
        }
        for (int index = 0; index < EXPECTED_VALUES.length; index++) {
            JsonObject step = object(steps.get(index + 1), "send step " + (index + 1));
            requireFields(step, "op", "eventType", "payload");
            if (!"send".equals(string(step, "op"))
                    || !"SupportBeanNumeric".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("send step " + (index + 1) + " is not pinned");
            }
            JsonObject payload = object(step.get("payload"), "payload step " + (index + 1));
            if (!payload.get("bigint").isNull() || !INPUT_VALUES[index].equals(string(payload, "bigdec"))) {
                throw new IllegalArgumentException("send step " + (index + 1) + " payload is not pinned");
            }
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
            throw new IllegalArgumentException("JSON object has unexpected or missing fields");
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
            long result = value.asLong();
            if (result < Integer.MIN_VALUE || result > Integer.MAX_VALUE
                    || Double.compare(value.asDouble(), result) != 0) {
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            }
            return (int) result;
        } catch (RuntimeException ex) {
            if (ex instanceof IllegalArgumentException
                    && ex.getMessage() != null && ex.getMessage().startsWith(name + " must be")) {
                throw ex;
            }
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
            if (next > EXPECTED_VALUES.length || newEvents == null || newEvents.length != 1
                    || oldEvents != null && oldEvents.length > 0) {
                throw new IllegalStateException("listener callback must contain one new row only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time at sequence " + next);
            }
            String[] names = newEvents[0].getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, new String[]{"c0"})) {
                throw new IllegalStateException("aggregate field metadata is not pinned");
            }
            Object value = newEvents[0].get("c0");
            if (!(value instanceof BigDecimal)
                    || !EXPECTED_VALUES[next - 1].equals(((BigDecimal) value).toPlainString())) {
                throw new IllegalStateException("unexpected callback value at sequence " + next);
            }
            sequence = next;
            JsonObject record = new JsonObject().add("case", CASE).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents));
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray rows = new JsonArray();
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
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
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
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
}
