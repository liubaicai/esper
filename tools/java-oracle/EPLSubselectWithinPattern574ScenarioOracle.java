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
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
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
import java.util.HashSet;
import java.util.Set;

/**
 * Java oracle for EPLSubselectWithinPattern ords 1-4: subqueries within
 * pattern filters and stream filters over the SupportBean_S0/S1/S2 input
 * alphabet.
 *
 * Replays the nine spellings across four executions on one runtime,
 * mirroring the regression-suite harness: the three S* bean event types,
 * internal timer disabled, and the rethrowing exception handler so
 * statement failures surface to the sender thread. Each case deploys the
 * byte-exact EPL script from the scenario (single-statement or the
 * multi-statement create-window/insert/s0 scripts for the named-window
 * spellings), sends the pinned event sequence, records s0 listener
 * deliveries, and undeploys all before the next case.
 *
 * The regression-suite class is not on the oracle classpath, so
 * supportSingleRowFunction is mirrored locally as an always-true UDF
 * registered via configuration (imports entry) — same semantics as the
 * regression implementation.
 */
public final class EPLSubselectWithinPattern574ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-subselect-within-pattern-574";
    private static final String DESCRIPTION =
            "EPLSubselectWithinPattern ords 1-4: subqueries within pattern and stream filters over "
                    + "SupportBean_S0/S1/S2 — correlated exists over keepall (pattern and filter "
                    + "spellings), a followed-by scalar-subquery gate, rolling sum aggregation over "
                    + "length(2), a named-window subquery as UDF argument in a pattern, and the "
                    + "lastevent/named-window in-subquery quartet (including the followed-by "
                    + "scalar-subquery gate correlated to the earlier sp0 tag).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/"
                    + "EPLSubselectWithinPattern.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-3d9d3a714bd642e784bd",
            "java-runtime-9a15ec8213060ac0185c",
            "java-runtime-3c1d7cc144c168f6a0d6",
            "java-runtime-0db35509697665c0960e"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLSubselectCorrelated",
            "EPLSubselectAggregation",
            "EPLSubselectSubqueryAgainstNamedWindowInUDFInPattern",
            "EPLSubselectFilterPatternNamedWindowNoAlias"
    };
    private static final String[] STATIC_IDS = {
            "java-495107e31d1fe86086ab",
            "java-495107e31d1fe86086ab",
            "java-495107e31d1fe86086ab",
            "java-495107e31d1fe86086ab"
    };

    private static final String CASE_PATTERN_EXISTS = "correlated-pattern-exists";
    private static final String CASE_FILTER_EXISTS = "correlated-filter-exists";
    private static final String CASE_FOLLOWED_BY = "correlated-followed-by-scalar";
    private static final String CASE_AGGREGATION = "aggregation";
    private static final String CASE_NW_UDF = "named-window-udf";
    private static final String CASE_PATTERN_LASTEVENT = "noalias-pattern-lastevent";
    private static final String CASE_FILTER_LASTEVENT = "noalias-filter-lastevent";
    private static final String CASE_FILTER_NW = "noalias-filter-named-window";
    private static final String CASE_PATTERN_NW = "noalias-pattern-named-window";

    // Verbatim transcription of EPLSubselectWithinPattern.
    private static final String EPL_PATTERN_EXISTS =
            "@name('s0') select sp1.id as myid from pattern[every sp1=SupportBean_S0(exists "
            + "(select * from SupportBean_S1#keepall as stream1 where stream1.p10 = sp1.p00))]";
    private static final String EPL_FILTER_EXISTS =
            "@name('s0') select id as myid from SupportBean_S0(exists (select stream1.id "
            + "from SupportBean_S1#keepall as stream1 where stream1.p10 = stream0.p00)) as stream0";
    private static final String EPL_FOLLOWED_BY =
            "@name('s0') select sp0.p00||'+'||sp1.p10 as myid from pattern[every sp0=SupportBean_S0 "
            + "-> sp1=SupportBean_S1(p11 = (select stream2.p21 from SupportBean_S2#keepall "
            + "as stream2 where stream2.p20 = sp0.p00))]";
    private static final String EPL_AGGREGATION =
            "@name('s0') select * from SupportBean_S0(id = (select sum(id) from "
            + "SupportBean_S1#length(2)))";
    private static final String EPL_NW_UDF =
            "create window MyWindowSNW#unique(p00)#keepall as SupportBean_S0;\n"
            + "@name('s0') select * from "
            + "pattern[SupportBean_S1(supportSingleRowFunction((select * from MyWindowSNW)))];\n";
    private static final String EPL_PATTERN_LASTEVENT =
            "@name('s0') select s.id as myid from pattern [every s=SupportBean_S0(p00 in "
            + "(select p10 from SupportBean_S1#lastevent))]";
    private static final String EPL_FILTER_LASTEVENT =
            "@name('s0') select id as myid from SupportBean_S0(p00 in "
            + "(select p10 from SupportBean_S1#lastevent))";
    private static final String EPL_FILTER_NW =
            "create window MyS1Window#lastevent as select * from SupportBean_S1;\n"
            + "insert into MyS1Window select * from SupportBean_S1;\n"
            + "@name('s0') select id as myid from SupportBean_S0(p00 in (select p10 from "
            + "MyS1Window))";
    private static final String EPL_PATTERN_NW =
            "create window MyS1Window#lastevent as select * from SupportBean_S1;\n"
            + "insert into MyS1Window select * from SupportBean_S1;\n"
            + "@name('s0') select s.id as myid from pattern [every s=SupportBean_S0(p00 in "
            + "(select p10 from MyS1Window))];\n";

    private static final String[] CASE_NAMES = {
            CASE_PATTERN_EXISTS, CASE_FILTER_EXISTS, CASE_FOLLOWED_BY, CASE_AGGREGATION,
            CASE_NW_UDF, CASE_PATTERN_LASTEVENT, CASE_FILTER_LASTEVENT, CASE_FILTER_NW,
            CASE_PATTERN_NW
    };
    private static final int[] CASE_ORDINALS = {1, 1, 1, 2, 3, 4, 4, 4, 4};
    private static final int[] CASE_RUNTIME_INDEX = {0, 0, 0, 1, 2, 3, 3, 3, 3};
    private static final String[] CASE_EPLS = {
            EPL_PATTERN_EXISTS, EPL_FILTER_EXISTS, EPL_FOLLOWED_BY, EPL_AGGREGATION,
            EPL_NW_UDF, EPL_PATTERN_LASTEVENT, EPL_FILTER_LASTEVENT, EPL_FILTER_NW,
            EPL_PATTERN_NW
    };

    private static final String LISTENED_STATEMENT = "s0";
    private static final int EXPECTED_RECORDS = 19;
    private static final int EXPECTED_STEPS = 108;

    private EPLSubselectWithinPattern574ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLSubselectWithinPattern574ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S2.class);
        configuration.getCompiler().addPlugInSingleRowFunction("supportSingleRowFunction",
                EPLSubselectWithinPattern574ScenarioOracle.class.getName(), "supportSingleRowFunction");
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASE_NAMES) {
                runCase(caseName, configuration, runtime, allSteps, records);
            }
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
    /** Local mirror of the regression supportSingleRowFunction (always true);
     *  the varargs flag matters: Esper's method resolver tries element-type
     *  matching only for methods marked varargs, so a wildcard subquery row
     *  binds as a single Object argument. */
    public static boolean supportSingleRowFunction(Object... values) {
        return true;
    }
    /** Replays one case's steps on the shared runtime; sequence restarts per case. */
    private static void runCase(String caseName, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps,
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
                    // CompilerArguments(configuration) carries the compiler-level
                    // plug-in single-row function registry; the runtime path does
                    // not (ContextHashScenarioOracle precedent).
                    CompilerArguments compilerArgs = new CompilerArguments(configuration);
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
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value instanceof EventBean[]) {
            JsonArray items = new JsonArray();
            for (EventBean item : (EventBean[]) value) {
                items.add(row(item));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean_S0": {
                JsonValue p00 = payload.get("p00");
                SupportBean_S0 bean = p00 == null
                        ? new SupportBean_S0(integer(payload, "id"))
                        : new SupportBean_S0(integer(payload, "id"), string(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S1": {
                JsonValue p11 = payload.get("p11");
                SupportBean_S1 bean;
                if (p11 != null) {
                    bean = new SupportBean_S1(integer(payload, "id"), string(payload, "p10"),
                            string(payload, "p11"));
                } else if (payload.get("p10") != null) {
                    bean = new SupportBean_S1(integer(payload, "id"), string(payload, "p10"));
                } else {
                    bean = new SupportBean_S1(integer(payload, "id"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S2": {
                SupportBean_S2 bean = new SupportBean_S2(integer(payload, "id"),
                        string(payload, "p20"), string(payload, "p21"));
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
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASE_NAMES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (int index = 0; index < CASE_NAMES.length; index++) {
            offset = validateCaseSteps(steps, offset, CASE_NAMES[index], CASE_EPLS[index]);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateCaseSteps(JsonArray steps, int offset, String caseName,
                                         String epl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", epl);
        while (true) {
            JsonObject step = object(steps.get(offset), "case step");
            if ("undeploy-all".equals(string(step, "op"))) {
                requireFields(step, "op", "case");
                if (!caseName.equals(string(step, "case"))) {
                    throw new IllegalArgumentException("undeploy-all is not pinned for "
                            + caseName);
                }
                return offset + 1;
            }
            validateSend(steps.get(offset++), caseName);
        }
    }

    private static void validateSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        String eventType = string(step, "eventType");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !(eventType.equals("SupportBean_S0") || eventType.equals("SupportBean_S1")
                        || eventType.equals("SupportBean_S2"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        switch (eventType) {
            case "SupportBean_S0":
                if (!payload.names().contains("id")) {
                    throw new IllegalArgumentException("S0 payload is not pinned for " + caseName);
                }
                for (Member member : payload) {
                    if (!member.getName().equals("id") && !member.getName().equals("p00")) {
                        throw new IllegalArgumentException("S0 payload is not pinned for "
                                + caseName);
                    }
                }
                break;
            case "SupportBean_S1":
                if (!payload.names().contains("id")) {
                    throw new IllegalArgumentException("S1 payload is not pinned for " + caseName);
                }
                for (Member member : payload) {
                    if (!member.getName().equals("id") && !member.getName().equals("p10")
                            && !member.getName().equals("p11")) {
                        throw new IllegalArgumentException("S1 payload is not pinned for "
                                + caseName);
                    }
                }
                break;
            default:
                requireFields(payload, "id", "p20", "p21");
        }
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

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
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
