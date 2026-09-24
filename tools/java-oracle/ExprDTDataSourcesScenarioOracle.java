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
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.TreeSet;

/**
 * Java oracle for ExprDTDataSources' four executions.  Four cases replay on
 * fresh runtimes:
 *
 * minmax (ordinal 3, ExprDTDataSourcesMinMax): windowed min/max aggregates
 * and the per-row min(a,b) row function serve as interval endpoints for
 * before(b, 1 second) — delta in [1000, MAX] inclusive — over
 * SupportBean#length(2).  Sends (20000,20000) and (19000,20000) emit
 * {false,false} and {true,true}.
 *
 * all-combinations (ordinal 2, ExprDTDataSourcesAllCombinations): five
 * deploys, one per SupportDateTime field, each selecting the eleven
 * calendar getters with space-separated aliases.  The zoneddate/localdate
 * deploys observe the java.time asymmetries: 1-based getMonthValue and the
 * DayOfWeek enum (THURSDAY) where the other three representations read
 * 0-based Calendar.MONTH and the Calendar.DAY_OF_WEEK int.
 *
 * field-w-value (ordinal 1, ExprDTDataSourcesFieldWValue): advanceTime pins
 * engine time to the event instant; one 16-column select reads the eleven
 * getters off current_timestamp plus gethourOfDay off all five fields.  A
 * types step pins the all-Integer property-type assertion.
 *
 * start-end-ts (ordinal 0, ExprDTDataSourcesStartEndTS): compile-only.  Map
 * and object-array schema inheritance deploys assert the declared
 * startTS/endTS timestamp property names through a types record; the POJO
 * and XML sub-tests verify the same metadata then pin the unrepresentable
 * note (no Go boundary); three tryInvalidCompile probes pin the Java
 * message prefixes.
 */
