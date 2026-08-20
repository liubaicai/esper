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
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanDynRoot;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerInterface;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.util.Arrays;

/** Direct Esper 9.0.0 oracle for replayable ExprCoreExists executions. */
public final class ExprCoreExistsCastScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-exists-cast";
    private static final String[] CASES = {
            "exists-simple", "exists-inner", "exists-om", "exists-compile"
    };
    private static final String[] EVENT_TYPES = {
            "SupportBean", "SupportMarkerInterface", "SupportMarkerInterface", "SupportMarkerInterface"
    };
    private static final int[] SEND_COUNTS = {1, 5, 3, 3};

    private ExprCoreExistsCastScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-exists-cast.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id " + scenario.getString("id", ""));
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateScenario(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateScenario(JsonArray steps) {
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("scenario is missing case " + CASES[caseIndex]);
            }
            JsonObject marker = steps.get(offset++).asObject();
            if (!"case".equals(marker.getString("op", "")) ||
                    !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("scenario case order mismatch at " + caseIndex);
            }
            requireStepFields(marker, "case", "op", "case");
            for (int sendIndex = 0; sendIndex < SEND_COUNTS[caseIndex]; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !EVENT_TYPES[caseIndex].equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
                requireStepFields(step, "send", "op", "eventType", "payload");
                JsonValue payloadValue = step.get("payload");
                if (payloadValue == null || !payloadValue.isObject()) {
                    throw new IllegalArgumentException("scenario payload is required for " + CASES[caseIndex]);
                }
                validatePayload(payloadValue.asObject(), caseIndex, sendIndex);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void requireStepFields(JsonObject step, String kind, String... allowed) {
        if (step.names().size() != allowed.length) {
            throw new IllegalArgumentException("scenario " + kind + " has unsupported metadata");
        }
        for (String name : allowed) {
            if (!step.names().contains(name)) {
                throw new IllegalArgumentException("scenario " + kind + " is missing field " + name);
            }
        }
    }

    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        if (caseIndex == 0) {
            requireFieldCount(payload, 4, CASES[caseIndex]);
            requireString(payload, "theString", "abc");
            requireNumber(payload, "intPrimitive", 100);
            requireNumber(payload, "intBoxed", 3);
            requireDouble(payload, "floatBoxed", 9.5);
            return;
        }
        String[] shapes = caseIndex == 1
                ? new String[]{"null", "complex", "complex", "nested-support-bean", "support-bean-a"}
                : new String[]{"support-bean", "null", "string"};
        requireFieldCount(payload, 1, CASES[caseIndex]);
        requireString(payload, "shape", shapes[sendIndex]);
    }

    private static void requireFieldCount(JsonObject payload, int expected, String caseName) {
        if (payload.names().size() != expected) {
            throw new IllegalArgumentException("payload field count mismatch for " + caseName);
        }
    }

    private static void requireNumber(JsonObject payload, String name, long expected) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber() || value.asLong() != expected) {
            throw new IllegalArgumentException("payload number mismatch for " + name);
        }
    }

    private static void requireDouble(JsonObject payload, String name, double expected) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber() || Double.compare(value.asDouble(), expected) != 0) {
            throw new IllegalArgumentException("payload decimal mismatch for " + name);
        }
    }

    private static void requireString(JsonObject payload, String name, String expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("payload string mismatch for " + name);
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportMarkerInterface", SupportMarkerInterface.class);
        String runtimeName = "parity-expr-core-exists-cast-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeName, configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    "@name('s0') " + eplFor(caseName),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 was not deployed");
            }
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplFor(String caseName) {
        if ("exists-simple".equals(caseName)) {
            return "select exists(theString) as c0, exists(intBoxed?) as c1, " +
                    "exists(dummy?) as c2, exists(intPrimitive?) as c3, " +
                    "exists(intPrimitive) as c4 from SupportBean";
        }
        if ("exists-inner".equals(caseName)) {
            return "select exists(item?.id) as t0, " +
                    "exists(item?.id?) as t1, " +
                    "exists(item?.item.intBoxed) as t2, " +
                    "exists(item?.indexed[0]?) as t3, " +
                    "exists(item?.mapped('keyOne')?) as t4, " +
                    "exists(item?.nested?) as t5, " +
                    "exists(item?.nested.nestedValue?) as t6, " +
                    "exists(item?.nested.nestedNested?) as t7, " +
                    "exists(item?.nested.nestedNested.nestedNestedValue?) as t8, " +
                    "exists(item?.nested.nestedNested.nestedNestedValue.dummy?) as t9, " +
                    "exists(item?.nested.nestedNested.dummy?) as t10 " +
                    "from SupportMarkerInterface";
        }
        if ("exists-om".equals(caseName) || "exists-compile".equals(caseName)) {
            return "select exists(item?.intBoxed) as t0 from SupportMarkerInterface";
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active || !"send".equals(op)) {
                continue;
            }
            JsonObject payload = step.get("payload").asObject();
            if ("SupportBean".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toSupportBean(payload), "SupportBean");
            } else if ("SupportMarkerInterface".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toDynamicRoot(payload), "SupportMarkerInterface");
            } else {
                throw new IllegalArgumentException("unsupported event type");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean(payload.getString("theString", ""), payload.getInt("intPrimitive", 0));
        bean.setIntBoxed(payload.getInt("intBoxed", 0));
        bean.setFloatBoxed((float) payload.getDouble("floatBoxed", 0.0));
        return bean;
    }

    private static SupportBeanDynRoot toDynamicRoot(JsonObject payload) {
        String shape = payload.getString("shape", "");
        switch (shape) {
            case "null":
                return new SupportBeanDynRoot(null);
            case "complex":
                return new SupportBeanDynRoot(SupportBeanComplexProps.makeDefaultBean());
            case "nested-support-bean":
                return new SupportBeanDynRoot(new SupportBeanDynRoot(new SupportBean()));
            case "support-bean-a":
                return new SupportBeanDynRoot(new SupportBean_A("10"));
            case "support-bean":
                return new SupportBeanDynRoot(new SupportBean());
            case "string":
                return new SupportBeanDynRoot("abc");
            default:
                throw new IllegalArgumentException("unsupported shape " + shape);
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
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
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
