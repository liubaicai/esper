import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
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
import java.time.format.DateTimeParseException;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Java oracle for EPLVariablesOutputRate: variable-driven output rates.
 *
 * Replays the four executions on one runtime, mirroring the regression-suite
 * harness: preconfigured long variable var_output_limit initialized to 3,
 * SupportBean and SupportMarketDataBean event types, internal timer disabled,
 * and the rethrowing exception handler so statement failures surface to the
 * sender/advancer thread.  The three event-rate cases share one observable
 * timeline ("output last every var_output_limit events" redeployed per case
 * over the plain, SODA-model, and epl-to-model compile-deploy forms); the
 * time-snapshot case runs "output snapshot every var_output_limit seconds"
 * and ends with the null-variable time-period evaluation failure observed
 * through the harness rethrow wrapper.
 */
public final class EPLVariablesOutputRateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-variable-output-rate";
    private static final String DESCRIPTION =
            "EPLVariablesOutputRate variable-driven output rates: event-count 'output last every var_output_limit "
                    + "events' across plain, SODA-model, and epl-to-model compile-deploy forms plus time-based "
                    + "'output snapshot every var_output_limit seconds' with on-set variable reassignment and the "
                    + "null-rate evaluation failure.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/"
                    + "EPLVariablesOutputRate.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-0b028b6fe8bddbbb6480",
            "java-runtime-0539523182c81174c7f6",
            "java-runtime-303a9b5ee74d52e62125",
            "java-runtime-4909a80e9612b815310e"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLVariableOutputRateEventsAll",
            "EPLVariableOutputRateEventsAllOM",
            "EPLVariableOutputRateEventsAllCompile",
            "EPLVariableOutputRateTimeAll"
    };
    private static final String[] STATIC_IDS = {
            "java-b8a2d4466ca4d002fa69",
            "java-b08aeecf9371278c24f6",
            "java-9ee028b9ce2729673e8e",
            "java-b3199a3fee2738477788"
    };

    private static final String CASE_EVENTS = "events";
    private static final String CASE_EVENTS_OM = "events-om";
    private static final String CASE_EVENTS_COMPILE = "events-compile";
    private static final String CASE_TIME_SNAPSHOT = "time-snapshot";
    private static final String VARIABLE_NAME = "var_output_limit";
    private static final String EPL_EVENTS =
            "@name('s0') select count(*) as cnt from SupportBean output last every var_output_limit events";
    private static final String EPL_ON_SET = "on SupportMarketDataBean set var_output_limit = volume";
    private static final String EPL_TIME_SNAPSHOT =
            "@name('s0') select count(*) as cnt from SupportBean output snapshot every var_output_limit seconds";
    private static final String ERROR_STATEMENT = "s0";

    /**
     * Message of the RuntimeException thrown out of advanceTime for the
     * 14-second tick: the harness rethrow wrapper around the EPException from
     * TimePeriodComputeNCGivenTPNonCalEval ("Received null value evaluating
     * time period" rendered by ExprTimePeriodForge.makeTimePeriodParamNullException).
     */
    private static final String ERROR_MESSAGE = "Unexpected exception in statement 's0': "
            + "Failed to evaluate time period, received a null value for "
            + "'Received null value evaluating time period'";

    // Listener-visible row values per case, in callback order.
    private static final long[] EVENTS_VALUES = {3, 8, 10, 11, 12, 13};
    private static final long[] TIME_SNAPSHOT_VALUES = {2, 4, 4, 4, 6};
    private static final int EVENTS_RECORDS = EVENTS_VALUES.length;
    private static final int TIME_SNAPSHOT_RECORDS = TIME_SNAPSHOT_VALUES.length + 1;
    private static final int EXPECTED_RECORDS =
            3 * EVENTS_RECORDS + TIME_SNAPSHOT_RECORDS;
    private static final int EXPECTED_STEPS = 99;

    private EPLVariablesOutputRateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLVariablesOutputRateScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        configuration.getCommon().addVariable(VARIABLE_NAME, long.class, "3");
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_EVENTS, runtime, allSteps, records);
            runCase(CASE_EVENTS_OM, runtime, allSteps, records);
            runCase(CASE_EVENTS_COMPILE, runtime, allSteps, records);
            runCase(CASE_TIME_SNAPSHOT, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /** Replays one case's steps on the shared runtime; sequence restarts per case. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
        int[] seq = new int[] {0};
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (string(step, "op")) {
                case "set-variable": {
                    Object value = decodeAssignedValue(step.get("payload"));
                    runtime.getVariableService().setVariableValue(null, string(step, "name"), value);
                    break;
                }
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        if (ERROR_STATEMENT.equals(statement.getName())) {
                            statement.addListener(listener(caseName, seq, records));
                        }
                    }
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"), object(step.get("payload"), "payload"));
                    break;
                case "advance-time":
                    runtime.getEventService().advanceTime(instant(step).toEpochMilli());
                    break;
                case "advance-time-error": {
                    try {
                        runtime.getEventService().advanceTime(instant(step).toEpochMilli());
                    } catch (RuntimeException ex) {
                        recordAdvanceTimeError(caseName, runtime, ++seq[0], ex, records);
                        break;
                    }
                    throw new IllegalStateException("advance-time to " + string(step, "at")
                            + " did not fail for case " + caseName);
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
    }

    /** Emits the pinned advance-time-error record; drift from the pinned message fails hard. */
    private static void recordAdvanceTimeError(String caseName, EPRuntime runtime, int sequence,
                                               RuntimeException caught, JsonArray records) {
        if (caught.getMessage() == null || !ERROR_MESSAGE.equals(caught.getMessage())) {
            throw new IllegalStateException("advance-time-error message drift: expected ["
                    + ERROR_MESSAGE + "] got [" + caught.getMessage() + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "advance-time-error");
        record.add("statement", ERROR_STATEMENT);
        record.add("sequence", sequence);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("value", caught.getMessage());
        records.add(record);
    }

    /** Listener emitting one record per new row; an old stream is a drift failure. */
    private static UpdateListener listener(String caseName, int[] seq, JsonArray records) {
        return (newEvents, oldEvents, statement, runtime) -> {
            if (oldEvents != null) {
                throw new IllegalStateException("unexpected old stream for " + caseName + "/"
                        + statement.getName());
            }
            if (newEvents == null) {
                return;
            }
            for (EventBean event : newEvents) {
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("sequence", seq[0]);
                record.add("time",
                        Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                JsonArray newRows = new JsonArray();
                JsonObject row = new JsonObject();
                row.add("kind", "row");
                JsonObject fields = new JsonObject();
                for (String property : event.getEventType().getPropertyNames()) {
                    fields.add(property, renderFieldValue(event.get(property)));
                }
                row.add("fields", fields);
                newRows.add(row);
                record.add("new", newRows);
                records.add(record);
            }
        };
    }

    /** Canonical row-field rendering: numbers as JSON numbers, null, boolean, string. */
    private static JsonValue renderFieldValue(Object value) {
        if (value == null) {
            return Json.NULL;
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof String) {
            return Json.value((String) value);
        }
        return Json.value(String.valueOf(value));
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportMarketDataBean": {
                JsonValue volume = payload.get("volume");
                Long volumeValue = volume == null || volume.isNull() ? null : volume.asLong();
                SupportMarketDataBean bean = new SupportMarketDataBean(
                        string(payload, "symbol"), payload.getDouble("price", 0), volumeValue,
                        string(payload, "feed"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /** Decodes a tagged assignment value; long-tagged values keep the Java boxed width. */
    private static Object decodeAssignedValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value instanceof JsonObject) {
            JsonObject tag = value.asObject();
            String type = tag.getString("type", "");
            JsonValue inner = tag.get("value");
            switch (type) {
                case "integer":
                    return inner.isNull() ? null : (Integer) inner.asInt();
                case "long":
                    return inner.isNull() ? null : inner.asLong();
                case "short":
                    return inner.isNull() ? null : (short) inner.asInt();
                case "byte":
                    return inner.isNull() ? null : (byte) inner.asInt();
                case "float":
                    return inner.isNull() ? null : (float) inner.asDouble();
                case "double":
                    return inner.isNull() ? null : inner.asDouble();
                case "boolean":
                    return inner.isNull() ? null : inner.asBoolean();
                case "string":
                    return inner.isNull() ? null : inner.asString();
                default:
                    throw new IllegalArgumentException("unknown assignment type tag " + type);
            }
        }
        throw new IllegalArgumentException("set-variable payload must be a tagged object");
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
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            String expectedCase = index == 3 ? CASE_TIME_SNAPSHOT
                    : (index == 0 ? CASE_EVENTS : (index == 1 ? CASE_EVENTS_OM : CASE_EVENTS_COMPILE));
            String expectedEpl = index == 3 ? EPL_TIME_SNAPSHOT : EPL_EVENTS;
            if (!expectedCase.equals(string(definition, "case"))
                    || integer(definition, "ordinal") != index
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !expectedEpl.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateEventsCase(steps, offset, CASE_EVENTS);
        offset = validateEventsCase(steps, offset, CASE_EVENTS_OM);
        offset = validateEventsCase(steps, offset, CASE_EVENTS_COMPILE);
        offset = validateTimeSnapshotCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateEventsCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateSetVariable(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, EPL_EVENTS, "s0");
        for (String theString : new String[] {"E1", "E2", "E3"}) {
            validateBeanSend(steps.get(offset++), caseName, theString);
        }
        validateDeploy(steps.get(offset++), caseName, EPL_ON_SET, "s1");
        validateSetterSend(steps.get(offset++), caseName, 5L);
        for (String theString : new String[] {"E4", "E5", "E6", "E7"}) {
            validateBeanSend(steps.get(offset++), caseName, theString);
        }
        validateBeanSend(steps.get(offset++), caseName, "E8");
        validateSetterSend(steps.get(offset++), caseName, 2L);
        validateBeanSend(steps.get(offset++), caseName, "E9");
        validateBeanSend(steps.get(offset++), caseName, "E10");
        validateSetterSend(steps.get(offset++), caseName, 1L);
        validateBeanSend(steps.get(offset++), caseName, "E11");
        validateBeanSend(steps.get(offset++), caseName, "E12");
        validateSetterSend(steps.get(offset++), caseName, null);
        validateBeanSend(steps.get(offset++), caseName, "E13");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateTimeSnapshotCase(JsonArray steps, int offset) {
        String caseName = CASE_TIME_SNAPSHOT;
        validateCaseMarker(steps.get(offset++), caseName);
        validateSetVariable(steps.get(offset++), caseName);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), caseName, EPL_TIME_SNAPSHOT, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E1");
        validateBeanSend(steps.get(offset++), caseName, "E2");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:02.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:03Z");
        validateDeploy(steps.get(offset++), caseName, EPL_ON_SET, "s1");
        validateSetterSend(steps.get(offset++), caseName, 5L);
        validateSetterSend(steps.get(offset++), caseName, 1L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:03.200Z");
        validateBeanSend(steps.get(offset++), caseName, "E3");
        validateBeanSend(steps.get(offset++), caseName, "E4");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:03.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:04Z");
        validateSetterSend(steps.get(offset++), caseName, 4L);
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:04.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:05Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:07.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:08Z");
        validateBeanSend(steps.get(offset++), caseName, "E5");
        validateBeanSend(steps.get(offset++), caseName, "E6");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:11.999Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:12Z");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:13Z");
        validateSetterSend(steps.get(offset++), caseName, 2L);
        validateBeanSend(steps.get(offset++), caseName, "E7");
        validateBeanSend(steps.get(offset++), caseName, "E8");
        validateAdvanceTime(steps.get(offset++), caseName, "1970-01-01T00:00:13.999Z");
        validateSetterSend(steps.get(offset++), caseName, null);
        validateAdvanceTimeError(steps.get(offset++), caseName, "1970-01-01T00:00:14Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateSetVariable(JsonValue value, String caseName) {
        JsonObject step = object(value, "set-variable step");
        requireFields(step, "op", "case", "name", "payload");
        if (!"set-variable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !VARIABLE_NAME.equals(string(step, "name"))) {
            throw new IllegalArgumentException("set-variable step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "set-variable payload");
        requireFields(payload, "type", "value");
        if (!"long".equals(string(payload, "type"))
                || longInteger(payload.get("value"), "set-variable value") != 3L) {
            throw new IllegalArgumentException("set-variable payload is not pinned for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String expectedEpl, String expectedStatement) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString");
        if (!expectedString.equals(string(payload, "theString"))) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateSetterSend(JsonValue value, String caseName, Long expectedVolume) {
        JsonObject step = object(value, "SupportMarketDataBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("setter step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload");
        requireFields(payload, "symbol", "price", "volume", "feed");
        JsonValue volume = payload.get("volume");
        if (!"".equals(string(payload, "symbol"))
                || payload.getDouble("price", Double.NaN) != 0d
                || !"".equals(string(payload, "feed"))
                || (expectedVolume == null ? !(volume == null || volume.isNull())
                        : volume == null || volume.isNull()
                                || longInteger(volume, "volume") != expectedVolume)) {
            throw new IllegalArgumentException("setter payload is not pinned for " + caseName);
        }
    }

    private static void validateAdvanceTime(JsonValue value, String caseName, String expectedAt) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "case", "at");
        if (!"advance-time".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedAt.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step is not pinned for " + caseName);
        }
    }

    private static void validateAdvanceTimeError(JsonValue value, String caseName, String expectedAt) {
        JsonObject step = object(value, "advance-time-error step");
        requireFields(step, "op", "case", "at");
        if (!"advance-time-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedAt.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time-error step is not pinned for " + caseName);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static Instant instant(JsonObject step) {
        try {
            return Instant.parse(string(step, "at"));
        } catch (DateTimeParseException ex) {
            throw new IllegalArgumentException("step time is not an instant: " + string(step, "at"), ex);
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }

    /** Local mirror of the regression SupportMarketDataBean (regression-lib is not on the oracle classpath). */
    public static final class SupportMarketDataBean {
        private final String symbol;
        private final double price;
        private final Long volume;
        private final String feed;

        public SupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() {
            return symbol;
        }

        public double getPrice() {
            return price;
        }

        public Long getVolume() {
            return volume;
        }

        public String getFeed() {
            return feed;
        }
    }
}
