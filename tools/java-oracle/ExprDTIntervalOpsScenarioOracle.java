import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeBean;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeObjectArray;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportDateTime;
import com.espertech.esper.regressionlib.support.bean.SupportTimeStartEndA;
import com.espertech.esper.regressionlib.support.bean.SupportTimeStartEndB;
import com.espertech.esper.regressionlib.support.schedule.SupportDateTimeFieldType;
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
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Java oracle for ExprDTIntervalOps ordinals 0, 17 and 1.  Three cases replay
 * on fresh runtimes:
 *
 * calendar-ops (ordinal 0, ExprDTIntervalCalendarOps): the five where-clauses
 * run over all five A_<FT>/B_<FT> object-array field types, mirroring
 * assertExpression's variant-major/field-type-minor loop.  Java deploys
 * "select * ... where <clause>" and asserts listener-invoked == expected per
 * A send; this oracle deploys the same clause in the select-clause form
 * "select <clause> as c0 ..." (the form ExprDTIntervalBeforeInSelectClause
 * proves compiles) so every A send emits one row carrying the observed
 * boolean and the Go trace compares totally.  The per-send expected flag is
 * still verified, mirroring env.assertListener.
 *
 * point-in-time-calendar (ordinal 17, ExprDTIntervalPointInTimeWCalendarOps):
 * one unidirectional SupportDateTime x SupportBean#lastevent deployment
 * emitting c0..c4; the SupportBean seed produces no row.
 *
 * invalid (ordinal 1, ExprDTIntervalInvalid): 23 tryInvalidCompile probes
 * compile without the runtime path (compileWCheckedEx(epl, null)) and record
 * the pinned startsWith prefixes.
 */
