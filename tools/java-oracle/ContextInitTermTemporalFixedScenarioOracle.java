import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
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
 * Direct Esper 9.0.0 oracle for the context-init-term-temporal-fixed parity
 * scenario. Each case mirrors one execution of ContextInitTermTemporalFixed:
 * initiated-terminated contexts with filter and pattern boundaries (with
 * correlated termination and output snapshot when terminated), every-second
 * cron contexts, daily 9-to-5 cron contexts with joins, patterns with
 * timers, multi-statement deployments and grouped/ungrouped aggregation.
 * Mid-case deployments use a "deploy" step that attaches a new statement
 * listener.
 */
public final class ContextInitTermTemporalFixedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private ContextInitTermTemporalFixedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextInitTermTemporalFixedScenarioOracle <scenario.json>");
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

        String[] cases = {"correlated-output-snapshot", "filter-pattern-correlated", "filter-output-snapshot",
                "pattern-pattern", "every-second", "daily-join", "daily-pattern-time",
                "turned-off", "turned-on", "daily-agg-ungrouped", "daily-agg-grouped"};
        for (String caseName : cases) {
            if (!hasCase(steps, caseName)) {
                continue;
            }
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static String initialAdvanceTime(JsonArray steps, String wanted) {
        boolean active = false;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = wanted.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if ("advance-time".equals(op)) {
                return step.getString("at", "");
            }
            if (!"send".equals(op) && !"deploy".equals(op)) {
                return null;
            }
        }
        return null;
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("intBoxed", Integer.class);
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

        String dailyContext = "@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *);\n";
        String[] statements;
        String[] traced;
        if ("correlated-output-snapshot".equals(caseName)) {
            statements = new String[]{
                    "@public create context EveryNowAndThen as start SupportBean_S0 as s0 end SupportBean_S1(p10 = s0.p00) as s1;\n" +
                            "@name('s0') context EveryNowAndThen select context.s0.id as c1, context.s1.id as c2, sum(intPrimitive) as c3 " +
                            "from SupportBean#keepall output snapshot when terminated",
            };
            traced = new String[]{"s0"};
        } else if ("filter-pattern-correlated".equals(caseName)) {
            statements = new String[]{
                    "@public create context EveryNowAndThen as start SupportBean_S0 as s0 end pattern [SupportBean_S1(p10 = s0.p00)];\n" +
                            "@name('s0') context EveryNowAndThen select context.s0.p00 as c1, sum(intPrimitive) as c2 " +
                            "from SupportBean#keepall",
            };
            traced = new String[]{"s0"};
        } else if ("filter-output-snapshot".equals(caseName)) {
            statements = new String[]{
                    "@public create context EveryNowAndThen as start SupportBean_S0 as s0 end SupportBean_S1 as s1;\n" +
                            "@name('s0') context EveryNowAndThen select context.s0.p00 as c1, sum(intPrimitive) as c2 " +
                            "from SupportBean#keepall output snapshot when terminated",
            };
            traced = new String[]{"s0"};
        } else if ("pattern-pattern".equals(caseName)) {
            statements = new String[]{
                    "@public create context EveryNowAndThen as " +
                            "start pattern [s0=SupportBean_S0 -> timer:interval(1 sec)] " +
                            "end pattern [s1=SupportBean_S1 -> timer:interval(1 sec)];\n" +
                            "@name('s0') context EveryNowAndThen select context.s0.p00 as c1, sum(intPrimitive) as c2 " +
                            "from SupportBean#keepall",
            };
            traced = new String[]{"s0"};
        } else if ("every-second".equals(caseName)) {
            statements = new String[]{
                    "@public create context EverySecond as start (*, *, *, *, *, *) end (*, *, *, *, *, *);\n" +
                            "@name('s0') context EverySecond select * from SupportBean",
            };
            traced = new String[]{"s0"};
        } else if ("daily-join".equals(caseName)) {
            statements = new String[]{
                    dailyContext +
                            "@name('s0') context NineToFive " +
                            "select sb.theString as col1, sb.intPrimitive as col2, s0.id as col3, s0.p00 as col4 " +
                            "from SupportBean#keepall as sb full outer join SupportBean_S0#keepall as s0 on p00 = theString",
            };
            traced = new String[]{"s0"};
        } else if ("daily-pattern-time".equals(caseName)) {
            statements = new String[]{
                    dailyContext +
                            "@name('s0') context NineToFive select * from pattern[every timer:interval(10 sec)]",
            };
            traced = new String[]{"s0"};
        } else if ("turned-off".equals(caseName)) {
            statements = new String[]{
                    "@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)",
                    "@Name('A') context NineToFive select * from SupportBean",
                    "@Name('B') context NineToFive select * from SupportBean",
                    "@Name('C') context NineToFive select * from SupportBean",
            };
            traced = new String[]{"A", "B", "C"};
        } else if ("turned-on".equals(caseName)) {
            statements = new String[]{
                    "@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)",
                    "@Name('A') context NineToFive select * from SupportBean",
                    "@Name('B') context NineToFive select * from SupportBean",
            };
            traced = new String[]{"A", "B"};
        } else if ("daily-agg-ungrouped".equals(caseName)) {
            statements = new String[]{
                    dailyContext +
                            "@Name('S1') context NineToFive select theString as c1, sum(intPrimitive) as c2 from SupportBean",
            };
            traced = new String[]{"S1"};
        } else if ("daily-agg-grouped".equals(caseName)) {
            statements = new String[]{
                    "@public create context NestedContext as start (0, 8, *, *, *) end (0, 9, *, *, *);\n" +
                            "@Name('s0') context NestedContext select theString as c1, count(*) as c2 from SupportBean group by theString",
            };
            traced = new String[]{"s0"};
        } else {
            throw new IllegalArgumentException("unsupported case " + caseName);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-context-init-term-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            // The case's first advance-time step is the deployment-time clock:
            // cron contexts evaluate their schedule from that instant, matching
            // the regression harness which sends the start time before deploy.
            String initialTime = initialAdvanceTime(allSteps, caseName);
            if (initialTime != null) {
                runtime.getEventService().advanceTime(Instant.parse(initialTime).toEpochMilli());
            }
            // Deploy the initial statements (all but the last A/B/C entries, which
            // are deployed mid-case by "deploy" steps).
            String[] initial;
            String[] pending;
            if ("turned-off".equals(caseName) || "turned-on".equals(caseName)) {
                initial = new String[]{statements[0], statements[1]};
                pending = new String[statements.length - 2];
                System.arraycopy(statements, 2, pending, 0, pending.length);
            } else {
                initial = statements;
                pending = new String[0];
            }
            int pendingIndex = 0;
            Map<String, TraceWriter> writers = new HashMap<>();
            for (String epl : initial) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId("parity-context-init-term-" + caseName + "-" + deploymentCount()));
                for (EPStatement candidate : deployment.getStatements()) {
                    String name = candidate.getName();
                    boolean wanted = false;
                    for (String tracedName : traced) {
                        if (tracedName.equals(name)) {
                            wanted = true;
                            break;
                        }
                    }
                    if (!wanted) {
                        continue;
                    }
                    TraceWriter writer = new TraceWriter(records, caseName, candidate, runtime);
                    candidate.addListener(writer);
                    writers.put(name, writer);
                }
            }
            replayCase(allSteps, caseName, runtime, records, writers, pending, traced, initialTime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static int deploymentCount = 0;

    private static int deploymentCount() {
        return ++deploymentCount;
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, JsonArray records,
                                   Map<String, TraceWriter> writers, String[] pending, String[] traced) throws Exception {
        replayCase(allSteps, caseName, runtime, records, writers, pending, traced, null);
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime, JsonArray records,
                                   Map<String, TraceWriter> writers, String[] pending, String[] traced,
                                   String initialTime) throws Exception {
        boolean active = false;
        int pendingIndex = 0;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if ("send".equals(op)) {
                send(runtime, step);
            } else if ("advance-time".equals(op)) {
                if (initialTime != null && initialTime.equals(step.getString("at", ""))) {
                    continue;
                }
                runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
            } else if ("deploy".equals(op)) {
                if (pendingIndex >= pending.length) {
                    throw new IllegalStateException("no pending statement for deploy step " + step.getString("statement", ""));
                }
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(pending[pendingIndex],
                        new CompilerArguments(runtime.getRuntimePath()));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId("parity-context-init-term-" + caseName + "-mid-" + pendingIndex));
                pendingIndex++;
                for (EPStatement candidate : deployment.getStatements()) {
                    String name = candidate.getName();
                    boolean wanted = false;
                    for (String tracedName : traced) {
                        if (tracedName.equals(name)) {
                            wanted = true;
                            break;
                        }
                    }
                    if (!wanted) {
                        continue;
                    }
                    TraceWriter writer = new TraceWriter(records, caseName, candidate, runtime);
                    candidate.addListener(writer);
                    writers.put(name, writer);
                }
            } else {
                throw new IllegalArgumentException("unsupported op " + op);
            }
        }
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
            JsonValue intBoxed = payload.get("intBoxed");
            event.put("intBoxed", intBoxed.isNull() ? null : intBoxed.asInt());
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
