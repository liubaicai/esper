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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the infra-named-window-subquery parity
 * scenario. Mirrors InfraNamedWindowSubquery ordinals 0-2:
 * InfraSubqueryTwoConsumerWindow (an on-window set trigger whose scalar
 * subquery observes the just-inserted #length(1) row and assigns the
 * deployment variable myvar), InfraSubqueryLateConsumerAggregation (a
 * late-deployed select-* consumer filtered by an uncorrelated count(*)
 * subquery; the pre-attach E1/E2 preload stays unobserved and the post-attach
 * E3 insert fires the listener once), and InfraSubqueryWithFilterInParens (an
 * exists-subquery with a parens named-window filter re-evaluated per
 * SupportBean_S0 trigger).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own).
 * Deploy steps queue per module: two-consumer-window and filter-in-parens
 * compile all their statements as ONE module exactly like the source's single
 * compileDeploy call, while late-consumer-aggregation compiles three
 * single-statement modules sharing one path (the @public window resolves
 * across deployments through CompilerArguments(runtime.getRuntimePath())).
 * Deployed markers emit one record per statement label, including the
 * create-variable statement of ord 0. The read-variable step reads myvar
 * through the variable service keyed by the 'assign' statement's deployment
 * id, mirroring env.deploymentId("assign"). Listeners attach to 's0'
 * (late-consumer-aggregation and filter-in-parens) after the deploy returns,
 * so the ord-1 preload output for E1/E2 is never observed.
 */
public final class InfraNamedWindowSubqueryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-subquery";
    private static final String DESCRIPTION =
            "InfraNamedWindowSubquery named-window subquery slice (ords 0-2): an on-window set"
                    + " trigger whose scalar subquery observes the just-inserted #length(1) row"
                    + " and assigns the deployment variable myvar (two-consumer-window, ord 0),"
                    + " a late-deployed select-* consumer whose uncorrelated count(*) subquery"
                    + " filter passes for the post-attach E3 insert while the pre-attach E1/E2"
                    + " preload stays unobserved (late-consumer-aggregation, ord 1), and an"
                    + " exists-subquery with a parens named-window filter re-evaluated live per"
                    + " SupportBean_S0 trigger (filter-in-parens, ord 2). Deployed markers pin"
                    + " the module fan-out; the ord-0 variable record carries the myvar value"
                    + " the Java assertRuntime pins (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "infra/namedwindow/InfraNamedWindowSubquery.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowSubquery.java";

    private static final String[] CASES = {
            "two-consumer-window", "late-consumer-aggregation", "filter-in-parens"};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-901e88676d86ad580af1",
            "java-runtime-4261803b5e9dcef8a867",
            "java-runtime-95adfcdb21f327e1cc4e",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraSubqueryTwoConsumerWindow",
            "InfraSubqueryLateConsumerAggregation",
            "InfraSubqueryWithFilterInParens",
    };
    private static final String[] STATIC_IDS = {
            "java-f8f4ec7164371451341d",
            "java-ff6a2dd6de98a3f1bb81",
            "java-557b2408e6489254dd8e",
    };
    private static final String[] CASE_OBSERVATIONS = {
            "deployed+variable; one four-statement module (length-1 window over (mycount long),"
                    + " constant-1L insert-count, create variable long myvar = 0, on-window assign"
                    + " setting myvar to the scalar mycount subquery): the E1 insert fires the"
                    + " on-set trigger after the window root view holds the row, so the"
                    + " subquery-consumer sees the just-inserted row and myvar reads 1 (a"
                    + " pre-insert evaluation would leave it null)",
            "listener+deployed; three separately deployed statements sharing one path (@public"
                    + " keepall window, wildcard insert, s0 select-* whose where clause is an"
                    + " uncorrelated count(*) subquery over the same window): E1/E2 insert between"
                    + " the insert and s0 deploys so the s0 preload output stays unobserved (the"
                    + " listener attaches after deploy), and the post-attach E3 insert fires the"
                    + " listener once with the full SupportBean row",
            "listener+deployed; one three-statement module (keepall window, wildcard insert, s0"
                    + " selecting exists(select * from MyWindow(theString='E1')) as c0 from"
                    + " SupportBean_S0): each S0(0) trigger re-evaluates the parens-filtered"
                    + " exists-subquery against live window contents, yielding c0 false on the"
                    + " empty window, false after the E2 insert and true after the E1 insert,"
                    + " each as exactly one new event with no old data",
    };
    private static final String[] CASE_EPLS = {
            "\n create window MyWindowTwo#length(1) as (mycount long);\n"
                    + " @Name('insert-count') insert into MyWindowTwo select 1L as mycount"
                    + " from SupportBean;\n"
                    + " create variable long myvar = 0;\n"
                    + " @Name('assign') on MyWindowTwo set myvar ="
                    + " (select mycount from MyWindowTwo);",
            "@public create window MyWindow#keepall as SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean;\n"
                    + "@name('s0') select * from MyWindow where"
                    + " (select count(*) from MyWindow) > 0;\n",
            "create window MyWindow#keepall as SupportBean;\n"
                    + "@name('insert') insert into MyWindow select * from SupportBean;\n"
                    + "@name('s0') select exists (select * from MyWindow(theString='E1')) as c0"
                    + " from SupportBean_S0;\n",
    };

    // Transcriptions of InfraNamedWindowSubquery lines 63-66
    // (two-consumer-window), 79/80/85 (late-consumer-aggregation) and 38-40
    // (filter-in-parens). The ord-0 source module text carries a leading
    // newline before the first statement; per-statement deploy EPLs exclude
    // the source's inter-statement whitespace exactly like the other
    // per-statement scenario slices.
    private static final String EPL_TWO_CREATE =
            "create window MyWindowTwo#length(1) as (mycount long)";
    private static final String EPL_TWO_INSERT_COUNT =
            "@Name('insert-count') insert into MyWindowTwo select 1L as mycount from SupportBean";
    private static final String EPL_TWO_VARIABLE = "create variable long myvar = 0";
    private static final String EPL_TWO_ASSIGN =
            "@Name('assign') on MyWindowTwo set myvar = (select mycount from MyWindowTwo)";

    private static final String EPL_LATE_CREATE =
            "@public create window MyWindow#keepall as SupportBean";
    private static final String EPL_LATE_INSERT = "insert into MyWindow select * from SupportBean";
    private static final String EPL_LATE_S0 =
            "@name('s0') select * from MyWindow where (select count(*) from MyWindow) > 0";

    private static final String EPL_PARENS_CREATE =
            "create window MyWindow#keepall as SupportBean";
    private static final String EPL_PARENS_INSERT =
            "@name('insert') insert into MyWindow select * from SupportBean";
    private static final String EPL_PARENS_S0 =
            "@name('s0') select exists (select * from MyWindow(theString='E1')) as c0"
                    + " from SupportBean_S0";

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out. Two-consumer-window and filter-in-parens compile one module;
     * late-consumer-aggregation compiles three single-statement modules (the
     * suite's three compileDeploy calls sharing one RegressionPath).
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> twoModules = new HashMap<>();
        twoModules.put("create", 0);
        twoModules.put("insert-count", 0);
        twoModules.put("variable", 0);
        twoModules.put("assign", 0);
        modules.put("two-consumer-window", twoModules);
        Map<String, Integer> lateModules = new HashMap<>();
        lateModules.put("create", 0);
        lateModules.put("insert", 1);
        lateModules.put("s0", 2);
        modules.put("late-consumer-aggregation", lateModules);
        Map<String, Integer> parensModules = new HashMap<>();
        parensModules.put("create", 0);
        parensModules.put("insert", 0);
        parensModules.put("s0", 0);
        modules.put("filter-in-parens", parensModules);
        MODULE_KEYS = Collections.unmodifiableMap(modules);

        Map<String, Set<String>> listened = new HashMap<>();
        listened.put("two-consumer-window", Collections.emptySet());
        listened.put("late-consumer-aggregation",
                new HashSet<>(Collections.singletonList("s0")));
        listened.put("filter-in-parens", new HashSet<>(Collections.singletonList("s0")));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    private static final int[] EXPECTED_CASE_RECORDS = {5, 4, 6};
    private static final int EXPECTED_RECORDS = 15;
    private static final int EXPECTED_STEPS = 36;

    private InfraNamedWindowSubqueryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowSubqueryScenarioOracle <scenario.json>");
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
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        Map<String, String> deploymentIdsByLabel = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        int pendingModule = -1;
        try {
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
                if ("deploy".equals(operation)) {
                    String statementName = string(step, "statement");
                    Integer module = MODULE_KEYS.get(caseName).get(statementName);
                    if (module == null) {
                        throw new IllegalStateException("case " + caseName
                                + " deploys unknown statement " + statementName);
                    }
                    if (!pendingStatements.isEmpty() && pendingModule != module.intValue()) {
                        deployModule(runtime, caseName, pendingStatements, pendingEpls,
                                statementsByName, deploymentIdsByLabel, sequences, records);
                        pendingStatements.clear();
                        pendingEpls.clear();
                    }
                    pendingModule = module.intValue();
                    pendingStatements.add(statementName);
                    pendingEpls.add(string(step, "epl"));
                    continue;
                }
                if (!pendingStatements.isEmpty()) {
                    deployModule(runtime, caseName, pendingStatements, pendingEpls,
                            statementsByName, deploymentIdsByLabel, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                switch (operation) {
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statementsByName.containsKey(label)) {
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
                    case "read-variable": {
                        String name = string(step, "name");
                        // env.deploymentId("assign"): the variable is
                        // deployment-private to the module that also carries
                        // the assign statement.
                        String deploymentId = deploymentIdsByLabel.get("assign");
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "read-variable without a deployed assign statement");
                        }
                        Object value = runtime.getVariableService()
                                .getVariableValue(deploymentId, name);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "variable");
                        record.add("sequence", 0);
                        record.add("name", name);
                        if (value == null) {
                            record.add("value", new JsonObject().add("state", "null"));
                        } else {
                            record.add("value", Json.value(((Number) value).longValue()));
                        }
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        deploymentIdsByLabel.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
                }
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, deploymentIdsByLabel, sequences, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (a whitespace-normalized form of the suite's module text),
     * then registers the statements by scenario label and attaches the case's
     * listeners. CompilerArguments(runtime.getRuntimePath()) carries the prior
     * deployments' public types so ord 1's s0 module resolves the @public
     * window. The deployment id is recorded per label so read-variable can
     * resolve the deployment-private myvar through the 'assign' deployment.
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, String> deploymentIdsByLabel,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        EPStatement[] deployed = deployment.getStatements();
        if (deployed.length != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.length + " statements");
        }
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        for (int index = 0; index < statementNames.size(); index++) {
            String label = statementNames.get(index);
            EPStatement statement = deployed[index];
            statementsByName.put(label, statement);
            deploymentIdsByLabel.put(label, deployment.getDeploymentId());
            if (listened.contains(label)) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setIntPrimitive(integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 bean = new SupportBean_S0(integer(payload, "id"));
                runtime.getEventService().sendEventBean(bean, type);
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

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object the Go normalizer also emits. */
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
        int offset = validateTwoConsumerCase(steps, 0);
        offset = validateLateConsumerCase(steps, offset);
        offset = validateParensCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact two-consumer-window step sequence mirroring
     * InfraSubqueryTwoConsumerWindow lines 63-72: the four-statement module
     * deploys (length-1 window, insert-count, create variable, on-set assign),
     * the E1 bean send, the myvar read the Java assertRuntime pins at 1L and
     * the case-end undeploy-all.
     */
    private static int validateTwoConsumerCase(JsonArray steps, int offset) {
        String caseName = CASES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_TWO_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert-count", EPL_TWO_INSERT_COUNT);
        validateDeploy(steps.get(offset++), caseName, "variable", EPL_TWO_VARIABLE);
        validateDeploy(steps.get(offset++), caseName, "assign", EPL_TWO_ASSIGN);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeployed(steps.get(offset++), caseName, "insert-count");
        validateDeployed(steps.get(offset++), caseName, "variable");
        validateDeployed(steps.get(offset++), caseName, "assign");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateReadVariable(steps.get(offset++), caseName, "myvar");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact late-consumer-aggregation step sequence mirroring
     * InfraSubqueryLateConsumerAggregation lines 79-90: the @public window and
     * insert modules, the E1/E2 preload sends, the s0 module with its
     * post-deploy listener attach, the E3 send and the case-end undeploy-all.
     */
    private static int validateLateConsumerCase(JsonArray steps, int offset) {
        String caseName = CASES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LATE_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_LATE_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 1);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_LATE_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E3", 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact filter-in-parens step sequence mirroring
     * InfraSubqueryWithFilterInParens lines 38-51: the three-statement module
     * deploys (keepall window, wildcard insert, s0 exists-subquery), then the
     * interleaved S0(0) trigger and E2/E1 sends whose assertEqualsNew sequence
     * pins c0 false, false, true.
     */
    private static int validateParensCase(JsonArray steps, int offset) {
        String caseName = CASES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_PARENS_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_PARENS_INSERT);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_PARENS_S0);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateS0Send(steps.get(offset++), caseName, 0);
        validateBeanSend(steps.get(offset++), caseName, "E2", 1);
        validateS0Send(steps.get(offset++), caseName, 0);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateS0Send(steps.get(offset++), caseName, 0);
        validateUndeployAll(steps.get(offset++), caseName);
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
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, int id) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (payload.get("id").asInt() != id) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateReadVariable(JsonValue value, String caseName,
                                             String expectedName) {
        JsonObject step = object(value, "read-variable step");
        requireFields(step, "op", "case", "name");
        if (!"read-variable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedName.equals(string(step, "name"))) {
            throw new IllegalArgumentException("read-variable step is not pinned for "
                    + caseName + "/" + expectedName);
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
