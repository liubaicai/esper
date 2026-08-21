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
import java.util.List;
import java.util.TreeSet;

/**
 * Java oracle for EPLSubselectFiltered first-slice scenarios (scalar subquery
 * with where/having/filter).
 *
 * Covers the first slice of EPLSubselectFiltered: 5 behavioral executions
 * across 7 scenario cases - the three HavingNoAgg variants
 * (having-no-filter-no-where, having-w-where, having-w-filter-w-where), the
 * three WhereConstant rounds (where-constant-single-column,
 * where-constant-two-column, where-constant-range), and SelectWithWhereJoined
 * (select-with-where-joined). Same-event, wildcard, previous, and
 * multi-stream joined executions remain in later slices.
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
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectFilteredScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epls[0], new CompilerArguments(config));
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
        return Json.value(String.valueOf(value));
    }
}
