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
import com.espertech.esper.common.internal.support.SupportBean_S0;
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
 * Java oracle for InfraNWTableSubqFilteredCorrel: a filtered correlated scalar
 * subquery (select intPrimitive from MyInfra(intPrimitive&lt;0) sw where
 * s0.p00=sw.theString) evaluated against a keepall named window and against a
 * composite-key table, covering the enable_window_subquery_indexshare create
 * hint, the disable_window_subquery_indexshare consumer hint, and explicit
 * create index on the correlated column.
 *
 * Replays the seven executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean and SupportBean_S0 from
 * esper-common plus the "S0" event type name the harness provides for the
 * consume statement's `from S0 s0` stream, internal timer disabled, and the
 * rethrowing exception handler so statement failures surface to the sender
 * thread.  The listener attaches to the consume statement only (create,
 * insert, and index deploy silently) with a per-case sequence counter;
 * records carry a new array only (default istream selector) with sorted field
 * names.
 */
public final class InfraNWTableSubqFilteredCorrelScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-subq-filtered-correl";
    private static final String DESCRIPTION =
            "InfraNWTableSubqFilteredCorrel filtered correlated scalar subquery (select intPrimitive "
                    + "from MyInfra(intPrimitive<0) sw where s0.p00=sw.theString) over a keepall named window "
                    + "and over a table, covering named-window subquery index sharing through "
                    + "enable_window_subquery_indexshare create hints and disable_window_subquery_indexshare "
                    + "consumer hints plus explicit create index on the correlated column, captured from the "
                    + "consume listener (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableSubqFilteredCorrel.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableSubqFilteredCorrel.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-6823e53d322aa502295b",
            "java-runtime-9ff8e85038e7fa442f0b",
            "java-runtime-11e662916b4e4bebf3ed",
            "java-runtime-fdb6dcb460167c720624",
            "java-runtime-d8db6be7fe12e953f8f2",
            "java-runtime-df8a638d47fce3bbe9e7",
            "java-runtime-1e359a7b2b7cee11d210"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion",
            "InfraNWTableSubqFilteredCorrelAssertion"
    };
    private static final String[] STATIC_IDS = {
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac",
            "java-cef495b8aa35bb19a5ac"
    };

    private static final String CASE_NW_NO_SHARE = "nw-no-share";
    private static final String CASE_NW_NO_SHARE_INDEX = "nw-no-share-index";
    private static final String CASE_NW_SHARE = "nw-share";
    private static final String CASE_NW_SHARE_DISABLE = "nw-share-disable";
    private static final String CASE_NW_SHARE_DISABLE_INDEX = "nw-share-disable-index";
    private static final String CASE_TABLE_NO_SHARE = "table-no-share";
    private static final String CASE_TABLE_NO_SHARE_INDEX = "table-no-share-index";

    // Verbatim transcriptions of InfraNWTableSubqFilteredCorrel lines 52-68;
    // create and insert are unnamed in the Java source.
    private static final String EPL_CREATE_NW =
            "@public create window MyInfra#keepall as select * from SupportBean";
    private static final String EPL_CREATE_NW_INDEX_SHARE =
            "@Hint('enable_window_subquery_indexshare') " + EPL_CREATE_NW;
    private static final String EPL_CREATE_TABLE =
            "@public create table MyInfra (theString string primary key, intPrimitive int primary key)";
    private static final String EPL_INSERT =
            "insert into MyInfra select theString, intPrimitive from SupportBean";
    private static final String EPL_INDEX =
            "@name('index') create index MyIndex on MyInfra(theString)";
    private static final String EPL_CONSUME =
            "@name('consume') select (select intPrimitive from MyInfra(intPrimitive<0) sw where "
                    + "s0.p00=sw.theString) as val from S0 s0";
    private static final String EPL_CONSUME_DISABLE_SHARE =
            "@Hint('disable_window_subquery_indexshare') " + EPL_CONSUME;

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(Arrays.asList("consume"));
    private static final int EXPECTED_RECORDS = 28;
    private static final int EXPECTED_STEPS = 94;

    private InfraNWTableSubqFilteredCorrelScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableSubqFilteredCorrelScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType("S0", SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_NW_NO_SHARE, runtime, allSteps, records);
            runCase(CASE_NW_NO_SHARE_INDEX, runtime, allSteps, records);
            runCase(CASE_NW_SHARE, runtime, allSteps, records);
            runCase(CASE_NW_SHARE_DISABLE, runtime, allSteps, records);
            runCase(CASE_NW_SHARE_DISABLE_INDEX, runtime, allSteps, records);
            runCase(CASE_TABLE_NO_SHARE, runtime, allSteps, records);
            runCase(CASE_TABLE_NO_SHARE_INDEX, runtime, allSteps, records);
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

    /** Replays one case's steps on the shared runtime; sequences restart per case. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
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
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        if (LISTENED_STATEMENTS.contains(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                    }
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Listener emitting one record per invocation with a per-case sequence
     * counter; the default istream selector means only a new array renders and
     * only when non-empty.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                // The fixed consume EPL consumes the event type registered as
                // "S0" (`from S0 s0`), and Esper routes sends by event type
                // name, so the scenario's SupportBean_S0 send must be
                // delivered under the "S0" type name for the statement to see
                // it (the class is registered under both names).
                runtime.getEventService().sendEventBean(new SupportBean_S0(
                        (int) longInteger(payload.get("id"), "id"),
                        string(payload, "p00")), "S0");
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
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
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        String[] expectedCases = {
                CASE_NW_NO_SHARE, CASE_NW_NO_SHARE_INDEX, CASE_NW_SHARE, CASE_NW_SHARE_DISABLE,
                CASE_NW_SHARE_DISABLE_INDEX, CASE_TABLE_NO_SHARE, CASE_TABLE_NO_SHARE_INDEX};
        int[] expectedOrdinals = {0, 1, 2, 3, 4, 5, 6};
        String[] expectedEpls = {
                EPL_CONSUME, EPL_CONSUME, EPL_CONSUME, EPL_CONSUME_DISABLE_SHARE,
                EPL_CONSUME_DISABLE_SHARE, EPL_CONSUME, EPL_CONSUME};
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !expectedEpls[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCase(steps, offset, CASE_NW_NO_SHARE, EPL_CREATE_NW, EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_NW_NO_SHARE_INDEX, EPL_CREATE_NW,
                EPL_CONSUME, true);
        offset = validateCase(steps, offset, CASE_NW_SHARE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_NW_SHARE_DISABLE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_CONSUME_DISABLE_SHARE, false);
        offset = validateCase(steps, offset, CASE_NW_SHARE_DISABLE_INDEX,
                EPL_CREATE_NW_INDEX_SHARE, EPL_CONSUME_DISABLE_SHARE, true);
        offset = validateCase(steps, offset, CASE_TABLE_NO_SHARE, EPL_CREATE_TABLE,
                EPL_CONSUME, false);
        offset = validateCase(steps, offset, CASE_TABLE_NO_SHARE_INDEX, EPL_CREATE_TABLE,
                EPL_CONSUME, true);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact per-case step sequence mirroring InfraNWTableSubqFilteredCorrel
     * lines 51-91: create, insert, the optional explicit index deploy, the two
     * SupportBean population sends, the consume statement, four S0 sends
     * interleaved with two more SupportBean sends, and the folded undeploy of
     * the module containing consume plus undeployAll.
     */
    private static int validateCase(JsonArray steps, int offset, String caseName,
                                    String createEpl, String consumeEpl, boolean withIndex) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        if (withIndex) {
            validateDeploy(steps.get(offset++), caseName, "index", EPL_INDEX);
        }
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", -2);
        validateDeploy(steps.get(offset++), caseName, "consume", consumeEpl);
        validateS0Send(steps.get(offset++), caseName, 10, "E1");
        validateS0Send(steps.get(offset++), caseName, 20, "E2");
        validateBeanSend(steps.get(offset++), caseName, "E3", -3);
        validateBeanSend(steps.get(offset++), caseName, "E4", 4);
        validateS0Send(steps.get(offset++), caseName, -3, "E3");
        validateS0Send(steps.get(offset++), caseName, 20, "E4");
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

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, long expectedId,
                                       String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (longInteger(payload.get("id"), "id") != expectedId
                || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for " + caseName);
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
}
