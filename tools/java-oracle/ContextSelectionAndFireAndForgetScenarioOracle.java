import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierPartitioned;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
import com.espertech.esper.common.client.context.ContextPartitionSelectorCategory;
import com.espertech.esper.common.client.context.EPContextPartitionService;
import com.espertech.esper.common.client.context.InvalidContextPartitionSelector;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetPreparedQuery;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetPreparedQueryParameterized;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.context.SupportSelectorById;
import com.espertech.esper.regressionlib.support.context.SupportSelectorPartitioned;
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
import java.util.IdentityHashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Replays the context-selection-faf scenario against the pinned Esper
 * runtime and emits the Java trace for the differential comparison.
 *
 * invalid (ord 0, ContextSelectionAndFireAndForgetInvalid): five fixture
 * deploys register the SegmentedSB/SegmentedS0 contexts, the context-bound
 * WinSB/WinS0 keepall windows and the context-free WinS1 window; two
 * compile-error probes pin the fire-and-forget join rejections — a join of
 * two context-bound windows under a context clause and a join of two
 * context-bound windows without one — after G1/G2 events materialize one
 * partition per key.
 *
 * iterate-statement (ord 1, ContextSelectionIterateStatement): one module
 * deploys the PartitionedByString keyed context and s0 (context.key1 plus a
 * per-partition sum over length(5)); three sends deliver listener rows; the
 * default iterator and the all/by-id {0,1,2} selectors yield {E1,10},{E2,41}
 * in order, by-id {1} yields {E2,41}, empty and null id sets yield no rows,
 * a nil selector fails with 'No selector provided', and the context-free s2
 * fails with 'Iterator with context selector is only supported for
 * statements under context'.
 *
 * named-window-query (ord 2,
 * ContextSelectionAndFireAndForgetNamedWindowQuery): a keyed context, a
 * context-bound keepall window and an insert-into feed materialize
 * partitions E1={10} and E2={20,21}; fire-and-forget queries pin the
 * context-free sums 51 and 41, the segmented {E2} and by-id {1} sums 41,
 * the context-clause rows {E1,10},{E2,20},{E2,21} and the filtered pair,
 * and a category selector over the keyed context is rejected as an invalid
 * context partition selector. Java's milestone calls are documented no-ops
 * and carry no steps.
 */
public final class ContextSelectionAndFireAndForgetScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-selection-faf";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextSelectionAndFireAndForget.java";
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-c8c49c4c40e41d383d25",
        "java-runtime-6dd5b9086935002cc50d",
        "java-runtime-bf4cefd62580e2abd2c6",
    };
    private static final String[] EXECUTION_NAMES = {
        "ContextSelectionAndFireAndForgetInvalid",
        "ContextSelectionIterateStatement",
        "ContextSelectionAndFireAndForgetNamedWindowQuery",
    };
    private static final String[] STATIC_IDS = {
        "java-1c493e94eff9beb161b3",
        "java-1c493e94eff9beb161b3",
        "java-1c493e94eff9beb161b3",
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET", "INVALIDITY"};
    private static final String[] CASES = {
        "invalid",
        "iterate-statement",
        "named-window-query",
    };
    private static final String[] CASE_OBSERVATIONS = {
        "compile-error; five fixture deploys register two segmented contexts, two context-bound keepall windows and one context-free window; the first probe joins two context-bound windows under a context clause, the second joins two context-bound windows without one; G1/G2 events materialize one partition per key before the second probe",
        "listener+snapshot+snapshot-selector+selector-error; one module deploys the keyed context and s0 (context.key1 plus per-partition sum over length(5)); E1/E2/E2 deliver three listener rows; the default iterator and the all/by-id {0,1,2} selectors yield {E1,10},{E2,41} in order, by-id {1} yields {E2,41}, empty and null id sets yield no rows, a nil selector fails with 'No selector provided', and the context-free s2 fails with 'Iterator with context selector is only supported for statements under context'",
        "faf+selector-error; a keyed context, a context-bound keepall window and an insert-into feed materialize partitions E1={10} and E2={20,21}; context-free sums pin 51 and 41, segmented {E2} and by-id {1} pin 41, context-clause queries pin all three rows and the filtered pair, and a category selector over the keyed context is rejected as an invalid context partition selector",
    };
    private static final String[] CASE_EPLS = {
        "context SegmentedSB select * from WinSB, WinS0",
        "@Name('s0') context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from SupportBean#length(5)",
        "context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow",
    };

    /**
     * The complete step sequence per case as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|ids|selector|filterProperty|filterValue keys. Deploy steps carry
     * the byte-exact EPL text; "filtered" snapshot-selector steps encode the
     * by-id empty/null sets (filterProperty "ids" with a payload array or
     * null) and the nil-selector/non-context probes (filterProperty "nil"
     * and "non-context"); faf selector "category" marks the invalid-selector
     * probe. Java's milestone calls are documented no-ops and carry no
     * steps.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("invalid", new String[]{
            "deploy|ctx-sb|||@public create context SegmentedSB as partition by theString from SupportBean||||||||",
            "deployed|ctx-sb|||||||||||",
            "deploy|ctx-s0|||@public create context SegmentedS0 as partition by p00 from SupportBean_S0||||||||",
            "deployed|ctx-s0|||||||||||",
            "deploy|win-sb|||@public context SegmentedSB create window WinSB#keepall as SupportBean||||||||",
            "deployed|win-sb|||||||||||",
            "deploy|win-s0|||@public context SegmentedS0 create window WinS0#keepall as SupportBean_S0||||||||",
            "deployed|win-s0|||||||||||",
            "deploy|win-s1|||@public create window WinS1#keepall as SupportBean_S1||||||||",
            "deployed|win-s1|||||||||||",
            "build-error|join-context-clause|||context SegmentedSB select * from WinSB, WinS0||Joins in runtime queries for context partitions are not supported||||||",
            "deploy|ctx-string|||@public create context PartitionedByString partition by theString from SupportBean||||||||",
            "deployed|ctx-string|||||||||||",
            "deploy|win-one|||@public context PartitionedByString create window MyWindowOne#keepall as SupportBean||||||||",
            "deployed|win-one|||||||||||",
            "deploy|ctx-p00|||@public create context PartitionedByP00 partition by p00 from SupportBean_S0||||||||",
            "deployed|ctx-p00|||||||||||",
            "deploy|win-two|||@public context PartitionedByP00 create window MyWindowTwo#keepall as SupportBean_S0||||||||",
            "deployed|win-two|||||||||||",
            "send|||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":10}|||||||",
            "send|||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":11}|||||||",
            "send|||SupportBean_S0||{\"id\":1,\"p00\":\"G2\"}|||||||",
            "send|||SupportBean_S0||{\"id\":2,\"p00\":\"G1\"}|||||||",
            "build-error|join-no-context|||select mw1.intPrimitive as c1, mw2.id as c2 from MyWindowOne mw1, MyWindowTwo mw2 where mw1.theString = mw2.p00||Joins against named windows that are under context are not supported||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("iterate-statement", new String[]{
            "deploy|module|||create context PartitionedByString partition by theString from SupportBean;\n@Name('s0') context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from SupportBean#length(5);\n||||||||",
            "deployed|module|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":21}|||||||",
            "snapshot|s0|||||||||||",
            "snapshot-selector|s0|||||||||all||",
            "snapshot-selector|s0||||||||[0,1,2]|ids||",
            "snapshot-selector|s0||||||||[1]|ids||",
            "snapshot-selector|s0||||[]|||||filtered|ids|",
            "snapshot-selector|s0||||null|||||filtered|ids|",
            "snapshot-selector|s0|||||No selector provided||||filtered|nil|",
            "deploy|s2|||@name('s2') select * from SupportBean||||||||",
            "deployed|s2|||||||||||",
            "snapshot-selector|s2|||||Iterator with context selector is only supported for statements under context||||filtered|non-context|",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("named-window-query", new String[]{
            "deploy|ctx|||@public create context PartitionedByString partition by theString from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|win|||@public context PartitionedByString create window MyWindow#keepall as SupportBean||||||||",
            "deployed|win|||||||||||",
            "deploy|insert|||insert into MyWindow select * from SupportBean||||||||",
            "deployed|insert|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":21}|||||||",
            "faf|sum-all|||select sum(intPrimitive) as c1 from MyWindow||||||all||",
            "faf|sum-filtered|||select sum(intPrimitive) as c1 from MyWindow where intPrimitive > 15||||||all||",
            "faf|sum-segmented|||select sum(intPrimitive) as c1 from MyWindow|[[\"E2\"]]|||||segmented||",
            "faf|sum-byid|||select sum(intPrimitive) as c1 from MyWindow|||||[1]|ids||",
            "faf|context-all|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow||||||all||",
            "faf|context-filtered|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15||||||all||",
            "faf|context-segmented|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15|[[\"E2\"]]|||||segmented||",
            "faf|invalid-selector|||context PartitionedByString select * from MyWindow||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered, ContextPartitionSelectorById, ContextPartitionSelectorSegmented] interfaces but received com||||category||",
            "undeploy-all||||||||||||",
        });
    }

    private ContextSelectionAndFireAndForgetScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextSelectionAndFireAndForgetScenarioOracle <scenario.json>");
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        validateScenario(scenario);
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ContextSelectionAndFireAndForgetScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            Map<String, String> contextDeploymentIds = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
            Set<EPStatement> listened = Collections.newSetFromMap(new IdentityHashMap<>());

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (op) {
                    case "deploy":
                        deployStep(runtime, config, caseName, step, records,
                            deploymentIds, statementsByName, contextDeploymentIds,
                            deployedModules, sequences, listened);
                        break;
                    case "deployed": {
                        String label = step.getString("statement", "");
                        String key = label + ":deployed";
                        int seq = sequences.getOrDefault(key, 0) + 1;
                        sequences.put(key, seq);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", seq);
                        record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "snapshot":
                        snapshotStep(runtime, caseName, step, records, statementsByName,
                            contextDeploymentIds);
                        break;
                    case "snapshot-selector":
                        snapshotSelectorStep(runtime, caseName, step, records, statementsByName,
                            contextDeploymentIds);
                        break;
                    case "faf":
                        fafStep(runtime, config, caseName, step, records, deployedModules);
                        break;
                    case "build-error":
                        buildErrorStep(config, caseName, step, records, deployedModules);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        statementsByName.clear();
                        contextDeploymentIds.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + op);
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module. Deploys accumulate into the
     * module path like RegressionPath so later modules resolve the earlier
     * contexts and named windows. The create-context statement's context
     * name is parsed from the EPL so the context-partition service reads can
     * resolve the context's deployment id. The s0 listener attaches for the
     * iterate-statement case, mirroring env.addListener("s0").
     */
    private static void deployStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                   List<JsonObject> records, Map<String, String> deploymentIds,
                                   Map<String, EPStatement> statementsByName,
                                   Map<String, String> contextDeploymentIds,
                                   List<EPCompiled> deployedModules,
                                   Map<String, Integer> sequences, Set<EPStatement> listened) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs(config, deployedModules));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            deployedModules.add(compiled);
            deploymentIds.put(label, deployment.getDeploymentId());
            String contextName = parseContextName(epl);
            if (contextName != null) {
                contextDeploymentIds.put(contextName, deployment.getDeploymentId());
            }
            for (EPStatement stmt : deployment.getStatements()) {
                statementsByName.put(stmt.getName(), stmt);
                String name = stmt.getName();
                if (listenedNames(caseName).contains(name) && !listened.contains(stmt)) {
                    listened.add(stmt);
                    stmt.addListener(listener(caseName, name, sequences, records, runtime));
                }
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    /** The statement names each case listens on, mirroring env.addListener calls. */
    private static Set<String> listenedNames(String caseName) {
        if ("iterate-statement".equals(caseName)) {
            return new HashSet<>(Collections.singletonList("s0"));
        }
        return Collections.emptySet();
    }

    /** Parses the context name out of a create-context EPL module. */
    private static String parseContextName(String epl) {
        Matcher matcher = Pattern.compile("create context\\s+(\\w+)").matcher(epl);
        return matcher.find() ? matcher.group(1) : null;
    }

    /** Sends one event bean decoded from the step payload. */
    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull() ? null : theString.asString());
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null && !intPrimitive.isNull()) {
                    bean.setIntPrimitive(intPrimitive.asInt());
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                JsonValue p00 = payload.get("p00");
                runtime.getEventService().sendEventBean(
                    new SupportBean_S0(payload.get("id").asInt(),
                        p00 == null || p00.isNull() ? null : p00.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Emits one snapshot record mirroring the Java execution's
     * assertPropsPerRowIterator("s0", ...) call: the statement's default
     * iterator rows in partition-allocation order plus the statement's
     * partition descriptors.
     */
    private static void snapshotStep(EPRuntime runtime, String caseName, JsonObject step,
                                     List<JsonObject> records, Map<String, EPStatement> statementsByName,
                                     Map<String, String> contextDeploymentIds) {
        String name = step.getString("statement", "");
        EPStatement statement = statementsByName.get(name);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot " + name);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = rows(iterateRows(statement.iterator()));
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        JsonArray partitions = partitions(runtime, statement, contextDeploymentIds, ContextPartitionSelectorAll.INSTANCE);
        if (partitions.size() > 0) {
            record.add("partitions", partitions);
        }
        records.add(record);
    }

    /**
     * Replays one selector-targeted iterator step, mirroring the Java
     * execution's statement.iterator(selector) calls: "all" maps to
     * ContextPartitionSelectorAll, "ids" maps to SupportSelectorById over
     * the step's id array, "filtered" with filterProperty "ids" maps to
     * SupportSelectorById over the payload id set (an empty array and null
     * both select nothing), and "filtered" with filterProperty "nil" or
     * "non-context" passes a null selector to pin the IllegalArgumentException
     * and UnsupportedOperationException messages.
     */
    private static void snapshotSelectorStep(EPRuntime runtime, String caseName, JsonObject step,
                                             List<JsonObject> records,
                                             Map<String, EPStatement> statementsByName,
                                             Map<String, String> contextDeploymentIds) {
        String name = step.getString("statement", "");
        EPStatement statement = statementsByName.get(name);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot-selector " + name);
        }
        String selectorKind = step.getString("selector", "");
        ContextPartitionSelector selector;
        // Null-id sets select nothing; Esper's partition service may not
        // accept a null id collection, so the descriptor read is skipped for
        // that probe (the Java execution only asserts the empty iterator).
        boolean collectPartitions = true;
        switch (selectorKind) {
            case "all":
                selector = ContextPartitionSelectorAll.INSTANCE;
                break;
            case "ids": {
                Set<Integer> ids = new HashSet<>();
                JsonValue raw = step.get("ids");
                if (raw != null && raw.isArray()) {
                    for (JsonValue id : raw.asArray()) {
                        ids.add(id.asInt());
                    }
                }
                selector = new SupportSelectorById(ids);
                break;
            }
            case "filtered": {
                String property = step.getString("filterProperty", "");
                if ("ids".equals(property)) {
                    Set<Integer> ids = null;
                    JsonValue payload = step.get("payload");
                    if (payload != null && payload.isArray()) {
                        ids = new HashSet<>();
                        for (JsonValue id : payload.asArray()) {
                            ids.add(id.asInt());
                        }
                    }
                    if (ids == null) {
                        collectPartitions = false;
                    }
                    selector = new SupportSelectorById(ids);
                } else if ("nil".equals(property) || "non-context".equals(property)) {
                    // The nil-selector and non-context probes pass a null
                    // selector; the statement decides which rejection fires.
                    selector = null;
                } else {
                    throw new IllegalStateException("filtered selector property " + property + " is not pinned");
                }
                break;
            }
            default:
                throw new IllegalStateException("unsupported selector " + selectorKind);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot-selector");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        String expected = step.getString("expectError", "");
        try {
            Iterator<EventBean> it = statement.iterator(selector);
            JsonArray rows = rows(iterateRows(it));
            if (rows.size() > 0) {
                record.add("new", rows);
            }
            if (collectPartitions) {
                JsonArray partitions = partitions(runtime, statement, contextDeploymentIds, selector);
                if (partitions.size() > 0) {
                    record.add("partitions", partitions);
                }
            }
        } catch (IllegalArgumentException | UnsupportedOperationException ex) {
            if (expected.isEmpty()) {
                throw ex;
            }
            // The Java execution pins these probes with assertEquals, so the
            // message must match exactly.
            if (!expected.equals(ex.getMessage())) {
                throw new IllegalStateException("selector-error message drift for " + selectorKind
                    + ": expected [" + expected + "] got [" + ex.getMessage() + "]");
            }
            record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "selector-error");
            record.add("statement", name);
            record.add("sequence", 0);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Replays one fire-and-forget step, mirroring the Java execution's
     * runQuery/runQueryAll helpers: the EPL compiles through compileFAF
     * (compiler.compileQuery with the accumulated module path) and executes
     * via executeQuery, preparedQuery.execute, the parameterized prepare
     * path, the SODA eplToModel round-trip and the model executeQuery; every
     * form must return the same rows. Selector "all" additionally runs the
     * no-selector executeQuery like runQueryAll. Selector "category" pins
     * the InvalidContextPartitionSelector rejection of a category selector
     * over the keyed context. Rows sort by their compact field rendering
     * because the Java assertions are any-order.
     */
    private static void fafStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                List<JsonObject> records, List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String selectorKind = step.getString("selector", "");
        String expected = step.getString("expectError", "");
        ContextPartitionSelector selector;
        switch (selectorKind) {
            case "all":
                selector = ContextPartitionSelectorAll.INSTANCE;
                break;
            case "ids": {
                Set<Integer> ids = new HashSet<>();
                JsonValue raw = step.get("ids");
                if (raw != null && raw.isArray()) {
                    for (JsonValue id : raw.asArray()) {
                        ids.add(id.asInt());
                    }
                }
                selector = new SupportSelectorById(ids);
                break;
            }
            case "segmented": {
                List<Object[]> keys = new ArrayList<>();
                JsonValue payload = step.get("payload");
                if (payload != null && payload.isArray()) {
                    for (JsonValue tuple : payload.asArray()) {
                        JsonArray values = tuple.asArray();
                        Object[] key = new Object[values.size()];
                        for (int i = 0; i < values.size(); i++) {
                            key[i] = values.get(i).asString();
                        }
                        keys.add(key);
                    }
                }
                selector = new SupportSelectorPartitioned(keys);
                break;
            }
            case "category":
                selector = new ContextPartitionSelectorCategory() {
                    public Set<String> getLabels() {
                        return null;
                    }
                };
                break;
            default:
                throw new IllegalStateException("unsupported faf selector " + selectorKind);
        }
        EPCompiled compiled;
        try {
            compiled = EPCompilerProvider.getCompiler().compileQuery(epl, compilerArgs(config, deployedModules));
        } catch (Exception ex) {
            throw new IllegalStateException("faf " + label + " failed to compile: " + rootCauseMessage(ex), ex);
        }
        if ("category".equals(selectorKind)) {
            try {
                runtime.getFireAndForgetService().executeQuery(compiled, new ContextPartitionSelector[]{selector});
                throw new IllegalStateException("faf " + label + " unexpectedly accepted a category selector");
            } catch (InvalidContextPartitionSelector ex) {
                // The pinned prefix ends at "received com" because the regression
                // suite's anonymous selector lives under com.espertech; this oracle's
                // anonymous class does not, so verify the stable prefix only.
                String stablePrefix = "Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered, ContextPartitionSelectorById, ContextPartitionSelectorSegmented] interfaces but received ";
                if (!ex.getMessage().startsWith(stablePrefix)) {
                    throw new IllegalStateException("selector-error message drift for " + label
                        + ": expected prefix [" + stablePrefix + "] got [" + ex.getMessage() + "]");
                }
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "selector-error");
                record.add("statement", label);
                record.add("sequence", 0);
                record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
                if (!expected.isEmpty()) {
                    record.add("value", expected);
                }
                records.add(record);
                return;
            }
        }
        ContextPartitionSelector[] selectors = new ContextPartitionSelector[]{selector};
        JsonArray fafRows;
        try {
            EPFireAndForgetQueryResult result =
                runtime.getFireAndForgetService().executeQuery(compiled, selectors);
            fafRows = sortedRows(result.getArray());

            EPFireAndForgetPreparedQuery preparedQuery =
                runtime.getFireAndForgetService().prepareQuery(compiled);
            assertSameRows(label, "prepared", fafRows, sortedRows(preparedQuery.execute(selectors).getArray()));

            EPFireAndForgetPreparedQueryParameterized parameterized =
                runtime.getFireAndForgetService().prepareQueryWithParameters(compiled);
            assertSameRows(label, "parameterized", fafRows,
                sortedRows(runtime.getFireAndForgetService().executeQuery(parameterized, selectors).getArray()));

            EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, config);
            EPCompiled compiledFromModel =
                EPCompilerProvider.getCompiler().compileQuery(model, compilerArgs(config, deployedModules));
            EPFireAndForgetPreparedQuery preparedModel =
                runtime.getFireAndForgetService().prepareQuery(compiledFromModel);
            assertSameRows(label, "soda-prepared", fafRows, sortedRows(preparedModel.execute(selectors).getArray()));
            assertSameRows(label, "soda", fafRows,
                sortedRows(runtime.getFireAndForgetService().executeQuery(compiledFromModel, selectors).getArray()));

            if ("all".equals(selectorKind)) {
                // runQueryAll also runs the same query without a selector.
                assertSameRows(label, "no-selector", fafRows,
                    sortedRows(runtime.getFireAndForgetService().executeQuery(compiled).getArray()));
            }
        } catch (Exception ex) {
            throw new IllegalStateException("faf " + label + " failed: " + rootCauseMessage(ex), ex);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "faf");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        if (fafRows.size() > 0) {
            record.add("new", fafRows);
        }
        records.add(record);
    }

    /**
     * Compiles an expected-invalid fire-and-forget query, mirroring
     * SupportMessageAssertUtil.tryInvalidFAFCompile: the query compiles
     * through compileQuery with the accumulated module path, must fail, and
     * the message must start with the pinned prefix; the record value
     * carries the asserted prefix.
     */
    private static void buildErrorStep(Configuration config, String caseName, JsonObject step,
                                       List<JsonObject> records, List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            EPCompilerProvider.getCompiler().compileQuery(step.getString("epl", ""),
                compilerArgs(config, deployedModules));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
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

    /** Compiler arguments with the accumulated module path, like RegressionPath. */
    private static CompilerArguments compilerArgs(Configuration config, List<EPCompiled> deployedModules) {
        CompilerArguments args = new CompilerArguments(config);
        for (EPCompiled deployed : deployedModules) {
            args.getPath().add(deployed);
        }
        return args;
    }

    /** Asserts one alternate execution form returned the same rows. */
    private static void assertSameRows(String label, String form, JsonArray expected, JsonArray actual) {
        if (!expected.toString().equals(actual.toString())) {
            throw new IllegalStateException("faf " + label + " " + form + " rows drift: expected "
                + expected + " got " + actual);
        }
    }

    /** Iterates one statement iterator into a row list. */
    private static List<JsonObject> iterateRows(Iterator<EventBean> it) {
        List<JsonObject> rows = new ArrayList<>();
        while (it.hasNext()) {
            rows.add(row(it.next()));
        }
        return rows;
    }

    /** Renders the row list as a JSON array in iteration order. */
    private static JsonArray rows(List<JsonObject> rows) {
        JsonArray array = new JsonArray();
        for (JsonObject row : rows) {
            array.add(row);
        }
        return array;
    }

    /** Sorts rows by their compact field rendering for the any-order pins. */
    private static JsonArray sortedRows(EventBean[] events) {
        List<JsonObject> rows = new ArrayList<>();
        if (events != null) {
            for (EventBean event : events) {
                rows.add(row(event));
            }
        }
        rows.sort((left, right) -> left.get("fields").toString().compareTo(right.get("fields").toString()));
        JsonArray array = new JsonArray();
        for (JsonObject row : rows) {
            array.add(row);
        }
        return array;
    }

    /**
     * Renders the statement-scoped partition identifiers for snapshot
     * records: one record per ContextPartitionIdentifierPartitioned
     * carrying its id, key and empty properties object.
     */
    private static JsonArray partitions(EPRuntime runtime, EPStatement statement,
                                        Map<String, String> contextDeploymentIds,
                                        ContextPartitionSelector selector) {
        String contextName = String.valueOf(statement.getProperty(StatementProperty.CONTEXTNAME));
        String depId = contextDeploymentIds.get(contextName);
        if (depId == null) {
            return new JsonArray();
        }
        EPContextPartitionService service = runtime.getContextPartitionService();
        ContextPartitionCollection collection = service.getContextPartitions(depId, contextName, selector);
        JsonArray array = new JsonArray();
        List<Map.Entry<Integer, ContextPartitionIdentifier>> entries =
            new ArrayList<>(collection.getIdentifiers().entrySet());
        entries.sort(Map.Entry.comparingByKey());
        for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
            ContextPartitionIdentifierPartitioned id = (ContextPartitionIdentifierPartitioned) entry.getValue();
            JsonObject partition = new JsonObject();
            partition.add("id", entry.getKey());
            partition.add("key", "key:" + id.getKeys()[0]);
            partition.add("properties", new JsonObject());
            array.add(partition);
        }
        return array;
    }

    /** Per-statement listener emitting one IR-pair record per callback. */
    private static UpdateListener listener(String caseName, String statementName,
                                           Map<String, Integer> sequences, List<JsonObject> records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            int sequence = sequences.getOrDefault(statementName, 0) + 1;
            sequences.put(statementName, sequence);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for stable field order. */
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
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * integral numbers as JSON numbers, other numbers as doubles, booleans
     * and characters passthrough, everything else stringified.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        if (value instanceof Iterable) {
            JsonArray array = new JsonArray();
            for (Object element : (Iterable<?>) value) {
                array.add(normalize(element));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Deepest non-null cause message, the canonical cross-runtime error text. */
    private static String rootCauseMessage(Throwable throwable) {
        Throwable current = throwable;
        while (true) {
            Throwable cause = current.getCause();
            if (cause == null || cause == current) {
                break;
            }
            current = cause;
        }
        return current.getMessage();
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
            "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = scenario.get("cases").asArray();
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("cases must contain exactly " + CASES.length + " entries");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonObject entry = cases.get(i).asObject();
            requireFields(entry, "case", "ordinal", "runtimeId", "executionName", "observation", "epl");
            if (!CASES[i].equals(string(entry, "case"))
                    || ORDINALS[i] != entry.getInt("ordinal", -1)
                    || !RUNTIME_IDS[i].equals(string(entry, "runtimeId"))
                    || !EXECUTION_NAMES[i].equals(string(entry, "executionName"))
                    || !CASE_OBSERVATIONS[i].equals(string(entry, "observation"))
                    || !CASE_EPLS[i].equals(string(entry, "epl"))) {
                throw new IllegalArgumentException("case " + i + " metadata is not pinned");
            }
        }

        JsonArray steps = scenario.get("steps").asArray();
        int offset = 0;
        for (String caseName : CASES) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("missing case " + caseName);
            }
            JsonObject marker = steps.get(offset).asObject();
            if (!"case".equals(string(marker, "op")) || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("step " + offset + " is not the " + caseName + " case marker");
            }
            offset++;
            String[] want = CASE_STEPS.get(caseName);
            if (offset + want.length > steps.size()) {
                throw new IllegalArgumentException("case " + caseName + " is truncated");
            }
            for (String pinned : want) {
                String key = stepKey(steps.get(offset).asObject());
                if (!pinned.equals(key)) {
                    throw new IllegalArgumentException("case " + caseName + " step " + offset
                        + " = [" + key + "], want [" + pinned + "]");
                }
                offset++;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has " + (steps.size() - offset) + " trailing steps");
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|ids|selector|filterProperty|filterValue with the payload compacted
     * and ids rendered as a JSON array. Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
            "op", "case", "statement", "name", "eventType", "epl", "payload",
            "expectError", "compileWithoutPath", "mode", "ids",
            "selector", "filterProperty", "filterValue"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        String payload = "";
        JsonValue payloadValue = step.get("payload");
        if (payloadValue != null) {
            payload = payloadValue.toString();
        }
        String ids = "";
        JsonValue idsValue = step.get("ids");
        if (idsValue != null) {
            ids = idsValue.toString();
        }
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
            + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payload
            + "|" + string(step, "expectError") + "|" + cwp
            + "|" + string(step, "mode") + "|" + ids
            + "|" + string(step, "selector") + "|" + string(step, "filterProperty")
            + "|" + string(step, "filterValue");
    }

    private static void requireFields(JsonObject object, String... names) {
        if (object.names().size() != names.length) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("JSON object is missing field " + name);
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be a string array");
        }
        JsonArray array = value.asArray();
        if (array.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(array.get(i).asString())) {
                throw new IllegalArgumentException(name + " is not pinned");
            }
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        return value == null || value.isNull() ? "" : value.asString();
    }
}
