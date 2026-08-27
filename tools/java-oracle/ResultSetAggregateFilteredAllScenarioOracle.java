import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
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

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for ResultSetAggregateAllAggFunctions (ordinal 2). */
public final class ResultSetAggregateFilteredAllScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-filtered-all";
    private static final String[] CASES = {
            "filtered-length3-all", "filtered-sums-length2", "filtered-stateless",
            "filtered-exact-big", "filtered-distinct-epl", "filtered-distinct-soda"
    };

    private ResultSetAggregateFilteredAllScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetAggregateFilteredAllScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id");
        }
        JsonValue value = scenario.get("steps");
        if (value == null || !value.isArray() || value.asArray().size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = value.asArray();
        validateShape(steps);
        JsonArray records = new JsonArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateShape(JsonArray steps) {
        int nextCase = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                if (nextCase >= CASES.length || !CASES[nextCase].equals(step.getString("case", ""))) {
                    throw new IllegalArgumentException("cases must appear once in source order");
                }
                nextCase++;
            } else if ("send".equals(op)) {
                JsonValue payload = step.get("payload");
                if (payload == null || !payload.isObject()) {
                    throw new IllegalArgumentException("send payload is required");
                }
            } else {
                throw new IllegalArgumentException("unsupported operation: " + op);
            }
        }
        if (nextCase != CASES.length) {
            throw new IllegalArgumentException("scenario must contain all filtered aggregate cases");
        }
    }

    private static void runCase(JsonArray steps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> bean = new HashMap<>();
        bean.put("intBoxed", Integer.class);
        bean.put("boolPrimitive", Boolean.class);
        bean.put("floatPrimitive", Float.class);
        bean.put("doublePrimitive", Double.class);
        bean.put("longPrimitive", Long.class);
        bean.put("shortPrimitive", Short.class);
        configuration.getCommon().addEventType("SupportBean", bean);
        Map<String, Object> numeric = new HashMap<>();
        numeric.put("bigint", BigInteger.class);
        numeric.put("bigdec", BigDecimal.class);
        configuration.getCommon().addEventType("SupportBeanNumeric", numeric);

        String runtimeName = "parity-resultset-aggregate-filtered-all-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String epl = eplFor(caseName);
            EPCompiled compiled;
            if ("filtered-distinct-soda".equals(caseName)) {
                EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
                model = SerializableObjectCopier.copyMayFail(model);
                Module module = new Module();
                module.getItems().add(new ModuleItem(model));
                module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                compiled = EPCompilerProvider.getCompiler().compile(epl,
                        new CompilerArguments(runtime.getRuntimePath()));
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeName));
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
            replay(steps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplFor(String caseName) {
        switch (caseName) {
            case "filtered-length3-all":
                return "@name('s0') select avedev(intBoxed, boolPrimitive) as cavedev, " +
                        "avg(intBoxed, boolPrimitive) as cavg, fmax(intBoxed, boolPrimitive) as cmax, " +
                        "median(intBoxed, boolPrimitive) as cmedian, fmin(intBoxed, boolPrimitive) as cmin, " +
                        "stddev(intBoxed, boolPrimitive) as cstddev, sum(intBoxed, boolPrimitive) as csum, " +
                        "fmaxever(intBoxed, boolPrimitive) as cfmaxever, " +
                        "fminever(intBoxed, boolPrimitive) as cfminever from SupportBean#length(3)";
            case "filtered-sums-length2":
                return "@name('s0') select sum(floatPrimitive, boolPrimitive) as c1, " +
                        "sum(doublePrimitive, boolPrimitive) as c2, sum(longPrimitive, boolPrimitive) as c3, " +
                        "sum(shortPrimitive, boolPrimitive) as c4 from SupportBean#length(2)";
            case "filtered-stateless":
                return "@name('s0') select fmax(intBoxed, boolPrimitive) as c1, " +
                        "fmin(intBoxed, boolPrimitive) as c2 from SupportBean";
            case "filtered-exact-big":
                return "@name('s0') select avg(bigdec, bigint < 100) as c1, " +
                        "sum(bigdec, bigint < 100) as c2, sum(bigint, bigint < 100) as c3 " +
                        "from SupportBeanNumeric#length(2)";
            case "filtered-distinct-epl":
            case "filtered-distinct-soda":
                return "@name('s0') select avedev(distinct intBoxed,boolPrimitive) as cavedev, " +
                        "avg(distinct intBoxed,boolPrimitive) as cavg, fmax(distinct intBoxed,boolPrimitive) as cmax, " +
                        "median(distinct intBoxed,boolPrimitive) as cmedian, fmin(distinct intBoxed,boolPrimitive) as cmin, " +
                        "stddev(distinct intBoxed,boolPrimitive) as cstddev, sum(distinct intBoxed,boolPrimitive) as csum " +
                        "from SupportBean#length(3)";
            default:
                throw new IllegalArgumentException("unsupported case: " + caseName);
        }
    }

    private static void replay(JsonArray steps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
            } else if (active && "send".equals(op)) {
                send(runtime, step);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            JsonValue intBoxed = payload.get("intBoxed");
            event.put("intBoxed", intBoxed == null || intBoxed.isNull() ? null : intBoxed.asInt());
            JsonValue bool = payload.get("boolPrimitive");
            event.put("boolPrimitive", bool == null || bool.isNull() ? false : bool.asBoolean());
            event.put("floatPrimitive", (float) payload.getDouble("floatPrimitive", 0.0));
            event.put("doublePrimitive", payload.getDouble("doublePrimitive", 0.0));
            event.put("longPrimitive", payload.getLong("longPrimitive", 0L));
            event.put("shortPrimitive", (short) payload.getInt("shortPrimitive", 0));
            runtime.getEventService().sendEventMap(event, eventType);
        } else if ("SupportBeanNumeric".equals(eventType)) {
            String bigint = payload.getString("bigint", "");
            String bigdec = payload.getString("bigdec", "");
            if (bigint.isEmpty() || bigdec.isEmpty()) {
                throw new IllegalArgumentException("SupportBeanNumeric bigint and bigdec are required");
            }
            Map<String, Object> event = new HashMap<>();
            event.put("bigint", new BigInteger(bigint));
            event.put("bigdec", new BigDecimal(bigdec));
            runtime.getEventService().sendEventMap(event, eventType);
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
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
            JsonObject record = new JsonObject().add("case", caseName).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray result = new JsonArray();
            if (events == null) {
                return result;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                result.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return result;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
            }
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
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
}
