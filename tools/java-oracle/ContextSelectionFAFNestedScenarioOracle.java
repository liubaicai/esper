import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierCategory;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierHash;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierInitiatedTerminated;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierNested;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierPartitioned;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
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
import com.espertech.esper.regressionlib.support.context.SupportSelectorByHashCode;
import com.espertech.esper.regressionlib.support.context.SupportSelectorById;
import com.espertech.esper.regressionlib.support.context.SupportSelectorFilteredPassAll;
import com.espertech.esper.regressionlib.support.context.SupportSelectorNested;
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
 * Replays the context-selection-faf-nested scenario against the pinned Esper
 * runtime and emits the Java trace for the differential comparison.
 *
 * nested-named-window-query (ord 3,
 * ContextSelectionFAFNestedNamedWindowQuery): a nested context whose ACtx
 * parent is an overlapping initiated-terminated context (SupportBean_S0
 * starts a partition, SupportBean_S1(id=s0.id) ends it) and whose BCtx child
 * is a three-category context over SupportBean (grp1 intPrimitive &lt; 0,
 * grp2 = 0, grp3 &gt; 0). Every S0 activation eagerly instantiates all three
 * category leaves in declaration order with globally allocated leaf ids, so
 * S0(1) materializes leaves 0-2 and S0(2) materializes leaves 3-5. SupportBean
 * events broadcast to every live parent partition and land in one leaf per
 * parent by category; the triggering S0 never enters MyWindow. The
 * context-bound keepall window fed by insert-into answers the group-by merge
 * fire-and-forget query across all leaves ({E1,5},{E2,-2},{E3,10}), the same
 * query under a by-id {2} selector ({E1,3},{E3,5}), and the per-partition
 * context-clause query projecting context.ACtx.s0.p00 and context.BCtx.label
 * under by-id {2} ({S0_1,grp3,E1,3},{S0_1,grp3,E3,5}). A context-level
 * snapshot-selector step pins the six leaf descriptors (id, composite key,
 * startTime/endTime, initiating S0 p00, category label), and a segmented
 * selector over the nested context is rejected as an invalid context
 * partition selector. Java's milestone calls are documented no-ops and carry
 * no steps.
 */
public final class ContextSelectionFAFNestedScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-selection-faf-nested";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextSelectionAndFireAndForget.java";
    private static final int[] ORDINALS = {3};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-f51a1493ad61c1f0d0d1",
    };
    private static final String[] EXECUTION_NAMES = {
        "ContextSelectionFAFNestedNamedWindowQuery",
    };
    private static final String[] STATIC_IDS = {
        "java-1c493e94eff9beb161b3",
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};
    private static final String[] CASES = {
        "nested-named-window-query",
    };
    private static final String[] CASE_OBSERVATIONS = {
        "deploy+send+snapshot-selector+faf+selector-error; the nested context deploys an overlapping initiated ACtx parent (S0 starts, S1(id=s0.id) ends) over a three-category BCtx child; S0(1) and S0(2) eagerly materialize leaves 0-2 and 3-5; SupportBean events broadcast to all live parents and route by category so leaf2={E1(1),E3(5),E1(2)} and leaf5={E3(5),E1(2)}; the all-selector group-by merge pins {E1,5},{E2,-2},{E3,10}, by-id {2} pins {E1,3},{E3,5}, the context-clause per-partition query pins {S0_1,grp3,E1,3},{S0_1,grp3,E3,5}, and a segmented selector is rejected as an invalid context partition selector",
    };
    private static final String[] CASE_EPLS = {
        "context NestedContext select context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as c4 from MyWindow group by theString",
    };

    /**
     * The complete step sequence per case as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|ids|selector|filterProperty|filterValue|at keys. Deploy steps carry
     * the byte-exact EPL text; the snapshot-selector step carries the context
     * name in "name" and reads context-level leaf descriptors; faf selector
     * "segmented" marks the invalid-selector probe. Java's milestone calls
     * are documented no-ops and carry no steps.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("nested-named-window-query", new String[]{
            "deploy|ctx|||@public create context NestedContext context ACtx initiated by SupportBean_S0 as s0 terminated by SupportBean_S1(id=s0.id), context BCtx group by intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2, group by intPrimitive > 0 as grp3 from SupportBean|||||||||",
            "deployed|ctx||||||||||||",
            "deploy|win|||@public context NestedContext create window MyWindow#keepall as SupportBean|||||||||",
            "deployed|win||||||||||||",
            "deploy|insert|||insert into MyWindow select * from SupportBean|||||||||",
            "deployed|insert||||||||||||",
            "send|||SupportBean_S0||{\"id\":1,\"p00\":\"S0_1\"}||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||||||||",
            "send|||SupportBean_S0||{\"id\":2,\"p00\":\"S0_2\"}||||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":-1}||||||||",
            "send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":5}||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":2}||||||||",
            "snapshot-selector|ctx|NestedContext||||||||all|||",
            "faf|sum-all|||select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString||||||all|||",
            "faf|sum-byid|||select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString|||||[2]|ids|||",
            "faf|context-byid|||context NestedContext select context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as c4 from MyWindow group by theString|||||[2]|ids|||",
            "faf|invalid-selector|||context NestedContext select * from MyWindow||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorById, ContextPartitionSelectorNested] interfaces but received com||||segmented|||",
            "undeploy-all|||||||||||||",
        });
    }

    private ContextSelectionFAFNestedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextSelectionFAFNestedScenarioOracle <scenario.json>");
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
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ContextSelectionFAFNestedScenarioOracle-" + caseName, config);
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
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                            Instant.parse(step.getString("at", "")).toEpochMilli());
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
                    case "compile-error":
                        compileErrorStep(config, caseName, step, records, deployedModules);
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
     * resolve the context's deployment id.
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
            case "SupportBean_S1": {
                JsonValue p10 = payload.get("p10");
                runtime.getEventService().sendEventBean(
                    new SupportBean_S1(payload.get("id").asInt(),
                        p10 == null || p10.isNull() ? null : p10.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Emits one snapshot record mirroring the Java execution's
     * assertPropsPerRowIterator call: the statement's default iterator rows
     * in partition-allocation order plus the statement's partition
     * descriptors.
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
     * Replays one selector-targeted step. When the step carries a context
     * name in "name" the read is context-level, mirroring
     * EPContextPartitionService.getContextPartitions over the nested
     * context's leaf partitions; otherwise the step mirrors the Java
     * execution's statement.iterator(selector) calls. Selector "all" maps to
     * ContextPartitionSelectorAll, "ids" to SupportSelectorById over the
     * step's id array, "nested" to SupportSelectorNested over the pinned
     * per-level all selectors, and "segmented"/"hashes"/"filtered" build the
     * matching selector kinds that nested contexts reject with
     * InvalidContextPartitionSelector.
     */
    private static void snapshotSelectorStep(EPRuntime runtime, String caseName, JsonObject step,
                                             List<JsonObject> records,
                                             Map<String, EPStatement> statementsByName,
                                             Map<String, String> contextDeploymentIds) {
        String label = step.getString("statement", "");
        String contextName = step.getString("name", "");
        String expected = step.getString("expectError", "");
        ContextPartitionSelector selector = selectorFor(step);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot-selector");
        if (!label.isEmpty()) {
            record.add("statement", label);
        }
        if (!contextName.isEmpty()) {
            record.add("name", contextName);
        }
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        try {
            if (!contextName.isEmpty()) {
                // Context-level read: the leaf descriptors of the nested
                // context in partition-id order.
                String depId = contextDeploymentIds.get(contextName);
                if (depId == null) {
                    throw new IllegalStateException("no context deployment for " + contextName);
                }
                EPContextPartitionService service = runtime.getContextPartitionService();
                ContextPartitionCollection collection =
                    service.getContextPartitions(depId, contextName, selector);
                JsonArray partitions = nestedPartitions(collection);
                if (partitions.size() > 0) {
                    record.add("partitions", partitions);
                }
            } else {
                EPStatement statement = statementsByName.get(label);
                if (statement == null) {
                    throw new IllegalStateException("no statement for snapshot-selector " + label);
                }
                Iterator<EventBean> it = statement.iterator(selector);
                JsonArray rows = rows(iterateRows(it));
                if (rows.size() > 0) {
                    record.add("new", rows);
                }
                JsonArray partitions = partitions(runtime, statement, contextDeploymentIds, selector);
                if (partitions.size() > 0) {
                    record.add("partitions", partitions);
                }
            }
        } catch (InvalidContextPartitionSelector | IllegalArgumentException
                 | UnsupportedOperationException ex) {
            if (expected.isEmpty()) {
                throw ex;
            }
            // The pinned prefix ends at "received com" because the selector
            // helper classes live under com.espertech; verify the stable
            // prefix only.
            if (!ex.getMessage().startsWith(expected)) {
                throw new IllegalStateException("selector-error message drift for "
                    + step.getString("selector", "")
                    + ": expected prefix [" + expected + "] got [" + ex.getMessage() + "]");
            }
            record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "selector-error");
            if (!label.isEmpty()) {
                record.add("statement", label);
            }
            if (!contextName.isEmpty()) {
                record.add("name", contextName);
            }
            record.add("sequence", 0);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Maps one step's selector encoding to the Java selector instance:
     * "all" is ContextPartitionSelectorAll, "ids" is SupportSelectorById
     * over the step's id array, "nested" is SupportSelectorNested over the
     * pinned per-level all selectors, "segmented" is SupportSelectorPartitioned
     * over the payload key tuples, "hashes" is SupportSelectorByHashCode over
     * the id array, and "filtered" is the pass-all filtered selector. The
     * last three kinds are invalid over nested contexts and drive the
     * selector-error records.
     */
    private static ContextPartitionSelector selectorFor(JsonObject step) {
        String selectorKind = step.getString("selector", "");
        switch (selectorKind) {
            case "all":
                return ContextPartitionSelectorAll.INSTANCE;
            case "ids": {
                Set<Integer> ids = new HashSet<>();
                JsonValue raw = step.get("ids");
                if (raw != null && raw.isArray()) {
                    for (JsonValue id : raw.asArray()) {
                        ids.add(id.asInt());
                    }
                }
                return new SupportSelectorById(ids);
            }
            case "nested":
                return new SupportSelectorNested(ContextPartitionSelectorAll.INSTANCE,
                    ContextPartitionSelectorAll.INSTANCE);
            case "segmented": {
                List<Object[]> keys = new ArrayList<>();
                JsonValue payload = step.get("payload");
                if (payload != null && payload.isArray()) {
                    for (JsonValue tuple : payload.asArray()) {
                        JsonArray values = tuple.asArray();
                        Object[] key = new Object[values.size()];
                        for (int i = 0; i < values.size(); i++) {
                            JsonValue value = values.get(i);
                            key[i] = value.isString() ? value.asString()
                                : value.isNumber() ? (Object) value.asInt()
                                : value.toString();
                        }
                        keys.add(key);
                    }
                }
                return new SupportSelectorPartitioned(keys);
            }
            case "hashes": {
                Set<Integer> hashes = new HashSet<>();
                JsonValue raw = step.get("ids");
                if (raw != null && raw.isArray()) {
                    for (JsonValue id : raw.asArray()) {
                        hashes.add(id.asInt());
                    }
                }
                return new SupportSelectorByHashCode(hashes);
            }
            case "filtered":
                return new SupportSelectorFilteredPassAll();
            default:
                throw new IllegalStateException("unsupported selector " + selectorKind);
        }
    }

    /**
     * Replays one fire-and-forget step, mirroring the Java execution's
     * runQuery/runQueryAll helpers: the EPL compiles through compileFAF
     * (compiler.compileQuery with the accumulated module path) and executes
     * via executeQuery, preparedQuery.execute, the parameterized prepare
     * path, the SODA eplToModel round-trip and the model executeQuery; every
     * form must return the same rows. Selector "all" additionally runs the
     * no-selector executeQuery like runQueryAll. Any selector kind rejected
     * with InvalidContextPartitionSelector records the pinned message prefix
     * as a selector-error. Rows sort by their compact field rendering
     * because the Java assertions are any-order.
     */
    private static void fafStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                List<JsonObject> records, List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String selectorKind = step.getString("selector", "");
        String expected = step.getString("expectError", "");
        ContextPartitionSelector selector = selectorFor(step);
        EPCompiled compiled;
        try {
            compiled = EPCompilerProvider.getCompiler().compileQuery(epl, compilerArgs(config, deployedModules));
        } catch (Exception ex) {
            throw new IllegalStateException("faf " + label + " failed to compile: " + rootCauseMessage(ex), ex);
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
        } catch (InvalidContextPartitionSelector ex) {
            if (expected.isEmpty()) {
                throw ex;
            }
            // The pinned prefix ends at "received com" because the selector
            // helper classes live under com.espertech; verify the stable
            // prefix only.
            if (!ex.getMessage().startsWith(expected)) {
                throw new IllegalStateException("selector-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + ex.getMessage() + "]");
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "selector-error");
            record.add("statement", label);
            record.add("sequence", 0);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            record.add("value", expected);
            records.add(record);
            return;
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
     * Compiles an expected-invalid module, mirroring the deploy path's
     * compiler.compile boundary: the module must fail to compile and the
     * message must start with the pinned prefix; the record value carries
     * the asserted prefix.
     */
    private static void compileErrorStep(Configuration config, String caseName, JsonObject step,
                                         List<JsonObject> records, List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""),
                compilerArgs(config, deployedModules));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("compile-error probe " + label + " unexpectedly compiled");
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
     * records: one record per identifier carrying its id, composite key and
     * properties object. Nested identifiers flatten to the leaf descriptor
     * shape (parent startTime/endTime plus initiating p00, child label).
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
        return nestedPartitions(collection);
    }

    /**
     * Renders one partition collection as leaf descriptors in partition-id
     * order: each record carries the leaf id, a composite
     * "nested:<parent>/<child>" key, and flat properties (startTime/endTime
     * and initiating.p00 from the initiated parent level, label from the
     * category child level, hash from a hash child level, key:<value> from a
     * partitioned level). Non-nested identifiers render their own level.
     */
    private static JsonArray nestedPartitions(ContextPartitionCollection collection) {
        JsonArray array = new JsonArray();
        List<Map.Entry<Integer, ContextPartitionIdentifier>> entries =
            new ArrayList<>(collection.getIdentifiers().entrySet());
        entries.sort(Map.Entry.comparingByKey());
        for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
            array.add(renderPartition(entry.getKey(), entry.getValue()));
        }
        return array;
    }

    private static JsonObject renderPartition(int id, ContextPartitionIdentifier identifier) {
        JsonObject properties = new JsonObject();
        String parentPart = "";
        String childPart = "";
        ContextPartitionIdentifier[] chain = identifier instanceof ContextPartitionIdentifierNested
            ? ((ContextPartitionIdentifierNested) identifier).getIdentifiers()
            : new ContextPartitionIdentifier[]{identifier};
        for (int level = 0; level < chain.length; level++) {
            ContextPartitionIdentifier current = chain[level];
            boolean parentLevel = chain.length > 1 && level < chain.length - 1;
            if (current instanceof ContextPartitionIdentifierInitiatedTerminated) {
                ContextPartitionIdentifierInitiatedTerminated initiated =
                    (ContextPartitionIdentifierInitiatedTerminated) current;
                properties.add("startTime", initiated.getStartTime());
                properties.add("endTime", initiated.getEndTime() == null
                    ? Json.NULL : Json.value(initiated.getEndTime()));
                Map<String, Object> initiating = initiated.getProperties();
                Object event = null;
                if (initiating != null) {
                    for (Object value : initiating.values()) {
                        if (value instanceof EventBean) {
                            event = value;
                            break;
                        }
                    }
                }
                if (event instanceof EventBean) {
                    properties.add("initiating.p00",
                        normalize(((EventBean) event).get("p00")));
                }
                parentPart = "start:" + initiated.getStartTime();
            } else if (current instanceof ContextPartitionIdentifierCategory) {
                String label = ((ContextPartitionIdentifierCategory) current).getLabel();
                properties.add("label", label);
                childPart = "category:" + label;
            } else if (current instanceof ContextPartitionIdentifierHash) {
                long hash = ((ContextPartitionIdentifierHash) current).getHash();
                properties.add("hash", hash);
                childPart = "hash:" + hash;
            } else if (current instanceof ContextPartitionIdentifierPartitioned) {
                Object[] keys = ((ContextPartitionIdentifierPartitioned) current).getKeys();
                String rendered = keys == null || keys.length == 0 ? "" : String.valueOf(keys[0]);
                childPart = "key:" + rendered;
            }
        }
        String key;
        if (identifier instanceof ContextPartitionIdentifierNested) {
            key = "nested:" + parentPart + "/" + childPart;
        } else {
            key = childPart.isEmpty() ? parentPart : childPart;
        }
        JsonObject partition = new JsonObject();
        partition.add("id", id);
        partition.add("key", key);
        partition.add("properties", properties);
        return partition;
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
     * mode|ids|selector|filterProperty|filterValue|at with the payload
     * compacted and ids rendered as a JSON array. Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
            "op", "case", "statement", "name", "eventType", "epl", "payload",
            "expectError", "compileWithoutPath", "mode", "ids", "at",
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
            + "|" + string(step, "filterValue") + "|" + string(step, "at");
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
