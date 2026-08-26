import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportEnum;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.common.internal.util.DeploymentIdNamePair;

import java.io.Serializable;
import java.lang.reflect.Array;
import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.TreeMap;

/**
 * Java oracle for EPLVariablesUse variable-in-filter scenarios.
 *
 * Covers VariableInFilter, VariableInFilterBoolean, SimpleSameModule, and the
 * runtime-API executions EPRuntime (variable introspection, runtime set,
 * bulk-set rollback, create-on-the-fly) and ConstantVariable (constant
 * variables in filters, write protection, ESPER-653 Date constant) by
 * replaying deterministic event sequences and recording observable output.
 */
public class EPLVariablesUseScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLVariablesUseScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
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
        // Register variables needed by each case
        switch (caseName) {
            case "variable-in-filter":
                config.getCommon().addVariable("var1IF", String.class, null);
                break;
            case "variable-in-filter-boolean":
                config.getCommon().addVariable("var1IFB", String.class, null);
                config.getCommon().addVariable("var2IFB", String.class, null);
                break;
            case "simple-same-module":
                config.getCommon().addVariable("var_simple_module_const", Boolean.class, true);
                break;
            case "simple-preconfigured":
                config.getCommon().addVariable("var_simple_preconfig_const", "boolean", true, true);
                break;
            case "simple-two-modules":
                // variable is declared by module A via EPL; no preconfigured registration
                break;
            case "invoke-method":
                config.getCommon().addImport(MySimpleVariableServiceFactory.class);
                config.getCommon().addImport(MySimpleVariableService.class);
                config.getCommon().addVariable("myInitService", MySimpleVariableService.class, MySimpleVariableServiceFactory.makeService());
                break;
            case "filter-constant-custom-type":
                config.getCommon().addEventType(MyVariableCustomEvent.class);
                config.getCommon().addImport(MyVariableCustomType.class);
                config.getCommon().addVariable("my_variable_custom_typed", MyVariableCustomType.class.getName(), MyVariableCustomType.of("abc"), true);
                break;
            case "ep-runtime":
                config.getCommon().addVariable("var1", Integer.class, -1);
                config.getCommon().addVariable("var2", String.class, "abc");
                break;
            case "constant-variable":
                // Preconfigured constants from TestSuiteEPLVariable.configure;
                // SupportEnum import backs the enum-constant declarations.
                config.getCommon().addImport(SupportEnum.class);
                config.getCommon().addVariable("MYCONST_TWO", "string", null, true);
                config.getCommon().addVariable("MYCONST_THREE", "boolean", true, true);
                break;
        }
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLVariablesUseScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPLs(caseName);
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            for (String epl : epls) {
                CompilerArguments compilerArgs = new CompilerArguments(config);
                compilerArgs.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                deployedModules.add(compiled);
                for (EPStatement stmt : deployment.getStatements()) {
                    deploymentIds.put(stmt.getName(), deployment.getDeploymentId());
                    statementsByName.put(stmt.getName(), stmt);
                }
            }

            // The select statement (named "s0") is optional: the runtime-API
            // cases carry no fixed listener statement.
            int[] seq = new int[] {0};
            EPStatement selectStmt = statementsByName.get("s0");
            if (selectStmt != null) {
                attachSelectListener(selectStmt, caseName, seq, records);
            }

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
                    case "send": {
                        String type = step.getString("eventType", "");
                        JsonObject payload = step.get("payload").asObject();
                        switch (type) {
                            case "SupportBean": {
                                SupportBean bean = new SupportBean();
                                JsonValue theStringValue = payload.get("theString");
                                bean.setTheString(theStringValue == null || theStringValue.isNull() ? null : theStringValue.asString());
                                JsonValue intPrimitive = payload.get("intPrimitive");
                                if (intPrimitive != null && !intPrimitive.isNull()) {
                                    bean.setIntPrimitive(intPrimitive.asInt());
                                }
                                JsonValue intBoxed = payload.get("intBoxed");
                                if (intBoxed != null && !intBoxed.isNull()) {
                                    bean.setIntBoxed(intBoxed.asInt());
                                }
                                JsonValue shortBoxed = payload.get("shortBoxed");
                                if (shortBoxed != null && !shortBoxed.isNull()) {
                                    bean.setShortBoxed((short) shortBoxed.asInt());
                                }
                                JsonValue enumValue = payload.get("enumValue");
                                if (enumValue != null && !enumValue.isNull()) {
                                    bean.setEnumValue(SupportEnum.valueOf(enumValue.asString()));
                                }
                                runtime.getEventService().sendEventBean(bean, type);
                                break;
                            }
                            case "SupportBean_S0": {
                                SupportBean_S0 s0 = new SupportBean_S0(
                                    payload.getInt("id", 0),
                                    payload.getString("p00", ""),
                                    payload.getString("p01", "")
                                );
                                runtime.getEventService().sendEventBean(s0, type);
                                break;
                            }
                            case "MyVariableCustomEvent": {
                                MyVariableCustomEvent customEvent = new MyVariableCustomEvent(
                                    MyVariableCustomType.of(payload.getString("name", "")));
                                runtime.getEventService().sendEventBean(customEvent, type);
                                break;
                            }
                            default:
                                throw new IllegalStateException("unknown type: " + type);
                        }
                        break;
                    }
                    case "read-variable":
                        readVariable(runtime, deploymentIds, caseName, step, records);
                        break;
                    case "set-variable":
                        setVariableStep(runtime, deploymentIds, caseName, step, records);
                        break;
                    case "deploy":
                        deployStep(runtime, config, caseName, step, records, deploymentIds, statementsByName, seq, deployedModules);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, config, caseName, step, records, deployedModules);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        statementsByName.clear();
                        break;
                    case "undeploy": {
                        // Targeted module removal, mirroring Java's
                        // undeployModuleContaining(statementName) between
                        // operator statements that reuse the same name.
                        String owner = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(owner);
                        if (deploymentId != null) {
                            runtime.getDeploymentService().undeploy(deploymentId);
                            deploymentIds.remove(owner);
                            statementsByName.remove(owner);
                        }
                        break;
                    }
                    default:
                        throw new IllegalStateException("unsupported variables-use step op " + op);
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPLs(String caseName) {
        return switch (caseName) {
            case "variable-in-filter" -> new String[]{
                "@name('set') on SupportBean_S0 set var1IF = p00",
                "@name('s0') select theString, intPrimitive from SupportBean(theString = var1IF)"
            };
            case "variable-in-filter-boolean" -> new String[]{
                "@name('set') on SupportBean_S0 set var1IFB = p00, var2IFB = p01",
                "@name('s0') select theString, intPrimitive from SupportBean(theString = var1IFB or theString = var2IFB)"
            };
            case "simple-same-module" -> new String[]{
                "@name('s0') select var_simple_module_const as c0 from SupportBean"
            };
            case "simple-preconfigured" -> new String[]{
                "@name('s0') select var_simple_preconfig_const as c0 from SupportBean"
            };
            case "simple-two-modules" -> new String[]{
                "@public create variable boolean var_simple_twomodule_const = true",
                "@name('s0') select var_simple_twomodule_const as c0 from SupportBean"
            };
            case "invoke-method" -> new String[]{
                "@public create constant variable MySimpleVariableService myService = MySimpleVariableServiceFactory.makeService()",
                "@name('s0') select myService.doSomething() as c0, myInitService.doSomething() as c1 from SupportBean"
            };
            case "filter-constant-custom-type" -> new String[]{
                "@name('s0') select * from MyVariableCustomEvent(name=my_variable_custom_typed)"
            };
            case "ep-runtime" -> new String[]{
                "@name('set') on SupportBean set var1 = intPrimitive, var2 = theString"
            };
            case "constant-variable" -> new String[]{
                // All statements arrive through scenario "deploy" steps so
                // both runtimes replay the identical mid-case timeline.
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    /** Attaches the shared row-recording listener; seq persists across redeploys. */
    private static void attachSelectListener(EPStatement statement, String caseName, int[] seq, List<JsonObject> records) {
        statement.addListener((newData, oldData, stmt, rt) -> {
            if (newData != null) {
                for (EventBean event : newData) {
                    seq[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "listener");
                    record.add("statement", stmt.getName());
                    record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                    record.add("sequence", seq[0]);
                    JsonArray newArr = new JsonArray();
                    JsonObject newItem = new JsonObject();
                    newItem.add("kind", "row");
                    JsonObject fields = new JsonObject();
                    for (String prop : event.getEventType().getPropertyNames()) {
                        Object value = event.get(prop);
                        if (isStructValue(value)) {
                            fields.add(prop, renderVariableValue(value));
                        } else {
                            fields.add(prop, value == null ? null : value.toString());
                        }
                    }
                    newItem.add("fields", fields);
                    newArr.add(newItem);
                    record.add("new", newArr);
                    records.add(record);
                }
            }
        });
    }

    /**
     * Emits {"case","operation":"variable","name","value"} with canonical value
     * rendering, mirroring EPLVariablesOnSetScenarioOracle.readVariable. Null
     * keeps the legacy {"state":"null"} object form; the nondeterministic
     * ESPER-653 Date constant renders the fixed "<date>" marker.
     */
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
        JsonObject variableRecord = new JsonObject();
        variableRecord.add("case", caseName);
        variableRecord.add("operation", "variable");
        variableRecord.add("name", name);
        if (value == null) {
            // Legacy top-level null form kept identical to prior scenarios
            variableRecord.add("value", new JsonObject().add("state", "null"));
        } else if (value instanceof java.util.Date) {
            variableRecord.add("value", "<date>");
        } else {
            variableRecord.add("value", renderVariableValue(value));
        }
        records.add(variableRecord);
    }

    /**
     * Executes one runtime variable-write step. Steps carrying expectError
     * attempt the write and emit {"operation":"set-variable-error"} with the
     * caught root-cause message ("&lt;no-error&gt;" if it unexpectedly succeeds);
     * plain writes stay silent. The bulk form is an ordered assignment array
     * replayed through the map-valued set API so validation failures roll back.
     */
    private static void setVariableStep(EPRuntime runtime, Map<String, String> deploymentIds, String caseName,
                                        JsonObject step, List<JsonObject> records) {
        String expected = step.getString("expectError", "");
        JsonValue payload = step.get("payload");
        boolean bulk = payload != null && payload.isArray();
        String caught = "<no-error>";
        try {
            if (bulk) {
                LinkedHashMap<DeploymentIdNamePair, Object> assignments = new LinkedHashMap<>();
                for (JsonValue entryVal : payload.asArray()) {
                    JsonObject entry = entryVal.asObject();
                    String entryOwner = entry.getString("statement", "");
                    String entryDeploymentId = entryOwner.isEmpty() ? null : deploymentIds.get(entryOwner);
                    assignments.put(new DeploymentIdNamePair(entryDeploymentId, entry.getString("name", "")),
                        decodeAssignedValue(entry.get("value")));
                }
                runtime.getVariableService().setVariableValue(assignments);
            } else {
                Object value = payload == null || payload.isNull() ? null : decodeAssignedValue(payload);
                String owner = step.getString("statement", "");
                String deploymentId = owner.isEmpty() ? null : deploymentIds.get(owner);
                runtime.getVariableService().setVariableValue(deploymentId, step.getString("name", ""), value);
            }
        } catch (RuntimeException ex) {
            // Canonical root-cause message: strips JVM wrapper layers so both
            // runtimes record the identical underlying failure text.
            caught = rootCauseMessage(ex);
        }
        if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("set-variable message drift for case "
                + caseName + ": expected [" + expected + "] got [" + caught + "]");
        }
        if (!expected.isEmpty()) {
            JsonObject errorRecord = new JsonObject();
            errorRecord.add("case", caseName);
            errorRecord.add("operation", "set-variable-error");
            errorRecord.add("value", caught);
            records.add(errorRecord);
        }
    }

    /** Compiles and deploys a labeled mid-case statement; failures are compile-error records. */
    private static void deployStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                   List<JsonObject> records, Map<String, String> deploymentIds,
                                   Map<String, EPStatement> statementsByName, int[] seq,
                                   List<EPCompiled> deployedModules) {
        String caught = "<no-error>";
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            // Recreation-style deploys compile fresh (like eplToModelCompileDeploy):
            // the accumulated path still carries the original declaring module,
            // which would trip the duplicate public-variable check even though
            // undeployment freed the runtime registration.
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
            deploymentIds.put(step.getString("statement", ""), deployment.getDeploymentId());
            for (EPStatement stmt : deployment.getStatements()) {
                statementsByName.put(stmt.getName(), stmt);
                if ("s0".equals(stmt.getName())) {
                    attachSelectListener(stmt, caseName, seq, records);
                }
            }
        } catch (Exception ex) {
            caught = rootCauseMessage(ex);
        }
        if (!caught.equals("<no-error>")) {
            JsonObject errorRecord = new JsonObject();
            errorRecord.add("case", caseName);
            errorRecord.add("operation", "compile-error");
            errorRecord.add("statement", step.getString("statement", ""));
            errorRecord.add("sequence", 0);
            errorRecord.add("value", caught);
            records.add(errorRecord);
        }
    }

    /** Builds an expected-invalid statement and records {"operation":"compile-error"}. */
    private static void buildErrorStep(EPRuntime runtime, Configuration config, String caseName,
                                       JsonObject step, List<JsonObject> records,
                                       List<EPCompiled> deployedModules) throws Exception {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            // Persistent module path: undeployed modules keep contributing
            // their public objects to later compiles, mirroring the Java
            // regression suite's accumulated RegressionPath semantics.
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            // Canonical root-cause message: the innermost validation sentence,
            // matching the Go engine's bare aligned text without the trailing
            // bracketed statement source.
            caught = rootCauseMessage(ex);
        }
        if (!expected.isEmpty() && !expected.equals(caught)) {
            throw new IllegalStateException("compile-error message drift for " + label
                + ": expected [" + expected + "] got [" + caught + "]");
        }
        JsonObject errorRecord = new JsonObject();
        errorRecord.add("case", caseName);
        errorRecord.add("operation", "compile-error");
        errorRecord.add("statement", label);
        errorRecord.add("sequence", 0);
        errorRecord.add("value", caught);
        records.add(errorRecord);
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
     * Decodes one assignment value. Tagged objects {"type","value"} pin the
     * boxed width so runtime type-mismatch messages name the same Java type on
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
                case "integer":
                    return inner.isNull() ? null : inner.asInt();
                case "long":
                    return inner.isNull() ? null : inner.asLong();
                case "short":
                    return inner.isNull() ? null : (short) inner.asInt();
                case "byte":
                    return inner.isNull() ? null : (byte) inner.asInt();
                case "float":
                    return inner.isNull() ? null : (float) inner.asDouble();
                case "double":
                    return inner.isNull() ? null : inner.asDouble();
                case "boolean":
                    return inner.isNull() ? null : inner.asBoolean();
                case "string":
                    return inner.isNull() ? null : inner.asString();
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

    /** Struct values (POJO variables with public fields) render canonically. */
    private static boolean isStructValue(Object value) {
        if (value == null || value.getClass().isArray()) {
            return false;
        }
        if (value instanceof Number || value instanceof String || value instanceof Boolean
                || value instanceof Character || value instanceof Map || value instanceof EventBean) {
            return false;
        }
        return value.getClass().getFields().length > 0;
    }

    /**
     * Canonical struct rendering: numbers long-truncated, strings/booleans passthrough,
     * arrays as recursively rendered element arrays, POJO structs as sorted public-field objects.
     */
    private static JsonValue renderVariableValue(Object value) {
        if (value == null) {
            return Json.NULL;
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof String) {
            return Json.value((String) value);
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value.getClass().isArray()) {
            return renderArrayValue(value);
        }
        TreeMap<String, JsonValue> members = new TreeMap<>();
        for (Field field : value.getClass().getFields()) {
            if (Modifier.isStatic(field.getModifiers())) {
                continue;
            }
            Object fieldValue;
            try {
                fieldValue = field.get(value);
            } catch (IllegalAccessException e) {
                throw new IllegalStateException("failed reading public field " + field.getName(), e);
            }
            members.put(field.getName(), renderVariableValue(fieldValue));
        }
        JsonObject object = new JsonObject();
        for (Map.Entry<String, JsonValue> entry : members.entrySet()) {
            object.add(entry.getKey(), entry.getValue());
        }
        return object;
    }

    /** Canonical array rendering: elements as Number→longValue, null→null, else String.valueOf. */
    private static JsonArray renderArrayValue(Object value) {
        JsonArray array = new JsonArray();
        for (int i = 0; i < Array.getLength(value); i++) {
            Object element = Array.get(value, i);
            if (element == null) {
                array.add(Json.NULL);
            } else if (element instanceof Number) {
                array.add(((Number) element).longValue());
            } else if (element instanceof Boolean) {
                array.add(((Boolean) element).booleanValue());
            } else {
                array.add(String.valueOf(element));
            }
        }
        return array;
    }

    /** Local mirror of EPLVariablesUse.MySimpleVariableServiceFactory (regression-lib is not on the oracle classpath). */
    public static class MySimpleVariableServiceFactory {
        public static MySimpleVariableService makeService() {
            return new MySimpleVariableService();
        }
    }

    /** Local mirror of EPLVariablesUse.MySimpleVariableService. */
    public static class MySimpleVariableService implements Serializable {
        private static final long serialVersionUID = 5053839663706543568L;

        public String doSomething() {
            return "hello";
        }
    }

    /** Local mirror of EPLVariablesUse.MyVariableCustomEvent. */
    public static class MyVariableCustomEvent implements Serializable {
        private static final long serialVersionUID = 8457503203367721176L;
        private final MyVariableCustomType name;

        MyVariableCustomEvent(MyVariableCustomType name) {
            this.name = name;
        }

        public MyVariableCustomType getName() {
            return name;
        }
    }

    /**
     * Local mirror of EPLVariablesUse.MyVariableCustomType; the name field is
     * public so canonical struct rendering exposes it deterministically.
     */
    public static class MyVariableCustomType implements Serializable {
        private static final long serialVersionUID = 7612595617141560464L;
        public final String name;

        MyVariableCustomType(String name) {
            this.name = name;
        }

        public static MyVariableCustomType of(String name) {
            return new MyVariableCustomType(name);
        }

        public String getName() {
            return name;
        }

        @Override
        public boolean equals(Object o) {
            if (this == o) return true;
            if (o == null || getClass() != o.getClass()) return false;
            MyVariableCustomType myType = (MyVariableCustomType) o;
            return Objects.equals(name, myType.name);
        }

        @Override
        public int hashCode() {
            return Objects.hash(name);
        }
    }
}
