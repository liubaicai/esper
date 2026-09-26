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
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Java oracle for the two ExprDTFormat executions (Draft 4.551).  Two
 * cases replay on fresh runtimes; each advances the clock to
 * 2002-05-30T09:00:00.000, deploys the byte-exact s0 select over the
 * SupportDateTime bean, pins the assertStmtTypesAllSame surface through a
 * types record, sends a populated SupportDateTime.make() bean and the
 * all-null bean, verifies each delivered row against the pinned expected
 * columns like assertPropsNew, then undeployAll:
 *
 * format-simple (ordinal 0, ExprDTFormatSimple): no-arg format() renders
 * current_timestamp and the Date/long/Calendar representations through a
 * new SimpleDateFormat() (the en_US-pinned "5/30/02, 9:00 AM"), localdate
 * through DateTimeFormatter.ISO_DATE_TIME ("2002-05-30T09:00:00"), and
 * zoneddate through DateTimeFormatter.ISO_ZONED_DATE_TIME
 * ("2002-05-30T09:00:00Z[UTC]").  The all-null send keeps
 * current_timestamp.format() non-null while val1..val5 go null.
 *
 * format-wstring (ordinal 1, ExprDTFormatWString): the pattern
 * "yyyy.MM.dd G 'at' HH:mm:ss" renders "2002.05.30 AD at 09:00:00" across
 * all five representations, SimpleDateFormat.getDateInstance() renders
 * "May 30, 2002", and DateTimeFormatter.BASIC_ISO_DATE renders "20020530";
 * the all-null send emits null in every column.
 *
 * String cells render verbatim and null cells render as the tagged
 * {"state":"null"} object on both sides.
 */
