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
import com.espertech.esper.common.internal.support.SupportBean_S2;
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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the InfraTableInsertInto insert-into-table parity
 * scenario.
 *
 * Covers five executions of InfraTableInsertInto across five scenario
 * cases. Insert-delete (insert-delete) replays InfraInsertIntoAndDelete:
 * the compound-PK table MyTable(pkey0 string, pkey1 int, c0 long) filled
 * by an insert-into from SupportBean and drained by an
 * on SupportBean_S0 delete matching pkey1 = id and pkey0 = p00 through an
 * insert/delete/reinsert cycle observed with table snapshots at the
 * pinned assertion positions. SameModuleUnkeyed (same-module-unkeyed)
 * fills the unkeyed single-row table MyTableSM once from a single module.
 * TwoModulesUnkeyed (two-modules-unkeyed) deploys the create-table and
 * the insert-into separately and replays the second insert through the
 * send-error op, recording the canonical root-cause text of the
 * unique-index violation ("Unique index violation, table 'MyTableIIU' is
 * a declared to hold a single un-keyed row"). Wildcard-map
 * (wildcard-map) is the map-representation subset of
 * InfraInsertIntoWildcard.tryAssertionWildcard: a @public @buseventtype
 * schema MySchema(p0 string, p1 string) copied by select * into TheTable
 * and observed after one map event. SameModuleKeyed (same-module-keyed)
 * replays InfraInsertIntoSameModuleKeyed: MyTableIIK(pkey string primary
 * key, thesum sum(int)) receives primary-key routing from SupportBean,
 * into-table sum(id) aggregation grouped by p00 from SupportBean_S0,
 * on-insert creation from SupportBean_S1.p10 and on-merge not-matched
 * creation from SupportBean_S2.p20.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive/
 * longPrimitive, SupportBean_S0 payloads carrying id/p00,
 * SupportBean_S1 payloads carrying id/p10, SupportBean_S2 payloads
 * carrying id/p20 and MySchema payloads carrying p0/p1 sent as a map
 * underlying (fields absent from a payload keep their defaults).
 *
 * Observations follow the standard protocol: the pinned suite attaches NO
 * listeners to these statements (every assertion reads the table iterator),
 * so observations are snapshot records over the create-table statement
 * iterator placed at the pinned assertPropsPerRowIterator[AnyOrder]
 * positions (rows keep engine iteration order with sorted field names)
 * plus send-error records carrying the deepest non-null cause message so
 * both runtimes compare identical failure text. Attaching listeners anyway
 * would record insert-into-table output rows carrying engine-internal
 * generated names that are not part of the observable contract.
 */
public class InfraTableInsertIntoScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: InfraTableInsertIntoScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S2.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread and the
        // env.assertThat EPException catch observes them; mirror that
        // wrapper so sendError's root-cause strip matches.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("InfraTableInsertIntoScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            List<EPStatement> statements = new ArrayList<>();
            for (String epl : buildEPL(caseName)) {
                // Sequential deploys mirror env.compileDeploy(epl, path);
                // each statement sees the @public tables and schemas of the
                // previous deployments through the accumulated runtime path.
                CompilerArguments compilerArgs = new CompilerArguments(config);
                compilerArgs.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                // The pinned suite attaches NO listeners to these statements
                // (every assertion reads the table iterator), so the oracle
                // records no listener rows; insert-into-table output rows
                // would otherwise carry engine-internal generated names that
                // are not part of the observable contract.
                for (EPStatement added : deployment.getStatements()) {
                    statements.add(added);
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
                } else if ("send-error".equals(op)) {
                    sendError(runtime, caseName, step, records);
                } else if ("snapshot".equals(op)) {
                    snapshot(statements, caseName, step, records, runtime);
                } else {
                    throw new IllegalStateException("unsupported op " + op);
                }
            }

        } finally {
            runtime.destroy();
        }
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
            case "SupportBean_S2" -> {
                SupportBean_S2 event = new SupportBean_S2(payload.getInt("id", 0), payload.getString("p20", null));
                runtime.getEventService().sendEventBean(event, "SupportBean_S2");
            }
            case "MySchema" -> {
                Map<String, Object> event = new HashMap<>();
                for (String name : payload.names()) {
                    JsonValue value = payload.get(name);
                    if (!(value instanceof JsonString)) {
                        throw new IllegalStateException("MySchema payload member " + name + " must be a string");
                    }
                    event.put(name, ((JsonString) value).asString());
                }
                runtime.getEventService().sendEventMap(event, "MySchema");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    /**
     * Sends an event expected to fail; emits the exact caught root-cause
     * message or the "<no-error>" drift marker.
     */
    private static void sendError(EPRuntime runtime, String caseName, JsonObject step, List<JsonObject> records) {
        String eventType = step.getString("eventType", "");
        JsonObject errorRecord = new JsonObject();
        errorRecord.add("case", caseName);
        errorRecord.add("operation", "send-error");
        errorRecord.add("statement", eventType);
        errorRecord.add("sequence", 0);
        try {
            sendEvent(runtime, step);
            errorRecord.add("value", "<no-error>");
        } catch (RuntimeException ex) {
            // Canonical root-cause message: strips JVM wrapper layers such
            // as EPException("Unexpected exception in statement ...") so
            // both runtimes record identical underlying failure text.
            errorRecord.add("value", rootCauseMessage(ex));
        }
        records.add(errorRecord);
    }

    /** Deepest non-null cause message, the canonical cross-runtime error text. */
    private static String rootCauseMessage(Throwable throwable) {
        Throwable current = throwable;
        while (true) {
            Throwable cause = current.getCause();
            if (cause == null || cause == current) {
                break;
            }
            current = cause;
        }
        return current.getMessage();
    }

    /**
     * Snapshot of the create-table statement iterator state, placed at the
     * pinned assertPropsPerRowIterator[AnyOrder] positions; rows keep engine
     * iteration order and field names are sorted.
     */
    private static void snapshot(List<EPStatement> statements, String caseName, JsonObject step,
                                 List<JsonObject> records, EPRuntime runtime) {
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
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", target.getName());
        record.add("sequence", 0);
        record.add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = new JsonArray();
        java.util.Iterator<EventBean> it = target.iterator();
        while (it.hasNext()) {
            rows.add(renderRow(it.next()));
        }
        record.add("new", rows);
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
            case "insert-delete" -> new String[]{
                "@Name('table') @public create table MyTable(c0 long, pkey1 int primary key, pkey0 string primary key)",
                "@Name('Insert-Into-Table') insert into MyTable select intPrimitive as pkey1, longPrimitive as c0, theString as pkey0 from SupportBean",
                "@Name('Delete-Table') on SupportBean_S0 delete from MyTable where pkey1 = id and pkey0 = p00"
            };
            case "same-module-unkeyed" -> new String[]{
                "@name('create') @public create table MyTableSM(theString string);\n" +
                    "@name('tbl-insert') insert into MyTableSM select theString from SupportBean;\n"
            };
            case "two-modules-unkeyed" -> new String[]{
                "@name('create') @public create table MyTableIIU(theString string)",
                "@name('tbl-insert') insert into MyTableIIU select theString from SupportBean"
            };
            case "wildcard-map" -> new String[]{
                "@public @buseventtype create schema MySchema (p0 string, p1 string)",
                "@name('create') @public create table TheTable (p0 string, p1 string)",
                "insert into TheTable select * from MySchema"
            };
            case "same-module-keyed" -> new String[]{
                "@name('create') create table MyTableIIK(" +
                    "pkey string primary key," +
                    "thesum sum(int));\n" +
                    "insert into MyTableIIK select theString as pkey from SupportBean;\n" +
                    "into table MyTableIIK select sum(id) as thesum from SupportBean_S0 group by p00;\n" +
                    "on SupportBean_S1 insert into MyTableIIK select p10 as pkey;\n" +
                    "on SupportBean_S2 merge MyTableIIK where p20 = pkey when not matched then insert into MyTableIIK select p20 as pkey;"
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
        return Json.value(String.valueOf(value));
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread wrapped as
     * "Unexpected exception in statement '&lt;name&gt;': &lt;cause&gt;", which
     * sendError's root-cause strip then reduces to the underlying message.
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
