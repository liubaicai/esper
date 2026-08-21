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
import com.espertech.esper.common.internal.support.SupportBean_S2;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithIntArray;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithManyArray;
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
import java.util.HashMap;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for EPLSubselectFiltered first-slice scenarios (scalar subquery
 * with where/having/filter).
 *
 * Covers the first slice of EPLSubselectFiltered: 26 behavioral executions
 * across 31 scenario cases - the three HavingNoAgg variants
 * (having-no-filter-no-where, having-w-where, having-w-filter-w-where), the
 * three WhereConstant rounds (where-constant-single-column,
 * where-constant-two-column, where-constant-range), SelectWithWhereJoined
 * (select-with-where-joined), the three multi-stream join rounds
 * (joined-2-streams, joined-3-streams, joined-3-scene-two), the three
 * MultikeyWArray rounds (multikey-array-primitive, multikey-array-two-field,
 * multikey-array-composite), the two Joined4 numeric-coercion executions
 * (joined-4-coercion-p1/p2/p3, joined-4-back-coercion-p1/p2), the two
 * JoinFiltered executions (join-filtered-one, join-filtered-two), and the
 * three WherePrevious variants (where-previous, where-previous-om,
 * where-previous-compile) - the three variants are behaviorally equivalent
 * and replay the same statement and event sequence, differing only in the
 * compilation path for the OM/Compile rounds. Also covered are the three
 * SameEvent variants (same-event, same-event-om, same-event-compile) with a
 * SupportBean_S1 outer stream and SelectWildcard (select-wildcard) with a
 * SupportBean_S0 outer stream; both select the wildcard subquery
 * (select * from SupportBean_S1#length(1000)) as events1, whose single-event
 * column is observed through assertSame identity equivalence rendered as a
 * field-snapshot row object {"kind":"row","fields":{"id":...,"p10":...}}.
 *
 * Finale slice: select-scene-one (irstream correlated price subquery over a
 * Map-typed SupportMarketDataBean), where-2-subquery (OR of two correlated id
 * subqueries), subselect-mix-max (sort-window wildcard subqueries over a
 * Map-typed SupportSensorEvent) and subselect-prior (three-statement
 * insert-into chain over the derived Pair / PairDuplicatesRemoved types,
 * compiled as one module); event-valued columns render as row objects through
 * the normalize Map and EventBean branches.
 */
public class EPLSubselectFilteredScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectFilteredScenarioOracle <scenario.json>");
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
        Map<String, Object> s3Type = new HashMap<>();
        s3Type.put("id", Integer.class);
        s3Type.put("p30", String.class);
        config.getCommon().addEventType("SupportBean_S3", s3Type);
        config.getCommon().addEventType(SupportEventWithIntArray.class);
        config.getCommon().addEventType(SupportEventWithManyArray.class);
        // Regression-lib support beans are not on the runner classpath; register
        // them as Map event types (same rationale as SupportBean_S3 above).
        Map<String, Object> marketDataType = new HashMap<>();
        marketDataType.put("symbol", String.class);
        marketDataType.put("price", Double.class);
        marketDataType.put("volume", Long.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketDataType);
        Map<String, Object> sensorType = new HashMap<>();
        sensorType.put("id", Integer.class);
        sensorType.put("type", String.class);
        sensorType.put("device", String.class);
        sensorType.put("measurement", Double.class);
        sensorType.put("confidence", Double.class);
        config.getCommon().addEventType("SupportSensorEvent", sensorType);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectFilteredScenarioOracle-" + caseName, config);
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
                            event.setTheString(payload.getString("theString", ""));
                            event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                            JsonValue intBoxedVal = payload.get("intBoxed");
                            if (intBoxedVal instanceof JsonNumber) {
                                event.setIntBoxed(((JsonNumber) intBoxedVal).asInt());
                            }
                            JsonValue longBoxedVal = payload.get("longBoxed");
                            if (longBoxedVal instanceof JsonNumber) {
                                event.setLongBoxed(((JsonNumber) longBoxedVal).asLong());
                            }
                            JsonValue doubleBoxedVal = payload.get("doubleBoxed");
                            if (doubleBoxedVal instanceof JsonNumber) {
                                event.setDoubleBoxed(((JsonNumber) doubleBoxedVal).asDouble());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean");
                        }
                        case "SupportBean_S0" -> {
                            SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                            JsonValue p00 = payload.get("p00");
                            if (p00 instanceof JsonString) {
                                event.setP00(((JsonString) p00).asString());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                        }
                        case "SupportBean_S1" -> {
                            SupportBean_S1 event = new SupportBean_S1(payload.getInt("id", 0));
                            JsonValue p10 = payload.get("p10");
                            if (p10 instanceof JsonString) {
                                event.setP10(((JsonString) p10).asString());
                            }
                            JsonValue p11 = payload.get("p11");
                            if (p11 instanceof JsonString) {
                                event.setP11(((JsonString) p11).asString());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean_S1");
                        }
                        case "SupportEventWithManyArray" -> {
                            String id = payload.getString("id", null);
                            if (id == null) {
                                throw new IllegalStateException("SupportEventWithManyArray payload requires id");
                            }
                            JsonValue intOneVal = payload.get("intOne");
                            int[] intOne = null;
                            if (intOneVal instanceof JsonArray) {
                                JsonArray intOneJson = (JsonArray) intOneVal;
                                intOne = new int[intOneJson.size()];
                                for (int i = 0; i < intOneJson.size(); i++) {
                                    intOne[i] = intOneJson.get(i).asInt();
                                }
                            }
                            JsonValue valueVal = payload.get("value");
                            int value = valueVal instanceof JsonNumber ? ((JsonNumber) valueVal).asInt() : 0;
                            runtime.getEventService().sendEventBean(
                                new SupportEventWithManyArray(id).withIntOne(intOne).withValue(value), "SupportEventWithManyArray");
                        }
                        case "SupportEventWithIntArray" -> {
                            String id = payload.getString("id", null);
                            if (id == null) {
                                throw new IllegalStateException("SupportEventWithIntArray payload requires id");
                            }
                            JsonValue arrayVal = payload.get("array");
                            int[] array = null;
                            if (arrayVal instanceof JsonArray) {
                                JsonArray arrayJson = (JsonArray) arrayVal;
                                array = new int[arrayJson.size()];
                                for (int i = 0; i < arrayJson.size(); i++) {
                                    array[i] = arrayJson.get(i).asInt();
                                }
                            }
                            JsonValue valueVal = payload.get("value");
                            int value = valueVal instanceof JsonNumber ? ((JsonNumber) valueVal).asInt() : 0;
                            runtime.getEventService().sendEventBean(new SupportEventWithIntArray(id, array, value), "SupportEventWithIntArray");
                        }
                        case "SupportBean_S2" -> {
                            SupportBean_S2 event = new SupportBean_S2(payload.getInt("id", 0));
                            JsonValue p20 = payload.get("p20");
                            if (p20 instanceof JsonString) {
                                event.setP20(((JsonString) p20).asString());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean_S2");
                        }
                        case "SupportBean_S3" -> {
                            Map<String, Object> m = new HashMap<>();
                            m.put("id", payload.getInt("id", 0));
                            JsonValue p30 = payload.get("p30");
                            if (p30 instanceof JsonString) {
                                m.put("p30", ((JsonString) p30).asString());
                            }
                            runtime.getEventService().sendEventMap(m, "SupportBean_S3");
                        }
                        case "SupportMarketDataBean" -> {
                            Map<String, Object> m = new HashMap<>();
                            JsonValue symbolVal = payload.get("symbol");
                            if (symbolVal instanceof JsonString) {
                                m.put("symbol", ((JsonString) symbolVal).asString());
                            }
                            JsonValue priceVal = payload.get("price");
                            if (priceVal instanceof JsonNumber) {
                                m.put("price", ((JsonNumber) priceVal).asDouble());
                            }
                            JsonValue volumeVal = payload.get("volume");
                            if (volumeVal instanceof JsonNumber) {
                                m.put("volume", ((JsonNumber) volumeVal).asLong());
                            }
                            runtime.getEventService().sendEventMap(m, "SupportMarketDataBean");
                        }
                        case "SupportSensorEvent" -> {
                            Map<String, Object> m = new HashMap<>();
                            m.put("id", payload.getInt("id", 0));
                            JsonValue typeVal = payload.get("type");
                            if (typeVal instanceof JsonString) {
                                m.put("type", ((JsonString) typeVal).asString());
                            }
                            JsonValue deviceVal = payload.get("device");
                            if (deviceVal instanceof JsonString) {
                                m.put("device", ((JsonString) deviceVal).asString());
                            }
                            JsonValue measurementVal = payload.get("measurement");
                            if (measurementVal instanceof JsonNumber) {
                                m.put("measurement", ((JsonNumber) measurementVal).asDouble());
                            }
                            JsonValue confidenceVal = payload.get("confidence");
                            if (confidenceVal instanceof JsonNumber) {
                                m.put("confidence", ((JsonNumber) confidenceVal).asDouble());
                            }
                            runtime.getEventService().sendEventMap(m, "SupportSensorEvent");
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
            case "having-no-filter-no-where" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean#keepall having theString = 'ID1') as c0 from SupportBean_S0"
            };
            case "having-w-where" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean#keepall where intPrimitive > 15 having theString = 'ID1') as c0 from SupportBean_S0"
            };
            case "having-w-filter-w-where" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(intPrimitive < 20) #keepall where intPrimitive > 15 having theString = 'ID1') as c0 from SupportBean_S0"
            };
            case "where-constant-single-column" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#length(1000) where p10='X') as ids1 from SupportBean_S0"
            };
            case "where-constant-two-column" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#length(1000) where p10='X' and p11='Y') as ids1 from SupportBean_S0"
            };
            case "where-constant-range" -> new String[]{
                "@name('s0') select (select theString from SupportBean#lastevent where intPrimitive between 10 and 20) as ids1 from SupportBean_S0"
            };
            case "select-with-where-joined" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#length(1000) where p10=s0.p00) as ids1 from SupportBean_S0 as s0"
            };
            case "joined-2-streams" -> new String[]{
                "@name('s0') select (select id from SupportBean_S0#length(1000) where p00=s1.p10 and p00=s2.p20) as ids0 from SupportBean_S1#keepall as s1, SupportBean_S2#keepall as s2 where s1.id = s2.id"
            };
            case "joined-3-streams" -> new String[]{
                "@name('s0') select (select id from SupportBean_S0#length(1000) where p00=s1.p10 and p00=s3.p30) as ids0 from SupportBean_S1#keepall as s1, SupportBean_S2#keepall as s2, SupportBean_S3#keepall as s3 where s1.id = s2.id and s2.id = s3.id"
            };
            case "joined-3-scene-two" -> new String[]{
                "@name('s0') select (select id from SupportBean_S0#length(1000) where p00=s1.p10 and p00=s3.p30 and p00=s2.p20) as ids0 from SupportBean_S1#keepall as s1, SupportBean_S2#keepall as s2, SupportBean_S3#keepall as s3 where s1.id = s2.id and s2.id = s3.id"
            };
            case "multikey-array-primitive" -> new String[]{
                "@name('s0') select (select id from SupportEventWithManyArray#keepall as sm where sm.intOne = se.array) as value from SupportEventWithIntArray as se"
            };
            case "multikey-array-two-field" -> new String[]{
                "@name('s0') select (select id from SupportEventWithManyArray#keepall as sm where sm.intOne = se.array and sm.value = se.value) as value from SupportEventWithIntArray as se"
            };
            case "multikey-array-composite" -> new String[]{
                "@name('s0') select (select id from SupportEventWithManyArray#keepall as sm where sm.intOne = se.array and sm.value > se.value) as value from SupportEventWithIntArray as se"
            };
            case "joined-4-coercion-p1" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(theString='S')#length(1000)   where intBoxed=s1.longBoxed and intBoxed=s2.doubleBoxed and doubleBoxed=s3.intBoxed) as ids0 from SupportBean(theString='A')#keepall as s1, SupportBean(theString='B')#keepall as s2, SupportBean(theString='C')#keepall as s3 where s1.intPrimitive = s2.intPrimitive and s2.intPrimitive = s3.intPrimitive"
            };
            case "joined-4-coercion-p2" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(theString='S')#length(1000)   where doubleBoxed=s3.intBoxed and intBoxed=s2.doubleBoxed and intBoxed=s1.longBoxed) as ids0 from SupportBean(theString='A')#keepall as s1, SupportBean(theString='B')#keepall as s2, SupportBean(theString='C')#keepall as s3 where s1.intPrimitive = s2.intPrimitive and s2.intPrimitive = s3.intPrimitive"
            };
            case "joined-4-coercion-p3" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(theString='S')#length(1000)   where doubleBoxed=s3.intBoxed and intBoxed=s1.longBoxed and intBoxed=s2.doubleBoxed) as ids0 from SupportBean(theString='A')#keepall as s1, SupportBean(theString='B')#keepall as s2, SupportBean(theString='C')#keepall as s3 where s1.intPrimitive = s2.intPrimitive and s2.intPrimitive = s3.intPrimitive"
            };
            case "joined-4-back-coercion-p1" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(theString='S')#length(1000)   where longBoxed=s1.intBoxed and longBoxed=s2.doubleBoxed and intBoxed=s3.longBoxed) as ids0 from SupportBean(theString='A')#keepall as s1, SupportBean(theString='B')#keepall as s2, SupportBean(theString='C')#keepall as s3 where s1.intPrimitive = s2.intPrimitive and s2.intPrimitive = s3.intPrimitive"
            };
            case "joined-4-back-coercion-p2" -> new String[]{
                "@name('s0') select (select intPrimitive from SupportBean(theString='S')#length(1000)   where longBoxed=s2.doubleBoxed and intBoxed=s3.longBoxed and longBoxed=s1.intBoxed ) as ids0 from SupportBean(theString='A')#keepall as s1, SupportBean(theString='B')#keepall as s2, SupportBean(theString='C')#keepall as s3 where s1.intPrimitive = s2.intPrimitive and s2.intPrimitive = s3.intPrimitive"
            };
            case "join-filtered-one" -> new String[]{
                "@name('s0') select s0.id as s0id, s1.id as s1id, (select p20 from SupportBean_S2#length(1000) where id=s0.id) as s2p20, (select prior(1, p20) from SupportBean_S2#length(1000) where id=s0.id) as s2p20Prior, (select prev(1, p20) from SupportBean_S2#length(10) where id=s0.id) as s2p20Prev from SupportBean_S0#keepall as s0, SupportBean_S1#keepall as s1 where s0.id = s1.id and p00||p10 = (select p20 from SupportBean_S2#length(1000) where id=s0.id)"
            };
            case "join-filtered-two" -> new String[]{
                "@name('s0') select s0.id as s0id, s1.id as s1id, (select p20 from SupportBean_S2#length(1000) where id=s0.id) as s2p20, (select prior(1, p20) from SupportBean_S2#length(1000) where id=s0.id) as s2p20Prior, (select prev(1, p20) from SupportBean_S2#length(10) where id=s0.id) as s2p20Prev from SupportBean_S0#keepall as s0, SupportBean_S1#keepall as s1 where s0.id = s1.id and (select s0.p00||s1.p10 = p20 from SupportBean_S2#length(1000) where id=s0.id)"
            };
            case "where-previous" -> new String[]{
                "@name('s0') select (select prev(1, id) from SupportBean_S1#length(1000) where id=s0.id) as value from SupportBean_S0 as s0"
            };
            case "where-previous-om" -> new String[]{
                "@name('s0') select (select prev(1, id) from SupportBean_S1#length(1000) where id=s0.id) as value from SupportBean_S0 as s0"
            };
            case "where-previous-compile" -> new String[]{
                "@name('s0') select (select prev(1, id) from SupportBean_S1#length(1000) where id=s0.id) as value from SupportBean_S0 as s0"
            };
            case "same-event" -> new String[]{
                "@name('s0') select (select * from SupportBean_S1#length(1000)) as events1 from SupportBean_S1"
            };
            case "same-event-om" -> new String[]{
                "@name('s0') select (select * from SupportBean_S1#length(1000)) as events1 from SupportBean_S1"
            };
            case "same-event-compile" -> new String[]{
                "@name('s0') select (select * from SupportBean_S1#length(1000)) as events1 from SupportBean_S1"
            };
            case "select-wildcard" -> new String[]{
                "@name('s0') select (select * from SupportBean_S1#length(1000)) as events1 from SupportBean_S0"
            };
            case "select-scene-one" -> new String[]{
                "@name('s0') select irstream s0.price as s0price, (select price from SupportMarketDataBean(symbol='S1')#length(10) s1 where s0.volume = s1.volume) as s1price from SupportMarketDataBean(symbol='S0')#length(2) s0"
            };
            case "where-2-subquery" -> new String[]{
                "@name('s0') select id from SupportBean_S0 as s0 where id = (select id from SupportBean_S1#length(1000) where s0.id = id) or id = (select id from SupportBean_S2#length(1000) where s0.id = id)"
            };
            case "subselect-mix-max" -> new String[]{
                "@name('s0') select (select * from SupportSensorEvent#sort(1, measurement desc)) as high, (select * from SupportSensorEvent#sort(1, measurement asc)) as low from SupportSensorEvent"
            };
            case "subselect-prior" -> new String[]{
                "insert into Pair select * from SupportSensorEvent(device='A')#lastevent as a, SupportSensorEvent(device='B')#lastevent as b where a.type = b.type",
                "insert into PairDuplicatesRemoved select * from Pair(1=2)",
                "@name('s0') insert into PairDuplicatesRemoved select * from Pair where a.id != coalesce((select a.id from PairDuplicatesRemoved#lastevent), -1) and b.id != coalesce((select b.id from PairDuplicatesRemoved#lastevent), -1)"
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