public final class ExprDTIntervalOpsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-interval-ops";
    private static final String DESCRIPTION = "ExprDTIntervalOps ords 0/17/1: calendar-ops replays ExprDTIntervalCalendarOps (five where-clauses "
            + "over the five A_<FT>/B_<FT> object-array field types, deployed in the select-clause form '<clause> "
            + "as c0' so the observed boolean is total per A send), point-in-time-calendar replays "
            + "ExprDTIntervalPointInTimeWCalendarOps (set('month',1).before over the five SupportDateTime "
            + "representations), and invalid replays ExprDTIntervalInvalid's 23 tryInvalidCompile probes with "
            + "pinned Java message prefixes.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/"
                    + "ExprDTIntervalOps.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-e29e7c3671fa1488e788",
            "java-runtime-bcdd330f46ca3dd63da7",
            "java-runtime-eb0d233c3d7f90db1120"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTIntervalCalendarOps",
            "ExprDTIntervalPointInTimeWCalendarOps",
            "ExprDTIntervalInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-7d6d5e191799aa72b645",
            "java-583eee3ee090b7f63076",
            "java-8b79d6633a735b6cc1ec"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "calendar-ops",
            "point-in-time-calendar",
            "invalid"
    };
    private static final int[] ORDINALS = {0, 17, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; 25 deployments (5 where-clauses x 5 field types) each send one B_<FT> seed then A_<FT> "
            + "rows; Java asserts listener invoked iff expected under 'select * ... where <clause>' — the parity "
            + "deploy renders the same clause as '<clause> as c0' so every A send emits one row carrying the "
            + "observed boolean",
            "listener; one deployment emits c0..c4 per SupportDateTime send: the SupportBean seed produces no "
            + "row (unidirectional), 2002-05-30T09:00:00.000 yields all-true and 2003-05-30T08:00:00.000 yields "
            + "all-false",
            "compile-error; 23 tryInvalidCompile probes record the pinned Java message prefixes (startsWith "
            + "assertion); the Go runner verifies the nearest expressible rejection for the representable subset "
            + "and pins prefix-only for the unrepresentable rest"
    };
    private static final String[] CASE_EPLS = {
            "@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b",
            "@name('s0') select longdate.set('month', 1).before(longPrimitive) as c0, utildate.set('month', "
            + "1).before(longPrimitive) as c1,caldate.set('month', 1).before(longPrimitive) as "
            + "c2,localdate.set('month', 1).before(longPrimitive) as c3,zoneddate.set('month', "
            + "1).before(longPrimitive) as c4 from SupportDateTime unidirectional, SupportBean#lastevent",
            "select a.before('x') from SupportTimeStartEndA as a"
    };

    private static final int EXPECTED_STEPS = 118;
    private static final int EXPECTED_RECORDS = 60;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError.  Deploy steps
     * carry the byte-exact EPL text; send steps carry the compacted payload
     * (A/SupportDateTime sends pin the expected flag); build-error steps
     * carry the pinned expectError prefix.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("calendar-ops", new String[]{
                "deploy|calendar-ops|v1-msec||@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b||",
                "send|calendar-ops||B_MSEC||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2999-01-01T09:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v1-date||@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_DATE#lastevent as a, B_DATE#lastevent as b||",
                "send|calendar-ops||B_DATE||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2999-01-01T09:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v1-cal||@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_CAL#lastevent as a, B_CAL#lastevent as b||",
                "send|calendar-ops||B_CAL||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2999-01-01T09:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v1-ldt||@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_LDT#lastevent as a, B_LDT#lastevent as b||",
                "send|calendar-ops||B_LDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2999-01-01T09:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v1-zdt||@name('s0') select a.withDate(2001, 1, 1).before(b) as c0 from A_ZDT#lastevent as a, B_ZDT#lastevent as b||",
                "send|calendar-ops||B_ZDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2999-01-01T09:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v2-msec||@name('s0') select a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b||",
                "send|calendar-ops||B_MSEC||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2999-01-01T10:00:00.001Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2999-01-01T08:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v2-date||@name('s0') select a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)) as c0 from A_DATE#lastevent as a, B_DATE#lastevent as b||",
                "send|calendar-ops||B_DATE||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2999-01-01T10:00:00.001Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2999-01-01T08:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v2-cal||@name('s0') select a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)) as c0 from A_CAL#lastevent as a, B_CAL#lastevent as b||",
                "send|calendar-ops||B_CAL||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2999-01-01T10:00:00.001Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2999-01-01T08:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v2-ldt||@name('s0') select a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)) as c0 from A_LDT#lastevent as a, B_LDT#lastevent as b||",
                "send|calendar-ops||B_LDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2999-01-01T10:00:00.001Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2999-01-01T08:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v2-zdt||@name('s0') select a.withDate(2001, 1, 1).before(b.withDate(2001, 1, 1)) as c0 from A_ZDT#lastevent as a, B_ZDT#lastevent as b||",
                "send|calendar-ops||B_ZDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2999-01-01T10:00:00.001Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2999-01-01T08:00:00.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v3-msec||@name('s0') select a.before(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b||",
                "send|calendar-ops||B_MSEC||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v3-date||@name('s0') select a.before(b) as c0 from A_DATE#lastevent as a, B_DATE#lastevent as b||",
                "send|calendar-ops||B_DATE||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v3-cal||@name('s0') select a.before(b) as c0 from A_CAL#lastevent as a, B_CAL#lastevent as b||",
                "send|calendar-ops||B_CAL||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v3-ldt||@name('s0') select a.before(b) as c0 from A_LDT#lastevent as a, B_LDT#lastevent as b||",
                "send|calendar-ops||B_LDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v3-zdt||@name('s0') select a.before(b) as c0 from A_ZDT#lastevent as a, B_ZDT#lastevent as b||",
                "send|calendar-ops||B_ZDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v4-msec||@name('s0') select a.withTime(8, 59, 59, 0).before(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b||",
                "send|calendar-ops||B_MSEC||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v4-date||@name('s0') select a.withTime(8, 59, 59, 0).before(b) as c0 from A_DATE#lastevent as a, B_DATE#lastevent as b||",
                "send|calendar-ops||B_DATE||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v4-cal||@name('s0') select a.withTime(8, 59, 59, 0).before(b) as c0 from A_CAL#lastevent as a, B_CAL#lastevent as b||",
                "send|calendar-ops||B_CAL||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v4-ldt||@name('s0') select a.withTime(8, 59, 59, 0).before(b) as c0 from A_LDT#lastevent as a, B_LDT#lastevent as b||",
                "send|calendar-ops||B_LDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v4-zdt||@name('s0') select a.withTime(8, 59, 59, 0).before(b) as c0 from A_ZDT#lastevent as a, B_ZDT#lastevent as b||",
                "send|calendar-ops||B_ZDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":0}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2002-05-30T08:59:59.000Z\",\"duration\":2000,\"expected\":false}|",
                "deploy|calendar-ops|v5-msec||@name('s0') select a.after(b) as c0 from A_MSEC#lastevent as a, B_MSEC#lastevent as b||",
                "send|calendar-ops||B_MSEC||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2002-05-30T09:00:01.000Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_MSEC||{\"start\":\"2002-05-30T09:00:01.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v5-date||@name('s0') select a.after(b) as c0 from A_DATE#lastevent as a, B_DATE#lastevent as b||",
                "send|calendar-ops||B_DATE||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2002-05-30T09:00:01.000Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_DATE||{\"start\":\"2002-05-30T09:00:01.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v5-cal||@name('s0') select a.after(b) as c0 from A_CAL#lastevent as a, B_CAL#lastevent as b||",
                "send|calendar-ops||B_CAL||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2002-05-30T09:00:01.000Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_CAL||{\"start\":\"2002-05-30T09:00:01.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v5-ldt||@name('s0') select a.after(b) as c0 from A_LDT#lastevent as a, B_LDT#lastevent as b||",
                "send|calendar-ops||B_LDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2002-05-30T09:00:01.000Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_LDT||{\"start\":\"2002-05-30T09:00:01.001Z\",\"duration\":0,\"expected\":true}|",
                "deploy|calendar-ops|v5-zdt||@name('s0') select a.after(b) as c0 from A_ZDT#lastevent as a, B_ZDT#lastevent as b||",
                "send|calendar-ops||B_ZDT||{\"start\":\"2002-05-30T09:00:00.000Z\",\"duration\":1000}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2002-05-30T09:00:01.000Z\",\"duration\":0,\"expected\":false}|",
                "send|calendar-ops||A_ZDT||{\"start\":\"2002-05-30T09:00:01.001Z\",\"duration\":0,\"expected\":true}|",
                "undeploy-all|calendar-ops|||||",
        });
        CASE_STEPS.put("point-in-time-calendar", new String[]{
                "deploy|point-in-time-calendar|s0||@name('s0') select longdate.set('month', 1).before(longPrimitive) as c0, utildate.set('month', 1).before(longPrimitive) as c1,caldate.set('month', 1).before(longPrimitive) as c2,localdate.set('month', 1).before(longPrimitive) as c3,zoneddate.set('month', 1).before(longPrimitive) as c4 from SupportDateTime unidirectional, SupportBean#lastevent||",
                "send|point-in-time-calendar||SupportBean||{\"longPrimitive\":1022749200000}|",
                "send|point-in-time-calendar||SupportDateTime||{\"date\":\"2002-05-30T09:00:00.000Z\",\"expected\":true}|",
                "send|point-in-time-calendar||SupportDateTime||{\"date\":\"2003-05-30T08:00:00.000Z\",\"expected\":false}|",
                "undeploy-all|point-in-time-calendar|||||",
        });
        CASE_STEPS.put("invalid", new String[]{
                "build-error|invalid|before-string-param||select a.before('x') from SupportTimeStartEndA as a||Failed to validate select-clause expression 'a.before('x')': Failed to resolve enumeration method, date-time method or mapped property 'a.before('x')': For date-time method 'before' the first parameter expression returns 'String', however requires a Date, Calendar, Long-type return value or event (with timestamp)",
                "build-error|invalid|before-untimestamped-event||select a.before(b) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'a.before(b)': For date-time method 'before' the first parameter is event type 'SupportBean', however no timestamp property has been defined for this event type",
                "build-error|invalid|before-boolean-param||select a.before(true) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'a.before(true)': For date-time method 'before' the first parameter expression returns 'boolean', however requires a Date, Calendar, Long-type return value or event (with timestamp)",
                "build-error|invalid|before-no-params||select a.before() from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'a.before()': Parameters mismatch for date-time method 'before', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing interval start value, or an expression providing timestamp or timestamped-event and an expression providing interval start value and an expression providing interval finishes value, but receives no parameters",
                "build-error|invalid|before-string-target||select theString.before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'theString.before(a)': Date-time enumeration method 'before' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property but received String",
                "build-error|invalid|before-untimestamped-target||select b.before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'b.before(a)': Date-time enumeration method 'before' requires either a Calendar, Date, long, LocalDateTime or ZonedDateTime value as input or events of an event type that declares a timestamp property",
                "build-error|invalid|before-get-target||select a.get('month').before(a) from SupportTimeStartEndA#lastevent as a, SupportBean#lastevent as b||Failed to validate select-clause expression 'a.get(\"month\").before(a)': Failed to resolve method 'get': Could not find enumeration method, date-time method, instance method or property named 'get'",
                "build-error|invalid|before-string-threshold||select a.before(b, 'abc') from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.before(b,\"abc\")': Failed to validate date-time method 'before', expected a time-period expression or a numeric-type result for expression parameter 1 but received String ",
                "build-error|invalid|before-string-threshold-2||select a.before(b, 1, 'def') from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.before(b,1,\"def\")': Failed to validate date-time method 'before', expected a time-period expression or a numeric-type result for expression parameter 2 but received String ",
                "build-error|invalid|before-four-params||select a.before(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.before(b,1,2,3)': Parameters mismatch for date-time method 'before', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing interval start value, or an expression providing timestamp or timestamped-event and an expression providing interval start value and an expression providing interval finishes value, but receives 4 expressions ",
                "build-error|invalid|coincides-four-params||select a.coincides(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.coincides(b,1,2,3)': Parameters mismatch for date-time method 'coincides', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing threshold for start and end value, or an expression providing timestamp or timestamped-event and an expression providing threshold for start value and an expression providing threshold for end value, but receives 4 expressions ",
                "build-error|invalid|coincides-negative||select a.coincides(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.coincides(b,-1)': The coincides date-time method does not allow negative start and end values ",
                "build-error|invalid|during-four-params||select a.during(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.during(b,1,2,3)': Parameters mismatch for date-time method 'during', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance interval both start and end and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance start and an expression providing maximum distance start and an expression providing minimum distance end and an expression providing maximum distance end, but receives 4 expressions ",
                "build-error|invalid|finishes-three-params||select a.finishes(b, 1, 2) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.finishes(b,1,2)': Parameters mismatch for date-time method 'finishes', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between end timestamps, but receives 3 expressions ",
                "build-error|invalid|finishes-negative||select a.finishes(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.finishes(b,-1)': The finishes date-time method does not allow negative threshold value ",
                "build-error|invalid|finishedby-negative||select a.finishedby(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.finishedby(b,-1)': The finishedby date-time method does not allow negative threshold value ",
                "build-error|invalid|meets-three-params||select a.meets(b, 1, 2) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.meets(b,1,2)': Parameters mismatch for date-time method 'meets', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between start and end timestamps, but receives 3 expressions ",
                "build-error|invalid|meets-negative||select a.meets(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.meets(b,-1)': The meets date-time method does not allow negative threshold value ",
                "build-error|invalid|metby-negative||select a.metBy(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.metBy(b,-1)': The metBy date-time method does not allow negative threshold value ",
                "build-error|invalid|overlaps-four-params||select a.overlaps(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.overlaps(b,1,2,3)': Parameters mismatch for date-time method 'overlaps', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance interval both start and end, or an expression providing timestamp or timestamped-event and an expression providing minimum distance interval both start and end and an expression providing maximum distance interval both start and end, but receives 4 expressions ",
                "build-error|invalid|starts-four-params||select a.starts(b, 1, 2, 3) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.starts(b,1,2,3)': Parameters mismatch for date-time method 'starts', the method has multiple footprints accepting an expression providing timestamp or timestamped-event, or an expression providing timestamp or timestamped-event and an expression providing maximum distance between start timestamps, but receives 4 expressions ",
                "build-error|invalid|starts-negative||select a.starts(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.starts(b,-1)': The starts date-time method does not allow negative threshold value ",
                "build-error|invalid|startedby-negative||select a.startedBy(b, -1) from SupportTimeStartEndA#lastevent as a, SupportTimeStartEndB#lastevent as b||Failed to validate select-clause expression 'a.startedBy(b,-1)': The startedBy date-time method does not allow negative threshold value ",
                "undeploy-all|invalid|||||",
        });
    }

    private ExprDTIntervalOpsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTIntervalOpsScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
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
     * Replays the case's steps on a fresh runtime.  Deploy steps undeploy the
     * previous statement first, mirroring the undeployAll() between
     * assertExpression calls; build-error steps compile without the runtime
     * path exactly like env.tryInvalidCompile(epl, message).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = configure(caseName);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            ListenerRecorder listener = new ListenerRecorder(caseName, runtime);
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
                    case "deploy": {
                        runtime.getDeploymentService().undeployAll();
                        listener.reset();
                        String epl = string(step, "epl");
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, new CompilerArguments(configuration));
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        boolean found = false;
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(listener);
                                found = true;
                            }
                        }
                        if (!found) {
                            throw new IllegalStateException(
                                    "statement s0 was not deployed for " + string(step, "statement"));
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, listener, caseName, step, records);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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

    /**
     * Per-case event-type registration mirroring TestSuiteExprDateTime:
     * calendar-ops registers the ten A_<FT>/B_<FT> object-array types with
     * startTS/endTS timestamp properties; point-in-time-calendar registers
     * the SupportDateTime and SupportBean beans; invalid registers the
     * SupportTimeStartEndA/B beans (longdateStart/longdateEnd timestamps)
     * plus SupportBean.
     */
    private static Configuration configure(String caseName) {
        Configuration configuration = new Configuration();
        switch (caseName) {
            case "calendar-ops":
                for (SupportDateTimeFieldType fieldType : SupportDateTimeFieldType.values()) {
                    ConfigurationCommonEventTypeObjectArray objectArray =
                            new ConfigurationCommonEventTypeObjectArray();
                    objectArray.setStartTimestampPropertyName("startTS");
                    objectArray.setEndTimestampPropertyName("endTS");
                    for (String prefix : new String[]{"A_", "B_"}) {
                        configuration.getCommon().addEventType(prefix + fieldType.name(),
                                "startTS,endTS".split(","),
                                new Object[]{fieldType.getClazz(), fieldType.getClazz()},
                                objectArray);
                    }
                }
                break;
            case "point-in-time-calendar":
                configuration.getCommon().addEventType(SupportDateTime.class);
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case "invalid":
                ConfigurationCommonEventTypeBean bean = new ConfigurationCommonEventTypeBean();
                bean.setStartTimestampPropertyName("longdateStart");
                bean.setEndTimestampPropertyName("longdateEnd");
                configuration.getCommon().addEventType(
                        "SupportTimeStartEndA", SupportTimeStartEndA.class.getName(), bean);
                configuration.getCommon().addEventType(
                        "SupportTimeStartEndB", SupportTimeStartEndB.class.getName(), bean);
                configuration.getCommon().addEventType(SupportBean.class);
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

    /**
     * Sends one event, drains the deliveries it produced into listener
     * records, and verifies the observed booleans against the pinned
     * expected flag — the assertListener equivalent.  B_<FT> seeds and the
     * SupportBean send must produce no delivery.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        boolean expectRow = eventType.startsWith("A_") || "SupportDateTime".equals(eventType);
        if (eventType.startsWith("A_") || eventType.startsWith("B_")) {
            String fieldType = eventType.substring(2);
            SupportDateTimeFieldType type = SupportDateTimeFieldType.valueOf(fieldType);
            String start = javaDateString(string(payload, "start"));
            long duration = longInteger(payload.get("duration"), "duration");
            Object[] event = new Object[]{type.makeStart(start), type.makeEnd(start, duration)};
            runtime.getEventService().sendEventObjectArray(event, eventType);
        } else if ("SupportDateTime".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    SupportDateTime.make(javaDateString(string(payload, "date"))), "SupportDateTime");
        } else if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setLongPrimitive(longInteger(payload.get("longPrimitive"), "longPrimitive"));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
        List<JsonObject> delivered = listener.drain();
        if (!expectRow) {
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
        boolean expected = payload.getBoolean("expected", false);
        if ("SupportDateTime".equals(eventType)) {
            JsonObject fields = object(row.get("fields"), "fields");
            for (String column : new String[]{"c0", "c1", "c2", "c3", "c4"}) {
                if (fields.getBoolean(column, !expected) != expected) {
                    throw new IllegalStateException("observed " + column + " drift for "
                            + eventType + ": expected " + expected + " got " + row);
                }
            }
        } else {
            JsonObject fields = object(row.get("fields"), "fields");
            if (fields.getBoolean("c0", !expected) != expected) {
                throw new IllegalStateException("observed c0 drift for " + eventType
                        + ": expected " + expected + " got " + row);
            }
        }
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

    /** Converts the RFC3339 payload instant to the Java default format. */
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
        if (value instanceof LocalDateTime) {
            return Json.value(((LocalDateTime) value).atZone(ZoneId.systemDefault())
                    .toInstant().toEpochMilli());
        }
        if (value instanceof java.time.ZonedDateTime) {
            return Json.value(((java.time.ZonedDateTime) value).toInstant().toEpochMilli());
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Buffers rendered rows per delivery so the send step can verify the
     * observed booleans before emitting the listener record.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> pending = new ArrayList<>();
        private long sequence;

        private ListenerRecorder(String caseName, EPRuntime runtime) {
            this.caseName = caseName;
            this.runtime = runtime;
        }

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
     * op|case|statement|eventType|epl|payload|expectError with the payload
     * compacted.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload", "expectError"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError");
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
