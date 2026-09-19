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
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the EPL insert-into empty-property-type and
 * event-typed-column parity scenario.
 *
 * Covers four executions across six scenario cases. NamedWindowModelAfter
 * (named-window-model-after) replays EPLInsertIntoEmptyPropType ord 0: an
 * empty-schema keepall window EmptyPropWin populated by a zero-column
 * insert-into from SupportBean, a fire-and-forget "insert into EmptyPropWin
 * select null" / "delete from EmptyPropWin" pair, an on-SupportBean_S0
 * not-matched merge insert and an on-SupportBean_S1 insert, observed through
 * window snapshots and a value record pinning the row event-type name
 * "EmptyPropWin". CreateSchemaInsertInto (create-schema-map,
 * create-schema-objectarray, create-schema-bean) replays the map,
 * objectarray and bean sub-rounds of EPLInsertIntoEmptyPropType ord 1; the
 * map sub-round's soda=true/false double run is compile-path-only and is
 * replayed once (approved difference). Each sub-round sends one SupportBean
 * into an empty schema and observes the s0 listener row plus a value record
 * pinning the delivered row's event-type name (the suite's
 * getEventType().getName()/instanceof assertions); the objectarray
 * sub-round additionally records the s0 subscriber delivery.
 * EventTypedColumnOnMerge (event-typed-column-on-merge) and
 * POJOTypedColumnOnMerge (pojo-typed-column-on-merge) replay
 * EPLInsertIntoEventTypedColumnFromProp ords 0-1: an on-merge writes the
 * triggering event into a table's event-typed lastevent column and into
 * CarOutputStream's outputevent column, and a one-minute timeout pattern
 * deletes the table row and emits the stored event offline, observed
 * through s0 listener records. The map case renders the full nested
 * {carId, tracked} map; the pojo case renders the SupportBean outputevent
 * projected to the asserted field {theString}.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive/
 * longPrimitive, SupportBean_S0 payloads carrying id/p00, SupportBean_S1
 * payloads carrying id/p10 and CarEvent payloads carrying carId/tracked
 * sent as a map underlying (fields absent from a payload keep their
 * defaults).
 *
 * Observations follow the standard protocol: snapshot records over the
 * named statement iterators at the pinned assertIterator positions, value
 * records carrying the first row's event-type name, listener records for
 * the subscribed s0 statements and one subscriber record for the
 * objectarray sub-round. Esper dispatches subscribers before listeners
 * (StatementResultServiceImpl.dispatchInternal runs the natural delivery
 * strategy first), so callback records are buffered per step and emitted
 * in the frozen contract order listener, subscriber, value.
 */
public class EplInsertIntoTypedColumnsScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EplInsertIntoTypedColumnsScenarioOracle <scenario.json>");
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
        // The pinned runner compiles with allow-subscriber so the
        // objectarray sub-round can attach its s0 subscriber.
        config.getCompiler().getByteCode().setAllowSubscriber(true);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EplInsertIntoTypedColumnsScenarioOracle-" + caseName, config);
        long[] currentTime = {0};
        runtime.getEventService().advanceTime(currentTime[0]);
        // Callback records buffer here and are flushed after each step in
        // the frozen contract order (listener, subscriber, value); Esper
        // delivers subscribers before listeners.
        List<JsonObject> pending = new ArrayList<>();
        try {
            List<EPStatement> statements = new ArrayList<>();
            Map<String, Integer> listenerSeq = new HashMap<>();
            int[] subscriberSeq = {0};
            boolean[] valueEmitted = {false};
            boolean emitValueOnFirstDelivery = caseName.startsWith("create-schema-");
            for (String epl : buildEPL(caseName)) {
                // Sequential deploys mirror env.compileDeploy(epl, path);
                // each statement sees the @public schemas, windows and
                // tables of the previous deployments through the accumulated
                // runtime path. The typed-column cases deploy their whole
                // module as a single compileDeploy like the Java source.
                CompilerArguments compilerArgs = new CompilerArguments(config);
                compilerArgs.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement added : deployment.getStatements()) {
                    statements.add(added);
                    if ("s0".equals(added.getName())) {
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
                            // The create-schema sub-rounds pin the delivered
                            // row's event-type name (the suite's
                            // getEventType().getName()/instanceof assertions)
                            // as a value record on the first delivery.
                            if (emitValueOnFirstDelivery && !valueEmitted[0] && newData != null && newData.length > 0) {
                                valueEmitted[0] = true;
                                JsonObject valueRecord = new JsonObject();
                                valueRecord.add("case", caseName);
                                valueRecord.add("operation", "value");
                                valueRecord.add("statement", stmt.getName());
                                valueRecord.add("sequence", 0);
                                valueRecord.add("value", newData[0].getEventType().getName());
                                pending.add(valueRecord);
                            }
                        });
                        if ("create-schema-objectarray".equals(caseName)) {
                            stmt.setSubscriber(new TraceSubscriber(pending, caseName, runtime, stmt.getName(), subscriberSeq));
                        }
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
                } else if ("advance-time".equals(op)) {
                    currentTime[0] = Instant.parse(step.getString("at", "")).toEpochMilli();
                    runtime.getEventService().advanceTime(currentTime[0]);
                } else if ("snapshot".equals(op)) {
                    snapshot(statements, caseName, step, records, runtime,
                        "any".equals(step.getString("mode", "")));
                } else if ("value".equals(op)) {
                    value(statements, caseName, step, records);
                } else if ("faf-insert".equals(op)) {
                    // compileExecuteFAFNoResult("insert into <W> select null"):
                    // the statement field carries the window name directly
                    // and the query result is discarded.
                    CompilerArguments fafArgs = new CompilerArguments(config);
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled fafCompiled = EPCompilerProvider.getCompiler().compileQuery(
                        "insert into " + step.getString("statement", "") + " select null", fafArgs);
                    runtime.getFireAndForgetService().executeQuery(fafCompiled);
                } else if ("faf-delete".equals(op)) {
                    // compileExecuteFAFNoResult("delete from <W>").
                    CompilerArguments fafArgs = new CompilerArguments(config);
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled fafCompiled = EPCompilerProvider.getCompiler().compileQuery(
                        "delete from " + step.getString("statement", ""), fafArgs);
                    runtime.getFireAndForgetService().executeQuery(fafCompiled);
                } else {
                    throw new IllegalStateException("unsupported op " + op);
                }
                flushPending(pending, records);
            }
            flushPending(pending, records);

        } finally {
            runtime.destroy();
            // Subscriber delivery can be deferred past the last step's
            // flush; emit any records still buffered after destroy.
            flushPending(pending, records);
        }
    }

    /**
     * Emits buffered callback records in the frozen contract order
     * listener, subscriber, value; Esper's dispatchInternal runs the
     * subscriber delivery strategy before the listener loop, so the
     * natural callback order is subscriber first.
     */
    private static void flushPending(List<JsonObject> pending, List<JsonObject> records) {
        if (pending.isEmpty()) {
            return;
        }
        pending.sort(Comparator.comparingInt(EplInsertIntoTypedColumnsScenarioOracle::operationOrder));
        records.addAll(pending);
        pending.clear();
    }

    private static int operationOrder(JsonObject record) {
        return switch (record.getString("operation", "")) {
            case "listener" -> 0;
            case "subscriber" -> 1;
            default -> 2;
        };
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
                JsonValue longPrimitiveVal = payload.get("longPrimitive");
                if (longPrimitiveVal instanceof JsonNumber) {
                    event.setLongPrimitive(((JsonNumber) longPrimitiveVal).asLong());
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
            case "CarEvent" -> {
                Map<String, Object> event = new HashMap<>();
                JsonValue carIdVal = payload.get("carId");
                if (carIdVal instanceof JsonString) {
                    event.put("carId", ((JsonString) carIdVal).asString());
                }
                JsonValue trackedVal = payload.get("tracked");
                if (trackedVal != null && trackedVal.isBoolean()) {
                    event.put("tracked", trackedVal.asBoolean());
                }
                runtime.getEventService().sendEventMap(event, "CarEvent");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    /**
     * Snapshot of the named statement iterator state, placed at the pinned
     * assertIterator positions; mode-any snapshots emit the canonical
     * marshaled-fields row order so the trace is stable against engine
     * iteration order on both sides. The scenario's statement labels map
     * to statement names per case ('window'->'window', 'table'->'table',
     * 'create'->'create'); this scenario uses 'window' only.
     */
    private static void snapshot(List<EPStatement> statements, String caseName, JsonObject step,
                                 List<JsonObject> records, EPRuntime runtime, boolean canonical) {
        String wanted = step.getString("statement", "");
        EPStatement target = null;
        for (EPStatement candidate : statements) {
            if (wanted.equals(candidate.getName())) {
                target = candidate;
                break;
            }
        }
        if (target == null) {
            throw new IllegalStateException("no statement named " + wanted + " in case " + caseName);
        }
        List<JsonObject> projected = new ArrayList<>();
        for (java.util.Iterator<EventBean> it = target.iterator(); it.hasNext(); ) {
            projected.add(renderRow(it.next()));
        }
        if (canonical) {
            projected.sort(Comparator.comparing(row -> row.get("fields").asObject().toString()));
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", target.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = new JsonArray();
        for (JsonObject row : projected) {
            rows.add(row);
        }
        record.add("new", rows);
        records.add(record);
    }

    /**
     * Emits the event-type name of the first row of a fresh snapshot of the
     * named statement, pinning the suite's getEventType().getName()
     * assertions.
     */
    private static void value(List<EPStatement> statements, String caseName, JsonObject step,
                              List<JsonObject> records) {
        String wanted = step.getString("statement", "");
        EPStatement target = null;
        for (EPStatement candidate : statements) {
            if (wanted.equals(candidate.getName())) {
                target = candidate;
                break;
            }
        }
        if (target == null) {
            throw new IllegalStateException("no statement named " + wanted + " in case " + caseName);
        }
        java.util.Iterator<EventBean> it = target.iterator();
        if (!it.hasNext()) {
            throw new IllegalStateException("no rows for value op on statement " + wanted + " in case " + caseName);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "value");
        record.add("statement", target.getName());
        record.add("sequence", 0);
        record.add("value", it.next().getEventType().getName());
        records.add(record);
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
            case "named-window-model-after" -> new String[]{
                "@public create schema EmptyPropSchema()",
                "@name('window') @public create window EmptyPropWin#keepall as EmptyPropSchema",
                "insert into EmptyPropWin() select null from SupportBean",
                "on SupportBean_S0 merge EmptyPropWin when not matched then insert select null",
                "on SupportBean_S1 insert into EmptyPropWin select null"
            };
            case "create-schema-map" -> new String[]{
                "@public create map schema EmptyMapSchema as ()",
                "insert into EmptyMapSchema() select null from SupportBean",
                "@name('s0') select * from EmptyMapSchema"
            };
            case "create-schema-objectarray" -> new String[]{
                "@public create objectarray schema EmptyOASchema()",
                "@public insert into EmptyOASchema select null from SupportBean",
                "@name('s0') select * from EmptyOASchema"
            };
            case "create-schema-bean" -> new String[]{
                "@public create schema MyBeanWithoutProps as com.espertech.esper.regressionlib.support.bean.SupportBeanWithoutProps",
                "@public insert into MyBeanWithoutProps select null from SupportBean",
                "@name('s0') select * from MyBeanWithoutProps"
            };
            case "event-typed-column-on-merge" -> new String[]{
                "@public @buseventtype create schema CarEvent(carId string, tracked boolean);\n" +
                    "create table StatusTable(carId string primary key, lastevent CarEvent);\n" +
                    "on CarEvent(tracked=true) as ce merge StatusTable as st where ce.carId = st.carId \n" +
                    "  when matched \n" +
                    "    then update set lastevent = ce \n" +
                    "  when not matched \n" +
                    "    then insert(carId, lastevent) select ce.carId, ce \n" +
                    "    then insert into CarOutputStream select 'online' as status, ce as outputevent;\n" +
                    "insert into CarTimeoutStream select e.* \n" +
                    "  from pattern[every e=CarEvent(tracked=true) -> (timer:interval(1 minutes) and not CarEvent(carId = e.carId, tracked=true))];\n" +
                    "on CarTimeoutStream as cts merge StatusTable as st where cts.carId = st.carId \n" +
                    "  when matched \n" +
                    "    then delete \n" +
                    "    then insert into CarOutputStream select 'offline' as status, lastevent as outputevent;\n" +
                    "@name('s0') select * from CarOutputStream"
            };
            case "pojo-typed-column-on-merge" -> new String[]{
                "create schema CarOutputStream(status string, outputevent com.espertech.esper.common.internal.support.SupportBean);\n" +
                    "create table StatusTable(theString string primary key, lastevent com.espertech.esper.common.internal.support.SupportBean);\n" +
                    "on SupportBean as ce merge StatusTable as st where ce.theString = st.theString \n" +
                    "  when matched \n" +
                    "    then update set lastevent = ce \n" +
                    "  when not matched \n" +
                    "    then insert select ce.theString as theString, ce as lastevent\n" +
                    "    then insert into CarOutputStream select 'online' as status, ce as outputevent;\n" +
                    "insert into CarTimeoutStream select e.* \n" +
                    "  from pattern[every e=SupportBean -> (timer:interval(1 minutes) and not SupportBean(theString = e.theString))];\n" +
                    "on CarTimeoutStream as cts merge StatusTable as st where cts.theString = st.theString \n" +
                    "  when matched \n" +
                    "    then delete \n" +
                    "    then insert into CarOutputStream select 'offline' as status, lastevent as outputevent;\n" +
                    "@name('s0') select * from CarOutputStream"
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
            // The pojo case asserts only theString on the delivered
            // outputevent bean; render the asserted-field projection so the
            // trace carries no unasserted engine defaults.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Subscriber for the objectarray sub-round's s0: Esper binds the
     * reflective update(Object[][], Object[][]) signature for multi-row
     * delivery (EventBean[] is not a bindable subscriber signature). The
     * EmptyOASchema row type has no properties, so every delivered row is
     * an empty Object[] and renders {"kind":"row","fields":{}}.
     */
    public static final class TraceSubscriber {
        private final List<JsonObject> pending;
        private final String caseName;
        private final EPRuntime runtime;
        private final String statementName;
        private final int[] sequence;

        private TraceSubscriber(List<JsonObject> pending, String caseName, EPRuntime runtime,
                                String statementName, int[] sequence) {
            this.pending = pending;
            this.caseName = caseName;
            this.runtime = runtime;
            this.statementName = statementName;
            this.sequence = sequence;
        }

        public void update(Object[][] insertStream, Object[][] removeStream) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "subscriber");
            record.add("statement", statementName);
            record.add("sequence", ++sequence[0]);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray rows = new JsonArray();
            if (insertStream != null) {
                for (Object[] row : insertStream) {
                    // Esper delivers select-* rows to Object[][] subscribers
                    // as a single column carrying the underlying; for the
                    // empty objectarray schema that is an empty Object[].
                    if (row.length != 1 || !(row[0] instanceof Object[]) || ((Object[]) row[0]).length != 0) {
                        throw new IllegalStateException("expected an empty-schema row for statement " +
                            statementName + " in case " + caseName);
                    }
                    JsonObject item = new JsonObject();
                    item.add("kind", "row");
                    item.add("fields", new JsonObject());
                    rows.add(item);
                }
            }
            record.add("new", rows);
            pending.add(record);
        }
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
