import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.collection.Pair;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowTypes;
import com.espertech.esper.regressionlib.support.dataflow.SupportGenericOutputOpWPort;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

/**
 * Direct Esper 9.0.0 oracle for the EPLDataflowTypes differential chain.
 *
 * Replays the two suite executions against the real regression-lib operator
 * classes: EPLDataflowBeanType fans one SupportBean source instruction out to
 * MySupportBeanOutputOp and SupportGenericOutputOpWPort, and
 * EPLDataflowMapType fans one HashMap source instruction out to MyMapOutputOp
 * and DefaultSupportCaptureOp. Both executions drain a blocking run() and the
 * captures are read after COMPLETE, one record per sink delivery.
 */
public final class DataflowTypesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-types";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowTypes.java";
    private static final String BEAN_TYPE = "bean-type";
    private static final String MAP_TYPE = "map-type";
    private static final String[] CASES = {BEAN_TYPE, MAP_TYPE};
    private static final int[] ORDINALS = {0, 1};
    private static final String[] RUNTIME_IDS = {"java-runtime-a277455fea9aac5b640b", "java-runtime-8006afc99e90b53a008c"};
    private static final String[] EXECUTION_NAMES = {"EPLDataflowBeanType", "EPLDataflowMapType"};
    private static final int[] RECORD_COUNTS = {2, 2};
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String SCHEMA_DEPLOYMENT_ID = "flow-schema";
    private static final String FLOW_NAME = "MyDataFlowOne";
    private static final String BEAN_TYPE_EPL =
            "@name('flow') create dataflow MyDataFlowOne DefaultSupportSourceOp -> outstream<SupportBean> {} "
                    + "MySupportBeanOutputOp(outstream) {} SupportGenericOutputOpWPort(outstream) {}";
    private static final String MAP_SCHEMA_EPL = "@public create map schema MyMap (p0 String, p1 int)";
    private static final String MAP_TYPE_EPL =
            "@name('flow') create dataflow MyDataFlowOne DefaultSupportSourceOp -> outstream<MyMap> {} "
                    + "MyMapOutputOp(outstream) {} DefaultSupportCaptureOp(outstream) {}";

    private DataflowTypesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowTypesScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, records);
        }
        if (records.size() != 4) {
            throw new IllegalStateException("expected 4 capture records, got " + records.size());
        }
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT).add("java", System.getProperty("java.version")).add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenario.getString("id", ""));
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
            throw new IllegalArgumentException("scenario must contain the two selected cases");
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
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != CASES.length * 2) {
            throw new IllegalArgumentException("scenario must contain exactly " + (CASES.length * 2) + " steps");
        }
        JsonArray steps = stepValue.asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(caseIndex * 2), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            JsonObject advance = object(steps.get(caseIndex * 2 + 1), "advance-time " + caseIndex);
            if (!"advance-time".equals(advance.getString("op", ""))
                    || !"1970-01-01T00:00:00Z".equals(advance.getString("at", ""))) {
                throw new IllegalArgumentException("each case must pin the epoch advance-time step");
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

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        String runtimeURI = "parity-dataflow-types-" + RUNTIME_IDS[caseIndex];

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        // Session configuration per TestSuiteEPLDataflow.configure: the SupportBean
        // type plus the import groups that resolve the dataflow operator simple
        // names in EPL text.
        configuration.getCommon().addEventType(SupportBean.class.getSimpleName(), SupportBean.class);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(SupportGenericOutputOpWPort.class.getPackage().getName() + ".*");
        configuration.getCommon().addImport(EPLDataflowTypes.MySupportBeanOutputOp.class);
        configuration.getCommon().addImport(EPLDataflowTypes.MyMapOutputOp.class);
        configuration.getCommon().addImport(SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            long[] sequence = new long[]{0};
            long expectedRecords = RECORD_COUNTS[caseIndex];
            if (BEAN_TYPE.equals(caseName)) {
                runBeanType(configuration, runtime, records, sequence);
            } else if (MAP_TYPE.equals(caseName)) {
                runMapType(configuration, runtime, records, sequence);
            } else {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
            if (sequence[0] != expectedRecords) {
                throw new IllegalStateException("case " + caseName + " produced " + sequence[0]
                        + " records, expected " + expectedRecords);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void runBeanType(Configuration configuration, EPRuntime runtime, JsonArray records,
                                    long[] sequence) throws Exception {
        EPCompiled compiled = compile(configuration, runtime, BEAN_TYPE_EPL);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        DefaultSupportSourceOp source = new DefaultSupportSourceOp(new Object[]{new SupportBean("E1", 1)});
        EPLDataflowTypes.MySupportBeanOutputOp outputOne = new EPLDataflowTypes.MySupportBeanOutputOp();
        SupportGenericOutputOpWPort<SupportBean> outputTwo = new SupportGenericOutputOpWPort<>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source, outputOne, outputTwo));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        instance.run();

        List<SupportBean> beans = outputOne.getAndReset();
        if (beans.size() != 1) {
            throw new IllegalStateException("MySupportBeanOutputOp received " + beans.size() + " beans, expected 1");
        }
        SupportBean bean = beans.get(0);
        assertBean(bean, "MySupportBeanOutputOp");

        Pair<List<SupportBean>, List<Integer>> received = outputTwo.getAndReset();
        if (received.getFirst().size() != 1) {
            throw new IllegalStateException("SupportGenericOutputOpWPort received "
                    + received.getFirst().size() + " beans, expected 1");
        }
        assertBean(received.getFirst().get(0), "SupportGenericOutputOpWPort");
        if (!received.getSecond().equals(Collections.singletonList(0))) {
            throw new IllegalStateException("SupportGenericOutputOpWPort delivered on ports "
                    + received.getSecond() + ", expected [0]");
        }

        captureRecord(records, sequence, BEAN_TYPE, "capture", "flow:MySupportBeanOutputOp", beanRow(bean));
        captureRecord(records, sequence, BEAN_TYPE, "capture-port-0", "flow:SupportGenericOutputOpWPort",
                beanRow(received.getFirst().get(0)));
    }

    private static void runMapType(Configuration configuration, EPRuntime runtime, JsonArray records,
                                   long[] sequence) throws Exception {
        // The @public schema deploys first; the flow compiles against the
        // accumulated runtime path that now carries MyMap.
        EPCompiled schema = compile(configuration, runtime, MAP_SCHEMA_EPL);
        deploy(runtime, schema, SCHEMA_DEPLOYMENT_ID);
        EPCompiled compiled = compile(configuration, runtime, MAP_TYPE_EPL);
        deploy(runtime, compiled, FLOW_DEPLOYMENT_ID);

        Map<String, Object> sourceMap = new HashMap<String, Object>();
        sourceMap.put("p0", "E1");
        sourceMap.put("p1", 1);
        DefaultSupportSourceOp source = new DefaultSupportSourceOp(new Object[]{sourceMap});
        EPLDataflowTypes.MyMapOutputOp outputOne = new EPLDataflowTypes.MyMapOutputOp();
        DefaultSupportCaptureOp<Map<String, Object>> outputTwo = new DefaultSupportCaptureOp<Map<String, Object>>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source, outputOne, outputTwo));
        EPDataFlowInstance instance = runtime.getDataFlowService().instantiate(FLOW_DEPLOYMENT_ID, FLOW_NAME, options);
        instance.run();

        List<Map<String, Object>> maps = outputOne.getAndReset();
        if (maps.size() != 1) {
            throw new IllegalStateException("MyMapOutputOp received " + maps.size() + " maps, expected 1");
        }
        assertMap(maps.get(0), "MyMapOutputOp");

        List<List<Map<String, Object>>> batches = outputTwo.getAndReset();
        if (batches.size() != 1) {
            throw new IllegalStateException("DefaultSupportCaptureOp received " + batches.size()
                    + " signal-delimited batches, expected 1");
        }
        List<Map<String, Object>> batch = batches.get(0);
        if (batch.size() != 1) {
            throw new IllegalStateException("DefaultSupportCaptureOp batch 0 holds " + batch.size()
                    + " rows, expected 1");
        }
        assertMap(batch.get(0), "DefaultSupportCaptureOp");

        captureRecord(records, sequence, MAP_TYPE, "capture", "flow:MyMapOutputOp", mapRow(maps.get(0)));
        captureRecord(records, sequence, MAP_TYPE, "capture", "flow:DefaultSupportCaptureOp", mapRow(batch.get(0)));
    }

    private static void assertBean(SupportBean bean, String sink) {
        if (!"E1".equals(bean.getTheString()) || bean.getIntPrimitive() != 1) {
            throw new IllegalStateException(sink + " received SupportBean{theString=" + bean.getTheString()
                    + ", intPrimitive=" + bean.getIntPrimitive() + "}, expected {E1, 1}");
        }
    }

    private static void assertMap(Map<String, Object> map, String sink) {
        if (!"E1".equals(map.get("p0")) || !Integer.valueOf(1).equals(map.get("p1"))) {
            throw new IllegalStateException(sink + " received map " + map + ", expected {p0=E1, p1=1}");
        }
    }

    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled, String deploymentId) throws Exception {
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(deploymentId));
    }

    private static void captureRecord(JsonArray records, long[] sequence, String caseName, String operation,
                                      String statement, JsonObject fields) {
        JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", operation)
                .add("statement", statement)
                .add("sequence", ++sequence[0])
                .add("time", Instant.ofEpochMilli(0L).toString());
        record.add("new", new JsonArray().add(new JsonObject().add("kind", "row").add("fields", fields)));
        records.add(record);
    }

    private static JsonObject beanRow(SupportBean bean) {
        // Property names in sorted order: intPrimitive before theString.
        return new JsonObject()
                .add("intPrimitive", bean.getIntPrimitive())
                .add("theString", bean.getTheString());
    }

    private static JsonObject mapRow(Map<String, Object> map) {
        JsonObject fields = new JsonObject();
        for (Map.Entry<String, Object> entry : new TreeMap<String, Object>(map).entrySet()) {
            fields.add(entry.getKey(), normalize(entry.getValue()));
        }
        return fields;
    }

    private static JsonValue normalize(Object value) {
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
