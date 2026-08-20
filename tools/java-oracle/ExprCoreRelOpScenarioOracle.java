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
import java.math.BigDecimal;
import java.math.BigInteger;
import java.time.Instant;
import java.util.Arrays;

/** Direct Esper 9.0.0 oracle for ExprCoreRelOp's observable executions. */
public final class ExprCoreRelOpScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String[] CASES = {
            "relop-string",
            "relop-int",
            "relop-long",
            "relop-float",
            "relop-double",
            "relop-big-decimal",
            "relop-int-big-decimal",
            "relop-big-integer",
            "relop-int-big-integer",
            "relop-null"
    };

    private ExprCoreRelOpScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-relop.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (scenarioID.isBlank()) {
            throw new IllegalArgumentException("scenario id is required");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        for (String caseName : CASES) {
            if (!hasCase(steps, caseName)) {
                throw new IllegalArgumentException("scenario is missing case " + caseName);
            }
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (JsonValue value : steps) {
            JsonObject step = value.asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        String runtimeName = "parity-expr-core-relop-" + caseName;
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
        if ("relop-string".equals(caseName)) {
            return "select theString >= 'B' as c0, theString > 'B' as c1, " +
                    "theString <= 'B' as c2, theString < 'B' as c3 from SupportBean";
        }
        if ("relop-int".equals(caseName)) {
            return numericEpl("intPrimitive", "2");
        }
        if ("relop-long".equals(caseName)) {
            return numericEpl("longBoxed", "2L");
        }
        if ("relop-float".equals(caseName)) {
            return numericEpl("floatPrimitive", "2f");
        }
        if ("relop-double".equals(caseName)) {
            return numericEpl("doublePrimitive", "2d");
        }
        if ("relop-big-decimal".equals(caseName)) {
            return numericEpl("bigDecimal", "BigDecimal.valueOf(2, 0)");
        }
        if ("relop-int-big-decimal".equals(caseName)) {
            return numericEpl("intPrimitive", "BigDecimal.valueOf(2, 0)");
        }
        if ("relop-big-integer".equals(caseName)) {
            return numericEpl("bigInteger", "BigInteger.valueOf(2)");
        }
        if ("relop-int-big-integer".equals(caseName)) {
            return numericEpl("intPrimitive", "BigInteger.valueOf(2)");
        }
        if ("relop-null".equals(caseName)) {
            return "select intPrimitive > cast(null, int) as c0, intBoxed > 0 as c1, " +
                    "cast(null, int) > intPrimitive as c2, cast(null, int) > intBoxed as c3, " +
                    "cast(null, int) > cast(null, int) as c4, intPrimitive > intBoxed as c5, " +
                    "intBoxed > intPrimitive as c6, doubleBoxed > intBoxed as c7 from SupportBean";
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    private static String numericEpl(String lhs, String rhs) {
        return "select " + lhs + " >= " + rhs + " as c0, " + lhs + " > " + rhs + " as c1, " +
                lhs + " <= " + rhs + " as c2, " + lhs + " < " + rhs + " as c3 from SupportBean";
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
                if (!"SupportBean".equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("unsupported event type");
                }
                runtime.getEventService().sendEventBean(toSupportBean(step.get("payload").asObject()), "SupportBean");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
        SupportBean bean = new SupportBean();
        bean.setTheString(nullableString(payload, "theString"));
        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
        bean.setLongBoxed(nullableLong(payload, "longBoxed"));
        bean.setFloatPrimitive((float) payload.getDouble("floatPrimitive", 0));
        bean.setDoublePrimitive(payload.getDouble("doublePrimitive", 0));
        bean.setIntBoxed(nullableInteger(payload, "intBoxed"));
        bean.setDoubleBoxed(nullableDouble(payload, "doubleBoxed"));
        bean.setBigDecimal(nullableBigDecimal(payload, "bigDecimal"));
        bean.setBigInteger(nullableBigInteger(payload, "bigInteger"));
        return bean;
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static Integer nullableInteger(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asInt();
    }

    private static Long nullableLong(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asLong();
    }

    private static Double nullableDouble(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asDouble();
    }

    private static BigDecimal nullableBigDecimal(JsonObject payload, String name) {
        String value = nullableString(payload, name);
        return value == null ? null : new BigDecimal(value);
    }

    private static BigInteger nullableBigInteger(JsonObject payload, String name) {
        String value = nullableString(payload, name);
        return value == null ? null : new BigInteger(value);
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
