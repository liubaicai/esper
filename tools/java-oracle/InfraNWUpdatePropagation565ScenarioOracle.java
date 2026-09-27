import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
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
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the named-window update-propagation bundle: four
 * executions sharing one semantic — a named window's row lifecycle
 * propagating to its downstream consumers.
 *
 * <p>update-namedwindow replays EPLOtherUpdateNamedWindow
 * (EPLOtherUpdateIStream ord 6): update-istream copy-on-write over a keepall
 * window; the insert listener observes the pre-update copy while the
 * create-window statement and the select/onselect/oninsert consumers observe
 * the updated row, and the second update-istream preprocesses the on-insert
 * routed target. The Java execution's milestone(0/1/2) calls are serde
 * checkpoints that are no-ops in the base harness; here they are modeled as
 * redeploy barriers — the scenario interleaves an undeploy-all step (which
 * destroys every deployment including the named window's data) with explicit
 * redeploy and replayed-send steps.
 *
 * <p>onupdate-multidispatch replays InfraNamedWindowOnUpdateWMultiDispatch
 * ord 0: an uncorrelated on-update trigger over a #time(25
 * hour)#firstunique(company) intersect window. Esper's on-trigger action is
 * preemptive: it observes the window before the same event's continuous
 * insert lands, so totals accumulate over the retained value-3 row (0, 9,
 * 8, 7) while the admitted BComp row stays untouched. The Java execution
 * leaves the final s0 delivery's batch shape undefined ([1,2] or [2]); the
 * count is therefore pinned deterministically through iterator snapshots of
 * s0 instead of a listener batch.
 *
 * <p>outputrate-snapshot replays InfraNamedWindowOutputrate ord 0: grouped
 * irstream count with output snapshot every 1 second over virtual time,
 * including the identical-repeat emission at t=3000. The irstream's old-row
 * pairing is undefined for snapshot output and stays out of the records.
 *
 * <p>removestream-chain replays InfraNamedWindowRemoveStream ord 0: insert
 * rstream into W2 select rstream * from W1 cascades W1's length(2) evictions
 * into W2 and W2's into W3, pinned by any-order iterator snapshots.
 *
 * <p>Each case runs on a fresh runtime with the clock pinned at 0 before any
 * statement is deployed. The internal timer is disabled so schedules fire
 * only on the pinned advance-time steps.
 */
