import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonMethodRef;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the epl-from-clause-method-variable parity
 * scenario. Replays the five observable executions of
 * regression-lib/.../epl/fromclausemethod/EPLFromClauseMethodVariable.java in
 * registration order (the sixth execution, EPLFromClauseMethodVariableInvalid,
 * is compile-only and excluded under the invalidity policy; the two trailing
 * invalid compiles inside EPLFromClauseMethodContextVariable are likewise
 * excluded and are covered by Go-side Build-rejection tests).
 *
 * Cases. constant-variable joins SupportBean with the constant variable
 * MyConstantServiceVariable as the method source (fetchABean(intPrimitive)
 * returning a bean whose id is "_" + intPrimitive + "_"). The two
 * nonconstant cases share byte-identical steps and records: an on-set
 * statement mutates the MyNonConstantServiceVariable POJO postfix property and
 * the method source samples the variable's current value per invocation
 * (all three SupportBean sends use theString "E1" verbatim per the suite);
 * soda-true compiles both statements through eplToModel with a byte-exact
 * toEPL round-trip guard before compiling the model, soda-false compiles text
 * directly. context-variable declares MyContext (initiated by SupportBean_S0,
 * terminated by SupportBean_S1 which is registered but never sent), a
 * per-partition variable var seeded by MyNonConstantServiceVariableFactory.make()
 * (postfix "context_postfix"), the context-typed method join, and the
 * context MyContext on SupportBean_S2 set var.postfix mutation, observed
 * through listener records only. map-and-oa runs two triggerless
 * deploy/iterate/undeploy cycles over the Map and ObjectArray method sources
 * (instance metadata and static metadata respectively), observed under
 * iterator snapshot records only; no events are sent.
 *
 * Protocol notes. Each case owns a fresh Configuration mirroring the
 * TestSuiteEPLFromClauseMethodWConfig harness (method ref and imports of the
 * mirror service classes, the five preconfigured variables including the
 * unused null-valued MyNullMap, query-plan logging, and the SupportBean plus
 * SupportBean_S0/S1/S2 event types), a fresh runtime with a distinct URI,
 * internal timer off and initialize(0L), so every record carries epoch-0 time
 * ("1970-01-01T00:00:00Z"). Each case owns one sequence counter shared by
 * listener and snapshot records, starting at 1 in emission order. Regression
 * service classes are mirrored locally because regression-lib is not on the
 * oracle classpath; the EPL resolves the mirror class simple names through the
 * harness imports exactly as the suite resolves its own classes.
 */
public final class EPLFromClauseMethodVariableScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-from-clause-method-variable";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/fromclausemethod/EPLFromClauseMethodVariable.java";
    private static final String RUNTIME_URI_PREFIX = "parity-epl-from-clause-method-variable-";

    private static final String CASE_CONSTANT = "constant-variable";
    private static final String CASE_NONCONSTANT_TRUE = "nonconstant-soda-true";
    private static final String CASE_NONCONSTANT_FALSE = "nonconstant-soda-false";
    private static final String CASE_CONTEXT = "context-variable";
    private static final String CASE_MAP_OA = "map-and-oa";
    private static final String[] CASES = {CASE_CONSTANT, CASE_NONCONSTANT_TRUE, CASE_NONCONSTANT_FALSE, CASE_CONTEXT, CASE_MAP_OA};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-b6134319ddad78ee7888",
            "java-runtime-7fa38fe9688391d3f9da",
            "java-runtime-1c125ff69ef065d76608",
            "java-runtime-26c863d3cc9d5aca7cc7",
            "java-runtime-2989d0cd737e30ae5633"};
    private static final String[] EXECUTION_NAMES = {
            "EPLFromClauseMethodConstantVariable",
            "EPLFromClauseMethodNonConstantVariable{soda=true}",
            "EPLFromClauseMethodNonConstantVariable{soda=false}",
            "EPLFromClauseMethodContextVariable",
            "EPLFromClauseMethodVariableMapAndOA"};
    private static final String[] OBSERVATIONS = {"listener", "listener", "listener", "listener", "iterator"};
    private static final int[] RECORD_COUNTS = {2, 3, 3, 4, 2};
    private static final int TOTAL_RECORDS = 14;
    private static final int TOTAL_STEPS = 32;

    // Pinned per-case EPL (suite-exact text; the s0 select is the observed
    // statement of each case and equals the scenario's differential surface).
    private static final String EPL_CONSTANT_SELECT =
            "@name('s0') select id as c0 from SupportBean as sb, method:MyConstantServiceVariable.fetchABean(intPrimitive) as h0";
    private static final String EPL_NONCONSTANT_ONSET =
            "on SupportBean_S0 set MyNonConstantServiceVariable.postfix=p00";
    private static final String EPL_NONCONSTANT_SELECT =
            "@name('s0') select id as c0 from SupportBean as sb, method:MyNonConstantServiceVariable.fetchABean(intPrimitive) as h0";
    private static final String EPL_CONTEXT_DECLARE =
            "@public create context MyContext initiated by SupportBean_S0 as c_s0 terminated by SupportBean_S1(id=c_s0.id)";
    private static final String EPL_CONTEXT_VARIABLE =
            "@public context MyContext create variable MyNonConstantServiceVariable var = MyNonConstantServiceVariableFactory.make()";
    private static final String EPL_CONTEXT_SELECT =
            "@name('s0') context MyContext select id as c0 from SupportBean(intPrimitive=context.c_s0.id) as sb, " +
                    "method:var.fetchABean(intPrimitive) as h0";
    private static final String EPL_CONTEXT_ONSET =
            "context MyContext on SupportBean_S2(id = context.c_s0.id) set var.postfix=p20";
    private static final String EPL_MAP_SELECT =
            "@name('s0') select field1, field2 from method:MyMethodHandlerMap.getMapData()";
    private static final String EPL_OA_SELECT =
            "@name('s0') select field1, field2 from method:MyMethodHandlerOA.getOAData()";

    // Pinned send plans: {eventType, key1, key2} where SupportBean carries
    // (theString, intPrimitive) and SupportBean_S0/S2 carry (id, p00|p20)
    // with null meaning the payload omits the property.
    private static final String[][] CONSTANT_SENDS = {
            {"SupportBean", "E1", "10"},
            {"SupportBean", "E2", "20"},
    };
    private static final String[][] NONCONSTANT_SENDS = {
            {"SupportBean", "E1", "10"},
            {"SupportBean_S0", "1", "newpostfix"},
            {"SupportBean", "E1", "20"},
            {"SupportBean_S0", "2", "postfix"},
            {"SupportBean", "E1", "30"},
    };
    private static final String[][] CONTEXT_SENDS = {
            {"SupportBean_S0", "1", null},
            {"SupportBean_S0", "2", null},
            {"SupportBean", "E1", "1"},
            {"SupportBean", "E2", "2"},
            {"SupportBean_S2", "1", "a"},
            {"SupportBean_S2", "2", "b"},
            {"SupportBean", "E1", "1"},
            {"SupportBean", "E2", "2"},
    };
    private static final String[] MAP_OA_SNAPSHOT_LABELS = {"map", "oa"};

    private EPLFromClauseMethodVariableScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLFromClauseMethodVariableScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, steps, records);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " records, got " + records.size());
        }
        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT)
                .add("java", System.getProperty("java.version"))
                .add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("scenario id mismatch: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(scenario.getString("javaSource", JAVA_SOURCE))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain the five selected cases");
        }
        JsonArray caseDefinitions = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[i].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != TOTAL_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + TOTAL_STEPS + " steps");
        }
        JsonArray steps = stepValue.asArray();
        int stepIndex = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            stepIndex = validateCaseSteps(steps, stepIndex, caseIndex);
        }
        if (stepIndex != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static int validateCaseSteps(JsonArray steps, int stepIndex, int caseIndex) {
        String caseName = CASES[caseIndex];
        JsonObject marker = object(steps.get(stepIndex++), "case marker " + caseIndex);
        if (!"case".equals(marker.getString("op", "")) || !caseName.equals(marker.getString("case", ""))) {
            throw new IllegalArgumentException("cases must appear once in source order at step " + (stepIndex - 1));
        }
        JsonObject advance = object(steps.get(stepIndex++), "advance-time " + caseIndex);
        if (!"advance-time".equals(advance.getString("op", ""))
                || !"1970-01-01T00:00:00Z".equals(advance.getString("at", ""))) {
            throw new IllegalArgumentException("case " + caseName + " must advance time to the epoch");
        }
        if (CASE_MAP_OA.equals(caseName)) {
            for (String expectedLabel : MAP_OA_SNAPSHOT_LABELS) {
                JsonObject step = object(steps.get(stepIndex++), "snapshot step " + stepIndex);
                if (!"snapshot".equals(step.getString("op", ""))
                        || !"s0".equals(step.getString("statement", ""))
                        || !"fifo".equals(step.getString("mode", ""))
                        || !expectedLabel.equals(step.getString("label", ""))) {
                    throw new IllegalArgumentException("step " + (stepIndex - 1)
                            + " must be a fifo snapshot of s0 labeled " + expectedLabel);
                }
            }
            return stepIndex;
        }
        String[][] sends;
        if (CASE_CONSTANT.equals(caseName)) {
            sends = CONSTANT_SENDS;
        } else if (CASE_NONCONSTANT_TRUE.equals(caseName) || CASE_NONCONSTANT_FALSE.equals(caseName)) {
            sends = NONCONSTANT_SENDS;
        } else if (CASE_CONTEXT.equals(caseName)) {
            sends = CONTEXT_SENDS;
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }
        for (String[] expected : sends) {
            validateSend(object(steps.get(stepIndex++), "send step " + stepIndex), expected, stepIndex - 1);
        }
        return stepIndex;
    }

    private static void validateSend(JsonObject step, String[] expected, int stepIndex) {
        String eventType = expected[0];
        if (!"send".equals(step.getString("op", "")) || !eventType.equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send " + eventType);
        }
        JsonValue payloadValue = step.get("payload");
        if (payloadValue == null || !payloadValue.isObject()) {
            throw new IllegalArgumentException("step " + stepIndex + " payload is required");
        }
        JsonObject payload = payloadValue.asObject();
        if ("SupportBean".equals(eventType)) {
            if (payload.size() != 2
                    || !payload.names().contains("theString")
                    || !payload.names().contains("intPrimitive")) {
                throw new IllegalArgumentException("step " + stepIndex
                        + " SupportBean payload must contain exactly theString and intPrimitive");
            }
            if (!expected[1].equals(payload.getString("theString", null))) {
                throw new IllegalArgumentException("step " + stepIndex + " theString mismatch");
            }
            JsonValue intPrimitive = payload.get("intPrimitive");
            if (intPrimitive == null || !intPrimitive.isNumber()
                    || intPrimitive.asInt() != Integer.parseInt(expected[2])) {
                throw new IllegalArgumentException("step " + stepIndex + " intPrimitive mismatch");
            }
            return;
        }
        String property = "SupportBean_S2".equals(eventType) ? "p20" : "p00";
        boolean withProperty = expected[2] != null;
        int expectedSize = withProperty ? 2 : 1;
        if (payload.size() != expectedSize
                || !payload.names().contains("id")
                || (withProperty != payload.names().contains(property))) {
            throw new IllegalArgumentException("step " + stepIndex + " " + eventType
                    + " payload must contain id" + (withProperty ? " and " + property : " only"));
        }
        JsonValue id = payload.get("id");
        if (id == null || !id.isNumber() || id.asInt() != Integer.parseInt(expected[1])) {
            throw new IllegalArgumentException("step " + stepIndex + " id mismatch");
        }
        if (withProperty) {
            JsonValue value = payload.get(property);
            if (value == null || !value.isString() || !expected[2].equals(value.asString())) {
                throw new IllegalArgumentException("step " + stepIndex + " " + property + " mismatch");
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static void runCase(int caseIndex, JsonArray steps, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = newConfiguration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(RUNTIME_URI_PREFIX + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            int[] deployCount = new int[]{0};
            if (CASE_CONSTANT.equals(caseName)) {
                EPStatement s0 = findStatement(deploy(EPL_CONSTANT_SELECT, configuration, runtime, caseName, deployCount), caseName);
                s0.addListener(writer.listener(s0));
            } else if (CASE_NONCONSTANT_TRUE.equals(caseName) || CASE_NONCONSTANT_FALSE.equals(caseName)) {
                boolean soda = CASE_NONCONSTANT_TRUE.equals(caseName);
                deployMaybeSoda(EPL_NONCONSTANT_ONSET, soda, configuration, runtime, caseName, deployCount);
                EPStatement s0 = findStatement(
                        deployMaybeSoda(EPL_NONCONSTANT_SELECT, soda, configuration, runtime, caseName, deployCount), caseName);
                s0.addListener(writer.listener(s0));
            } else if (CASE_CONTEXT.equals(caseName)) {
                deploy(EPL_CONTEXT_DECLARE, configuration, runtime, caseName, deployCount);
                deploy(EPL_CONTEXT_VARIABLE, configuration, runtime, caseName, deployCount);
                EPStatement s0 = findStatement(deploy(EPL_CONTEXT_SELECT, configuration, runtime, caseName, deployCount), caseName);
                s0.addListener(writer.listener(s0));
                deploy(EPL_CONTEXT_ONSET, configuration, runtime, caseName, deployCount);
            } else if (!CASE_MAP_OA.equals(caseName)) {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
            replaySteps(steps, caseName, configuration, runtime, writer, deployCount);
            if (writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.sequence
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Fresh configuration mirroring the TestSuiteEPLFromClauseMethodWConfig
     * harness: the mirror service classes stand in for the regression-lib
     * classes and resolve in EPL through the harness imports.
     */
    private static Configuration newConfiguration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addMethodRef(MyStaticService.class, new ConfigurationCommonMethodRef());
        configuration.getCommon().addImport(MyStaticService.class);
        configuration.getCommon().addImport(MyNonConstantServiceVariableFactory.class);
        configuration.getCommon().addImport(MyNonConstantServiceVariable.class);
        configuration.getCommon().addVariable("MyConstantServiceVariable", MyConstantServiceVariable.class, new MyConstantServiceVariable());
        configuration.getCommon().addVariable("MyNonConstantServiceVariable", MyNonConstantServiceVariable.class, new MyNonConstantServiceVariable("postfix"));
        configuration.getCommon().addVariable("MyNullMap", MyMethodHandlerMap.class, null);
        configuration.getCommon().addVariable("MyMethodHandlerMap", MyMethodHandlerMap.class, new MyMethodHandlerMap("a", "b"));
        configuration.getCommon().addVariable("MyMethodHandlerOA", MyMethodHandlerOA.class, new MyMethodHandlerOA("a", "b"));
        configuration.getCommon().getLogging().setEnableQueryPlan(true);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getCommon().addEventType(SupportBean_S2.class);
        return configuration;
    }

    private static EPDeployment deploy(String epl, Configuration configuration, EPRuntime runtime,
                                       String caseName, int[] deployCount) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return deployCompiled(compiled, runtime, caseName, deployCount);
    }

    /**
     * soda=true deployment path: EPL to model, copy, byte-exact toEPL
     * round-trip guard, then compile the model (precedent
     * ResultSetAggregateMaxMinGroupByOMViewCompileScenarioOracle).
     */
    private static EPDeployment deployMaybeSoda(String epl, boolean soda, Configuration configuration, EPRuntime runtime,
                                                String caseName, int[] deployCount) throws Exception {
        if (!soda) {
            return deploy(epl, configuration, runtime, caseName, deployCount);
        }
        EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
        model = SerializableObjectCopier.copyMayFail(model);
        if (!epl.equals(model.toEPL())) {
            throw new IllegalStateException("soda toEPL round-trip mismatch: expected [" + epl + "] got [" + model.toEPL() + "]");
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, compilerArgs);
        return deployCompiled(compiled, runtime, caseName, deployCount);
    }

    private static EPDeployment deployCompiled(EPCompiled compiled, EPRuntime runtime, String caseName, int[] deployCount)
            throws Exception {
        deployCount[0]++;
        DeploymentOptions options = new DeploymentOptions()
                .setDeploymentId(RUNTIME_URI_PREFIX + caseName + "-d" + deployCount[0]);
        return runtime.getDeploymentService().deploy(compiled, options);
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
    }

    private static void replaySteps(JsonArray steps, String caseName, Configuration configuration, EPRuntime runtime,
                                    TraceWriter writer, int[] deployCount) throws Exception {
        boolean active = false;
        int mapOaCycle = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = object(steps.get(i), "step " + i);
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if ("send".equals(op)) {
                send(runtime, step, caseName);
            } else if ("advance-time".equals(op)) {
                runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
            } else if ("snapshot".equals(op)) {
                // map-and-oa cycle: deploy, observe the iterator, undeploy.
                String epl = mapOaCycle == 0 ? EPL_MAP_SELECT : EPL_OA_SELECT;
                EPDeployment deployment;
                try {
                    deployment = deploy(epl, configuration, runtime, caseName, deployCount);
                } catch (Exception ex) {
                    throw new IllegalStateException("map-and-oa cycle " + mapOaCycle + " deploy failed", ex);
                }
                EPStatement s0 = findStatement(deployment, caseName);
                List<EventBean> rows = new ArrayList<>();
                for (Iterator<EventBean> it = s0.iterator(); it.hasNext(); ) {
                    rows.add(it.next());
                }
                writer.writeSnapshot(s0, rows);
                runtime.getDeploymentService().undeployAll();
                mapOaCycle++;
            } else {
                throw new IllegalArgumentException("unsupported op " + op + " in case " + caseName);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step, String caseName) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, eventType);
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            int id = payload.getInt("id", 0);
            JsonValue p00 = payload.get("p00");
            SupportBean_S0 event = p00 == null || p00.isNull() ? new SupportBean_S0(id) : new SupportBean_S0(id, p00.asString());
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        if ("SupportBean_S2".equals(eventType)) {
            int id = payload.getInt("id", 0);
            JsonValue p20 = payload.get("p20");
            SupportBean_S2 event = p20 == null || p20.isNull() ? new SupportBean_S2(id) : new SupportBean_S2(id, p20.asString());
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalArgumentException("unsupported event type " + eventType + " in case " + caseName);
    }

    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        private UpdateListener listener(EPStatement statement) {
            return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
                boolean hasNew = newEvents != null && newEvents.length > 0;
                boolean hasOld = oldEvents != null && oldEvents.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                JsonObject record = base("listener", statement.getName());
                if (hasNew) {
                    record.add("new", rows(newEvents));
                }
                if (hasOld) {
                    record.add("old", rows(oldEvents));
                }
                records.add(record);
            };
        }

        private void writeSnapshot(EPStatement statement, List<EventBean> snapshotRows) {
            JsonObject record = base("snapshot", statement.getName());
            record.add("new", rows(snapshotRows.toArray(new EventBean[0])));
            records.add(record);
        }

        private JsonObject base(String operation, String statement) {
            return new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement)
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }

    /** Local mirror of the pinned regression-lib SupportBean_A bean. */
    public static class SupportBeanA {
        private final String id;

        public SupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyConstantServiceVariable. */
    public static class MyConstantServiceVariable implements Serializable {
        public SupportBeanA fetchABean(int intPrimitive) {
            return new SupportBeanA("_" + intPrimitive + "_");
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyNonConstantServiceVariable. */
    public static class MyNonConstantServiceVariable implements Serializable {
        private String postfix;

        public MyNonConstantServiceVariable(String postfix) {
            this.postfix = postfix;
        }

        public void setPostfix(String postfix) {
            this.postfix = postfix;
        }

        public String getPostfix() {
            return postfix;
        }

        public SupportBeanA fetchABean(int intPrimitive) {
            return new SupportBeanA("_" + intPrimitive + "_" + postfix);
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyStaticService. */
    public static class MyStaticService {
        public static SupportBeanA fetchABean(int intPrimitive) {
            return new SupportBeanA("_" + intPrimitive + "_");
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyNonConstantServiceVariableFactory. */
    public static class MyNonConstantServiceVariableFactory {
        public static MyNonConstantServiceVariable make() {
            return new MyNonConstantServiceVariable("context_postfix");
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyMethodHandlerMap. */
    public static class MyMethodHandlerMap implements Serializable {
        private final String field1;
        private final String field2;

        public MyMethodHandlerMap(String field1, String field2) {
            this.field1 = field1;
            this.field2 = field2;
        }

        public Map<String, Object> getMapDataMetadata() {
            Map<String, Object> fields = new HashMap<String, Object>();
            fields.put("field1", String.class);
            fields.put("field2", String.class);
            return fields;
        }

        public Map<String, Object>[] getMapData() {
            Map[] maps = new Map[1];
            HashMap<String, Object> row = new HashMap<String, Object>();
            maps[0] = row;
            row.put("field1", field1);
            row.put("field2", field2);
            return maps;
        }
    }

    /** Local mirror of EPLFromClauseMethodVariable.MyMethodHandlerOA. */
    public static class MyMethodHandlerOA implements Serializable {
        private final String field1;
        private final String field2;

        public MyMethodHandlerOA(String field1, String field2) {
            this.field1 = field1;
            this.field2 = field2;
        }

        public static LinkedHashMap<String, Object> getOADataMetadata() {
            LinkedHashMap<String, Object> fields = new LinkedHashMap<String, Object>();
            fields.put("field1", String.class);
            fields.put("field2", String.class);
            return fields;
        }

        public Object[][] getOAData() {
            return new Object[][]{{field1, field2}};
        }
    }
}
