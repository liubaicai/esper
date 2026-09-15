import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
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
import java.util.HashMap;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ViewTimeWin time-window view scenarios.
 *
 * Covers the fifteen implemented ViewTimeWin executions (ords 2-16) replayed
 * as twenty-one scenario cases, each case running against its own fresh
 * runtime and replaying its pinned module verbatim (only the @name('s0')
 * annotations are sanctioned additions): just-select-star replays
 * ViewTimeJustSelectStar: sliding #time(1 sec) over SupportMarketDataBean
 * observed by listener batches and snapshots at 0.6 s and around the 1.5 s /
 * 1.6 s expiries. sum, sum-group-by and sum-w-filter replay the windowed
 * sum(price) family over bare 30 ms windows (ungrouped, group by symbol,
 * filtered to symbol = 'IBM') with the quiet 35 s expiry splitting the
 * before/after sums. month-scoped replays ViewTimeWindowMonthScoped:
 * rstream calendar-month expiry across the 2002-02/03 month boundaries.
 * w-prev replays ViewTimeWindowWPrev: prev(1)/prevtail/prevcount/prevwindow
 * columns over the sliding 1 s window, the window array surfacing as a JSON
 * array per row. prepared-stmt replays ViewTimeWindowPreparedStmt: the
 * unnamed text select rstream theString from SupportBean#time(?::int) is
 * compiled once and deployed twice with positional substitution values 4 and
 * 3 and runtime statement names s0/s1, listeners attached after both deploys,
 * so E1 expires on s1 at 4 s and on s0 at 5 s. variable-stmt replays
 * ViewTimeWindowVariableStmt: the same unnamed text over variable
 * TIME_WIN_ONE deployed as s0 (value 4), then the variable is set to 3 and
 * the text deploys again as s1. time-period replays ViewTimeWindowTimePeriod:
 * #time(4 sec) as s0 and #time(3000 milliseconds) as s1. variable-time-period
 * replays ViewTimeWindowVariableTimePeriodStmt: TIME_WIN_TWO milliseconds as
 * s0 (default 4000), then TIME_WIN_TWO is set to 0.05 and TIME_WIN_TWO
 * minutes deploys as s1. time-period-params-1..7 replay the seven
 * ViewTimeWindowTimePeriodParams interval spellings (30000, 30E6
 * milliseconds, 30000 seconds, 500 minutes, 8.33333333333333333333 hours,
 * 0.34722222222222222222222222222222 days, 0.1 hour 490 min 240 sec) each
 * equal to 30000 s: quiet at the 29999 s boundary, expiry at 30000 s, then
 * the pinned undeploy. flip-timer-1s, flip-timer-10s-large-start,
 * flip-timer-months-ms-epoch and flip-timer-months-ms-2002 replay the four
 * ViewTimeWindowFlipTimer variants, the month spellings flipping one calendar
 * month plus 10/50 ms after epoch and 2002-05-01 starts, observed with
 * snapshots one millisecond before and at each flip time.
 *
 * Configuration follows the pinned harness: SupportMarketDataBean is a map
 * type {symbol string, price double, volume long, feed string} (sends set
 * symbol, price defaults 0.0, volume 0L, feed null, mirroring
 * makeMarketDataEvent); SupportBean is mirrored locally as {theString string,
 * intPrimitive int, doubleBoxed Double} because the pinned bean lives outside
 * the oracle classpath; variables TIME_WIN_ONE int 4 and TIME_WIN_TWO double
 * 4000 are registered exactly as the pinned TestSuiteView.configure. The
 * internal timer is disabled and advanceTime(0) is not issued at runtime
 * creation because every case begins with its own advance-time step (or, for
 * sum-group-by and sum-w-filter, a deploy at the default time zero).
 *
 * Each case owns an ordered deployment plan. Most cases compile
 * buildEPL(caseName) once and deploy a single s0. The five multi-deploy cases
 * deviate as in the pinned test: prepared-stmt compiles once and deploys
 * twice through DeploymentOptions with a positional substitution parameter
 * (1 -> 4 then 1 -> 3) and a runtime statement-name override (s0 then s1),
 * adding listeners to both statements only after both deploys; variable-stmt
 * and variable-time-period deploy their per-variable texts as s0, apply the
 * set-variable step from the scenario, then deploy as s1; time-period deploys
 * its two inline-named texts back to back. Undeploy steps resolve the named
 * statement's deployment id recorded at deploy time.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch containing rows, one sequence counter per case starting at 1 shared
 * across the case's statements in delivery order, time rendered from the
 * current engine time, and new/old row arrays rendered with the scalar
 * normalization rules (prevwindow surfaces as an array). Snapshot records
 * carry no time or sequence: step {op:"snapshot", statement:"s0"} iterates
 * the named statement and emits its current window contents.
 */
