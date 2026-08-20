import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.util.DateTime;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.ZonedDateTime;
import java.util.Arrays;
import java.util.Calendar;
import java.util.Date;
import java.util.HashMap;
import java.util.Map;
import java.util.TimeZone;

/** Direct Esper 9.0.0 oracle for ExprDTBetween's observable comparisons. */
public final class DTBetweenScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String[] CASES = {
            "include-current", "include-constants", "exclude-long", "exclude-util",
            "exclude-cal", "exclude-ldt", "exclude-zdt", "types", "nulls"
    };

    private DTBetweenScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/dt-between.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (scenarioID.isBlank()) {
            throw new IllegalArgumentException("scenario id is required");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        boolean excludesRun = false;
        for (String caseName : CASES) {
            if (!hasCase(steps, caseName)) {
                continue;
            }
            if (caseName.startsWith("exclude-")) {
                if (!excludesRun) {
                    runExcludeCases(steps, records);
                    excludesRun = true;
                }
            } else {
                runCase(steps, caseName, records);
            }
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (JsonValue value : steps) {
            JsonObject step = value.asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DateTime.class);
        addStartEndType(configuration);
        addDateTimeType(configuration);
        if (caseName.startsWith("exclude-")) {
            configuration.getCommon().addVariable("VAR_TRUE", Boolean.class, true);
            configuration.getCommon().addVariable("VAR_FALSE", Boolean.class, false);
        }
        if ("nulls".equals(caseName)) {
            configuration.getCommon().addVariable("VAR_NULL", Boolean.class, null);
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-expr-dt-between-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        StatementHandle handle = null;
        try {
            handle = deploy(runtime, configuration, initialEPL(caseName), records, caseName);
            boolean active = false;
            for (JsonValue value : allSteps) {
                JsonObject step = value.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (op) {
                    case "advance-time":
                        runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "send":
                        send(runtime, step);
                        break;
                    case "deploy":
                        if (!"constants".equals(step.getString("statement", ""))) {
                            throw new IllegalArgumentException("unsupported dt-between deployment marker");
                        }
                        runtime.getDeploymentService().undeploy(handle.deployment.getDeploymentId());
                        handle = deploy(runtime, configuration, constantsEPL(caseName), records, caseName);
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void runExcludeCases(JsonArray allSteps, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DateTime.class);
        addStartEndType(configuration);
        addDateTimeType(configuration);
        configuration.getCommon().addVariable("VAR_TRUE", Boolean.class, true);
        configuration.getCommon().addVariable("VAR_FALSE", Boolean.class, false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-expr-dt-between-exclude", configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            for (String caseName : new String[]{"exclude-long", "exclude-util", "exclude-cal", "exclude-ldt", "exclude-zdt"}) {
                if (!hasCase(allSteps, caseName)) {
                    continue;
                }
                StatementHandle handle = deploy(runtime, configuration, initialEPL(caseName), records, caseName);
                boolean active = false;
                for (JsonValue value : allSteps) {
                    JsonObject step = value.asObject();
                    String op = step.getString("op", "");
                    if ("case".equals(op)) {
                        active = caseName.equals(step.getString("case", ""));
                        continue;
                    }
                    if (!active) {
                        continue;
                    }
                    if ("advance-time".equals(op)) {
                        runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    } else if ("send".equals(op)) {
                        send(runtime, step);
                    } else if ("deploy".equals(op)) {
                        if (!"constants".equals(step.getString("statement", ""))) {
                            throw new IllegalArgumentException("unsupported dt-between deployment marker");
                        }
                        runtime.getDeploymentService().undeploy(handle.deployment.getDeploymentId());
                        handle = deploy(runtime, configuration, constantsEPL(caseName), records, caseName);
                    } else {
                        throw new IllegalArgumentException("unsupported operation " + op);
                    }
                }
                runtime.getDeploymentService().undeployAll();
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void addStartEndType(Configuration configuration) {
        Map<String, Object> type = new HashMap<>();
        type.put("key", String.class);
        type.put("longdateStart", Long.class);
        type.put("utildateStart", Date.class);
        type.put("caldateStart", Calendar.class);
        type.put("ldtStart", LocalDateTime.class);
        type.put("zdtStart", ZonedDateTime.class);
        type.put("longdateEnd", Long.class);
        type.put("utildateEnd", Date.class);
        type.put("caldateEnd", Calendar.class);
        type.put("ldtEnd", LocalDateTime.class);
        type.put("zdtEnd", ZonedDateTime.class);
        configuration.getCommon().addEventType("SupportTimeStartEndA", type);
    }

    private static void addDateTimeType(Configuration configuration) {
        Map<String, Object> type = new HashMap<>();
        type.put("longdate", Long.class);
        type.put("utildate", Date.class);
        type.put("caldate", Calendar.class);
        type.put("localdate", LocalDateTime.class);
        type.put("zoneddate", ZonedDateTime.class);
        type.put("longPrimitive", long.class);
        type.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportDateTime", type);

        Map<String, Object> bean = new HashMap<>();
        bean.put("longPrimitive", long.class);
        bean.put("longBoxed", Long.class);
        configuration.getCommon().addEventType("SupportBean", bean);
    }

    private static String initialEPL(String caseName) {
        if ("include-current".equals(caseName)) {
            return "@name('s0') select " +
                    "current_timestamp.after(longdateStart) as val0, " +
                    "current_timestamp.between(longdateStart, longdateEnd) as val1, " +
                    "current_timestamp.between(utildateStart, caldateEnd) as val2, " +
                    "current_timestamp.between(caldateStart, utildateEnd) as val3, " +
                    "current_timestamp.between(utildateStart, utildateEnd) as val4, " +
                    "current_timestamp.between(caldateStart, caldateEnd) as val5, " +
                    "current_timestamp.between(caldateEnd, caldateStart) as val6, " +
                    "current_timestamp.between(ldtStart, ldtEnd) as val7, " +
                    "current_timestamp.between(zdtStart, zdtEnd) as val8 " +
                    "from SupportTimeStartEndA";
        }
        if ("include-constants".equals(caseName)) {
            return constantsEPL(caseName);
        }
        if (caseName.startsWith("exclude-")) {
            return excludeEPL(excludeField(caseName));
        }
        if ("types".equals(caseName)) {
            return "@name('s0') select " +
                    "dt.longdate.between(bean.longPrimitive, bean.longBoxed) as c0, " +
                    "dt.utildate.between(bean.longPrimitive, bean.longBoxed) as c1, " +
                    "dt.caldate.between(bean.longPrimitive, bean.longBoxed) as c2, " +
                    "dt.localdate.between(bean.longPrimitive, bean.longBoxed) as c3, " +
                    "dt.zoneddate.between(bean.longPrimitive, bean.longBoxed) as c4 " +
                    "from SupportDateTime as dt unidirectional, SupportBean#lastevent as bean";
        }
        if ("nulls".equals(caseName)) {
            return "@name('s0') select " +
                    "current_timestamp.between(longdateStart, longdateEnd, VAR_NULL, true) as val0, " +
                    "current_timestamp.between(longdateStart, longdateEnd) as val1 " +
                    "from SupportTimeStartEndA";
        }
        throw new IllegalArgumentException("unsupported case " + caseName);
    }

    private static String constantsEPL(String ignoredCaseName) {
        if (ignoredCaseName.startsWith("exclude-")) {
            String field = "longdateStart";
            return "@name('s0') select " +
                    field + ".between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), true, true) as val0, " +
                    field + ".between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), true, false) as val1, " +
                    field + ".between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), false, true) as val2, " +
                    field + ".between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), false, false) as val3 " +
                    "from SupportTimeStartEndA";
        }
        return "@name('s0') select " +
                "longdateStart.between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val0, " +
                "utildateStart.between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val1, " +
                "caldateStart.between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val2, " +
                "ldtStart.between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val3, " +
                "zdtStart.between(DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val4, " +
                "longdateStart.between(DateTime.toCalendar('2002-05-30T09:01:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\"), DateTime.toCalendar('2002-05-30T09:00:00.000', \"yyyy-MM-dd'T'HH:mm:ss.SSS\")) as val5 " +
                "from SupportTimeStartEndA";
    }

    private static String excludeEPL(String field) {
        return "@name('s0') select " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", true, true) as val0, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", true, false) as val1, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", false, true) as val2, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", false, false) as val3, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", VAR_TRUE, VAR_TRUE) as val4, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", VAR_TRUE, VAR_FALSE) as val5, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", VAR_FALSE, VAR_TRUE) as val6, " +
                "current_timestamp.between(" + field + ", " + endField(field) + ", VAR_FALSE, VAR_FALSE) as val7 " +
                "from SupportTimeStartEndA";
    }

    private static String excludeField(String caseName) {
        switch (caseName) {
            case "exclude-long":
                return "longdateStart";
            case "exclude-util":
                return "utildateStart";
            case "exclude-cal":
                return "caldateStart";
            case "exclude-ldt":
                return "ldtStart";
            case "exclude-zdt":
                return "zdtStart";
            default:
                throw new IllegalArgumentException("unsupported endpoint type " + caseName);
        }
    }

    private static String endField(String startField) {
        return startField.substring(0, startField.length() - "Start".length()) + "End";
    }

    private static StatementHandle deploy(EPRuntime runtime, Configuration configuration, String epl, JsonArray records, String caseName) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                new CompilerArguments(configuration));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        EPStatement statement = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                statement = candidate;
                break;
            }
        }
        if (statement == null) {
            throw new IllegalStateException("statement s0 was not deployed");
        }
        statement.addListener(new TraceWriter(records, caseName, statement, runtime));
        return new StatementHandle(deployment, statement);
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportTimeStartEndA".equals(eventType)) {
            JsonValue startValue = payload.get("start");
            if (startValue == null) {
                Map<String, Object> event = new HashMap<>();
                event.put("key", payload.getString("key", null));
                runtime.getEventService().sendEventMap(event, eventType);
                return;
            }
            if (startValue.isNull()) {
                Map<String, Object> event = new HashMap<>();
                event.put("key", payload.getString("key", null));
                putNullTime(event, "Start");
                putNullTime(event, "End");
                runtime.getEventService().sendEventMap(event, eventType);
                return;
            }
            Instant start = Instant.parse(startValue.asString());
            long startMillis = start.toEpochMilli();
            long endMillis = startMillis + payload.getLong("duration", 0L);
            Map<String, Object> event = new HashMap<>();
            event.put("key", payload.getString("key", null));
            putTime(event, "longdateStart", startMillis);
            putTime(event, "longdateEnd", endMillis);
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportDateTime".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            JsonValue dateValue = payload.get("date");
            if (dateValue == null) {
                // Leave date-time properties absent so the event adapter exposes Missing.
            } else if (dateValue.isNull()) {
                putNullTime(event, "");
            } else {
                Instant instant = Instant.parse(dateValue.asString());
                putTime(event, "longdate", instant.toEpochMilli());
            }
            event.put("longPrimitive", payload.getLong("longPrimitive", 0L));
            JsonValue boxedValue = payload.get("longBoxed");
            event.put("longBoxed", boxedValue == null || boxedValue.isNull() ? null : boxedValue.asLong());
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("longPrimitive", payload.getLong("longPrimitive", 0L));
            JsonValue boxedValue = payload.get("longBoxed");
            event.put("longBoxed", boxedValue == null || boxedValue.isNull() ? null : boxedValue.asLong());
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        throw new IllegalArgumentException("unsupported event type " + eventType);
    }

    private static void putTime(Map<String, Object> event, String prefix, long millis) {
        event.put(prefix, millis);
        String suffix = prefix.endsWith("Start") ? "Start" : prefix.endsWith("End") ? "End" : "";
        if (suffix.isEmpty()) {
            event.put("utildate", new Date(millis));
            event.put("caldate", calendar(millis));
            LocalDateTime local = LocalDateTime.ofInstant(Instant.ofEpochMilli(millis), ZoneOffset.UTC);
            event.put("localdate", local);
            event.put("zoneddate", local.atZone(ZoneOffset.UTC));
            return;
        }
        event.put("utildate" + suffix, new Date(millis));
        event.put("caldate" + suffix, calendar(millis));
        LocalDateTime local = LocalDateTime.ofInstant(Instant.ofEpochMilli(millis), ZoneOffset.UTC);
        event.put("ldt" + suffix, local);
        event.put("zdt" + suffix, local.atZone(ZoneOffset.UTC));
    }

    private static void putNullTime(Map<String, Object> event, String suffix) {
        if (suffix.isEmpty()) {
            event.put("longdate", null);
            event.put("utildate", null);
            event.put("caldate", null);
            event.put("localdate", null);
            event.put("zoneddate", null);
            return;
        }
        event.put("longdate" + suffix, null);
        event.put("utildate" + suffix, null);
        event.put("caldate" + suffix, null);
        event.put("ldt" + suffix, null);
        event.put("zdt" + suffix, null);
    }

    private static Calendar calendar(long millis) {
        Calendar calendar = Calendar.getInstance(TimeZone.getTimeZone("UTC"));
        calendar.setTimeInMillis(millis);
        return calendar;
    }

    private static final class StatementHandle {
        private final EPDeployment deployment;
        @SuppressWarnings("unused")
        private final EPStatement statement;

        private StatementHandle(EPDeployment deployment, EPStatement statement) {
            this.deployment = deployment;
            this.statement = statement;
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
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
}
