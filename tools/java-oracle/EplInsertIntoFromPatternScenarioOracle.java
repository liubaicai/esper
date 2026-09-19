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
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
 * Java oracle for the EPL insert-into from-pattern parity scenario.
 *
 * Covers all four EPLInsertIntoFromPattern executions, one fresh runtime
 * per case. pattern-wildcard-props replays EPLInsertIntoPropsWildcard
 * (ord 0): an @public insert-into with an explicit column list (es0id,
 * es1id) selecting the tag properties es0.id/es1.id from
 * pattern [every (es0=SupportBean_S0 or es1=SupportBean_S1)] into
 * MyThirdStream, observed through the s0 select-* listener.
 * pattern-explicit-props replays EPLInsertIntoProps (ord 1): an @public
 * insert-into with an explicit column list (s0, s1) selecting the tagged
 * events themselves into MySecondStream, observed through the s0 listener
 * selecting s0.id as es0id, s1.id as es1id. pattern-no-props replays
 * EPLInsertIntoNoProps (ord 2): an @public insert-into without a column
 * list selecting the tagged events into MyStream (default tag-named
 * columns), observed through the s0 listener over MyStream#length(10).
 * pattern-named-window replays EPLInsertIntoFromPatternNamedWindow
 * (ord 3): a PositionW named window with
 * win:time(1 hour).std:unique(intPrimitive) retention fed from
 * SupportBean, and an s1 insert-into selecting * from
 * pattern[every a = PositionW -> every b = PositionW]; the listener
 * attaches to the insert-into statement itself.
 *
 * The first three cases send SupportBean_S1{id:10,p10:""} then
 * SupportBean_S0{id:20,p00:""}; the absent OR-branch tag projects null,
 * so the listener rows are {es0id:null,es1id:10} then
 * {es0id:20,es1id:null}. The named-window case sends
 * SupportBean{theString:E1,intPrimitive:1} then E2/intPrimitive:1 and
 * observes exactly one s1 listener row carrying the a/b tagged events;
 * the suite asserts only the delivery, and the trace projects each bean
 * to the asserted fields {theString,intPrimitive}.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive,
 * SupportBean_S0 payloads carrying id/p00 and SupportBean_S1 payloads
 * carrying id/p10 (fields absent from a payload keep their defaults).
 *
 * Observations are listener records only: {case, operation, statement,
 * sequence, time, new, old} emitted for the named s0/s1 statements.
 * Milestone calls in the Java source emit no records and are omitted
 * from the scenario.
 */
public class EplInsertIntoFromPatternScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EplInsertIntoFromPatternScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread; mirror that
        // wrapper so failures surface as exceptions rather than log lines.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EplInsertIntoFromPatternScenarioOracle-" + caseName, config);
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
                    if ("s0".equals(added.getName()) || "s1".equals(added.getName())) {
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
                            record.add("new", renderRows(newData));
                            record.add("old", renderRows(oldData));
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
                SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0), payload.getString("p00", null));
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
            }
            case "SupportBean_S1" -> {
                SupportBean_S1 event = new SupportBean_S1(payload.getInt("id", 0), payload.getString("p10", null));
                runtime.getEventService().sendEventBean(event, "SupportBean_S1");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    private static JsonArray renderRows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(renderRow(event));
        }
        return rows;
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "pattern-wildcard-props" -> new String[]{
                "@public insert into MyThirdStream(es0id, es1id) select es0.id, es1.id from " +
                    "pattern [every (es0=SupportBean_S0 or es1=SupportBean_S1)]",
                "@name('s0') select * from MyThirdStream"
            };
            case "pattern-explicit-props" -> new String[]{
                "@public insert into MySecondStream(s0, s1) select es0, es1 from " +
                    "pattern [every (es0=SupportBean_S0 or es1=SupportBean_S1)]",
                "@name('s0') select s0.id as es0id, s1.id as es1id from MySecondStream"
            };
            case "pattern-no-props" -> new String[]{
                "@public insert into MyStream select es0, es1 from " +
                    "pattern [every (es0=SupportBean_S0 or es1=SupportBean_S1)]",
                "@name('s0') select es0.id as es0id, es1.id as es1id from MyStream#length(10)"
            };
            case "pattern-named-window" -> new String[]{
                "@public create window PositionW.win:time(1 hour).std:unique(intPrimitive) as select * from SupportBean",
                "insert into PositionW select * from SupportBean",
                "@name('s1') insert into Foo select * from pattern[every a = PositionW -> every b = PositionW]"
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
            // The named-window case asserts the a/b tagged events; render
            // the asserted-field projection {theString,intPrimitive} so
            // the trace carries no unasserted engine defaults.
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
