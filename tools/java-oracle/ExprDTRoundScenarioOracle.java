import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Calendar;
import java.util.Date;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the ExprDTRound datetime-rounding parity scenario.
 *
 * Covers all four registered executions against a five-representation
 * SupportDateTime map type rebuilt structurally from one ISO instant string,
 * mirroring SupportDateTime.make under the pinned system default timezone:
 * longdate carries the full-precision epoch-millis Long, utildate the
 * full-precision Date, caldate a system-default Calendar whose MILLISECOND
 * field is forced to 0, localdate a system-default LocalDateTime and
 * zoneddate that local view at ZoneId.systemDefault().
 *
 * Input replays ExprDTRoundInput: roundCeiling('hour') applied to all five
 * representations of 2002-05-30T09:01:02.003 yields ten o'clock in every
 * column (val0..val4). Ceil and floor replay ExprDTRoundCeil and
 * ExprDTRoundFloor: roundCeiling/roundFloor over msec,sec,minutes,hour,day,
 * month,year on utildate (val0..val6). Half replays ExprDTRoundHalf phase A
 * (roundHalf over the same seven units at 2002-05-30T15:30:02.550: msec is
 * the identity, seconds round up past the .500 tie, minutes stay, the hour
 * rounds up at the 30-minute tie, the day crosses to May 31st past noon, the
 * month carries to June 1st following Apache Commons month-length-dependent
 * rounding and the year stays at 2002-01-01) followed by phase B, a
 * redeployment selecting utildate.roundHalf('min') driven by the
 * round-half-min deploy marker: 29.999 rounds down, the exact 30.000 tie
 * rounds up and 30.001 rounds up.
 *
 * Records follow the standard protocol: the automatic s0 listener emits one
 * record per delivery with per-case sequences starting at one (the counter
 * survives the mid-case redeployment in half), only the new stream is
 * recorded, and time is frozen at epoch zero because the internal timer is
 * disabled and time advances to zero only. Instant-valued output cells
 * (Date, Calendar, LocalDateTime, ZonedDateTime) collapse to epoch-millis
 * numbers, the convention established by ExprCoreExistsCastScenarioOracle;
 * longdate is already an epoch-millis Long.
 */
