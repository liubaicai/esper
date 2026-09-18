import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for EPLVariablesCreate (ords 0,1,2,3,5,6; ord 4 is
 * compile-failure-only and covered by Go probes). Replays the shared scenario:
 * each case runs on a fresh runtime, deploy steps compile the pinned EPL text
 * (compileWithoutPath steps compile without the accumulated module path,
 * mirroring the Java execution's path-less compileDeploy calls), listener
 * records capture IR pairs for 's0' and the create-variable statements
 * 'create-one'/'create-two' (attached on first deploy only, mirroring
 * addListener), read-variable steps emit getVariableValue records, and
 * set-variable steps write through the variable service with expectError
 * steps recording the canonical VariableValueException text.
 */
public class EPLVariablesCreateScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-variables-create";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesCreate.java";

    private static final String[] CASES = {
        "variable-om",
        "variable-compile-start-stop",
        "variable-subscribe-iterate",
        "variable-declaration-select",
        "variable-dimension-primitive",
        "variable-generic-type",
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 5, 6};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-67e6441b95a4ca30917b",
        "java-runtime-74285cbcf0d7aeabf02f",
        "java-runtime-06466d91981a6c410b39",
        "java-runtime-75cb3e01ac92790b5194",
        "java-runtime-71775e7db7d1ffd875ba",
        "java-runtime-4ac4b7b111d10a20f74d",
    };
    private static final String[] EXECUTION_NAMES = {
        "EPLVariableOM",
        "EPLVariableCompileStartStop",
        "EPLVariableSubscribeAndIterate",
        "EPLVariableDeclarationAndSelect",
        "EPLVariableDimensionAndPrimitive",
        "EPLVariableGenericType",
    };
    private static final String[] STATIC_IDS = {
        "java-6a445f399d99f5d4785d",
        "java-6a445f399d99f5d4785d",
        "java-6a445f399d99f5d4785d",
        "java-6a445f399d99f5d4785d",
        "java-6a445f399d99f5d4785d",
        "java-6a445f399d99f5d4785d",
    };
    private static final String[] JAVA_FLAGS = {"RUNTIMEOPS"};
    private static final String[] CASE_OBSERVATIONS = {
        "listener; two create-variable deployments (uninitialized long, string initialized to \"abc\") feed a select that observes new=[null,\"abc\"] on the first SupportBean; the SODA toEPL assertions are API-only approved differences",
        "listener+variable; select over two module variables, then ESPER-545: an on-pattern set increments module variable FOO to 1, undeploy-all plus redeploy of the create module resets FOO to 0, and a failed compile leaves no residue so the same private create redeploys",
        "listener+variable; create-variable statements deliver committed writes as IR pairs (new=current, old=previous), the on-set assignments apply sequentially so var2SAI sees the new var1SAI, iterator reads track current values, and redeploying create-two resets it to 20 while create-one survives at 400",
        "listener; 29 create-variable modules pin the declaration typing/coercion matrix (string-to-int, expression folding, equality-as-initializer, char/byte/short/float widths, uninitialized nulls) projected by one select",
        "variable+set-variable-error; runtime set/get over int[primitive], int[], Object[] and Object[][] variables: exact-order reads after writes, and rejections for String[] into int[], int[] into Integer[]/Object[]/Object[][] (primitive arrays are not covariant)",
        "listener+variable; a List<String> variable initialized to [a,b] reads back in exact order and projects c0=[a,b] plus the enum-where filtered c1=[a] on a SupportBean send",
    };
    private static final String[] CASE_EPLS = {
        "@name('s0') select var1OMCreate, var2OMCreate from SupportBean",
        "@name('create') @public create variable int FOO = 0",
        "@name('set') on SupportBean set var1SAI = intPrimitive * 2, var2SAI = var1SAI + 1",
        "@name('s0') select varX1,varX2,varX3,varX4,varX5,varX6,varX7,varX8,varX9,varX10,varX11,varX12,varX13,varX14,varX15,varX16,varX17,varX18,varX19,varX20,varX21,varX22,varX23,varX24,varX25,varX26,varX27,varX28,varX29 from SupportBean",
        "@name('vars') create variable int[primitive] int_prim = null;\ncreate variable int[] int_boxed = null;\ncreate variable java.lang.Object[] objectarray = null;\ncreate variable java.lang.Object[][] objectarray_2dim = null;\n",
        "@name('var') create variable List<String> mylist = Arrays.asList('a', 'b');\n@name('s0') select mylist as c0, mylist.where(v => v = 'a') as c1 from SupportBean;\n",
    };

    /** The 29-variable declaration matrix of EPLVariableDeclarationAndSelect. */
    private static final Object[][] DECLARATIONS = {
        {"varX1", "int", "1"},
        {"varX2", "int", "'2'"},
        {"varX3", "INTEGER", " 3+2 "},
        {"varX4", "bool", " true|false "},
        {"varX5", "boolean", " varX1=1 "},
        {"varX6", "double", " 1.11 "},
        {"varX7", "double", " 1.20d "},
        {"varX8", "Double", " ' 1.12 ' "},
        {"varX9", "float", " 1.13f*2f "},
        {"varX10", "FLOAT", " -1.14f "},
        {"varX11", "string", " ' XXXX ' "},
        {"varX12", "string", " \"a\" "},
        {"varX13", "character", "'a'"},
        {"varX14", "char", "'x'"},
        {"varX15", "short", " 20 "},
        {"varX16", "SHORT", " ' 9 ' "},
        {"varX17", "long", " 20*2 "},
        {"varX18", "LONG", " ' 9 ' "},
        {"varX19", "byte", " 20*2 "},
        {"varX20", "BYTE", "9+1"},
        {"varX21", "int", null},
        {"varX22", "bool", null},
        {"varX23", "double", null},
        {"varX24", "float", null},
        {"varX25", "string", null},
        {"varX26", "char", null},
        {"varX27", "short", null},
        {"varX28", "long", null},
        {"varX29", "BYTE", null},
    };


    /** Pinned per-case step keys: op|statement|name|eventType|epl|payload|expectError|compileWithoutPath. */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("variable-om", new String[]{
            "deploy|create-var1|||@public create variable long var1OMCreate|||",
            "deployed|create-var1||||||",
            "deploy|create-var2|||@public create variable string var2OMCreate = \"abc\"|||",
            "deployed|create-var2||||||",
            "deploy|s0|||@name('s0') select var1OMCreate, var2OMCreate from SupportBean|||",
            "deployed|s0||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||",
            "deploy|create-arrdouble|||create variable double[] arrdouble = {1.0d,2.0d}|||1",
            "deployed|create-arrdouble||||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("variable-compile-start-stop", new String[]{
            "deploy|create-var1|||@public create variable long var1CSS|||",
            "deployed|create-var1||||||",
            "deploy|create-var2|||@public create variable string var2CSS = \"abc\"|||",
            "deployed|create-var2||||||",
            "deploy|s0|||@name('s0') select var1CSS, var2CSS from SupportBean|||",
            "deployed|s0||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||",
            "deploy|create|||@name('create') @public create variable int FOO = 0|||",
            "deployed|create||||||",
            "deploy|set|||on pattern [every SupportBean] set FOO = FOO + 1|||",
            "deployed|set||||||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "read-variable|create|FOO|||||",
            "undeploy-all|||||||",
            "deploy|create|||@name('create') @public create variable int FOO = 0|||1",
            "deployed|create||||||",
            "read-variable|create|FOO|||||",
            "deploy|create-x|||@private create variable int x = 123|||1",
            "deployed|create-x||||||",
            "build-error|missing-script|||select missingScript(x) from SupportBean|||",
            "deploy|create-x2|||@private create variable int x = 123|||1",
            "deployed|create-x2||||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("variable-subscribe-iterate", new String[]{
            "deploy|create-one|||@name('create-one') @public create variable long var1SAI = null|||",
            "deployed|create-one||||||",
            "read-variable|create-one|var1SAI|||||",
            "deploy|create-two|||@name('create-two') @public create variable long var2SAI = 20|||",
            "deployed|create-two||||||",
            "read-variable|create-two|var2SAI|||||",
            "deploy|set|||@name('set') on SupportBean set var1SAI = intPrimitive * 2, var2SAI = var1SAI + 1|||",
            "deployed|set||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}||",
            "read-variable|create-one|var1SAI|||||",
            "read-variable|create-two|var2SAI|||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":200}||",
            "read-variable|create-one|var1SAI|||||",
            "read-variable|create-two|var2SAI|||||",
            "undeploy|set||||||",
            "undeploy|create-two||||||",
            "deploy|create-two|||@name('create-two') @public create variable long var2SAI = 20|||1",
            "deployed|create-two||||||",
            "read-variable|create-one|var1SAI|||||",
            "read-variable|create-two|var2SAI|||||",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("variable-declaration-select", declarationSelectSteps());
        CASE_STEPS.put("variable-dimension-primitive", new String[]{
            "deploy|vars|||@name('vars') create variable int[primitive] int_prim = null;\ncreate variable int[] int_boxed = null;\ncreate variable java.lang.Object[] objectarray = null;\ncreate variable java.lang.Object[][] objectarray_2dim = null;\n|||",
            "deployed|vars||||||",
            "set-variable|vars|int_prim|||{\"type\":\"int-array\",\"value\":[1,2]}||",
            "read-variable|vars|int_prim|||||",
            "set-variable|vars|int_prim|||{\"type\":\"string-array\",\"value\":[]}|Variable 'int_prim' of declared type int[] cannot be assigned a value of type String[]|",
            "set-variable|vars|int_boxed|||{\"type\":\"integer-array\",\"value\":[1,2]}||",
            "read-variable|vars|int_boxed|||||",
            "set-variable|vars|int_boxed|||{\"type\":\"int-array\",\"value\":[]}|Variable 'int_boxed' of declared type Integer[] cannot be assigned a value of type int[]|",
            "set-variable|vars|objectarray|||{\"type\":\"integer-array\",\"value\":[1,2]}||",
            "read-variable|vars|objectarray|||||",
            "set-variable|vars|objectarray|||{\"type\":\"int-array\",\"value\":[]}|Variable 'objectarray' of declared type Object[] cannot be assigned a value of type int[]|",
            "set-variable|vars|objectarray_2dim|||{\"type\":\"object-array-2dim\",\"value\":[[1,2]]}||",
            "read-variable|vars|objectarray_2dim|||||",
            "set-variable|vars|objectarray_2dim|||{\"type\":\"int-array\",\"value\":[]}|Variable 'objectarray_2dim' of declared type Object[][] cannot be assigned a value of type int[]|",
            "undeploy-all|||||||",
        });
        CASE_STEPS.put("variable-generic-type", new String[]{
            "deploy|var|||@name('var') create variable List<String> mylist = Arrays.asList('a', 'b');\n@name('s0') select mylist as c0, mylist.where(v => v = 'a') as c1 from SupportBean;\n|||",
            "deployed|var||||||",
            "read-variable|var|mylist|||||",
            "send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
            "undeploy-all|||||||",
        });
    }

    /** Builds the pinned step keys for the 29-declaration case. */
    private static String[] declarationSelectSteps() {
        List<String> keys = new ArrayList<>();
        for (Object[] decl : DECLARATIONS) {
            String epl = "@public create variable " + decl[1] + " " + decl[0];
            if (decl[2] != null) {
                epl += " = " + decl[2];
            }
            keys.add("deploy|create-" + decl[0] + "|||" + epl + "|||");
            keys.add("deployed|create-" + decl[0] + "||||||");
        }
        StringBuilder buf = new StringBuilder("@name('s0') select ");
        String delimiter = "";
        for (Object[] decl : DECLARATIONS) {
            buf.append(delimiter).append(decl[0]);
            delimiter = ",";
        }
        buf.append(" from SupportBean");
        keys.add("deploy|s0|||" + buf + "|||");
        keys.add("deployed|s0||||||");
        keys.add("send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||");
        keys.add("undeploy-all|||||||");
        return keys.toArray(new String[0]);
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLVariablesCreateScenarioOracle <scenario.json>");
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
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesCreateScenarioOracle-" + caseName, config);
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
                    case "send": {
                        String type = step.getString("eventType", "");
                        if (!"SupportBean".equals(type)) {
                            throw new IllegalStateException("unknown type: " + type);
                        }
                        JsonObject payload = step.get("payload").asObject();
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

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module. compileWithoutPath steps compile
     * without the accumulated module path, mirroring the Java execution's
     * path-less compileDeploy calls (redeployed create modules and private
     * creates); other deploys accumulate into the path like RegressionPath.
     * Listeners attach to 's0', 'create-one' and 'create-two' on first deploy
     * only — Java's redeploy of create-two does not re-add the listener.
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
                if (("s0".equals(name) || "create-one".equals(name) || "create-two".equals(name))
                        && !listened.contains(name)) {
                    listened.add(name);
                    stmt.addListener(listener(caseName, name, sequences, records, runtime));
                }
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    /** Emits {"operation":"variable","name","value"} mirroring getVariableValue. */
    private static void readVariable(EPRuntime runtime, Map<String, String> deploymentIds,
                                     String caseName, JsonObject step, List<JsonObject> records) {
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
     * Executes one runtime variable write. Steps carrying expectError attempt
     * the write and emit {"operation":"set-variable-error"} with the caught
     * root-cause message ("&lt;no-error&gt;" if it unexpectedly succeeds);
     * plain writes stay silent exactly like the Java execution.
     */
    private static void setVariableStep(EPRuntime runtime, Map<String, String> deploymentIds, String caseName,
                                        JsonObject step, List<JsonObject> records) {
        String expected = step.getString("expectError", "");
        String owner = step.getString("statement", "");
        String deploymentId = owner.isEmpty() ? null : deploymentIds.get(owner);
        String caught = "<no-error>";
        try {
            Object value = decodeAssignedValue(step.get("payload"));
            runtime.getVariableService().setVariableValue(deploymentId, step.getString("name", ""), value);
        } catch (RuntimeException ex) {
            caught = rootCauseMessage(ex);
        }
        if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("set-variable message drift for case "
                + caseName + ": expected [" + expected + "] got [" + caught + "]");
        }
        if (!expected.isEmpty()) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "set-variable-error");
            record.add("sequence", 0);
            record.add("value", caught);
            records.add(record);
        }
    }

    /** Compiles an expected-invalid statement; emits {"operation":"compile-error"}. */
    private static void buildErrorStep(EPRuntime runtime, Configuration config, String caseName,
                                       JsonObject step, List<JsonObject> records,
                                       List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            // Java's tryInvalidCompile compiles with no path
            // (compileWCheckedEx(epl) -> null path), so build-error probes do
            // not see the accumulated module path.
            CompilerArguments compilerArgs = new CompilerArguments(config);
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = rootCauseMessage(ex);
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
        }
        if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("compile-error message drift for " + label
                + ": expected [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", caught);
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
     * characters passthrough, arrays elementwise, everything else stringified.
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
        return Json.value(String.valueOf(value));
    }

    /**
     * Canonical variable-value rendering: numbers long-truncated except
     * float/double which keep their double value, strings/booleans/chars
     * passthrough, arrays elementwise, collections elementwise.
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
        return Json.value(String.valueOf(value));
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
     * array shape so runtime type-mismatch messages name the same Java type on
     * both sides; bare numbers follow Java autoboxing semantics (Integer).
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
                case "int-array": {
                    JsonArray elements = inner.asArray();
                    int[] array = new int[elements.size()];
                    for (int i = 0; i < elements.size(); i++) {
                        array[i] = elements.get(i).asInt();
                    }
                    return array;
                }
                case "integer-array": {
                    JsonArray elements = inner.asArray();
                    Integer[] array = new Integer[elements.size()];
                    for (int i = 0; i < elements.size(); i++) {
                        array[i] = elements.get(i).isNull() ? null : elements.get(i).asInt();
                    }
                    return array;
                }
                case "string-array": {
                    JsonArray elements = inner.asArray();
                    String[] array = new String[elements.size()];
                    for (int i = 0; i < elements.size(); i++) {
                        array[i] = elements.get(i).isNull() ? null : elements.get(i).asString();
                    }
                    return array;
                }
                case "object-array-2dim": {
                    JsonArray rows = inner.asArray();
                    Object[][] array = new Object[rows.size()][];
                    for (int i = 0; i < rows.size(); i++) {
                        JsonArray elements = rows.get(i).asArray();
                        array[i] = new Object[elements.size()];
                        for (int j = 0; j < elements.size(); j++) {
                            array[i][j] = elements.get(j).isNull() ? null : elements.get(j).asInt();
                        }
                    }
                    return array;
                }
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
