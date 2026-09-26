import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.DateTime;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportDateTime;
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
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Calendar;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Java oracle for the four ExprDTWithMax/ExprDTWithMin executions (Draft
 * 4.549).  Four cases replay on fresh runtimes; each deploys the byte-exact
 * s0 select over the SupportDateTime bean, pins the assertStmtTypes surface
 * through a types record, sends one SupportDateTime.make() bean and verifies
 * the delivered row against the pinned expected columns like
 * assertPropsNew, then undeployAll:
 *
 * with-max-input (ordinal 0, ExprDTWithMaxInput): withMax('month') applied
 * to all five representations of 2002-05-30T09:00:00.000 emits
 * 2002-12-30T09:00:00.000 in val0..val4; the types step pins the asserted
 * DATE/LONGBOXED/CALENDAR/LOCALDATETIME/ZONEDDATETIME types.
 *
 * with-max-fields (ordinal 1, ExprDTWithMaxFields): withMax over
 * msec,sec,minutes,hour,day,month,year,week on utildate.  'week' clamps
 * WEEK_OF_YEAR keeping the day-of-week (Thursday stays -> 2002-12-26);
 * 'year' reaches GregorianCalendar's actual maximum year 292278994 whose
 * epoch-millis overflows int64 to the wrapped 9223372030035600000, pinned
 * verbatim through Date.getTime() like the regression assertion's
 * getArrayCoerced("util") comparison.
 *
 * with-min-input (ordinal 0, ExprDTWithMinInput): withMin('month') emits
 * 2002-01-30T09:00:00.000 in val0..val4.
 *
 * with-min-fields (ordinal 1, ExprDTWithMinFields): withMin over the same
 * eight units at 2002-05-30T09:01:02.003; 'year' reaches year 1 and 'week'
 * clamps to week 1 keeping Thursday (2002-01-03).
 *
 * Instant-valued cells render as epoch-millis numbers on both sides, the
 * ExprDTRound/ExprDTDataSources instant-token convention.
 */
