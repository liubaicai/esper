import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.EventType;
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
import com.espertech.esper.regressionlib.suite.expr.define.ExprDefineValueParameter;

import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the expr-define-value-parameter parity
 * scenario. Mirrors ExprDefineValueParameter: ExprDefineValueParameterEVEVE
 * (ord 7, the five-parameter declared expression cc deployed on the shared
 * path, then one module deploying s0/s1/s2 over three filtered
 * SupportBean_S0#lastevent streams with the c0 String type asserted),
 * ExprDefineValueParameterCache (ord 9, STATICHOOK: the module-local
 * ExprDefineLocalService variable feeding doit(theString) with the
 * getCalculations().size() assertions pinned as iterator-count records), and
 * ExprDefineValueParameterSubquery (ord 11, the statement-local cc
 * expression fed by two scalar subqueries over SupportBean_S0#lastevent
 * emitting c0=null when no S0 event arrived).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own;
 * every execution ends with undeployAll). Every deploy step compiles the
 * pinned module text exactly like the source's per-call compileDeploy; the
 * eveve module deploy yields the three named statements. Deployed markers
 * emit one record per deploy step. Listener records carry the new-data
 * rows. The types step verifies s0's c0 property type in-process before
 * emitting the pinned record, and the iterator-count steps verify the
 * service's calculations size against the pinned count before emitting the
 * observed count.
 */
