import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
import com.espertech.esper.common.internal.util.DeploymentIdNamePair;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.variable.EPLVariablesEventTyped;
import com.espertech.esper.regressionlib.support.bean.SupportBeanAtoFBase;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportBean_S3;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPDeploymentDependencyConsumed;
import com.espertech.esper.runtime.client.EPDeploymentDependencyProvided;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.client.util.EPObjectType;

import java.lang.reflect.Array;
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
import java.util.TreeMap;

/**
 * Scenario oracle for EPLVariablesEventTyped (all six executions). Replays the
 * shared scenario: each case runs on a fresh runtime configured with the
 * suite's event types and the seven preconfigured global variables of
 * TestSuiteEPLVariable (vars0_A/vars1_A/varsobj1/vars2/vars3/varsobj2/
 * myNonSerializable). Deploy steps compile the pinned EPL text with the
 * accumulated module path, listeners attach to 's0', 'set', 'set-two' and
 * 'Select' on every deploy (the Java execution re-adds the 'set' listener on
 * each redeploy), read-variable steps emit getVariableValue records (with the
 * bulk-map read cross-checked internally), set-variable steps write through
 * the variable service (array payloads are the bulk DeploymentIdNamePair map
 * form), snapshot steps emit statement iterator rows, and error steps pin the
 * "type-mismatch" category token while asserting the exact Java message
 * internally (declared-type rendering differs between event-type-name and
 * class-name declarations). The ord-5 'variable' deploy additionally asserts
 * the consumed/provided EVENTTYPE dependency items internally since the Go
 * surface cannot model a variable-declaration event-type edge.
 */
public class EPLVariablesEventTypedScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-variables-event-typed";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesEventTyped.java";

    private static final String[] CASES = {
        "event-typed-scene-one",
        "event-typed-scene-two",
        "event-typed-config",
        "event-typed-set-prop",
        "event-typed-invalid",
        "event-typed-create-schema",
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-0ccc8cc9831c8681b7b1",
        "java-runtime-683d12f7c34c448319da",
        "java-runtime-cc00e385af1428a74014",
        "java-runtime-067180db58b5432f57dc",
        "java-runtime-400881e0dc41c6d3e4e4",
        "java-runtime-0794ac05da19ce34dbaf",
    };
    private static final String[] EXECUTION_NAMES = {
        "EPLVariableEventTypedSceneOne",
        "EPLVariableEventTypedSceneTwo",
        "EPLVariableConfig",
        "EPLVariableEventTypedSetProp",
        "EPLVariableInvalid",
        "EPLVariableEventTypedCreateSchema",
    };
    private static final String[] STATIC_IDS = {
        "java-0d1b65aff5dcb7751e5b",
        "java-0d1b65aff5dcb7751e5b",
        "java-0d1b65aff5dcb7751e5b",
        "java-0d1b65aff5dcb7751e5b",
        "java-0d1b65aff5dcb7751e5b",
        "java-0d1b65aff5dcb7751e5b",
    };
    private static final String[] JAVA_FLAGS = {"SERDEREQUIRED", "RUNTIMEOPS", "INVALIDITY"};
    private static final String[] CASE_OBSERVATIONS = {
        "listener+variable+snapshot; Object/bean/event-type variables read null until API-set, an on-SupportBean_S0(p00='X') set assigns 1 to the Object var, the arrival event to the event-typed var and null to the bean var (listener and iterator both observe {1,S0(2,'X'),null}), a bulk set nulls all three, a second bulk set restores {10L,A('A1'),S0(2,'X')}, and an on-SupportBean_A(id='Y') set assigns the arrival bean",
        "listener; a five-variable module (two bean-typed, one event-typed, two boxed longs) feeds a seven-column select that reads null until API-set, an on-SupportBean_B set mutates the held bean's theString/intPrimitive to {'EX',-999}, and an on-SupportBean(intPrimitive=0) set replaces the variable's whole event so the select reads {'E2',0,...}",
        "variable; preconfigured global variables read back their seeded values (SupportBean_S0 id 10, SupportBean_S1 id 20, constant 123, NonSerializable 'abc', then SupportBean_S2 id 30, SupportBean_S3 id 40, constant 'ABC') and a deployment-scoped create variable object varsobj3=222 reads 222",
        "listener+variable+snapshot; set-prop on a null bean variable is a silent no-op, after an API-set bean the on-set writes emit varbean.theString/varbean.intPrimitive columns {'A',1} observed by listener, iterator and dependent select, the write is copy-on-write (the variable holds a new bean), sequential assignments self-evaluate to '>E3<', and an int literal widens into longPrimitive",
        "set-variable-error+compile-error; a runtime SupportBean_S1 write into event-typed vars0_A and compile probes assigning arrival/int to mismatched variables all fail with the type-mismatch category (declared-type rendering differs between event-type-name and class-name declarations, so the category token is pinned)",
        "listener; a create-schema OrderEvent deployment feeds an event-typed variable whose on-OrderEvent assignment is read back as orderEvent.orderId c0='O1' then 'O2' on SupportBean sends (the variable-declaration EVENTTYPE dependency edge is unmodelable in Go and asserted only inside the oracle)",
    };
    private static final String[] CASE_EPLS = {
        "@name('s0') select varobject, varbean, varbean.id, vartype, vartype.id from SupportBean",
        "@Name('Select') select varbean.theString as c0,varbean.intPrimitive as c1,vars0.id as c2,vars0.p00 as c3,varobj as c4,varbeannull.theString as c5, varobjnull as c6 from SupportBean_A",
        "@name('create') create variable object varsobj3=222",
        "@name('set') on SupportBean_A set varbean.theString = 'A', varbean.intPrimitive = 1",
        "on SupportBean_S0 arrival set vars1_A = arrival",
        "on OrderEvent as oe set orderEvent = oe;\n@name('s0') select orderEvent.orderId as c0 from SupportBean;\n",
    };

    /** Exact Java messages asserted internally for the ord-4 probes. */
    private static final String MSG_SET_VARS0_A =
        "Variable 'vars0_A' of declared event type 'SupportBean_S0' underlying type '"
            + SupportBean_S0.class.getName() + "' cannot be assigned a value of type '"
            + SupportBean_S1.class.getName() + "'";
    private static final String MSG_COMPILE_VARS1_A =
        "Failed to validate assignment expression 'vars1_A=arrival': Variable 'vars1_A' of declared event type '"
            + SupportBean_S1.class.getName() + "' underlying type '" + SupportBean_S1.class.getName()
            + "' cannot be assigned a value of type '" + SupportBean_S0.class.getName() + "'";
    private static final String MSG_COMPILE_VARS0_A =
        "Failed to validate assignment expression 'vars0_A=1': Variable 'vars0_A' of declared event type 'SupportBean_S0' underlying type '"
            + SupportBean_S0.class.getName() + "' cannot be assigned a value of type 'int'";

    private static final EPLVariablesEventTyped.NonSerializable NON_SERIALIZABLE =
        EPLVariablesEventTyped.NON_SERIALIZABLE;

    /** Pinned per-case step keys: op|statement|name|eventType|epl|payload|expectError|compileWithoutPath. */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("event-typed-scene-one", new String[]{
            "deploy|v0|||@name('v0') @public create variable Object varobject = null|||",
            "deployed|v0||||||",
            "deploy|v1|||@name('v1') @public create variable com.espertech.esper.regressionlib.support.bean.SupportBean_A varbean = null|||",
            "deployed|v1||||||",
            "deploy|v2|||@name('v2') @public create variable SupportBean_S0 vartype = null|||",
            "deployed|v2||||||",
            "deploy|s0|||@name('s0') select varobject, varbean, varbean.id, vartype, vartype.id from SupportBean|||",
            "deployed|s0||||||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "set-variable|v0|varobject|||\"abc\"||",
            "set-variable|v1|varbean|||{\"type\":\"SupportBean_A\",\"value\":{\"id\":\"A1\"}}||",
            "set-variable|v2|vartype|||{\"type\":\"SupportBean_S0\",\"value\":{\"id\":1}}||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "deploy|set|||@name('set') on SupportBean_S0(p00='X') arrival set varobject=1, vartype=arrival, varbean=null|||",
            "deployed|set||||||",
            "send|||SupportBean_S0||{\"id\":2,\"p00\":\"X\"}||",
            "read-variable|v0|varobject|||||",
            "read-variable|v2|vartype|||||",
            "read-variable|v2|vartype|||||",
            "snapshot|set||||||",
            "set-variable||bulk|||[{\"deployment\":\"v0\",\"name\":\"varobject\",\"value\":null},{\"deployment\":\"v2\",\"name\":\"vartype\",\"value\":null},{\"deployment\":\"v1\",\"name\":\"varbean\",\"value\":null}]||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "set-variable||bulk|||[{\"deployment\":\"v0\",\"name\":\"varobject\",\"value\":{\"type\":\"long\",\"value\":10}},{\"deployment\":\"v2\",\"name\":\"vartype\",\"value\":{\"type\":\"SupportBean_S0\",\"value\":{\"id\":2,\"p00\":\"X\"}}},{\"deployment\":\"v1\",\"name\":\"varbean\",\"value\":{\"type\":\"SupportBean_A\",\"value\":{\"id\":\"A1\"}}}]||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "deploy|set-two|||@name('set-two') on SupportBean_A(id='Y') arrival set varobject=null, vartype=null, varbean=arrival|||",
            "deployed|set-two||||||",
            "send|||SupportBean_A||{\"id\":\"Y\"}||",
            "read-variable|v0|varobject|||||",
            "read-variable|v2|vartype|||||",
            "read-variable|v1|varbean|||||",
            "snapshot|set-two||||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("event-typed-scene-two", new String[]{
            "deploy|vars|||@name('vars') @public create variable com.espertech.esper.common.internal.support.SupportBean varbeannull;\n@public create variable com.espertech.esper.common.internal.support.SupportBean varbean;\n@public create variable SupportBean_S0 vars0;\n@public create variable long varobj;\n@public create variable long varobjnull;\n|||",
            "deployed|vars||||||",
            "deploy|Select|||@Name('Select') select varbean.theString as c0,varbean.intPrimitive as c1,vars0.id as c2,vars0.p00 as c3,varobj as c4,varbeannull.theString as c5, varobjnull as c6 from SupportBean_A|||",
            "deployed|Select||||||",
            "send|||SupportBean_A||{\"id\":\"A1\"}||",
            "set-variable|vars|varobj|||{\"type\":\"long\",\"value\":101}||",
            "set-variable|vars|vars0|||{\"type\":\"SupportBean_S0\",\"value\":{\"id\":1,\"p00\":\"S01\"}}||",
            "set-variable|vars|varbean|||{\"type\":\"SupportBean\",\"value\":{\"theString\":\"E1\",\"intPrimitive\":-1}}||",
            "send|||SupportBean_A||{\"id\":\"A2\"}||",
            "deploy|Update|||@Name('Update') on SupportBean_B set varbean.theString = 'EX', varbean.intPrimitive = -999|||",
            "deployed|Update||||||",
            "send|||SupportBean_B||{\"id\":\"B1\"}||",
            "send|||SupportBean_A||{\"id\":\"A3\"}||",
            "deploy|Update2|||@Name('Update2') on SupportBean(intPrimitive = 0) as sb set varbean = sb|||",
            "deployed|Update2||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":0}||",
            "send|||SupportBean_A||{\"id\":\"A4\"}||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("event-typed-config", new String[]{
            "read-variable||vars0_A|||||",
            "read-variable||vars1_A|||||",
            "read-variable||varsobj1|||||",
            "read-variable||myNonSerializable|||||",
            "read-variable||vars2|||||",
            "read-variable||vars3|||||",
            "read-variable||varsobj2|||||",
            "deploy|create|||@name('create') create variable object varsobj3=222|||",
            "deployed|create||||||",
            "read-variable|create|varsobj3|||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("event-typed-set-prop", new String[]{
            "deploy|create|||@name('create') @public create variable SupportBean varbean|||",
            "deployed|create||||||",
            "deploy|s0|||@name('s0') select varbean.theString,varbean.intPrimitive,varbean.getTheString() from SupportBean_S0|||",
            "deployed|s0||||||",
            "send|||SupportBean_S0||{\"id\":1}||",
            "deploy|set|||@name('set') on SupportBean_A set varbean.theString = 'A', varbean.intPrimitive = 1|||",
            "deployed|set||||||",
            "send|||SupportBean_A||{\"id\":\"E1\"}||",
            "send|||SupportBean_S0||{\"id\":2}||",
            "set-variable|create|varbean|||{\"type\":\"SupportBean\",\"value\":{}}||",
            "send|||SupportBean_A||{\"id\":\"E2\"}||",
            "snapshot|s0||||||",
            "send|||SupportBean_S0||{\"id\":3}||",
            "read-variable|create|varbean|||||",
            "undeploy|set||||||",
            "deploy|set|||@name('set') on SupportBean_A set varbean.theString = SupportBean_A.id, varbean.theString = '>'||varbean.theString||'<'|||",
            "deployed|set||||||",
            "send|||SupportBean_A||{\"id\":\"E3\"}||",
            "read-variable|create|varbean|||||",
            "undeploy|set||||||",
            "deploy|set|||@name('set') on SupportBean_A set varbean.longPrimitive = 1|||",
            "deployed|set||||||",
            "send|||SupportBean_A||{\"id\":\"E4\"}||",
            "read-variable|create|varbean|||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("event-typed-invalid", new String[]{
            "set-variable||vars0_A|||{\"type\":\"SupportBean_S1\",\"value\":{\"id\":1}}|type-mismatch|",
            "build-error|set-vars1_A|||on SupportBean_S0 arrival set vars1_A = arrival||type-mismatch|",
            "build-error|set-vars0_A|||on SupportBean_S0 arrival set vars0_A = 1||type-mismatch|",
        });
        CASE_STEPS.put("event-typed-create-schema", new String[]{
            "deploy|schema|||@buseventtype @public @name('schema') create schema OrderEvent(orderId string);|||",
            "deployed|schema||||||",
            "deploy|variable|||@public @name('variable') create variable OrderEvent orderEvent;|||",
            "deployed|variable||||||",
            "deploy|onset|||on OrderEvent as oe set orderEvent = oe;\n@name('s0') select orderEvent.orderId as c0 from SupportBean;\n|||",
            "deployed|onset||||||",
            "send|||OrderEvent||{\"orderId\":\"O1\"}||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "send|||OrderEvent||{\"orderId\":\"O2\"}||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "undeploy-all|||||||",
        });
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLVariablesEventTypedScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_A.class);
        config.getCommon().addEventType(SupportBean_B.class);
        // Suite-level preconfigured global variables (TestSuiteEPLVariable:122-129).
        config.getCommon().addVariable("vars0_A", "SupportBean_S0", new SupportBean_S0(10));
        config.getCommon().addVariable("vars1_A", SupportBean_S1.class.getName(), new SupportBean_S1(20));
        config.getCommon().addVariable("varsobj1", Object.class.getName(), 123, true);
        config.getCommon().addVariable("vars2", "SupportBean_S2", new SupportBean_S2(30));
        config.getCommon().addVariable("vars3", SupportBean_S3.class, new SupportBean_S3(40));
        config.getCommon().addVariable("varsobj2", Object.class, "ABC", true);
        config.getCommon().addVariable("myNonSerializable", EPLVariablesEventTyped.NonSerializable.class, NON_SERIALIZABLE);
        // TestSuiteEPLVariable:133 — the suite compiles with iterableUnbound
        // so unwindowed selects iterate the transformed last event.
        config.getCompiler().getViewResources().setIterableUnbound(true);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesEventTypedScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
            // The bean most recently assigned to 'varbean' through the runtime
            // API in event-typed-set-prop: read-variable steps assert the
            // variable holds a copy-on-write copy, never the same instance.
            SupportBean[] lastSetVarbean = new SupportBean[1];

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
                            deploymentIds, statementsByName, deployedModules, sequences);
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
                        sendStep(runtime, caseName, step);
                        break;
                    case "read-variable":
                        readVariable(runtime, deploymentIds, caseName, step, records, lastSetVarbean);
                        break;
                    case "set-variable":
                        setVariableStep(runtime, deploymentIds, caseName, step, records, lastSetVarbean);
                        break;
                    case "snapshot":
                        snapshotStep(runtime, statementsByName, caseName, step, records);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, config, caseName, step, records);
                        break;
                    case "undeploy": {
                        String owner = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(owner);
                        if (deploymentId == null) {
                            throw new IllegalStateException("no deployment for statement " + owner);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        deploymentIds.remove(owner);
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

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module with the accumulated module
     * path, mirroring the Java execution's RegressionPath compileDeploy
     * calls. Listeners attach to 's0', 'set', 'set-two' and 'Select' on every
     * deploy — the Java execution re-adds the 'set' listener on each redeploy
     * of ord 3. The ord-5 'variable' deploy asserts the consumed/provided
     * EVENTTYPE dependency items internally (unmodelable on the Go side).
     */
    private static void deployStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                   List<JsonObject> records, Map<String, String> deploymentIds,
                                   Map<String, EPStatement> statementsByName, List<EPCompiled> deployedModules,
                                   Map<String, Integer> sequences) {
        String label = step.getString("statement", "");
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            deployedModules.add(compiled);
            deploymentIds.put(label, deployment.getDeploymentId());
            for (EPStatement stmt : deployment.getStatements()) {
                statementsByName.put(stmt.getName(), stmt);
                String name = stmt.getName();
                if ("s0".equals(name) || "set".equals(name) || "set-two".equals(name) || "Select".equals(name)) {
                    stmt.addListener(listener(caseName, name, sequences, records, runtime));
                }
            }
            if ("event-typed-create-schema".equals(caseName) && "variable".equals(label)) {
                assertVariableDeploymentDependencies(runtime, deploymentIds.get("schema"), deployment.getDeploymentId());
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    /**
     * Mirrors the ord-5 dependency assertions: the variable deployment
     * consumes exactly the EVENTTYPE OrderEvent provided by the schema
     * deployment, and the schema deployment provides it to exactly the
     * variable deployment.
     */
    private static void assertVariableDeploymentDependencies(EPRuntime runtime, String deployIdSchema, String deployIdVariable) {
        EPDeploymentDependencyConsumed consumed =
            runtime.getDeploymentService().getDeploymentDependenciesConsumed(deployIdVariable);
        if (consumed.getDependencies().size() != 1) {
            throw new IllegalStateException("variable deployment consumed " + consumed.getDependencies().size() + " dependencies");
        }
        EPDeploymentDependencyConsumed.Item consumedItem = consumed.getDependencies().iterator().next();
        if (!deployIdSchema.equals(consumedItem.getDeploymentId())
            || consumedItem.getObjectType() != EPObjectType.EVENTTYPE
            || !"OrderEvent".equals(consumedItem.getObjectName())) {
            throw new IllegalStateException("variable deployment consumed unexpected dependency " + consumedItem);
        }
        EPDeploymentDependencyProvided provided =
            runtime.getDeploymentService().getDeploymentDependenciesProvided(deployIdSchema);
        if (provided.getDependencies().size() != 1) {
            throw new IllegalStateException("schema deployment provided " + provided.getDependencies().size() + " dependencies");
        }
        EPDeploymentDependencyProvided.Item providedItem = provided.getDependencies().iterator().next();
        if (providedItem.getObjectType() != EPObjectType.EVENTTYPE
            || !"OrderEvent".equals(providedItem.getObjectName())
            || !Collections.singleton(deployIdVariable).equals(providedItem.getDeploymentIds())) {
            throw new IllegalStateException("schema deployment provided unexpected dependency " + providedItem);
        }
    }

    /** Sends one event: bean payloads construct the pinned bean, OrderEvent sends a map. */
    private static void sendStep(EPRuntime runtime, String caseName, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                JsonValue theString = payload.get("theString");
                JsonValue intPrimitive = payload.get("intPrimitive");
                SupportBean bean = new SupportBean(
                    theString == null || theString.isNull() ? null : theString.asString(),
                    intPrimitive == null || intPrimitive.isNull() ? 0 : intPrimitive.asInt());
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                int id = payload.get("id") == null || payload.get("id").isNull() ? 0 : payload.get("id").asInt();
                JsonValue p00 = payload.get("p00");
                SupportBean_S0 bean = p00 == null || p00.isNull()
                    ? new SupportBean_S0(id)
                    : new SupportBean_S0(id, p00.asString());
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(new SupportBean_A(payload.getString("id", null)), type);
                break;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(new SupportBean_B(payload.getString("id", null)), type);
                break;
            case "OrderEvent": {
                Map<String, Object> map = new HashMap<>();
                for (String name : payload.names()) {
                    JsonValue value = payload.get(name);
                    map.put(name, value.isNull() ? null : value.asString());
                }
                runtime.getEventService().sendEventMap(map, type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Emits {"operation":"variable","name","value"} mirroring getVariableValue.
     * Deployment-scoped reads additionally cross-check the bulk map form, and
     * ord-3 varbean reads assert the copy-on-write instance (never the bean
     * passed to setVariableValue).
     */
    private static void readVariable(EPRuntime runtime, Map<String, String> deploymentIds,
                                     String caseName, JsonObject step, List<JsonObject> records,
                                     SupportBean[] lastSetVarbean) {
        String name = step.getString("name", "");
        String owner = step.getString("statement", "");
        String deploymentId = null;
        if (!owner.isEmpty()) {
            deploymentId = deploymentIds.get(owner);
            if (deploymentId == null) {
                throw new IllegalStateException("no deployment for statement " + owner);
            }
        }
        Object value = runtime.getVariableService().getVariableValue(deploymentId, name);
        if (deploymentId != null) {
            // Mirror the Java execution's bulk getVariableValue(Set) reads.
            DeploymentIdNamePair pair = new DeploymentIdNamePair(deploymentId, name);
            Object bulk = runtime.getVariableService()
                .getVariableValue(Collections.singleton(pair)).get(pair);
            if (value == null ? bulk != null : !value.equals(bulk)) {
                throw new IllegalStateException("bulk getVariableValue disagrees for " + name);
            }
        }
        if ("event-typed-set-prop".equals(caseName) && "varbean".equals(name) && lastSetVarbean[0] != null) {
            if (value == lastSetVarbean[0]) {
                throw new IllegalStateException("varbean holds the API-assigned instance; expected a copy-on-write copy");
            }
            if (!(value instanceof SupportBean)) {
                throw new IllegalStateException("varbean is not a SupportBean: " + value);
            }
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "variable");
        record.add("sequence", 0);
        record.add("name", name);
        if (value == null) {
            record.add("value", new JsonObject().add("state", "null"));
        } else {
            record.add("value", renderVariableValue(value));
        }
        records.add(record);
    }

    /**
     * Executes one runtime variable write. An array payload is the bulk form:
     * ordered {deployment,name,value} entries collected into the
     * DeploymentIdNamePair map, mirroring setVariableValue(Map). Steps
     * carrying expectError assert the rejection internally (the exact Java
     * message for the vars0_A probe) and emit the pinned category token;
     * plain writes stay silent exactly like the Java execution.
     */
    private static void setVariableStep(EPRuntime runtime, Map<String, String> deploymentIds, String caseName,
                                        JsonObject step, List<JsonObject> records, SupportBean[] lastSetVarbean) {
        String expected = step.getString("expectError", "");
        String owner = step.getString("statement", "");
        String deploymentId = owner.isEmpty() ? null : deploymentIds.get(owner);
        JsonValue payload = step.get("payload");
        String caught = "<no-error>";
        try {
            if (payload != null && payload.isArray()) {
                Map<DeploymentIdNamePair, Object> newValues = new HashMap<>();
                for (JsonValue entryVal : payload.asArray()) {
                    JsonObject entry = entryVal.asObject();
                    String entryDeployment = entry.getString("deployment", "");
                    String entryDeploymentId = entryDeployment.isEmpty() ? null : deploymentIds.get(entryDeployment);
                    if (!entryDeployment.isEmpty() && entryDeploymentId == null) {
                        throw new IllegalStateException("no deployment for statement " + entryDeployment);
                    }
                    newValues.put(new DeploymentIdNamePair(entryDeploymentId, entry.getString("name", "")),
                        decodeAssignedValue(entry.get("value")));
                }
                runtime.getVariableService().setVariableValue(newValues);
            } else {
                Object value = decodeAssignedValue(payload);
                if ("event-typed-set-prop".equals(caseName) && "varbean".equals(name(step))
                        && value instanceof SupportBean) {
                    lastSetVarbean[0] = (SupportBean) value;
                }
                runtime.getVariableService().setVariableValue(deploymentId, step.getString("name", ""), value);
            }
        } catch (RuntimeException ex) {
            caught = rootCauseMessage(ex);
        }
        if (!expected.isEmpty()) {
            if ("type-mismatch".equals(expected)) {
                // The pinned category: verify the rejection is the Java
                // VariableValueException with the exact asserted message.
                if (!MSG_SET_VARS0_A.equals(caught)) {
                    throw new IllegalStateException("set-variable message drift for case "
                        + caseName + ": expected [" + MSG_SET_VARS0_A + "] got [" + caught + "]");
                }
            } else if (!expected.equals(caught)) {
                throw new IllegalStateException("set-variable message drift for case "
                    + caseName + ": expected [" + expected + "] got [" + caught + "]");
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "set-variable-error");
            record.add("sequence", 0);
            record.add("value", expected);
            records.add(record);
        }
    }

    private static String name(JsonObject step) {
        return step.getString("name", "");
    }

    /**
     * Compiles an expected-invalid statement with no module path (Java's
     * tryInvalidCompile compiles with a null path). The exact Java message is
     * asserted internally; the record carries the pinned category token.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration config, String caseName,
                                       JsonObject step, List<JsonObject> records) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        String message;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
            caught = "<no-error>";
            message = null;
        } catch (Exception ex) {
            caught = rootCauseMessage(ex);
            message = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
        }
        if ("type-mismatch".equals(expected)) {
            String pinned = "set-vars1_A".equals(label) ? MSG_COMPILE_VARS1_A : MSG_COMPILE_VARS0_A;
            // Java's SupportMessageAssertUtil.assertMessage is a startsWith
            // check: the exception appends the EPL text after the message.
            if (message == null || !message.startsWith(pinned)) {
                throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + pinned + "] got [" + message + "]");
            }
        } else if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("compile-error message drift for " + label
                + ": expected [" + expected + "] got [" + caught + "]");
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

    /** Emits {"operation":"snapshot"} mirroring assertIterator on the statement. */
    private static void snapshotStep(EPRuntime runtime, Map<String, EPStatement> statementsByName,
                                     String caseName, JsonObject step, List<JsonObject> records) {
        String label = step.getString("statement", "");
        EPStatement statement = statementsByName.get(label);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot " + label);
        }
        List<EventBean> events = new ArrayList<>();
        java.util.Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            events.add(iterator.next());
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        JsonArray newRows = rows(events.toArray(new EventBean[0]));
        if (newRows.size() > 0) {
            record.add("new", newRows);
        }
        records.add(record);
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
     * characters passthrough, arrays elementwise, beans as canonical sorted
     * property objects, everything else stringified.
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
        if (value.getClass().isArray()) {
            return renderArrayValue(value);
        }
        if (value instanceof Iterable) {
            JsonArray array = new JsonArray();
            for (Object element : (Iterable<?>) value) {
                array.add(normalize(element));
            }
            return array;
        }
        JsonValue bean = renderBeanValue(value);
        if (bean != null) {
            return bean;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Canonical variable-value rendering: numbers long-truncated except
     * float/double which keep their double value, strings/booleans/chars
     * passthrough, arrays elementwise, collections elementwise, beans as
     * canonical sorted property objects.
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
        if (value.getClass().isArray()) {
            return renderArrayValue(value);
        }
        if (value instanceof Iterable) {
            JsonArray array = new JsonArray();
            for (Object element : (Iterable<?>) value) {
                array.add(renderVariableValue(element));
            }
            return array;
        }
        JsonValue bean = renderBeanValue(value);
        if (bean != null) {
            return bean;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Canonical bean rendering over the Java-observed property surface:
     * SupportBean renders theString/intPrimitive/longPrimitive, SupportBean_S0
     * renders id plus p00..p03 (its value-equals surface), the id-only beans
     * render id, and NonSerializable renders myString. Returns null for
     * non-bean values so callers keep their legacy paths.
     */
    private static JsonValue renderBeanValue(Object value) {
        TreeMap<String, JsonValue> members = new TreeMap<>();
        if (value instanceof SupportBean bean) {
            members.put("intPrimitive", Json.value((long) bean.getIntPrimitive()));
            members.put("longPrimitive", Json.value(bean.getLongPrimitive()));
            members.put("theString", bean.getTheString() == null ? Json.NULL : Json.value(bean.getTheString()));
        } else if (value instanceof SupportBean_S0 bean) {
            members.put("id", Json.value((long) bean.getId()));
            members.put("p00", bean.getP00() == null ? Json.NULL : Json.value(bean.getP00()));
            members.put("p01", bean.getP01() == null ? Json.NULL : Json.value(bean.getP01()));
            members.put("p02", bean.getP02() == null ? Json.NULL : Json.value(bean.getP02()));
            members.put("p03", bean.getP03() == null ? Json.NULL : Json.value(bean.getP03()));
        } else if (value instanceof SupportBean_S1 bean) {
            members.put("id", Json.value((long) bean.getId()));
        } else if (value instanceof SupportBean_S2 bean) {
            members.put("id", Json.value((long) bean.getId()));
        } else if (value instanceof SupportBean_S3 bean) {
            members.put("id", Json.value((long) bean.getId()));
        } else if (value instanceof SupportBeanAtoFBase bean) {
            members.put("id", bean.getId() == null ? Json.NULL : Json.value(bean.getId()));
        } else if (value instanceof EPLVariablesEventTyped.NonSerializable bean) {
            members.put("myString", Json.value(bean.getMyString()));
        } else {
            return null;
        }
        JsonObject object = new JsonObject();
        for (Map.Entry<String, JsonValue> entry : members.entrySet()) {
            object.add(entry.getKey(), entry.getValue());
        }
        return object;
    }

    /**
     * Canonical array rendering: elements as Number→long/double, null→null,
     * nested arrays via String.valueOf (Java's "[e1, e2]" Object[] toString).
     */
    private static JsonArray renderArrayValue(Object value) {
        JsonArray array = new JsonArray();
        for (int i = 0; i < Array.getLength(value); i++) {
            Object element = Array.get(value, i);
            if (element == null) {
                array.add(Json.NULL);
            } else if (element instanceof Integer || element instanceof Long
                    || element instanceof Short || element instanceof Byte) {
                array.add(((Number) element).longValue());
            } else if (element instanceof Number) {
                array.add(((Number) element).doubleValue());
            } else if (element instanceof Boolean) {
                array.add(((Boolean) element).booleanValue());
            } else if (element.getClass().isArray()) {
                array.add(javaArrayString(element));
            } else {
                array.add(String.valueOf(element));
            }
        }
        return array;
    }

    /** Java Object[] toString rendering: "[e1, e2]" with element recursion. */
    private static String javaArrayString(Object array) {
        StringBuilder buf = new StringBuilder("[");
        for (int i = 0; i < Array.getLength(array); i++) {
            if (i > 0) {
                buf.append(", ");
            }
            Object element = Array.get(array, i);
            buf.append(element != null && element.getClass().isArray()
                ? javaArrayString(element) : String.valueOf(element));
        }
        return buf.append(']').toString();
    }

    /**
     * Decodes one assignment value. Tagged objects {"type","value"} pin the
     * bean shape or boxed width so runtime type-mismatch messages name the
     * same Java type on both sides; bare numbers follow Java autoboxing
     * semantics (Integer).
     */
    private static Object decodeAssignedValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value instanceof JsonObject) {
            JsonObject tag = value.asObject();
            String type = tag.getString("type", "");
            JsonValue inner = tag.get("value");
            switch (type) {
                case "long":
                    return inner.asLong();
                case "SupportBean": {
                    if (inner == null || inner.isNull()) {
                        return new SupportBean();
                    }
                    JsonObject bean = inner.asObject();
                    JsonValue theString = bean.get("theString");
                    JsonValue intPrimitive = bean.get("intPrimitive");
                    return new SupportBean(
                        theString == null || theString.isNull() ? null : theString.asString(),
                        intPrimitive == null || intPrimitive.isNull() ? 0 : intPrimitive.asInt());
                }
                case "SupportBean_S0": {
                    JsonObject bean = inner.asObject();
                    int id = bean.get("id") == null || bean.get("id").isNull() ? 0 : bean.get("id").asInt();
                    JsonValue p00 = bean.get("p00");
                    return p00 == null || p00.isNull()
                        ? new SupportBean_S0(id)
                        : new SupportBean_S0(id, p00.asString());
                }
                case "SupportBean_S1":
                    return new SupportBean_S1(inner.asObject().get("id").asInt());
                case "SupportBean_A":
                    return new SupportBean_A(inner.asObject().getString("id", null));
                default:
                    throw new IllegalStateException("unknown assignment type tag " + type);
            }
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

    /** Renders one step as its pinned key; unknown fields are rejected. */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
            "op", "case", "statement", "name", "eventType", "epl", "payload",
            "expectError", "compileWithoutPath"));
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
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
            + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payload
            + "|" + string(step, "expectError") + "|" + cwp;
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
