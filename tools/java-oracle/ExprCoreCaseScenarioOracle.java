import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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

/** Direct Esper 9.0.0 oracle for the replayable ExprCoreCase executions. */
public final class ExprCoreCaseScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-case";
    private static final String[] CASES = {
            "case-with-else", "case-branches3", "case-simple-numeric"
    };
    private static final String[] EVENT_TYPES = {
            "SupportMarketDataBean", "SupportMarketDataBean", "SupportBean"
    };
    private static final int[] SEND_COUNTS = {2, 3, 4};

    private ExprCoreCaseScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-case.json";
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
        if (caseIndex < 2) {
            requireFieldCount(payload, 3, CASES[caseIndex]);
            requireString(payload, "symbol", caseIndex == 0 ? (sendIndex == 0 ? "CSCO" : "DELL") :
                    (sendIndex == 0 ? "DELL" : sendIndex == 1 ? "MSFT" : "GE"));
            requireNumber(payload, "volume", caseIndex == 0 ? (sendIndex == 0 ? 4000 : 20) : 10000);
            requireDouble(payload, "price", 0.0);
            return;
        }
        requireFieldCount(payload, 4, "case-simple-numeric");
        int[] intValues = {2, 5, 12, 1};
        long[] longValues = {2, 1, 1, 2};
        double[] floatValues = {1, 1, 12, 3};
        double[] doubleValues = {1, 5, 4, 4};
        requireNumber(payload, "intPrimitive", intValues[sendIndex]);
        requireNumber(payload, "longPrimitive", longValues[sendIndex]);
        requireDouble(payload, "floatPrimitive", floatValues[sendIndex]);
        requireDouble(payload, "doublePrimitive", doubleValues[sendIndex]);
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
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
        String runtimeName = "parity-expr-core-case-" + caseName;
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
        if ("case-with-else".equals(caseName)) {
            return "select case when symbol='DELL' then 3 * volume else volume end as p1 " +
                    "from SupportMarketDataBean#length(3)";
        }
        if ("case-branches3".equals(caseName)) {
            return "select case when (symbol='GE') then volume " +
                    "when (symbol='DELL') then volume / 2.0 " +
                    "when (symbol='MSFT') then volume / 3.0 end as c0 " +
                    "from SupportMarketDataBean";
        }
        if ("case-simple-numeric".equals(caseName)) {
            return "select case intPrimitive when longPrimitive then (intPrimitive + longPrimitive) " +
                    "when doublePrimitive then intPrimitive * doublePrimitive " +
                    "when floatPrimitive then floatPrimitive / doublePrimitive " +
                    "else (intPrimitive + longPrimitive + floatPrimitive + doublePrimitive) end as c0 " +
                    "from SupportBean";
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
            String eventType = step.getString("eventType", "");
            JsonObject payload = step.get("payload").asObject();
            if ("SupportMarketDataBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(toMarketData(payload), eventType);
            } else if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(toSupportBean(payload), eventType);
            } else {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
        }
    }

    private static SupportMarketDataBean toMarketData(JsonObject payload) {
        return new SupportMarketDataBean(
                payload.getString("symbol", ""),
                payload.getDouble("price", 0.0),
                payload.getLong("volume", 0L),
                null);
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
        bean.setLongPrimitive(payload.getLong("longPrimitive", 0L));
        bean.setFloatPrimitive((float) payload.getDouble("floatPrimitive", 0.0));
        bean.setDoublePrimitive(payload.getDouble("doublePrimitive", 0.0));
        return bean;
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