public final class ExprDefineValueParameterScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-define-value-parameter";
    private static final String DESCRIPTION =
            "ExprDefineValueParameter slice (ords 7/9/11): eveve deploys @public create"
                    + " expression cc { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 ||"
                    + " c.p00} on the shared path, then one module deploys s0/s1/s2 over"
                    + " three filtered SupportBean_S0#lastevent streams — s0 asserts c0"
                    + " String and the third send fires all three joins emitting"
                    + " 'BxCyA'/'BxAyC'/'CxByA'; cache deploys create variable"
                    + " ExprDefineLocalService myService plus create expression doit"
                    + " {v -> myService.calc(v)} feeding select doit(theString) as c0 —"
                    + " each SupportBean('E10',-1) emits c0=10 and the service's"
                    + " calculations list grows 1 then 2 (STATICHOOK is metadata-only);"
                    + " subquery binds statement-local expression cc {(v1,v2) -> v1||v2}"
                    + " over two scalar subqueries on SupportBean_S0#lastevent — one"
                    + " SupportBean_S1(0) send with no S0 events emits c0=null. Deployed"
                    + " markers pin the module fan-out, listener records carry the"
                    + " new-data rows, the types record pins the asserted c0 String"
                    + " type, and iterator-count records pin the service invocation counts (Java"
                    + " source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/expr/define/ExprDefineValueParameter.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/"
                    + "ExprDefineValueParameter.java";

    private static final String[] CASES = {"eveve", "cache", "subquery"};
    private static final int[] ORDINALS = {7, 9, 11};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d1b67308d74aec6b3092",
            "java-runtime-65eb95bacf881252faa4",
            "java-runtime-45b280bc6b857cf5fa3a",
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDefineValueParameterEVEVE",
            "ExprDefineValueParameterCache",
            "ExprDefineValueParameterSubquery",
    };
    private static final String[] STATIC_IDS = {
            "java-2f93b37cf7c6dc83e2dd",
            "java-c325db71b15aab20ac8e",
            "java-a816df5a959d4e4aa7a2",
    };

    // Transcriptions of ExprDefineValueParameter.java lines 185-194 (eveve),
    // 218-220 (cache) and 53-54 (subquery).
    private static final String EPL_CC =
            "@public create expression cc { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 || c.p00}";
    private static final String EPL_MODULE =
            "@name('s0') select cc(e2, 'x', e3, 'y', e1) as c0 from \n"
                    + "SupportBean_S0(id=1)#lastevent as e1, SupportBean_S0(id=2)#lastevent as e2,"
                    + " SupportBean_S0(id=3)#lastevent as e3;\n"
                    + "@name('s1') select cc(e2, 'x', e3, 'y', e1) as c0 from \n"
                    + "SupportBean_S0(id=1)#lastevent as e3, SupportBean_S0(id=2)#lastevent as e2,"
                    + " SupportBean_S0(id=3)#lastevent as e1;\n"
                    + "@name('s2') select cc(e1, 'x', e2, 'y', e3) as c0 from \n"
                    + "SupportBean_S0(id=1)#lastevent as e3, SupportBean_S0(id=2)#lastevent as e2,"
                    + " SupportBean_S0(id=3)#lastevent as e1;\n";
    private static final String EPL_CACHE_MODULE =
            "create variable ExprDefineLocalService myService = new ExprDefineLocalService();\n"
                    + "create expression doit {v -> myService.calc(v)};\n"
                    + "@name('s0') select doit(theString) as c0 from SupportBean;\n";
    private static final String EPL_SUBQUERY =
            "@name('s0') expression cc { (v1, v2) -> v1 || v2} "
                    + "select cc((select p00 from SupportBean_S0#lastevent),"
                    + " (select p01 from SupportBean_S0#lastevent)) as c0 from SupportBean_S1";

    private static final String[] CASE_EPLS = {
            EPL_CC + ";\n" + EPL_MODULE,
            EPL_CACHE_MODULE,
            EPL_SUBQUERY + ";\n",
    };

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+types+listener; @public create expression cc"
                    + " { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 || c.p00} deploys on the"
                    + " shared path, then one module deploys s0/s1/s2 over three filtered"
                    + " SupportBean_S0#lastevent streams; s0 asserts c0 String; S0(1,'A') and"
                    + " S0(3,'C') stay silent while S0(2,'B') completes the join and fires"
                    + " s0='BxCyA', s1='BxAyC', s2='CxByA'",
            "deployed+listener+iterator-count; create variable"
                    + " ExprDefineLocalService myService plus create expression doit"
                    + " {v -> myService.calc(v)} feed select doit(theString) as c0: each"
                    + " SupportBean('E10',-1) emits c0=10 and the service's calculations list"
                    + " grows 1 then 2 (STATICHOOK: the services registry is metadata-only)",
            "deployed+listener; statement-local expression cc"
                    + " { (v1, v2) -> v1 || v2} receives two scalar subqueries over"
                    + " SupportBean_S0#lastevent projecting p00 and p01; one"
                    + " SupportBean_S1(0) send with no S0 events emits c0=null (null||null)",
    };

    private static final int[] EXPECTED_CASE_RECORDS = {6, 5, 2};
    private static final int EXPECTED_RECORDS = 13;
    private static final int EXPECTED_STEPS = 23;

    private ExprDefineValueParameterScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDefineValueParameterScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            int before = records.size();
            runCase(index, allSteps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASES[index] + " emitted "
                        + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
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

    /** Replays one case's steps on a fresh runtime (one runtime per Java execution). */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        // Mirrors TestSuiteExprDefine.configure: the suite imports the
        // service class so the unqualified create-variable type resolves.
        configuration.getCommon().addImport(ExprDefineValueParameter.ExprDefineLocalService.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        Set<String> deployedLabels = new HashSet<>();
        List<EPCompiled> deployedModules = new ArrayList<>();
        try {
            if ("cache".equals(caseName)) {
                // Mirrors ExprDefineLocalService.services.clear() ahead of
                // the module deploy that constructs the service instance.
                services().clear();
            }
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String label = string(step, "statement");
                        CompilerArguments compilerArgs =
                                new CompilerArguments(configuration);
                        for (EPCompiled deployed : deployedModules) {
                            compilerArgs.getPath().add(deployed);
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            statementsByName.put(statement.getName(), statement);
                            if (listened(caseName, statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                        }
                        deployedLabels.add(label);
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployedLabels.contains(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "types":
                        typesStep(caseName, statementsByName, string(step, "statement"),
                                records, runtime);
                        break;
                    case "iterator-count": {
                        // Mirrors assertEquals(n, service.getCalculations().size()):
                        // the step's declared count is the Java-asserted value
                        // and the record carries the observed size.
                        JsonValue declared = step.get("count");
                        int observed = services().get(0).getCalculations().size();
                        if (declared == null || !declared.isNumber()
                                || declared.asInt() != observed) {
                            throw new IllegalStateException("iterator-count drift for "
                                    + caseName + ": declared "
                                    + (declared == null ? "<none>" : declared.toString())
                                    + " observed " + observed);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "iterator-count");
                        record.add("statement", string(step, "statement"));
                        record.add("count", observed);
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        deployedLabels.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /** Statements that carry the recording listener per case. */
    private static boolean listened(String caseName, String statement) {
        switch (caseName) {
            case "eveve":
                return "s0".equals(statement) || "s1".equals(statement)
                        || "s2".equals(statement);
            case "cache":
            case "subquery":
                return "s0".equals(statement);
            default:
                return false;
        }
    }

    /**
     * The execution's static services registry is package-private inside the
     * suite class; the oracle reads it reflectively exactly like the
     * execution's services.get(0).getCalculations() assertions.
     */
    @SuppressWarnings("unchecked")
    private static List<ExprDefineValueParameter.ExprDefineLocalService> services()
            throws Exception {
        Field field = ExprDefineValueParameter.ExprDefineLocalService.class
                .getDeclaredField("services");
        field.setAccessible(true);
        return (List<ExprDefineValueParameter.ExprDefineLocalService>) field.get(null);
    }

    /**
     * Emits a {"operation":"types"} record carrying exactly the
     * Java-asserted event-type surface for the statement: EVEVE's
     * assertTypeExpected(env, String.class) pins c0 String on s0. The actual
     * event type is verified against the pinned surface before recording so
     * a drift fails the oracle (env.assertStatement semantics).
     */
    private static void typesStep(String caseName, Map<String, EPStatement> statements,
                                  String label, JsonArray records, EPRuntime runtime) {
        if (!"eveve".equals(caseName) || !"s0".equals(label)) {
            throw new IllegalStateException("types statement " + label
                    + " is not pinned in case " + caseName);
        }
        EPStatement statement = statements.get(label);
        if (statement == null) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        EventType eventType = statement.getEventType();
        Class<?> propertyType = eventType.getPropertyType("c0");
        String actual = propertyType == null ? "null" : propertyType.getSimpleName();
        if (!"String".equals(actual)) {
            throw new IllegalStateException("property type drift for " + caseName
                    + "/s0.c0: expected String got " + actual);
        }
        JsonObject pinned = new JsonObject();
        pinned.add("c0", "String");
        JsonObject value = new JsonObject();
        value.add("properties", pinned);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                if (payload.get("theString") != null) {
                    bean.setTheString(payload.get("theString").asString());
                }
                if (payload.get("intPrimitive") != null) {
                    bean.setIntPrimitive(integer(payload, "intPrimitive"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                int id = integer(payload, "id");
                SupportBean_S0 bean = payload.get("p00") != null
                        ? new SupportBean_S0(id, payload.get("p00").asString())
                        : new SupportBean_S0(id);
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S1": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S1(integer(payload, "id")), type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
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
            array.add(row(event));
        }
        return array;
    }

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
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, EventBean fragments as nested row objects,
     * Java arrays as JSON arrays and collections as JSON arrays — the same
     * shapes the Go normalizer emits.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object item : (Collection<?>) value) {
                array.add(normalize(item));
            }
            return array;
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
        validateStringArray(scenario.get("javaFlags"), new String[]{"STATICHOOK"},
                "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
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
        offset = validateEveveCase(steps, offset);
        offset = validateCacheCase(steps, offset);
        offset = validateSubqueryCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact eveve step sequence mirroring ExprDefineValueParameterEVEVE
     * lines 183-205: the cc expression deploy on the shared path, the
     * three-statement module deploy, the c0 type assertion, the three S0
     * sends (the join fires on the third) and undeploy-all.
     */
    private static int validateEveveCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[0]);
        validateDeploy(steps.get(offset++), CASES[0], "cc", EPL_CC);
        validateDeployed(steps.get(offset++), CASES[0], "cc");
        validateDeploy(steps.get(offset++), CASES[0], "module", EPL_MODULE);
        validateDeployed(steps.get(offset++), CASES[0], "module");
        validateTypes(steps.get(offset++), CASES[0], "s0");
        validateS0Send(steps.get(offset++), CASES[0], 1, "A");
        validateS0Send(steps.get(offset++), CASES[0], 3, "C");
        validateS0Send(steps.get(offset++), CASES[0], 2, "B");
        validateUndeployAll(steps.get(offset++), CASES[0]);
        return offset;
    }

    /**
     * Exact cache step sequence mirroring ExprDefineValueParameterCache
     * lines 218-234: the module deploy (variable + expression + select),
     * the two E10 sends each followed by the calculations-size assertion,
     * and undeploy-all.
     */
    private static int validateCacheCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[1]);
        validateDeploy(steps.get(offset++), CASES[1], "module", EPL_CACHE_MODULE);
        validateDeployed(steps.get(offset++), CASES[1], "module");
        validateBeanSend(steps.get(offset++), CASES[1], "E10", -1);
        validateServiceCount(steps.get(offset++), CASES[1], 1);
        validateBeanSend(steps.get(offset++), CASES[1], "E10", -1);
        validateServiceCount(steps.get(offset++), CASES[1], 2);
        validateUndeployAll(steps.get(offset++), CASES[1]);
        return offset;
    }

    /**
     * Exact subquery step sequence mirroring
     * ExprDefineValueParameterSubquery lines 53-60: the statement-local
     * expression select deploy, the single S1(0) send and undeploy-all.
     */
    private static int validateSubqueryCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[2]);
        validateDeploy(steps.get(offset++), CASES[2], "s0", EPL_SUBQUERY);
        validateDeployed(steps.get(offset++), CASES[2], "s0");
        validateS1Send(steps.get(offset++), CASES[2], 0);
        validateUndeployAll(steps.get(offset++), CASES[2]);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName,
                                       String expectedStatement, String expectedEpl) {
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

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateTypes(JsonValue value, String caseName,
                                      String expectedStatement) {
        JsonObject step = object(value, "types step");
        requireFields(step, "op", "case", "statement");
        if (!"types".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("types step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateServiceCount(JsonValue value, String caseName,
                                             int expectedCount) {
        JsonObject step = object(value, "iterator-count step");
        requireFields(step, "op", "case", "statement", "count");
        if (!"iterator-count".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"s0".equals(string(step, "statement"))
                || integer(step, "count") != expectedCount) {
            throw new IllegalArgumentException("iterator-count step is not pinned for "
                    + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String theString, int intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(payload.get("theString").asString())
                || payload.get("intPrimitive").asInt() != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, int id, String p00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (payload.get("id").asInt() != id || !p00.equals(payload.get("p00").asString())) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateS1Send(JsonValue value, String caseName, int id) {
        JsonObject step = object(value, "SupportBean_S1 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S1".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S1 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S1 payload");
        requireFields(payload, "id");
        if (payload.get("id").asInt() != id) {
            throw new IllegalArgumentException("SupportBean_S1 id is not pinned for " + caseName);
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
                || !new HashSet<>(object.names()).equals(
                        new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields "
                    + (object == null ? "<null>" : object.names()) + ", expected "
                    + Arrays.toString(expectedNames));
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
