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
 * Java oracle for ResultSetQueryTypeRollupDimensionality unbound-rollup
 * scenarios (rollup/cube/grouping sets group-by clauses over unbound
 * streams).
 *
 * Covers the four unbound-rollup executions of
 * ResultSetQueryTypeRollupDimensionality across 10 scenario cases:
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
 *
 * Events are SupportBean payloads carrying theString/intPrimitive/
 * longPrimitive/doublePrimitive (fields absent from a payload keep their
 * defaults) and SupportBean_S0 payloads carrying id only.
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
                            runtime.getEventService().sendEventBean(event, "SupportBean");
                        }
                        case "SupportBean_S0" -> {
                            SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                            runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                        }
                        default -> throw new IllegalStateException("unknown eventType: " + type);
                    }
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
        return Json.value(String.valueOf(value));
    }
}
