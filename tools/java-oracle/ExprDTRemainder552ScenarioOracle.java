import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeBean;
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
import com.espertech.esper.regressionlib.support.bean.SupportTimeStartEndA;
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
 * Java oracle for the six datetime-remainder executions (Draft 4.552):
 * ExprDTGet ord 0/1, ExprDTPlusMinus ord 0/1, ExprDTWithDate and
 * ExprDTWithTime.  Each case replays on a fresh runtime; the
 * deploy -> types -> send/set-variable cycle mirrors the executions'
 * compileDeploy -> assertStmtTypes -> sendEventBean -> assertPropsNew ->
 * runtimeSetVariable -> undeployAll flow:
 *
 * get-fields (ExprDTGetFields): utildate.get over
 * msec/sec/minutes/hour/day/month/year/week on 2002-05-30T09:01:02.003
 * emits {3,2,1,9,30,4,2002,22}.
 *
 * get-input (ExprDTGetInput): milestone 0 reads get('month') across all
 * five SupportDateTime representations of 2002-05-30T09:00:00.000 emitting
 * {4,4,4,5,5}; milestone 1 reads abc.get('month') on a SupportTimeStartEndA
 * event emitting {4}.  Milestone 2 (e.get()/e.get('abc') on
 * SupportEventWithJustGet) pins bean-method overload preference and is not
 * datetime surface; it is intentionally excluded on both sides.
 *
 * plusminus-simple (ExprDTPlusMinusSimple): a 'var' deployment creates
 * long varmsec, the clock advances to 2002-05-30T09:00:00.000 and twelve
 * plus/minus(varmsec) columns run over the null bean and varmsec
 * 0/1000/172800000.
 *
 * plusminus-timeperiod (ExprDTPlusMinusTimePeriod): the same twelve
 * columns over the literal "1 hour 10 sec 20 msec" period emit
 * 10:00:10.020/07:59:49.980.
 *
 * withdate (ExprDTWithDate): one module declares int varyear/varmonth/
 * varday with the s0 select; null assignments keep the input field.
 *
 * withtime (ExprDTWithTime): a 'variables' deployment declares the four
 * int variables before the s0 select deploys; null assignments keep the
 * input field.
 *
 * Integer cells render as JSON numbers, instant cells as epoch-millis
 * numbers, and null cells as the tagged {"state":"null"} object on both
 * sides.
 */
