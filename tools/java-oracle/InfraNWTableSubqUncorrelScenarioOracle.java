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
import com.espertech.esper.common.internal.support.SupportBean_A;
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
 * Java oracle for InfraNWTableSubqUncorrel: uncorrelated scalar subqueries
 * (select a from MyInfra) over a keepall named window and over a table,
 * including the enable_window_subquery_indexshare create hint, the
 * disable_window_subquery_indexshare consumer hint, multi-row scalar null
 * results, and on-delete row propagation.
 *
 * Replays the four executions on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean and SupportBean_A from
 * esper-common plus a local mirror of the regression SupportMarketDataBean
 * (regression-lib is not on the oracle classpath), internal timer disabled,
 * and the rethrowing exception handler so statement failures surface to the
 * sender thread.  Listeners attach to create, select, selectTwo, and delete
 * with per-statement sequence counters that restart per case; records carry
 * new and old arrays (each only when non-empty) with sorted field names.
 */
public final class InfraNWTableSubqUncorrelScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-subq-uncorrel";
    private static final String DESCRIPTION =
            "InfraNWTableSubqUncorrel uncorrelated scalar subqueries (select a from MyInfra) over a keepall "
                    + "named window and over a table, covering named-window subquery index sharing through "
                    + "enable_window_subquery_indexshare create hints and disable_window_subquery_indexshare "
                    + "consumer hints, multi-row scalar null results, and on-delete row propagation captured "
                    + "from create, select, selectTwo, and delete listeners (Java source regression-lib/src/main/"
                    + "java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqUncorrel.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableSubqUncorrel.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-e175cfaf55ce541ca088",
            "java-runtime-f03db66c82987b5cef54",
            "java-runtime-d521ebd5c45c89c8e842",
            "java-runtime-1a498d1407159a6d8c90"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false}",
            "InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=false}",
            "InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=true, "
                    + "disableIndexShareConsumer=true}",
            "InfraNWTableSubqUncorrelAssertion{namedWindow=false, enableIndexShareCreate=false, "
                    + "disableIndexShareConsumer=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-5af40542812749ae30e4",
            "java-5af40542812749ae30e4",
            "java-5af40542812749ae30e4",
            "java-5af40542812749ae30e4"
    };

    private static final String CASE_NW_NO_SHARE = "nw-no-share";
    private static final String CASE_NW_INDEX_SHARE = "nw-index-share";
    private static final String CASE_NW_DISABLE_SHARE = "nw-disable-share";
    private static final String CASE_TABLE = "table";

    // Verbatim transcriptions of InfraNWTableSubqUncorrel lines 53-69 and 91-114.
    private static final String EPL_CREATE_NW =
            "@name('create') @public create window MyInfra#keepall as "
                    + "select theString as a, longPrimitive as b, longBoxed as c from SupportBean";
    private static final String EPL_CREATE_NW_INDEX_SHARE =
            "@Hint('enable_window_subquery_indexshare') " + EPL_CREATE_NW;
    private static final String EPL_CREATE_TABLE =
            "@name('create') @public create table MyInfra(a string primary key, b long, c long)";
    private static final String EPL_INSERT =
            "insert into MyInfra select theString as a, longPrimitive as b, longBoxed as c from SupportBean";
    private static final String EPL_SELECT =
            "@name('select') select irstream (select a from MyInfra) as value, symbol from SupportMarketDataBean";
    private static final String EPL_SELECT_DISABLE_SHARE =
            "@Hint('disable_window_subquery_indexshare') " + EPL_SELECT;
    private static final String EPL_SELECT_TWO =
            "@name('selectTwo') select irstream (select a from MyInfra) as value, symbol from SupportMarketDataBean";
    private static final String EPL_SELECT_TWO_DISABLE_SHARE =
            "@Hint('disable_window_subquery_indexshare') " + EPL_SELECT_TWO;
    private static final String EPL_DELETE =
            "@name('delete') on SupportBean_A delete from MyInfra where id = a";

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("create", "select", "selectTwo", "delete"));
    private static final int EXPECTED_RECORDS = 67;
    private static final int EXPECTED_STEPS = 72;

    private InfraNWTableSubqUncorrelScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableSubqUncorrelScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
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
            runCase(CASE_NW_INDEX_SHARE, runtime, allSteps, records);
            runCase(CASE_NW_DISABLE_SHARE, runtime, allSteps, records);
            runCase(CASE_TABLE, runtime, allSteps, records);
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
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty.
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
            case "SupportMarketDataBean": {
                JsonValue volume = payload.get("volume");
                SupportMarketDataBean bean = new SupportMarketDataBean(
                        string(payload, "symbol"),
                        payload.getDouble("price", Double.NaN),
                        volume == null || volume.isNull() ? null : volume.asLong(),
                        string(payload, "feed"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setLongPrimitive(longInteger(payload.get("longPrimitive"), "longPrimitive"));
                JsonValue longBoxed = payload.get("longBoxed");
                bean.setLongBoxed(longBoxed == null || longBoxed.isNull()
                        ? null : longBoxed.asLong());
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(string(payload, "id")), type);
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
                CASE_NW_NO_SHARE, CASE_NW_INDEX_SHARE, CASE_NW_DISABLE_SHARE, CASE_TABLE};
        int[] expectedOrdinals = {0, 1, 0, 1};
        String[] expectedEpls = {
                EPL_SELECT, EPL_SELECT, EPL_SELECT_DISABLE_SHARE, EPL_SELECT};
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
        offset = validateCase(steps, offset, CASE_NW_NO_SHARE, EPL_CREATE_NW,
                EPL_SELECT, EPL_SELECT_TWO);
        offset = validateCase(steps, offset, CASE_NW_INDEX_SHARE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_SELECT, EPL_SELECT_TWO);
        offset = validateCase(steps, offset, CASE_NW_DISABLE_SHARE, EPL_CREATE_NW_INDEX_SHARE,
                EPL_SELECT_DISABLE_SHARE, EPL_SELECT_TWO_DISABLE_SHARE);
        offset = validateCase(steps, offset, CASE_TABLE, EPL_CREATE_TABLE,
                EPL_SELECT, EPL_SELECT_TWO);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact per-case step sequence including the mid-case selectTwo and delete
     * deployments, mirroring InfraNWTableSubqUncorrel lines 51-145.
     */
    private static int validateCase(JsonArray steps, int offset, String caseName,
                                    String createEpl, String selectEpl, String selectTwoEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        validateDeploy(steps.get(offset++), caseName, "select", selectEpl);
        validateMarketSend(steps.get(offset++), caseName, "M1");
        validateBeanSend(steps.get(offset++), caseName, "S1", 1, 2);
        validateDeploy(steps.get(offset++), caseName, "selectTwo", selectTwoEpl);
        validateMarketSend(steps.get(offset++), caseName, "M1");
        validateBeanSend(steps.get(offset++), caseName, "S2", 10, 20);
        validateMarketSend(steps.get(offset++), caseName, "M2");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_DELETE);
        validateASend(steps.get(offset++), caseName, "S1");
        validateMarketSend(steps.get(offset++), caseName, "M3");
        validateASend(steps.get(offset++), caseName, "S2");
        validateMarketSend(steps.get(offset++), caseName, "M4");
        validateBeanSend(steps.get(offset++), caseName, "S3", 100, 200);
        validateMarketSend(steps.get(offset++), caseName, "M5");
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

    private static void validateMarketSend(JsonValue value, String caseName,
                                           String expectedSymbol) {
        JsonObject step = object(value, "SupportMarketDataBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportMarketDataBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload");
        requireFields(payload, "symbol", "price", "volume", "feed");
        if (!expectedSymbol.equals(string(payload, "symbol"))
                || payload.getDouble("price", Double.NaN) != 0.0
                || !payload.get("volume").isNull()
                || !"".equals(string(payload, "feed"))) {
            throw new IllegalArgumentException(
                    "SupportMarketDataBean payload is not pinned for " + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedLongPrimitive, long expectedLongBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "longPrimitive", "longBoxed");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("longPrimitive"), "longPrimitive") != expectedLongPrimitive
                || longInteger(payload.get("longBoxed"), "longBoxed") != expectedLongBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateASend(JsonValue value, String caseName, String expectedId) {
        JsonObject step = object(value, "SupportBean_A step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_A".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_A step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_A payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException("SupportBean_A payload is not pinned for " + caseName);
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
