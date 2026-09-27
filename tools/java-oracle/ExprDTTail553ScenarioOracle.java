import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeBean;
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
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0_Container;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportDateTime;
import com.espertech.esper.regressionlib.support.bean.SupportTimeStartEndA;
import com.espertech.esper.regressionlib.support.bean.SupportTimeStartEndB;
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
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Calendar;
import java.util.Date;
import java.util.GregorianCalendar;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TimeZone;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Java oracle for the five datetime-tail executions (Draft 4.553):
 * ExprToCalendarChain, ExprDTToDateCalMSecValue, ExprDTDocSamples,
 * ExprDTIntervalOpsCreateSchema and ExprDTInvalid.  Each case replays on a
 * fresh runtime; the deploy -> types -> send/undeploy cycle mirrors the
 * executions' compileDeploy -> assertStmtTypes -> sendEventBean ->
 * assertPropsNew/assertEqualsNew/assertListenerInvokedFlag ->
 * tryInvalidCompile -> undeployAll flow:
 *
 * tocalendar-chain (ExprToCalendarChain): advanceTime(0), deploy s0
 * current_timestamp.toCalendar().add(Calendar.DAY_OF_MONTH,1) - a
 * Calendar.add method call, not a date-time chain op - then the
 * SupportBean("E1",0) send emits the null "c" cell.
 *
 * todatecalmsec-value (ExprDTToDateCalMSecValue): the unnamed warmup deploy
 * of the same chain (a compile-only deployed marker), advanceTime to
 * 2002-05-30T09:00:00.000, the eighteen-column toDate/toCalendar/toMillisec
 * s0 select, the Date x6/Calendar x6/Long x6 types pin, then make(startTime)
 * emits the instant x18 and make(null) emits only the three
 * current_timestamp cells.
 *
 * docsamples (ExprDTDocSamples): the fifteen doc selects over
 * RFIDEvent.timeTaken compile one deployment each (deployed markers), then
 * four pattern [a=A -> b=B] probes assert the listener-invoked flag per
 * after() condition.
 *
 * intervalops-createschema (ExprDTIntervalOpsCreateSchema): the map and
 * object-array create-schema legs over the five timestamp field types send
 * TypeA then TypeB and assert a.includes(b) == true; the Avro/JSON/
 * JSON-provided legs are unrepresentable on the Go side and excluded on
 * both.  The bean-tail deploys the SupportBeanXXX longPrimitive/longBoxed
 * timestamp schema and asserts a.get('month') == 4.
 *
 * invalid (ExprDTInvalid): eleven tryInvalidCompile probes record the
 * pinned Java message prefixes (the suite's startsWith assertion).
 *
 * Instant cells render as epoch-millis numbers, boolean/int cells as JSON
 * numbers/booleans, and null cells as the tagged {"state":"null"} object on
 * both sides.
 */
public final class ExprDTTail553ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-tail-553";
    private static final String DESCRIPTION = "ExprDTToDateCalMSec/ExprDTDocSamples/"
            + "ExprDTIntervalOpsCreateSchema/ExprDTInvalid executions: "
            + "tocalendar-chain replays ExprToCalendarChain "
            + "(current_timestamp.toCalendar().add(DAY_OF_MONTH,1) is a Calendar "
            + "method call, not a date-time chain op, and Java observes null), "
            + "todatecalmsec-value replays ExprDTToDateCalMSecValue (the 18-column "
            + "toDate/toCalendar/toMillisec matrix pins Date x6, Calendar x6, "
            + "Long x6 types and, for the null bean, only the three "
            + "current_timestamp cells), docsamples replays ExprDTDocSamples "
            + "(15 compile-only doc selects over RFIDEvent.timeTaken then four "
            + "pattern after() probes pinned by their listener-invoked flag), "
            + "intervalops-createschema replays ExprDTIntervalOpsCreateSchema "
            + "(map + object-array create-schema TypeA/TypeB startts/endts "
            + "timestamps over five field types asserting a.includes(b) true, "
            + "then the SupportBeanXXX bean-timestamp tail asserting "
            + "a.get('month')==4; the Avro/JSON/JSON-provided legs are "
            + "unrepresentable), invalid replays ExprDTInvalid's 11 "
            + "tryInvalidCompile probes with pinned Java message prefixes.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-96fba8bd8db354b4058e",
            "java-runtime-9d3c9122a833c6235a1d",
            "java-runtime-a3cec43b565226d9cbac",
            "java-runtime-8c90cf3d4a6abbd4848f",
            "java-runtime-59302e42b09f7b2f6818"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprToCalendarChain",
            "ExprDTToDateCalMSecValue",
            "ExprDTDocSamples",
            "ExprDTIntervalOpsCreateSchema",
            "ExprDTInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-30f73c23a62ab5959453",
            "java-30f73c23a62ab5959453",
            "java-1788456b92fc17823173",
            "java-46150495284a50e52094",
            "java-0147f234d5dc15c47624"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "tocalendar-chain",
            "todatecalmsec-value",
            "docsamples",
            "intervalops-createschema",
            "invalid"
    };
    private static final int[] ORDINALS = {0, 1, 0, 0, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; advanceTime(0), deploy s0 the toCalendar().add() chain "
                    + "select, send SupportBean('E1',0) emits the null c column "
                    + "(Calendar.add is a method call returning void, not a "
                    + "date-time chain op)",
            "deployed+types+listener x2; the unnamed warmup deploy of the same "
                    + "toCalendar().add() chain emits a deployed marker, "
                    + "advanceTime(2002-05-30T09:00:00.000), deploy s0 the "
                    + "18-column toDate/toCalendar/toMillisec select, types "
                    + "pins Date x6 + Calendar x6 + Long x6, send make(t) "
                    + "emits the instant x18, send make(null) emits only the "
                    + "three current_timestamp cells",
            "deployed x15+count x4; fifteen compile-only doc selects over "
                    + "RFIDEvent.timeTaken each emit a deployed marker, then "
                    + "four pattern [a=A -> b=B] after() probes send A then B "
                    + "and emit the listener-invoked count (true, true, true, "
                    + "then false reversed)",
            "deployed x10+listener x11; ten map/object-array legs deploy the "
                    + "create-schema TypeA/TypeB timestamp module and send A "
                    + "then B, the second send emits a.includes(b)=true (the "
                    + "Avro/JSON legs are excluded as unrepresentable), then "
                    + "the bean-tail deploy emits a.get('month')==4 for the "
                    + "SupportBeanXXX longPrimitive timestamp",
            "compile-error; 11 tryInvalidCompile probes record the pinned "
                    + "Java message prefixes (startsWith assertion); the Go "
                    + "runner verifies the nearest expressible rejection for "
                    + "the three representable probes and pins prefix-only "
                    + "for the unrepresentable rest"
    };

    private static final String T00 = "2002-05-30T09:00:00.000Z";
    private static final long T00_MILLIS = 1022749200000L;
    private static final String EPOCH = "1970-01-01T00:00:00Z";

    private static final String CHAIN_EPL =
            "@name('s0') select current_timestamp.toCalendar().add(Calendar.DAY_OF_MONTH,1) as c from SupportBean";
    private static final String WARMUP_EPL =
            "select current_timestamp.toCalendar().add(Calendar.DAY_OF_MONTH,1) from SupportBean";
    private static final String VALUE_EPL =
            "@name('s0') select "
                    + "current_timestamp.toDate() as val0,"
                    + "utildate.toDate() as val1,"
                    + "longdate.toDate() as val2,"
                    + "caldate.toDate() as val3,"
                    + "localdate.toDate() as val4,"
                    + "zoneddate.toDate() as val5,"
                    + "current_timestamp.toCalendar() as val6,"
                    + "utildate.toCalendar() as val7,"
                    + "longdate.toCalendar() as val8,"
                    + "caldate.toCalendar() as val9,"
                    + "localdate.toCalendar() as val10,"
                    + "zoneddate.toCalendar() as val11,"
                    + "current_timestamp.toMillisec() as val12,"
                    + "utildate.toMillisec() as val13,"
                    + "longdate.toMillisec() as val14,"
                    + "caldate.toMillisec() as val15,"
                    + "localdate.toMillisec() as val16,"
                    + "zoneddate.toMillisec() as val17"
                    + " from SupportDateTime";

    private static final String[] DOC_SELECT_LABELS = {
            "doc-format", "doc-get-month", "doc-get-month-of-year",
            "doc-minus-minutes", "doc-minus-millis", "doc-plus-minutes",
            "doc-plus-millis", "doc-round-ceiling", "doc-round-floor",
            "doc-set-month", "doc-with-date", "doc-with-max",
            "doc-to-calendar", "doc-to-date", "doc-to-millisec"
    };
    private static final String[] DOC_SELECT_EPLS = {
            "select timeTaken.format() as timeTakenStr from RFIDEvent",
            "select timeTaken.get('month') as timeTakenMonth from RFIDEvent",
            "select timeTaken.getMonthOfYear() as timeTakenMonth from RFIDEvent",
            "select timeTaken.minus(2 minutes) as timeTakenMinus2Min from RFIDEvent",
            "select timeTaken.minus(2*60*1000) as timeTakenMinus2Min from RFIDEvent",
            "select timeTaken.plus(2 minutes) as timeTakenMinus2Min from RFIDEvent",
            "select timeTaken.plus(2*60*1000) as timeTakenMinus2Min from RFIDEvent",
            "select timeTaken.roundCeiling('min') as timeTakenRounded from RFIDEvent",
            "select timeTaken.roundFloor('min') as timeTakenRounded from RFIDEvent",
            "select timeTaken.set('month', 3) as timeTakenMonth from RFIDEvent",
            "select timeTaken.withDate(2002, 4, 30) as timeTakenDated from RFIDEvent",
            "select timeTaken.withMax('sec') as timeTakenMaxSec from RFIDEvent",
            "select timeTaken.toCalendar() as timeTakenCal from RFIDEvent",
            "select timeTaken.toDate() as timeTakenDate from RFIDEvent",
            "select timeTaken.toMillisec() as timeTakenLong from RFIDEvent"
    };

    /** One pinned pattern after() probe: condition, A/B starts, invoked flag. */
    private static final class PatternProbe {
        private final String condition;
        private final String startA;
        private final String startB;
        private final boolean invoked;

        private PatternProbe(String condition, String startA, String startB, boolean invoked) {
            this.condition = condition;
            this.startA = startA;
            this.startB = startB;
            this.invoked = invoked;
        }
    }

    private static final PatternProbe[] PATTERN_PROBES = {
            new PatternProbe("a.longdateStart.after(b)", "2002-05-30T09:00:00.000Z",
                    "2002-05-30T08:59:59.999Z", true),
            new PatternProbe("a.after(b.longdateStart)", "2002-05-30T09:00:00.000Z",
                    "2002-05-30T08:59:59.999Z", true),
            new PatternProbe("a.after(b)", "2002-05-30T09:00:00.000Z",
                    "2002-05-30T08:59:59.999Z", true),
            new PatternProbe("a.after(b)", "2002-05-30T08:59:59.999Z",
                    "2002-05-30T09:00:00.000Z", false)
    };

    private static final String[] LEG_REPS = {"map", "objectarray"};
    private static final String[] LEG_TYPES = {
            "msec", "calendar", "date", "localdatetime", "zoneddatetime"};
    private static final Map<String, String> LEG_JAVA_TYPES = new HashMap<>();
    static {
        LEG_JAVA_TYPES.put("msec", "long");
        LEG_JAVA_TYPES.put("calendar", "java.util.Calendar");
        LEG_JAVA_TYPES.put("date", "java.util.Date");
        LEG_JAVA_TYPES.put("localdatetime", "java.time.LocalDateTime");
        LEG_JAVA_TYPES.put("zoneddatetime", "java.time.ZonedDateTime");
    }

    private static String legEpl(String rep, String legType) {
        String annotation = "@EventRepresentation('"
                + ("objectarray".equals(rep) ? "objectarray" : "map") + "') ";
        String javaType = LEG_JAVA_TYPES.get(legType);
        return annotation + "@buseventtype @public create schema TypeA as (startts "
                + javaType + ", endts " + javaType
                + ") starttimestamp startts endtimestamp endts;\n"
                + annotation + "@buseventtype @public create schema TypeB as (startts "
                + javaType + ", endts " + javaType
                + ") starttimestamp startts endtimestamp endts;\n"
                + "@name('s0') select a.includes(b) as val0 from TypeA#lastevent as a, TypeB#lastevent as b;\n";
    }

    private static final String BEAN_TAIL_EPL =
            "@public @buseventtype create schema SupportBeanXXX as "
                    + "com.espertech.esper.common.internal.support.SupportBean starttimestamp longPrimitive endtimestamp longBoxed;\n"
                    + "@name('s0') select a.get('month') as val0 from SupportBeanXXX a;\n";

    private static String patternEpl(String condition) {
        return "@name('s0') select * from pattern [a=A -> b=B] as abc where " + condition;
    }

    /** One pinned deploy step: statement label, byte-exact EPL, whether the
     * compile is itself observed (deployed record) and whether the s0
     * select's rows ("rows") or pattern deliveries ("pattern") are
     * listener-observed. */
    private static final class DeployPin {
        private final String statement;
        private final String epl;
        private final boolean deployed;
        private final String listener;

        private DeployPin(String statement, String epl, boolean deployed, String listener) {
            this.statement = statement;
            this.epl = epl;
            this.deployed = deployed;
            this.listener = listener;
        }
    }

    /** Per-case pinned deployments in step order. */
    private static final Map<String, DeployPin[]> DEPLOYS = new HashMap<>();
    static {
        DEPLOYS.put("tocalendar-chain", new DeployPin[]{
                new DeployPin("s0", CHAIN_EPL, false, "rows")});
        DEPLOYS.put("todatecalmsec-value", new DeployPin[]{
                new DeployPin("warmup", WARMUP_EPL, true, ""),
                new DeployPin("s0", VALUE_EPL, false, "rows")});
        List<DeployPin> docs = new ArrayList<>();
        for (int index = 0; index < DOC_SELECT_LABELS.length; index++) {
            docs.add(new DeployPin(DOC_SELECT_LABELS[index], DOC_SELECT_EPLS[index], true, ""));
        }
        for (PatternProbe probe : PATTERN_PROBES) {
            docs.add(new DeployPin("s0", patternEpl(probe.condition), false, "pattern"));
        }
        DEPLOYS.put("docsamples", docs.toArray(new DeployPin[0]));
        List<DeployPin> interval = new ArrayList<>();
        for (String rep : LEG_REPS) {
            for (String legType : LEG_TYPES) {
                interval.add(new DeployPin("schema-" + rep + "-" + legType,
                        legEpl(rep, legType), true, "rows"));
            }
        }
        interval.add(new DeployPin("bean-tail", BEAN_TAIL_EPL, true, "rows"));
        DEPLOYS.put("intervalops-createschema", interval.toArray(new DeployPin[0]));
    }

    /** One pinned send: Java event type, the fields the payload carries and
     * the resolved assertion (expected cells or the pattern invoked flag). */
    private static final class SendPin {
        private final String eventType;
        private final String payload;
        private final Object[] expected; // null: no row assertion
        private final Boolean invoked;   // pattern probes only

        private SendPin(String eventType, String payload, Object[] expected, Boolean invoked) {
            this.eventType = eventType;
            this.payload = payload;
            this.expected = expected;
            this.invoked = invoked;
        }
    }

    private static final JsonObject NULL_TOKEN = new JsonObject().add("state", "null");

    private static Object[] expectedCells(int count, boolean allMillis) {
        Object[] cells = new Object[count];
        for (int index = 0; index < count; index++) {
            cells[index] = allMillis ? (Object) T00_MILLIS : null;
        }
        return cells;
    }

    private static String expectedText(Object[] expected) {
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < expected.length; index++) {
            if (index > 0) {
                text.append(",");
            }
            Object cell = expected[index];
            if (cell == null) {
                text.append("{\"state\":\"null\"}");
            } else if (cell instanceof Boolean) {
                text.append(((Boolean) cell) ? "true" : "false");
            } else {
                text.append(cell);
            }
        }
        return text.toString();
    }

    /** Per-case pinned send sequence in step order; the payload is the
     * byte-exact compacted JSON. */
    private static final Map<String, SendPin[]> SENDS = new HashMap<>();
    static {
        SENDS.put("tocalendar-chain", new SendPin[]{
                new SendPin("SupportBean",
                        "{\"theString\":\"E1\",\"intPrimitive\":0,\"expected\":[{\"state\":\"null\"}]}",
                        new Object[]{null}, null)});
        Object[] nullRow = expectedCells(18, false);
        nullRow[0] = T00_MILLIS;
        nullRow[6] = T00_MILLIS;
        nullRow[12] = T00_MILLIS;
        SENDS.put("todatecalmsec-value", new SendPin[]{
                new SendPin("SupportDateTime",
                        "{\"date\":\"" + T00 + "\",\"expected\":[" + expectedText(expectedCells(18, true)) + "]}",
                        expectedCells(18, true), null),
                new SendPin("SupportDateTime",
                        "{\"date\":null,\"expected\":[" + expectedText(nullRow) + "]}",
                        nullRow, null)});
        List<SendPin> docs = new ArrayList<>();
        for (PatternProbe probe : PATTERN_PROBES) {
            docs.add(new SendPin("A", "{\"date\":\"" + probe.startA + "\"}", null, null));
            docs.add(new SendPin("B", "{\"date\":\"" + probe.startB + "\",\"expected\":["
                            + (probe.invoked ? "true" : "false") + "]}",
                    new Object[]{probe.invoked}, probe.invoked));
        }
        SENDS.put("docsamples", docs.toArray(new SendPin[0]));
        List<SendPin> interval = new ArrayList<>();
        for (int index = 0; index < LEG_REPS.length * LEG_TYPES.length; index++) {
            interval.add(new SendPin("TypeA",
                    "{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}", null, null));
            interval.add(new SendPin("TypeB",
                    "{\"start\":\"2002-05-30T09:00:00.500Z\",\"duration\":200,\"expected\":[true]}",
                    new Object[]{true}, null));
        }
        interval.add(new SendPin("SupportBeanXXX",
                "{\"longPrimitive\":" + T00_MILLIS + ",\"expected\":[4]}",
                new Object[]{4L}, null));
        SENDS.put("intervalops-createschema", interval.toArray(new SendPin[0]));
    }

    /** Java-asserted s0 property types for todatecalmsec-value. */
    private static final Map<String, String> TYPE_PROPERTIES = new HashMap<>();
    static {
        for (int index = 0; index < 6; index++) {
            TYPE_PROPERTIES.put("val" + index, "Date");
            TYPE_PROPERTIES.put("val" + (index + 6), "Calendar");
            TYPE_PROPERTIES.put("val" + (index + 12), "Long");
        }
    }

    /** One pinned ExprDTInvalid probe: label, byte-exact EPL, pinned
     * startsWith prefix. */
    private static final class InvalidProbe {
        private final String label;
        private final String epl;
        private final String expect;

        private InvalidProbe(String label, String epl, String expect) {
            this.label = label;
            this.epl = epl;
            this.expect = expect;
        }
    }

    private static final InvalidProbe[] INVALID_PROBES = {
            new InvalidProbe("set-contained-collection",
                    "select contained.set('hour', 1) from SupportBean_ST0_Container",
                    "Failed to validate select-clause expression 'contained.set(\"hour\",1)': Date-time enumeration method 'set' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property but received collection of events of type 'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'"),
            new InvalidProbe("set-window-collection",
                    "select window(*).set('hour', 1) from SupportBean#keepall",
                    "Failed to validate select-clause expression 'window(*).set(\"hour\",1)': Date-time enumeration method 'set' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property but received collection of events of type 'SupportBean'"),
            new InvalidProbe("set-invalid-field",
                    "select utildate.set('invalid') from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.set('invalid')': Failed to resolve enumeration method, date-time method or mapped property 'utildate.set('invalid')': Parameters mismatch for date-time method 'set', the method requires an expression providing a string-type calendar field name and an expression providing an integer-type value"),
            new InvalidProbe("set-lambda-param",
                    "select utildate.set(x => true) from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.set()': Parameters mismatch for date-time method 'set', the method requires an expression providing a string-type calendar field name and an expression providing an integer-type value"),
            new InvalidProbe("set-no-params",
                    "select utildate.set() from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.set()': Parameters mismatch for date-time method 'set', the method requires an expression providing a string-type calendar field name and an expression providing an integer-type value"),
            new InvalidProbe("set-numeric-param",
                    "select utildate.set(1) from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.set(1)': Parameters mismatch for date-time method 'set', the method requires an expression providing a string-type calendar field name and an expression providing an integer-type value"),
            new InvalidProbe("between-string-bounds",
                    "select utildate.between('a', 'b') from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.between(\"a\",\"b\")': Failed to validate date-time method 'between', expected a long-typed, Date-typed or Calendar-typed result for expression parameter 0 but received String"),
            new InvalidProbe("between-int-endpoint",
                    "select utildate.between(utildate, utildate, 1, true) from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.between(utildate,utildate,...(42 chars)': Failed to validate date-time method 'between', expected a boolean-type result for expression parameter 2 but received int"),
            new InvalidProbe("format-formatter-param",
                    "select utildate.format(java.time.format.DateTimeFormatter.ISO_ORDINAL_DATE) from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.format(ParseCaseSensitive(...(114 chars)': Date-time enumeration method 'format' invalid format, expected string-format or DateFormat but received java.time.format.DateTimeFormatter"),
            new InvalidProbe("format-dateformat-param",
                    "select zoneddate.format(SimpleDateFormat.getInstance()) from SupportDateTime",
                    "Failed to validate select-clause expression 'zoneddate.format(SimpleDateFormat.g...(48 chars)': Date-time enumeration method 'format' invalid format, expected string-format or DateTimeFormatter but received java.text.DateFormat"),
            new InvalidProbe("format-null-param",
                    "select utildate.format(null) from SupportDateTime",
                    "Failed to validate select-clause expression 'utildate.format(null)': Failed to validate date-time method 'format', expected a non-null result for expression parameter 0 but received a null-typed expression")
    };

    private static final String[] CASE_EPLS = {
            CHAIN_EPL,
            VALUE_EPL,
            patternEpl(PATTERN_PROBES[0].condition),
            legEpl("map", "msec"),
            INVALID_PROBES[0].epl
    };

    private static final int EXPECTED_STEPS = 102;
    private static final int EXPECTED_RECORDS = 57;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|name|eventType|epl|payload|expectError|at - the
     * same nine-field layout the Go runner pins.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        List<String> chain = new ArrayList<>();
        chain.add(advanceKey("tocalendar-chain", EPOCH));
        chain.add(deployKey("tocalendar-chain", DEPLOYS.get("tocalendar-chain")[0]));
        chain.add(sendKey("tocalendar-chain", SENDS.get("tocalendar-chain")[0]));
        chain.add(undeployKey("tocalendar-chain"));
        CASE_STEPS.put("tocalendar-chain", chain.toArray(new String[0]));

        List<String> value = new ArrayList<>();
        value.add(deployKey("todatecalmsec-value", DEPLOYS.get("todatecalmsec-value")[0]));
        value.add(advanceKey("todatecalmsec-value", T00));
        value.add(deployKey("todatecalmsec-value", DEPLOYS.get("todatecalmsec-value")[1]));
        value.add(typesKey("todatecalmsec-value"));
        value.add(sendKey("todatecalmsec-value", SENDS.get("todatecalmsec-value")[0]));
        value.add(sendKey("todatecalmsec-value", SENDS.get("todatecalmsec-value")[1]));
        value.add(undeployKey("todatecalmsec-value"));
        CASE_STEPS.put("todatecalmsec-value", value.toArray(new String[0]));

        List<String> docs = new ArrayList<>();
        for (int index = 0; index < DOC_SELECT_LABELS.length; index++) {
            docs.add(deployKey("docsamples", DEPLOYS.get("docsamples")[index]));
        }
        for (int index = 0; index < PATTERN_PROBES.length; index++) {
            docs.add(deployKey("docsamples", DEPLOYS.get("docsamples")[DOC_SELECT_LABELS.length + index]));
            docs.add(sendKey("docsamples", SENDS.get("docsamples")[index * 2]));
            docs.add(sendKey("docsamples", SENDS.get("docsamples")[index * 2 + 1]));
            docs.add(undeployKey("docsamples"));
        }
        CASE_STEPS.put("docsamples", docs.toArray(new String[0]));

        List<String> interval = new ArrayList<>();
        DeployPin[] legs = DEPLOYS.get("intervalops-createschema");
        SendPin[] intervalSends = SENDS.get("intervalops-createschema");
        for (int index = 0; index < LEG_REPS.length * LEG_TYPES.length; index++) {
            interval.add(deployKey("intervalops-createschema", legs[index]));
            interval.add(sendKey("intervalops-createschema", intervalSends[index * 2]));
            interval.add(sendKey("intervalops-createschema", intervalSends[index * 2 + 1]));
            interval.add(undeployKey("intervalops-createschema"));
        }
        interval.add(deployKey("intervalops-createschema", legs[legs.length - 1]));
        interval.add(sendKey("intervalops-createschema", intervalSends[intervalSends.length - 1]));
        interval.add(undeployKey("intervalops-createschema"));
        CASE_STEPS.put("intervalops-createschema", interval.toArray(new String[0]));

        List<String> invalid = new ArrayList<>();
        for (InvalidProbe probe : INVALID_PROBES) {
            invalid.add("build-error|invalid|" + probe.label + "|||" + probe.epl
                    + "||" + probe.expect + "|");
        }
        invalid.add(undeployKey("invalid"));
        CASE_STEPS.put("invalid", invalid.toArray(new String[0]));
    }

    private static String deployKey(String caseName, DeployPin pin) {
        return "deploy|" + caseName + "|" + pin.statement + "|||" + pin.epl + "|||";
    }

    private static String typesKey(String caseName) {
        return "types|" + caseName + "|s0||||||";
    }

    private static String sendKey(String caseName, SendPin pin) {
        return "send|" + caseName + "|||" + pin.eventType + "||" + pin.payload + "||";
    }

    private static String advanceKey(String caseName, String at) {
        return "advance-time|" + caseName + "|||||||" + at;
    }

    private static String undeployKey(String caseName) {
        return "undeploy-all|" + caseName + "|||||||";
    }

    private ExprDTTail553ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTTail553ScenarioOracle <scenario.json>");
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
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
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
     * Replays the case's steps on a fresh runtime: deployments attach the
     * row/pattern listener and emit the deployed marker for compile-only
     * observables; sends deliver the pinned event and assert the pinned
     * cells or the pattern invoked flag; build-error steps run the
     * tryInvalidCompile probe.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = configure(caseName);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            CaseState state = new CaseState(caseName, runtime);
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
                    case "advance-time":
                        advanceTimeStep(runtime, caseName, step);
                        break;
                    case "deploy":
                        deployStep(runtime, configuration, state, step, records);
                        break;
                    case "types":
                        typesStep(runtime, state, step, records);
                        break;
                    case "send":
                        sendStep(runtime, state, step, records);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        state.listener.reset();
                        state.legRep = "";
                        state.legType = "";
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

    /** Per-case replay state: the row recorder, the pinned step cursors and
     * the emitted-record sequence. */
    private static final class CaseState {
        private final String caseName;
        private final EPRuntime runtime;
        private final ListenerRecorder listener;
        private final Map<String, EPStatement> statements = new HashMap<>();
        private int deployIndex;
        private int sendIndex;
        private long sequence;
        private String legRep = "";
        private String legType = "";

        private CaseState(String caseName, EPRuntime runtime) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.listener = new ListenerRecorder();
        }
    }

    /** advanceTime with the pinned instant; both EPOCH and T00 are pinned. */
    private static void advanceTimeStep(EPRuntime runtime, String caseName, JsonObject step) {
        String at = string(step, "at");
        if (!EPOCH.equals(at) && !T00.equals(at)) {
            throw new IllegalStateException("case " + caseName
                    + " advance-time " + at + " is not pinned");
        }
        runtime.getEventService().advanceTime(Instant.parse(at).toEpochMilli());
    }

    /**
     * Compiles and deploys the pinned EPL for this deploy index, attaches
     * the recorder to s0 when the deployment is listener-observed and emits
     * the deployed marker when the compile is the observable - mirroring
     * compileDeploy (+ addListener for s0 selects).
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   CaseState state, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        DeployPin[] pins = DEPLOYS.get(state.caseName);
        if (pins == null || state.deployIndex >= pins.length) {
            throw new IllegalStateException("case " + state.caseName
                    + " has no pinned deploy " + state.deployIndex);
        }
        DeployPin pin = pins[state.deployIndex++];
        if (!pin.statement.equals(label) || !pin.epl.equals(epl)) {
            throw new IllegalStateException("case " + state.caseName + " deploy " + label
                    + " carries an unpinned EPL");
        }
        if ("intervalops-createschema".equals(state.caseName)) {
            trackLeg(state, label);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(epl, new CompilerArguments(configuration));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        if ("rows".equals(pin.listener) || "pattern".equals(pin.listener)) {
            for (EPStatement statement : deployment.getStatements()) {
                if ("s0".equals(statement.getName())) {
                    state.statements.put("s0", statement);
                    statement.addListener(state.listener);
                }
            }
        }
        if (pin.deployed) {
            JsonObject record = new JsonObject();
            record.add("case", state.caseName);
            record.add("operation", "deployed");
            record.add("statement", label);
            record.add("sequence", ++state.sequence);
            record.add("time", currentTime(runtime));
            records.add(record);
        }
    }

    /** Tracks the create-schema leg label so TypeA/TypeB sends resolve the
     * representation and timestamp field type. */
    private static void trackLeg(CaseState state, String label) {
        if ("bean-tail".equals(label)) {
            state.legRep = "";
            state.legType = "";
            return;
        }
        for (String rep : LEG_REPS) {
            for (String legType : LEG_TYPES) {
                if (("schema-" + rep + "-" + legType).equals(label)) {
                    state.legRep = rep;
                    state.legType = legType;
                    return;
                }
            }
        }
        throw new IllegalStateException("malformed leg label " + label);
    }

    /**
     * Emits the types record for s0 after verifying every pinned property
     * type - the assertStmtTypes equivalent.
     */
    private static void typesStep(EPRuntime runtime, CaseState state, JsonObject step,
                                  JsonArray records) {
        if (!"todatecalmsec-value".equals(state.caseName)
                || !"s0".equals(string(step, "statement"))) {
            throw new IllegalStateException("types step is not pinned for " + state.caseName);
        }
        EPStatement statement = state.statements.get("s0");
        if (statement == null) {
            throw new IllegalStateException("s0 was not deployed for " + state.caseName);
        }
        EventType eventType = statement.getEventType();
        JsonObject properties = new JsonObject();
        for (Map.Entry<String, String> entry : new TreeMap<>(TYPE_PROPERTIES).entrySet()) {
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
        record.add("case", state.caseName);
        record.add("operation", "types");
        record.add("statement", "s0");
        record.add("sequence", 0);
        record.add("time", currentTime(runtime));
        record.add("value", value);
        records.add(record);
    }

    /**
     * Sends the pinned event for this send index, then resolves the pinned
     * assertion: the pattern probes emit the listener-invoked count record
     * (assertListenerInvokedFlag), every other observed send drains exactly
     * one row, verifies the pinned cells and emits the listener record
     * (assertPropsNew/assertEqualsNew).
     */
    private static void sendStep(EPRuntime runtime, CaseState state, JsonObject step,
                                 JsonArray records) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        SendPin[] pins = SENDS.get(state.caseName);
        if (pins == null || state.sendIndex >= pins.length) {
            throw new IllegalStateException("case " + state.caseName
                    + " has no pinned send " + state.sendIndex);
        }
        SendPin pin = pins[state.sendIndex++];
        if (!pin.eventType.equals(eventType)
                || !pin.payload.equals(payload.toString())) {
            throw new IllegalStateException("case " + state.caseName + " send "
                    + (state.sendIndex - 1) + " is not pinned");
        }
        deliver(runtime, state, pin, payload);
        if (pin.invoked != null) {
            verifyInvoked(state, pin, records);
            return;
        }
        List<JsonObject> delivered = state.listener.drain();
        if (pin.expected == null) {
            if (!delivered.isEmpty()) {
                throw new IllegalStateException("unexpected delivery for " + eventType);
            }
            return;
        }
        if (delivered.size() != 1) {
            throw new IllegalStateException("expected one delivery for " + eventType
                    + ", got " + delivered.size());
        }
        JsonObject row = delivered.get(0);
        verifyExpected(state, eventType, pin, row);
        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "listener");
        record.add("statement", "s0");
        record.add("sequence", ++state.sequence);
        record.add("time", currentTime(runtime));
        JsonArray newArray = new JsonArray();
        newArray.add(row);
        record.add("new", newArray);
        records.add(record);
    }

    /** Delivers the pinned event on the runtime: SupportBean /
     * SupportDateTime beans, the A/B timestamped beans, the TypeA/TypeB
     * leg events (map or object-array per the tracked leg) and the
     * SupportBeanXXX timestamped bean. */
    private static void deliver(EPRuntime runtime, CaseState state, SendPin pin,
                                JsonObject payload) {
        String eventType = pin.eventType;
        switch (eventType) {
            case "SupportBean": {
                SupportBean bean = new SupportBean(
                        string(payload, "theString"),
                        (int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, "SupportBean");
                return;
            }
            case "SupportDateTime": {
                JsonValue date = payload.get("date");
                SupportDateTime bean = date == null || date.isNull()
                        ? SupportDateTime.make(null)
                        : SupportDateTime.make(javaDateString(date.asString()));
                runtime.getEventService().sendEventBean(bean, "SupportDateTime");
                return;
            }
            case "A": {
                String date = string(payload, "date");
                SupportTimeStartEndA bean = SupportTimeStartEndA.make("E1",
                        javaDateString(date), 0);
                runtime.getEventService().sendEventBean(bean, "A");
                return;
            }
            case "B": {
                String date = string(payload, "date");
                SupportTimeStartEndB bean = SupportTimeStartEndB.make("E2",
                        javaDateString(date), 0);
                runtime.getEventService().sendEventBean(bean, "B");
                return;
            }
            case "TypeA":
            case "TypeB": {
                String start = string(payload, "start");
                long duration = longInteger(payload.get("duration"), "duration");
                Instant startInstant = Instant.parse(start);
                Instant endInstant = startInstant.plusMillis(duration);
                Object startValue = makeTimestamp(state.legType, startInstant);
                Object endValue = makeTimestamp(state.legType, endInstant);
                if ("objectarray".equals(state.legRep)) {
                    runtime.getEventService().sendEventObjectArray(
                            new Object[]{startValue, endValue}, eventType);
                } else {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("startts", startValue);
                    event.put("endts", endValue);
                    runtime.getEventService().sendEventMap(event, eventType);
                }
                return;
            }
            case "SupportBeanXXX": {
                SupportBean bean = new SupportBean();
                bean.setLongPrimitive(longInteger(payload.get("longPrimitive"), "longPrimitive"));
                runtime.getEventService().sendEventBean(bean, "SupportBeanXXX");
                return;
            }
            default:
                throw new IllegalStateException("unknown eventType " + eventType);
        }
    }

    /** Renders one leg timestamp value in the pinned Java field type. */
    private static Object makeTimestamp(String legType, Instant instant) {
        switch (legType) {
            case "msec":
                return instant.toEpochMilli();
            case "calendar":
                GregorianCalendar calendar = new GregorianCalendar(TimeZone.getTimeZone("UTC"));
                calendar.setTimeInMillis(instant.toEpochMilli());
                return calendar;
            case "date":
                return Date.from(instant);
            case "localdatetime":
                return LocalDateTime.ofInstant(instant, ZoneId.of("UTC"));
            case "zoneddatetime":
                return ZonedDateTime.ofInstant(instant, ZoneId.of("UTC"));
            default:
                throw new IllegalStateException("unknown leg type " + legType);
        }
    }

    /**
     * Emits the listener-invoked count record after verifying the observed
     * flag - the assertListenerInvokedFlag equivalent.
     */
    private static void verifyInvoked(CaseState state, SendPin pin, JsonArray records) {
        boolean invoked = !state.listener.drain().isEmpty();
        if (invoked != pin.invoked) {
            throw new IllegalStateException("pattern invoked = " + invoked
                    + ", want " + pin.invoked);
        }
        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "count");
        record.add("statement", "s0");
        record.add("sequence", ++state.sequence);
        record.add("time", currentTime(state.runtime));
        record.add("name", "listener-invoked");
        record.add("count", invoked ? 1 : 0);
        records.add(record);
    }

    /**
     * Verifies the delivered row's fields against the pinned expected cells,
     * mirroring assertPropsNew/assertEqualsNew; null cells compare through
     * the tagged {"state":"null"} object.
     */
    private static void verifyExpected(CaseState state, String eventType, SendPin pin,
                                       JsonObject row) {
        String[] columns;
        if ("SupportBean".equals(eventType) && "tocalendar-chain".equals(state.caseName)) {
            columns = new String[]{"c"};
        } else if ("TypeB".equals(eventType) || "SupportBeanXXX".equals(eventType)) {
            columns = new String[]{"val0"};
        } else {
            columns = new String[pin.expected.length];
            for (int index = 0; index < columns.length; index++) {
                columns[index] = "val" + index;
            }
        }
        if (columns.length != pin.expected.length) {
            throw new IllegalStateException("column count drift for " + eventType);
        }
        JsonObject fields = object(row.get("fields"), "fields");
        for (int index = 0; index < pin.expected.length; index++) {
            JsonValue want = cellValue(pin.expected[index]);
            JsonValue got = fields.get(columns[index]);
            if (!want.equals(got)) {
                throw new IllegalStateException("observed " + columns[index]
                        + " drift for " + eventType + ": expected " + want + " got " + got);
            }
        }
    }

    private static JsonValue cellValue(Object cell) {
        if (cell == null) {
            return nullCell();
        }
        if (cell instanceof Boolean) {
            return Json.value((Boolean) cell);
        }
        if (cell instanceof Number) {
            return Json.value(((Number) cell).longValue());
        }
        return Json.value(String.valueOf(cell));
    }

    /**
     * Compiles an expected-invalid probe without the runtime path, mirroring
     * env.tryInvalidCompile(epl, message) -> compileWCheckedEx(epl, null),
     * and emits a compile-error record carrying the pinned prefix after
     * verifying the caught message starts with it.
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        InvalidProbe pinned = null;
        for (InvalidProbe probe : INVALID_PROBES) {
            if (probe.label.equals(label)) {
                pinned = probe;
            }
        }
        if (pinned == null || !pinned.epl.equals(epl) || !pinned.expect.equals(expected)) {
            throw new IllegalStateException("build-error probe " + label + " is not pinned");
        }
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Per-case event-type registration mirroring TestSuiteExprDateTime:
     * tocalendar-chain registers SupportBean; todatecalmsec-value registers
     * SupportDateTime + SupportBean; docsamples registers the RFIDEvent map
     * type plus the start-timestamp A/B beans; intervalops-createschema
     * registers SupportBean (the create-schema SupportBeanXXX type is
     * declared by the module itself); invalid registers SupportDateTime,
     * SupportBean and SupportBean_ST0_Container.
     */
    private static Configuration configure(String caseName) {
        Configuration configuration = new Configuration();
        switch (caseName) {
            case "tocalendar-chain":
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case "todatecalmsec-value":
                configuration.getCommon().addEventType(SupportBean.class);
                configuration.getCommon().addEventType(SupportDateTime.class);
                break;
            case "docsamples": {
                Map<String, Object> meta = new HashMap<>();
                meta.put("timeTaken", Date.class);
                configuration.getCommon().addEventType("RFIDEvent", meta);
                ConfigurationCommonEventTypeBean leg = new ConfigurationCommonEventTypeBean();
                leg.setStartTimestampPropertyName("longdateStart");
                configuration.getCommon().addEventType("A",
                        SupportTimeStartEndA.class.getName(), leg);
                configuration.getCommon().addEventType("B",
                        SupportTimeStartEndB.class.getName(), leg);
                break;
            }
            case "intervalops-createschema":
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case "invalid":
                configuration.getCommon().addEventType(SupportDateTime.class);
                configuration.getCommon().addEventType(SupportBean.class);
                configuration.getCommon().addEventType(SupportBean_ST0_Container.class);
                break;
            default:
                throw new IllegalStateException("unsupported case " + caseName);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    /** The tagged {"state":"null"} cell every null value renders as. */
    private static JsonObject nullCell() {
        return new JsonObject().add("state", "null");
    }

    /** Renders an ISO instant as the local-format string the beans parse. */
    private static String javaDateString(String iso) {
        Instant instant = Instant.parse(iso);
        return LocalDateTime.ofInstant(instant, ZoneId.of("UTC"))
                .format(DateTimeFormatter.ofPattern(DateTime.DEFAULT_XMLLIKE_DATE_FORMAT));
    }

    private static String currentTime(EPRuntime runtime) {
        return Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString();
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

    /** Canonical cell rendering: instants collapse to epoch-millis numbers,
     * null is tagged. */
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
     * op|case|statement|name|eventType|epl|payload|expectError|at with the
     * payload compacted.  Unknown fields are rejected per op.
     */
    private static String stepKey(JsonObject step) {
        String operation = string(step, "op");
        Map<String, Set<String>> allowed = new HashMap<>();
        allowed.put("case", new HashSet<>(Arrays.asList("op", "case")));
        allowed.put("advance-time", new HashSet<>(Arrays.asList("op", "case", "at")));
        allowed.put("deploy", new HashSet<>(Arrays.asList("op", "case", "statement", "epl")));
        allowed.put("types", new HashSet<>(Arrays.asList("op", "case", "statement")));
        allowed.put("send", new HashSet<>(Arrays.asList("op", "case", "eventType", "payload")));
        allowed.put("build-error",
                new HashSet<>(Arrays.asList("op", "case", "statement", "epl", "expectError")));
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
                + "|" + string(step, "name") + "|" + string(step, "eventType")
                + "|" + string(step, "epl") + "|" + payloadText
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