public class ExprDTRoundScenarioOracle {
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ExprDTRoundScenarioOracle <scenario.json>");
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
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = configure();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ExprDTRoundScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            ListenerRecorder listener = new ListenerRecorder(caseName, runtime, records);
            deploy(config, runtime, caseName, "s0", listener);
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
                switch (op) {
                    case "deploy" -> {
                        // Mirrors env.undeployAll() plus recompile-deploy inside
                        // ExprDTRoundHalf; the listener keeps counting sequences.
                        runtime.getDeploymentService().undeployAll();
                        deploy(config, runtime, caseName, step.getString("statement", ""), listener);
                    }
                    case "send" -> sendEvent(runtime, step);
                    default -> throw new IllegalStateException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /** Five-representation SupportDateTime mirroring SupportDateTime.make. */
    private static Configuration configure() {
        Configuration config = new Configuration();
        Map<String, Object> type = new LinkedHashMap<>();
        type.put("longdate", Long.class);
        type.put("utildate", Date.class);
        type.put("caldate", Calendar.class);
        type.put("localdate", LocalDateTime.class);
        type.put("zoneddate", java.time.ZonedDateTime.class);
        config.getCommon().addEventType("SupportDateTime", type);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        return config;
    }

    /**
     * Statement-key to pinned-EPL mapping taken verbatim from ExprDTRound;
     * every case deploys s0 and half redeploys s0 for the 'min' variant.
     */
    private static String eplFor(String caseName, String statementKey) {
        if (!"s0".equals(statementKey) && !"round-half-min".equals(statementKey)) {
            throw new IllegalStateException("unknown statement " + statementKey + " in case " + caseName);
        }
        if ("round-half-min".equals(statementKey)) {
            if (!"half".equals(caseName)) {
                throw new IllegalStateException("statement " + statementKey + " belongs to case half");
            }
            return "@name('s0') select utildate.roundHalf('min') as val0 from SupportDateTime";
        }
        switch (caseName) {
            case "input":
                return "@name('s0') select " +
                    "utildate.roundCeiling('hour') as val0," +
                    "longdate.roundCeiling('hour') as val1," +
                    "caldate.roundCeiling('hour') as val2," +
                    "localdate.roundCeiling('hour') as val3," +
                    "zoneddate.roundCeiling('hour') as val4" +
                    " from SupportDateTime";
            case "ceil":
                return "@name('s0') select " +
                    "utildate.roundCeiling('msec') as val0," +
                    "utildate.roundCeiling('sec') as val1," +
                    "utildate.roundCeiling('minutes') as val2," +
                    "utildate.roundCeiling('hour') as val3," +
                    "utildate.roundCeiling('day') as val4," +
                    "utildate.roundCeiling('month') as val5," +
                    "utildate.roundCeiling('year') as val6" +
                    " from SupportDateTime";
            case "floor":
                return "@name('s0') select " +
                    "utildate.roundFloor('msec') as val0," +
                    "utildate.roundFloor('sec') as val1," +
                    "utildate.roundFloor('minutes') as val2," +
                    "utildate.roundFloor('hour') as val3," +
                    "utildate.roundFloor('day') as val4," +
                    "utildate.roundFloor('month') as val5," +
                    "utildate.roundFloor('year') as val6" +
                    " from SupportDateTime";
            case "half":
                return "@name('s0') select " +
                    "utildate.roundHalf('msec') as val0," +
                    "utildate.roundHalf('sec') as val1," +
                    "utildate.roundHalf('minutes') as val2," +
                    "utildate.roundHalf('hour') as val3," +
                    "utildate.roundHalf('day') as val4," +
                    "utildate.roundHalf('month') as val5," +
                    "utildate.roundHalf('year') as val6" +
                    " from SupportDateTime";
            default:
                throw new IllegalStateException("case " + caseName + " deploys no statements");
        }
    }

    private static void deploy(Configuration config, EPRuntime runtime, String caseName,
                               String statementKey, UpdateListener listener) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler()
            .compile(eplFor(caseName, statementKey), new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        boolean found = false;
        for (EPStatement added : deployment.getStatements()) {
            if ("s0".equals(added.getName())) {
                added.addListener(listener);
                found = true;
            }
        }
        if (!found) {
            throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
        }
    }

    /**
     * Rebuilds the SupportDateTime payload structurally: the scenario carries
     * one ISO instant string and make()-equivalent derivation fills all five
     * representations, including the caldate MILLISECOND=0 forcing.
     */
    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        if (!"SupportDateTime".equals(type)) {
            throw new IllegalStateException("unknown eventType: " + type);
        }
        JsonObject payload = step.get("payload").asObject();
        String datestr = payload.getString("date", null);
        if (datestr == null) {
            throw new IllegalStateException("SupportDateTime payload requires an ISO date string");
        }
        Instant instant = Instant.parse(datestr);
        long millis = instant.toEpochMilli();

        Map<String, Object> event = new LinkedHashMap<>();
        event.put("longdate", Long.valueOf(millis));
        event.put("utildate", new Date(millis));
        Calendar calendar = Calendar.getInstance();
        calendar.setTimeInMillis(millis);
        calendar.set(Calendar.MILLISECOND, 0);
        event.put("caldate", calendar);
        LocalDateTime local = LocalDateTime.ofInstant(instant, ZoneId.systemDefault());
        event.put("localdate", local);
        event.put("zoneddate", local.atZone(ZoneId.systemDefault()));
        runtime.getEventService().sendEventMap(event, "SupportDateTime");
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    /**
     * Canonical cell rendering: instants collapse to epoch-millis numbers so
     * Date/Calendar/LocalDateTime/ZonedDateTime compare numerically across
     * runtimes while preserving the caldate millisecond truncation.
     */
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
        if (value instanceof Date) {
            return Json.value(((Date) value).getTime());
        }
        if (value instanceof Calendar) {
            return Json.value(((Calendar) value).getTimeInMillis());
        }
        if (value instanceof LocalDateTime) {
            return Json.value(((LocalDateTime) value).atZone(ZoneId.systemDefault()).toInstant().toEpochMilli());
        }
        if (value instanceof java.time.ZonedDateTime) {
            return Json.value(((java.time.ZonedDateTime) value).toInstant().toEpochMilli());
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per case, surviving
     * the mid-case redeployment in half; only the new stream is recorded.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private long sequence;

        private ListenerRecorder(String caseName, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", ++sequence);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = new JsonArray();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    newArray.add(renderRow(event));
                }
            }
            record.add("new", newArray);
            records.add(record);
        }
    }
}
