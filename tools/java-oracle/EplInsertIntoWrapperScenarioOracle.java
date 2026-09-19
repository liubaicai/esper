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
import com.espertech.esper.regressionlib.suite.epl.insertinto.EPLInsertIntoWrapper;
import com.espertech.esper.regressionlib.support.bean.SupportBeanSimple;
import com.espertech.esper.regressionlib.support.bean.SupportEventContainsSupportBean;
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
 * Java oracle for the EPL insert-into wrapper parity scenario.
 *
 * Covers all three EPLInsertIntoWrapper executions, one fresh runtime
 * per case. wrapper-bean replays EPLInsertIntoWrapperBean (ord 0): two
 * @public producers into one shared wrapper type — i1 'insert into
 * WrappedBean select *, intPrimitive as p0 from SupportBean' observed
 * through the i1 listener as {theString:E1,intPrimitive:1,p0:1}, and i2
 * 'insert into WrappedBean select sb from SupportEventContainsSupportBean
 * sb' observed through the i2 listener as {theString:E2,intPrimitive:2,
 * p0:null} ('sb' resolves to the nested-bean property and the unprovided
 * p0 column projects null). three-stream-wrapper replays
 * EPLInsertInto3StreamWrapper (ord 1): three chained 'insert into select
 * irstream *' producers over #length(2) windows with || concat columns
 * (StreamA -> StreamB propA -> StreamC propB), observed through the s2
 * listener; the third send removes e1's row through all three windows so
 * s2 sees e3's new row and e1's fully-wrapped old row in one invocation.
 * split-fork-join replays EPLInsertIntoOnSplitForkJoin (ord 2): a single
 * module of on-trigger multi-clause splits — transpose(UDF(event)) maps
 * SupportBean_S0 to MyEvent, where-filtered fork branches route through
 * BStream/DStreamOne/DStreamTwo/FStreamOne/FStreamTwo, and 'output all'
 * dual inserts feed FinalStream — observed through the final listener,
 * which fires once for S0(1,true,true,false) and is not invoked for
 * S0(1,true,true,true).
 *
 * Events are SupportBean payloads carrying theString/intPrimitive,
 * SupportEventContainsSupportBean payloads carrying a nested 'sb' bean,
 * SupportBeanSimple payloads carrying myString/myInt, and
 * SupportBean_S0 payloads carrying id/p00..p03 (the scenario encodes id
 * as a string; the bean constructor takes int).
 *
 * Observations are listener records only: {case, operation, statement,
 * sequence, time, new, old} emitted for the named i1/i2 (wrapper-bean),
 * s2 (three-stream-wrapper) and final (split-fork-join) statements.
 * Milestone calls in the Java source emit no records and are omitted
 * from the scenario.
 */
public class EplInsertIntoWrapperScenarioOracle {

    /**
     * Asserted-field projections per listened statement. i1/i2 project
     * the WrappedBean columns the Java assertions read; s2 projects the
     * full StreamC row (myInt/propA ride along unasserted); final
     * projects the full MyEvent row.
     */
    private static final Map<String, String[]> PROJECTIONS = Map.of(
        "i1", new String[]{"theString", "intPrimitive", "p0"},
        "i2", new String[]{"theString", "intPrimitive", "p0"},
        "s2", new String[]{"myString", "myInt", "propA", "propB"},
        "final", new String[]{"id", "propOne", "propTwo", "propThree"}
    );

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EplInsertIntoWrapperScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBeanSimple.class);
        config.getCommon().addEventType(SupportEventContainsSupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread; mirror that
        // wrapper so failures surface as exceptions rather than log lines.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EplInsertIntoWrapperScenarioOracle-" + caseName, config);
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
                // path. split-fork-join is a single module, so the loop
                // deploys it once.
                CompilerArguments compilerArgs = new CompilerArguments(config);
                compilerArgs.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement added : deployment.getStatements()) {
                    if (PROJECTIONS.containsKey(added.getName())) {
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
                            record.add("new", renderRows(newData, PROJECTIONS.get(stmt.getName())));
                            record.add("old", renderRows(oldData, PROJECTIONS.get(stmt.getName())));
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
            case "SupportEventContainsSupportBean" -> {
                JsonObject sb = payload.get("sb").asObject();
                SupportBean inner = new SupportBean();
                JsonValue theStringVal = sb.get("theString");
                if (theStringVal instanceof JsonString) {
                    inner.setTheString(((JsonString) theStringVal).asString());
                }
                JsonValue intPrimitiveVal = sb.get("intPrimitive");
                if (intPrimitiveVal instanceof JsonNumber) {
                    inner.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                }
                runtime.getEventService().sendEventBean(
                    new SupportEventContainsSupportBean(inner), "SupportEventContainsSupportBean");
            }
            case "SupportBeanSimple" -> {
                SupportBeanSimple event = new SupportBeanSimple(
                    payload.getString("myString", null), payload.getInt("myInt", 0));
                runtime.getEventService().sendEventBean(event, "SupportBeanSimple");
            }
            case "SupportBean_S0" -> {
                // The scenario encodes id as a string (the Go decoder's
                // wrapperS0.ID is a string); the bean constructor takes
                // int. Accept either JSON shape.
                SupportBean_S0 event = new SupportBean_S0(
                    parseInt(payload.get("id")),
                    payload.getString("p00", null),
                    payload.getString("p01", null),
                    payload.getString("p02", null));
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
            case "wrapper-bean" -> new String[]{
                "@name('i1') @public insert into WrappedBean select *, intPrimitive as p0 from SupportBean",
                "@name('i2') @public insert into WrappedBean select sb from SupportEventContainsSupportBean sb"
            };
            case "three-stream-wrapper" -> new String[]{
                "@name('s0') @public insert into StreamA select irstream * from SupportBeanSimple#length(2)",
                "@name('s1') @public insert into StreamB select irstream *, myString||'A' as propA from StreamA#length(2)",
                "@name('s2') @public insert into StreamC select irstream *, propA||'B' as propB from StreamB#length(2)"
            };
            case "split-fork-join" -> new String[]{
                "@Name('A') \n" +
                    "on SupportBean_S0 event insert into AStream select transpose(" + EPLInsertIntoWrapper.class.getName() + ".transpose(event));\n" +
                    "\n" +
                    "@Name('B') on AStream insert into BStream select * where propOne;\n" +
                    "\n" +
                    "@Name('C') select * from AStream;\n" +
                    "\n" +
                    "@Name('D') \n" +
                    "on BStream insert into DStreamOne \n" +
                    "select * where propTwo\n" +
                    "insert into DStreamTwo select * where not propTwo;\n" +
                    "\n" +
                    "@Name('E') on DStreamTwo\n" +
                    "insert into FinalStream select * insert into otherstream select * output all;\n" +
                    "\n" +
                    "@Name('F') on DStreamOne\n" +
                    "insert into FStreamOne select * where propThree\n" +
                    "insert into FStreamTwo select * where not propThree;\n" +
                    "\n" +
                    "@Name('G') on FStreamTwo\n" +
                    "insert into FinalStream select * insert into otherstream select * output all;\n" +
                    "\n" +
                    "@name('final') select * from FinalStream;\n"
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
