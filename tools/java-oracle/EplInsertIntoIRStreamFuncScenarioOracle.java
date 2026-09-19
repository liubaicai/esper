import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the EPL insert-into istream() parity scenario.
 *
 * Covers the single EPLInsertIntoIRStreamFunc execution (ord 0, variant
 * "direct"), one fresh runtime per case. lastevent-irstream deploys
 * '@name('i0') @public insert irstream into MyStream select irstream
 * theString as c0, istream() as c1 from SupportBean#lastevent' then
 * '@name('s0') select * from MyStream': 'insert irstream into' routes
 * the remove stream as inserts, so i0's listener sees proper IR pairs
 * (new {E2,true} old {E1,false}) while the MyStream consumer s0 sees
 * the routed remove rows flattened as inserts ({E1,false} delivered as
 * newData). join-irstream deploys '@name('s0') select irstream
 * theString as c0, id as c1, istream() as c2 from SupportBean#lastevent,
 * SupportBean_S0#lastevent': the listener sees new {E1,10,true} once
 * SupportBean_S0(10) arrives, then new {E2,10,true} old {E1,10,false}
 * when the SupportBean side replaces.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive and
 * SupportBean_S0 payloads carrying id (the scenario encodes id as a
 * JSON number; the bean constructor takes int).
 *
 * Observations are listener records only: {case, operation, statement,
 * sequence, time, new, old} emitted for the named i0/s0 statements
 * (lastevent-irstream) and s0 (join-irstream). The Java execution's
 * SODA leg 'select istream() from SupportBean' asserts only the
 * compile-time property type (Boolean) and emits no listener records,
 * so it is omitted from the scenario (approved difference); the
 * undeployAll between legs is modelled as separate cases with fresh
 * runtimes.
 */
public class EplInsertIntoIRStreamFuncScenarioOracle {

    /**
     * Asserted-field projections per listened statement, keyed by
     * case/statement because s0 projects {c0,c1} in lastevent-irstream
     * and {c0,c1,c2} in join-irstream.
     */
    private static final Map<String, String[]> PROJECTIONS = Map.of(
        "lastevent-irstream/i0", new String[]{"c0", "c1"},
        "lastevent-irstream/s0", new String[]{"c0", "c1"},
        "join-irstream/s0", new String[]{"c0", "c1", "c2"}
    );

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EplInsertIntoIRStreamFuncScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread; mirror that
        // wrapper so failures surface as exceptions rather than log lines.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EplInsertIntoIRStreamFuncScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        // Callback records buffer here and are flushed after each step;
        // listener delivery is synchronous on the sending thread.
        List<JsonObject> pending = new ArrayList<>();
        try {
            Map<String, Integer> listenerSeq = new HashMap<>();
            for (String epl : buildEPL(caseName)) {
                // Sequential deploys mirror env.compileDeploy(epl, path);
                // each statement sees the @public schemas and windows of
                // the previous deployments through the accumulated runtime
                // path.
                CompilerArguments compilerArgs = new CompilerArguments(config);
                compilerArgs.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement added : deployment.getStatements()) {
                    String projectionKey = caseName + "/" + added.getName();
                    if (PROJECTIONS.containsKey(projectionKey)) {
                        EPStatement stmt = added;
                        stmt.addListener((newData, oldData, statement, rt) -> {
                            if ((newData == null || newData.length == 0) && (oldData == null || oldData.length == 0)) {
                                return;
                            }
                            JsonObject record = new JsonObject();
                            record.add("case", caseName);
                            record.add("operation", "listener");
                            record.add("statement", stmt.getName());
                            record.add("sequence", listenerSeq.merge(stmt.getName(), 1, Integer::sum));
                            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                            record.add("new", renderRows(newData, PROJECTIONS.get(projectionKey)));
                            record.add("old", renderRows(oldData, PROJECTIONS.get(projectionKey)));
                            pending.add(record);
                        });
                    }
                }
            }

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                } else {
                    throw new IllegalStateException("unsupported op " + op);
                }
                flushPending(pending, records);
            }
            flushPending(pending, records);

        } finally {
            runtime.destroy();
            flushPending(pending, records);
        }
    }

    private static void flushPending(List<JsonObject> pending, List<JsonObject> records) {
        records.addAll(pending);
        pending.clear();
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "SupportBean");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean" -> {
                SupportBean event = new SupportBean();
                JsonValue theStringVal = payload.get("theString");
                if (theStringVal instanceof JsonString) {
                    event.setTheString(((JsonString) theStringVal).asString());
                }
                JsonValue intPrimitiveVal = payload.get("intPrimitive");
                if (intPrimitiveVal instanceof JsonNumber) {
                    event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                }
                runtime.getEventService().sendEventBean(event, "SupportBean");
            }
            case "SupportBean_S0" -> {
                // The scenario encodes id as a JSON number (the Go
                // decoder's istreamFuncS0.ID is an int); the bean
                // constructor takes int. Accept either JSON shape.
                SupportBean_S0 event = new SupportBean_S0(parseInt(payload.get("id")));
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    private static int parseInt(JsonValue value) {
        if (value instanceof JsonNumber) {
            return ((JsonNumber) value).asInt();
        }
        if (value instanceof JsonString) {
            return Integer.parseInt(((JsonString) value).asString());
        }
        return 0;
    }

    private static JsonArray renderRows(EventBean[] events, String[] projection) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(renderRow(event, projection));
        }
        return rows;
    }

    private static JsonObject renderRow(EventBean event, String[] projection) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(projection))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "lastevent-irstream" -> new String[]{
                "@name('i0') @public insert irstream into MyStream " +
                    "select irstream theString as c0, istream() as c1 " +
                    "from SupportBean#lastevent",
                "@name('s0') select * from MyStream"
            };
            case "join-irstream" -> new String[]{
                "@name('s0') select irstream theString as c0, id as c1, istream() as c2 " +
                    "from SupportBean#lastevent, SupportBean_S0#lastevent"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBean) {
            // Render the asserted-field projection {theString,intPrimitive}
            // so the trace carries no unasserted engine defaults.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread wrapped as
     * "Unexpected exception in statement '&lt;name&gt;': &lt;cause&gt;".
     */
    public static class RethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return new ExceptionHandler() {
                @Override
                public void handle(ExceptionHandlerContext context) {
                    throw new RuntimeException("Unexpected exception in statement '" + context.getStatementName() +
                        "': " + context.getThrowable().getMessage(), context.getThrowable());
                }
            };
        }
    }
}
