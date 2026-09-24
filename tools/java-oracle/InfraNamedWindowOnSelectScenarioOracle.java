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
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the infra-named-window-on-select parity
 * scenario. Mirrors InfraNamedWindowOnSelect ordinals 0-2:
 * InfraNamedWindowOnSelectSimple (an on-S0 delete observed through the keepall
 * window statement's new/old rows), InfraNamedWindowOnSelectSceneTwo (an on-A
 * insert-into routing the ordered window rows into MyStream for a select-*
 * consumer, plus an I% direct insert and an on-B delete-all), and
 * InfraNamedWindowOnSelectWPattern (an on-pattern select emitting one
 * join-shaped row, stream_0 the retained Z bean and stream_1 the pattern
 * match, when the every-A -> B pattern fires).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own).
 * Deploy steps queue per module: simple and wpattern deploy three
 * single-statement modules exactly like the source's three compileDeploy
 * calls, while scene-two compiles its five leading statements as ONE module
 * (the suite's single compileDeploy) and the delete statement as a second
 * module. Deployed markers emit one record per statement label. Listeners
 * attach to 'create' (simple), 'select' and 'consumer' (scene-two) and 's0'
 * (wpattern). Snapshot steps iterate the create statement in engine order.
 * Sends dispatch SupportBean/SupportBean_S0 beans and SupportBean_A/_B maps
 * (the regression-lib beans are not on the oracle classpath; only the type
 * name participates in the pinned EPL).
 */
public final class InfraNamedWindowOnSelectScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-on-select";
    private static final String DESCRIPTION =
            "InfraNamedWindowOnSelect named-window on-trigger select slice (ords 0-2): an on-S0"
                    + " delete observed through the keepall window statement's new/old rows"
                    + " (simple, ord 0), an on-A insert-into that routes the ordered window rows"
                    + " into MyStream for a select-* consumer plus an I% direct insert and an"
                    + " on-B delete-all (scene-two, ord 1), and an on-pattern select that emits"
                    + " a join-shaped row (stream_0 the retained Z bean, stream_1 the pattern"
                    + " match) when the every-A -> B pattern fires (wpattern, ord 2)."
                    + " Deployed markers pin the module fan-out; listener records carry the full"
                    + " SupportBean row and ordered iterator snapshots pin the window contents"
                    + " (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/namedwindow/InfraNamedWindowOnSelect.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnSelect.java";

    private static final String[] CASES = {"simple", "scene-two", "wpattern"};
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-8a348801e82d24a6c755",
            "java-runtime-1bbd8705a7bbea1db0cd",
            "java-runtime-caeff76ee56490d832ea",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNamedWindowOnSelectSimple",
            "InfraNamedWindowOnSelectSceneTwo",
            "InfraNamedWindowOnSelectWPattern",
    };
    private static final String[] STATIC_IDS = {
            "java-763e7086c0fdad5f4c49",
            "java-0f1cbcf0f43473325b2b",
            "java-4201486cef18d7d66843",
    };
    private static final String[] CASE_OBSERVATIONS = {
            "listener+deployed; three separately deployed statements (public keepall window,"
                    + " wildcard insert, on-S0 delete where intPrimitive = id): the E1 insert"
                    + " arrives as the window statement's new row and the S0(1) trigger removes"
                    + " it as the window statement's old row",
            "listener+snapshot+deployed; one five-statement module (keepall window, E% insert,"
                    + " on-A insert-into MyStream selecting mywin.* ordered by theString asc,"
                    + " select-* consumer, I% insert into MyStream) plus a second module's on-B"
                    + " delete-all: A1 fires one select row and one consumer row for E1, I2"
                    + " reaches the consumer only, A2 fires the two-row ordered batch [E1,E3] on"
                    + " select and two separate consumer events, and B1 empties the window so"
                    + " A3 fires nothing",
            "listener+deployed; a keepall window pre-populated by the Z-filtered insert feeds"
                    + " an on-pattern select: the A(1) event arms every e ="
                    + " SupportBean(theString = 'A'), the B(1) event matches intPrimitive ="
                    + " e.intPrimitive, and s0 emits one join-shaped row whose stream_0 column"
                    + " is the retained Z bean and stream_1 the pattern match",
    };
    private static final String[] CASE_EPLS = {
            "@Name('create') @public create window MyWindow.win:keepall() as SupportBean;\n"
                    + "@Name('insert') insert into MyWindow select * from SupportBean;\n"
                    + "@Name('delete') on SupportBean_S0 delete from MyWindow"
                    + " where intPrimitive = id;\n",
            "@name('create') @public create window MyWindow#keepall as select * from SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean(theString like 'E%');\n"
                    + "@name('select') on SupportBean_A insert into MyStream select mywin.*"
                    + " from MyWindow as mywin order by theString asc;\n"
                    + "@name('consumer') select * from MyStream;\n"
                    + "insert into MyStream select * from SupportBean(theString like 'I%');\n"
                    + "@name('delete') on SupportBean_B delete from MyWindow;\n",
            "@public create window MyWindow.win:keepall() as SupportBean;\n"
                    + "insert into MyWindow select * from SupportBean(theString = 'Z');\n"
                    + "@Name('s0') on pattern[every e = SupportBean(theString = 'A')"
                    + " -> SupportBean(intPrimitive = e.intPrimitive)] select * from MyWindow;\n",
    };

    // Transcriptions of InfraNamedWindowOnSelect lines 50/53/56 (simple),
    // 81-85 plus 134 (scene-two) and 149/150/153 (wpattern). The create
    // statements carry the source's own @public annotation; the scenario
    // lists one deploy step per statement while this oracle compiles each
    // module exactly like the suite's compileDeploy calls.
    private static final String EPL_SIMPLE_CREATE =
            "@Name('create') @public create window MyWindow.win:keepall() as SupportBean";
    private static final String EPL_SIMPLE_INSERT =
            "@Name('insert') insert into MyWindow select * from SupportBean";
    private static final String EPL_SIMPLE_DELETE =
            "@Name('delete') on SupportBean_S0 delete from MyWindow where intPrimitive = id";

    private static final String EPL_SCENE_CREATE =
            "@name('create') @public create window MyWindow#keepall as select * from SupportBean";
    private static final String EPL_SCENE_INSERT =
            "insert into MyWindow select * from SupportBean(theString like 'E%')";
    private static final String EPL_SCENE_SELECT =
            "@name('select') on SupportBean_A insert into MyStream select mywin.*"
                    + " from MyWindow as mywin order by theString asc";
    private static final String EPL_SCENE_CONSUMER = "@name('consumer') select * from MyStream";
    private static final String EPL_SCENE_INSERT_I =
            "insert into MyStream select * from SupportBean(theString like 'I%')";
    private static final String EPL_SCENE_DELETE =
            "@name('delete') on SupportBean_B delete from MyWindow";

    private static final String EPL_PATTERN_CREATE =
            "@public create window MyWindow.win:keepall() as SupportBean";
    private static final String EPL_PATTERN_INSERT =
            "insert into MyWindow select * from SupportBean(theString = 'Z')";
    private static final String EPL_PATTERN_S0 =
            "@Name('s0') on pattern[every e = SupportBean(theString = 'A')"
                    + " -> SupportBean(intPrimitive = e.intPrimitive)] select * from MyWindow";

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out. Simple and wpattern deploy three single-statement modules;
     * scene-two compiles create/insert/select/consumer/insert-i as one module
     * (the suite's single compileDeploy) and the delete as a second module.
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> simpleModules = new HashMap<>();
        simpleModules.put("create", 0);
        simpleModules.put("insert", 1);
        simpleModules.put("delete", 2);
        modules.put("simple", simpleModules);
        Map<String, Integer> sceneModules = new HashMap<>();
        sceneModules.put("create", 0);
        sceneModules.put("insert", 0);
        sceneModules.put("select", 0);
        sceneModules.put("consumer", 0);
        sceneModules.put("insert-i", 0);
        sceneModules.put("delete", 1);
        modules.put("scene-two", sceneModules);
        Map<String, Integer> patternModules = new HashMap<>();
        patternModules.put("create", 0);
        patternModules.put("insert", 1);
        patternModules.put("s0", 2);
        modules.put("wpattern", patternModules);
        MODULE_KEYS = Collections.unmodifiableMap(modules);

        Map<String, Set<String>> listened = new HashMap<>();
        listened.put("simple", new HashSet<>(Collections.singletonList("create")));
        listened.put("scene-two", new HashSet<>(Arrays.asList("select", "consumer")));
        listened.put("wpattern", new HashSet<>(Collections.singletonList("s0")));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    private static final int[] EXPECTED_CASE_RECORDS = {5, 15, 4};
    private static final int EXPECTED_RECORDS = 24;
    private static final int EXPECTED_STEPS = 45;

    private InfraNamedWindowOnSelectScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOnSelectScenarioOracle <scenario.json>");
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
        if ("scene-two".equals(caseName)) {
            configuration.getCommon().addEventType("SupportBean_A", idSchema());
            configuration.getCommon().addEventType("SupportBean_B", idSchema());
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
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
                                statementsByName, sequences, records);
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
                            statementsByName, sequences, records);
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
                    case "snapshot": {
                        EPStatement statement = statementsByName.get(string(step, "statement"));
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "snapshot targets unknown statement in case " + caseName);
                        }
                        if (!"ordered".equals(string(step, "mode"))) {
                            throw new IllegalStateException(
                                    "unsupported snapshot mode in case " + caseName);
                        }
                        records.add(snapshot(runtime, statement, caseName));
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
                }
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
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
     * in step order (exactly the Java source text of the suite's module), then
     * registers the statements by scenario label and attaches the case's
     * listeners. CompilerArguments(runtime.getRuntimePath()) carries the prior
     * deployments' public types so later modules resolve the @public window.
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
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
            if (listened.contains(label)) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    /** Map event type for the regression-lib SupportBean_A/_B triggers (id only). */
    private static Map<String, Object> idSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("id", String.class);
        return schema;
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
            case "SupportBean_A":
            case "SupportBean_B": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", string(payload, "id"));
                runtime.getEventService().sendEventMap(event, type);
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

    /**
     * Snapshot of the statement iterator: one record with sequence 0 holding
     * the engine iterator order (keepall iteration is insertion order, the
     * order the Java exact-order iterator asserts pin). An empty iterator
     * omits the new array, matching the Go normalizer's omitempty rendering.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            rows.add(row(iterator.next()));
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        return record;
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
        int offset = validateSimpleCase(steps, 0);
        offset = validateSceneTwoCase(steps, offset);
        offset = validateWPatternCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact simple step sequence mirroring InfraNamedWindowOnSelectSimple
     * lines 50-69: three single-statement module deploys (window create with
     * the listener, wildcard insert, on-S0 delete), the E1 bean send, the
     * S0(1) trigger send and the case-end undeploy-all.
     */
    private static int validateSimpleCase(JsonArray steps, int offset) {
        String caseName = CASES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SIMPLE_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SIMPLE_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SIMPLE_DELETE);
        validateDeployed(steps.get(offset++), caseName, "delete");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateS0Send(steps.get(offset++), caseName, 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact scene-two step sequence mirroring InfraNamedWindowOnSelectSceneTwo
     * lines 81-142: the five-statement module deploy with its deployed
     * markers, the E1/A1/I2/E3/A2 send wave with the three create iterator
     * snapshots the source asserts, the second module's on-B delete deploy,
     * the B1/A3 sends and the case-end undeploy-all that mirrors the
     * delete-then-create module teardown.
     */
    private static int validateSceneTwoCase(JsonArray steps, int offset) {
        String caseName = CASES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SCENE_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SCENE_INSERT);
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SCENE_SELECT);
        validateDeploy(steps.get(offset++), caseName, "consumer", EPL_SCENE_CONSUMER);
        validateDeploy(steps.get(offset++), caseName, "insert-i", EPL_SCENE_INSERT_I);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeployed(steps.get(offset++), caseName, "select");
        validateDeployed(steps.get(offset++), caseName, "consumer");
        validateDeployed(steps.get(offset++), caseName, "insert-i");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateBeanSend(steps.get(offset++), caseName, "I2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        validateSnapshot(steps.get(offset++), caseName, "create");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A2");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_SCENE_DELETE);
        validateDeployed(steps.get(offset++), caseName, "delete");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_B", "B1");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A3");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact wpattern step sequence mirroring InfraNamedWindowOnSelectWPattern
     * lines 149-162: the window create and Z-filtered insert modules, the Z
     * pre-population send, the on-pattern select module, and the A(1)/B(1)
     * sends that arm then fire the followed-by.
     */
    private static int validateWPatternCase(JsonArray steps, int offset) {
        String caseName = CASES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_PATTERN_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_PATTERN_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "Z", 0);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_PATTERN_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "A", 1);
        validateBeanSend(steps.get(offset++), caseName, "B", 1);
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

    private static void validateIdSend(JsonValue value, String caseName,
                                       String eventType, String id) {
        JsonObject step = object(value, eventType + " step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(eventType + " step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), eventType + " payload");
        requireFields(payload, "id");
        if (!id.equals(payload.get("id").asString())) {
            throw new IllegalArgumentException(eventType + " payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"ordered".equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
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
