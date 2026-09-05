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
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Java oracle for EPLSubselectWithinFilter and EPLSubselectWithinHaving:
 * subqueries within stream filters and within a grouped having clause.
 *
 * Replays the four executions on one runtime, mirroring the regression-suite
 * harness: SupportBean_S0, SupportBean_S1, and SupportBean event types plus a
 * local mirror of the regression SupportMaxAmountEvent (regression-lib is not
 * on the oracle classpath), internal timer disabled, and the rethrowing
 * exception handler so statement failures surface to the sender thread.  The
 * two within-filter cases deploy one inlined_class statement whose where
 * clause correlates a subquery over SupportBean_S1#keepall with the
 * enclosing S0 event through MyUtil.compareIt; the two within-having cases
 * create MyInfra (named window or table), seed it from SupportMaxAmountEvent
 * inserts, and gate a grouped length-window sum with a correlated subquery
 * against MyInfra.
 */
public final class EPLSubselectWithinFilterHavingScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-subselect-within-filter-having";
    private static final String DESCRIPTION =
            "EPLSubselectWithinFilter subqueries within stream filters with an inlined-class UDF (exists and "
                    + "scalar-row correlation over SupportBean_S1#keepall) plus EPLSubselectWithinHaving grouped "
                    + "having gated by a correlated subquery over MyInfra, replayed once as a named window and once "
                    + "as a table (second oracle source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/epl/subselect/EPLSubselectWithinHaving.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/"
                    + "EPLSubselectWithinFilter.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-844ff5e51e235151dccc",
            "java-runtime-1405e55b4e78331c4ec7",
            "java-runtime-5249f7c19ee8d26d8f30",
            "java-runtime-4b4ca324ee2a41b8ccd6"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLSubselectWithinFilterExistsWhereAndUDF",
            "EPLSubselectWithinFilterRowWhereAndUDF",
            "EPLSubselectHavingSubselectWithGroupBy{namedWindow=true}",
            "EPLSubselectHavingSubselectWithGroupBy{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-e0811e7acc03cb5daab2",
            "java-09e87f283cdd09c8434f",
            "java-d69f172401e51113c81d",
            "java-d69f172401e51113c81d"
    };

    private static final String CASE_FILTER_EXISTS = "within-filter-exists";
    private static final String CASE_FILTER_ROW = "within-filter-row";
    private static final String CASE_HAVING_NW = "within-having-named-window";
    private static final String CASE_HAVING_TABLE = "within-having-table";

    // Verbatim transcription of EPLSubselectWithinFilter lines 31-35.
    private static final String EPL_FILTER_EXISTS =
            "@name('s0') "
            + "inlined_class \"\"\"\n"
            + "  public class MyUtil { public static boolean compareIt(String one, String two) { return one.equals(two); } }\n"
            + "\"\"\" \n"
            + "select * from SupportBean_S0(exists (select * from SupportBean_S1#keepall where MyUtil.compareIt(s0.p00,p10))) as s0;\n";
    // Verbatim transcription of EPLSubselectWithinFilter lines 51-55.
    private static final String EPL_FILTER_ROW =
            "@name('s0') "
            + "inlined_class \"\"\"\n"
            + "  public class MyUtil { public static boolean compareIt(String one, String two) { return one.equals(two); } }\n"
            + "\"\"\" \n"
            + "select * from SupportBean_S0('abc' = (select p11 from SupportBean_S1#keepall where MyUtil.compareIt(s0.p00,p10))) as s0;\n";
    // The regression deploy lacks a statement name; @name('create') only pins a
    // stable name (InfraNWTableSubqueryScenarioOracle precedent).
    private static final String EPL_CREATE_NW =
            "@name('create') @public create window MyInfra#unique(key) as SupportMaxAmountEvent";
    private static final String EPL_CREATE_TABLE =
            "@name('create') @public create table MyInfra(key string primary key, maxAmount double)";
    private static final String EPL_INSERT =
            "@name('insert') insert into MyInfra select * from SupportMaxAmountEvent";
    // Verbatim transcription of EPLSubselectWithinHaving lines 45-48.
    private static final String EPL_HAVING =
            "@name('s0') select theString as c0, sum(intPrimitive) as c1 "
            + "from SupportBean#groupwin(theString)#length(2) as sb "
            + "group by theString "
            + "having sum(intPrimitive) > (select maxAmount from MyInfra as mw where sb.theString = mw.key)";

    private static final String LISTENED_STATEMENT = "s0";
    private static final int EXPECTED_RECORDS = 14;
    private static final int EXPECTED_STEPS = 63;

    private EPLSubselectWithinFilterHavingScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLSubselectWithinFilterHavingScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportMaxAmountEvent.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_FILTER_EXISTS, runtime, allSteps, records);
            runCase(CASE_FILTER_ROW, runtime, allSteps, records);
            runCase(CASE_HAVING_NW, runtime, allSteps, records);
            runCase(CASE_HAVING_TABLE, runtime, allSteps, records);
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
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        if (LISTENED_STATEMENT.equals(statement.getName())) {
                            statement.addListener(listener(caseName, seq, records));
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

    /** Listener emitting one record per new row; an old stream is a drift failure. */
    private static UpdateListener listener(String caseName, int[] seq, JsonArray records) {
        return (newEvents, oldEvents, statement, runtime) -> {
            if (oldEvents != null && oldEvents.length > 0) {
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
                newRows.add(row(event));
                record.add("new", newRows);
                records.add(record);
            }
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
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
            case "SupportBean_S0": {
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"), string(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S1": {
                JsonValue p11 = payload.get("p11");
                SupportBean_S1 bean = p11 == null
                        ? new SupportBean_S1(integer(payload, "id"), string(payload, "p10"))
                        : new SupportBean_S1(integer(payload, "id"), string(payload, "p10"),
                                string(payload, "p11"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportMaxAmountEvent": {
                SupportMaxAmountEvent bean = new SupportMaxAmountEvent(
                        string(payload, "key"), payload.getDouble("maxAmount", Double.NaN));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean": {
                SupportBean bean = new SupportBean(
                        string(payload, "theString"), integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
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
                CASE_FILTER_EXISTS, CASE_FILTER_ROW, CASE_HAVING_NW, CASE_HAVING_TABLE};
        int[] expectedOrdinals = {0, 1, 0, 1};
        String[] expectedEpls = {EPL_FILTER_EXISTS, EPL_FILTER_ROW, EPL_HAVING, EPL_HAVING};
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
        offset = validateFilterExistsCase(steps, offset);
        offset = validateFilterRowCase(steps, offset);
        offset = validateHavingCase(steps, offset, CASE_HAVING_NW, EPL_CREATE_NW);
        offset = validateHavingCase(steps, offset, CASE_HAVING_TABLE, EPL_CREATE_TABLE);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateFilterExistsCase(JsonArray steps, int offset) {
        String caseName = CASE_FILTER_EXISTS;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FILTER_EXISTS);
        validateS0Send(steps.get(offset++), caseName, 1, "a");
        validateS1Send(steps.get(offset++), caseName, 10, "x", false);
        validateS0Send(steps.get(offset++), caseName, 2, "a");
        validateS1Send(steps.get(offset++), caseName, 11, "a", false);
        validateS0Send(steps.get(offset++), caseName, 3, "a");
        validateS0Send(steps.get(offset++), caseName, 4, "x");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateFilterRowCase(JsonArray steps, int offset) {
        String caseName = CASE_FILTER_ROW;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FILTER_ROW);
        validateS0Send(steps.get(offset++), caseName, 1, "a");
        validateS1Send(steps.get(offset++), caseName, 10, "x", true);
        validateS0Send(steps.get(offset++), caseName, 1, "a");
        validateS1Send(steps.get(offset++), caseName, 11, "a", true);
        validateS0Send(steps.get(offset++), caseName, 3, "a");
        validateS0Send(steps.get(offset++), caseName, 4, "x");
        validateS0Send(steps.get(offset++), caseName, 5, "y");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateHavingCase(JsonArray steps, int offset, String caseName,
                                          String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_HAVING);
        validateMaxAmountSend(steps.get(offset++), caseName, "G1", 10);
        validateMaxAmountSend(steps.get(offset++), caseName, "G2", 20);
        validateMaxAmountSend(steps.get(offset++), caseName, "G3", 30);
        validateBeanSend(steps.get(offset++), caseName, "G1", 5);
        validateBeanSend(steps.get(offset++), caseName, "G2", 19);
        validateBeanSend(steps.get(offset++), caseName, "G3", 28);
        validateBeanSend(steps.get(offset++), caseName, "G2", 2);
        validateBeanSend(steps.get(offset++), caseName, "G2", 18);
        validateBeanSend(steps.get(offset++), caseName, "G1", 4);
        validateBeanSend(steps.get(offset++), caseName, "G3", 2);
        validateBeanSend(steps.get(offset++), caseName, "G3", 29);
        validateBeanSend(steps.get(offset++), caseName, "G3", 4);
        validateBeanSend(steps.get(offset++), caseName, "G1", 6);
        validateBeanSend(steps.get(offset++), caseName, "G2", 2);
        validateBeanSend(steps.get(offset++), caseName, "G3", 26);
        validateBeanSend(steps.get(offset++), caseName, "G1", 99);
        validateBeanSend(steps.get(offset++), caseName, "G1", 1);
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

    private static void validateS0Send(JsonValue value, String caseName, int expectedId,
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
        if (integer(payload, "id") != expectedId || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    private static void validateS1Send(JsonValue value, String caseName, int expectedId,
                                       String expectedP10, boolean withP11) {
        JsonObject step = object(value, "SupportBean_S1 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S1".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S1 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S1 payload");
        if (withP11) {
            requireFields(payload, "id", "p10", "p11");
        } else {
            requireFields(payload, "id", "p10");
        }
        if (integer(payload, "id") != expectedId
                || !expectedP10.equals(string(payload, "p10"))
                || (withP11 && !"abc".equals(string(payload, "p11")))) {
            throw new IllegalArgumentException("SupportBean_S1 payload is not pinned for " + caseName);
        }
    }

    private static void validateMaxAmountSend(JsonValue value, String caseName, String expectedKey,
                                              double expectedMaxAmount) {
        JsonObject step = object(value, "SupportMaxAmountEvent step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportMaxAmountEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportMaxAmountEvent step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportMaxAmountEvent payload");
        requireFields(payload, "key", "maxAmount");
        if (!expectedKey.equals(string(payload, "key"))
                || payload.getDouble("maxAmount", Double.NaN) != expectedMaxAmount) {
            throw new IllegalArgumentException("SupportMaxAmountEvent payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         int expectedIntPrimitive) {
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
                || integer(payload, "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
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

    /** Local mirror of the regression SupportMaxAmountEvent (regression-lib is not on the oracle classpath). */
    public static final class SupportMaxAmountEvent {
        private final String key;
        private final double maxAmount;

        public SupportMaxAmountEvent(String key, double maxAmount) {
            this.key = key;
            this.maxAmount = maxAmount;
        }

        public String getKey() {
            return key;
        }

        public double getMaxAmount() {
            return maxAmount;
        }
    }
}
