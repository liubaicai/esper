import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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

/** Direct Esper 9.0.0 oracle for the replayable LIKE/REGEXP executions. */
public final class ExprCoreLikeRegexpScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String[] CASES = {
            "like-constants",
            "like-expressions",
            "regexp-constants",
            "regexp-expressions"
    };
    private static final String[] EVENT_TYPES = {
            "SupportBean",
            "SupportBean_S0",
            "SupportBean",
            "SupportBean_S0"
    };
    private static final int[] SEND_COUNTS = {4, 5, 4, 6};

    private ExprCoreLikeRegexpScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-like-regexp.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!"expr-core-like-regexp".equals(scenarioID)) {
            throw new IllegalArgumentException("unsupported scenario id " + scenarioID);
        }
        JsonArray steps = scenario.get("steps").asArray();
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
        if (steps == null) {
            throw new IllegalArgumentException("scenario steps are required");
        }
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
            for (int send = 0; send < SEND_COUNTS[caseIndex]; send++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !EVENT_TYPES[caseIndex].equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        String runtimeName = "parity-expr-core-like-regexp-" + caseName;
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
        if ("like-constants".equals(caseName)) {
            return "select theString like 'A%' as c0, intPrimitive like '1%' as c1 from SupportBean";
        }
        if ("like-expressions".equals(caseName)) {
            return "select p00 like p01 as c0, id like p02 as c1 from SupportBean_S0";
        }
        if ("regexp-constants".equals(caseName)) {
            return "select theString regexp '.*Jack.*' as c0, intPrimitive regexp '.*1.*' as c1 from SupportBean";
        }
        if ("regexp-expressions".equals(caseName)) {
            return "select p00 regexp p01 as c0, id regexp p02 as c1 from SupportBean_S0";
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            if ("case".equals(step.getString("op", ""))) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            String eventType = step.getString("eventType", "");
            if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(toSupportBean(step.get("payload").asObject()), eventType);
            } else if ("SupportBean_S0".equals(eventType)) {
                runtime.getEventService().sendEventBean(toSupportBeanS0(step.get("payload").asObject()), eventType);
            } else {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setTheString(nullableString(payload, "theString"));
        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
        return bean;
    }

    private static SupportBean_S0 toSupportBeanS0(JsonObject payload) {
        SupportBean_S0 bean = new SupportBean_S0(payload.getInt("id", 0));
        bean.setP00(nullableString(payload, "p00"));
        bean.setP01(nullableString(payload, "p01"));
        bean.setP02(nullableString(payload, "p02"));
        bean.setP03(nullableString(payload, "p03"));
        return bean;
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
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