public final class InfraNWUpdatePropagation565ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-update-propagation-565";
    private static final String DESCRIPTION =
            "EPLOtherUpdateNamedWindow (EPLOtherUpdateIStream ord 6), InfraNamedWindowOnUpdateWMultiDispatch ord 0, InfraNamedWindowOutputrate ord 0 and InfraNamedWindowRemoveStream ord 0: one named-window row-lifecycle propagation bundle. update-namedwindow replays update-istream copy-on-write over a keepall AWindow — the insert listener sees the pre-update {E1,oldvalue} copy while the window/select/onselect/oninsert consumers see {E1,newvalue}, and the second update-istream rewrites MyOtherStream routed rows to {a,b}; the three Java milestones are redeploy barriers (undeploy-all plus explicit redeploy and send replay steps). onupdate-multidispatch replays the uncorrelated on S2 update of the #time(25 hour)#firstunique(company) intersect window — the preemptive trigger sees the window pre-insert, so totals run 0, 6+3=9, 9+5=8, 8+4=7 while the rejected firstunique duplicates and the admitted BComp row carry value 3/4/5/4 through select count snapshots (the Java-defined-ambiguous final s0 batch shape [1,2] or [2] is pinned deterministically through the s0 iterator instead of a listener batch). outputrate-snapshot replays select irstream theString, count(*) group-by-theString output snapshot every 1 second over virtual time, including the identical-repeat emission at t=3000. removestream-chain replays the insert-rstream cascade W1->W2->W3 over length(2) windows pinned by any-order iterator snapshots.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-7c91cbd63d65e3078e25",
            "java-runtime-fcbbceeaf04849dc64bc",
            "java-runtime-da3a1e5e9ab73d4a067e",
            "java-runtime-4971ea67797956c303ee"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherUpdateNamedWindow",
            "InfraNamedWindowOnUpdateWMultiDispatch",
            "InfraNamedWindowOutputrate",
            "InfraNamedWindowRemoveStream"
    };
    private static final String[] STATIC_IDS = {
            "java-082395e7cb9dbac98bea",
            "java-608c6908a0d53bf60fb5",
            "java-3ffa81dff07fbd6b6a84",
            "java-5d1931fb11b4c00a1ecc"
    };
    private static final String[] FLAGS = {"EXCLUDEWHENINSTRUMENTED"};

    private static final String CASE_UPDATE = "update-namedwindow";
    private static final String CASE_MULTIDISPATCH = "onupdate-multidispatch";
    private static final String CASE_OUTPUTRATE = "outputrate-snapshot";
    private static final String CASE_CHAIN = "removestream-chain";
    private static final String[] CASE_NAMES = {
            CASE_UPDATE, CASE_MULTIDISPATCH, CASE_OUTPUTRATE, CASE_CHAIN};
    private static final int[] ORDINALS = {6, 0, 0, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "update-istream copy-on-write over a keepall window with insert/window/select consumers, on-select and on-insert triggers and a second update-istream over the routed target; three milestone redeploy barriers",
            "uncorrelated on-update trigger over a time+firstunique intersect window: the preemptive trigger sees the pre-insert window so totals accumulate over retained value 3 (0->9->8->7) while BComp is admitted untouched; s0 count pinned by iterator snapshots (final batch shape [1,2] or [2] is defined-ambiguous in Java)",
            "grouped irstream count over a keepall window with output snapshot every 1 second: each virtual-time tick emits the full group set as new rows, including the identical-repeat emission at t=3000",
            "insert rstream chain over length(2) windows: W1 evictions insert into W2, W2 evictions insert into W3; any-order iterator snapshots pin the cascade after each send wave"
    };

    // Byte-exact EPL pins (EPLOtherUpdateIStream.java lines 517-542;
    // InfraNamedWindowOnUpdateWMultiDispatch.java lines 37-45;
    // InfraNamedWindowOutputrate.java lines 24-30;
    // InfraNamedWindowRemoveStream.java lines 26-32).
    private static final String EPL_UPDATE_WINDOW =
            "@name('window') @public create window AWindow#keepall select * from MyMapTypeNW";
    private static final String EPL_UPDATE_INSERT =
            "@name('insert') insert into AWindow select * from MyMapTypeNW";
    private static final String EPL_UPDATE_SELECT =
            "@name('select') select * from AWindow";
    private static final String EPL_UPDATE_ISTREAM =
            "update istream AWindow set p1='newvalue'";
    private static final String EPL_UPDATE_ONSELECT =
            "@name('onselect') on SupportBean(theString='A') select win.* from AWindow as win";
    private static final String EPL_UPDATE_ONINSERT =
            "@name('oninsert') @public on SupportBean(theString='B') insert into MyOtherStream select win.* from AWindow as win";
    private static final String EPL_UPDATE_OTHER =
            "update istream MyOtherStream set p0='a', p1='b'";
    private static final String EPL_UPDATE_S0 =
            "@name('s0') select * from MyOtherStream";

    private static final String EPL_MD_SCHEMA =
            "@public @buseventtype create schema S2 ( company string, value double, total double)";
    private static final String EPL_MD_CREATE =
            "@name('create') @public create window S2Win#time(25 hour)#firstunique(company) as S2";
    private static final String EPL_MD_INSERT =
            "insert into S2Win select * from S2#firstunique(company)";
    private static final String EPL_MD_UPDATE =
            "on S2 as a update S2Win as b set total = b.value + a.value";
    private static final String EPL_MD_S0 =
            "@name('s0') select count(*) as cnt from S2Win";

    private static final String EPL_OR_CREATE =
            "@public create window MyWindowOne#keepall as (theString string, intv int)";
    private static final String EPL_OR_INSERT =
            "insert into MyWindowOne select theString, intPrimitive as intv from SupportBean";
    private static final String EPL_OR_S0 =
            "@name('s0') select irstream theString, count(*) as c from MyWindowOne group by theString output snapshot every 1 second";

    private static final String EPL_RS_C1 =
            "@name('c1') @public create window W1#length(2) as select * from SupportBean";
    private static final String EPL_RS_C2 =
            "@name('c2') @public create window W2#length(2) as select * from SupportBean";
    private static final String EPL_RS_C3 =
            "@name('c3') @public create window W3#length(2) as select * from SupportBean";
    private static final String EPL_RS_INSERT = "insert into W1 select * from SupportBean";
    private static final String EPL_RS_ROUTE2 = "insert rstream into W2 select rstream * from W1";
    private static final String EPL_RS_ROUTE3 = "insert rstream into W3 select rstream * from W2";

    // Labels whose deliveries produce listener records per case; the row
    // field projection per label.
    private static final Map<String, Set<String>> LISTENED = new HashMap<>();
    private static final Map<String, Map<String, String[]>> FIELDS = new HashMap<>();
    private static final Map<String, String[]> SNAP_FIELDS = new HashMap<>();
    private static final Set<String> NEW_ONLY = new HashSet<>();

    static {
        LISTENED.put(CASE_UPDATE, new HashSet<>(List.of(
                "window", "insert", "select", "onselect", "oninsert", "s0")));
        LISTENED.put(CASE_MULTIDISPATCH, new HashSet<>(List.of("upd")));
        LISTENED.put(CASE_OUTPUTRATE, new HashSet<>(List.of("s0")));
        LISTENED.put(CASE_CHAIN, new HashSet<>());

        String[] p0p1 = {"p0", "p1"};
        Map<String, String[]> updateFields = new HashMap<>();
        for (String label : new String[]{"window", "insert", "select", "onselect", "oninsert", "s0"}) {
            updateFields.put(label, p0p1);
        }
        FIELDS.put(CASE_UPDATE, updateFields);
        FIELDS.put(CASE_MULTIDISPATCH, Map.of("upd", new String[]{"company", "value", "total"}));
        FIELDS.put(CASE_OUTPUTRATE, Map.of("s0", new String[]{"theString", "c"}));
        FIELDS.put(CASE_CHAIN, Map.of());

        SNAP_FIELDS.put(CASE_MULTIDISPATCH + "/s0", new String[]{"cnt"});
        SNAP_FIELDS.put(CASE_MULTIDISPATCH + "/create", new String[]{"company", "value", "total"});
        SNAP_FIELDS.put(CASE_CHAIN + "/c1", new String[]{"theString"});
        SNAP_FIELDS.put(CASE_CHAIN + "/c2", new String[]{"theString"});
        SNAP_FIELDS.put(CASE_CHAIN + "/c3", new String[]{"theString"});

        NEW_ONLY.add(CASE_OUTPUTRATE + "/s0");
    }

    private static final int[] EXPECTED_CASE_RECORDS = {16, 11, 4, 6};
    private static final int EXPECTED_RECORDS = 37;
    private static final int EXPECTED_STEPS = 91;

    private InfraNWUpdatePropagation565ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWUpdatePropagation565ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray steps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            int before = records.size();
            runCaseOnFreshRuntime(CASE_NAMES[index], steps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                for (int r = before; r < records.size(); r++) {
                    System.err.println("REC " + records.get(r).toString());
                }
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

    private static void runCaseOnFreshRuntime(String caseName, JsonArray allSteps,
                                              JsonArray records) throws Exception {
        Configuration configuration = configurationFor(caseName);
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

    private static Configuration configurationFor(String caseName) {
        Configuration configuration = new Configuration();
        switch (caseName) {
            case CASE_UPDATE:
                configuration.getCommon().addEventType("MyMapTypeNW", mapSchema("p0", "p1"));
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case CASE_MULTIDISPATCH:
                // S2 comes from the case's own @public create schema
                // statement; nothing is registered here.
                break;
            case CASE_OUTPUTRATE:
            case CASE_CHAIN:
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            default:
                throw new IllegalArgumentException("unknown case " + caseName);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
    }

    private static Map<String, Object> mapSchema(String... fields) {
        Map<String, Object> schema = new LinkedHashMap<>();
        for (String field : fields) {
            schema.put(field, Object.class);
        }
        return schema;
    }

    private static void runCase(String caseName, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
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
            String operation = string(step, "op");
            switch (operation) {
                case "deploy":
                    deployStatement(configuration, runtime, caseName,
                            string(step, "statement"), string(step, "epl"),
                            statementsByName, sequences, records);
                    break;
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    EPStatement statement = statementsByName.get(string(step, "statement"));
                    if (statement == null) {
                        throw new IllegalStateException(
                                "snapshot targets unknown statement " + string(step, "statement")
                                        + " in case " + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName,
                            string(step, "statement"), "any".equals(string(step, "mode"))));
                    break;
                }
                case "advance-time":
                    runtime.getEventService().advanceTime(
                            Instant.parse(string(step, "at")).toEpochMilli());
                    break;
                case "undeploy-all":
                    // Milestone redeploy barrier plus the trailing teardown:
                    // undeploy destroys every deployment including the named
                    // window's contents; the following deploy/send steps
                    // redeploy the statements and replay the sends.
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    // deployStatement mirrors the Java execution's env.compileDeploy per
    // statement: each deploy step is one compilation plus one deployment,
    // keeping the statement's declared name (or the scenario label when the
    // EPL carries no @name).
    private static void deployStatement(Configuration configuration, EPRuntime runtime,
                                        String caseName, String label, String epl,
                                        Map<String, EPStatement> statementsByName,
                                        Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        if ("schema".equals(label)) {
            // The create-schema statement declares the S2 map type; the Go
            // runner registers it at environment build time, so no runtime
            // statement is tracked on either side — but the EPL still
            // deploys to match the Java compileDeploy surface.
            CompilerArguments schemaArgs = new CompilerArguments(configuration);
            schemaArgs.getPath().add(runtime.getRuntimePath());
            runtime.getDeploymentService().deploy(
                    EPCompilerProvider.getCompiler().compile(epl, schemaArgs),
                    new DeploymentOptions());
            return;
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        EPStatement[] deployed = deployment.getStatements();
        if (deployed.length != 1) {
            throw new IllegalStateException("deploy " + label + " in case " + caseName
                    + " produced " + deployed.length + " statements");
        }
        EPStatement statement = deployed[0];
        statementsByName.put(label, statement);
        if (LISTENED.get(caseName).contains(label)) {
            statement.addListener(listener(caseName, label, sequences, records, runtime));
        }
    }

    private static UpdateListener listener(String caseName, String label,
                                           Map<String, Integer> sequences, JsonArray records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            String[] fields = FIELDS.get(caseName).get(label);
            JsonArray newRows = rows(newEvents, fields);
            JsonArray oldRows = rows(oldEvents, fields);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                // Force-dispatched empty pairs carry no record, the same
                // convention the Go runner follows.
                return;
            }
            int sequence = sequences.merge(label, 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", label);
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0 && !NEW_ONLY.contains(caseName + "/" + label)) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       String label, boolean anyOrder) {
        String[] fields = SNAP_FIELDS.get(caseName + "/" + label);
        if (fields == null) {
            throw new IllegalStateException(
                    "snapshot fields not pinned for " + caseName + "/" + label);
        }
        JsonArray rows = new JsonArray();
        for (java.util.Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            EventBean event = iterator.next();
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject values = new JsonObject();
            for (String field : fields) {
                values.add(field, normalize(event.get(field)));
            }
            item.add("fields", values);
            rows.add(item);
        }
        if (anyOrder) {
            sortRowsCanonical(rows);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        return record;
    }

    private static void sortRowsCanonical(JsonArray rows) {
        List<JsonObject> values = new ArrayList<>();
        for (JsonValue value : rows) {
            values.add(value.asObject());
        }
        values.sort(Comparator.comparing(value -> value.get("fields").asObject().toString()));
        for (int index = rows.size() - 1; index >= 0; index--) {
            rows.remove(index);
        }
        for (JsonObject value : values) {
            rows.add(value);
        }
    }

    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject values = new JsonObject();
            for (String field : fields) {
                values.add(field, normalize(event.get(field)));
            }
            item.add("fields", values);
            array.add(item);
        }
        return array;
    }

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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean":
                runtime.getEventService().sendEventBean(new SupportBean(
                        string(payload, "theString"),
                        (int) longInteger(payload.get("intPrimitive"), "intPrimitive")), type);
                break;
            case "MyMapTypeNW": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("p0", string(payload, "p0"));
                event.put("p1", string(payload, "p1"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            case "S2": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("company", string(payload, "company"));
                event.put("value", doubleValue(payload.get("value"), "value"));
                event.put("total", doubleValue(payload.get("total"), "total"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        String[] sourceFiles = {
                "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherUpdateIStream.java",
                "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdateWMultiDispatch.java",
                "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOutputrate.java",
                "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowRemoveStream.java",
                "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean.java"
        };
        validateStringArray(scenario.get("javaSourceFiles"), sourceFiles, "javaSourceFiles");
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        String[][] caseDeploys = {
                {"window", "insert", "select", "update", "onselect", "oninsert", "update-other", "s0"},
                {"schema", "create", "insert", "upd", "s0"},
                {"create", "insert", "s0"},
                {"c1", "c2", "c3", "insert", "route-w2", "route-w3"}
        };
        Map<String, Map<String, String>> caseEpls = new HashMap<>();
        caseEpls.put(CASE_UPDATE, Map.of(
                "window", EPL_UPDATE_WINDOW, "insert", EPL_UPDATE_INSERT,
                "select", EPL_UPDATE_SELECT, "update", EPL_UPDATE_ISTREAM,
                "onselect", EPL_UPDATE_ONSELECT, "oninsert", EPL_UPDATE_ONINSERT,
                "update-other", EPL_UPDATE_OTHER, "s0", EPL_UPDATE_S0));
        caseEpls.put(CASE_MULTIDISPATCH, Map.of(
                "schema", EPL_MD_SCHEMA, "create", EPL_MD_CREATE,
                "insert", EPL_MD_INSERT, "upd", EPL_MD_UPDATE, "s0", EPL_MD_S0));
        caseEpls.put(CASE_OUTPUTRATE, Map.of(
                "create", EPL_OR_CREATE, "insert", EPL_OR_INSERT, "s0", EPL_OR_S0));
        caseEpls.put(CASE_CHAIN, Map.of(
                "c1", EPL_RS_C1, "c2", EPL_RS_C2, "c3", EPL_RS_C3,
                "insert", EPL_RS_INSERT, "route-w2", EPL_RS_ROUTE2, "route-w3", EPL_RS_ROUTE3));
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl", "deploys");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateStringArray(definition.get("deploys"), caseDeploys[index],
                    CASE_NAMES[index] + " deploys");
            StringBuilder joined = new StringBuilder();
            for (String label : caseDeploys[index]) {
                if (joined.length() > 0) {
                    joined.append("\n");
                }
                joined.append(caseEpls.get(CASE_NAMES[index]).get(label));
            }
            if (!joined.toString().equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case epl is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateUpdateCase(steps, offset);
        offset = validateMultidispatchCase(steps, offset);
        offset = validateOutputrateCase(steps, offset);
        offset = validateChainCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has an unexpected step suffix");
        }
    }

    private static int validateUpdateCase(JsonArray steps, int offset) {
        String name = CASE_UPDATE;
        validateCaseMarker(steps.get(offset++), name);
        String[] base = {"window", "insert", "select", "update"};
        String[] baseEpl = {EPL_UPDATE_WINDOW, EPL_UPDATE_INSERT, EPL_UPDATE_SELECT, EPL_UPDATE_ISTREAM};
        for (int index = 0; index < base.length; index++) {
            validateDeploy(steps.get(offset++), name, base[index], baseEpl[index]);
        }
        validateUndeployAll(steps.get(offset++), name); // milestone(0)
        for (int index = 0; index < base.length; index++) {
            validateDeploy(steps.get(offset++), name, base[index], baseEpl[index]);
        }
        validateSendMap(steps.get(offset++), name, "E1", "oldvalue");
        validateDeploy(steps.get(offset++), name, "onselect", EPL_UPDATE_ONSELECT);
        validateSendBean(steps.get(offset++), name, "A", 0);
        validateUndeployAll(steps.get(offset++), name); // milestone(1)
        for (int index = 0; index < base.length; index++) {
            validateDeploy(steps.get(offset++), name, base[index], baseEpl[index]);
        }
        validateDeploy(steps.get(offset++), name, "onselect", EPL_UPDATE_ONSELECT);
        validateSendMap(steps.get(offset++), name, "E1", "oldvalue");
        validateSendBean(steps.get(offset++), name, "A", 0);
        validateDeploy(steps.get(offset++), name, "oninsert", EPL_UPDATE_ONINSERT);
        validateSendBean(steps.get(offset++), name, "B", 1);
        validateUndeployAll(steps.get(offset++), name); // milestone(2)
        for (int index = 0; index < base.length; index++) {
            validateDeploy(steps.get(offset++), name, base[index], baseEpl[index]);
        }
        validateDeploy(steps.get(offset++), name, "onselect", EPL_UPDATE_ONSELECT);
        validateDeploy(steps.get(offset++), name, "oninsert", EPL_UPDATE_ONINSERT);
        validateSendMap(steps.get(offset++), name, "E1", "oldvalue");
        validateSendBean(steps.get(offset++), name, "A", 0);
        validateSendBean(steps.get(offset++), name, "B", 1);
        validateDeploy(steps.get(offset++), name, "update-other", EPL_UPDATE_OTHER);
        validateDeploy(steps.get(offset++), name, "s0", EPL_UPDATE_S0);
        validateSendBean(steps.get(offset++), name, "B", 1);
        validateUndeployAll(steps.get(offset++), name);
        return offset;
    }

    private static int validateMultidispatchCase(JsonArray steps, int offset) {
        String name = CASE_MULTIDISPATCH;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "schema", EPL_MD_SCHEMA);
        validateDeploy(steps.get(offset++), name, "create", EPL_MD_CREATE);
        validateDeploy(steps.get(offset++), name, "insert", EPL_MD_INSERT);
        validateDeploy(steps.get(offset++), name, "upd", EPL_MD_UPDATE);
        validateDeploy(steps.get(offset++), name, "s0", EPL_MD_S0);
        validateSendS2(steps.get(offset++), name, "AComp", 3);
        validateSnapshot(steps.get(offset++), name, "s0", "ordered");
        validateSnapshot(steps.get(offset++), name, "create", "ordered");
        validateSendS2(steps.get(offset++), name, "AComp", 6);
        validateSnapshot(steps.get(offset++), name, "s0", "ordered");
        validateSnapshot(steps.get(offset++), name, "create", "ordered");
        validateSendS2(steps.get(offset++), name, "AComp", 5);
        validateSnapshot(steps.get(offset++), name, "s0", "ordered");
        validateSnapshot(steps.get(offset++), name, "create", "ordered");
        validateSendS2(steps.get(offset++), name, "BComp", 4);
        validateSnapshot(steps.get(offset++), name, "s0", "ordered");
        validateSnapshot(steps.get(offset++), name, "create", "ordered");
        validateUndeployAll(steps.get(offset++), name);
        return offset;
    }

    private static int validateOutputrateCase(JsonArray steps, int offset) {
        String name = CASE_OUTPUTRATE;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "create", EPL_OR_CREATE);
        validateDeploy(steps.get(offset++), name, "insert", EPL_OR_INSERT);
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), name, "s0", EPL_OR_S0);
        validateSendBean(steps.get(offset++), name, "A", 1);
        validateSendBean(steps.get(offset++), name, "A", 2);
        validateSendBean(steps.get(offset++), name, "B", 4);
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:01Z");
        validateSendBean(steps.get(offset++), name, "B", 5);
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:02Z");
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:03Z");
        validateSendBean(steps.get(offset++), name, "A", 5);
        validateSendBean(steps.get(offset++), name, "C", 1);
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:04Z");
        validateUndeployAll(steps.get(offset++), name);
        return offset;
    }

    private static int validateChainCase(JsonArray steps, int offset) {
        String name = CASE_CHAIN;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "c1", EPL_RS_C1);
        validateDeploy(steps.get(offset++), name, "c2", EPL_RS_C2);
        validateDeploy(steps.get(offset++), name, "c3", EPL_RS_C3);
        validateDeploy(steps.get(offset++), name, "insert", EPL_RS_INSERT);
        validateDeploy(steps.get(offset++), name, "route-w2", EPL_RS_ROUTE2);
        validateDeploy(steps.get(offset++), name, "route-w3", EPL_RS_ROUTE3);
        validateSendBean(steps.get(offset++), name, "E1", 1);
        validateSendBean(steps.get(offset++), name, "E2", 1);
        validateSnapshot(steps.get(offset++), name, "c1", "any");
        validateSendBean(steps.get(offset++), name, "E3", 1);
        validateSnapshot(steps.get(offset++), name, "c1", "any");
        validateSnapshot(steps.get(offset++), name, "c2", "any");
        validateSendBean(steps.get(offset++), name, "E4", 1);
        validateSendBean(steps.get(offset++), name, "E5", 1);
        validateSnapshot(steps.get(offset++), name, "c1", "any");
        validateSnapshot(steps.get(offset++), name, "c2", "any");
        validateSnapshot(steps.get(offset++), name, "c3", "any");
        validateUndeployAll(steps.get(offset++), name);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !expectedCase.equals(string(step, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String statement,
                                       String epl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !statement.equals(string(step, "statement"))
                || !epl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + statement);
        }
    }

    private static void validateSendMap(JsonValue value, String caseName, String p0, String p1) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"MyMapTypeNW".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("MyMapTypeNW step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "MyMapTypeNW payload");
        requireFields(payload, "p0", "p1");
        if (!p0.equals(string(payload, "p0")) || !p1.equals(string(payload, "p1"))) {
            throw new IllegalArgumentException("MyMapTypeNW payload is not pinned");
        }
    }

    private static void validateSendBean(JsonValue value, String caseName, String text, long n) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!text.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != n) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateSendS2(JsonValue value, String caseName, String company, double v) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"S2".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("S2 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "S2 payload");
        requireFields(payload, "company", "value", "total");
        if (!company.equals(string(payload, "company"))
                || doubleValue(payload.get("value"), "value") != v
                || doubleValue(payload.get("total"), "total") != 0) {
            throw new IllegalArgumentException("S2 payload is not pinned");
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String statement,
                                         String mode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !statement.equals(string(step, "statement"))
                || !mode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + statement);
        }
    }

    private static void validateAdvance(JsonValue value, String caseName, String at) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "case", "at");
        if (!"advance-time".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !at.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step is not pinned for " + caseName);
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
                || !new HashSet<>(object.names()).equals(
                new HashSet<>(Arrays.asList(expectedNames)))) {
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
        try {
            return Long.parseLong(value.toString(), 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static double doubleValue(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON number");
        }
        return Double.parseDouble(value.toString());
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
