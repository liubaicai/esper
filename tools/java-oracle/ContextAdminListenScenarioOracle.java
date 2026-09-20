import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierCategory;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierHash;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierInitiatedTerminated;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierNested;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierPartitioned;
import com.espertech.esper.common.client.context.ContextPartitionStateListener;
import com.espertech.esper.common.client.context.ContextStateEventContextActivated;
import com.espertech.esper.common.client.context.ContextStateEventContextCreated;
import com.espertech.esper.common.client.context.ContextStateEventContextDeactivated;
import com.espertech.esper.common.client.context.ContextStateEventContextDestroyed;
import com.espertech.esper.common.client.context.ContextStateEventContextPartitionAllocated;
import com.espertech.esper.common.client.context.ContextStateEventContextPartitionDeallocated;
import com.espertech.esper.common.client.context.ContextStateEventContextStatementAdded;
import com.espertech.esper.common.client.context.ContextStateEventContextStatementRemoved;
import com.espertech.esper.common.client.context.ContextStateListener;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Scenario oracle for ContextAdminListen (ords 2,3,4,5,6). Replays the shared
 * scenario: each case runs on the default runtime (URI "default", the value
 * the Java executions assert on statement and partition events), deploy steps
 * compile the pinned EPL text against the accumulated module path
 * (compileWithoutPath steps compile without it, mirroring the ord-4
 * path-less compileDeploy calls), and labeled SupportContextListener
 * equivalents emit one normalized context-event record per callback.
 * Deployment ids normalize to the owning deploy-step label; partition events
 * carry the partition id and a normalized identifier (category label, nested
 * parent/leaf pair, hash bucket, partitioned keys, or the initiated-
 * terminated initiating event type). The listener re-entrantly registers
 * itself as a partition listener inside onContextCreated and asserts
 * getContextProperties is non-null inside onContextPartitionAllocated,
 * exactly like SupportContextListener. admin:listeners snapshot steps mirror
 * the getContextStateListeners iterator and admin:partition-listeners steps
 * the per-context getContextPartitionStateListeners iterator, emitting the
 * registered listener labels in iteration order. The ord-5 cases register
 * listeners through addContextPartitionStateListener only, so no created
 * event reaches them; the nested case's @now parent level renders an
 * initiatedTerminated identifier with no initiatingEvent.
 */
