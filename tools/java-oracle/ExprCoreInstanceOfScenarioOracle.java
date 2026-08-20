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
import com.espertech.esper.regressionlib.support.bean.*;
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

/** Direct Esper 9.0.0 oracle for replayable ExprCoreInstanceOf executions. */
public final class ExprCoreInstanceOfScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-instanceof";
    private static final String[] CASES = {
            "instanceof-simple", "instanceof-string-om", "instanceof-string-compile",
            "instanceof-dynamic-types", "instanceof-dynamic-hierarchy"
    };
    private static final String[] EVENT_TYPES = {
            "SupportBean", "SupportBean", "SupportBean", "SupportBeanDynRoot", "SupportBeanDynRoot"
    };
    private static final int[] SEND_COUNTS = {2, 2, 2, 5, 6};

    private ExprCoreInstanceOfScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-instanceof.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!SCENARIO_ID.equals(scenarioID)) {
            throw new IllegalArgumentException("unsupported scenario id " + scenarioID);
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateScenario(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID)
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
            requireStringOrNull(payload, "theString", sendIndex == 0 ? "abc" : null);
            requireNumber(payload, "intPrimitive", 100);
            requireNull(payload, "intBoxed");
            if (sendIndex == 0) {
                requireDouble(payload, "floatBoxed", 100.0);
            } else {
                requireNull(payload, "floatBoxed");
            }
            return;
        }
        if (caseIndex == 1 || caseIndex == 2) {
            requireFieldCount(payload, 2, CASES[caseIndex]);
            requireStringOrNull(payload, "theString", sendIndex == 0 ? "abc" : null);
            requireNumber(payload, "intPrimitive", 100);
            return;
        }
        if (caseIndex == 3) {
            String[] types = {"string", "float32", "null", "int", "int64"};
            requireString(payload, "itemType", types[sendIndex]);
            if (sendIndex == 2) {
                requireFieldCount(payload, 1, CASES[caseIndex]);
                return;
            }
            requireFieldCount(payload, 2, CASES[caseIndex]);
            if (sendIndex == 0) {
                requireString(payload, "itemValue", "abc");
            } else if (sendIndex == 1) {
                requireNumber(payload, "itemValue", 100);
            } else if (sendIndex == 3) {
                requireNumber(payload, "itemValue", 10);
            } else {
                requireNumber(payload, "itemValue", 99);
            }
            return;
        }
        String[] types = {"dyn-root", "plus", "superg-impl", "base-ab-impl", "b-impl", "a-impl"};
        requireString(payload, "itemType", types[sendIndex]);
        switch (sendIndex) {
            case 0:
                requireFieldCount(payload, 2, CASES[caseIndex]);
                requireString(payload, "itemValue", "abc");
                break;
            case 1:
                requireFieldCount(payload, 1, CASES[caseIndex]);
                break;
            case 2:
                requireFieldCount(payload, 4, CASES[caseIndex]);
                requireString(payload, "valueG", "");
                requireString(payload, "valueA", "");
                requireString(payload, "valueBaseAB", "");
                break;
            case 3:
                requireFieldCount(payload, 2, CASES[caseIndex]);
                requireString(payload, "valueBaseAB", "");
                break;
            case 4:
                requireFieldCount(payload, 3, CASES[caseIndex]);
                requireString(payload, "valueB", "");
                requireString(payload, "valueBaseAB", "");
                break;
            case 5:
                requireFieldCount(payload, 3, CASES[caseIndex]);
                requireString(payload, "valueA", "");
                requireString(payload, "valueBaseAB", "");
                break;
            default:
                throw new IllegalArgumentException("unsupported hierarchy vector");
        }
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

    private static void requireStringOrNull(JsonObject payload, String name, String expected) {
        JsonValue value = payload.get(name);
        if (expected == null) {
            requireNull(payload, name);
        } else if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("payload string mismatch for " + name);
        }
    }

    private static void requireNull(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isNull()) {
            throw new IllegalArgumentException("payload null mismatch for " + name);
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBeanDynRoot", SupportBeanDynRoot.class);
        String runtimeName = "parity-expr-core-instanceof-" + caseName;
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
        if ("instanceof-simple".equals(caseName)) {
            return "select instanceof(theString,string) as c0, " +
                    "instanceof(intBoxed,int) as c1, " +
                    "instanceof(floatBoxed,java.lang.Float) as c2, " +
                    "instanceof(theString,java.lang.Float,char,byte) as c3, " +
                    "instanceof(intPrimitive,java.lang.Integer) as c4, " +
                    "instanceof(intPrimitive,long) as c5, " +
                    "instanceof(intPrimitive,long,long,java.lang.Number) as c6, " +
                    "instanceof(floatBoxed,long,float) as c7 from SupportBean";
        }
        if ("instanceof-string-om".equals(caseName) || "instanceof-string-compile".equals(caseName)) {
            return "select instanceof(theString,string) as t0, " +
                    "instanceof(theString,float,string,int) as t1 from SupportBean";
        }
        if ("instanceof-dynamic-types".equals(caseName)) {
            return "select instanceof(item?, string) as t0, " +
                    "instanceof(item?, int) as t1, " +
                    "instanceof(item?, java.lang.Float) as t2, " +
                    "instanceof(item?, java.lang.Float, char, byte) as t3, " +
                    "instanceof(item?, java.lang.Integer) as t4, " +
                    "instanceof(item?, long) as t5, " +
                    "instanceof(item?, long, long, java.lang.Number) as t6, " +
                    "instanceof(item?, long, float) as t7 from SupportBeanDynRoot";
        }
        if ("instanceof-dynamic-hierarchy".equals(caseName)) {
            return "select instanceof(item?, " + SupportMarkerInterface.class.getName() + ") as t0, " +
                    "instanceof(item?, " + ISupportA.class.getName() + ") as t1, " +
                    "instanceof(item?, " + ISupportBaseAB.class.getName() + ") as t2, " +
                    "instanceof(item?, " + ISupportBaseABImpl.class.getName() + ") as t3, " +
                    "instanceof(item?, " + ISupportA.class.getName() + ", " + ISupportB.class.getName() + ") as t4, " +
                    "instanceof(item?, " + ISupportBaseAB.class.getName() + ", " + ISupportB.class.getName() + ") as t5, " +
                    "instanceof(item?, " + ISupportAImplSuperG.class.getName() + ", " + ISupportB.class.getName() + ") as t6, " +
                    "instanceof(item?, " + ISupportAImplSuperGImplPlus.class.getName() + ", " + SupportBeanAtoFBase.class.getName() + ") as t7 " +
                    "from SupportBeanDynRoot";
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
            } else if ("SupportBeanDynRoot".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toDynamicRoot(payload), "SupportBeanDynRoot");
            } else {
                throw new IllegalArgumentException("unsupported event type");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean(nullableString(payload, "theString"), payload.getInt("intPrimitive", 0));
        bean.setIntBoxed(nullableInteger(payload, "intBoxed"));
        bean.setFloatBoxed(nullableFloat(payload, "floatBoxed"));
        return bean;
    }

    private static SupportBeanDynRoot toDynamicRoot(JsonObject payload) {
        String type = payload.getString("itemType", "");
        Object item;
        switch (type) {
            case "string":
                item = payload.getString("itemValue", "");
                break;
            case "float32":
                item = (float) payload.getDouble("itemValue", 0.0);
                break;
            case "null":
                item = null;
                break;
            case "int":
                item = payload.getInt("itemValue", 0);
                break;
            case "int64":
                item = payload.getLong("itemValue", 0L);
                break;
            case "dyn-root":
                item = new SupportBeanDynRoot(payload.getString("itemValue", ""));
                break;
            case "plus":
                item = new ISupportAImplSuperGImplPlus();
                break;
            case "superg-impl":
                item = new ISupportAImplSuperGImpl(payload.getString("valueG", ""),
                        payload.getString("valueA", ""), payload.getString("valueBaseAB", ""));
                break;
            case "base-ab-impl":
                item = new ISupportBaseABImpl(payload.getString("valueBaseAB", ""));
                break;
            case "b-impl":
                item = new ISupportBImpl(payload.getString("valueB", ""),
                        payload.getString("valueBaseAB", ""));
                break;
            case "a-impl":
                item = new ISupportAImpl(payload.getString("valueA", ""),
                        payload.getString("valueBaseAB", ""));
                break;
            default:
                throw new IllegalArgumentException("unsupported item type " + type);
        }
        return new SupportBeanDynRoot(item);
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static Integer nullableInteger(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asInt();
    }

    private static Float nullableFloat(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : (float) value.asDouble();
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
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Float || value instanceof Double) {
                return Json.value(((Number) value).doubleValue());
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
}