public final class ExprDTWithMinMax549ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-with-minmax-549";
    private static final String DESCRIPTION = "ExprDTWithMax/ExprDTWithMin executions: "
            + "with-max-input replays ExprDTWithMaxInput (withMax('month') over all five "
            + "SupportDateTime representations of 2002-05-30T09:00:00.000 emits "
            + "2002-12-30T09:00:00.000 in val0..val4), with-max-fields replays "
            + "ExprDTWithMaxFields (withMax over msec,sec,minutes,hour,day,month,year,week "
            + "on utildate: 'week' keeps the day-of-week -> 2002-12-26, 'year' wraps "
            + "epoch-millis to 9223372030035600000), with-min-input replays "
            + "ExprDTWithMinInput (withMin('month') emits 2002-01-30T09:00:00.000), "
            + "with-min-fields replays ExprDTWithMinFields (withMin at "
            + "2002-05-30T09:01:02.003: 'year' -> 0001-05-30 epoch-millis -62122863537997, "
            + "'week' -> 2002-01-03).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-097a5db3742ae25c3b0b",
            "java-runtime-c6bba43af400a7375075",
            "java-runtime-3f79b4bf903cca74fe6a",
            "java-runtime-8ca773479bf038c94597"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTWithMaxInput",
            "ExprDTWithMaxFields",
            "ExprDTWithMinInput",
            "ExprDTWithMinFields"
    };
    private static final String[] STATIC_IDS = {
            "java-d4afe3864ba41bfd7918",
            "java-d87f60ced41e2a52719f",
            "java-2224592eef4aca9b4e7b",
            "java-f3bf884f776301d91900"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "with-max-input",
            "with-max-fields",
            "with-min-input",
            "with-min-fields"
    };
    private static final int[] ORDINALS = {0, 1, 0, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+types; deploy s0 withMax('month') over all five representations, "
            + "types pins {Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send "
            + "make(2002-05-30T09:00:00.000) emits 2002-12-30T09:00:00.000 epoch-millis "
            + "in val0..val4",
            "listener+types; deploy s0 withMax over msec,sec,minutes,hour,day,month,year,"
            + "week on utildate, types pins all-Date, send make(2002-05-30T09:00:00.000) "
            + "emits {...999, ...59.000, ...59:00, ...23:00, 2002-05-31, 2002-12-30, "
            + "wrapped-year 9223372030035600000, 2002-12-26}",
            "listener+types; deploy s0 withMin('month') over all five representations, "
            + "types pins {Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send "
            + "make(2002-05-30T09:00:00.000) emits 2002-01-30T09:00:00.000 epoch-millis "
            + "in val0..val4",
            "listener+types; deploy s0 withMin over msec,sec,minutes,hour,day,month,year,"
            + "week on utildate, types pins all-Date, send make(2002-05-30T09:01:02.003) "
            + "emits {...02.000, ...00.003, ...00:02.003, 00:01:02.003, 2002-05-01, "
            + "2002-01-30, year-1 -62122863537997, 2002-01-03}"
    };

    private static final String MAX_INPUT_EPL =
            "@name('s0') select "
            + "utildate.withMax('month') as val0,"
            + "longdate.withMax('month') as val1,"
            + "caldate.withMax('month') as val2,"
            + "localdate.withMax('month') as val3,"
            + "zoneddate.withMax('month') as val4"
            + " from SupportDateTime";
    private static final String MIN_INPUT_EPL =
            "@name('s0') select "
            + "utildate.withMin('month') as val0,"
            + "longdate.withMin('month') as val1,"
            + "caldate.withMin('month') as val2,"
            + "localdate.withMin('month') as val3,"
            + "zoneddate.withMin('month') as val4"
            + " from SupportDateTime";
    private static final String MAX_FIELDS_EPL =
            "@name('s0') select "
            + "utildate.withMax('msec') as val0,"
            + "utildate.withMax('sec') as val1,"
            + "utildate.withMax('minutes') as val2,"
            + "utildate.withMax('hour') as val3,"
            + "utildate.withMax('day') as val4,"
            + "utildate.withMax('month') as val5,"
            + "utildate.withMax('year') as val6,"
            + "utildate.withMax('week') as val7"
            + " from SupportDateTime";
    private static final String MIN_FIELDS_EPL =
            "@name('s0') select "
            + "utildate.withMin('msec') as val0,"
            + "utildate.withMin('sec') as val1,"
            + "utildate.withMin('minutes') as val2,"
            + "utildate.withMin('hour') as val3,"
            + "utildate.withMin('day') as val4,"
            + "utildate.withMin('month') as val5,"
            + "utildate.withMin('year') as val6,"
            + "utildate.withMin('week') as val7"
            + " from SupportDateTime";

    private static final String[] CASE_EPLS = {
            MAX_INPUT_EPL,
            MAX_FIELDS_EPL,
            MIN_INPUT_EPL,
            MIN_FIELDS_EPL
    };

    private static String eplFor(String caseName) {
        switch (caseName) {
            case "with-max-input":
                return MAX_INPUT_EPL;
            case "with-max-fields":
                return MAX_FIELDS_EPL;
            case "with-min-input":
                return MIN_INPUT_EPL;
            case "with-min-fields":
                return MIN_FIELDS_EPL;
            default:
                throw new IllegalStateException("case " + caseName + " deploys no statements");
        }
    }

    /** Java-asserted property types per case (assertStmtTypes simple names). */
    private static final Map<String, Map<String, String>> TYPE_PROPERTIES = new HashMap<>();
    static {
        Map<String, String> input = new HashMap<>();
        input.put("val0", "Date");
        input.put("val1", "Long");
        input.put("val2", "Calendar");
        input.put("val3", "LocalDateTime");
        input.put("val4", "ZonedDateTime");
        Map<String, String> fields = new HashMap<>();
        for (int index = 0; index < 8; index++) {
            fields.put("val" + index, "Date");
        }
        TYPE_PROPERTIES.put("with-max-input", input);
        TYPE_PROPERTIES.put("with-max-fields", fields);
        TYPE_PROPERTIES.put("with-min-input", input);
        TYPE_PROPERTIES.put("with-min-fields", fields);
    }

    private static final String DATE_T00 = "2002-05-30T09:00:00.000Z";
    private static final String DATE_T03 = "2002-05-30T09:01:02.003Z";

    /** Pinned assertPropsNew epoch-millis values per case, in select order. */
    private static final Map<String, long[]> EXPECTED_MILLIS = new HashMap<>();
    static {
        EXPECTED_MILLIS.put("with-max-input", new long[]{
                1041238800000L, 1041238800000L, 1041238800000L, 1041238800000L, 1041238800000L});
        EXPECTED_MILLIS.put("with-max-fields", new long[]{
                1022749200999L, 1022749259000L, 1022752740000L, 1022799600000L,
                1022835600000L, 1041238800000L, 9223372030035600000L, 1040893200000L});
        EXPECTED_MILLIS.put("with-min-input", new long[]{
                1012381200000L, 1012381200000L, 1012381200000L, 1012381200000L, 1012381200000L});
        EXPECTED_MILLIS.put("with-min-fields", new long[]{
                1022749262000L, 1022749260003L, 1022749202003L, 1022716862003L,
                1020243662003L, 1012381262003L, -62122863537997L, 1010048462003L});
    }

    private static final Map<String, String> SEND_DATES = new HashMap<>();
    static {
        SEND_DATES.put("with-max-input", DATE_T00);
        SEND_DATES.put("with-max-fields", DATE_T00);
        SEND_DATES.put("with-min-input", DATE_T00);
        SEND_DATES.put("with-min-fields", DATE_T03);
    }

    private static final int EXPECTED_STEPS = 20;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|at.  Deploy steps
     * carry the byte-exact EPL text; send steps carry the compacted payload
     * including the pinned expected epoch-millis values.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        for (String caseName : CASES) {
            StringBuilder expected = new StringBuilder();
            long[] values = EXPECTED_MILLIS.get(caseName);
            for (int index = 0; index < values.length; index++) {
                if (index > 0) {
                    expected.append(",");
                }
                expected.append(values[index]);
            }
            String payload = "{\"date\":\"" + SEND_DATES.get(caseName)
                    + "\",\"expected\":[" + expected + "]}";
            CASE_STEPS.put(caseName, new String[]{
                    "deploy|" + caseName + "|s0||" + eplFor(caseName) + "|||",
                    "types|" + caseName + "|s0|||||",
                    "send|" + caseName + "||SupportDateTime||" + payload + "||",
                    "undeploy-all|" + caseName + "||||||",
            });
        }
    }

    private ExprDTWithMinMax549ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTWithMinMax549ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        rejectDuplicateKeys(scenario);
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, scenario.get("steps").asArray(), records);
        }
        if (records.size() != 8) {
            throw new IllegalStateException("expected 8 records, got " + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays the case's steps on a fresh runtime: deploy s0, verify the
     * pinned property-type surface, send one SupportDateTime bean and
     * undeployAll, mirroring the execution's compileDeploy -> assertStmtTypes
     * -> sendEventBean -> assertPropsNew -> undeployAll cycle.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = configure();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            ListenerRecorder listener = new ListenerRecorder();
            Map<String, EPStatement> statements = new HashMap<>();
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, listener, statements);
                        break;
                    case "types":
                        typesStep(caseName, step, statements, records, runtime);
                        break;
                    case "send":
                        sendEvent(runtime, listener, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        listener.reset();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /** SupportDateTime bean registration mirroring the suite. */
    private static Configuration configure() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportDateTime.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    /**
     * Compiles and deploys the pinned s0 select, attaching the listener,
     * mirroring env.compileDeploy(epl).addListener("s0").
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration, String caseName,
                                   JsonObject step, ListenerRecorder listener,
                                   Map<String, EPStatement> statements) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        if (!"s0".equals(label) || !eplFor(caseName).equals(epl)) {
            throw new IllegalStateException("case " + caseName + " deploy " + label
                    + " carries an unpinned EPL");
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(epl, new CompilerArguments(configuration));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                statements.put(label, statement);
                statement.addListener(listener);
            }
        }
        if (!statements.containsKey(label)) {
            throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
        }
    }

    /**
     * Emits a {"operation":"types"} record carrying the Java-asserted
     * property types after verifying them, mirroring assertStmtTypes /
     * assertStmtTypesAllSame: the input cases pin
     * Date/Long/Calendar/LocalDateTime/ZonedDateTime, the fields cases pin
     * all-Date.
     */
    private static void typesStep(String caseName, JsonObject step,
                                  Map<String, EPStatement> statements, JsonArray records,
                                  EPRuntime runtime) {
        String label = string(step, "statement");
        Map<String, String> pinned = TYPE_PROPERTIES.get(caseName);
        EPStatement statement = statements.get(label);
        if (pinned == null || statement == null) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        EventType eventType = statement.getEventType();
        JsonObject properties = new JsonObject();
        for (Map.Entry<String, String> entry : new TreeMap<>(pinned).entrySet()) {
            Class<?> propertyType = eventType.getPropertyType(entry.getKey());
            String actual = propertyType == null ? "null" : propertyType.getSimpleName();
            if (!entry.getValue().equals(actual)) {
                throw new IllegalStateException("property type drift for s0." + entry.getKey()
                        + ": expected " + entry.getValue() + " got " + actual);
            }
            properties.add(entry.getKey(), entry.getValue());
        }
        JsonObject value = new JsonObject();
        value.add("properties", properties);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    /**
     * Sends one SupportDateTime.make() bean, drains the delivery it
     * produced, and emits exactly one listener record after verifying the
     * row against the pinned expected epoch-millis values — the
     * assertPropsNew equivalent.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        if (!"SupportDateTime".equals(eventType)) {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        String date = string(payload, "date");
        if (!SEND_DATES.get(caseName).equals(date)) {
            throw new IllegalStateException("case " + caseName + " send date " + date
                    + " is not pinned");
        }
        runtime.getEventService().sendEventBean(
                SupportDateTime.make(javaDateString(date)), "SupportDateTime");
        List<JsonObject> delivered = listener.drain();
        if (delivered.size() != 1) {
            throw new IllegalStateException("expected one delivery for " + eventType
                    + ", got " + delivered.size());
        }
        JsonObject row = delivered.get(0);
        verifyExpected(caseName, eventType, payload, row);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "listener");
        record.add("statement", "s0");
        record.add("sequence", listener.nextSequence());
        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        JsonArray newArray = new JsonArray();
        newArray.add(row);
        record.add("new", newArray);
        records.add(record);
    }

    /**
     * Verifies the delivered row's fields against the payload's pinned
     * expected epoch-millis values in select order, mirroring assertPropsNew.
     */
    private static void verifyExpected(String caseName, String eventType, JsonObject payload,
                                       JsonObject row) {
        JsonArray expected = array(payload.get("expected"), "expected");
        int columns = caseName.endsWith("-input") ? 5 : 8;
        if (expected.size() != columns) {
            throw new IllegalStateException("expected " + columns + " values for "
                    + eventType + ", got " + expected.size());
        }
        long[] pinned = EXPECTED_MILLIS.get(caseName);
        JsonObject fields = object(row.get("fields"), "fields");
        for (int index = 0; index < columns; index++) {
            if (expected.get(index).asLong() != pinned[index]) {
                throw new IllegalStateException("send expected[" + index + "] for " + caseName
                        + " is not pinned");
            }
            String column = "val" + index;
            JsonValue want = expected.get(index);
            JsonValue got = fields.get(column);
            if (!jsonEquals(want, got)) {
                throw new IllegalStateException("observed " + column + " drift for "
                        + eventType + ": expected " + want + " got " + got);
            }
        }
    }

    /** Compares two normalized JSON cells: numbers numerically, else literally. */
    private static boolean jsonEquals(JsonValue want, JsonValue got) {
        if (want == null || got == null) {
            return want == got;
        }
        if (want.isNumber() && got.isNumber()) {
            return want.asLong() == got.asLong();
        }
        return want.equals(got);
    }

    /** Renders an ISO instant as the local-format string SupportDateTime.make parses. */
    private static String javaDateString(String iso) {
        Instant instant = Instant.parse(iso);
        return LocalDateTime.ofInstant(instant, ZoneId.of("UTC"))
                .format(DateTimeFormatter.ofPattern(DateTime.DEFAULT_XMLLIKE_DATE_FORMAT));
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

    /** Canonical cell rendering: instants collapse to epoch-millis numbers. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof java.util.Date) {
            return Json.value(((java.util.Date) value).getTime());
        }
        if (value instanceof java.util.Calendar) {
            return Json.value(((java.util.Calendar) value).getTimeInMillis());
        }
        if (value instanceof java.time.LocalDateTime) {
            java.time.LocalDateTime ldt = (java.time.LocalDateTime) value;
            return Json.value(ldt.atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli());
        }
        if (value instanceof java.time.ZonedDateTime) {
            return Json.value(((java.time.ZonedDateTime) value).toInstant().toEpochMilli());
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Buffers rendered rows per delivery so the send step can verify the
     * pinned expected values before emitting the listener record.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final List<JsonObject> pending = new ArrayList<>();
        private long sequence;

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null || newEvents.length == 0) {
                return;
            }
            for (EventBean event : newEvents) {
                pending.add(renderRow(event));
            }
        }

        private List<JsonObject> drain() {
            List<JsonObject> rows = new ArrayList<>(pending);
            pending.clear();
            return rows;
        }

        private long nextSequence() {
            return ++sequence;
        }

        private void reset() {
            pending.clear();
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|at with the payload
     * compacted.  Unknown fields are rejected per op.
     */
    private static String stepKey(JsonObject step) {
        String operation = string(step, "op");
        Map<String, Set<String>> allowed = new HashMap<>();
        allowed.put("case", new HashSet<>(Arrays.asList("op", "case")));
        allowed.put("deploy", new HashSet<>(Arrays.asList("op", "case", "statement", "epl")));
        allowed.put("types", new HashSet<>(Arrays.asList("op", "case", "statement")));
        allowed.put("send", new HashSet<>(Arrays.asList("op", "case", "eventType", "payload")));
        allowed.put("undeploy-all", new HashSet<>(Arrays.asList("op", "case")));
        Set<String> fields = allowed.get(operation);
        if (fields == null) {
            throw new IllegalArgumentException("step has unsupported op " + operation);
        }
        for (String field : step.names()) {
            if (!fields.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        return operation + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + string(step, "at");
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