public final class ExprDTFormat551ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-format-551";
    private static final String DESCRIPTION = "ExprDTFormat executions: "
            + "format-simple replays ExprDTFormatSimple (no-arg format() over "
            + "current_timestamp and all five SupportDateTime representations of "
            + "2002-05-30T09:00:00.000: the en_US SimpleDateFormat default "
            + "'5/30/02, 9:00 AM' for the legacy representations, ISO_DATE_TIME "
            + "'2002-05-30T09:00:00' and ISO_ZONED_DATE_TIME "
            + "'2002-05-30T09:00:00Z[UTC]' for the Java 8 representations; the "
            + "all-null send keeps val0 non-null), format-wstring replays "
            + "ExprDTFormatWString (the pattern \"yyyy.MM.dd G 'at' HH:mm:ss\" "
            + "renders '2002.05.30 AD at 09:00:00' across all five "
            + "representations, SimpleDateFormat.getDateInstance() renders "
            + "'May 30, 2002', DateTimeFormatter.BASIC_ISO_DATE renders "
            + "'20020530'; the all-null send emits null in every column).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/"
                    + "ExprDTFormat.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-2832950bf0c454ecf2df",
            "java-runtime-8a27995fa8e41684f6b8"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTFormatSimple",
            "ExprDTFormatWString"
    };
    private static final String[] STATIC_IDS = {
            "java-05592d8076a91dd5cbed",
            "java-05592d8076a91dd5cbed"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "format-simple",
            "format-wstring"
    };
    private static final int[] ORDINALS = {0, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 "
            + "no-arg format() over current_timestamp and all five representations, "
            + "types pins all-String, send make(2002-05-30T09:00:00.000) emits "
            + "{5/30/02, 9:00 AM x4, 2002-05-30T09:00:00, 2002-05-30T09:00:00Z[UTC]}, "
            + "send make(null) emits {5/30/02, 9:00 AM, null x5}",
            "listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 "
            + "pattern/formatter-object format() over all five representations, "
            + "types pins all-String, send make(2002-05-30T09:00:00.000) emits "
            + "{2002.05.30 AD at 09:00:00 x5, May 30, 2002, 20020530}, "
            + "send make(null) emits {null x7}"
    };

    private static final String FORMAT_SIMPLE_EPL =
            "@name('s0') select "
            + "current_timestamp.format() as val0,"
            + "utildate.format() as val1,"
            + "longdate.format() as val2,"
            + "caldate.format() as val3,"
            + "localdate.format() as val4,"
            + "zoneddate.format() as val5"
            + " from SupportDateTime";
    private static final String FORMAT_WSTRING_EPL =
            "@name('s0') select "
            + "longdate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val0,"
            + "utildate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val1,"
            + "caldate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val2,"
            + "localdate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val3,"
            + "zoneddate.format(\"yyyy.MM.dd G 'at' HH:mm:ss\") as val4,"
            + "utildate.format(SimpleDateFormat.getDateInstance()) as val5,"
            + "localdate.format(java.time.format.DateTimeFormatter.BASIC_ISO_DATE) as val6"
            + " from SupportDateTime";

    private static final String[] CASE_EPLS = {
            FORMAT_SIMPLE_EPL,
            FORMAT_WSTRING_EPL
    };

    /** Per-case pinned s0 deploy (one each); consulted by deployStep. */
    private static final Map<String, String> DEPLOY_EPLS = new HashMap<>();
    static {
        DEPLOY_EPLS.put("format-simple", FORMAT_SIMPLE_EPL);
        DEPLOY_EPLS.put("format-wstring", FORMAT_WSTRING_EPL);
    }

    /** Java-asserted property types per case (assertStmtTypesAllSame: String). */
    private static final Map<String, Map<String, String>> TYPE_PROPERTIES = new HashMap<>();
    static {
        Map<String, String> simple = new HashMap<>();
        for (int index = 0; index < 6; index++) {
            simple.put("val" + index, "String");
        }
        Map<String, String> wstring = new HashMap<>();
        for (int index = 0; index < 7; index++) {
            wstring.put("val" + index, "String");
        }
        TYPE_PROPERTIES.put("format-simple", simple);
        TYPE_PROPERTIES.put("format-wstring", wstring);
    }

    private static final String DATE_T00 = "2002-05-30T09:00:00.000Z";

    /**
     * Pinned assertPropsNew string cells per case send, in select order;
     * a null cell renders as the tagged {"state":"null"} object in the
     * trace protocol.  format-simple send 2 is the all-null bean:
     * current_timestamp.format() still renders the advanced clock.
     */
    private static final Map<String, String[][]> EXPECTED = new HashMap<>();
    static {
        String sdf = "5/30/02, 9:00 AM";
        EXPECTED.put("format-simple", new String[][]{
                {sdf, sdf, sdf, sdf, "2002-05-30T09:00:00", "2002-05-30T09:00:00Z[UTC]"},
                {sdf, null, null, null, null, null}});
        String pattern = "2002.05.30 AD at 09:00:00";
        EXPECTED.put("format-wstring", new String[][]{
                {pattern, pattern, pattern, pattern, pattern,
                        "May 30, 2002", "20020530"},
                {null, null, null, null, null, null, null}});
    }

    /** Send dates per case: the populated send pins DATE_T00, the null send pins JSON null. */
    private static final Map<String, String> SEND_DATES = new HashMap<>();
    static {
        SEND_DATES.put("format-simple", DATE_T00);
        SEND_DATES.put("format-wstring", DATE_T00);
    }

    private static final int EXPECTED_STEPS = 14;

    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        for (String caseName : CASES) {
            String[][] expected = EXPECTED.get(caseName);
            List<String> pinned = new ArrayList<>();
            pinned.add("advance-time|" + caseName + "||||||" + DATE_T00);
            pinned.add("deploy|" + caseName + "|s0||" + DEPLOY_EPLS.get(caseName) + "|||");
            pinned.add("types|" + caseName + "|s0|||||");
            for (int sendIndex = 0; sendIndex < expected.length; sendIndex++) {
                String[] row = expected[sendIndex];
                boolean nullSend = sendIndex == expected.length - 1;
                StringBuilder cells = new StringBuilder();
                for (int index = 0; index < row.length; index++) {
                    if (index > 0) {
                        cells.append(",");
                    }
                    cells.append(row[index] == null ? "{\"state\":\"null\"}"
                            : Json.value(row[index]).toString());
                }
                String payload = "{\"date\":" + (nullSend ? "null" : "\"" + DATE_T00 + "\"")
                        + ",\"expected\":[" + cells + "]}";
                pinned.add("send|" + caseName + "||SupportDateTime||" + payload + "||");
            }
            pinned.add("undeploy-all|" + caseName + "||||||");
            CASE_STEPS.put(caseName, pinned.toArray(new String[0]));
        }
    }

    private ExprDTFormat551ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTFormat551ScenarioOracle <scenario.json>");
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
        if (records.size() != 6) {
            throw new IllegalStateException("expected 6 records, got " + records.size());
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
     * Replays the case's steps on a fresh runtime: advanceTime to the
     * pinned instant, deploy s0, verify the pinned property-type surface,
     * send the populated and all-null SupportDateTime beans, then
     * undeployAll — mirroring the execution's advanceTime ->
     * compileDeploy -> assertStmtTypesAllSame -> sendEventBean ->
     * assertPropsNew -> undeployAll cycle.
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
            int sendIndex = 0;
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
                    case "advance-time": {
                        long atMillis = Instant.parse(string(step, "at")).toEpochMilli();
                        runtime.getEventService().advanceTime(atMillis);
                        break;
                    }
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, listener, statements);
                        break;
                    case "types":
                        typesStep(caseName, step, statements, records, runtime);
                        break;
                    case "send":
                        sendIndex++;
                        sendEvent(runtime, listener, caseName, sendIndex, step, records);
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
        String pinned = DEPLOY_EPLS.get(caseName);
        if (pinned == null || !"s0".equals(label) || !pinned.equals(epl)) {
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
     * property types after verifying them, mirroring
     * assertStmtTypesAllSame: every column is String-typed in both cases.
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
     * Sends one SupportDateTime.make() bean — the populated send parses
     * the pinned instant, the null send mirrors make(null) — drains the
     * delivery it produced, and emits exactly one listener record after
     * verifying the row against the pinned expected rendered strings —
     * the assertPropsNew equivalent.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  int sendIndex, JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        if (!"SupportDateTime".equals(eventType)) {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        JsonValue dateValue = payload.get("date");
        SupportDateTime bean;
        if (dateValue == null || dateValue.isNull()) {
            String[] pinned = EXPECTED.get(caseName)[sendIndex - 1];
            boolean allNull = true;
            for (int index = 1; index < pinned.length; index++) {
                allNull &= pinned[index] == null;
            }
            if (!allNull) {
                throw new IllegalStateException("case " + caseName
                        + " null send is not pinned at index " + sendIndex);
            }
            bean = SupportDateTime.make(null);
        } else {
            String date = dateValue.asString();
            if (!SEND_DATES.get(caseName).equals(date)) {
                throw new IllegalStateException("case " + caseName + " send date " + date
                        + " is not pinned");
            }
            bean = SupportDateTime.make(javaDateString(date));
        }
        runtime.getEventService().sendEventBean(bean, "SupportDateTime");
        List<JsonObject> delivered = listener.drain();
        if (delivered.size() != 1) {
            throw new IllegalStateException("expected one delivery for " + eventType
                    + ", got " + delivered.size());
        }
        JsonObject row = delivered.get(0);
        verifyExpected(caseName, eventType, sendIndex, payload, row);
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
     * expected rendered strings in select order, mirroring assertPropsNew.
     * Null cells compare through the tagged {"state":"null"} object.
     */
    private static void verifyExpected(String caseName, String eventType, int sendIndex,
                                       JsonObject payload, JsonObject row) {
        JsonArray expected = array(payload.get("expected"), "expected");
        String[] pinned = EXPECTED.get(caseName)[sendIndex - 1];
        if (expected.size() != pinned.length) {
            throw new IllegalStateException("expected " + pinned.length + " values for "
                    + eventType + ", got " + expected.size());
        }
        JsonObject fields = object(row.get("fields"), "fields");
        for (int index = 0; index < pinned.length; index++) {
            JsonValue want = expected.get(index);
            JsonValue pinnedCell = pinned[index] == null
                    ? nullCell() : new JsonString(pinned[index]);
            if (!pinnedCell.equals(want)) {
                throw new IllegalStateException("send expected[" + index + "] for " + caseName
                        + " is not pinned");
            }
            String column = "val" + index;
            JsonValue got = fields.get(column);
            if (!want.equals(got)) {
                throw new IllegalStateException("observed " + column + " drift for "
                        + eventType + ": expected " + want + " got " + got);
            }
        }
    }

    /** The tagged {"state":"null"} cell every null value renders as. */
    private static JsonObject nullCell() {
        return new JsonObject().add("state", "null");
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

    /** Canonical cell rendering: instants collapse to epoch-millis numbers, null is tagged. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            return nullCell();
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
        allowed.put("advance-time", new HashSet<>(Arrays.asList("op", "case", "at")));
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