public class ContextAdminListenScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-admin-listen";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextAdminListen.java";

    private static final String[] CASES = {
        "category",
        "nested",
        "add-remove-listener",
        "partition-add-remove-listener",
        "partition-add-remove-listener-nested",
        "multiple-statements",
    };
    private static final int[] ORDINALS = {2, 3, 4, 5, 5, 6};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-2021f021c6e12684fb81",
        "java-runtime-4b1466f2815a381815b2",
        "java-runtime-410d2c5d3daf011b6c7b",
        "java-runtime-206c08a7f3d6c239050c",
        "java-runtime-206c08a7f3d6c239050c",
        "java-runtime-6dd25578221d74ff6660",
    };
    private static final String[] EXECUTION_NAMES = {
        "ContextAdminListenCategory",
        "ContextAdminListenNested",
        "ContextAddRemoveListener",
        "ContextAdminPartitionAddRemoveListener",
        "ContextAdminPartitionAddRemoveListener",
        "ContextAdminListenMultipleStatements",
    };
    private static final String[] STATIC_IDS = {
        "java-3a026095a61c4060c91b",
        "java-3a026095a61c4060c91b",
        "java-3a026095a61c4060c91b",
        "java-3a026095a61c4060c91b",
        "java-3a026095a61c4060c91b",
        "java-3a026095a61c4060c91b",
    };
    private static final String[] JAVA_FLAGS = {"OBSERVEROPS", "RUNTIMEOPS"};
    private static final String[] CASE_OBSERVATIONS = {
        "context-event; a context-state listener registered before deploy observes created at ctx deploy and re-entrantly registers as a partition listener; deploying s0 into the two-category context eagerly allocates pos then neg (allocated[1] label 'neg') before activated; no events are sent; undeploying s0 then ctx emits statement-removed, two partition-deallocated, deactivated and destroyed",
        "context-event; nested category-over-keyed context: ctx deploy emits created, s0 deploy emits statement-added then activated, SupportBean(\"E1\",1) emits exactly one partition-allocated whose nested identifier carries parent label 'pos' and leaf keys [\"E1\"], s0 undeploy emits statement-removed/partition-deallocated/deactivated and ctx undeploy emits destroyed",
        "context-event+admin; three context-state listeners registered before deploy each observe created; removing l0 before ctx undeploy leaves l0 silent while l1 and l2 observe destroyed; the listener iterator yields [l1,l2] in registration order; remove-listeners empties the registry; a redeploy plus undeploy-all tail invokes no listener",
        "context-event+admin; three partition-state listeners registered after the ctx and s0 deploys each observe one partition-allocated on SupportBean_S0(1) whose initiatedTerminated identifier carries initiatingEvent SupportBean_S0; removing l0 leaves it silent while l1 and l2 observe the partition-deallocated on SupportBean_S1(1); the partition-listener iterator yields [l1,l2] in registration order; remove-all empties the registry; SupportBean_S0(2)/SupportBean_S1(2) silently allocate and deallocate leaf id 1; undeploy-all",
        "context-event+admin; the same partition-listener registry lifecycle under a nested context whose NeverEndingStory parent starts at @now and whose ABSession leaf runs start SupportBean_S0 as s0 end SupportBean_S1; the allocated identifier nests a parent initiatedTerminated level with no initiatingEvent over the leaf's SupportBean_S0 initiatingEvent; the iterator yields [l1,l2], remove-all empties the registry and the S0(2)/S1(2) tail stays silent",
        "context-event; one partition-state listener added after ctx deploy observes statement-added(a), activated, statement-added(b) — activated fires once after the first statement — and SupportBean_S0(1) emits exactly one partition-allocated despite two deployed statements; undeploy-all tears down a, b and the context in dependency order",
    };
    private static final String[] CASE_EPLS = {
        "@name('s0') context MyContext select count(*) from SupportBean",
        "@name('s0') context MyContext select count(*) from SupportBean",
        "@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1",
        "@name('ctx') @public create context MyContextStartEnd start SupportBean_S0 as s0 end SupportBean_S1",
        "@name('ctx') @public create context MyContextStartEndWithNeverEnding context NeverEndingStory start @now, context ABSession start SupportBean_S0 as s0 end SupportBean_S1",
        "@name('a') context MyContextStartS0EndS1 select count(*) from SupportBean",
    };

    /** Pinned per-case step keys: op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids. */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("category", new String[]{
            "add-listener|l0||||||||",
            "deploy|ctx|||@name('ctx') @public create context MyContext group by intPrimitive > 0 as pos, group by intPrimitive < 0 as neg from SupportBean|||||",
            "deploy|s0|||@name('s0') context MyContext select count(*) from SupportBean|||||",
            "deployed|s0||||||||",
            "undeploy|s0||||||||",
            "undeploy|ctx||||||||",
            "remove-listeners|||||||||",
        });
        CASE_STEPS.put("nested", new String[]{
            "add-listener|l0||||||||",
            "deploy|ctx|||@name('ctx') @public create context MyContext context ContextPosNeg group by intPrimitive > 0 as pos, group by intPrimitive < 0 as neg from SupportBean, context ByString partition by theString from SupportBean|||||",
            "deploy|s0|||@name('s0') context MyContext select count(*) from SupportBean|||||",
            "deployed|s0||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||||",
            "undeploy|s0||||||||",
            "undeploy|ctx||||||||",
            "remove-listeners|||||||||",
        });
        CASE_STEPS.put("add-remove-listener", new String[]{
            "add-listener|l0||||||||",
            "add-listener|l1||||||||",
            "add-listener|l2||||||||",
            "deploy|ctx|||@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1|||1||",
            "deployed|ctx||||||||",
            "remove-listener|l0||||||||",
            "undeploy|ctx||||||||",
            "snapshot|ctx|||||||admin:listeners|",
            "remove-listeners|||||||||",
            "snapshot|ctx|||||||admin:listeners|",
            "deploy|ctx|||@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1|||1||",
            "deployed|ctx||||||||",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("multiple-statements", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyContextStartS0EndS1 start SupportBean_S0 as s0 end SupportBean_S1|||||",
            "deployed|ctx||||||||",
            "add-partition-listener|l0|ctx|||||||",
            "deploy|a|||@name('a') context MyContextStartS0EndS1 select count(*) from SupportBean|||||",
            "deployed|a||||||||",
            "deploy|b|||@name('b') context MyContextStartS0EndS1 select count(*) from SupportBean_S0|||||",
            "deployed|b||||||||",
            "send|||SupportBean_S0||{\"id\":1}||||",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("partition-add-remove-listener", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyContextStartEnd start SupportBean_S0 as s0 end SupportBean_S1|||||",
            "deployed|ctx||||||||",
            "deploy|s0|||@name('s0') context MyContextStartEnd select count(*) from SupportBean|||||",
            "deployed|s0||||||||",
            "add-partition-listener|l0|ctx|||||||",
            "add-partition-listener|l1|ctx|||||||",
            "add-partition-listener|l2|ctx|||||||",
            "send|||SupportBean_S0||{\"id\":1}||||",
            "remove-partition-listener|l0|ctx|||||||",
            "send|||SupportBean_S1||{\"id\":1}||||",
            "snapshot||ctx||||||admin:partition-listeners|",
            "remove-partition-listeners||ctx|||||||",
            "snapshot||ctx||||||admin:partition-listeners|",
            "send|||SupportBean_S0||{\"id\":2}||||",
            "send|||SupportBean_S1||{\"id\":2}||||",
            "undeploy-all|||||||||",
        });
        CASE_STEPS.put("partition-add-remove-listener-nested", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyContextStartEndWithNeverEnding context NeverEndingStory start @now, context ABSession start SupportBean_S0 as s0 end SupportBean_S1|||||",
            "deployed|ctx||||||||",
            "deploy|s0|||@name('s0') context MyContextStartEndWithNeverEnding select count(*) from SupportBean|||||",
            "deployed|s0||||||||",
            "add-partition-listener|l0|ctx|||||||",
            "add-partition-listener|l1|ctx|||||||",
            "add-partition-listener|l2|ctx|||||||",
            "send|||SupportBean_S0||{\"id\":1}||||",
            "remove-partition-listener|l0|ctx|||||||",
            "send|||SupportBean_S1||{\"id\":1}||||",
            "snapshot||ctx||||||admin:partition-listeners|",
            "remove-partition-listeners||ctx|||||||",
            "snapshot||ctx||||||admin:partition-listeners|",
            "send|||SupportBean_S0||{\"id\":2}||||",
            "send|||SupportBean_S1||{\"id\":2}||||",
            "undeploy-all|||||||||",
        });
    }

    private static final Pattern CONTEXT_NAME_PATTERN =
        Pattern.compile("create\\s+context\\s+([A-Za-z_][A-Za-z0-9_]*)");

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ContextAdminListenScenarioOracle <scenario.json>");
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
        config.getCompiler().getViewResources().setIterableUnbound(true);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The Java executions assert runtimeURI "default" on statement and
        // partition events, so each case runs on the default runtime; destroy
        // between cases yields a fresh instance under the same URI.
        EPRuntime runtime = EPRuntimeProvider.getDefaultRuntime(config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, String> contextNames = new HashMap<>();
            Map<String, RecordingContextListener> listeners = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();

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
                        deployStep(runtime, config, step,
                            deploymentIds, contextNames, deployedModules, records);
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
                    case "add-listener": {
                        String label = step.getString("statement", "");
                        if (listeners.containsKey(label)) {
                            throw new IllegalStateException("listener " + label + " already registered");
                        }
                        RecordingContextListener listener = new RecordingContextListener(
                            runtime, caseName, label, records, sequences, deploymentIds);
                        runtime.getContextPartitionService().addContextStateListener(listener);
                        listeners.put(label, listener);
                        break;
                    }
                    case "remove-partition-listener": {
                        RecordingContextListener listener = listeners.get(step.getString("statement", ""));
                        if (listener == null) {
                            break;
                        }
                        String contextLabel = step.getString("name", "");
                        String deploymentId = deploymentIds.get(contextLabel);
                        String contextName = contextNames.get(contextLabel);
                        if (deploymentId == null || contextName == null) {
                            throw new IllegalStateException("no context deployment for label " + contextLabel);
                        }
                        runtime.getContextPartitionService().removeContextPartitionStateListener(
                            deploymentId, contextName, listener);
                        break;
                    }
                    case "remove-partition-listeners": {
                        String contextLabel = step.getString("name", "");
                        String deploymentId = deploymentIds.get(contextLabel);
                        String contextName = contextNames.get(contextLabel);
                        if (deploymentId == null || contextName == null) {
                            throw new IllegalStateException("no context deployment for label " + contextLabel);
                        }
                        runtime.getContextPartitionService().removeContextPartitionStateListeners(
                            deploymentId, contextName);
                        break;
                    }
                    case "add-partition-listener": {
                        String label = step.getString("statement", "");
                        if (listeners.containsKey(label)) {
                            throw new IllegalStateException("listener " + label + " already registered");
                        }
                        String contextLabel = step.getString("name", "");
                        String deploymentId = deploymentIds.get(contextLabel);
                        String contextName = contextNames.get(contextLabel);
                        if (deploymentId == null || contextName == null) {
                            throw new IllegalStateException("no context deployment for label " + contextLabel);
                        }
                        RecordingContextListener listener = new RecordingContextListener(
                            runtime, caseName, label, records, sequences, deploymentIds);
                        runtime.getContextPartitionService().addContextPartitionStateListener(
                            deploymentId, contextName, listener);
                        listeners.put(label, listener);
                        break;
                    }
                    case "remove-listener": {
                        RecordingContextListener listener = listeners.get(step.getString("statement", ""));
                        if (listener != null) {
                            runtime.getContextPartitionService().removeContextStateListener(listener);
                        }
                        break;
                    }
                    case "remove-listeners":
                        runtime.getContextPartitionService().removeContextStateListeners();
                        break;
                    case "snapshot":
                        snapshotStep(runtime, caseName, step, records, listeners,
                            deploymentIds, contextNames);
                        break;
                    case "undeploy": {
                        String owner = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(owner);
                        if (deploymentId == null) {
                            throw new IllegalStateException("no deployment for statement " + owner);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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
     * Compiles and deploys one labeled module. compileWithoutPath steps
     * compile without the accumulated module path, mirroring the Java
     * execution's path-less compileDeploy; other deploys accumulate into the
     * path like RegressionPath. Create-context deploys record the context
     * name under the step label so partition-listener steps can resolve the
     * (deploymentId, contextName) pair.
     */
    private static void deployStep(EPRuntime runtime, Configuration config, JsonObject step,
                                   Map<String, String> deploymentIds,
                                   Map<String, String> contextNames, List<EPCompiled> deployedModules,
                                   List<JsonObject> pendingRecords) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            if (!step.getBoolean("compileWithoutPath", false)) {
                for (EPCompiled deployed : deployedModules) {
                    compilerArgs.getPath().add(deployed);
                }
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            if (!step.getBoolean("compileWithoutPath", false)) {
                deployedModules.add(compiled);
            }
            deploymentIds.put(label, deployment.getDeploymentId());
            // Listener callbacks that fired during this deploy recorded the
            // raw UUID before the label map filled; rewrite them now.
            for (JsonObject record : pendingRecords) {
                JsonObject value = record.get("value") != null && record.get("value").isObject()
                    ? record.get("value").asObject() : null;
                if (value == null) {
                    continue;
                }
                for (String field : new String[]{"contextDeploymentId", "statementDeploymentId"}) {
                    JsonValue raw = value.get(field);
                    if (raw != null && raw.isString()
                        && deployment.getDeploymentId().equals(raw.asString())) {
                        value.set(field, label);
                    }
                }
            }
            Matcher matcher = CONTEXT_NAME_PATTERN.matcher(epl);
            if (matcher.find()) {
                contextNames.put(label, matcher.group(1));
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
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
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Emits one {"operation":"admin"} record for the pinned probe:
     * "admin:listeners" mirrors the getContextStateListeners iterator and
     * "admin:partition-listeners" mirrors the per-context
     * getContextPartitionStateListeners iterator, each carrying the
     * registered listener labels in iteration order.
     */
    private static void snapshotStep(EPRuntime runtime, String caseName, JsonObject step,
                                     List<JsonObject> records,
                                     Map<String, RecordingContextListener> listeners,
                                     Map<String, String> deploymentIds,
                                     Map<String, String> contextNames) {
        String mode = step.getString("mode", "");
        JsonArray labels = new JsonArray();
        if ("admin:listeners".equals(mode)) {
            for (Iterator<ContextStateListener> it =
                     runtime.getContextPartitionService().getContextStateListeners(); it.hasNext(); ) {
                labels.add(listenerLabel(listeners, it.next()));
            }
        } else if ("admin:partition-listeners".equals(mode)) {
            String contextLabel = step.getString("name", "");
            String deploymentId = deploymentIds.get(contextLabel);
            String contextName = contextNames.get(contextLabel);
            if (deploymentId == null || contextName == null) {
                throw new IllegalStateException("no context deployment for label " + contextLabel);
            }
            for (Iterator<ContextPartitionStateListener> it =
                     runtime.getContextPartitionService().getContextPartitionStateListeners(
                         deploymentId, contextName); it.hasNext(); ) {
                labels.add(listenerLabel(listeners, it.next()));
            }
        } else {
            throw new IllegalStateException("unknown admin probe " + mode);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "admin");
        String statement = step.getString("statement", "");
        if (!statement.isEmpty()) {
            record.add("statement", statement);
        }
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        String name = step.getString("name", "");
        if (!name.isEmpty()) {
            record.add("name", name);
        }
        JsonObject value = new JsonObject();
        value.add("listeners", labels);
        record.add("value", value);
        records.add(record);
    }

    /** Resolves a registered listener instance back to its step label. */
    private static String listenerLabel(Map<String, RecordingContextListener> listeners,
                                        Object candidate) {
        for (Map.Entry<String, RecordingContextListener> entry : listeners.entrySet()) {
            if (entry.getValue() == candidate) {
                return entry.getKey();
            }
        }
        throw new IllegalStateException("unregistered listener in listener registry iterator");
    }

    /**
     * Normalizes one deployment id to the deploy-step label that created it;
     * unknown ids pass through so the diff reports drift.
     */
    private static String normalizeDeploymentId(Map<String, String> deploymentIds, String deploymentId) {
        for (Map.Entry<String, String> entry : deploymentIds.entrySet()) {
            if (entry.getValue().equals(deploymentId)) {
                return entry.getKey();
            }
        }
        return deploymentId;
    }

    /**
     * Renders the normalized partition identifier: the typed
     * ContextPartitionIdentifier classes map to a type tag plus their pinned
     * payload (category label, nested parent/leaf pair, hash bucket,
     * partitioned keys, or the initiated-terminated initiating event type).
     */
    private static JsonValue renderIdentifier(ContextPartitionIdentifier identifier) {
        JsonObject object = new JsonObject();
        if (identifier instanceof ContextPartitionIdentifierNested) {
            object.add("type", "nested");
            JsonArray identifiers = new JsonArray();
            for (ContextPartitionIdentifier nested :
                    ((ContextPartitionIdentifierNested) identifier).getIdentifiers()) {
                identifiers.add(renderIdentifier(nested));
            }
            object.add("identifiers", identifiers);
            return object;
        }
        if (identifier instanceof ContextPartitionIdentifierCategory) {
            object.add("type", "category");
            object.add("label", ((ContextPartitionIdentifierCategory) identifier).getLabel());
            return object;
        }
        if (identifier instanceof ContextPartitionIdentifierHash) {
            object.add("type", "hash");
            object.add("hash", ((ContextPartitionIdentifierHash) identifier).getHash());
            return object;
        }
        if (identifier instanceof ContextPartitionIdentifierPartitioned) {
            object.add("type", "partitioned");
            JsonArray keys = new JsonArray();
            for (Object key : ((ContextPartitionIdentifierPartitioned) identifier).getKeys()) {
                keys.add(normalize(key));
            }
            object.add("keys", keys);
            return object;
        }
        if (identifier instanceof ContextPartitionIdentifierInitiatedTerminated) {
            object.add("type", "initiatedTerminated");
            Map<String, Object> properties =
                ((ContextPartitionIdentifierInitiatedTerminated) identifier).getProperties();
            Object initiating = properties == null ? null : properties.get("s0");
            if (initiating instanceof EventBean) {
                object.add("initiatingEvent", ((EventBean) initiating).getEventType().getName());
            } else if (initiating != null) {
                object.add("initiatingEvent", String.valueOf(initiating));
            }
            return object;
        }
        object.add("type", identifier == null ? "unknown" : identifier.getClass().getSimpleName());
        return object;
    }

    /**
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * integral numbers as JSON numbers, other numbers as doubles, booleans
     * passthrough, everything else stringified.
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

    /**
     * SupportContextListener equivalent: implements both listener interfaces,
     * re-entrantly registers itself as a partition listener inside
     * onContextCreated, asserts getContextProperties is non-null inside
     * onContextPartitionAllocated, and emits one normalized context-event
     * record per callback.
     */
    private static class RecordingContextListener implements ContextStateListener, ContextPartitionStateListener {
        private final EPRuntime runtime;
        private final String caseName;
        private final String label;
        private final List<JsonObject> records;
        private final Map<String, Integer> sequences;
        private final Map<String, String> deploymentIds;

        RecordingContextListener(EPRuntime runtime, String caseName, String label,
                                 List<JsonObject> records, Map<String, Integer> sequences,
                                 Map<String, String> deploymentIds) {
            this.runtime = runtime;
            this.caseName = caseName;
            this.label = label;
            this.records = records;
            this.sequences = sequences;
            this.deploymentIds = deploymentIds;
        }

        public void onContextCreated(ContextStateEventContextCreated event) {
            record("created", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, null, null);
            runtime.getContextPartitionService().addContextPartitionStateListener(
                event.getContextDeploymentId(), event.getContextName(), this);
        }

        public void onContextDestroyed(ContextStateEventContextDestroyed event) {
            record("destroyed", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, null, null);
        }

        public void onContextActivated(ContextStateEventContextActivated event) {
            record("activated", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, null, null);
        }

        public void onContextDeactivated(ContextStateEventContextDeactivated event) {
            record("deactivated", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, null, null);
        }

        public void onContextStatementAdded(ContextStateEventContextStatementAdded event) {
            record("statement-added", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), event.getStatementDeploymentId(), event.getStatementName(),
                null, null);
        }

        public void onContextStatementRemoved(ContextStateEventContextStatementRemoved event) {
            record("statement-removed", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), event.getStatementDeploymentId(), event.getStatementName(),
                null, null);
        }

        public void onContextPartitionAllocated(ContextStateEventContextPartitionAllocated event) {
            if (runtime.getContextPartitionService().getContextProperties(
                    event.getContextDeploymentId(), event.getContextName(), event.getId()) == null) {
                throw new IllegalStateException("context properties for " + event.getContextName()
                    + " id " + event.getId() + " not queryable in allocated callback");
            }
            record("partition-allocated", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, event.getId(), event.getIdentifier());
        }

        public void onContextPartitionDeallocated(ContextStateEventContextPartitionDeallocated event) {
            record("partition-deallocated", event.getRuntimeURI(), event.getContextDeploymentId(),
                event.getContextName(), null, null, event.getId(), null);
        }

        private void record(String kind, String runtimeURI, String contextDeploymentId,
                            String contextName, String statementDeploymentId, String statementName,
                            Integer partitionId, ContextPartitionIdentifier identifier) {
            int sequence = sequences.getOrDefault(label, 0) + 1;
            sequences.put(label, sequence);
            JsonObject value = new JsonObject();
            value.add("event", kind);
            value.add("runtimeURI", runtimeURI);
            value.add("contextDeploymentId", normalizeDeploymentId(deploymentIds, contextDeploymentId));
            value.add("contextName", contextName);
            if (statementName != null) {
                value.add("statementName", statementName);
                value.add("statementDeploymentId",
                    normalizeDeploymentId(deploymentIds, statementDeploymentId));
            }
            if (partitionId != null) {
                value.add("partitionId", partitionId);
            }
            if (identifier != null) {
                value.add("identifier", renderIdentifier(identifier));
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "context-event");
            record.add("name", label);
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            record.add("value", value);
            records.add(record);
        }
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
