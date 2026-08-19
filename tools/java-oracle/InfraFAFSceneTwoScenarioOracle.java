import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for InfraSelectWildcardSceneTwo. Each case
 * creates the named-window or table form of MyInfra, inserts the four
 * SupportBean rows, and executes the two fire-and-forget snapshot queries.
 */
public final class InfraFAFSceneTwoScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "infra-faf-scene-two";
    private static final String[] CASES = {"named-window", "table"};
    private static final String POSITIVE_STATEMENT = "faf-positive";
    private static final String NEGATIVE_STATEMENT = "faf-negative";

    private InfraFAFSceneTwoScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: InfraFAFSceneTwoScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!SCENARIO_ID.equals(scenarioID)) {
            throw new IllegalArgumentException("unexpected scenario id " + scenarioID);
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        validateCases(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateCases(JsonArray steps) {
        int caseCount = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            caseCount++;
            String caseName = step.getString("case", "");
            if (!isCase(caseName)) {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
        }
        if (caseCount != CASES.length) {
            throw new IllegalArgumentException("expected exactly two cases");
        }
        for (String expected : CASES) {
            int matches = 0;
            for (int i = 0; i < steps.size(); i++) {
                JsonObject step = steps.get(i).asObject();
                if ("case".equals(step.getString("op", "")) && expected.equals(step.getString("case", ""))) {
                    matches++;
                }
            }
            if (matches != 1) {
                throw new IllegalArgumentException("expected one case " + expected);
            }
        }
    }

    private static boolean isCase(String caseName) {
        for (String candidate : CASES) {
            if (candidate.equals(caseName)) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        int caseIndex = findCase(allSteps, caseName);
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intBoxed", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("infra-faf-scene-two-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            String create;
            if ("named-window".equals(caseName)) {
                create = "@name('create') @public create window MyInfra.win:keepall() as select theString as key, intBoxed as value from SupportBean";
            } else {
                create = "@name('create') @public create table MyInfra (key string primary key, value int)";
            }
            String insert = "insert into MyInfra(key, value) select irstream theString, intBoxed from SupportBean";
            EPCompiled setup = EPCompilerProvider.getCompiler().compile(
                    create + ";\n" + insert,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(setup);
            if (deployment == null) {
                throw new IllegalStateException("setup deployment was not created");
            }

            int sends = 0;
            int snapshots = 0;
            boolean positiveSeen = false;
            boolean negativeSeen = false;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    send(runtime, step);
                    sends++;
                } else if ("snapshot".equals(op)) {
                    String statement = step.getString("statement", "");
                    if (POSITIVE_STATEMENT.equals(statement)) {
                        if (positiveSeen) {
                            throw new IllegalArgumentException("duplicate positive snapshot");
                        }
                        positiveSeen = true;
                    } else if (NEGATIVE_STATEMENT.equals(statement)) {
                        if (negativeSeen) {
                            throw new IllegalArgumentException("duplicate negative snapshot");
                        }
                        negativeSeen = true;
                    } else {
                        throw new IllegalArgumentException("unsupported snapshot statement " + statement);
                    }
                    snapshot(runtime, caseName, statement, records);
                    snapshots++;
                } else {
                    throw new IllegalArgumentException("unsupported op " + op);
                }
            }
            if (sends != 4 || snapshots != 2 || !positiveSeen || !negativeSeen) {
                throw new IllegalArgumentException("case " + caseName + " must contain four sends and two snapshots");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static int findCase(JsonArray steps, String caseName) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && caseName.equals(step.getString("case", ""))) {
                return i;
            }
        }
        throw new IllegalArgumentException("case not found " + caseName);
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        if (!"SupportBean".equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("unsupported event type " + step.getString("eventType", ""));
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("intBoxed", payload.get("intBoxed").asInt());
        runtime.getEventService().sendEventMap(event, "SupportBean");
    }

    private static void snapshot(EPRuntime runtime, String caseName, String statementName, JsonArray records) throws Exception {
        String predicate;
        if (POSITIVE_STATEMENT.equals(statementName)) {
            predicate = "value> 0";
        } else if (NEGATIVE_STATEMENT.equals(statementName)) {
            predicate = "value < 0";
        } else {
            throw new IllegalArgumentException("unsupported snapshot statement " + statementName);
        }
        String queryText = "select * from MyInfra where " + predicate;
        try {
            EPCompiled query = EPCompilerProvider.getCompiler().compileQuery(
                    queryText,
                    new CompilerArguments(runtime.getRuntimePath()));
            EventBean[] events = runtime.getFireAndForgetService().executeQuery(query).getArray();
            boolean sort = "table".equals(caseName) || NEGATIVE_STATEMENT.equals(statementName);
            JsonArray rows = rows(events, sort);
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "snapshot")
                    .add("statement", statementName)
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (rows.size() > 0) {
                record.add("new", rows);
            }
            records.add(record);
        } catch (RuntimeException ex) {
            throw new IllegalStateException("FAF query failed for " + caseName + "/" + statementName, ex);
        }
    }

    private static JsonArray rows(EventBean[] events, boolean sort) {
        List<NormalizedRow> normalized = new ArrayList<>();
        if (events != null) {
            for (EventBean event : events) {
                normalized.add(normalizeRow(event));
            }
        }
        if (sort) {
            Collections.sort(normalized, Comparator
                    .comparing((NormalizedRow row) -> row.key)
                    .thenComparingLong(row -> row.value));
        }
        JsonArray output = new JsonArray();
        for (NormalizedRow row : normalized) {
            output.add(row.json);
        }
        return output;
    }

    private static NormalizedRow normalizeRow(EventBean event) {
        Object keyValue = event.get("key");
        Object valueValue = event.get("value");
        String key = String.valueOf(keyValue);
        long value = ((Number) valueValue).longValue();
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        java.util.Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        return new NormalizedRow(key, value, new JsonObject().add("kind", "row").add("fields", fields));
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

    private static final class NormalizedRow {
        private final String key;
        private final long value;
        private final JsonObject json;

        private NormalizedRow(String key, long value, JsonObject json) {
            this.key = key;
            this.value = value;
            this.json = json;
        }
    }
}