public final class ExprDTRemainder552ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-remainder-552";
    private static final String DESCRIPTION = "ExprDTGet/ExprDTPlusMinus/"
            + "ExprDTWithDate/ExprDTWithTime executions: get-fields replays "
            + "ExprDTGetFields (utildate.get over "
            + "msec,sec,minutes,hour,day,month,year,week on "
            + "2002-05-30T09:01:02.003 emits {3,2,1,9,30,4,2002,22} — 0-based "
            + "month, ISO week 22), get-input replays ExprDTGetInput "
            + "(get('month') over all five representations of "
            + "2002-05-30T09:00:00.000 emits {4,4,4,5,5} — Calendar-backed reps "
            + "0-based vs LDT/ZDT 1-based, then the SupportTimeStartEndA event "
            + "milestone emits {4}; the SupportEventWithJustGet bean-method "
            + "milestone is excluded as non-datetime DSL), plusminus-simple "
            + "replays ExprDTPlusMinusSimple (varmsec 0/1000/172800000 shifts "
            + "all representations of 2002-05-30T09:00:00.000 by 0/+1s/+2d; "
            + "the null-bean send emits only current_timestamp), "
            + "plusminus-timeperiod replays ExprDTPlusMinusTimePeriod "
            + "(.plus/.minus(1 hour 10 sec 20 msec) = +/-3610020ms emits "
            + "10:00:10.020/07:59:49.980), withdate replays ExprDTWithDate "
            + "(variable varyear/varmonth/varday with null-keeps-field: "
            + "(2004,8,3) emits 2004-09-03T09:00, (null,8,null) emits "
            + "2002-09-30T09:00), withtime replays ExprDTWithTime (variable "
            + "varhour/varmin/varsec/varmsec with null-keeps-field: all-null "
            + "emits the unchanged instant, (1,2,3,4) emits 01:02:03.004, "
            + "(0,null,null,6) emits 00:00:00.006).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-5080e24710592989a761",
            "java-runtime-bf73801b376a885fb2fa",
            "java-runtime-684e3de645e51f1896c9",
            "java-runtime-fb1954df531a2bf64b7d",
            "java-runtime-e14ad77bdd6257964633",
            "java-runtime-a3c430889e0a5976707a"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTGetFields",
            "ExprDTGetInput",
            "ExprDTPlusMinusSimple",
            "ExprDTPlusMinusTimePeriod",
            "ExprDTWithDate",
            "ExprDTWithTime"
    };
    private static final String[] STATIC_IDS = {
            "java-be89d7aef06f908e34ed",
            "java-be89d7aef06f908e34ed",
            "java-038b5ae418fa9a980078",
            "java-038b5ae418fa9a980078",
            "java-709b9139cb524eef97dd",
            "java-209fc0139b57c99890bb"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "get-fields",
            "get-input",
            "plusminus-simple",
            "plusminus-timeperiod",
            "withdate",
            "withtime"
    };
    private static final int[] ORDINALS = {0, 1, 0, 1, 0, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+types; deploy s0 utildate.get over msec,sec,minutes,hour,"
            + "day,month,year,week, types pins all-Integer, send "
            + "make(2002-05-30T09:01:02.003) emits {3,2,1,9,30,4,2002,22}",
            "listener x2+types; deploy s0 get('month') over all five "
            + "representations, types pins all-Integer, send "
            + "make(2002-05-30T09:00:00.000) emits {4,4,4,5,5}, undeploy, "
            + "redeploy s0 abc.get('month') on SupportTimeStartEndA, send "
            + "make(A0,2002-05-30T09:00:00.000,0) emits {4} "
            + "(SupportEventWithJustGet milestone excluded: bean-method get() "
            + "preference is not datetime DSL)",
            "listener x4+types; deploy 'var' create variable long varmsec, "
            + "advanceTime(2002-05-30T09:00:00.000), deploy s0 twelve "
            + "plus/minus(varmsec) columns, types pins "
            + "{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}x2, send "
            + "make(null) emits only current_timestamp x2, send "
            + "make(startTime) at varmsec=0 emits the instant x12, set "
            + "varmsec=1000 emits +/-1s, set varmsec=172800000 emits +/-2d",
            "listener x2+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 "
            + "twelve plus/minus(1 hour 10 sec 20 msec) columns, types pins "
            + "{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}x2, send "
            + "make(startTime) emits 10:00:10.020 x6 + 07:59:49.980 x6, send "
            + "make(null) emits only the two current_timestamp columns",
            "listener x3+types; advanceTime(2002-05-30T09:00:00.000), deploy s0 "
            + "create-variable module + withDate(varyear,varmonth,varday), "
            + "types pins {Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}, "
            + "send make(null) emits only current_timestamp, set (2004,8,3) "
            + "emits 2004-09-03T09:00 x6, set (null,8,null) emits "
            + "2002-09-30T09:00 x6",
            "listener x4+types; deploy 'variables' create variable module, "
            + "advanceTime(2002-05-30T09:00:00.000), deploy s0 "
            + "withTime(varhour,varmin,varsec,varmsec), types pins "
            + "{Long,Date,Long,Calendar,LocalDateTime,ZonedDateTime}, send "
            + "make(null) emits only current_timestamp, send at varhour=null "
            + "emits the unchanged instant x6, set (1,2,3,4) emits "
            + "01:02:03.004 x6, set (0,null,null,6) emits 00:00:00.006 x6"
    };

    private static final String GET_FIELDS_EPL =
            "@name('s0') select "
            + "utildate.get('msec') as val0,"
            + "utildate.get('sec') as val1,"
            + "utildate.get('minutes') as val2,"
            + "utildate.get('hour') as val3,"
            + "utildate.get('day') as val4,"
            + "utildate.get('month') as val5,"
            + "utildate.get('year') as val6,"
            + "utildate.get('week') as val7"
            + " from SupportDateTime";

    private static final String GET_INPUT_EPL =
            "@name('s0') select "
            + "utildate.get('month') as val0,"
            + "longdate.get('month') as val1,"
            + "caldate.get('month') as val2, "
            + "localdate.get('month') as val3, "
            + "zoneddate.get('month') as val4 "
            + " from SupportDateTime";

    private static final String GET_INPUT_MILE1_EPL =
            "@name('s0') select abc.get('month') as val0 from SupportTimeStartEndA as abc";

    private static final String PLUS_SIMPLE_VARS_EPL =
            "@name('var') @public create variable long varmsec";

    private static final String PLUS_SIMPLE_EPL =
            "@name('s0') select "
            + "current_timestamp.plus(varmsec) as val0,"
            + "utildate.plus(varmsec) as val1,"
            + "longdate.plus(varmsec) as val2,"
            + "caldate.plus(varmsec) as val3,"
            + "localdate.plus(varmsec) as val4,"
            + "zoneddate.plus(varmsec) as val5,"
            + "current_timestamp.minus(varmsec) as val6,"
            + "utildate.minus(varmsec) as val7,"
            + "longdate.minus(varmsec) as val8,"
            + "caldate.minus(varmsec) as val9,"
            + "localdate.minus(varmsec) as val10,"
            + "zoneddate.minus(varmsec) as val11"
            + " from SupportDateTime";

    private static final String TIME_PERIOD_EPL =
            "@name('s0') select "
            + "current_timestamp.plus(1 hour 10 sec 20 msec) as val0,"
            + "utildate.plus(1 hour 10 sec 20 msec) as val1,"
            + "longdate.plus(1 hour 10 sec 20 msec) as val2,"
            + "caldate.plus(1 hour 10 sec 20 msec) as val3,"
            + "localdate.plus(1 hour 10 sec 20 msec) as val4,"
            + "zoneddate.plus(1 hour 10 sec 20 msec) as val5,"
            + "current_timestamp.minus(1 hour 10 sec 20 msec) as val6,"
            + "utildate.minus(1 hour 10 sec 20 msec) as val7,"
            + "longdate.minus(1 hour 10 sec 20 msec) as val8,"
            + "caldate.minus(1 hour 10 sec 20 msec) as val9,"
            + "localdate.minus(1 hour 10 sec 20 msec) as val10,"
            + "zoneddate.minus(1 hour 10 sec 20 msec) as val11"
            + " from SupportDateTime";

    private static final String WITH_DATE_EPL =
            "create variable int varyear;\n"
            + "create variable int varmonth;\n"
            + "create variable int varday;\n"
            + "@name('s0') select "
            + "current_timestamp.withDate(varyear, varmonth, varday) as val0,"
            + "utildate.withDate(varyear, varmonth, varday) as val1,"
            + "longdate.withDate(varyear, varmonth, varday) as val2,"
            + "caldate.withDate(varyear, varmonth, varday) as val3,"
            + "localdate.withDate(varyear, varmonth+1, varday) as val4,"
            + "zoneddate.withDate(varyear, varmonth+1, varday) as val5"
            + " from SupportDateTime";

    private static final String WITH_TIME_VARS_EPL =
            "@name('variables') @public create variable int varhour;\n"
            + "@public create variable int varmin;\n"
            + "@public create variable int varsec;\n"
            + "@public create variable int varmsec;\n";

    private static final String WITH_TIME_EPL =
            "@name('s0') select "
            + "current_timestamp.withTime(varhour, varmin, varsec, varmsec) as val0,"
            + "utildate.withTime(varhour, varmin, varsec, varmsec) as val1,"
            + "longdate.withTime(varhour, varmin, varsec, varmsec) as val2,"
            + "caldate.withTime(varhour, varmin, varsec, varmsec) as val3,"
            + "localdate.withTime(varhour, varmin, varsec, varmsec) as val4,"
            + "zoneddate.withTime(varhour, varmin, varsec, varmsec) as val5"
            + " from SupportDateTime";

    private static final String[] CASE_EPLS = {
            GET_FIELDS_EPL,
            GET_INPUT_EPL,
            PLUS_SIMPLE_EPL,
            TIME_PERIOD_EPL,
            WITH_DATE_EPL,
            WITH_TIME_EPL
    };

    /** One pinned deploy step: statement label + byte-exact EPL. */
    private static final class DeployPin {
        private final String statement;
        private final String epl;

        private DeployPin(String statement, String epl) {
            this.statement = statement;
            this.epl = epl;
        }
    }

    /** Per-case pinned deployments in order. */
    private static final Map<String, DeployPin[]> DEPLOYS = new HashMap<>();
    static {
        DEPLOYS.put("get-fields", new DeployPin[]{
                new DeployPin("s0", GET_FIELDS_EPL)});
        DEPLOYS.put("get-input", new DeployPin[]{
                new DeployPin("s0", GET_INPUT_EPL),
                new DeployPin("s0", GET_INPUT_MILE1_EPL)});
        DEPLOYS.put("plusminus-simple", new DeployPin[]{
                new DeployPin("var", PLUS_SIMPLE_VARS_EPL),
                new DeployPin("s0", PLUS_SIMPLE_EPL)});
        DEPLOYS.put("plusminus-timeperiod", new DeployPin[]{
                new DeployPin("s0", TIME_PERIOD_EPL)});
        DEPLOYS.put("withdate", new DeployPin[]{
                new DeployPin("s0", WITH_DATE_EPL)});
        DEPLOYS.put("withtime", new DeployPin[]{
                new DeployPin("variables", WITH_TIME_VARS_EPL),
                new DeployPin("s0", WITH_TIME_EPL)});
    }

    /** One pinned runtimeSetVariable step. */
    private static final class VarSetPin {
        private final String statement;
        private final String name;
        private final Long value;

        private VarSetPin(String statement, String name, Long value) {
            this.statement = statement;
            this.name = name;
            this.value = value;
        }
    }

    /** Per-case pinned set-variable sequence in step order. */
    private static final Map<String, VarSetPin[]> VAR_SETS = new HashMap<>();
    static {
        VAR_SETS.put("plusminus-simple", new VarSetPin[]{
                new VarSetPin("var", "varmsec", 1000L),
                new VarSetPin("var", "varmsec", 172800000L)});
        VAR_SETS.put("withdate", new VarSetPin[]{
                new VarSetPin("s0", "varyear", 2004L),
                new VarSetPin("s0", "varmonth", 8L),
                new VarSetPin("s0", "varday", 3L),
                new VarSetPin("s0", "varyear", null),
                new VarSetPin("s0", "varmonth", 8L),
                new VarSetPin("s0", "varday", null)});
        VAR_SETS.put("withtime", new VarSetPin[]{
                new VarSetPin("variables", "varhour", null),
                new VarSetPin("variables", "varhour", 1L),
                new VarSetPin("variables", "varmin", 2L),
                new VarSetPin("variables", "varsec", 3L),
                new VarSetPin("variables", "varmsec", 4L),
                new VarSetPin("variables", "varhour", 0L),
                new VarSetPin("variables", "varmin", null),
                new VarSetPin("variables", "varsec", null),
                new VarSetPin("variables", "varmsec", 6L)});
    }

    /** Variables declared Java int (all withdate/withtime vars; varmsec is long). */
    private static final Set<String> INT_VARIABLE_CASES =
            new HashSet<>(Arrays.asList("withdate", "withtime"));

    private static final String DATE_T00 = "2002-05-30T09:00:00.000Z";
    private static final String DATE_GET = "2002-05-30T09:01:02.003Z";
    private static final long T00 = 1022749200000L;

    /**
     * Pinned assertPropsNew cells per case send, in select order; a null
     * cell renders as the tagged {"state":"null"} object in the trace
     * protocol.
     */
    private static final Map<String, Long[][]> EXPECTED = new HashMap<>();
    static {
        EXPECTED.put("get-fields", new Long[][]{
                {3L, 2L, 1L, 9L, 30L, 4L, 2002L, 22L}});
        EXPECTED.put("get-input", new Long[][]{
                {4L, 4L, 4L, 5L, 5L},
                {4L}});
        EXPECTED.put("plusminus-simple", new Long[][]{
                {T00, null, null, null, null, null, T00, null, null, null, null, null},
                repeat(1022749200000L, 12),
                concat(repeat(1022749201000L, 6), repeat(1022749199000L, 6)),
                concat(repeat(1022922000000L, 6), repeat(1022576400000L, 6))});
        EXPECTED.put("plusminus-timeperiod", new Long[][]{
                concat(repeat(1022752810020L, 6), repeat(1022745589980L, 6)),
                {1022752810020L, null, null, null, null, null,
                        1022745589980L, null, null, null, null, null}});
        EXPECTED.put("withdate", new Long[][]{
                {T00, null, null, null, null, null},
                repeat(1094202000000L, 6),
                repeat(1033376400000L, 6)});
        EXPECTED.put("withtime", new Long[][]{
                {T00, null, null, null, null, null},
                repeat(T00, 6),
                repeat(1022720523004L, 6),
                repeat(1022716800006L, 6)});
    }

    /** Send dates per case in step order; "" pins the make(null) bean. */
    private static final Map<String, String[]> SEND_DATES = new HashMap<>();
    static {
        SEND_DATES.put("get-fields", new String[]{DATE_GET});
        SEND_DATES.put("get-input", new String[]{DATE_T00, DATE_T00});
        SEND_DATES.put("plusminus-simple",
                new String[]{"", DATE_T00, DATE_T00, DATE_T00});
        SEND_DATES.put("plusminus-timeperiod", new String[]{DATE_T00, ""});
        SEND_DATES.put("withdate", new String[]{"", DATE_T00, DATE_T00});
        SEND_DATES.put("withtime",
                new String[]{"", DATE_T00, DATE_T00, DATE_T00});
    }

    /** Send event types per case in step order. */
    private static final Map<String, String[]> SEND_EVENT_TYPES = new HashMap<>();
    static {
        SEND_EVENT_TYPES.put("get-fields", new String[]{"SupportDateTime"});
        SEND_EVENT_TYPES.put("get-input",
                new String[]{"SupportDateTime", "SupportTimeStartEndA"});
        SEND_EVENT_TYPES.put("plusminus-simple", new String[]{
                "SupportDateTime", "SupportDateTime", "SupportDateTime", "SupportDateTime"});
        SEND_EVENT_TYPES.put("plusminus-timeperiod",
                new String[]{"SupportDateTime", "SupportDateTime"});
        SEND_EVENT_TYPES.put("withdate", new String[]{
                "SupportDateTime", "SupportDateTime", "SupportDateTime"});
        SEND_EVENT_TYPES.put("withtime", new String[]{
                "SupportDateTime", "SupportDateTime", "SupportDateTime", "SupportDateTime"});
    }

    /** Java-asserted property types per case per types step. */
    private static final Map<String, Map<String, String>[]> TYPE_PROPERTIES = new HashMap<>();
    static {
        Map<String, String> getFields = new HashMap<>();
        for (int index = 0; index < 8; index++) {
            getFields.put("val" + index, "Integer");
        }
        TYPE_PROPERTIES.put("get-fields", array(getFields));
        Map<String, String> getInput = new HashMap<>();
        for (int index = 0; index < 5; index++) {
            getInput.put("val" + index, "Integer");
        }
        TYPE_PROPERTIES.put("get-input", array(getInput));
        TYPE_PROPERTIES.put("plusminus-simple", array(repTypes(12)));
        TYPE_PROPERTIES.put("plusminus-timeperiod", array(repTypes(12)));
        TYPE_PROPERTIES.put("withdate", array(repTypes(6)));
        TYPE_PROPERTIES.put("withtime", array(repTypes(6)));
    }

    private static Map<String, String>[] array(Map<String, String> map) {
        @SuppressWarnings("unchecked")
        Map<String, String>[] result = new Map[]{map};
        return result;
    }

    /** The LONGBOXED/DATE/LONGBOXED/CALENDAR/LDT/ZDT rep cycle. */
    private static Map<String, String> repTypes(int count) {
        String[] names = {"Long", "Date", "Long", "Calendar", "LocalDateTime", "ZonedDateTime"};
        Map<String, String> properties = new HashMap<>();
        for (int index = 0; index < count; index++) {
            properties.put("val" + index, names[index % names.length]);
        }
        return properties;
    }

    private static Long[] repeat(Long value, int count) {
        Long[] row = new Long[count];
        Arrays.fill(row, value);
        return row;
    }

    private static Long[] concat(Long[] first, Long[] second) {
        Long[] row = Arrays.copyOf(first, first.length + second.length);
        System.arraycopy(second, 0, row, first.length, second.length);
        return row;
    }

    private static final int EXPECTED_STEPS = 65;

    /** Pinned step keys per case, built exactly like the Go runner's table. */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        for (String caseName : CASES) {
            List<String> pinned = new ArrayList<>();
            switch (caseName) {
                case "get-fields":
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(undeployKey(caseName));
                    break;
                case "get-input":
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(undeployKey(caseName));
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[1]));
                    pinned.add(sendKey(caseName, 1));
                    pinned.add(undeployKey(caseName));
                    break;
                case "plusminus-simple":
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(advanceKey(caseName));
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[1]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(sendKey(caseName, 1));
                    pinned.add(setVariableKey(caseName, 0));
                    pinned.add(sendKey(caseName, 2));
                    pinned.add(setVariableKey(caseName, 1));
                    pinned.add(sendKey(caseName, 3));
                    pinned.add(undeployKey(caseName));
                    break;
                case "plusminus-timeperiod":
                    pinned.add(advanceKey(caseName));
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(sendKey(caseName, 1));
                    pinned.add(undeployKey(caseName));
                    break;
                case "withdate":
                    pinned.add(advanceKey(caseName));
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(setVariableKey(caseName, 0));
                    pinned.add(setVariableKey(caseName, 1));
                    pinned.add(setVariableKey(caseName, 2));
                    pinned.add(sendKey(caseName, 1));
                    pinned.add(setVariableKey(caseName, 3));
                    pinned.add(setVariableKey(caseName, 4));
                    pinned.add(setVariableKey(caseName, 5));
                    pinned.add(sendKey(caseName, 2));
                    pinned.add(undeployKey(caseName));
                    break;
                case "withtime":
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[0]));
                    pinned.add(advanceKey(caseName));
                    pinned.add(deployKey(caseName, DEPLOYS.get(caseName)[1]));
                    pinned.add(typesKey(caseName));
                    pinned.add(sendKey(caseName, 0));
                    pinned.add(setVariableKey(caseName, 0));
                    pinned.add(sendKey(caseName, 1));
                    pinned.add(setVariableKey(caseName, 1));
                    pinned.add(setVariableKey(caseName, 2));
                    pinned.add(setVariableKey(caseName, 3));
                    pinned.add(setVariableKey(caseName, 4));
                    pinned.add(sendKey(caseName, 2));
                    pinned.add(setVariableKey(caseName, 5));
                    pinned.add(setVariableKey(caseName, 6));
                    pinned.add(setVariableKey(caseName, 7));
                    pinned.add(setVariableKey(caseName, 8));
                    pinned.add(sendKey(caseName, 3));
                    pinned.add(undeployKey(caseName));
                    break;
                default:
                    throw new IllegalStateException("unknown case " + caseName);
            }
            CASE_STEPS.put(caseName, pinned.toArray(new String[0]));
        }
    }

    private static String deployKey(String caseName, DeployPin pin) {
        return "deploy|" + caseName + "|" + pin.statement + "|||" + pin.epl + "|||";
    }

    private static String typesKey(String caseName) {
        return "types|" + caseName + "|s0||||||";
    }

    private static String sendKey(String caseName, int index) {
        Long[] row = EXPECTED.get(caseName)[index];
        StringBuilder cells = new StringBuilder();
        for (int cell = 0; cell < row.length; cell++) {
            if (cell > 0) {
                cells.append(",");
            }
            cells.append(row[cell] == null ? "{\"state\":\"null\"}"
                    : Json.value(row[cell]).toString());
        }
        String date = SEND_DATES.get(caseName)[index];
        String payload = "{\"date\":" + (date.isEmpty() ? "null" : "\"" + date + "\"")
                + ",\"expected\":[" + cells + "]}";
        return "send|" + caseName + "|||" + SEND_EVENT_TYPES.get(caseName)[index]
                + "||" + payload + "||";
    }

    private static String setVariableKey(String caseName, int index) {
        VarSetPin pin = VAR_SETS.get(caseName)[index];
        String payload = pin.value == null ? "null" : String.valueOf(pin.value);
        return "set-variable|" + caseName + "|" + pin.statement + "|" + pin.name
                + "|||" + payload + "||";
    }

    private static String advanceKey(String caseName) {
        return "advance-time|" + caseName + "|||||||" + DATE_T00;
    }


    private static String undeployKey(String caseName) {
        return "undeploy-all|" + caseName + "|||||||";
    }

    private ExprDTRemainder552ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTRemainder552ScenarioOracle <scenario.json>");
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
        if (records.size() != 22) {
            throw new IllegalStateException("expected 22 records, got " + records.size());
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
     * Replays the case's steps on a fresh runtime: optional create-variable
     * and s0 deployments, the advance-time, types, send and set-variable
     * ops, then undeployAll — mirroring the execution's
     * advanceTime -> compileDeploy -> assertStmtTypes -> sendEventBean ->
     * assertPropsNew -> runtimeSetVariable -> undeployAll cycle.
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
            Map<String, String> deploymentIds = new HashMap<>();
            // The RegressionPath executions (plusminus-simple, withtime)
            // compile the s0 select against the earlier create-variable
            // module's compiled form.
            List<EPCompiled> pathCompileds = new ArrayList<>();
            boolean usePath = "plusminus-simple".equals(caseName)
                    || "withtime".equals(caseName);
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
                        String at = string(step, "at");
                        if (!DATE_T00.equals(at)) {
                            throw new IllegalStateException("case " + caseName
                                    + " advance-time " + at + " is not pinned");
                        }
                        runtime.getEventService().advanceTime(
                                Instant.parse(at).toEpochMilli());
                        break;
                    }
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step,
                                listener, statements, deploymentIds,
                                pathCompileds, usePath);
                        break;
                    case "types":
                        typesStep(caseName, step, statements, records, runtime);
                        break;
                    case "send":
                        sendIndex++;
                        sendEvent(runtime, listener, caseName, sendIndex, step, records);
                        break;
                    case "set-variable":
                        setVariableStep(caseName, step, deploymentIds, runtime);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deploymentIds.clear();
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

    /** SupportDateTime and SupportTimeStartEndA bean registration; the
     * start/end timestamp property config mirrors the regression suite and
     * is what makes event-datetime abc.get('month') resolve longdateStart. */
    private static Configuration configure() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportDateTime.class);
        ConfigurationCommonEventTypeBean configBean = new ConfigurationCommonEventTypeBean();
        configBean.setStartTimestampPropertyName("longdateStart");
        configBean.setEndTimestampPropertyName("longdateEnd");
        configuration.getCommon().addEventType("SupportTimeStartEndA",
                SupportTimeStartEndA.class.getName(), configBean);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    /**
     * Compiles and deploys the pinned EPL for this deploy index, recording
     * the deployment id for runtimeSetVariable and attaching the listener
     * to s0 when the module selects — mirroring compileDeploy(path) +
     * addListener("s0").
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, JsonObject step,
                                   ListenerRecorder listener,
                                   Map<String, EPStatement> statements,
                                   Map<String, String> deploymentIds,
                                   List<EPCompiled> pathCompileds,
                                   boolean usePath) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        DeployPin[] pins = DEPLOYS.get(caseName);
        DeployPin pin = null;
        for (DeployPin candidate : pins) {
            if (candidate.statement.equals(label) && candidate.epl.equals(epl)) {
                pin = candidate;
            }
        }
        if (pin == null) {
            throw new IllegalStateException("case " + caseName + " deploy " + label
                    + " carries an unpinned EPL");
        }
        CompilerArguments arguments = new CompilerArguments(configuration);
        if (usePath) {
            arguments.getPath().getCompileds().addAll(pathCompileds);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, arguments);
        if (usePath) {
            pathCompileds.add(compiled);
        }
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        deploymentIds.put(label, deployment.getDeploymentId());
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                statements.put("s0", statement);
                statement.addListener(listener);
            }
        }
    }

    /**
     * Emits a {"operation":"types"} record carrying the Java-asserted
     * property types after verifying them, mirroring assertStmtTypes /
     * assertStmtTypesAllSame.
     */
    private static void typesStep(String caseName, JsonObject step,
                                  Map<String, EPStatement> statements, JsonArray records,
                                  EPRuntime runtime) {
        String label = string(step, "statement");
        Map<String, String>[] pins = TYPE_PROPERTIES.get(caseName);
        EPStatement statement = statements.get(label);
        if (pins == null || statement == null || !"s0".equals(label)) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        Map<String, String> pinned = pins[0];
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
     * the pinned instant, the null send mirrors make(null) — or the
     * SupportTimeStartEndA milestone bean; drains the delivery it produced
     * and emits exactly one listener record after verifying the row against
     * the pinned expected cells — the assertPropsNew equivalent.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  int sendIndex, JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        String[] pinnedTypes = SEND_EVENT_TYPES.get(caseName);
        if (pinnedTypes == null || sendIndex > pinnedTypes.length
                || !pinnedTypes[sendIndex - 1].equals(eventType)) {
            throw new IllegalStateException("case " + caseName + " send " + sendIndex
                    + " carries unpinned event type " + eventType);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        JsonValue dateValue = payload.get("date");
        String pinnedDate = SEND_DATES.get(caseName)[sendIndex - 1];
        if (dateValue == null || dateValue.isNull()) {
            if (!pinnedDate.isEmpty()) {
                throw new IllegalStateException("case " + caseName
                        + " null send is not pinned at index " + sendIndex);
            }
        } else if (!pinnedDate.equals(dateValue.asString())) {
            throw new IllegalStateException("case " + caseName + " send date "
                    + dateValue.asString() + " is not pinned");
        }
        if ("SupportDateTime".equals(eventType)) {
            SupportDateTime bean = dateValue == null || dateValue.isNull()
                    ? SupportDateTime.make(null)
                    : SupportDateTime.make(javaDateString(dateValue.asString()));
            runtime.getEventService().sendEventBean(bean, "SupportDateTime");
        } else if ("SupportTimeStartEndA".equals(eventType)) {
            SupportTimeStartEndA bean = SupportTimeStartEndA.make("A0",
                    javaDateString(dateValue.asString()), 0);
            runtime.getEventService().sendEventBean(bean, "SupportTimeStartEndA");
        } else {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
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
     * Executes one runtimeSetVariable against the deployment the pinned
     * statement name addresses — env.runtimeSetVariable(deployment, name,
     * value): long varmsec takes a Long, the int variables take Integer or
     * null (null keeps the field in withDate/withTime).
     */
    private static void setVariableStep(String caseName, JsonObject step,
                                        Map<String, String> deploymentIds,
                                        EPRuntime runtime) {
        String label = string(step, "statement");
        String name = string(step, "name");
        VarSetPin[] pins = VAR_SETS.get(caseName);
        boolean pinned = false;
        for (VarSetPin pin : pins == null ? new VarSetPin[0] : pins) {
            if (pin.statement.equals(label) && pin.name.equals(name)) {
                pinned = true;
            }
        }
        if (!pinned) {
            throw new IllegalStateException("case " + caseName + " set-variable "
                    + label + "." + name + " is not pinned");
        }
        String deploymentId = deploymentIds.get(label);
        if (deploymentId == null) {
            throw new IllegalStateException("set-variable deployment " + label
                    + " was not deployed in case " + caseName);
        }
        JsonValue payload = step.get("payload");
        Object value;
        if (payload == null || payload.isNull()) {
            value = null;
        } else if (INT_VARIABLE_CASES.contains(caseName)) {
            value = (int) longInteger(payload, "payload");
        } else {
            value = longInteger(payload, "payload");
        }
        runtime.getVariableService().setVariableValue(deploymentId, name, value);
    }

    /**
     * Verifies the delivered row's fields against the payload's pinned
     * expected cells in select order, mirroring assertPropsNew.  Null
     * cells compare through the tagged {"state":"null"} object.
     */
    private static void verifyExpected(String caseName, String eventType, int sendIndex,
                                       JsonObject payload, JsonObject row) {
        JsonArray expected = array(payload.get("expected"), "expected");
        Long[] pinned = EXPECTED.get(caseName)[sendIndex - 1];
        if (expected.size() != pinned.length) {
            throw new IllegalStateException("expected " + pinned.length + " values for "
                    + eventType + ", got " + expected.size());
        }
        JsonObject fields = object(row.get("fields"), "fields");
        for (int index = 0; index < pinned.length; index++) {
            JsonValue want = expected.get(index);
            JsonValue pinnedCell = pinned[index] == null
                    ? nullCell() : Json.value(pinned[index]);
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

    /** Renders an ISO instant as the local-format string the beans parse. */
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
        allowed.put("set-variable",
                new HashSet<>(Arrays.asList("op", "case", "statement", "name", "payload")));
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
