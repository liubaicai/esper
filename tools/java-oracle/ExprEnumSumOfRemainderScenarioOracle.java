import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
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

import java.math.BigDecimal;
import java.math.BigInteger;
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
 * Direct Esper 9.0.0 oracle for the four remaining ExprEnumSumOf executions
 * (ordinals 1, 3, 4, 5) replayed as one differential chain:
 *
 * sum-events-plus (ord 1, ExprEnumSumEventsPlus): beans.sumOf over
 * SupportBean_Container.beans with the element, element+index and
 * element+index+size lambda footprints plus a case-when null branch; null
 * and empty collections both yield null.
 *
 * sum-scalar-string (ord 3, ExprEnumSumScalarStringValue): strvals.sumOf
 * over SupportCollection.strvals through the extractNum/extractBigDecimal
 * single-row functions (ExprEnumMinMax.MyService semantics: substring(1)
 * parsed as Integer/BigDecimal) with the same three lambda footprints.
 *
 * sum-invalid (ord 4, ExprEnumSumInvalid): two tryInvalidCompile probes
 * record the pinned Java message prefixes; both compile without the
 * runtime path, mirroring env.tryInvalidCompile's path-less
 * compileWCheckedEx.
 *
 * sum-array (ord 5, ExprEnumSumArray): constant collections {1d, 2d},
 * {BigInteger.valueOf(1), BigInteger.valueOf(2)}, {1L, 2L} and
 * {1L, 2L, null} summed over one SupportBean.
 *
 * Mirroring SupportEvalRunner, each listener case deploys
 * "@name('s0') <case EPL>" once, sends every assertion event, then
 * undeploys. The pinned case EPL is the contract text verbatim (implicit
 * aliases, double space in "sumOf( (x, i)"). SupportBean is the shared
 * regression bean; SupportBean_Container and SupportCollection are map
 * types because the regression-lib jar is not on the oracle classpath —
 * beans is declared as the SupportBean[] event-type array and strvals as
 * String[]. BigDecimal/BigInteger results render as exact decimal strings
 * (toPlainString()/toString()) matching the Go trace normalizer. The
 * TraceWriter skips null/null listener callbacks.
 */
public final class ExprEnumSumOfRemainderScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-enum-sumof-remainder";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumSumOf.java";

    private static final String DESCRIPTION =
            "ExprEnumSumOf ordinals 1, 3, 4 and 5 (the remaining executions): sum-events-plus sums intBoxed over SupportBean_Container.beans with element/index/size lambda footprints and a case-when null branch; sum-scalar-string sums extractNum/extractBigDecimal UDF results over SupportCollection.strvals; sum-invalid records the two tryInvalidCompile message prefixes; sum-array sums constant Double, BigInteger and nullable Long collections. Each listener case deploys s0 once, sends every assertion event, then undeploys.";

    private static final String[] CASES = {
            "sum-events-plus", "sum-scalar-string", "sum-invalid", "sum-array"};
    private static final int[] ORDINALS = {1, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-8497175e9fc13501285c",
            "java-runtime-93ec466957ff19bc38a6",
            "java-runtime-bbe116cdf8ad7e13172f",
            "java-runtime-b0455a5d1e4b447f34b1"
    };
    private static final String[] EXECUTIONS = {
            "ExprEnumSumEventsPlus",
            "ExprEnumSumScalarStringValue",
            "ExprEnumSumInvalid",
            "ExprEnumSumArray"
    };
    private static final String[] STATIC_IDS = {
            "java-7443fe2db668140640df",
            "java-ce9a9b8124ed9b9cda09",
            "java-3bdebdfafa650da60077",
            "java-2c99e8eabfc5cdb0b018"
    };
    private static final String[] OBSERVATIONS = {
            "listener",
            "listener",
            "compile-error; two tryInvalidCompile probes pin the Java message prefixes: beans.sumof() rejects a collection of events as the 0-parameter input and strvals.sumOf(v => null) rejects a null-typed lambda result; both compile without the runtime path",
            "listener"
    };
    private static final String[] CASE_EPLS = {
            "select beans.sumOf(x => intBoxed) c0, beans.sumOf( (x, i) => intBoxed + i*10) c1, beans.sumOf( (x, i, s) => intBoxed + i*10 + s*100) c2, beans.sumOf( (x, i) => case when i = 1 then null else 1 end) c3 from SupportBean_Container",
            "select strvals.sumOf(v => extractNum(v)) c0, strvals.sumOf(v => extractBigDecimal(v)) c1, strvals.sumOf( (v, i) => extractNum(v) + i*10) c2, strvals.sumOf( (v, i, s) => extractNum(v) + i*10 + s*100) c3 from SupportCollection",
            "select beans.sumof() from SupportBean_Container",
            "select {1d, 2d}.sumOf() c0, {BigInteger.valueOf(1), BigInteger.valueOf(2)}.sumOf() c1, {1L, 2L}.sumOf() c2, {1L, 2L, null}.sumOf() c3 from SupportBean"
    };

    // Verbatim transcriptions of ExprEnumSumOf lines 165-169 (prefixes only;
    // the JVM FQN suffix of the first message diverges by design).
    private static final String[] PROBE_STATEMENTS = {
            "sumof-no-param", "sumof-null-lambda"
    };
    private static final String[] PROBE_EPLS = {
            "select beans.sumof() from SupportBean_Container",
            "select strvals.sumOf(v => null) from SupportCollection"
    };
    private static final String[] PROBE_ERRORS = {
            "Failed to validate select-clause expression 'beans.sumof()': Invalid input for built-in enumeration method 'sumof' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '",
            "Failed to validate select-clause expression 'strvals.sumOf()': Failed to validate enumeration method 'sumOf', expected a non-null result for expression parameter 0 but received a null-typed expression"
    };

    private static final int EXPECTED_STEPS = 22;
    private static final int EXPECTED_RECORDS = 11;

    private ExprEnumSumOfRemainderScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprEnumSumOfRemainderScenarioOracle <scenario.json>");
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
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        Map<String, Object> containerType = new HashMap<>();
        containerType.put("beans", "SupportBean[]");
        configuration.getCommon().addEventType("SupportBean_Container", containerType);
        Map<String, Object> collectionType = new HashMap<>();
        collectionType.put("strvals", String[].class);
        configuration.getCommon().addEventType("SupportCollection", collectionType);
        configuration.getCompiler().addPlugInSingleRowFunction("extractNum",
                ExprEnumSumOfRemainderScenarioOracle.class.getName(), "extractNum");
        configuration.getCompiler().addPlugInSingleRowFunction("extractBigDecimal",
                ExprEnumSumOfRemainderScenarioOracle.class.getName(), "extractBigDecimal");

        String runtimeURI = "parity-" + SCENARIO_ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            boolean active = false;
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
                    String epl = "@name('s0') " + CASE_EPLS[caseIndex];
                    // CompilerArguments(configuration) carries the compiler-level
                    // plug-in single-row functions (extractNum/extractBigDecimal);
                    // the runtime path does not (ContextHashScenarioOracle precedent).
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                    findStatement(deployment).addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("build-error".equals(operation)) {
                    buildErrorStep(runtime, configuration, caseName, step, records);
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
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

    /**
     * Mirrors ExprEnumMinMax.MyService.extractNum: Integer.parseInt of the
     * value after its leading 'E' marker.
     */
    public static Integer extractNum(String value) {
        return Integer.parseInt(value.substring(1));
    }

    /**
     * Mirrors ExprEnumMinMax.MyService.extractBigDecimal: new BigDecimal of
     * the value after its leading 'E' marker.
     */
    public static BigDecimal extractBigDecimal(String value) {
        return new BigDecimal(value.substring(1));
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean_Container".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            JsonValue beans = payload.get("beans");
            if (beans == null || beans.isNull()) {
                event.put("beans", null);
            } else {
                JsonArray items = beans.asArray();
                SupportBean[] list = new SupportBean[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    JsonObject bean = items.get(index).asObject();
                    SupportBean item = new SupportBean();
                    item.setTheString(bean.getString("theString", null));
                    JsonValue intBoxed = bean.get("intBoxed");
                    if (intBoxed != null && !intBoxed.isNull()) {
                        item.setIntBoxed(intBoxed.asInt());
                    }
                    list[index] = item;
                }
                event.put("beans", list);
            }
            runtime.getEventService().sendEventMap(event, "SupportBean_Container");
        } else if ("SupportCollection".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            JsonValue strvals = payload.get("strvals");
            if (strvals == null || strvals.isNull()) {
                event.put("strvals", null);
            } else {
                JsonArray items = strvals.asArray();
                String[] list = new String[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    list[index] = items.get(index).asString();
                }
                event.put("strvals", list);
            }
            runtime.getEventService().sendEventMap(event, "SupportCollection");
        } else if ("SupportBean".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean(), "SupportBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). Probes marked compileWithoutPath compile without the
     * runtime path, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String epl = step.getString("epl", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
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
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned steps — deploy s0, the assertion sends and undeploy-all
     * for listener cases; the two path-less build-error probes and
     * undeploy-all for sum-invalid. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        String[][] details = {
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
                {"build-error", "build-error", "undeploy-all"},
                {"deploy", "send", "undeploy-all"},
        };
        String[][] eventTypes = {
                {"SupportBean_Container"},
                {"SupportCollection"},
                {},
                {"SupportBean"},
        };
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int probes = 0;
            for (String operation : details[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!eventTypes[caseIndex][0].equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        break;
                    case "build-error":
                        requireFields(step, "op", "case", "statement", "epl", "expectError",
                                "compileWithoutPath");
                        if (!step.getBoolean("compileWithoutPath", false)
                                || !PROBE_STATEMENTS[probes].equals(string(step, "statement"))
                                || !PROBE_EPLS[probes].equals(string(step, "epl"))
                                || !PROBE_ERRORS[probes].equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException("build-error step " + cursor + " is not pinned");
                        }
                        probes++;
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
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
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
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
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
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
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
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
