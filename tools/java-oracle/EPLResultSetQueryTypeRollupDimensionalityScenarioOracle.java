import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ResultSetQueryTypeRollupDimensionality unbound-rollup and
 * cube scenarios (rollup/cube/grouping sets group-by clauses over unbound
 * streams).
 *
 * Covers six executions of ResultSetQueryTypeRollupDimensionality across 14
 * scenario cases. Unbound-rollup family (10 cases):
 * UnboundRollup2Dim (unbound-rollup-2dim, sum(longPrimitive) grouped by
 * rollup(theString, intPrimitive)), UnboundRollup1Dim
 * (unbound-rollup-1dim-rollup and unbound-rollup-1dim-cube replaying
 * tryAssertionUnboundRollup1Dim with rollup(theString) and
 * cube(theString)), UnboundRollupUnenclosed (unbound-rollup-unenclosed-a/b/c
 * replaying tryAssertionUnboundRollupUnenclosed with theString prepended to
 * rollup(intPrimitive, longPrimitive), a flat grouping sets list, and
 * theString combined with grouping sets over the numeric pair), and
 * UnboundRollup3Dim (unbound-rollup-3dim-rollup,
 * unbound-rollup-3dim-gs and their -join variants replaying
 * tryAssertionUnboundRollup3Dim with rollup and grouping sets over three
 * keys, the -join variants adding a SupportBean_S0#lastevent stream).
 * Cube family (4 cases): UnboundCubeUnenclosed
 * (unbound-cube-unenclosed-a/b/c replaying the three equivalent syntaxes
 * theString + cube(i,l), flat grouping sets, and theString + inner grouping
 * sets with an empty set) and UnboundCube4Dim (unbound-cube-4dim, a
 * four-dimension cube with sum(intBoxed)).
 *
 * Events are SupportBean payloads carrying theString/intPrimitive/
 * longPrimitive/doublePrimitive/intBoxed (fields absent from a payload keep
 * their defaults) and SupportBean_S0 payloads carrying id only.
 *
 * Extends differential coverage by twelve further executions across 17
 * additional scenario cases: warray-unbound, warray-bound and warray-join
 * (count aggregation grouped by rollup over an int[] key compared by content,
 * the join variant priming a bare SupportBean), warray-gs (sum over grouping
 * sets of three typed arrays emitting one row per set in declaration order),
 * nw-cube and nw-cube-gs (IRStream pairs from a named window fed by inserts
 * and emptied by an on-delete statement, cube versus byte-equivalent grouping
 * sets), onselect-rollup (grouped on-select over a named window triggered by
 * SupportBean_S0), out-when-term-last/-last-opt/-last-optdis/-all/-snapshot
 * (context-partition output when terminated flushing exact-order batches;
 * five sub-run variants sharing one runtime ID), bound-gs-no-top and
 * bound-gs-top-detail (two-level bound grouping sets including an
 * expiry-driven update), mixed-access (window(*) rendering nested events as
 * recursive sorted-field rows), non-boxed-types (aliased output column
 * types c0/c1/c2 of every deployed statement recorded via the "types" op) and
 * groupby-computation (rollup over a computed case-when key).
 *
 * Additional send payloads follow the same absent-member-default convention:
 * SupportEventWithIntArray{id, array, value} and SupportThreeArrayEvent{id,
 * value, intArray, longArray, doubleArray} decode JSON arrays into their
 * int[]/long[]/double[] members.
 */
public class EPLResultSetQueryTypeRollupDimensionalityScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLResultSetQueryTypeRollupDimensionalityScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportEventWithIntArray.class);
        config.getCommon().addEventType(SupportThreeArrayEvent.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLResultSetQueryTypeRollupDimensionalityScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            StringBuilder moduleText = new StringBuilder();
            for (int i = 0; i < epls.length; i++) {
                if (i > 0) {
                    moduleText.append(';');
                }
                moduleText.append(epls[i]);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(moduleText.toString(), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0Statements.add(candidate);
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
                stmt.addListener((newData, oldData, statement, rt) -> {
                    if (newData != null && newData.length > 0) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("sequence", seq[0]);
                        record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                        JsonArray newArr = new JsonArray();
                        for (EventBean event : newData) {
                            JsonObject newItem = new JsonObject();
                            newItem.add("kind", "row");
                            JsonObject fields = new JsonObject();
                            for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                                fields.add(prop, normalize(event.get(prop)));
                            }
                            newItem.add("fields", fields);
                            newArr.add(newItem);
                        }
                        record.add("new", newArr);
                        if (oldData != null && oldData.length > 0) {
                            JsonArray oldArr = new JsonArray();
                            for (EventBean event : oldData) {
                                JsonObject oldItem = new JsonObject();
                                oldItem.add("kind", "row");
                                JsonObject oldFields = new JsonObject();
                                for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                                    oldFields.add(prop, normalize(event.get(prop)));
                                }
                                oldItem.add("fields", oldFields);
                                oldArr.add(oldItem);
                            }
                            record.add("old", oldArr);
                        }
                        records.add(record);
                    }
                });
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
                            JsonValue doublePrimitiveVal = payload.get("doublePrimitive");
                            if (doublePrimitiveVal instanceof JsonNumber) {
                                event.setDoublePrimitive(((JsonNumber) doublePrimitiveVal).asDouble());
                            }
                            JsonValue intBoxedVal = payload.get("intBoxed");
                            if (intBoxedVal instanceof JsonNumber) {
                                event.setIntBoxed(((JsonNumber) intBoxedVal).asInt());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean");
                        }
                        case "SupportBean_S0" -> {
                            SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                            runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                        }
                        case "SupportEventWithIntArray" -> {
                            SupportEventWithIntArray event = new SupportEventWithIntArray(
                                payload.getString("id", null),
                                readIntArray(payload, "array"),
                                payload.getInt("value", 0));
                            runtime.getEventService().sendEventBean(event, "SupportEventWithIntArray");
                        }
                        case "SupportThreeArrayEvent" -> {
                            SupportThreeArrayEvent event = new SupportThreeArrayEvent(
                                payload.getString("id", null),
                                payload.getInt("value", 0),
                                readIntArray(payload, "intArray"),
                                readLongArray(payload, "longArray"),
                                readDoubleArray(payload, "doubleArray"));
                            runtime.getEventService().sendEventBean(event, "SupportThreeArrayEvent");
                        }
                        default -> throw new IllegalStateException("unknown eventType: " + type);
                    }
                }
                if ("types".equals(op)) {
                    for (EPStatement candidate : deployment.getStatements()) {
                        JsonObject value = new JsonObject();
                        for (String prop : new TreeSet<>(java.util.Arrays.asList(candidate.getEventType().getPropertyNames()))) {
                            if (!prop.matches("c\\d+")) {
                                // records cover the explicitly aliased
                                // selection columns only; auto-named
                                // aggregate columns are engine-dependent
                                continue;
                            }
                            value.add(prop, candidate.getEventType().getPropertyType(prop).getSimpleName());
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "types");
                        record.add("statement", candidate.getName());
                        record.add("value", value);
                        records.add(record);
                    }
                    continue;
                }
            }

        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "unbound-rollup-2dim" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean group by rollup(theString, intPrimitive)"
            };
            case "unbound-rollup-1dim-rollup" -> new String[]{
                "@Name('s0')select theString as c0, sum(intPrimitive) as c1 from SupportBean group by rollup(theString)"
            };
            case "unbound-rollup-1dim-cube" -> new String[]{
                "@Name('s0')select theString as c0, sum(intPrimitive) as c1 from SupportBean group by cube(theString)"
            };
            case "unbound-rollup-unenclosed-a" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, rollup(intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-unenclosed-b" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),theString)"
            };
            case "unbound-rollup-unenclosed-c" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets((intPrimitive, longPrimitive),(intPrimitive), ())"
            };
            case "unbound-rollup-3dim-rollup" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall group by rollup(theString, intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-3dim-gs" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString),())"
            };
            case "unbound-rollup-3dim-rollup-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall, SupportBean_S0#lastevent group by rollup(theString, intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-3dim-gs-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall, SupportBean_S0#lastevent group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString),())"
            };
            case "unbound-cube-unenclosed-a" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, cube(intPrimitive, longPrimitive)"
            };
            case "unbound-cube-unenclosed-b" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString, longPrimitive),theString)"
            };
            case "unbound-cube-unenclosed-c" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets((intPrimitive, longPrimitive),(intPrimitive),(longPrimitive), ())"
            };
            case "unbound-cube-4dim" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, doublePrimitive as c3, sum(intBoxed) as c4 from SupportBean group by cube(theString, intPrimitive, longPrimitive, doublePrimitive)"
            };
            case "bound-rollup" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(3) group by rollup(theString, intPrimitive)"
            };
            case "bound-rollup-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(3), SupportBean_S0#lastevent group by rollup(theString, intPrimitive)"
            };
            case "unbound-rollup-2dim-batch" -> new String[]{
                "@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length_batch(4) group by rollup(theString, intPrimitive)"
            };
            case "warray-unbound" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray group by rollup(array, value)"
            };
            case "warray-bound" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray#keepall group by rollup(array, value)"
            };
            case "warray-join" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray#keepall, SupportBean#keepall group by rollup(array, value)"
            };
            case "warray-gs" -> new String[]{
                "@Name('s0') select sum(value) as thesum from SupportThreeArrayEvent group by grouping sets((intArray), (longArray), (doubleArray))"
            };
            case "nw-cube" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean(intBoxed = 0);\n" +
                    "on SupportBean(intBoxed = 3) delete from MyWindow;\n" +
                    "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from MyWindow " +
                    "group by cube(theString, intPrimitive)"
            };
            case "nw-cube-gs" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean(intBoxed = 0);\n" +
                    "on SupportBean(intBoxed = 3) delete from MyWindow;\n" +
                    "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from MyWindow " +
                    "group by grouping sets((theString, intPrimitive),(theString),(intPrimitive),())"
            };
            case "onselect-rollup" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean;\n" +
                    "@name('s0') on SupportBean_S0 as s0 select mw.theString as c0, sum(mw.intPrimitive) as c1, count(*) as c2 from MyWindow mw group by rollup(mw.theString);\n"
            };
            case "out-when-term-last" -> outputWhenTerminatedEPL("", "last");
            case "out-when-term-last-opt" -> outputWhenTerminatedEPL("@Hint('ENABLE_OUTPUTLIMIT_OPT')", "last");
            case "out-when-term-last-optdis" -> outputWhenTerminatedEPL("@Hint('DISABLE_OUTPUTLIMIT_OPT')", "last");
            case "out-when-term-all" -> outputWhenTerminatedEPL("", "all");
            case "out-when-term-snapshot" -> outputWhenTerminatedEPL("", "snapshot");
            case "bound-gs-no-top" -> new String[]{
                "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(4) " +
                    "group by grouping sets(theString, intPrimitive)"
            };
            case "bound-gs-top-detail" -> new String[]{
                "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(4) " +
                    "group by grouping sets((), (theString, intPrimitive))"
            };
            case "mixed-access" -> new String[]{
                "@name('s0') select sum(intPrimitive) as c0, theString as c1, window(*) as c2 " +
                    "from SupportBean#length(2) sb group by rollup(theString) order by theString"
            };
            case "non-boxed-types" -> new String[]{
                "@name('s0') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by intPrimitive, rollup(doublePrimitive, longPrimitive)",
                "@name('s1') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((intPrimitive, doublePrimitive, longPrimitive))",
                "@name('s2') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((intPrimitive, doublePrimitive, longPrimitive), (intPrimitive, doublePrimitive))",
                "@name('s3') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((doublePrimitive, intPrimitive), (longPrimitive, intPrimitive))"
            };
            case "groupby-computation" -> new String[]{
                "@name('s0') select longPrimitive as c0, sum(intPrimitive) as c1 " +
                    "from SupportBean group by rollup(case when longPrimitive > 0 then 1 else 0 end)"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static String[] outputWhenTerminatedEPL(String hint, String outputLimit) {
        return new String[]{
            "@name('ctx') create context MyContext start SupportBean_S0(id=1) end SupportBean_S0(id=0);\n" +
                hint + " @name('s0') context MyContext select theString as c0, sum(intPrimitive) as c1 " +
                "from SupportBean group by rollup(theString) output " + outputLimit + " when terminated"
        };
    }

    private static int[] readIntArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        int[] result = new int[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asInt();
        }
        return result;
    }

    private static long[] readLongArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        long[] result = new long[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asLong();
        }
        return result;
    }

    private static double[] readDoubleArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        double[] result = new double[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asDouble();
        }
        return result;
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
        if (value instanceof SupportBean_S1) {
            SupportBean_S1 event = (SupportBean_S1) value;
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(event.getId()));
            fields.add("p10", normalize(event.getP10()));
            fields.add("p11", normalize(event.getP11()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof int[]) {
            JsonArray array = new JsonArray();
            for (int item : (int[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof long[]) {
            JsonArray array = new JsonArray();
            for (long item : (long[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof double[]) {
            JsonArray array = new JsonArray();
            for (double item : (double[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof String[]) {
            JsonArray array = new JsonArray();
            for (String item : (String[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Object[]) {
            // covers window(*) results, which surface as arrays of event
            // underlyings rather than EventBean wrappers
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof SupportBean) {
            // renders nested SupportBean underlyings (window(*)) as
            // sorted-field rows mirroring the engine property names
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("bigDecimal", normalize(bean.getBigDecimal()));
            fields.add("bigInteger", normalize(bean.getBigInteger()));
            fields.add("boolBoxed", normalize(bean.getBoolBoxed()));
            fields.add("boolPrimitive", normalize(bean.isBoolPrimitive()));
            fields.add("byteBoxed", normalize(bean.getByteBoxed()));
            fields.add("bytePrimitive", normalize(bean.getBytePrimitive()));
            fields.add("charBoxed", normalize(bean.getCharBoxed()));
            fields.add("charPrimitive", normalize(bean.getCharPrimitive()));
            fields.add("doubleBoxed", normalize(bean.getDoubleBoxed()));
            fields.add("doublePrimitive", normalize(bean.getDoublePrimitive()));
            fields.add("enumValue", normalize(bean.getEnumValue()));
            fields.add("floatBoxed", normalize(bean.getFloatBoxed()));
            fields.add("floatPrimitive", normalize(bean.getFloatPrimitive()));
            fields.add("intBoxed", normalize(bean.getIntBoxed()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("longBoxed", normalize(bean.getLongBoxed()));
            fields.add("longPrimitive", normalize(bean.getLongPrimitive()));
            fields.add("shortBoxed", normalize(bean.getShortBoxed()));
            fields.add("shortPrimitive", normalize(bean.getShortPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the pinned SupportEventWithIntArray regression bean. */
    public static class SupportEventWithIntArray {
        private final String id;
        private final int[] array;
        private final int value;

        public SupportEventWithIntArray(String id, int[] array, int value) {
            this.id = id;
            this.array = array;
            this.value = value;
        }

        public String getId() {
            return id;
        }

        public int[] getArray() {
            return array;
        }

        public int getValue() {
            return value;
        }
    }

    /** Local mirror of the pinned SupportThreeArrayEvent regression bean. */
    public static class SupportThreeArrayEvent {
        private final String id;
        private final int value;
        private final int[] intArray;
        private final long[] longArray;
        private final double[] doubleArray;

        public SupportThreeArrayEvent(String id, int value, int[] intArray, long[] longArray, double[] doubleArray) {
            this.id = id;
            this.value = value;
            this.intArray = intArray;
            this.longArray = longArray;
            this.doubleArray = doubleArray;
        }

        public String getId() {
            return id;
        }

        public int getValue() {
            return value;
        }

        public int[] getIntArray() {
            return intArray;
        }

        public long[] getLongArray() {
            return longArray;
        }

        public double[] getDoubleArray() {
            return doubleArray;
        }
    }
}
