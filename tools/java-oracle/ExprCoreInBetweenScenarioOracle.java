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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.util.Arrays;

/** Direct Esper 9.0.0 oracle for the scalar ExprCoreInBetween executions. */
public final class ExprCoreInBetweenScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String[] CASES = {
            "in-numeric", "in-string", "between-string", "between-numeric", "in-range"
    };
    private static final int[] SEND_COUNTS = {22, 36, 48, 48, 10};
    // These vectors are part of the frozen Java/Go contract. Keep the JSON
    // value spelling so scenario mutations cannot silently change numeric
    // precision or turn a missing field into an unrelated default value.
    private static final String[] PAYLOAD_FIELDS = {
            "doubleBoxed", "theString", "theString", "doubleBoxed"
    };
    private static final String[][] PAYLOAD_VALUES = {
            {
                    "1", "null", "1.1", "1", "1.0999999999", "2", "4", "2",
                    "2.3333333333333335", "null", "5", "5", "0", "null", "-1", "1",
                    "null", "1.1", "1", "1.0999999999", "2", "4"
            },
            {
                    "\"0\"", "\"a\"", "\"b\"", "\"c\"", "\"d\"", "null", "\"0\"", "\"a\"", "\"b\"",
                    "\"c\"", "\"d\"", "null", "\"0\"", "\"b\"", "\"a\"", "\"c\"", "\"d\"", "null",
                    "\"0\"", "\"b\"", "\"a\"", "\"c\"", "\"d\"", "null", "\"0\"", "null", "\"b\"",
                    "\"0\"", "\"a\"", "\"b\"", "\"c\"", "\"d\"", "null", "\"0\"", "null", "\"b\""
            },
            {
                    "\"0\"", "\"a1\"", "\"a10\"", "\"c\"", "\"d\"", "null", "\"a0\"", "\"b9\"",
                    "\"b90\"", "\"0\"", "\"a1\"", "\"a10\"", "\"c\"", "\"d\"", "null", "\"a0\"",
                    "\"b9\"", "\"b90\"", "\"0\"", "null", "\"a0\"", "\"b9\"", "\"0\"", "null",
                    "\"a0\"", "\"b9\"", "\"0\"", "null", "\"a0\"", "\"b9\"", "\"0\"", "\"a1\"",
                    "\"a10\"", "\"c\"", "\"d\"", "null", "\"a0\"", "\"b9\"", "\"b90\"", "\"0\"",
                    "\"a1\"", "\"a10\"", "\"c\"", "\"d\"", "null", "\"a0\"", "\"b9\"", "\"b90\""
            },
            {
                    "1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
                    "1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
                    "1", "null", "1.1", "1", "null", "1.1", "1", "null", "1.1", "1", "null", "1.1",
                    "2", "1.0999999999", "2", "4", "15", "15.00001",
                    "1", "null", "1.1", "2", "1.0999999999", "2", "4", "15", "15.00001",
                    "1", "null", "1.1"
            }
    };
    private static final String[] RANGE_STRING_VALUES = {
            "\"E1\"", "\"E1\"", "\"E1\"", "\"E1\"", "\"E1\"", "\"E1\"",
            "\"a\"", "\"b\"", "\"c\"", "\"d\""
    };
    private static final int[] RANGE_INT_VALUES = {1, 2, 3, 4, 5, 3, 5, 5, 5, 5};

    private ExprCoreInBetweenScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-in-between.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!"expr-core-in-between".equals(scenarioID)) {
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
            for (int sendIndex = 0; sendIndex < SEND_COUNTS[caseIndex]; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !"SupportBean".equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
                validatePayload(step.get("payload").asObject(), caseIndex, sendIndex);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        if (payload == null) {
            throw new IllegalArgumentException("scenario payload is required");
        }
        if (caseIndex == CASES.length - 1) {
            if (payload.names().size() != 2 || payload.get("theString") == null ||
                    payload.get("intPrimitive") == null ||
                    !RANGE_STRING_VALUES[sendIndex].equals(payload.get("theString").toString()) ||
                    !Integer.toString(RANGE_INT_VALUES[sendIndex]).equals(payload.get("intPrimitive").toString())) {
                throw new IllegalArgumentException("scenario payload vector mismatch for " + CASES[caseIndex] + " send " + sendIndex);
            }
            return;
        }
        String field = PAYLOAD_FIELDS[caseIndex];
        if (payload.names().size() != 1 || payload.get(field) == null ||
                !PAYLOAD_VALUES[caseIndex][sendIndex].equals(payload.get(field).toString())) {
            throw new IllegalArgumentException("scenario payload vector mismatch for " + CASES[caseIndex] + " send " + sendIndex);
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        if ("in-range".equals(caseName)) {
            runRangeCase(allSteps, records);
            return;
        }
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        String runtimeName = "parity-expr-core-in-between-" + caseName;
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

    private static void runRangeCase(JsonArray allSteps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-expr-core-in-between-in-range", configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);

            EPStatement s0 = deployRangeStatement(runtime,
                    "@name('s0') select intPrimitive in (2:4) as ro, intPrimitive in [2:4] as rc, " +
                            "intPrimitive in [2:4) as rho, intPrimitive in (2:4] as rhc, " +
                            "intPrimitive not in (2:4) as nro, intPrimitive not in [2:4] as nrc, " +
                            "intPrimitive not in [2:4) as nrho, intPrimitive not in (2:4] as nrhc " +
                            "from SupportBean",
                    "s0", records, runtime);
            replayRange(allSteps, runtime, 0, 5);
            runtime.getDeploymentService().undeployAll();

            deployRangeStatement(runtime,
                    "@name('s1') select intPrimitive between 4 and 2 as r1, intPrimitive in [4:2] as r2 from SupportBean",
                    "s1", records, runtime);
            replayRange(allSteps, runtime, 5, 6);
            runtime.getDeploymentService().undeployAll();

            deployRangeStatement(runtime,
                    "@name('s2') select theString in ('a':'d') as ro from SupportBean",
                    "s2", records, runtime);
            replayRange(allSteps, runtime, 6, 10);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement deployRangeStatement(EPRuntime runtime, String epl, String name,
                                                     JsonArray records, EPRuntime traceRuntime) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                epl, new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                candidate.addListener(new TraceWriter(records, "in-range", candidate, traceRuntime));
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static void replayRange(JsonArray allSteps, EPRuntime runtime, int start, int end) {
        boolean active = false;
        int sendIndex = 0;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = "in-range".equals(step.getString("case", ""));
                continue;
            }
            if (!active || !"send".equals(op)) {
                continue;
            }
            if (sendIndex >= start && sendIndex < end) {
                runtime.getEventService().sendEventBean(toSupportBean(step.get("payload").asObject()), "SupportBean");
            }
            sendIndex++;
        }
    }

    private static String eplFor(String caseName) {
        if ("in-numeric".equals(caseName)) {
            return "select doubleBoxed in (1.1d, 7/3.5, 2*6/3, 0) as c0, " +
                    "doubleBoxed in (7/3d, null) as c1, " +
                    "doubleBoxed in (5,5,5,5,5,-1) as c2, " +
                    "doubleBoxed not in (1.1d, 7/3.5, 2*6/3, 0) as c3 from SupportBean";
        }
        if ("in-string".equals(caseName)) {
            return "select theString in ('a','b','c') as c0, " +
                    "theString in ('a') as c1, theString in ('a','b') as c2, " +
                    "theString in ('a',null) as c3, theString in (null) as c4, " +
                    "theString not in ('a','b','c') as c5, theString not in (null) as c6 from SupportBean";
        }
        if ("between-string".equals(caseName)) {
            return "select theString between 'a0' and 'b9' as c0, " +
                    "theString between 'b9' and 'a0' as c1, " +
                    "theString between null and 'b9' as c2, " +
                    "theString between null and null as c3, " +
                    "theString between 'a0' and null as c4, " +
                    "theString not between 'a0' and 'b9' as c5, " +
                    "theString not between 'b9' and 'a0' as c6 from SupportBean";
        }
        if ("between-numeric".equals(caseName)) {
            return "select doubleBoxed between 1.1 and 15 as c0, " +
                    "doubleBoxed between 15 and 1.1 as c1, " +
                    "doubleBoxed between null and 15 as c2, " +
                    "doubleBoxed between 15 and null as c3, " +
                    "doubleBoxed between null and null as c4, " +
                    "doubleBoxed not between 1.1 and 15 as c5, " +
                    "doubleBoxed not between 15 and 1.1 as c6, " +
                    "doubleBoxed not between 15 and null as c7 from SupportBean";
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
            if (active && "send".equals(op)) {
                runtime.getEventService().sendEventBean(toSupportBean(step.get("payload").asObject()), "SupportBean");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setTheString(nullableString(payload, "theString"));
        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
        bean.setDoubleBoxed(nullableDouble(payload, "doubleBoxed"));
        return bean;
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static Double nullableDouble(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asDouble();
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
