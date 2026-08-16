import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the context-init-term-filter-operators
 * parity scenario. Mirrors ContextInitTermFilterAllOperators (25-operator
 * correlated filter matrix against context.sb.id), 
 * ContextInitTermFilterBooleanOperator (arithmetic correlated filter
 * intPrimitive + context.sb.id = 5) and
 * ContextInitTermPatternInitiatedStraightSelect (OR-pattern start with
 * unbound-tag projections).
 */
public final class ContextInitTermFilterOperatorsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermFilterOperatorsScenarioOracle() {
    }

    private static final String CTX_EPL =
        "@public create context EverySupportBean as " +
        "initiated by SupportBean_S0 as sb " +
        "terminated after 10 days 5 hours 2 minutes 1 sec 11 milliseconds";

    private static final String CTX_EPL_PATTERN =
        "@public create context EverySupportBean as " +
        "initiated by pattern [every (a=SupportBean_S0 or b=SupportBean_S1)] " +
        "terminated after 1 minutes";

    private static String operatorEpl(String operator) {
        switch (operator) {
            case "op-eq-intboxed-lhs": return "context.sb.id = intBoxed";
            case "op-eq-intboxed-rhs": return "intBoxed = context.sb.id";
            case "op-gt-intboxed": return "context.sb.id > intBoxed";
            case "op-ge-intboxed": return "context.sb.id >= intBoxed";
            case "op-lt-intboxed": return "context.sb.id < intBoxed";
            case "op-le-intboxed": return "context.sb.id <= intBoxed";
            case "op-lt-intboxed-rev": return "intBoxed < context.sb.id";
            case "op-le-intboxed-rev": return "intBoxed <= context.sb.id";
            case "op-gt-intboxed-rev": return "intBoxed > context.sb.id";
            case "op-ge-intboxed-rev": return "intBoxed >= context.sb.id";
            case "op-in-intboxed": return "intBoxed in (context.sb.id)";
            case "op-between-intboxed": return "intBoxed between context.sb.id and context.sb.id";
            case "op-ne-intboxed-lhs": return "context.sb.id != intBoxed";
            case "op-ne-intboxed-rhs": return "intBoxed != context.sb.id";
            case "op-notin-intboxed": return "intBoxed not in (context.sb.id)";
            case "op-notbetween-intboxed": return "intBoxed not between context.sb.id and context.sb.id";
            case "op-is-intboxed-lhs": return "context.sb.id is intBoxed";
            case "op-is-intboxed-rhs": return "intBoxed is context.sb.id";
            case "op-isnot-intboxed-lhs": return "context.sb.id is not intBoxed";
            case "op-isnot-intboxed-rhs": return "intBoxed is not context.sb.id";
            case "op-eq-short-lhs": return "context.sb.id = shortBoxed";
            case "op-eq-short-rhs": return "shortBoxed = context.sb.id";
            case "op-gt-short": return "context.sb.id > shortBoxed";
            case "op-lt-short-rev": return "shortBoxed < context.sb.id";
            case "op-in-short": return "shortBoxed in (context.sb.id)";
            default:
                throw new IllegalArgumentException("unsupported operator case " + operator);
        }
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermFilterOperatorsScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
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
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            String caseName = step.getString("case", "");
            if (caseName.isBlank()) {
                throw new IllegalArgumentException("case without name at step " + i);
            }
            runCase(steps, i, caseName, records);
        }
        System.out.println(trace);
    }

    private static void runCase(JsonArray allSteps, int caseIndex, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("intBoxed", Integer.class);
        beanType.put("shortBoxed", Short.class);
        beanType.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        configuration.getCommon().addEventType("SupportBean_S1", s1Type);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-init-term-fops-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String contextEpl;
            String statementEpl;
            if ("op-boolean-arithmetic".equals(caseName)) {
                contextEpl = CTX_EPL;
                statementEpl = "@name('s0') context EverySupportBean " +
                        "select theString as c0,intPrimitive as c1,context.sb.p00 as c2 " +
                        "from SupportBean(intPrimitive + context.sb.id = 5)";
            } else if ("op-pattern-straight-select".equals(caseName)) {
                contextEpl = CTX_EPL_PATTERN;
                statementEpl = "@name('s0') context EverySupportBean " +
                        "select context.a.id as c1, context.b.id as c2, theString as c3 from SupportBean";
            } else {
                contextEpl = CTX_EPL;
                statementEpl = "@name('s0') context EverySupportBean " +
                        "select theString as c0,intPrimitive as c1,context.sb.p00 as c2 " +
                        "from SupportBean(" + operatorEpl(caseName) + ")";
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    contextEpl + ";\n" + statementEpl, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 not found");
            }
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);

            boolean active = false;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    send(runtime, step);
                } else if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                } else {
                    throw new IllegalArgumentException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", ""));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            JsonValue intBoxed = payload.get("intBoxed");
            event.put("intBoxed", intBoxed.isNull() ? null : intBoxed.asInt());
            JsonValue shortBoxed = payload.get("shortBoxed");
            event.put("shortBoxed", shortBoxed.isNull() ? null : (short) shortBoxed.asInt());
            JsonValue longBoxed = payload.get("longBoxed");
            event.put("longBoxed", longBoxed.isNull() ? null : longBoxed.asLong());
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", nullableString(payload, "p00"));
            event.put("p01", nullableString(payload, "p01"));
        } else if ("SupportBean_S1".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p10", nullableString(payload, "p10"));
            event.put("p11", nullableString(payload, "p11"));
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
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
                java.util.Arrays.sort(names);
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
}
