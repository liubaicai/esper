import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_A;
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
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for EPLOtherDistinct select-distinct scenarios.
 *
 * Covers the scalar SupportBean runtimes (OutputSimpleColumn,
 * OutputLimitEveryColumn, BatchWindow) and the MultikeyWArray family
 * (output-limit single/two array, FAF, iterator, on-select) by replaying
 * deterministic sequences and recording deduplicated listener output rows in
 * first-seen insertion order.
 */
public class EPOtherDistinctScenarioOracle {

    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPOtherDistinctScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            if (caseName.startsWith("distinct-mwarray-")) {
                runMultikeyCase(allSteps, caseName, records);
            } else if (caseName.equals("distinct-ondemand-onselect")) {
                runOnDemandCase(allSteps, caseName, records);
            } else {
                runScalarCase(allSteps, caseName, records);
            }
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    // ------------------------------------------------------------------
    // Scalar SupportBean cases: single statement s0 with a listener.
    // ------------------------------------------------------------------

    private static void runScalarCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        if (caseName.equals("distinct-snapshot-column-join") || caseName.equals("distinct-subquery")) {
            config.getCommon().addEventType(SupportBean_A.class);
        }
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPOtherDistinct-" + caseName, config);
        try {
            runtime.getEventService().advanceTime(0);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            EPStatement stmt = findStatement(deployment, "s0");
            java.util.Set<String> projected = caseName.equals("distinct-subquery")
                ? new java.util.HashSet<>(java.util.Arrays.asList("theString", "intPrimitive"))
                : null;
            int[] seq = {0};
            stmt.addListener((newData, oldData, statement, rt) -> {
                if (newData != null && newData.length > 0) {
                    seq[0]++;
                    records.add(listenerRecordProjected(caseName, statement.getName(), seq[0], rt.getEventService().getCurrentTime(), newData, projected));
                }
            });

            replaySteps(allSteps, caseName, runtime, null);

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    // ------------------------------------------------------------------
    // MultikeyWArray cases: SupportEventWithManyArray with int[] keys.
    // ------------------------------------------------------------------

    private static void runMultikeyCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        Map<String, Object> manyArrayType = new HashMap<>();
        manyArrayType.put("id", String.class);
        manyArrayType.put("intOne", int[].class);
        manyArrayType.put("intTwo", int[].class);
        config.getCommon().addEventType("SupportEventWithManyArray", manyArrayType);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPOtherDistinct-" + caseName, config);
        try {
            // Pin the virtual clock to epoch so record timestamps are
            // deterministic regardless of wall-clock start time.
            runtime.getEventService().advanceTime(0);

            Map<String, String[]> modules = buildMultikeyEPL(caseName);
            List<EPStatement> statements = new ArrayList<>();
            List<EPCompiled> compiledUnits = new ArrayList<>();
            EPDeployment lastDeployment = null;
            for (String[] sources : modules.values()) {
                StringBuilder joined = new StringBuilder();
                for (String source : sources) {
                    if (joined.length() > 0) {
                        joined.append(";\n");
                    }
                    joined.append(source);
                }
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(joined.toString(), new CompilerArguments(config));
                compiledUnits.add(compiled);
                lastDeployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement statement : lastDeployment.getStatements()) {
                    statements.add(statement);
                }
            }

            java.util.Set<String> listened = multikeyListenedStatements(caseName);
            Map<String, Integer> listenerSeq = new LinkedHashMap<>();
            for (EPStatement statement : statements) {
                if (!listened.contains(statement.getName())) {
                    continue;
                }
                statement.addListener((newData, oldData, st, rt) -> {
                    if (newData != null && newData.length > 0) {
                        int next = listenerSeq.merge(st.getName(), 1, Integer::sum);
                        records.add(listenerRecord(caseName, st.getName(), next, rt.getEventService().getCurrentTime(), newData));
                    }
                });
            }

            final EPDeployment deploymentForLookup = lastDeployment;
            final CompilerArguments fafArgs = new CompilerArguments(config);
            fafArgs.getPath().getCompileds().addAll(compiledUnits);
            replaySteps(allSteps, caseName, runtime, (statementKey) -> {
                if (statementKey.startsWith("faf-")) {
                    EPCompiled fafCompiled = EPCompilerProvider.getCompiler().compileQuery(
                        fafEPL(statementKey), fafArgs);
                    com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult result =
                        runtime.getFireAndForgetService().executeQuery(fafCompiled);
                    records.add(rowsRecord(caseName, "snapshot", statementKey, 0,
                        runtime.getEventService().getCurrentTime(), result.getArray()));
                } else {
                    EPStatement target = findStatement(deploymentForLookup, statementKey);
                    List<EventBean> rows = new ArrayList<>();
                    java.util.Iterator<EventBean> it = target.iterator();
                    while (it.hasNext()) {
                        rows.add(it.next());
                    }
                    records.add(rowsRecord(caseName, "snapshot", statementKey, 0,
                        runtime.getEventService().getCurrentTime(), rows.toArray(new EventBean[0])));
                }
            });

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Named-window on-demand surface over SupportBean: create window +
     * insert-into modules, an on-select trigger statement, and a FAF query
     * executed through the deployment path.
     */
    private static void runOnDemandCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_A.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPOtherDistinct-" + caseName, config);
        try {
            runtime.getEventService().advanceTime(0);
            StringBuilder module = new StringBuilder();
            for (String source : new String[]{
                "@public create window MyWindow#keepall as select * from SupportBean",
                "insert into MyWindow select * from SupportBean",
                "@name('s0') on SupportBean_A select distinct theString, intPrimitive from MyWindow order by theString, intPrimitive asc"}) {
                if (module.length() > 0) {
                    module.append(";\n");
                }
                module.append(source);
            }
            List<EPCompiled> compiledUnits = new ArrayList<>();
            EPCompiled windowCompiled = EPCompilerProvider.getCompiler().compile(module.toString(), new CompilerArguments(config));
            compiledUnits.add(windowCompiled);
            EPDeployment lastDeployment = runtime.getDeploymentService().deploy(windowCompiled, new DeploymentOptions());

            Map<String, Integer> listenerSeq = new LinkedHashMap<>();
            for (EPStatement statement : lastDeployment.getStatements()) {
                if (!"s0".equals(statement.getName())) {
                    continue;
                }
                statement.addListener((newData, oldData, st, rt) -> {
                    if (newData != null && newData.length > 0) {
                        int next = listenerSeq.merge(st.getName(), 1, Integer::sum);
                        records.add(listenerRecord(caseName, st.getName(), next, rt.getEventService().getCurrentTime(), newData));
                    }
                });
            }

            CompilerArguments fafArgs = new CompilerArguments(config);
            fafArgs.getPath().getCompileds().addAll(compiledUnits);

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
                    if ("SupportBean".equals(type)) {
                        SupportBean bean = new SupportBean();
                        bean.setTheString(payload.getString("theString", ""));
                        bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                        runtime.getEventService().sendEventBean(bean, type);
                    } else if ("SupportBean_A".equals(type)) {
                        runtime.getEventService().sendEventBean(new SupportBean_A(payload.getString("id", "")), type);
                    }
                } else if ("snapshot".equals(op)) {
                    EPCompiled fafCompiled = EPCompilerProvider.getCompiler().compileQuery(
                        "select distinct theString, intPrimitive from MyWindow order by theString, intPrimitive", fafArgs);
                    com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult result =
                        runtime.getFireAndForgetService().executeQuery(fafCompiled);
                    records.add(rowsRecord(caseName, "snapshot", step.getString("statement", "faf"), 0,
                        runtime.getEventService().getCurrentTime(), result.getArray()));
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private interface SnapshotHandler {
        void snapshot(String statementKey) throws Exception;
    }

    private static void replaySteps(JsonArray allSteps, String caseName, EPRuntime runtime, SnapshotHandler snapshots) throws Exception {
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
                sendStep(runtime, step);
            } else if ("advance-time".equals(op)) {
                long millis = java.time.Instant.parse(step.getString("at", "")).toEpochMilli();
                runtime.getEventService().advanceTime(millis);
            } else if ("snapshot".equals(op)) {
                if (snapshots == null) {
                    throw new IllegalStateException("case " + caseName + " has snapshot steps but no handler");
                }
                snapshots.snapshot(step.getString("statement", ""));
            }
        }
    }

    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(payload.getString("theString", ""));
                bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportEventWithManyArray": {
                Map<String, Object> event = new HashMap<>();
                event.put("id", payload.getString("id", "id"));
                JsonValue one = payload.get("intOne");
                event.put("intOne", one == null ? null : intArray(one));
                JsonValue two = payload.get("intTwo");
                event.put("intTwo", (two == null || !(two instanceof JsonArray)) ? null : intArray(two));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(new SupportBean_A(payload.getString("id", "")), type);
                break;
            }
            case "SupportBean_S0": {
                runtime.getEventService().sendEventBean(new SupportBean_S0(payload.getInt("id", 0)), type);
                break;
            }
            case "SupportBean_S1": {
                runtime.getEventService().sendEventBean(new SupportBean_S1(payload.getInt("id", 0)), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown event type: " + type);
        }
    }

    private static int[] intArray(JsonValue value) {
        JsonArray array = value.asArray();
        int[] values = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            values[i] = array.get(i).asInt();
        }
        return values;
    }

    /** listenerRecord restricted to the execution's asserted field surface. */
    private static JsonObject listenerRecordProjected(String caseName, String statement, int sequence, long currentTime, EventBean[] newData, java.util.Set<String> projected) {
        JsonObject record = listenerRecord(caseName, statement, sequence, currentTime, newData);
        if (projected == null) {
            return record;
        }
        JsonArray filtered = new JsonArray();
        for (JsonValue rowVal : record.get("new").asArray()) {
            JsonObject row = rowVal.asObject();
            JsonObject fields = new JsonObject();
            for (String name : projected) {
                fields.set(name, row.get("fields").asObject().get(name));
            }
            JsonObject copy = new JsonObject();
            copy.add("kind", "row");
            copy.add("fields", fields);
            filtered.add(copy);
        }
        record.set("new", filtered);
        return record;
    }

    private static JsonObject listenerRecord(String caseName, String statement, int sequence, long currentTime, EventBean[] newData) {
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "listener");
        record.add("statement", statement);
        record.add("time", java.time.Instant.ofEpochMilli(currentTime).toString());
        record.add("sequence", sequence);
        record.add("new", rowsOf(newData));
        return record;
    }

    private static JsonObject rowsRecord(String caseName, String operation, String statement, int sequence, long currentTime, EventBean[] rows) {
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", operation);
        record.add("statement", statement);
        record.add("time", java.time.Instant.ofEpochMilli(currentTime).toString());
        record.add("sequence", sequence);
        record.add("new", rowsOf(rows));
        return record;
    }

    private static JsonArray rowsOf(EventBean[] events) {
        JsonArray newArr = new JsonArray();
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject fields = new JsonObject();
            for (String prop : event.getEventType().getPropertyNames()) {
                Object value = event.get(prop);
                if (value == null) {
                    JsonObject nullState = new JsonObject();
                    nullState.add("state", "null");
                    fields.add(prop, nullState);
                } else if (value instanceof int[]) {
                    JsonArray array = new JsonArray();
                    for (int element : (int[]) value) {
                        array.add(element);
                    }
                    fields.add(prop, array);
                } else if (value instanceof Number) {
                    fields.add(prop, Json.value(((Number) value).longValue()));
                } else if (value instanceof Boolean) {
                    fields.add(prop, Json.value((Boolean) value));
                } else if (value instanceof EventBean) {
                    fields.add(prop, summarizeUnderlying((EventBean) value));
                } else {
                    fields.add(prop, String.valueOf(value));
                }
            }
            item.add("fields", fields);
            newArr.add(item);
        }
        return newArr;
    }

    private static String summarizeUnderlying(EventBean event) {
        return event.getEventType().getName() + ":" + String.valueOf(event.getUnderlying());
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " not found");
    }





    // ------------------------------------------------------------------
    // EPL construction
    // ------------------------------------------------------------------

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "distinct-simple-column" ->
                "@name('s0') select distinct theString,intPrimitive from SupportBean#keepall";
            case "distinct-output-every-column" ->
                "@name('s0') @IterableUnbound select distinct theString,intPrimitive from SupportBean#keepall output every 3 events";
            case "distinct-batch-window" ->
                "@name('s0') select distinct theString,intPrimitive from SupportBean#length_batch(3)";
            case "distinct-snapshot-column" ->
                "@name('s0') select distinct theString, intPrimitive from SupportBean#keepall output snapshot every 3 events order by theString asc";
            case "distinct-snapshot-column-join" ->
                "@name('s0') select distinct theString, intPrimitive from SupportBean#keepall a, SupportBean_A#keepall b where a.theString = b.id output snapshot every 3 events order by theString asc";
            case "distinct-subquery" ->
                "@name('s0') select * from SupportBean where theString in (select distinct id from SupportBean_A#keepall)";
            default -> throw new IllegalStateException("unknown scalar case: " + caseName);
        };
    }

    /** Returns ordered deployments per case; each entry is one full module. */
    private static Map<String, String[]> buildMultikeyEPL(String caseName) {
        Map<String, String[]> modules = new LinkedHashMap<>();
        switch (caseName) {
            case "distinct-mwarray-output-limit-single" ->
                modules.put("s0", new String[]{"@name('s0') select distinct intOne from SupportEventWithManyArray output every 1 seconds"});
            case "distinct-mwarray-output-limit-two" ->
                modules.put("s0", new String[]{"@name('s0') select distinct intOne,intTwo from SupportEventWithManyArray output every 1 seconds"});
            case "distinct-mwarray-faf" -> modules.put("window", new String[]{
                "@public create window MyWindow#keepall as SupportEventWithManyArray",
                "insert into MyWindow select * from SupportEventWithManyArray"});
            case "distinct-mwarray-iterate" -> modules.put("both", new String[]{
                "@name('s0') select distinct intOne from SupportEventWithManyArray#keepall",
                "@name('s1') select distinct intOne,intTwo from SupportEventWithManyArray#keepall"});
            case "distinct-mwarray-on-select" -> modules.put("all", new String[]{
                "@public create window MyWindow#keepall as SupportEventWithManyArray",
                "insert into MyWindow select * from SupportEventWithManyArray",
                "@name('s0') on SupportBean_S0 select distinct intOne from MyWindow",
                "@name('s1') on SupportBean_S1 select distinct intOne,intTwo from MyWindow"});
            default -> throw new IllegalStateException("unknown multikey case: " + caseName);
        }
        return modules;
    }

    /** Statements whose listener output is part of the observable contract. */
    private static java.util.Set<String> multikeyListenedStatements(String caseName) {
        switch (caseName) {
            case "distinct-mwarray-output-limit-single":
            case "distinct-mwarray-output-limit-two":
                return java.util.Collections.singleton("s0");
            case "distinct-mwarray-on-select": {
                java.util.Set<String> both = new java.util.HashSet<>();
                both.add("s0");
                both.add("s1");
                return both;
            }
            default:
                return java.util.Collections.emptySet();
        }
    }

    private static String fafEPL(String key) {
        return switch (key) {
            case "faf-single" -> "select distinct intOne from MyWindow";
            case "faf-two" -> "select distinct intOne,intTwo from MyWindow";
            default -> throw new IllegalStateException("unknown FAF key: " + key);
        };
    }
}