public class ViewTimeWinScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewTimeWinScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently against its own runtime,
        // matching one runtime ID per execution.
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("scenario", scenario.getString("description", ""));
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        // Pinned harness defaults (TestSuiteView.configure for ViewTimeWin):
        // TIME_WIN_ONE int 4, TIME_WIN_TWO double 4000.
        config.getCommon().addVariable("TIME_WIN_ONE", int.class, 4);
        config.getCommon().addVariable("TIME_WIN_TWO", double.class, 4000);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewTimeWinScenarioOracle-" + caseName, config);
        try {
            List<PlanEntry> plan = deploymentPlan(caseName);
            Map<String, EPCompiled> compiledByText = new HashMap<>();
            Map<String, EPStatement> statements = new LinkedHashMap<>();
            int[] seq = new int[] {0};
            int[] planIndex = new int[] {0};

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
                    continue;
                }
                if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    continue;
                }
                if ("deployed".equals(op)) {
                    if (planIndex[0] >= plan.size()) {
                        throw new IllegalStateException("unexpected deployed op for case " + caseName);
                    }
                    PlanEntry entry = plan.get(planIndex[0]);
                    EPStatement statement = deployPlanned(runtime, config, caseName, entry,
                        step.getString("statement", ""), compiledByText);
                    planIndex[0]++;
                    statements.put(statement.getName(), statement);
                    if (!"prepared-stmt".equals(caseName)) {
                        addListener(runtime, caseName, seq, records, statement);
                    } else if (planIndex[0] == plan.size()) {
                        // The pinned test adds listeners only after both
                        // deploys; nothing can fire before that anyway.
                        for (EPStatement pending : statements.values()) {
                            addListener(runtime, caseName, seq, records, pending);
                        }
                    }
                    continue;
                }
                if ("set-variable".equals(op)) {
                    JsonObject payload = step.get("payload").asObject();
                    String type = payload.getString("type", "");
                    JsonValue value = payload.get("value");
                    Object javaValue;
                    if ("int".equals(type)) {
                        javaValue = ((JsonNumber) value).asInt();
                    } else if ("double".equals(type)) {
                        javaValue = ((JsonNumber) value).asDouble();
                    } else {
                        throw new IllegalStateException("unsupported variable type: " + type);
                    }
                    runtime.getVariableService().setVariableValue(null, step.getString("name", ""), javaValue);
                    continue;
                }
                if ("undeploy".equals(op)) {
                    String statementName = step.getString("statement", "");
                    EPStatement statement = statements.get(statementName);
                    if (statement == null) {
                        throw new IllegalStateException(
                            "undeploy references statement " + statementName + " that was not deployed for case " + caseName);
                    }
                    runtime.getDeploymentService().undeploy(statement.getDeploymentId());
                    statements.remove(statementName);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPStatement statement = statements.get(statementName);
                    if (statement == null) {
                        throw new IllegalStateException(
                            "snapshot requires statement " + statementName + " for case " + caseName);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statementName);
                    record.add("new", rows(statement.iterator()));
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /** One planned deployment: EPL text, runtime statement name, substitution value. */
    private static final class PlanEntry {
        private final String epl;
        private final String statementName;
        private final Integer substitutionValue;

        private PlanEntry(String epl, String statementName, Integer substitutionValue) {
            this.epl = epl;
            this.statementName = statementName;
            this.substitutionValue = substitutionValue;
        }
    }

    /**
     * Ordered deployment plan per case. Most cases compile buildEPL once for
     * a single s0; the multi-deploy cases transcribe the pinned deployment
     * order, including the unnamed prepared and variable texts that receive
     * their s0/s1 names at deploy (or via added inline annotations exactly as
     * the pinned test spells them).
     */
    private static List<PlanEntry> deploymentPlan(String caseName) {
        return switch (caseName) {
            case "prepared-stmt" -> List.of(
                new PlanEntry("select rstream theString from SupportBean#time(?::int)", "s0", 4),
                new PlanEntry("select rstream theString from SupportBean#time(?::int)", "s1", 3));
            case "variable-stmt" -> List.of(
                new PlanEntry("@name('s0') select rstream theString from SupportBean#time(TIME_WIN_ONE)", "s0", null),
                new PlanEntry("@name('s1') select rstream theString from SupportBean#time(TIME_WIN_ONE)", "s1", null));
            case "time-period" -> List.of(
                new PlanEntry("@name('s0') select rstream theString from SupportBean#time(4 sec)", "s0", null),
                new PlanEntry("@name('s1') select rstream theString from SupportBean#time(3000 milliseconds)", "s1", null));
            case "variable-time-period" -> List.of(
                new PlanEntry("@name('s0')select rstream theString from SupportBean#time(TIME_WIN_TWO milliseconds)", "s0", null),
                new PlanEntry("@name('s1')select rstream theString from SupportBean#time(TIME_WIN_TWO minutes)", "s1", null));
            default -> List.of(new PlanEntry(buildEPL(caseName), "s0", null));
        };
    }

    private static EPStatement deployPlanned(EPRuntime runtime, Configuration config, String caseName,
                                             PlanEntry entry, String stepName,
                                             Map<String, EPCompiled> compiledByText) throws Exception {
        if (!entry.statementName.equals(stepName)) {
            throw new IllegalStateException("deployed step names " + stepName + " but the pinned plan expects " +
                entry.statementName + " for case " + caseName);
        }
        EPCompiled compiled = compiledByText.get(entry.epl);
        if (compiled == null) {
            compiled = EPCompilerProvider.getCompiler().compile(entry.epl, new CompilerArguments(config));
            compiledByText.put(entry.epl, compiled);
        }
        DeploymentOptions options = new DeploymentOptions();
        if (entry.substitutionValue != null) {
            int substitutionValue = entry.substitutionValue;
            String statementName = entry.statementName;
            // Positional form of the pinned SupportPortableDeploySubstitutionParams(1, value).
            options.setStatementSubstitutionParameter(env -> env.setObject(1, substitutionValue));
            options.setStatementNameRuntime(env -> statementName);
        }
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, options);
        EPStatement statement = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), stepName);
        if (statement == null) {
            throw new IllegalStateException("statement " + stepName + " was not deployed for case " + caseName);
        }
        return statement;
    }

    private static void addListener(EPRuntime runtime, String caseName, int[] seq, List<JsonObject> records,
                                    EPStatement statement) {
        statement.addListener((newData, oldData, stmt, rt) -> {
            boolean hasNew = newData != null && newData.length > 0;
            boolean hasOld = oldData != null && oldData.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            seq[0]++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", stmt.getName());
            record.add("sequence", seq[0]);
            record.add("time", Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newData));
            }
            if (hasOld) {
                record.add("old", rows(oldData));
            }
            records.add(record);
        });
    }

    /** Verbatim transcriptions of the pinned ViewTimeWin modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "scene-one", "scene-two" ->
                "@Name('s0') select irstream * from SupportBean#time(10 sec)";
            case "just-select-star" ->
                "@name('s0') select irstream * from SupportMarketDataBean#time(1 sec)";
            case "sum" ->
                "@name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean#time(30)";
            case "sum-group-by" ->
                "@name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean#time(30) group by symbol";
            case "sum-w-filter" ->
                "@name('s0') select symbol, volume, sum(price) as mySum " +
                    "from SupportMarketDataBean(symbol = 'IBM')#time(30)";
            case "month-scoped" ->
                "@name('s0') select rstream * from SupportBean#time(1 month)";
            case "w-prev" ->
                "@name('s0') select irstream symbol, " +
                    "prev(1, symbol) as prev1, " +
                    "prevtail(symbol) as prevtail, " +
                    "prevcount(symbol) as prevCountSym, " +
                    "prevwindow(symbol) as prevWindowSym " +
                    "from SupportMarketDataBean#time(1 sec)";
            case "time-period-params-1" -> timePeriodParamsEPL("30000");
            case "time-period-params-2" -> timePeriodParamsEPL("30E6 milliseconds");
            case "time-period-params-3" -> timePeriodParamsEPL("30000 seconds");
            case "time-period-params-4" -> timePeriodParamsEPL("500 minutes");
            case "time-period-params-5" -> timePeriodParamsEPL("8.33333333333333333333 hours");
            case "time-period-params-6" -> timePeriodParamsEPL("0.34722222222222222222222222222222 days");
            case "time-period-params-7" -> timePeriodParamsEPL("0.1 hour 490 min 240 sec");
            case "flip-timer-1s" -> flipTimerEPL("1");
            case "flip-timer-10s-large-start" -> flipTimerEPL("10");
            case "flip-timer-months-ms-epoch" -> flipTimerEPL("1 months 10 milliseconds");
            case "flip-timer-months-ms-2002" -> flipTimerEPL("1 months 50 milliseconds");
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static String timePeriodParamsEPL(String intervalSpec) {
        return "@name('s0') select irstream * from SupportBean#time(" + intervalSpec + ")";
    }

    private static String flipTimerEPL(String size) {
        return "@name('s0') select * from SupportBean#time(" + size + ")";
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            JsonValue priceVal = payload.get("price");
            event.put("price", priceVal instanceof JsonNumber ? ((JsonNumber) priceVal).asDouble() : 0.0d);
            JsonValue volumeVal = payload.get("volume");
            event.put("volume", volumeVal instanceof JsonNumber ? ((JsonNumber) volumeVal).asLong() : 0L);
            event.put("feed", payload.getString("feed", null));
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            LocalSupportBean event = new LocalSupportBean();
            JsonValue theStringVal = payload.get("theString");
            if (theStringVal instanceof JsonString) {
                event.setTheString(((JsonString) theStringVal).asString());
            }
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            if (intPrimitiveVal instanceof JsonNumber) {
                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
            }
            JsonValue doubleBoxedVal = payload.get("doubleBoxed");
            if (doubleBoxedVal instanceof JsonNumber) {
                event.setDoubleBoxed(((JsonNumber) doubleBoxedVal).asDouble());
            }
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonArray rows(Iterator<EventBean> iterator) {
        JsonArray array = new JsonArray();
        while (iterator.hasNext()) {
            array.add(row(iterator.next()));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        item.add("fields", fields);
        return item;
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
        if (value instanceof Object[]) {
            // covers prevwindow(symbol), which surfaces as an array of
            // window values rather than a scalar
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the pinned SupportBean regression bean members in use. */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private Double doubleBoxed;

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }

        public Double getDoubleBoxed() {
            return doubleBoxed;
        }

        public void setDoubleBoxed(Double doubleBoxed) {
            this.doubleBoxed = doubleBoxed;
        }
    }
}
