import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the epl-subselect-order-of-eval-index parity
 * unit: EPLSubselectOrderOfEval (subselect evaluation order) and
 * EPLSubselectIndex (implicit subquery-side index choice) with four cases and
 * 45 listener records over 170 pinned steps. Case correlated-subquery-order
 * deploys the two-statement module verbatim (the seed statement is unnamed and
 * unheard) with wall-clock bean times pinned to 1000/1010, which is provably
 * outcome-invariant because the correlation reads securityID only and the
 * clock never advances. Case order-of-eval-subselect-first keeps the default
 * selfSubselectPreeval=true so both not-in filters stay silent (zero records).
 * Case index-choices-overdefined-where replays the 19-cycle index-choice
 * matrix without the regression's @Hook(INDEX_CALLBACK_HOOK) plan assertions
 * (hook class is off the oracle classpath; plan-only, behavior-neutral,
 * keeping the DISABLE_UNIQUE_IMPLICIT_IDX hint on cycle 14). Case
 * unique-index-correlated replays unique/firstunique/time+unique/groupwin+
 * unique correlated scalar subqueries. Listeners are istream-only, sequence
 * numbering is continuous within a case across deploy cycles, and every
 * record time is epoch zero.
 */
public final class EPLSubselectOrderOfEvalIndexScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-subselect-order-of-eval-index";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/"
                    + "EPLSubselectOrderOfEval.java";
    private static final String DESCRIPTION =
            "EPLSubselectOrderOfEval correlated subquery window ordering plus subquery-first order-of-evaluation silence, with EPLSubselectIndex subquery index choices over an overdefined where clause and unique/firstunique/time/groupwin correlated subquery indexes (second oracle source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/"
                    + "EPLSubselectIndex.java).";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-3dbb926e23a4521d64d5",
            "java-runtime-9a68943733f98a1ea1dd",
            "java-runtime-f220d864166a5650e6f7",
            "java-runtime-0fbd10b1080bb8e7afa3",
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLSubselectCorrelatedSubqueryOrder",
            "EPLSubselectOrderOfEvaluationSubselectFirst",
            "EPLSubselectIndexChoicesOverdefinedWhere",
            "EPLSubselectUniqueIndexCorrelated",
    };
    private static final String[] STATIC_IDS = {
            "java-2ddf3ed58c64fe3674f7",
            "java-eaa8e4f13676cc852f92",
            "java-29ec8e7d3c6e96c8d5aa",
            "java-77258a6e645d2449b31c",
    };
    private static final String CASE_ORDER = "correlated-subquery-order";
    private static final String CASE_PREEVAL = "order-of-eval-subselect-first";
    private static final String CASE_INDEX_CHOICES = "index-choices-overdefined-where";
    private static final String CASE_UNIQUE_CORRELATED = "unique-index-correlated";

    private static final int EXPECTED_RECORDS = 45;
    private static final int EXPECTED_STEPS = 170;
    private static final String LISTENED_STATEMENT = "s0";

    // Case correlated-subquery-order: the verbatim two-statement module text.
    private static final String EPL_MODULE_ORDER =
            "select * from SupportTradeEventTwo#lastevent;\n"
                    + "@name('s0') select window(tl.*) as longItems,        (SELECT window(ts.*) AS shortItems "
                    + "FROM SupportTradeEventTwo#time(20 minutes) as ts WHERE ts.securityID=tl.securityID) "
                    + "from SupportTradeEventTwo#time(20 minutes) as tl where tl.securityID = 1000"
                    + "group by tl.securityID";

    // Case order-of-eval-subselect-first: two deploy cycles, the first keeps
    // the source's doubled @name annotation verbatim.
    private static final String EPL_PREEVAL_ONE =
            "@name('s0') @name('s0')select * from SupportBean(intPrimitive<10) where intPrimitive not in "
                    + "(select intPrimitive from SupportBean#unique(intPrimitive))";
    private static final String EPL_PREEVAL_TWO =
            "@name('s0') select * from SupportBean where intPrimitive not in (select intPrimitive from "
                    + "SupportBean(intPrimitive<10)#unique(intPrimitive))";

    // Case unique-index-correlated: four deploy cycles.
    private static final String EPL_UNIQUE =
            "@name('s0') select id as c0, (select intPrimitive from SupportBean#unique(theString) where "
                    + "theString = s0.p00) as c1 from SupportBean_S0 as s0";
    private static final String EPL_FIRST_UNIQUE =
            "@name('s0') select id as c0, (select intPrimitive from SupportBean#firstunique(theString) where "
                    + "theString = s0.p00) as c1 from SupportBean_S0 as s0";
    private static final String EPL_TIME_UNIQUE =
            "@name('s0') select id as c0, (select intPrimitive from SupportBean#time(1)#unique(theString) "
                    + "where theString = s0.p00) as c1 from SupportBean_S0 as s0";
    private static final String EPL_GROUPWIN_UNIQUE =
            "@name('s0') select id as c0, (select longPrimitive from SupportBean#groupwin(theString)"
                    + "#unique(intPrimitive) where theString = s0.p00 and intPrimitive = s0.id) as c1 "
                    + "from SupportBean_S0 as s0";

    // Case index-choices-overdefined-where: the 19 cycle EPLs in regression
    // order, mirroring the Java tryAssertion template (hook annotation
    // dropped, hint kept on cycle 14).
    private static final String[] EPL_INDEX_CHOICES = {
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(s2,i2) as ssb2 ) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.i2 = ssb1.i1 and ssb2.d2 = ssb1.d1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.d2 = ssb1.d1 and ssb2.i2 = ssb1.i1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.d2 = ssb1.d1 and ssb2.i2 = ssb1.i1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.i2 = ssb1.i1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.d2 = ssb1.d1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.i2 = ssb1.i1 and ssb2.d2 = ssb1.d1 and ssb2.l2 between 1 and 1000) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,i2) as ssb2 where ssb2.d2 = ssb1.d1 and ssb2.l2 between 1 and 1000) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(i2,d2,l2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.d2 = ssb1.d1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(i2,d2,l2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.i2 = ssb1.i1 and ssb2.d2 = ssb1.d1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,l2,i2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.i2 = ssb1.i1 and ssb2.d2 = ssb1.d1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(d2,l2,i2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb2.i2 = ssb1.i1 and ssb2.d2 = ssb1.d1 and ssb2.s2 between 'E3' and 'E4') as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(l2) as ssb2 where ssb2.l2 = ssb1.l1) as c1 from SupportSimpleBeanOne as ssb1",
            "@Hint('DISABLE_UNIQUE_IMPLICIT_IDX')@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(l2) as ssb2 where ssb2.l2 = ssb1.l1) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(l2) as ssb2 where ssb2.l2 = ssb1.l1 and ssb1.i1 between 1 and 20) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(s2) as ssb2 where ssb1.i1 > ssb2.i2) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(s2) as ssb2 where ssb1.i1 >= ssb2.i2) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(s2) as ssb2 where ssb1.i1 < ssb2.i2) as c1 from SupportSimpleBeanOne as ssb1",
            "@name('s0') select s1 as c0, (select s2 from SupportSimpleBeanTwo#unique(s2) as ssb2 where ssb1.i1 <= ssb2.i2) as c1 from SupportSimpleBeanOne as ssb1",
    };

    private EPLSubselectOrderOfEvalIndexScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLSubselectOrderOfEvalIndexScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType("SupportTradeEventTwo", TradeEventTwoMirror.class);
        configuration.getCommon().addEventType("SupportSimpleBeanOne", SimpleBeanOneMirror.class);
        configuration.getCommon().addEventType("SupportSimpleBeanTwo", SimpleBeanTwoMirror.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_ORDER, runtime, allSteps, records);
            runCase(CASE_PREEVAL, runtime, allSteps, records);
            runCase(CASE_INDEX_CHOICES, runtime, allSteps, records);
            runCase(CASE_UNIQUE_CORRELATED, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
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

    /** Replays one case's steps on the shared runtime; sequence runs continuously across deploy cycles. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
        int[] seq = new int[] {0};
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (string(step, "op")) {
                case "deploy": {
                    deployEpl(runtime, caseName, string(step, "epl"), seq, records);
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
    }

    /**
     * Deploys one scenario EPL. The correlated-subquery-order module carries
     * two statements separated by a semicolon-newline: the unnamed unheard
     * seed and the listened s0.
     */
    private static void deployEpl(EPRuntime runtime, String caseName, String epl, int[] seq,
                                  JsonArray records) throws Exception {
        if (CASE_ORDER.equals(caseName)) {
            String[] parts = epl.split(";\n", -1);
            if (parts.length != 2) {
                throw new IllegalStateException("order module must have two statements");
            }
            compileDeploy(runtime, parts[0]);
            EPDeployment deployment = compileDeploy(runtime, parts[1]);
            attachListener(deployment, caseName, seq, records);
            return;
        }
        EPDeployment deployment = compileDeploy(runtime, epl);
        attachListener(deployment, caseName, seq, records);
    }

    private static EPCompiled compile(EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment compileDeploy(EPRuntime runtime, String epl) throws Exception {
        return runtime.getDeploymentService().deploy(compile(runtime, epl), new DeploymentOptions());
    }

    private static void attachListener(EPDeployment deployment, String caseName, int[] seq,
                                       JsonArray records) {
        for (EPStatement statement : deployment.getStatements()) {
            if (LISTENED_STATEMENT.equals(statement.getName())) {
                statement.addListener(listener(caseName, seq, records));
            }
        }
    }

    /** Listener emitting one record per new row; an old stream is a drift failure. */
    private static UpdateListener listener(String caseName, int[] seq, JsonArray records) {
        return (newEvents, oldEvents, statement, runtime) -> {
            if (oldEvents != null && oldEvents.length > 0) {
                throw new IllegalStateException("unexpected old stream for " + caseName + "/"
                        + statement.getName());
            }
            if (newEvents == null) {
                return;
            }
            for (EventBean event : newEvents) {
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("sequence", seq[0]);
                record.add("time",
                        Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                JsonArray newRows = new JsonArray();
                newRows.add(row(event));
                record.add("new", newRows);
                records.add(record);
            }
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization with array support: window(x.*) columns arrive as
     * EventBean[] (or Object[] of underlyings) and render element-wise as
     * nested rows; integral numbers as JSON numbers, other numbers as
     * doubles, booleans, and null as the tagged {"state":"null"} object.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean element : (EventBean[]) value) {
                array.add(row(element));
            }
            return array;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object element : (Object[]) value) {
                array.add(normalize(element));
            }
            return array;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value instanceof TradeEventTwoMirror) {
            TradeEventTwoMirror bean = (TradeEventTwoMirror) value;
            JsonObject fields = new JsonObject();
            fields.add("price", normalize(bean.getPrice()));
            fields.add("securityID", normalize(bean.getSecurityID()));
            fields.add("time", normalize(bean.getTime()));
            fields.add("volume", normalize(bean.getVolume()));
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            item.add("fields", fields);
            return item;
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
        return Json.value(String.valueOf(value));
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean(
                        string(payload, "theString"), integer(payload, "intPrimitive"));
                JsonValue longPrimitive = payload.get("longPrimitive");
                if (longPrimitive != null) {
                    bean.setLongPrimitive(longPrimitive.asLong());
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"), string(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportTradeEventTwo": {
                TradeEventTwoMirror bean = new TradeEventTwoMirror(
                        payload.getLong("time", 0), integer(payload, "securityID"),
                        payload.getDouble("price", Double.NaN), payload.getLong("volume", 0));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportSimpleBeanOne": {
                JsonValue d1 = payload.get("d1");
                SimpleBeanOneMirror bean;
                if (d1 == null) {
                    bean = new SimpleBeanOneMirror(string(payload, "s1"), integer(payload, "i1"));
                } else {
                    bean = new SimpleBeanOneMirror(string(payload, "s1"), integer(payload, "i1"),
                            payload.getDouble("d1", Double.NaN), payload.getLong("l1", 0));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportSimpleBeanTwo": {
                JsonValue d2 = payload.get("d2");
                SimpleBeanTwoMirror bean;
                if (d2 == null) {
                    bean = new SimpleBeanTwoMirror(string(payload, "s2"), integer(payload, "i2"));
                } else {
                    bean = new SimpleBeanTwoMirror(string(payload, "s2"), integer(payload, "i2"),
                            payload.getDouble("d2", Double.NaN), payload.getLong("l2", 0));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");


        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        String[] expectedCases = {CASE_ORDER, CASE_PREEVAL, CASE_INDEX_CHOICES, CASE_UNIQUE_CORRELATED};
        int[] expectedOrdinals = {0, 1, 0, 1};
        String[] expectedEpls = {
                EPL_MODULE_ORDER.substring(EPL_MODULE_ORDER.indexOf(';') + 2),
                EPL_PREEVAL_ONE,
                EPL_INDEX_CHOICES[0],
                EPL_UNIQUE,
        };
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !expectedEpls[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateOrderCase(steps, offset);
        offset = validatePreevalCase(steps, offset);
        offset = validateIndexChoicesCase(steps, offset);
        offset = validateUniqueCorrelatedCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateOrderCase(JsonArray steps, int offset) {
        String caseName = CASE_ORDER;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_MODULE_ORDER);
        validateTradeSend(steps.get(offset++), caseName, 1000, 1000, 50.0, 1);
        validateTradeSend(steps.get(offset++), caseName, 1010, 1000, 50.0, 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validatePreevalCase(JsonArray steps, int offset) {
        String caseName = CASE_PREEVAL;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_PREEVAL_ONE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 5, false);
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_PREEVAL_TWO);
        validateBeanSend(steps.get(offset++), caseName, "E1", 5, false);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateIndexChoicesCase(JsonArray steps, int offset) {
        String caseName = CASE_INDEX_CHOICES;
        validateCaseMarker(steps.get(offset++), caseName);
        for (int cycle = 0; cycle < EPL_INDEX_CHOICES.length; cycle++) {
            validateDeploy(steps.get(offset++), caseName, "s0", EPL_INDEX_CHOICES[cycle]);
            if (cycle == 0) {
                validateTwoSend(steps.get(offset++), caseName, "E1", 1, 2.0, 3);
                validateOneSendFull(steps.get(offset++), caseName, "EX", 10, 11.0, 12);
                validateTwoSend(steps.get(offset++), caseName, "E2", 1, 2.0, 3);
                validateOneSendFull(steps.get(offset++), caseName, "EY", 10, 11.0, 12);
            } else if (cycle < 15) {
                validateTwoSend(steps.get(offset++), caseName, "E1", 1, 3.0, 10);
                validateTwoSend(steps.get(offset++), caseName, "E2", 1, 2.0, 0);
                validateTwoSend(steps.get(offset++), caseName, "E3", 1, 3.0, 9);
                validateOneSendFull(steps.get(offset++), caseName, "EX", 1, 3.0, 9);
            } else {
                int[] i2s = {1, 2, 3, 4, 5};
                switch (cycle) {
                    case 15:
                        validateTwoSend(steps.get(offset++), caseName, "E1", 1);
                        validateTwoSend(steps.get(offset++), caseName, "E2", 2);
                        break;
                    case 16:
                        validateTwoSend(steps.get(offset++), caseName, "E1", 2);
                        validateTwoSend(steps.get(offset++), caseName, "E2", 4);
                        break;
                    case 17:
                        validateTwoSend(steps.get(offset++), caseName, "E1", 2);
                        validateTwoSend(steps.get(offset++), caseName, "E2", 3);
                        break;
                    default:
                        validateTwoSend(steps.get(offset++), caseName, "E1", 1);
                        validateTwoSend(steps.get(offset++), caseName, "E2", 3);
                        break;
                }
                String[] s1s = {"A", "B", "C", "D", "E"};
                for (int index = 0; index < s1s.length; index++) {
                    validateOneSend(steps.get(offset++), caseName, s1s[index], i2s[index]);
                }
            }
            validateUndeployAll(steps.get(offset++), caseName);
        }
        return offset;
    }

    private static int validateUniqueCorrelatedCase(JsonArray steps, int offset) {
        String caseName = CASE_UNIQUE_CORRELATED;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_UNIQUE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, false);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2, false);
        validateBeanSend(steps.get(offset++), caseName, "E1", 3, false);
        validateBeanSend(steps.get(offset++), caseName, "E2", 4, false);
        validateS0Send(steps.get(offset++), caseName, 10, "E2");
        validateS0Send(steps.get(offset++), caseName, 11, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_FIRST_UNIQUE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, false);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2, false);
        validateBeanSend(steps.get(offset++), caseName, "E1", 3, false);
        validateBeanSend(steps.get(offset++), caseName, "E2", 4, false);
        validateS0Send(steps.get(offset++), caseName, 10, "E2");
        validateS0Send(steps.get(offset++), caseName, 11, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_TIME_UNIQUE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, false);
        validateBeanSend(steps.get(offset++), caseName, "E1", 2, false);
        validateBeanSend(steps.get(offset++), caseName, "E1", 3, false);
        validateBeanSend(steps.get(offset++), caseName, "E2", 4, false);
        validateS0Send(steps.get(offset++), caseName, 10, "E2");
        validateS0Send(steps.get(offset++), caseName, 11, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_GROUPWIN_UNIQUE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, true);
        validateBeanSend(steps.get(offset++), caseName, "E1", 2, true);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, true);
        validateS0Send(steps.get(offset++), caseName, 1, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be an array");
        }
        JsonArray array = value.asArray();
        if (array.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!expected[index].equals(array.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned");
            }
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static void validateTradeSend(JsonValue value, String caseName, long expectedTime,
                                          int expectedSecurityID, double expectedPrice,
                                          long expectedVolume) {
        JsonObject step = object(value, "SupportTradeEventTwo step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportTradeEventTwo".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportTradeEventTwo step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportTradeEventTwo payload");
        requireFields(payload, "time", "securityID", "price", "volume");
        if (payload.getLong("time", -1) != expectedTime
                || integer(payload, "securityID") != expectedSecurityID
                || payload.getDouble("price", Double.NaN) != expectedPrice
                || payload.getLong("volume", -1) != expectedVolume) {
            throw new IllegalArgumentException("SupportTradeEventTwo payload is not pinned for " + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         int expectedInt, boolean withLongPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (withLongPrimitive) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        } else {
            requireFields(payload, "theString", "intPrimitive");
        }
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, int expectedId,
                                       String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (integer(payload, "id") != expectedId || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    private static void validateTwoSend(JsonValue value, String caseName, String s2, int i2) {
        validateTwoSendFull(value, caseName, s2, i2, null, null);
    }

    private static void validateTwoSend(JsonValue value, String caseName, String s2, int i2,
                                        double d2, long l2) {
        validateTwoSendFull(value, caseName, s2, i2, d2, l2);
    }

    private static void validateTwoSendFull(JsonValue value, String caseName, String s2, int i2,
                                            Double d2, Long l2) {
        JsonObject step = object(value, "SupportSimpleBeanTwo step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportSimpleBeanTwo".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportSimpleBeanTwo step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportSimpleBeanTwo payload");
        if (d2 == null) {
            requireFields(payload, "s2", "i2");
        } else {
            requireFields(payload, "s2", "i2", "d2", "l2");
            if (payload.getDouble("d2", Double.NaN) != d2
                    || payload.getLong("l2", -1) != l2) {
                throw new IllegalArgumentException("SupportSimpleBeanTwo payload is not pinned for "
                        + caseName);
            }
        }
        if (!s2.equals(string(payload, "s2")) || integer(payload, "i2") != i2) {
            throw new IllegalArgumentException("SupportSimpleBeanTwo payload is not pinned for " + caseName);
        }
    }

    private static void validateOneSend(JsonValue value, String caseName, String s1, int i1) {
        JsonObject step = object(value, "SupportSimpleBeanOne step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportSimpleBeanOne".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportSimpleBeanOne step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportSimpleBeanOne payload");
        requireFields(payload, "s1", "i1");
        if (!s1.equals(string(payload, "s1")) || integer(payload, "i1") != i1) {
            throw new IllegalArgumentException("SupportSimpleBeanOne payload is not pinned for " + caseName);
        }
    }

    private static void validateOneSendFull(JsonValue value, String caseName, String s1, int i1,
                                            double d1, long l1) {
        JsonObject step = object(value, "SupportSimpleBeanOne step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportSimpleBeanOne".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportSimpleBeanOne step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportSimpleBeanOne payload");
        requireFields(payload, "s1", "i1", "d1", "l1");
        if (!s1.equals(string(payload, "s1")) || integer(payload, "i1") != i1
                || payload.getDouble("d1", Double.NaN) != d1 || payload.getLong("l1", -1) != l1) {
            throw new IllegalArgumentException("SupportSimpleBeanOne payload is not pinned for " + caseName);
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            List<String> names = new ArrayList<>();
            for (Member member : value.asObject()) {
                if (names.contains(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
                }
                names.add(member.getName());
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object.size() != expectedNames.length
                || !new ArrayList<>(object.names()).containsAll(Arrays.asList(expectedNames))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be a number");
        }
        return Integer.parseInt(value.toString());
    }

    /** Rethrows statement exceptions out of the runtime, mirroring the harness. */
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

    /** Local mirror of the regression-lib SupportTradeEventTwo bean. */
    public static class TradeEventTwoMirror {
        private long time;
        private int securityID;
        private double price;
        private long volume;

        public TradeEventTwoMirror(long time, int securityID, double price, long volume) {
            this.time = time;
            this.securityID = securityID;
            this.price = price;
            this.volume = volume;
        }

        public long getTime() {
            return time;
        }

        public int getSecurityID() {
            return securityID;
        }

        public double getPrice() {
            return price;
        }

        public long getVolume() {
            return volume;
        }
    }

    /** Local mirror of the regression-lib SupportSimpleBeanOne bean. */
    public static class SimpleBeanOneMirror {
        private String s1;
        private int i1;
        private double d1;
        private long l1;

        public SimpleBeanOneMirror(String s1, int i1) {
            this(s1, i1, 0.0, 0);
        }

        public SimpleBeanOneMirror(String s1, int i1, double d1, long l1) {
            this.s1 = s1;
            this.i1 = i1;
            this.d1 = d1;
            this.l1 = l1;
        }

        public String getS1() {
            return s1;
        }

        public int getI1() {
            return i1;
        }

        public double getD1() {
            return d1;
        }

        public long getL1() {
            return l1;
        }
    }

    /** Local mirror of the regression-lib SupportSimpleBeanTwo bean. */
    public static class SimpleBeanTwoMirror {
        private String s2;
        private int i2;
        private double d2;
        private long l2;

        public SimpleBeanTwoMirror(String s2, int i2) {
            this(s2, i2, 0.0, 0);
        }

        public SimpleBeanTwoMirror(String s2, int i2, double d2, long l2) {
            this.s2 = s2;
            this.i2 = i2;
            this.d2 = d2;
            this.l2 = l2;
        }

        public String getS2() {
            return s2;
        }

        public int getI2() {
            return i2;
        }

        public double getD2() {
            return d2;
        }

        public long getL2() {
            return l2;
        }
    }
}
