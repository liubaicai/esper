import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

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
 * Direct Esper 9.0.0 oracle for the infra-nwtable-event-type parity
 * scenario. Mirrors InfraNWTableEventType ordinals 0-2:
 * InfraNWTableEventTypeInvalid (two tryInvalidCompile probes pinning the
 * window/table name-collision prefixes against an existing event type),
 * InfraNWTableEventTypeDefineFields (the two cycles asserting the s0 event
 * type's positional column types c0=Integer[] and c1=int[] for the keepall
 * window and the unkeyed table), and
 * InfraNWTableEventTypeInsertIntoProtected (the module whose private
 * insert-into routes two Fubar map events into the protected keepall Snafu
 * window, asserted through assertPropsPerRowIterator).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own;
 * the two define-fields cycles share the pinned runtime id but replay on
 * separate runtimes, matching the execution's undeployAll between cycles).
 * Deploy steps queue per module: insert-into-protected compiles its three
 * statements as ONE module exactly like the source's single compileDeploy
 * call (the module declaration is prepended so the module text is
 * verbatim), while the define-fields cases compile one single-statement
 * module each. Deployed markers emit one record per statement label,
 * including the create-schema 'event' statement of ord 2. The build-error
 * steps compile the pinned probe text against the runtime path and record
 * the pinned Java message prefix once the failure message starts with it,
 * mirroring tryInvalidCompile. The types steps read the deployed s0
 * statement's event type and record positional {name,type} entries with
 * Java class simple names (Integer[] for int[], int[] for
 * int[primitive]). The snapshot step iterates the 'window' statement and
 * records the rows in iterator order.
 */
public final class InfraNWTableEventTypeScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-event-type";
    private static final String DESCRIPTION =
            "InfraNWTableEventType named-window/table event-type slice (ords 0-2): two"
                    + " tryInvalidCompile probes pinning the window/table name-collision"
                    + " messages against an existing event type (invalid, ord 0); the two"
                    + " define-fields cycles asserting the s0 event type's positional column"
                    + " types c0=Integer[] (int[]) and c1=int[] (int[primitive]) for the"
                    + " keepall window and unkeyed table variants (define-fields-window/-table,"
                    + " ord 1, one shared runtime); and the protected-window module replaying"
                    + " two Fubar map events through the private insert-into into the keepall"
                    + " Snafu window whose iterator yields foo/bar a:1, b:2 in order"
                    + " (insert-into-protected, ord 2). Compile-error records carry the pinned"
                    + " Java message prefixes; types records carry positional {name,type}"
                    + " entries with Java class names; the snapshot record mirrors"
                    + " assertPropsPerRowIterator (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "infra/nwtable/InfraNWTableEventType.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableEventType.java";

    private static final String[] CASES = {
            "invalid", "define-fields-window", "define-fields-table", "insert-into-protected"};
    private static final int[] ORDINALS = {0, 1, 1, 2};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d3c3a24f11288969df7e",
            "java-runtime-8dc1233d90336dcdb929",
            "java-runtime-14412293edec92196bd9",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableEventTypeInvalid",
            "InfraNWTableEventTypeDefineFields",
            "InfraNWTableEventTypeInsertIntoProtected",
    };
    private static final String[] STATIC_IDS = {
            "java-08b4a7da65c76bf5abdc",
            "java-50c0d9ea894da18ddb54",
            "java-c007ca393634b73ec8b5",
    };
    private static final String[] CASE_RUNTIME_IDS = {
            RUNTIME_IDS[0], RUNTIME_IDS[1], RUNTIME_IDS[1], RUNTIME_IDS[2]};
    private static final String[] CASE_EXECUTIONS = {
            EXECUTION_NAMES[0], EXECUTION_NAMES[1], EXECUTION_NAMES[1], EXECUTION_NAMES[2]};
    private static final String[] CASE_OBSERVATIONS = {
            "compile-error; two tryInvalidCompile probes: create window SchemaOne#keepall"
                    + " colliding with the SchemaOne event type (prefix 'Error starting"
                    + " statement: An event type or schema by name 'SchemaOne' already exists')"
                    + " and create table SchemaTwo colliding with the SchemaTwo event type"
                    + " (prefix 'An event type by name 'SchemaTwo' has already been declared')",
            "deployed+types; @name('s0') @public create window MyInfra#keepall as"
                    + " (c0 int[], c1 int[primitive]): the s0 event type's positional"
                    + " descriptors pin c0=Integer[] and c1=int[]",
            "deployed+types; @name('s0') @public create table MyInfra (c0 int[],"
                    + " c1 int[primitive]): the same positional descriptors pin c0=Integer[]"
                    + " and c1=int[] for the table variant (second cycle of the shared"
                    + " execution)",
            "deployed+snapshot; one three-statement module (public bus map schema"
                    + " Fubar(foo string, bar double), protected keepall window Snafu as"
                    + " Fubar, private insert into Snafu select *): two Fubar map sends route"
                    + " into the window whose iterator yields {foo:a,bar:1},{foo:b,bar:2} in"
                    + " order",
    };

    // Transcriptions of InfraNWTableEventType.java lines 61-62 and 66-67
    // (invalid), 75/76 (define-fields) and 34-37 (insert-into-protected).
    // The ord-0 probe texts keep the source's per-statement trailing
    // newlines; the ord-2 case EPL keeps the module declaration and the
    // doubled @public literal the source writes.
    private static final String EPL_INVALID_WINDOW =
            "create schema SchemaOne as (p0 string);\n"
                    + "create window SchemaOne#keepall as SchemaOne;\n";
    private static final String EPL_INVALID_TABLE =
            "create schema SchemaTwo as (p0 string);\n"
                    + "create table SchemaTwo(c0 int);\n";
    private static final String PREFIX_INVALID_WINDOW =
            "Error starting statement: An event type or schema by name 'SchemaOne' already exists";
    private static final String PREFIX_INVALID_TABLE =
            "An event type by name 'SchemaTwo' has already been declared";

    private static final String EPL_DEFINE_WINDOW =
            "@name('s0') @public create window MyInfra#keepall as (c0 int[], c1 int[primitive])";
    private static final String EPL_DEFINE_TABLE =
            "@name('s0') @public create table MyInfra (c0 int[], c1 int[primitive])";

    private static final String EPL_PROTECTED_EVENT =
            "@name('event') @public @buseventtype @public create map schema Fubar as"
                    + " (foo string, bar double)";
    private static final String EPL_PROTECTED_WINDOW =
            "@name('window') @protected create window Snafu#keepall as Fubar";
    private static final String EPL_PROTECTED_INSERT =
            "@name('insert') @private insert into Snafu select * from Fubar";
    private static final String EPL_PROTECTED_MODULE =
            "module test;\n" + EPL_PROTECTED_EVENT + ";\n" + EPL_PROTECTED_WINDOW + ";\n"
                    + EPL_PROTECTED_INSERT + ";\n";

    private static final String[] CASE_EPLS = {
            EPL_INVALID_WINDOW + EPL_INVALID_TABLE,
            EPL_DEFINE_WINDOW,
            EPL_DEFINE_TABLE,
            EPL_PROTECTED_MODULE,
    };

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner can map each onto one plan, while this oracle reproduces the Java
     * fan-out. Insert-into-protected compiles one three-statement module (the
     * module declaration is prepended at compile time); the define-fields
     * cases compile one single-statement module each.
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    static {
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        modules.put("invalid", Collections.emptyMap());
        Map<String, Integer> windowModule = new HashMap<>();
        windowModule.put("s0", 0);
        modules.put("define-fields-window", windowModule);
        Map<String, Integer> tableModule = new HashMap<>();
        tableModule.put("s0", 0);
        modules.put("define-fields-table", tableModule);
        Map<String, Integer> protectedModule = new HashMap<>();
        protectedModule.put("event", 0);
        protectedModule.put("window", 0);
        protectedModule.put("insert", 0);
        modules.put("insert-into-protected", protectedModule);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
    }

    private static final int[] EXPECTED_CASE_RECORDS = {2, 2, 2, 4};
    private static final int EXPECTED_RECORDS = 10;
    private static final int EXPECTED_STEPS = 24;

    private InfraNWTableEventTypeScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableEventTypeScenarioOracle <scenario.json>");
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

    /** Replays one case's steps on a fresh runtime (one runtime per case). */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
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
                                statementsByName);
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
                            statementsByName);
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
                    case "build-error": {
                        String statementName = string(step, "statement");
                        String epl = string(step, "epl");
                        String prefix = string(step, "expectError");
                        // tryInvalidCompile: compile the pinned probe text
                        // against the runtime path and require the failure
                        // message to start with the pinned prefix.
                        try {
                            CompilerArguments compilerArgs =
                                    new CompilerArguments(runtime.getRuntimePath());
                            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                            throw new IllegalStateException("build-error probe " + statementName
                                    + " unexpectedly compiled in case " + caseName);
                        } catch (Exception ex) {
                            if (ex instanceof IllegalStateException) {
                                throw ex;
                            }
                            String message = ex.getMessage() == null ? "" : ex.getMessage();
                            if (!message.startsWith(prefix)) {
                                throw new IllegalStateException("build-error probe "
                                        + statementName + " message " + message
                                        + " does not start with the pinned prefix " + prefix);
                            }
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "compile-error");
                        record.add("statement", statementName);
                        record.add("sequence", 0);
                        record.add("value", prefix);
                        records.add(record);
                        break;
                    }
                    case "types": {
                        String label = string(step, "statement");
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "types statement " + label + " was not deployed");
                        }
                        EventType eventType = statement.getEventType();
                        JsonArray entries = new JsonArray();
                        for (String name : eventType.getPropertyNames()) {
                            JsonObject entry = new JsonObject();
                            entry.add("name", name);
                            entry.add("type", eventType.getPropertyType(name).getSimpleName());
                            entries.add(entry);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "types");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("value", entries);
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "snapshot": {
                        String label = string(step, "statement");
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "snapshot statement " + label + " was not deployed");
                        }
                        JsonArray rows = new JsonArray();
                        Iterator<EventBean> iterator = statement.iterator();
                        while (iterator.hasNext()) {
                            rows.add(row(iterator.next()));
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "snapshot");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("new", rows);
                        records.add(record);
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
                        statementsByName);
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
     * in step order (a whitespace-normalized form of the suite's module text;
     * insert-into-protected prepends the `module test;` declaration so the
     * compiled text is verbatim), then registers the statements by scenario
     * label. CompilerArguments(runtime.getRuntimePath()) carries the prior
     * deployments' public types.
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        if ("insert-into-protected".equals(caseName)) {
            moduleEpl.append("module test;\n");
        }
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
        for (int index = 0; index < statementNames.size(); index++) {
            statementsByName.put(statementNames.get(index), deployed[index]);
        }
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if ("Fubar".equals(type)) {
            Map<String, Object> event = new LinkedHashMap<>();
            event.put("foo", string(payload, "foo"));
            event.put("bar", payload.get("bar").asDouble());
            runtime.getEventService().sendEventMap(event, type);
            return;
        }
        throw new IllegalArgumentException("unknown event type: " + type);
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
                    || !CASE_RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !CASE_EXECUTIONS[index].equals(string(definition, "executionName"))
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
        int offset = validateInvalidCase(steps, 0);
        offset = validateDefineFieldsCase(steps, offset, CASES[1], EPL_DEFINE_WINDOW);
        offset = validateDefineFieldsCase(steps, offset, CASES[2], EPL_DEFINE_TABLE);
        offset = validateInsertIntoProtectedCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact invalid step sequence mirroring InfraNWTableEventTypeInvalid
     * lines 60-69: the two tryInvalidCompile probes pinning the window and
     * table name-collision prefixes.
     */
    private static int validateInvalidCase(JsonArray steps, int offset) {
        String caseName = CASES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        validateBuildError(steps.get(offset++), caseName, "window-name-collision",
                EPL_INVALID_WINDOW, PREFIX_INVALID_WINDOW);
        validateBuildError(steps.get(offset++), caseName, "table-name-collision",
                EPL_INVALID_TABLE, PREFIX_INVALID_TABLE);
        return offset;
    }

    /**
     * Exact define-fields step sequence mirroring runAssertionType lines
     * 73-82: the single-statement create module deploys, the s0 statement's
     * event type is asserted, and the case ends with undeployAll.
     */
    private static int validateDefineFieldsCase(JsonArray steps, int offset,
                                                String caseName, String expectedEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", expectedEpl);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateTypes(steps.get(offset++), caseName, "s0");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact insert-into-protected step sequence mirroring
     * InfraNWTableEventTypeInsertIntoProtected lines 34-45: the
     * three-statement module deploys (create map schema, protected keepall
     * window, private insert-into), the two Fubar map sends, the window
     * iterator assertion and the case-end undeployAll.
     */
    private static int validateInsertIntoProtectedCase(JsonArray steps, int offset) {
        String caseName = CASES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "event", EPL_PROTECTED_EVENT);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_PROTECTED_WINDOW);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_PROTECTED_INSERT);
        validateDeployed(steps.get(offset++), caseName, "event");
        validateDeployed(steps.get(offset++), caseName, "window");
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateFubarSend(steps.get(offset++), caseName, "a", 1d);
        validateFubarSend(steps.get(offset++), caseName, "b", 2d);
        validateSnapshot(steps.get(offset++), caseName, "window");
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

    private static void validateBuildError(JsonValue value, String caseName,
                                           String expectedStatement, String expectedEpl,
                                           String expectedPrefix) {
        JsonObject step = object(value, "build-error step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"build-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedPrefix.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("build-error step is not pinned for " + caseName
                    + "/" + expectedStatement);
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

    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateFubarSend(JsonValue value, String caseName,
                                          String foo, double bar) {
        JsonObject step = object(value, "Fubar step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"Fubar".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("Fubar step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "Fubar payload");
        requireFields(payload, "foo", "bar");
        if (!foo.equals(payload.get("foo").asString())
                || payload.get("bar").asDouble() != bar) {
            throw new IllegalArgumentException("Fubar payload is not pinned for " + caseName);
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
