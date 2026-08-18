import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import java.util.HashMap;
import java.util.Map;
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

/**
 * Java oracle for EPLSubselectUnfiltered unfiltered scalar subselect scenarios.
 *
 * Covers 17 behavioral executions by replaying deterministic event sequences
 * and recording the observable listener output. StartStopStatement and
 * InvalidSubselect are excluded (lifecycle/error-only, covered by Go parity tests).
 */
public class EPLSubselectUnfilteredScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectUnfilteredScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        Map<String, Object> s3Type = new HashMap<>();
        s3Type.put("id", Integer.class);
        config.getCommon().addEventType("SupportBean_S3", s3Type);
        Map<String, Object> s4Type = new HashMap<>();
        s4Type.put("id", Integer.class);
        config.getCommon().addEventType("SupportBean_S4", s4Type);
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectUnfilteredScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            if (epls.length == 1) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epls[0], new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement candidate : deployment.getStatements()) {
                    if ("s0".equals(candidate.getName())) {
                        s0Statements.add(candidate);
                    }
                }
            } else {
                // Multi-statement EPL: compile as a single module so intermediate
                // event types (e.g. insert-into targets) are visible.
                StringBuilder combined = new StringBuilder();
                for (int i = 0; i < epls.length; i++) {
                    if (i > 0) combined.append(";\n");
                    combined.append(epls[i]);
                }
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(combined.toString(), new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement candidate : deployment.getStatements()) {
                    if ("s0".equals(candidate.getName())) {
                        s0Statements.add(candidate);
                    }
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
                stmt.addListener((newData, oldData, statement, rt) -> {
                    if (newData != null) {
                        for (EventBean event : newData) {
                            seq[0]++;
                            JsonObject record = new JsonObject();
                            record.add("case", caseName);
                            record.add("operation", "listener");
                            record.add("statement", statement.getName());
                            record.add("sequence", seq[0]);
                            record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                            JsonArray newArr = new JsonArray();
                            JsonObject newItem = new JsonObject();
                            newItem.add("kind", "row");
                            JsonObject fields = new JsonObject();
                            for (String prop : event.getEventType().getPropertyNames()) {
                                Object value = event.get(prop);
                                fields.add(prop, normalize(event.get(prop)));
                            }
                            newItem.add("fields", fields);
                            newArr.add(newItem);
                            record.add("new", newArr);
                            records.add(record);
                        }
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
                    String type = step.getString("eventType", "");
                    JsonObject payload = step.get("payload").asObject();
                    int id = payload.getInt("id", 0);
                    // Map-based event types avoid regression-lib classpath dependency.
                    if ("SupportBean_S3".equals(type)) {
                        Map<String, Object> m = new HashMap<>();
                        m.put("id", id);
                        runtime.getEventService().sendEventMap(m, type);
                        continue;
                    }
                    if ("SupportBean_S4".equals(type)) {
                        Map<String, Object> m = new HashMap<>();
                        m.put("id", id);
                        runtime.getEventService().sendEventMap(m, type);
                        continue;
                    }
                    Object event = switch (type) {
                        case "SupportBean_S0" -> {
                            SupportBean_S0 e = new SupportBean_S0(id);
                            String p00 = payload.getString("p00", null);
                            if (p00 != null) e.setP00(p00);
                            yield e;
                        }
                        case "SupportBean_S1" -> {
                            SupportBean_S1 e = new SupportBean_S1(id);
                            String p10 = payload.getString("p10", null);
                            if (p10 != null) e.setP10(p10);
                            String p11 = payload.getString("p11", null);
                            if (p11 != null) e.setP11(p11);
                            yield e;
                        }
                        case "SupportBean" -> {
                            SupportBean e = new SupportBean();
                            e.setTheString(payload.getString("theString", ""));
                            e.setIntPrimitive(payload.getInt("intPrimitive", 0));
                            yield e;
                        }
                        default -> throw new IllegalStateException("unknown type: " + type);
                    };
                    runtime.getEventService().sendEventBean(event, type);
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "expression" -> new String[]{
                "@name('s0') select (select p10 || p11 from SupportBean_S1#lastevent) as value from SupportBean_S0"
            };
            case "unlimited-stream" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#length(1000)) as idS1 from SupportBean_S0"
            };
            case "length-window" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#length(2)) as idS1 from SupportBean_S0"
            };
            case "as-after-subselect" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#lastevent) as idS1 from SupportBean_S0"
            };
            case "with-as-within-subselect" -> new String[]{
                "@name('s0') select (select id as myId from SupportBean_S1#lastevent) from SupportBean_S0"
            };
            case "no-as" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1#lastevent) from SupportBean_S0"
            };
            case "last-event" -> new String[]{
                "@name('s0') select theString, (select p00 from SupportBean_S0#lastevent()) as col from SupportBean"
            };
            case "self-subselect" -> new String[]{
                "insert into MyCount select count(*) as cnt from SupportBean_S0",
                "@name('s0') select (select cnt from MyCount#lastevent) as value from SupportBean_S0"
            };
            case "computed-result" -> new String[]{
                "@name('s0') select 100*(select id from SupportBean_S1#length(1000)) as idS1 from SupportBean_S0"
            };
            case "filter-inside" -> new String[]{
                "@name('s0') select (select id from SupportBean_S1(p10='A')#length(1000)) as idS1 from SupportBean_S0"
            };
            case "where-clause-expression" -> new String[]{
                "@name('s0') select id from SupportBean_S0 where (select p10='X' from SupportBean_S1#length(1000))"
            };
            case "where-clause-true" -> new String[]{
                "@name('s0') select id from SupportBean_S0 where (select true from SupportBean_S1#length(1000))"
            };
            case "stream-prior" -> new String[]{
                "@name('s0') select (select prior(0,id) from SupportBean_S1#length(1000)) as idS1 from SupportBean_S0"
            };
            case "two-subq-select" -> new String[]{
                "@name('s0') select (select id+1 as myId from SupportBean_S1#lastevent) as idS1_0, (select id+2 as myId from SupportBean_S1#lastevent) as idS1_1 from SupportBean_S0"
            };
            case "join-unfiltered" -> new String[]{
                "@name('s0') select (select id from SupportBean_S3#length(1000)) as idS3, (select id from SupportBean_S4#length(1000)) as idS4 from SupportBean_S0#keepall as s0, SupportBean_S1#keepall as s1 where s0.id = s1.id"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
    private static JsonValue normalize(Object value) {
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
