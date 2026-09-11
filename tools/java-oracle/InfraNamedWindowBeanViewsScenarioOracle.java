import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.context.util.StatementContext;
import com.espertech.esper.common.internal.event.bean.core.BeanEventType;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the infra-named-window-bean-views slice of
 * InfraNamedWindowViews.java: the ord-2 {@code InfraBeanBacked} execution that
 * runs its helper once per event-representation annotation over a window
 * declared {@code as SupportBean} (lines 298-305 with the helper at
 * 3591-3611), the ord-35 {@code InfraBeanContained} execution whose window is
 * declared {@code as (bean SupportBean_S0)} and fed by a stream-wildcard insert
 * (lines 307-318 with the helper at 3498-3509), the ord-37
 * {@code InfraBeanSchemaBacked} execution that aliases the bean class through a
 * schema and reads the window with a fire-and-forget query (lines 342-362) and
 * the ord-38 {@code InfraDeepSupertypeInsert} execution whose window is
 * declared from {@code as select * from SupportOverrideBase} and fed by a
 * subtype insert (lines 364-373).
 *
 * <p>None of the four executions moves the engine clock, so every record
 * carries the pinned start instant; one EPRuntime is created per case with the
 * clock pinned at 0 before any statement is deployed.
 *
 * <p>Deployment grouping follows the Java source, not the step fan-out: ord 2
 * deploys its four statements as four separate modules in every sub-run
 * (INV:3594/3596/3598/3605), ord 35 its window and insert as two modules per
 * sub-run (INV:3500/3503), ord 37 its schema, window and insert as ONE module
 * (INV:347-350) plus the {@code s0} consumer (INV:355), and ord 38 its window
 * and insert as one module (INV:366-368).  Joining the pinned step EPLs of one
 * module with ";\n" reproduces the source literal minus its trailing
 * terminator.
 *
 * <p>The representation matrix is replayed in full: ord 2 runs its helper four
 * times in source order (OBJECTARRAY, MAP, DEFAULT, AVRO; INV:300-303) and ord
 * 35 three times (OBJECTARRAY, MAP, DEFAULT; INV:309-313), each sub-run on a
 * fresh RegressionPath and torn down with undeployAll before the next one.  The
 * four sub-runs of ord 2 differ only by the annotation prefix of the create
 * statement, which the engine ignores for the {@code as <class>} form: the
 * window event type stays a bean type named after the window in every sub-run,
 * which the oracle checks per sub-run instead of trusting the annotation.  Ord
 * 35's AVRO variant is the compiler-reject assertion of INV:315-316: the oracle
 * compiles that EPL and requires the pinned failure message; it emits no record.
 *
 * <p>Rows: this slice's Java assertions are mostly metadata assertions.  Ord 2's
 * three listener assertions per sub-run ({@code assertEvent(event,
 * "MyWindowBB")}) check that the delivered event is a bean event of the window
 * type named MyWindowBB and never read a property, so its rows carry the
 * delivered event type name in the protocol's {@code type} field and project NO
 * field.  Ord 37's single assertion is the same metadata check on the
 * fire-and-forget row, so its one record is a {@code faf} record whose single
 * row carries {@code type=MyWindowBSB} and no fields.  Ord 35 asserts one real
 * value ({@code bean.p00 == "E1"}) and ord 38 one ({@code val == "1a"}), so
 * those rows project exactly those fields.  The oracle also checks the
 * window-type shape per sub-run (ord 2: a bean type named MyWindowBB with a
 * SupportBean underlying and NAMED_WINDOW metadata; ord 35: an Object[] window
 * for the objectarray annotation and a Map window for map/default) and that ord
 * 2's {@code s0} select is stateless, all as harness-internal expectations that
 * emit no records, exactly like the suite's {@code assertEvent} and
 * {@code assertStatelessStmt}.
 *
 * <p>Event representations: SupportBean and SupportBean_S0 are the real
 * configured classes; {@code SupportBean_A} is modelled as a map schema
 * (id:String) because the regression-lib bean is not on the oracle classpath and
 * the update trigger reads no property; SupportOverrideBase/-One/-OneA are
 * re-declared here with the same simple names, hierarchy and override chain as
 * the regression-lib beans so the deep-supertype insert and the {@code val}
 * read behave exactly like the suite's.
 *
 * <p>Record counts are pinned to the Java source and the engine's delivery
 * wiring: ord 2 emits 20 records (five per sub-run - the create and s0
 * callbacks of the insert, then the update's three deliveries to the window
 * listener, the s0 consumer and the update listener - over its four sub-runs),
 * ord 35 emits 3 (one create callback per sub-run), ord 37 emits 1 (the
 * fire-and-forget row) and ord 38 emits 1 (the window iterator state), 25 in
 * total over 57 scenario steps.
 */
public final class InfraNamedWindowBeanViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-bean-views";
    private static final String DESCRIPTION = "InfraNamedWindowViews bean slice: the bean-backed window with its representation matrix and on-trigger update, the contained-bean window fed by a stream-wildcard insert, the schema alias read through a fire-and-forget query, and the deep-supertype insert whose window reads the most-derived override (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowViews.java";

    private static final String[] CASE_NAMES = {
            "bean-backed",
            "bean-contained",
            "bean-schema-backed",
            "deep-supertype-insert"
    };
    private static final int[] ORDINALS = {2, 35, 37, 38};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f0a1da1fe931e132c21f",
            "java-runtime-fe6adc5803da60bf18f5",
            "java-runtime-f195548d023dfbde1aed",
            "java-runtime-baa0bd4ca41b9b2c5f94"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraBeanBacked",
            "InfraBeanContained",
            "InfraBeanSchemaBacked",
            "InfraDeepSupertypeInsert"
    };

    /**
     * Per-case slice description; pinned byte-exactly with the scenario and
     * the Go runner's case specs.  Every entry is a single literal line so the
     * shell script can pin the same bytes independently of this oracle.
     */
    private static final String[] CASE_DESCRIPTIONS = {
            "keepall window declared as SupportBean and replayed once per event-representation annotation: the annotation is ignored, so every sub-run delivers bean events of the window type named MyWindowBB through the create and s0 listeners and an on-trigger update that delivers the updated row as new and the pre-update row as old",
            "keepall window declared as (bean SupportBean_S0) with one sub-run per representation: the objectarray and map/default annotations shape the window underlying and the nested bean.p00 property is read through the stream-wildcard insert, while the avro variant must fail to compile",
            "keepall window declared over a schema alias of the SupportBean class: the schema type ABC is distinct from the configured SupportBean type, so the second bean send feeds only the window and the fire-and-forget query reads one row of the window type MyWindowBSB while the select over ABC stays uninvoked",
            "keepall window declared from as select * from SupportOverrideBase and fed by an insert from SupportOverrideOneA: the same underlying object is re-typed into the window and the val property reads the most-derived override"
    };

    // Transcriptions of InfraNamedWindowViews lines 3594/3596/3598/3605
    // (tryAssertionBeanBacked).  The create statement is deployed once per
    // representation with that representation's annotation prefix; the DEFAULT
    // variant's leading space (the source's "" + " @name..." concatenation) is
    // trimmed because it has no semantic effect.
    private static final String EPL_BB_CREATE =
            "@name('create') @public create window MyWindowBB#keepall as SupportBean";
    private static final String EPL_BB_CREATE_OBJECTARRAY =
            "@EventRepresentation('objectarray') @name('create') @public"
                    + " create window MyWindowBB#keepall as SupportBean";
    private static final String EPL_BB_CREATE_MAP =
            "@EventRepresentation('map') @name('create') @public"
                    + " create window MyWindowBB#keepall as SupportBean";
    private static final String EPL_BB_CREATE_AVRO =
            "@EventRepresentation('avro') @name('create') @public"
                    + " create window MyWindowBB#keepall as SupportBean";
    private static final String EPL_BB_INSERT =
            "@public insert into MyWindowBB select * from SupportBean";
    private static final String EPL_BB_S0 =
            "@name('s0') select * from MyWindowBB";
    private static final String EPL_BB_UPDATE =
            "@name('update') on SupportBean_A update MyWindowBB set theString='s'";

    // Transcriptions of InfraNamedWindowViews lines 3500/3503 and the
    // negative-compile text of 315 (tryAssertionBeanContained).  The insert is
    // unnamed and the window is a contained-bean type whose single property is
    // the SupportBean_S0 POJO.
    private static final String EPL_BC_CREATE =
            "@name('create') @public create window MyWindowBC#keepall as (bean SupportBean_S0)";
    private static final String EPL_BC_CREATE_OBJECTARRAY =
            "@EventRepresentation('objectarray') @name('create') @public"
                    + " create window MyWindowBC#keepall as (bean SupportBean_S0)";
    private static final String EPL_BC_CREATE_MAP =
            "@EventRepresentation('map') @name('create') @public"
                    + " create window MyWindowBC#keepall as (bean SupportBean_S0)";
    private static final String EPL_BC_INSERT =
            "insert into MyWindowBC select bean.* as bean from SupportBean_S0 as bean";
    private static final String EPL_BC_CREATE_AVRO =
            "@EventRepresentation('avro') @name('create')"
                    + " create window MyWindowBC#keepall as (bean SupportBean_S0)";
    private static final String EPL_BC_AVRO_ERROR =
            "Property 'bean' type 'com.espertech.esper.common.internal.support.SupportBean_S0'"
                    + " does not have a mapping to an Avro type ";

    // Transcriptions of InfraNamedWindowViews lines 347-349, 353 and 355
    // (InfraBeanSchemaBacked).  The schema statement aliases the configured
    // SupportBean class under the name ABC, which makes ABC a distinct event
    // type from SupportBean even though both wrap the same class.
    private static final String EPL_BSB_SCHEMA =
            "@public create schema ABC as com.espertech.esper.common.internal.support.SupportBean";
    private static final String EPL_BSB_CREATE =
            "@public create window MyWindowBSB#keepall as ABC";
    private static final String EPL_BSB_INSERT =
            "insert into MyWindowBSB select * from SupportBean";
    private static final String EPL_BSB_S0 =
            "@name('s0') select * from ABC";
    private static final String EPL_BSB_FAF =
            "select * from MyWindowBSB";

    // Transcriptions of InfraNamedWindowViews lines 366-367
    // (InfraDeepSupertypeInsert).  The window type is a bean replication of the
    // base class (only the val property) and the insert accepts the deeper
    // subtype, re-typing the same underlying object.
    private static final String EPL_DSI_CREATE =
            "@name('create') create window MyWindowDSI#keepall as select * from SupportOverrideBase";
    private static final String EPL_DSI_INSERT =
            "insert into MyWindowDSI select * from SupportOverrideOneA";

    private static final String[] CASE_CREATE_EPLS = {
            EPL_BB_CREATE, EPL_BC_CREATE, EPL_BSB_CREATE, EPL_DSI_CREATE
    };
    private static final String[] CASE_CREATE_OBJECTARRAY_EPLS = {
            EPL_BB_CREATE_OBJECTARRAY, EPL_BC_CREATE_OBJECTARRAY, "", ""
    };
    private static final String[] CASE_CREATE_MAP_EPLS = {
            EPL_BB_CREATE_MAP, EPL_BC_CREATE_MAP, "", ""
    };
    private static final String[] CASE_CREATE_AVRO_EPLS = {
            EPL_BB_CREATE_AVRO, "", "", ""
    };
    private static final String[] CASE_INSERT_EPLS = {
            EPL_BB_INSERT, EPL_BC_INSERT, EPL_BSB_INSERT, EPL_DSI_INSERT
    };
    private static final String[] CASE_S0_EPLS = {
            EPL_BB_S0, "", EPL_BSB_S0, ""
    };
    private static final String[] CASE_S2_EPLS = {"", "", "", ""};
    private static final String[] CASE_S3_EPLS = {"", "", "", ""};
    private static final String[] CASE_CONSUME_EPLS = {"", "", "", ""};
    private static final String[] CASE_DELETE_EPLS = {"", "", "", ""};
    private static final String[] CASE_VAR_EPLS = {"", "", "", ""};
    private static final String[] CASE_ONSET_EPLS = {"", "", "", ""};
    private static final String[] CASE_SCHEMA_EPLS = {"", "", EPL_BSB_SCHEMA, ""};
    private static final String[] CASE_UPDATE_EPLS = {EPL_BB_UPDATE, "", "", ""};
    private static final String[] CASE_FAF_EPLS = {"", "", EPL_BSB_FAF, ""};
    private static final String[] CASE_NEGATIVE_COMPILE_EPLS = {"", EPL_BC_CREATE_AVRO, "", ""};
    private static final String[][] CASE_DEPLOYS = {
            {"create", "insert", "s0", "update"},
            {"create", "insert"},
            {"schema", "create", "insert", "s0"},
            {"create", "insert"}
    };
    private static final String[][] CASE_LISTENED = {
            {"create", "s0", "update"},
            {"create"},
            {"s0"},
            {}
    };

    /**
     * Listener row projection per case and deploy position (parallel to
     * CASE_DEPLOYS; unused slots are empty).  Ord 2's three listeners assert
     * only the delivered event type, so they project no field and the record's
     * rows carry the type name instead; ord 35's create listener asserts
     * {@code bean.p00}.
     */
    private static final String[][][] CASE_LISTENER_FIELDS = {
            {
                    {}, {}, {}, {}
            },
            {
                    {"bean.p00"}, {}
            },
            {
                    {}, {}, {}, {}
            },
            {
                    {}, {}
            }
    };

    /**
     * Module grouping per deploy position: ord 2 deploys four single-statement
     * modules per sub-run, ord 35 two, ord 37 the schema/window/insert module
     * plus the consumer and ord 38 one module with both statements.
     */
    private static final int[][] CASE_MODULE_KEYS = {
            {0, 1, 2, 3},
            {0, 1},
            {0, 0, 0, 1},
            {0, 0}
    };

    /**
     * Snapshot-step count per case: only ord 38 reads an iterator
     * ({@code assertIterator("create", …)} at INV:370).
     */
    private static final int[] CASE_SNAPSHOTS = {0, 0, 0, 1};

    /**
     * Cases whose records carry the delivered event type name on every row
     * (ord 2's listener events and ord 37's fire-and-forget row): the Java
     * assertions of both are metadata-only checks on the window type.
     */
    private static final boolean[] CASE_ROW_TYPE_PINS = {true, false, true, false};

    /**
     * Listener discipline: create/s0/update for ord 2, create only for ord 35,
     * s0 only for ord 37 (never invoked, which is the point of that execution)
     * and no listener for ord 38.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    private static final Map<String, Map<String, Integer>> MODULE_KEYS;

    private static final Map<String, Map<String, String[]>> LISTENER_FIELDS;

    private static final Map<String, Boolean> ROW_TYPE_PINS;

    /**
     * Per-case record totals: five listener callbacks per sub-run over ord 2's
     * four sub-runs, one create callback per sub-run over ord 35's three
     * sub-runs, one fire-and-forget row for ord 37 and one iterator state for
     * ord 38.  Ord 2's five-per-cycle shape is the insert's create+s0 pair plus
     * the update's three deliveries: the on-trigger statement updates the
     * window root view (which drives the window's own listener with new AND old
     * and the s0 consumer with new only) before its own listener receives the
     * same pair.
     */
    private static final int[] EXPECTED_CASE_RECORDS = {20, 3, 1, 1};
    private static final int EXPECTED_RECORDS = 25;
    private static final int EXPECTED_STEPS = 57;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Map<String, String[]>> listenerFields = new HashMap<>();
        Map<String, Boolean> rowTypePins = new HashMap<>();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            listened.put(CASE_NAMES[index],
                    new HashSet<>(Arrays.asList(CASE_LISTENED[index])));
            Map<String, Integer> moduleKeys = new HashMap<>();
            Map<String, String[]> fieldsByStatement = new HashMap<>();
            for (int position = 0; position < CASE_DEPLOYS[index].length; position++) {
                String statement = CASE_DEPLOYS[index][position];
                moduleKeys.put(statement, CASE_MODULE_KEYS[index][position]);
                fieldsByStatement.put(statement, CASE_LISTENER_FIELDS[index][position]);
            }
            modules.put(CASE_NAMES[index], moduleKeys);
            listenerFields.put(CASE_NAMES[index], fieldsByStatement);
            rowTypePins.put(CASE_NAMES[index], CASE_ROW_TYPE_PINS[index]);
        }
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
        MODULE_KEYS = Collections.unmodifiableMap(modules);
        LISTENER_FIELDS = Collections.unmodifiableMap(listenerFields);
        ROW_TYPE_PINS = Collections.unmodifiableMap(rowTypePins);
    }

    /**
     * The regression-lib SupportOverrideBase hierarchy (SupportOverrideBase
     * lines 18-28, SupportOverrideOne lines 13-19, SupportOverrideOneA lines
     * 18-26): each class overrides {@code getVal()}, so the most-derived
     * override wins on virtual dispatch.  The oracle re-declares the three
     * classes because regression-lib is not on the oracle classpath; the simple
     * class names are what the suite sends and the window type derives from.
     */
    public static class SupportOverrideBase {
        private final String val;

        public SupportOverrideBase(String val) {
            this.val = val;
        }

        public String getVal() {
            return val;
        }
    }

    public static class SupportOverrideOne extends SupportOverrideBase {
        private final String valOne;

        public SupportOverrideOne(String valOne, String val) {
            super(val);
            this.valOne = valOne;
        }

        @Override
        public String getVal() {
            return valOne;
        }
    }

    public static class SupportOverrideOneA extends SupportOverrideOne {
        private final String valOneA;

        public SupportOverrideOneA(String valOneA, String valOne, String val) {
            super(valOne, val);
            this.valOneA = valOneA;
        }

        @Override
        public String getVal() {
            return valOneA;
        }
    }

    private InfraNamedWindowBeanViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowBeanViewsScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean_A", supportBeanASchema());
        configuration.getCommon().addEventType("SupportOverrideBase", SupportOverrideBase.class);
        configuration.getCommon().addEventType("SupportOverrideOne", SupportOverrideOne.class);
        configuration.getCommon().addEventType("SupportOverrideOneA", SupportOverrideOneA.class);
        // The suite's harness enables Avro (TestSuiteInfraNamedWindow line 127),
        // which is what makes ord 35's AVRO variant fail on the POJO mapping
        // rather than on the missing Avro provider.
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            int before = records.size();
            runCaseOnFreshRuntime(CASE_NAMES[index], configuration, allSteps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASE_NAMES[index] + " emitted "
                        + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
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

    /**
     * Map event type mirroring the regression-lib SupportBean_A read surface
     * (SupportBean_A lines 18-24 extends SupportBeanAtoFBase, whose single
     * property is id:String); the regression-lib bean is not on the oracle
     * classpath and ord 2's update trigger reads no property, so the trigger
     * only needs the same type name and the id column the suite's send sets.
     */
    private static Map<String, Object> supportBeanASchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("id", String.class);
        return schema;
    }

    /**
     * Replays one case on its own runtime with the clock pinned at 0.  A fresh
     * EPRuntime per case keeps that instant deterministic: none of the five
     * executions moves the clock, so without the pin the records would inherit
     * the wall clock or the previous case's position, and the scheduling
     * service assigns the clock unconditionally.  The explicit
     * {@code advanceTime(0)} before any statement is deployed cannot fire
     * anything because no statement exists yet.
     */
    private static void runCaseOnFreshRuntime(String caseName, Configuration configuration,
                                              JsonArray allSteps, JsonArray records)
            throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        try {
            runtime.getEventService().advanceTime(0);
            runCase(caseName, configuration, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Replays one case's steps on its runtime; sequences restart per case.
     * Compiles with CompilerArguments(configuration) plus the runtime path (the
     * sibling-oracle convention): the configuration makes the base event types
     * resolvable to the compiler, and the runtime path carries the prior
     * deployments' public types so the later modules of ords 45, 50 and 51 can
     * resolve the window created by the first module.  A pending module is
     * deployed when the next non-deploy step of the same case arrives, which is
     * what lets the late consumers be compiled mid-timeline at their source
     * positions (ord 45 INV:3135, ord 50 INV:3288/3300, ord 51 INV:3341).
     */
    private static void runCase(String caseName, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        int pendingModule = -1;
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                if (inCase) {
                    checkNegativeCompile(configuration, caseName);
                }
                continue;
            }
            if (!inCase) {
                continue;
            }
            String operation = string(step, "op");
            if ("deploy".equals(operation)) {
                String statementName = string(step, "statement");
                Integer module = MODULE_KEYS.get(caseName).get(statementName);
                if (module == null) {
                    throw new IllegalStateException("case " + caseName + " deploys unknown statement "
                            + statementName);
                }
                if (!pendingStatements.isEmpty() && pendingModule != module.intValue()) {
                    deployModule(configuration, runtime, caseName, pendingStatements,
                            pendingEpls, statementsByName, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                pendingModule = module.intValue();
                pendingStatements.add(statementName);
                pendingEpls.add(string(step, "epl"));
                continue;
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
                pendingStatements.clear();
                pendingEpls.clear();
            }
            switch (operation) {
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    EPStatement statement = statementsByName.get(string(step, "statement"));
                    if (statement == null) {
                        throw new IllegalStateException("snapshot targets unknown statement in case "
                                + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName, fieldsOf(step)));
                    break;
                }
                case "faf": {
                    // Fire-and-forget query against the named window, mirroring
                    // RegressionEnvironmentBase.compileExecuteFAF: compiled
                    // with the runtime path and executed on demand.
                    CompilerArguments fafArgs = new CompilerArguments(configuration);
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled query = EPCompilerProvider.getCompiler()
                            .compileQuery(string(step, "epl"), fafArgs);
                    EPFireAndForgetQueryResult result = runtime.getFireAndForgetService()
                            .executeQuery(query);
                    JsonArray rows = new JsonArray();
                    for (EventBean row : result.getArray()) {
                        checkDeliveryType(caseName, row);
                        rows.add(projectedRow(row, fieldsOf(step), rowTypePin(caseName)));
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "faf");
                    record.add("statement", string(step, "statement"));
                    record.add("sequence", 0);
                    record.add("time",
                            Instant.ofEpochMilli(runtime.getEventService().getCurrentTime())
                                    .toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "advance-time":
                    // Exactly one advanceTime per step, mirroring sendTimer in
                    // the suite (RegressionEnvironmentBase.advanceTime); the
                    // scenario pins the absolute virtual times byte-exactly.
                    runtime.getEventService().advanceTime(
                            Instant.parse(string(step, "at")).toEpochMilli());
                    break;
                case "undeploy": {
                    // Mirrors undeployModuleContaining in the suite: the
                    // deployment holding the statement is removed, so every
                    // statement of that module leaves the registry.
                    String statementName = string(step, "statement");
                    EPStatement statement = statementsByName.get(statementName);
                    if (statement == null) {
                        throw new IllegalStateException("undeploy targets unknown statement "
                                + statementName + " in case " + caseName);
                    }
                    String deploymentId = statement.getDeploymentId();
                    runtime.getDeploymentService().undeploy(deploymentId);
                    statementsByName.values().removeIf(
                            registered -> deploymentId.equals(registered.getDeploymentId()));
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        if (!pendingStatements.isEmpty()) {
            deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                    statementsByName, sequences, records);
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (exactly the Java source text of the suite's module), then
     * registers the statements in module order under the scenario's step ids
     * and attaches the case's listeners in statement order.
     */
    private static void deployModule(Configuration configuration, EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        List<EPStatement> deployed = new ArrayList<>();
        Set<String> deployedNames = new HashSet<>();
        for (EPStatement statement : deployment.getStatements()) {
            if (!deployedNames.add(statement.getName())) {
                throw new IllegalStateException("module of case " + caseName
                        + " deployed two statements named " + statement.getName());
            }
            deployed.add(statement);
        }
        if (deployed.size() != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.size() + " statements");
        }
        // Statements are matched by position, which is the order of the module
        // EPL text: the insert statement of every module (lines 2952/3126/3227/
        // 3278/3328) is unnamed in the Java source, so the engine assigns it a
        // generated name and the scenario's step id of that statement is the
        // module position; no listen or snapshot step resolves it.  Every
        // statement the source does name (create, s0, s2, s3 and delete) must
        // appear at the pinned position under exactly that name.
        Set<String> scenarioNames = new HashSet<>(statementNames);
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        Map<String, String[]> listenerFields =
                LISTENER_FIELDS.getOrDefault(caseName, Collections.emptyMap());
        for (int index = 0; index < statementNames.size(); index++) {
            String name = statementNames.get(index);
            EPStatement statement = deployed.get(index);
            if (scenarioNames.contains(statement.getName()) && !statement.getName().equals(name)) {
                throw new IllegalStateException("module of case " + caseName + " statement "
                        + statement.getName() + " is not the pinned statement at position "
                        + index + " (" + name + ")");
            }
            if (!scenarioNames.contains(statement.getName()) && deployedNames.contains(name)) {
                throw new IllegalStateException("module of case " + caseName + " pins the script "
                        + "name " + name + " for an unnamed statement while the module also names "
                        + "another statement " + name);
            }
            statementsByName.put(name, statement);
            checkDeployment(caseName, name, epls.get(index), statement);
            if (listened.contains(name)) {
                statement.addListener(listener(caseName, listenerFields.get(name), sequences, records,
                        runtime));
            }
        }
    }

    /**
     * The suite's {@code tryInvalidCompile} check of INV:315-316: ord 35's
     * AVRO-contained window EPL must fail to compile with the pinned message
     * prefix.  The check runs without a path exactly like the suite's, and emits
     * no record because it deploys nothing.
     */
    private static void checkNegativeCompile(Configuration configuration, String caseName)
            throws Exception {
        int index = caseIndex(caseName);
        String epl = CASE_NEGATIVE_COMPILE_EPLS[index];
        if (epl.isEmpty()) {
            return;
        }
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
        } catch (EPCompileException ex) {
            if (!ex.getMessage().startsWith(EPL_BC_AVRO_ERROR)) {
                throw new IllegalStateException("case " + caseName + " avro compile message = "
                        + ex.getMessage() + ", want prefix " + EPL_BC_AVRO_ERROR);
            }
            return;
        }
        throw new IllegalStateException("case " + caseName + " avro EPL compiled but must fail: "
                + epl);
    }

    /** The pinned index of one case name. */
    private static int caseIndex(String caseName) {
        for (int index = 0; index < CASE_NAMES.length; index++) {
            if (CASE_NAMES[index].equals(caseName)) {
                return index;
            }
        }
        throw new IllegalStateException("unknown case " + caseName);
    }

    /**
     * Harness-internal expectations the suite asserts through
     * {@code assertStatement} or {@code assertStatelessStmt} and that emit no
     * records of their own: ord 2's window must be a bean event named
     * MyWindowBB over SupportBean in every sub-run (which is what proves the
     * representation annotation was ignored, INV:3623-3626), its s0 select must
     * be stateless (INV:3599) and ord 35's window underlying must be Object[]
     * for the objectarray annotation and a Map for map/default (INV:3502 with
     * EventRepresentationChoice.matchesClass).
     */
    private static void checkDeployment(String caseName, String statementName, String epl,
                                        EPStatement statement) {
        if ("bean-backed".equals(caseName) && "create".equals(statementName)) {
            if (!(statement.getEventType() instanceof BeanEventType)
                    || !"MyWindowBB".equals(statement.getEventType().getName())
                    || !SupportBean.class.equals(statement.getEventType().getUnderlyingType())) {
                throw new IllegalStateException("case " + caseName + " window type = "
                        + statement.getEventType().getName() + " ("
                        + statement.getEventType().getClass().getSimpleName() + ", underlying "
                        + statement.getEventType().getUnderlyingType()
                        + "), want a bean window type MyWindowBB over SupportBean"
                        + " regardless of the representation annotation");
            }
            return;
        }
        if ("bean-backed".equals(caseName) && "s0".equals(statementName)) {
            StatementContext context = ((EPStatementSPI) statement).getStatementContext();
            if (!context.isStatelessSelect()) {
                throw new IllegalStateException("case " + caseName + " s0 is not stateless");
            }
            return;
        }
        if ("bean-contained".equals(caseName) && "create".equals(statementName)) {
            Class<?> underlying = statement.getEventType().getUnderlyingType();
            boolean objectArray = epl.contains("@EventRepresentation('objectarray')");
            if (objectArray && underlying != Object[].class) {
                throw new IllegalStateException("case " + caseName + " objectarray window underlying = "
                        + underlying + ", want Object[]");
            }
            if (!objectArray && (underlying == null || !Map.class.isAssignableFrom(underlying))) {
                throw new IllegalStateException("case " + caseName + " map/default window underlying = "
                        + underlying + ", want a Map implementation");
            }
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.  The
     * shapes this slice pins are the single new row of an ordinary arrival, the
     * new-plus-old pair of a unique-key replacement and of every statistics
     * update, the old-only delta of a delete whose passing row the filter lets
     * through, the new-only delta of a statistics update on an istream-free
     * select and the two-row new array of ord 51's join replay.  The record
     * time is the virtual clock at delivery.
     */
    private static UpdateListener listener(String caseName, String[] fields,
                                           Map<String, Integer> sequences, JsonArray records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(caseName, newEvents, fields);
            JsonArray oldRows = rows(caseName, oldEvents, fields);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /**
     * Snapshot of the statement iterator at the current virtual clock: one
     * record with sequence 0 holding the engine iterator order projected onto
     * the step's field list; the scenario's mode says whether the differential
     * compares that order positionally.  The suite mixes the two: ord 43's
     * unique-view window and its filtered consumer iterate a HashMap, so those
     * states are any-order, while ord 45's RowForAll sum, ord 49's statistics
     * row, ord 50's statistics and count rows and ord 51's recomputed join
     * result are ordered (INV:3343) except where the suite switched to the
     * any-order variant.  No snapshot of this slice is count-only, so no step
     * uses the empty-field encoding; an empty iterator omits the new array,
     * which is how the exhausted consumer iterator of INV:2982 shows up.  No
     * canonical reordering happens here; the mode is carried by the scenario
     * step, not by the record.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       String[] fields) {
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            EventBean event = iterator.next();
            checkDeliveryType(caseName, event);
            rows.add(projectedRow(event, fields, rowTypePin(caseName)));
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        return record;
    }

    /**
     * The snapshot step's projection list.  An empty array is the count-only
     * encoding: it projects no field, so each row renders {@code {}} and only
     * the row count is pinned.
     */
    private static String[] fieldsOf(JsonObject step) {
        JsonArray fields = array(step.get("fields"), "fields");
        String[] names = new String[fields.size()];
        for (int index = 0; index < fields.size(); index++) {
            JsonValue value = fields.get(index);
            if (!(value instanceof JsonString)) {
                throw new IllegalArgumentException("snapshot fields must be strings");
            }
            names[index] = value.asString();
        }
        return names;
    }

    /**
     * Projected row rendering projected to exactly the fields the Java
     * assertions read.  When the case's assertions are metadata-only (ord 2 and
     * ord 37) the row also carries the delivered event type name, which is the
     * language-neutral part of the suite's {@code assertEvent} predicate; the
     * field map stays empty there because Java never reads a property.
     */
    private static JsonObject projectedRow(EventBean event, String[] fields, boolean withType) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        if (withType) {
            item.add("type", event.getEventType().getName());
        }
        item.add("fields", projectedFields(event, fields));
        return item;
    }

    /** The projected field object of one event in the case's declared field order. */
    private static JsonObject projectedFields(EventBean event, String[] fields) {
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        return values;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(String caseName, EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        boolean withType = rowTypePin(caseName);
        for (EventBean event : events) {
            checkDeliveryType(caseName, event);
            array.add(projectedRow(event, fields, withType));
        }
        return array;
    }

    /** Whether the case's assertions carry the delivered event type on each row. */
    private static boolean rowTypePin(String caseName) {
        Boolean pin = ROW_TYPE_PINS.get(caseName);
        if (pin == null) {
            throw new IllegalStateException("unknown case " + caseName);
        }
        return pin;
    }

    /**
     * The suite's {@code assertEvent} predicate (INV:3622-3627) for the cases
     * whose listener or fire-and-forget rows are asserted as window-typed bean
     * events; a harness-internal expectation that emits no record of its own.
     */
    private static void checkDeliveryType(String caseName, EventBean event) {
        String expectedName = expectedWindowTypeName(caseName);
        if (expectedName == null) {
            return;
        }
        if (!(event.getEventType() instanceof BeanEventType)
                || !(event.getUnderlying() instanceof SupportBean)
                || event.getEventType().getMetadata().getTypeClass()
                        != com.espertech.esper.common.client.meta.EventTypeTypeClass.NAMED_WINDOW
                || !expectedName.equals(event.getEventType().getName())) {
            throw new IllegalStateException("case " + caseName + " delivered event type "
                    + event.getEventType().getName() + " ("
                    + event.getEventType().getClass().getSimpleName() + ", underlying "
                    + (event.getUnderlying() == null ? "null"
                            : event.getUnderlying().getClass().getSimpleName())
                    + "), want a bean event of window type " + expectedName + " over SupportBean");
        }
    }

    /** The window type name the metadata assertions of the case pin, or null. */
    private static String expectedWindowTypeName(String caseName) {
        if ("bean-backed".equals(caseName)) {
            return "MyWindowBB";
        }
        if ("bean-schema-backed".equals(caseName)) {
            return "MyWindowBSB";
        }
        return null;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object the Go normalizer also emits.
     */
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
        return Json.value(String.valueOf(value));
    }

    /**
     * Sends one scenario event under the event-type name the suite's
     * {@code sendEventBean} derives from the simple class name.  SupportBean's
     * no-arg send carries an empty payload (every property keeps its Java
     * default, theString included); SupportBean_S0, SupportBean_A and
     * SupportOverrideOneA carry the columns their EPL and assertions touch.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                if (theString != null) {
                    bean.setTheString(string(payload, "theString"));
                }
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null) {
                    bean.setIntPrimitive(integer(payload, "intPrimitive"));
                }
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null) {
                    bean.setLongBoxed(longInteger(longBoxed, "longBoxed"));
                }
                JsonValue longPrimitive = payload.get("longPrimitive");
                if (longPrimitive != null) {
                    bean.setLongPrimitive(longInteger(longPrimitive, "longPrimitive"));
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
            case "SupportBean_A": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", string(payload, "id"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "SupportOverrideOneA": {
                SupportOverrideOneA bean = new SupportOverrideOneA(
                        string(payload, "valOneA"), string(payload, "valOne"),
                        string(payload, "val"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASE_NAMES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "description", "createEpl", "createEplObjectArray", "createEplMap",
                    "createEplAvro", "insertEpl", "s0Epl", "s2Epl", "s3Epl", "consumeEpl",
                    "deleteEpl", "varEpl", "onSetEpl", "schemaEpl", "updateEpl", "fafEpl",
                    "negativeCompileEpl", "deploys", "listened", "iteratorSnapshots");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !CASE_CREATE_EPLS[index].equals(string(definition, "createEpl"))
                    || !CASE_CREATE_OBJECTARRAY_EPLS[index]
                            .equals(string(definition, "createEplObjectArray"))
                    || !CASE_CREATE_MAP_EPLS[index].equals(string(definition, "createEplMap"))
                    || !CASE_CREATE_AVRO_EPLS[index].equals(string(definition, "createEplAvro"))
                    || !CASE_INSERT_EPLS[index].equals(string(definition, "insertEpl"))
                    || !CASE_S0_EPLS[index].equals(string(definition, "s0Epl"))
                    || !CASE_S2_EPLS[index].equals(string(definition, "s2Epl"))
                    || !CASE_S3_EPLS[index].equals(string(definition, "s3Epl"))
                    || !CASE_CONSUME_EPLS[index].equals(string(definition, "consumeEpl"))
                    || !CASE_DELETE_EPLS[index].equals(string(definition, "deleteEpl"))
                    || !CASE_VAR_EPLS[index].equals(string(definition, "varEpl"))
                    || !CASE_ONSET_EPLS[index].equals(string(definition, "onSetEpl"))
                    || !CASE_SCHEMA_EPLS[index].equals(string(definition, "schemaEpl"))
                    || !CASE_UPDATE_EPLS[index].equals(string(definition, "updateEpl"))
                    || !CASE_FAF_EPLS[index].equals(string(definition, "fafEpl"))
                    || !CASE_NEGATIVE_COMPILE_EPLS[index]
                            .equals(string(definition, "negativeCompileEpl"))
                    || integer(definition, "iteratorSnapshots") != CASE_SNAPSHOTS[index]) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateStringArray(definition.get("deploys"), CASE_DEPLOYS[index],
                    "deploys for " + CASE_NAMES[index]);
            validateStringArray(definition.get("listened"), CASE_LISTENED[index],
                    "listened for " + CASE_NAMES[index]);
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = validateBeanBacked(steps, 0);
        offset = validateBeanContained(steps, offset);
        offset = validateBeanSchemaBacked(steps, offset);
        offset = validateDeepSupertypeInsert(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }



    /**
     * Exact InfraBeanBacked step sequence mirroring the helper at lines
     * 3591-3611 run once per representation (INV:300-303): create, insert and
     * s0 for the first fill, the on-trigger update module, the SupportBean_A
     * trigger and a full teardown, repeated for objectarray, map, default and
     * avro.  No clock moves.
     */
    private static int validateBeanBacked(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[0];
        validateCaseMarker(steps.get(offset++), caseName);
        String[] createEpls = {
                EPL_BB_CREATE_OBJECTARRAY, EPL_BB_CREATE_MAP, EPL_BB_CREATE, EPL_BB_CREATE_AVRO
        };
        for (String createEpl : createEpls) {
            validateDeploy(steps.get(offset++), caseName, "create", createEpl);
            validateDeploy(steps.get(offset++), caseName, "insert", EPL_BB_INSERT);
            validateDeploy(steps.get(offset++), caseName, "s0", EPL_BB_S0);
            validateEmptyBeanSend(steps.get(offset++), caseName);
            validateDeploy(steps.get(offset++), caseName, "update", EPL_BB_UPDATE);
            validateBeanASend(steps.get(offset++), caseName, "A1");
            validateUndeployAll(steps.get(offset++), caseName);
        }
        return offset;
    }

    /**
     * Exact InfraBeanContained step sequence mirroring the helper at lines
     * 3498-3509 run once per representation (INV:309-313): the contained-bean
     * window, the stream-wildcard insert, the SupportBean_S0 fill and a full
     * teardown, repeated for objectarray, map and default.  The avro variant of
     * INV:315-316 never deploys; the oracle checks its compile failure
     * separately.  No clock moves.
     */
    private static int validateBeanContained(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[1];
        validateCaseMarker(steps.get(offset++), caseName);
        String[] createEpls = {EPL_BC_CREATE_OBJECTARRAY, EPL_BC_CREATE_MAP, EPL_BC_CREATE};
        for (String createEpl : createEpls) {
            validateDeploy(steps.get(offset++), caseName, "create", createEpl);
            validateDeploy(steps.get(offset++), caseName, "insert", EPL_BC_INSERT);
            validateBeanS0Send(steps.get(offset++), caseName, 1, "E1");
            validateUndeployAll(steps.get(offset++), caseName);
        }
        return offset;
    }

    /**
     * Exact InfraBeanSchemaBacked step sequence mirroring lines 347-360: the
     * schema/window/insert module, the first bean send, the fire-and-forget
     * query of INV:353, the late {@code select * from ABC} consumer and the
     * second bean send whose only observable is that the consumer stays
     * silent.  No clock moves.
     */
    private static int validateBeanSchemaBacked(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[2];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "schema", EPL_BSB_SCHEMA);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_BSB_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_BSB_INSERT);
        validateEmptyBeanSend(steps.get(offset++), caseName);
        validateFaf(steps.get(offset++), caseName, EPL_BSB_FAF);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_BSB_S0);
        validateEmptyBeanSend(steps.get(offset++), caseName);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact InfraDeepSupertypeInsert step sequence mirroring lines 366-371: the
     * two-statement module, the subtype send and the single window iterator
     * state of INV:370.  No clock moves.
     */
    private static int validateDeepSupertypeInsert(JsonArray steps, int offset) {
        String caseName = CASE_NAMES[3];
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_DSI_CREATE);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_DSI_INSERT);
        validateOverrideSend(steps.get(offset++), caseName, "1a", "1", "base");
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", "val");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /** The no-argument SupportBean send of ords 2 and 37 (empty payload). */
    private static void validateEmptyBeanSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        requireFields(object(step.get("payload"), "SupportBean payload"));
    }

    /** Ord 35's SupportBean_S0 fill: new SupportBean_S0(1, "E1"). */
    private static void validateBeanS0Send(JsonValue value, String caseName, int expectedId,
                                           String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (integer(payload, "id") != expectedId || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    /** Ord 2's update trigger: new SupportBean_A("A1"). */
    private static void validateBeanASend(JsonValue value, String caseName, String expectedId) {
        JsonObject step = object(value, "SupportBean_A step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean_A".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_A step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_A payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException("SupportBean_A payload is not pinned for " + caseName);
        }
    }

    /** Ord 38's subtype send: new SupportOverrideOneA("1a", "1", "base"). */
    private static void validateOverrideSend(JsonValue value, String caseName, String expectedValOneA,
                                             String expectedValOne, String expectedVal) {
        JsonObject step = object(value, "SupportOverrideOneA step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportOverrideOneA".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportOverrideOneA step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportOverrideOneA payload");
        requireFields(payload, "valOneA", "valOne", "val");
        if (!expectedValOneA.equals(string(payload, "valOneA"))
                || !expectedValOne.equals(string(payload, "valOne"))
                || !expectedVal.equals(string(payload, "val"))) {
            throw new IllegalArgumentException("SupportOverrideOneA payload is not pinned for "
                    + caseName);
        }
    }

    /**
     * Ord 37's fire-and-forget query step: the query text is compiled with the
     * runtime path and executed on demand, and the step's projection list is
     * empty because the suite reads only the returned row's event type.
     */
    private static void validateFaf(JsonValue value, String caseName, String expectedEpl) {
        JsonObject step = object(value, "faf step");
        requireFields(step, "op", "case", "statement", "epl", "fields");
        if (!"faf".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"faf".equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("faf step is not pinned for " + caseName);
        }
        validateStringArray(step.get("fields"), new String[0], "faf fields for " + caseName);
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /** Deploy step of one module statement, pinned byte-exactly. */
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

    /**
     * Every snapshot step pins its statement, its mode and its projection
     * list. No snapshot of this slice is count-only, so every step carries the
     * non-empty field list its iterator assertion reads; ordered steps pin
     * exact order and any-mode steps leave the order the Java suite does not
     * assert unconstrained.
     */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedMode,
                                         String... expectedFields) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), expectedFields,
                "snapshot fields for " + caseName + "/" + expectedStatement);
    }

    /** Ord 45's module-scoped undeploys (undeployModuleContaining in the suite). */
    private static void validateUndeploy(JsonValue value, String caseName, String expectedStatement) {
        JsonObject step = object(value, "undeploy step");
        requireFields(step, "op", "case", "statement");
        if (!"undeploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("undeploy step is not pinned for " + caseName + "/"
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

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
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
                || !new HashSet<>(object.names()).equals(
                        new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields "
                    + (object == null ? "<null>" : object.names()) + ", expected "
                    + Arrays.toString(expectedNames));
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
