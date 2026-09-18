import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionSelectorById;
import com.espertech.esper.common.client.context.ContextPartitionVariableState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.variable.VariableNotFoundException;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
import com.espertech.esper.common.internal.util.DeploymentIdNamePair;
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
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for ContextVariables (ords 0-4). Replays the shared
 * scenario: each case runs on a fresh runtime, deploy steps compile the
 * pinned EPL text against the accumulated module path (compileWithoutPath
 * steps compile without it, mirroring the Java execution's path-less
 * compileDeploy of the ord-1 module tail), listener records capture IR pairs
 * for 's0'/'upd' and the create-variable statement 'var' (attached only where
 * the Java execution calls addListener), snapshot steps iterate the named
 * statement (the 'var' iterator yields one row per live partition), and
 * read-variable/set-variable steps exercise the partition-scoped variable
 * service by context partition id. Build-error probes compile with the path
 * like env.tryInvalidCompile(path, ...) and assert the pinned message prefix;
 * the record value carries the asserted prefix. The reclaim_group_aged hint
 * probe of ContextVariablesInvalid has no expression surface in the Go API,
 * so the oracle asserts it internally without a scenario step.
 */
public class ContextVariablesScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-variables";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextVariables.java";

    private static final String[] CASES = {
        "segmented-by-key",
        "overlapping",
        "iterate-and-listen",
        "get-set-api",
        "invalid",
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-700973f399356686a11e",
        "java-runtime-821c37bbc3256af33468",
        "java-runtime-ea85235554e99c41e863",
        "java-runtime-4d98fe6487ed39e7d1de",
        "java-runtime-c7b860b3ff30ed78f3f5",
    };
    private static final String[] EXECUTION_NAMES = {
        "ContextVariablesSegmentedByKey",
        "ContextVariablesOverlapping",
        "ContextVariablesIterateAndListen",
        "ContextVariablesGetSetAPI",
        "ContextVariablesInvalid",
    };
    private static final String[] STATIC_IDS = {
        "java-2443c804eb31da7902e7",
        "java-2443c804eb31da7902e7",
        "java-2443c804eb31da7902e7",
        "java-2443c804eb31da7902e7",
        "java-2443c804eb31da7902e7",
    };
    private static final String[] JAVA_FLAGS = {"RUNTIMEOPS"};
    private static final String[] CASE_OBSERVATIONS = {
        "listener; an uncorrelated on-set fires only for the partition the event keys into, allocating it if absent: SupportBean(\"P1\",0) allocates without setting, SupportBean(\"P2\",11) allocates and sets in one event, and S0(5,\"P3\") allocates P3 which reads back the initial 0",
        "listener; correlated on-set writes the initiating partition while the uncorrelated intPrimitive<0 on-set writes every live partition (P1 reads -1 after the P2-targeted write); terminated partitions reset to the initial 5 on re-initiation; the module tail deploys and undeploys a distinct-initiator context with an integer-typed variable",
        "listener+snapshot; the create-variable statement listener fires one IR pair per set (new=assigned, old=previous including the initial 5) only for the updated partition and stays silent on partition creation; iterators on 'upd' and 'var' yield one row per live partition, 'upd' ordered by partition id and 'var' any-order",
        "variable+set-variable-error+variable-error; partition-id-addressed variable get/set (partition 0 reads 5 then 10, partition 1 reads 5 then 11) and VariableNotFoundException for a global variable via the partition-scoped APIs",
        "compile-error; probes pin context-not-found, wrong-context variable, and out-of-context variable use in select, expr-window, limit, offset and output-every positions; the reclaim_group_aged hint probe is asserted inside the oracle only (Go hints are string-typed with no expression surface)",
    };
    private static final String[] CASE_EPLS = {
        "@name('s0') context MyCtx select mycontextvar from SupportBean_S0",
        "@name('s0') context MyCtx select mycontextvar from SupportBean_S2(p20 = context.s0.p00)",
        "@name('upd') context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive",
        "context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive",
        "context MyCtx create variable int mycontext_invalid1 = 0",
    };

    /** Pinned per-case step keys: op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids. */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("segmented-by-key", new String[]{
            "deploy|ctx|||@public create context MyCtx as partition by theString from SupportBean, p00 from SupportBean_S0|||||",
            "deployed|ctx||||||||",
            "deploy|var|||@public context MyCtx create variable int mycontextvar = 0|||||",
            "deployed|var||||||||",
            "deploy|upd|||context MyCtx on SupportBean(intPrimitive > 0) set mycontextvar = intPrimitive|||||",
            "deployed|upd||||||||",
            "deploy|s0|||@name('s0') context MyCtx select mycontextvar from SupportBean_S0|||||",
            "deployed|s0||||||||",
            "send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":0}||||",
            "send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":10}||||",
            "send|||SupportBean_S0||{\"id\":1,\"p00\":\"P1\"}||||",
            "send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":11}||||",
            "send|||SupportBean_S0||{\"id\":2,\"p00\":\"P2\"}||||",
            "send|||SupportBean_S0||{\"id\":3,\"p00\":\"P1\"}||||",
            "send|||SupportBean_S0||{\"id\":4,\"p00\":\"P2\"}||||",
            "send|||SupportBean_S0||{\"id\":5,\"p00\":\"P3\"}||||",
            "send|||SupportBean||{\"theString\":\"P3\",\"intPrimitive\":12}||||",
            "send|||SupportBean_S0||{\"id\":6,\"p00\":\"P3\"}||||",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("overlapping", new String[]{
            "deploy|ctx|||@public create context MyCtx as initiated by SupportBean_S0 s0 terminated by SupportBean_S1(p10 = s0.p00)|||||",
            "deployed|ctx||||||||",
            "deploy|var|||@public context MyCtx create variable int mycontextvar = 5|||||",
            "deployed|var||||||||",
            "deploy|upd|||context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
            "deployed|upd||||||||",
            "deploy|upd-all|||context MyCtx on SupportBean(intPrimitive < 0) set mycontextvar = intPrimitive|||||",
            "deployed|upd-all||||||||",
            "deploy|s0|||@name('s0') context MyCtx select mycontextvar from SupportBean_S2(p20 = context.s0.p00)|||||",
            "deployed|s0||||||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
            "send|||SupportBean_S2||{\"id\":1,\"p20\":\"P1\"}||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
            "send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":10}||||",
            "send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
            "send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":-1}||||",
            "send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
            "send|||SupportBean_S2||{\"id\":2,\"p20\":\"P1\"}||||",
            "send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":20}||||",
            "send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":21}||||",
            "send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
            "send|||SupportBean_S2||{\"id\":2,\"p20\":\"P1\"}||||",
            "send|||SupportBean_S1||{\"id\":0,\"p10\":\"P1\"}||||",
            "send|||SupportBean_S1||{\"id\":0,\"p10\":\"P2\"}||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
            "send|||SupportBean_S2||{\"id\":1,\"p20\":\"P1\"}||||",
            "undeploy-all|||||||||",
            "deploy|module|||@Name(\"context\")\ncreate context MyContext\ninitiated by distinct(theString) SupportBean as input\nterminated by SupportBean(theString = input.theString);\n\n@Name(\"ctx variable counter\")\ncontext MyContext create variable integer counter = 0;\n|||1||",
            "deployed|module||||||||",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("iterate-and-listen", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyCtx as initiated by SupportBean_S0 s0 terminated after 24 hours|||||",
            "deployed|ctx||||||||",
            "deploy|var|||@name('var') @public context MyCtx create variable int mycontextvar = 5|||||",
            "deployed|var||||||||",
            "deploy|upd|||@name('upd') context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
            "deployed|upd||||||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
            "send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":100}||||",
            "snapshot|upd||||||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
            "send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":101}||||",
            "snapshot|upd||||||||",
            "snapshot|var|||||||any|",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("get-set-api", new String[]{
            "deploy|ctx|||@public create context MyCtx as initiated by SupportBean_S0 s0 terminated after 24 hours|||||",
            "deployed|ctx||||||||",
            "deploy|var|||@name('var') @public context MyCtx create variable int mycontextvar = 5|||||",
            "deployed|var||||||||",
            "deploy|upd|||context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
            "deployed|upd||||||||",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
            "read-variable|var|mycontextvar|||||||[0]",
            "set-variable|var|mycontextvar|||10||||[0]",
            "read-variable|var|mycontextvar|||||||[0]",
            "send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
            "read-variable|var|mycontextvar|||||||[1]",
            "set-variable|var|mycontextvar|||11||||[1]",
            "read-variable|var|mycontextvar|||||||[1]",
            "deploy|globalvar|||@name('globalvar') create variable int myglobarvar = 0|||||",
            "deployed|globalvar||||||||",
            "set-variable|globalvar|myglobarvar|||11|Variable by name 'myglobarvar' is a global variable and not context-partitioned|||[0]",
            "read-variable|globalvar|myglobarvar||||Variable by name 'myglobarvar' is a global variable and not context-partitioned|||[1]",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("invalid", new String[]{
            "deploy|ctx-one|||@public create context MyCtxOne as partition by theString from SupportBean|||||",
            "deployed|ctx-one||||||||",
            "deploy|ctx-two|||@public create context MyCtxTwo as partition by p00 from SupportBean_S0|||||",
            "deployed|ctx-two||||||||",
            "deploy|var|||@public context MyCtxOne create variable int myctxone_int = 0|||||",
            "deployed|var||||||||",
            "build-error|invalid-context|||context MyCtx create variable int mycontext_invalid1 = 0||Context by name 'MyCtx' could not be found|||",
            "build-error|wrong-context|||context MyCtxTwo select myctxone_int from SupportBean_S0||Variable 'myctxone_int' defined for use with context 'MyCtxOne' is not available for use with context 'MyCtxTwo'|||",
            "build-error|outside-context-select|||select myctxone_int from SupportBean_S0||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
            "build-error|outside-context-expr-window|||select * from SupportBean_S0#expr(myctxone_int > 5)||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
            "build-error|outside-context-limit|||select * from SupportBean_S0#keepall limit myctxone_int||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
            "build-error|outside-context-offset|||select * from SupportBean_S0#keepall limit 10 offset myctxone_int||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
            "build-error|outside-context-output-every|||select * from SupportBean_S0#keepall output every myctxone_int events||Failed to validate the output rate limiting clause: Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
            "undeploy-all|||||||||",
        });
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ContextVariablesScenarioOracle <scenario.json>");
            System.exit(2);
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
        config.getCommon().addEventType(SupportBean_S2.class);
        config.getCompiler().getViewResources().setIterableUnbound(true);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ContextVariablesScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
            Set<String> listened = new HashSet<>();

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
                            deploymentIds, statementsByName, deployedModules, sequences, listened);
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
                        snapshotStep(caseName, step, records, statementsByName, runtime);
                        break;
                    case "read-variable":
                        readVariable(runtime, deploymentIds, caseName, step, records);
                        break;
                    case "set-variable":
                        setVariableStep(runtime, deploymentIds, caseName, step, records);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, config, caseName, step, records, deployedModules);
                        break;
                    case "undeploy": {
                        String owner = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(owner);
                        if (deploymentId == null) {
                            throw new IllegalStateException("no deployment for statement " + owner);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        deploymentIds.remove(owner);
                        statementsByName.remove(owner);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        statementsByName.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + op);
                }
            }

            if ("invalid".equals(caseName)) {
                // ContextVariablesInvalid's eighth probe exercises a context
                // variable inside a reclaim_group_aged hint parameter. The Go
                // API types hints as strings with no expression surface, so
                // the probe is asserted here only (approved difference).
                tryInvalidCompile(config, deployedModules,
                    "@Hint('reclaim_group_aged=myctxone_int') select longPrimitive, count(*) from SupportBean group by longPrimitive",
                    "Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context");
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module. compileWithoutPath steps compile
     * without the accumulated module path, mirroring the Java execution's
     * path-less compileDeploy of the ord-1 module tail; other deploys
     * accumulate into the path like RegressionPath. Listeners attach to the
     * statements the Java execution listens on: 's0' in segmented-by-key and
     * overlapping, 'var' and 'upd' in iterate-and-listen.
     */
    private static void deployStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                   List<JsonObject> records, Map<String, String> deploymentIds,
                                   Map<String, EPStatement> statementsByName, List<EPCompiled> deployedModules,
                                   Map<String, Integer> sequences, Set<String> listened) {
        String label = step.getString("statement", "");
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            if (!step.getBoolean("compileWithoutPath", false)) {
                for (EPCompiled deployed : deployedModules) {
                    compilerArgs.getPath().add(deployed);
                }
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            if (!step.getBoolean("compileWithoutPath", false)) {
                deployedModules.add(compiled);
            }
            deploymentIds.put(label, deployment.getDeploymentId());
            for (EPStatement stmt : deployment.getStatements()) {
                statementsByName.put(stmt.getName(), stmt);
                String name = stmt.getName();
                if (listenedNames(caseName).contains(name) && !listened.contains(name)) {
                    listened.add(name);
                    stmt.addListener(listener(caseName, name, sequences, records, runtime));
                }
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    /** The statement names each case listens on, mirroring env.addListener calls. */
    private static Set<String> listenedNames(String caseName) {
        switch (caseName) {
            case "segmented-by-key":
            case "overlapping":
                return new HashSet<>(Collections.singletonList("s0"));
            case "iterate-and-listen":
                return new HashSet<>(Arrays.asList("var", "upd"));
            default:
                return Collections.emptySet();
        }
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
            case "SupportBean_S2": {
                JsonValue p20 = payload.get("p20");
                runtime.getEventService().sendEventBean(
                    new SupportBean_S2(payload.get("id").asInt(),
                        p20 == null || p20.isNull() ? null : p20.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Iterates the named statement, mirroring assertPropsPerRowIterator /
     * assertPropsPerRowIteratorAnyOrder: one {"operation":"snapshot"} record
     * whose rows carry the statement's per-partition output.
     */
    private static void snapshotStep(String caseName, JsonObject step, List<JsonObject> records,
                                     Map<String, EPStatement> statementsByName, EPRuntime runtime) {
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
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
            rows.add(row(it.next()));
        }
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        records.add(record);
    }

    /**
     * Reads one context-partitioned variable through the partition-scoped
     * variable service, mirroring assertVariableValues: the result map must
     * contain exactly one entry whose single state carries the value. Steps
     * carrying expectError assert the VariableNotFoundException message and
     * emit {"operation":"variable-error"} with the pinned text.
     */
    private static void readVariable(EPRuntime runtime, Map<String, String> deploymentIds,
                                     String caseName, JsonObject step, List<JsonObject> records) {
        String name = step.getString("name", "");
        String owner = step.getString("statement", "");
        String deploymentId = deploymentIds.get(owner);
        if (deploymentId == null) {
            throw new IllegalStateException("no deployment for statement " + owner);
        }
        DeploymentIdNamePair pair = new DeploymentIdNamePair(deploymentId, name);
        String expected = step.getString("expectError", "");
        if (!expected.isEmpty()) {
            String caught;
            try {
                runtime.getVariableService().getVariableValue(
                    Collections.singleton(pair), selectorById(step));
                caught = "<no-error>";
            } catch (VariableNotFoundException ex) {
                caught = ex.getMessage();
            }
            if (!expected.equals(caught)) {
                throw new IllegalStateException("read-variable message drift for case "
                    + caseName + ": expected [" + expected + "] got [" + caught + "]");
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "variable-error");
            record.add("statement", owner);
            record.add("sequence", 0);
            record.add("name", name);
            record.add("value", caught);
            records.add(record);
            return;
        }
        Map<DeploymentIdNamePair, List<ContextPartitionVariableState>> states =
            runtime.getVariableService().getVariableValue(
                Collections.singleton(pair), selectorById(step));
        if (states.size() != 1) {
            throw new IllegalStateException("variable states map size " + states.size() + " for " + name);
        }
        List<ContextPartitionVariableState> list = states.get(pair);
        if (list == null || list.size() != 1) {
            throw new IllegalStateException("variable states list for " + name + " is not a single state");
        }
        ContextPartitionVariableState state = list.get(0);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "variable");
        record.add("statement", owner);
        record.add("sequence", 0);
        record.add("name", name);
        Object value = state.getState();
        if (value == null) {
            record.add("value", new JsonObject().add("state", "null"));
        } else {
            record.add("value", renderVariableValue(value));
        }
        records.add(record);
    }

    /**
     * Writes one context-partitioned variable through the partition-scoped
     * variable service, mirroring setVariableValue(map, agentInstanceId).
     * Steps carrying expectError assert the VariableNotFoundException message
     * and emit {"operation":"set-variable-error"} with the pinned text; plain
     * writes stay silent exactly like the Java execution.
     */
    private static void setVariableStep(EPRuntime runtime, Map<String, String> deploymentIds, String caseName,
                                        JsonObject step, List<JsonObject> records) {
        String name = step.getString("name", "");
        String owner = step.getString("statement", "");
        String deploymentId = deploymentIds.get(owner);
        if (deploymentId == null) {
            throw new IllegalStateException("no deployment for statement " + owner);
        }
        DeploymentIdNamePair pair = new DeploymentIdNamePair(deploymentId, name);
        int agentInstanceId = singleId(step);
        String expected = step.getString("expectError", "");
        String caught = "<no-error>";
        try {
            runtime.getVariableService().setVariableValue(
                Collections.singletonMap(pair, decodeAssignedValue(step.get("payload"))), agentInstanceId);
        } catch (VariableNotFoundException ex) {
            caught = ex.getMessage();
        }
        if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("set-variable message drift for case "
                + caseName + ": expected [" + expected + "] got [" + caught + "]");
        }
        if (!expected.isEmpty()) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "set-variable-error");
            record.add("statement", owner);
            record.add("sequence", 0);
            record.add("name", name);
            record.add("value", caught);
            records.add(record);
        }
    }

    /**
     * Compiles an expected-invalid statement with the accumulated module path,
     * mirroring env.tryInvalidCompile(path, epl, message): the compile must
     * fail and the message must start with the pinned prefix. The record value
     * carries the asserted prefix so the Go trace can pin the same text.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration config, String caseName,
                                       JsonObject step, List<JsonObject> records,
                                       List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
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

    /** Compiles an expected-invalid statement without emitting a record. */
    private static void tryInvalidCompile(Configuration config, List<EPCompiled> deployedModules,
                                          String epl, String expectedPrefix) {
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (EPCompileException ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("internal invalid probe unexpectedly compiled");
        }
        if (!caught.startsWith(expectedPrefix)) {
            throw new IllegalStateException("internal invalid probe message drift: expected prefix ["
                + expectedPrefix + "] got [" + caught + "]");
        }
    }

    /** Builds a by-id context partition selector from the step's ids field. */
    private static ContextPartitionSelectorById selectorById(JsonObject step) {
        Set<Integer> ids = new HashSet<>();
        JsonValue raw = step.get("ids");
        if (raw != null && raw.isArray()) {
            for (JsonValue id : raw.asArray()) {
                ids.add(id.asInt());
            }
        }
        return () -> ids;
    }

    /** Reads the single partition id pinned by a set-variable step. */
    private static int singleId(JsonObject step) {
        JsonValue raw = step.get("ids");
        if (raw == null || !raw.isArray() || raw.asArray().size() != 1) {
            throw new IllegalStateException("set-variable requires exactly one partition id");
        }
        return raw.asArray().get(0).asInt();
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
     * integral numbers as JSON numbers, other numbers as doubles, booleans and
     * characters passthrough, everything else stringified.
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

    /**
     * Canonical variable-value rendering: numbers long-truncated except
     * float/double which keep their double value, strings/booleans/chars
     * passthrough, collections elementwise.
     */
    private static JsonValue renderVariableValue(Object value) {
        if (value == null) {
            return Json.NULL;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof String) {
            return Json.value((String) value);
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
                array.add(renderVariableValue(element));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Decodes one assignment value; bare numbers follow Java autoboxing (Integer). */
    private static Object decodeAssignedValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isNumber()) {
            return value.asInt();
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.isString()) {
            return value.asString();
        }
        throw new IllegalStateException("unsupported assignment payload " + value);
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
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids
     * with the payload compacted and ids rendered as a JSON array. Unknown
     * fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
            "op", "case", "statement", "name", "eventType", "epl", "payload",
            "expectError", "compileWithoutPath", "mode", "ids"));
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
            + "|" + string(step, "mode") + "|" + ids;
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