public final class ExprDTDataSourcesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-dt-data-sources";
    private static final String DESCRIPTION = "ExprDTDataSources executions: minmax replays "
            + "ExprDTDataSourcesMinMax (windowed min/max and per-row min(a,b) as before(b,1 second) "
            + "interval endpoints over SupportBean#length(2); sends (20000,20000)->{false,false} and "
            + "(19000,20000)->{true,true}), all-combinations replays ExprDTDataSourcesAllCombinations "
            + "(five per-field deploys x eleven getters; zoneddate/localdate pin the java.time "
            + "asymmetries month=5 and DayOfWeek=THURSDAY), field-w-value replays "
            + "ExprDTDataSourcesFieldWValue (advanceTime(2002-05-30T09:01:02.003); 16 Integer-typed "
            + "columns over current_timestamp and the five fields), and start-end-ts replays "
            + "ExprDTDataSourcesStartEndTS compile-only (map/object-array schema inheritance asserts "
            + "startTS/endTS timestamp property names; POJO/XML sub-tests unrepresentable; three "
            + "tryInvalidCompile probes pin the Java prefixes).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/"
                    + "ExprDTDataSources.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-fb5317848fcedbabf016",
            "java-runtime-e1d8bce93d3743e9137f",
            "java-runtime-0ee5536b7a3d3d75f2eb",
            "java-runtime-ab467628bb158bf8ce6c"
    };
    private static final String[] EXECUTION_NAMES = {
            "ExprDTDataSourcesMinMax",
            "ExprDTDataSourcesAllCombinations",
            "ExprDTDataSourcesFieldWValue",
            "ExprDTDataSourcesStartEndTS"
    };
    private static final String[] STATIC_IDS = {
            "java-4f45a77a1aa86f1e7bbb",
            "java-69706afe80bb8912e34a",
            "java-f0520b6dd47cc7a13a1b",
            "java-b828fb8eddc4facd6116"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "minmax",
            "all-combinations",
            "field-w-value",
            "start-end-ts"
    };
    private static final int[] ORDINALS = {3, 2, 1, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; deploy s0 over SupportBean#length(2): send (20000,20000) emits "
            + "{c0:false,c1:false} (delta 0 < 1000), send (19000,20000) emits {c0:true,c1:true} "
            + "(delta 1000 inside [1000, MAX] inclusive)",
            "listener; five deploys (utildate,longdate,caldate,zoneddate,localdate) each send "
            + "SupportDateTime.make(2002-05-30T09:01:02.003) with caldate ms reset to 3 and emit "
            + "c0..c10 = {1,4|5,30,5|THURSDAY,150,1,9,3,2,22,2002} — the java.time fields read "
            + "1-based month and the DayOfWeek enum name",
            "listener+types; advanceTime(2002-05-30T09:01:02.003), the types step pins all 16 "
            + "columns Integer-typed, then one SupportDateTime send emits "
            + "{1,4,30,5,150,1,9,3,2,22,2002,9,9,9,9,9}",
            "types+unrepresentable+compile-error; map and object-array schema inheritance deploys "
            + "assert startTS/endTS timestamp property names, the POJO/XML sub-tests pin "
            + "unrepresentable notes, and three tryInvalidCompile probes record the pinned "
            + "Java prefixes"
    };

    private static final String MINMAX_EPL =
            "@name('s0') select "
            + "min(longPrimitive).before(max(longBoxed), 1 second) as c0,"
            + "min(longPrimitive, longBoxed).before(20000L, 1 second) as c1"
            + " from SupportBean#length(2)";
    private static final String FIELD_W_VALUE_EPL =
            "@name('s0') select "
            + "current_timestamp.getMinuteOfHour() as valmoh,"
            + "current_timestamp.getMonthOfYear() as valmoy,"
            + "current_timestamp.getDayOfMonth() as valdom,"
            + "current_timestamp.getDayOfWeek() as valdow,"
            + "current_timestamp.getDayOfYear() as valdoy,"
            + "current_timestamp.getEra() as valera,"
            + "current_timestamp.gethourOfDay() as valhod,"
            + "current_timestamp.getmillisOfSecond()  as valmos,"
            + "current_timestamp.getsecondOfMinute() as valsom,"
            + "current_timestamp.getweekyear() as valwye,"
            + "current_timestamp.getyear() as valyea,"
            + "utildate.gethourOfDay() as val1,"
            + "longdate.gethourOfDay() as val2,"
            + "caldate.gethourOfDay() as val3,"
            + "zoneddate.gethourOfDay() as val4,"
            + "localdate.gethourOfDay() as val5"
            + " from SupportDateTime";
    private static final String SELECT_EPL =
            "@name('s0') select * from ChildType dt where dt.before(current_timestamp())";
    private static final String SELECT_POJO_EPL =
            "@name('s2') select * from DerivedType dt where dt.before(current_timestamp())";
    private static final String SELECT_XML_EPL =
            "@name('s2') select * from MyXMLEvent dt where dt.before(current_timestamp())";
    private static final String SCHEMA_MAP_EPL =
            "@buseventtype @public create schema ParentType as (startTS long, endTS long) "
            + "starttimestamp startTS endtimestamp endTS;\n"
            + "@buseventtype @public create schema ChildType as (foo string) inherits ParentType;\n";
    private static final String SCHEMA_OBJECTARRAY_EPL =
            "@buseventtype @public create objectarray schema ParentType as (startTS long, endTS long) "
            + "starttimestamp startTS endtimestamp endTS;\n"
            + "@buseventtype @public create objectarray schema ChildType as (foo string) "
            + "inherits ParentType;\n";
    private static final String SCHEMA_POJO_EPL =
            "@public @buseventtype create schema InterfaceType as "
            + "com.espertech.esper.regressionlib.support.bean.SupportStartTSEndTSInterface "
            + "starttimestamp startTS endtimestamp endTS;\n"
            + "@public @buseventtype create schema DerivedType as "
            + "com.espertech.esper.regressionlib.support.bean.SupportStartTSEndTSImpl "
            + "inherits InterfaceType";
    private static final String SCHEMA_XML_EPL =
            "@XMLSchema(rootElementName='root', schemaText='') "
            + "@XMLSchemaField(name='startTS', xpath='/abc', type='string', castToType='long')"
            + "@XMLSchemaField(name='endTS', xpath='/def', type='string', castToType='long')"
            + "@public @buseventtype create xml schema MyXMLEvent() "
            + "starttimestamp startTS endtimestamp endTS;\n";
    private static final String SCHEMA_INCOMPATIBLE_EPL =
            "@public @buseventtype create schema T1 as (startTS long, endTS long) "
            + "starttimestamp startTS endtimestamp endTS;\n"
            + "@public @buseventtype create schema T2 as (startTSOne long, endTSOne long) "
            + "starttimestamp startTSOne endtimestamp endTSOne;\n";

    private static final String[] GETTER_METHODS = {
            "getMinuteOfHour", "getMonthOfYear", "getDayOfMonth", "getDayOfWeek",
            "getDayOfYear", "getEra", "gethourOfDay", "getmillisOfSecond",
            "getsecondOfMinute", "getweekyear", "getyear"
    };
    private static final String[] DATETIME_FIELDS = {
            "utildate", "longdate", "caldate", "zoneddate", "localdate"
    };
    private static final String DATE_T = "2002-05-30T09:01:02.003Z";

    private static String allCombinationsEPL(String field) {
        StringBuilder epl = new StringBuilder("@name('s0') select ");
        for (int index = 0; index < GETTER_METHODS.length; index++) {
            if (index > 0) {
                epl.append(",");
            }
            epl.append(field).append(".").append(GETTER_METHODS[index])
                    .append("() c").append(index);
        }
        epl.append(" from SupportDateTime");
        return epl.toString();
    }

    private static final Map<String, String> DEPLOY_EPLS = new HashMap<>();
    static {
        DEPLOY_EPLS.put("s0", MINMAX_EPL);
        DEPLOY_EPLS.put("field-w-value", FIELD_W_VALUE_EPL);
        DEPLOY_EPLS.put("schema-map", SCHEMA_MAP_EPL);
        DEPLOY_EPLS.put("schema-objectarray", SCHEMA_OBJECTARRAY_EPL);
        DEPLOY_EPLS.put("schema-pojo", SCHEMA_POJO_EPL);
        DEPLOY_EPLS.put("schema-xml", SCHEMA_XML_EPL);
        DEPLOY_EPLS.put("schema-incompatible", SCHEMA_INCOMPATIBLE_EPL);
        DEPLOY_EPLS.put("select-map", SELECT_EPL);
        DEPLOY_EPLS.put("select-objectarray", SELECT_EPL);
        DEPLOY_EPLS.put("select-pojo", SELECT_POJO_EPL);
        DEPLOY_EPLS.put("select-xml", SELECT_XML_EPL);
        for (String field : DATETIME_FIELDS) {
            DEPLOY_EPLS.put(field, allCombinationsEPL(field));
        }
    }

    private static final String[] CASE_EPLS = {
            MINMAX_EPL,
            allCombinationsEPL("utildate"),
            FIELD_W_VALUE_EPL,
            SELECT_EPL
    };

    private static final String[][] PROBES = {
            {"inherits-conflict-start",
                    "create schema T12 as () inherits T1,T2",
                    "Event type declares start timestamp as property 'startTS' however inherited "
                    + "event type 'T2' declares start timestamp as property 'startTSOne'"},
            {"inherits-conflict-end",
                    "create schema T12 as (startTSOne long, endTSXXX long) inherits T2 "
                    + "starttimestamp startTSOne endtimestamp endTSXXX",
                    "Event type declares end timestamp as property 'endTSXXX' however inherited "
                    + "event type 'T2' declares end timestamp as property 'endTSOne'"},
            {"null-start-ts-type",
                    "create schema T12 as (startTSOne null, endTSXXX long) "
                    + "starttimestamp startTSOne endtimestamp endTSXXX",
                    "Declared start timestamp property 'startTSOne' is expected to return a Date, "
                    + "Calendar or long-typed value but returns 'null'"}
    };

    private static final Map<String, String> UNREPRESENTABLE_NOTES = new HashMap<>();
    static {
        UNREPRESENTABLE_NOTES.put("pojo-inheritance",
                "POJO create-schema inheritance: create schema InterfaceType as <interface> "
                + "starttimestamp startTS endtimestamp endTS with DerivedType inherits "
                + "InterfaceType compiles and declares startTS/endTS; the typed Go surface has "
                + "no class-reference schema boundary");
        UNREPRESENTABLE_NOTES.put("xml-inheritance",
                "XML create-schema: @XMLSchemaField string xpath properties with "
                + "castToType='long' accepted as starttimestamp/endtimestamp on create xml "
                + "schema; the Go XML schema surface has no xpath-cast timestamp boundary");
    }

    /** Maps unrepresentable step labels to the deployed select statement. */
    private static final Map<String, String> UNREPRESENTABLE_STATEMENTS = new HashMap<>();
    static {
        UNREPRESENTABLE_STATEMENTS.put("pojo-inheritance", "select-pojo");
        UNREPRESENTABLE_STATEMENTS.put("xml-inheritance", "select-xml");
    }

    private static final String[] FIELD_W_VALUE_COLUMNS = {
            "valmoh", "valmoy", "valdom", "valdow", "valdoy", "valera",
            "valhod", "valmos", "valsom", "valwye", "valyea",
            "val1", "val2", "val3", "val4", "val5"
    };

    private static final int EXPECTED_STEPS = 49;
    private static final int EXPECTED_RECORDS = 16;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|at.  Deploy steps
     * carry the byte-exact EPL text; send steps carry the compacted payload
     * including the expected column values; build-error steps pin the Java
     * message prefix; unrepresentable steps pin the note.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("minmax", new String[]{
                "deploy|minmax|s0||" + MINMAX_EPL + "|||",
                "send|minmax||SupportBean||{\"longPrimitive\":20000,\"longBoxed\":20000,"
                        + "\"expected\":[false,false]}||",
                "send|minmax||SupportBean||{\"longPrimitive\":19000,\"longBoxed\":20000,"
                        + "\"expected\":[true,true]}||",
                "undeploy-all|minmax||||||",
        });
        List<String> allCombinations = new ArrayList<>();
        for (String field : DATETIME_FIELDS) {
            boolean java8 = field.equals("zoneddate") || field.equals("localdate");
            String month = java8 ? "5" : "4";
            String dow = java8 ? "\"THURSDAY\"" : "5";
            allCombinations.add("deploy|all-combinations|" + field + "||"
                    + allCombinationsEPL(field) + "|||");
            allCombinations.add("send|all-combinations||SupportDateTime||{\"date\":\"" + DATE_T
                    + "\",\"expected\":[1," + month + ",30," + dow + ",150,1,9,3,2,22,2002]}||");
            allCombinations.add("undeploy-all|all-combinations||||||");
        }
        CASE_STEPS.put("all-combinations", allCombinations.toArray(new String[0]));
        CASE_STEPS.put("field-w-value", new String[]{
                "advance-time|field-w-value||||||" + DATE_T,
                "deploy|field-w-value|field-w-value||" + FIELD_W_VALUE_EPL + "|||",
                "types|field-w-value|s0|||||",
                "send|field-w-value||SupportDateTime||{\"date\":\"" + DATE_T
                        + "\",\"expected\":[1,4,30,5,150,1,9,3,2,22,2002,9,9,9,9,9]}||",
                "undeploy-all|field-w-value||||||",
        });
        CASE_STEPS.put("start-end-ts", new String[]{
                "deploy|start-end-ts|schema-map||" + SCHEMA_MAP_EPL + "|||",
                "deploy|start-end-ts|select-map||" + SELECT_EPL + "|||",
                "types|start-end-ts|select-map|||||",
                "undeploy-all|start-end-ts||||||",
                "deploy|start-end-ts|schema-objectarray||" + SCHEMA_OBJECTARRAY_EPL + "|||",
                "deploy|start-end-ts|select-objectarray||" + SELECT_EPL + "|||",
                "types|start-end-ts|select-objectarray|||||",
                "undeploy-all|start-end-ts||||||",
                "deploy|start-end-ts|schema-pojo||" + SCHEMA_POJO_EPL + "|||",
                "deploy|start-end-ts|select-pojo||" + SELECT_POJO_EPL + "|||",
                "unrepresentable|start-end-ts|pojo-inheritance||||"
                        + UNREPRESENTABLE_NOTES.get("pojo-inheritance") + "|",
                "undeploy-all|start-end-ts||||||",
                "deploy|start-end-ts|schema-xml||" + SCHEMA_XML_EPL + "|||",
                "deploy|start-end-ts|select-xml||" + SELECT_XML_EPL + "|||",
                "unrepresentable|start-end-ts|xml-inheritance||||"
                        + UNREPRESENTABLE_NOTES.get("xml-inheritance") + "|",
                "undeploy-all|start-end-ts||||||",
                "deploy|start-end-ts|schema-incompatible||" + SCHEMA_INCOMPATIBLE_EPL + "|||",
                "build-error|start-end-ts|" + PROBES[0][0] + "||" + PROBES[0][1]
                        + "||" + PROBES[0][2] + "|",
                "build-error|start-end-ts|" + PROBES[1][0] + "||" + PROBES[1][1]
                        + "||" + PROBES[1][2] + "|",
                "build-error|start-end-ts|" + PROBES[2][0] + "||" + PROBES[2][1]
                        + "||" + PROBES[2][2] + "|",
                "undeploy-all|start-end-ts||||||",
        });
    }

    private ExprDTDataSourcesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDTDataSourcesScenarioOracle <scenario.json>");
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
     * Replays the case's steps on a fresh runtime.  Deploy steps compile
     * with the accumulated module path (env.compileDeploy(epl, path));
     * undeploy-all clears the path like RegressionPath.clear().  Select
     * deploys attach the trace listener for the event-producing cases and
     * record the deployed statement for the types/unrepresentable checks.
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
            List<EPCompiled> deployedModules = new ArrayList<>();
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
                    case "advance-time": {
                        long atMillis = Instant.parse(string(step, "at")).toEpochMilli();
                        runtime.getEventService().advanceTime(atMillis);
                        break;
                    }
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, listener,
                                deployedModules, statements);
                        break;
                    case "send":
                        sendEvent(runtime, listener, caseName, step, records);
                        break;
                    case "types":
                        typesStep(caseName, step, statements, records, runtime);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, deployedModules, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(caseName, step, statements, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployedModules.clear();
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

    /**
     * Per-case event-type registration mirroring the suite: SupportBean for
     * minmax, SupportDateTime for all-combinations and field-w-value.
     * start-end-ts declares its types through the create-schema deploys.
     */
    private static Configuration configure(String caseName) {
        Configuration configuration = new Configuration();
        switch (caseName) {
            case "minmax":
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case "all-combinations":
            case "field-w-value":
                configuration.getCommon().addEventType(SupportDateTime.class);
                break;
            default:
                break;
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    /**
     * Compiles and deploys one labeled module with the accumulated path,
     * mirroring env.compileDeploy(epl, path).  Event-producing deploys
     * attach the listener to s0; start-end-ts select deploys record the
     * deployed statement under the step label for the metadata checks.
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration, String caseName,
                                   JsonObject step, ListenerRecorder listener,
                                   List<EPCompiled> deployedModules,
                                   Map<String, EPStatement> statements) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        for (EPCompiled deployed : deployedModules) {
            compilerArgs.getPath().add(deployed);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        deployedModules.add(compiled);
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName()) || "s2".equals(statement.getName())) {
                statements.put(label, statement);
                statements.put(statement.getName(), statement);
            }
            if ("s0".equals(statement.getName()) && !"start-end-ts".equals(caseName)) {
                statement.addListener(listener);
            }
        }
    }

    /**
     * Sends one event, drains the deliveries it produced, and emits exactly
     * one listener record after verifying the row against the pinned
     * expected column values — the assertPropsNew equivalent.  The
     * all-combinations send re-sets the Calendar millisecond to 3 after
     * SupportDateTime.make zeroes it.
     */
    private static void sendEvent(EPRuntime runtime, ListenerRecorder listener, String caseName,
                                  JsonObject step, JsonArray records) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setLongPrimitive(longInteger(payload.get("longPrimitive"), "longPrimitive"));
            bean.setLongBoxed(longInteger(payload.get("longBoxed"), "longBoxed"));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else if ("SupportDateTime".equals(eventType)) {
            SupportDateTime sdt = SupportDateTime.make(javaDateString(string(payload, "date")));
            if ("all-combinations".equals(caseName)) {
                sdt.getCaldate().set(Calendar.MILLISECOND, 3);
            }
            runtime.getEventService().sendEventBean(sdt, "SupportDateTime");
        } else {
            throw new IllegalStateException("unknown eventType " + eventType);
        }
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
     * expected values in select order, mirroring assertPropsNew.
     */
    private static void verifyExpected(String caseName, String eventType, JsonObject payload,
                                       JsonObject row) {
        JsonArray expected = array(payload.get("expected"), "expected");
        String[] columns;
        switch (caseName) {
            case "minmax":
                columns = new String[]{"c0", "c1"};
                break;
            case "all-combinations":
                columns = new String[GETTER_METHODS.length];
                for (int index = 0; index < columns.length; index++) {
                    columns[index] = "c" + index;
                }
                break;
            case "field-w-value":
                columns = FIELD_W_VALUE_COLUMNS;
                break;
            default:
                throw new IllegalStateException("case " + caseName + " sends no events");
        }
        if (expected.size() != columns.length) {
            throw new IllegalStateException("expected " + columns.length + " values for "
                    + eventType + ", got " + expected.size());
        }
        JsonObject fields = object(row.get("fields"), "fields");
        for (int index = 0; index < columns.length; index++) {
            JsonValue want = expected.get(index);
            JsonValue got = fields.get(columns[index]);
            if (!jsonEquals(want, got)) {
                throw new IllegalStateException("observed " + columns[index] + " drift for "
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
            return Double.compare(want.asDouble(), got.asDouble()) == 0;
        }
        return want.equals(got);
    }

    /**
     * Emits a {"operation":"types"} record carrying exactly the
     * Java-asserted event-type surface for the statement after verifying
     * it: field-w-value pins all 16 columns Integer-typed; start-end-ts
     * pins the startTS/endTS timestamp property names.
     */
    private static void typesStep(String caseName, JsonObject step,
                                  Map<String, EPStatement> statements, JsonArray records,
                                  EPRuntime runtime) {
        String label = string(step, "statement");
        JsonObject value = new JsonObject();
        if ("field-w-value".equals(caseName)) {
            EPStatement statement = statements.get(label);
            if (statement == null) {
                throw new IllegalStateException("types statement " + label
                        + " was not deployed in case " + caseName);
            }
            EventType eventType = statement.getEventType();
            JsonObject pinned = new JsonObject();
            for (String column : FIELD_W_VALUE_COLUMNS) {
                Class<?> propertyType = eventType.getPropertyType(column);
                String actual = propertyType == null ? "null" : propertyType.getSimpleName();
                if (!"Integer".equals(actual)) {
                    throw new IllegalStateException("property type drift for s0." + column
                            + ": expected Integer got " + actual);
                }
                pinned.add(column, "Integer");
            }
            value.add("properties", pinned);
        } else if ("start-end-ts".equals(caseName)) {
            EPStatement statement = statements.get(label);
            if (statement == null) {
                throw new IllegalStateException("types statement " + label
                        + " was not deployed in case " + caseName);
            }
            EventType eventType = statement.getEventType();
            String start = eventType.getStartTimestampPropertyName();
            String end = eventType.getEndTimestampPropertyName();
            if (!"startTS".equals(start) || !"endTS".equals(end)) {
                throw new IllegalStateException("timestamp property drift for " + label
                        + ": expected startTS/endTS got " + start + "/" + end);
            }
            value.add("startTimestamp", start);
            value.add("endTimestamp", end);
        } else {
            throw new IllegalStateException("case " + caseName + " has no types assertions");
        }
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
     * Compiles an expected-invalid probe with the accumulated path and
     * emits {"operation":"compile-error"} carrying the pinned expectError
     * prefix after verifying the caught message starts with it
     * (SupportMessageAssertUtil.assertMessage semantics).
     */
    private static void buildErrorStep(Configuration configuration, String caseName, JsonObject step,
                                       List<EPCompiled> deployedModules, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
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
     * Emits the pinned "unrepresentable" record for a Java sub-test with no
     * Go boundary after verifying the asserted metadata the note documents:
     * the deployed s2 statement's event type declares startTS/endTS.
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            Map<String, EPStatement> statements,
                                            JsonArray records) {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        if (!note.equals(UNREPRESENTABLE_NOTES.get(label))) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " carries an unpinned note");
        }
        String selectLabel = UNREPRESENTABLE_STATEMENTS.get(label);
        EPStatement statement = statements.get(selectLabel);
        if (statement == null) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " has no deployed statement " + selectLabel);
        }
        EventType eventType = statement.getEventType();
        String start = eventType.getStartTimestampPropertyName();
        String end = eventType.getEndTimestampPropertyName();
        if (!"startTS".equals(start) || !"endTS".equals(end)) {
            throw new IllegalStateException("timestamp property drift for " + label
                    + ": expected startTS/endTS got " + start + "/" + end);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
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
        if (value instanceof EventBean) {
            return renderRow((EventBean) value);
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
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object element : (Object[]) value) {
                array.add(normalize(element));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            JsonObject fields = new JsonObject();
            for (Object key : new TreeSet<>(((Map<?, ?>) value).keySet())) {
                fields.add(String.valueOf(key), normalize(((Map<?, ?>) value).get(key)));
            }
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            item.add("fields", fields);
            return item;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Buffers rendered rows per delivery so the send step can verify the
     * pinned expected values before emitting the listener record.
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
     * op|case|statement|eventType|epl|payload|expectError|at with the payload
     * compacted.  Unknown fields are rejected per op.
     */
    private static String stepKey(JsonObject step) {
        String operation = string(step, "op");
        Map<String, Set<String>> allowed = new HashMap<>();
        allowed.put("case", new HashSet<>(Arrays.asList("op", "case")));
        allowed.put("advance-time", new HashSet<>(Arrays.asList("op", "case", "at")));
        allowed.put("deploy", new HashSet<>(Arrays.asList("op", "case", "statement", "epl")));
        allowed.put("send", new HashSet<>(Arrays.asList("op", "case", "eventType", "payload")));
        allowed.put("types", new HashSet<>(Arrays.asList("op", "case", "statement")));
        allowed.put("build-error",
                new HashSet<>(Arrays.asList("op", "case", "statement", "epl", "expectError")));
        allowed.put("unrepresentable",
                new HashSet<>(Arrays.asList("op", "case", "statement", "expectError")));
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
