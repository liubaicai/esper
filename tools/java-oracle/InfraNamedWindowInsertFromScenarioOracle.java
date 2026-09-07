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
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNamedWindowInsertFrom insert-from-window executions:
 * type-by-window creation after an existing named window with shared insert
 * delivery, seeded keep-all and filtered and unique-index create-window-insert
 * with deploy-time row copying that never reaches create-statement listeners,
 * filtered routing inserts into each window, and the lenient partial-column
 * inserts over map and object-array schemas.
 *
 * Replays the four pinned executions (ordinals 0, 1, 5, and 6 of the suite's
 * executions()) on one runtime with undeployAll between cases, mirroring the
 * regression-suite harness: SupportBean and SupportBean_S0 from esper-common,
 * internal timer disabled, and the rethrowing exception handler so statement
 * failures surface to the sender thread.  The suite compiles each execution as
 * ONE module while this scenario deploys steps as separate modules, so
 * window-referencing creates carry @public (see EPL_CREATE_WINDOW_TWO for the
 * one whitespace-only transcription this forces).  Listeners attach when the
 * deployment containing the statement completes, exactly like the source's
 * compileDeploy(...).addListener(...) chaining: the ord-1 seeded
 * create-window-insert statements copy rows during their deployment BEFORE the
 * listener attaches, so seed rows never reach the windowTwo/windowThree/
 * windowFour listeners (the source pins this with assertListenerNotInvoked),
 * while the ord-1 "window" statement attaches its listener before the five
 * seed-window sends and therefore records them.  Listener rows project to
 * intPrimitive and theString (sorted; the ord-0 selectOne row carries only
 * theString since it selects that one property), the ord-1 seed snapshots
 * project to theString (the Java iterator asserts are theString-only), and the
 * lenient snapshots project to c0 and c1.  Ordered snapshots emit the engine
 * iterator order the Java exact-order iterator asserts pin; the single
 * mode-any snapshot (windowFour, asserted any-order in Java) emits the
 * canonical marshaled-fields row order.
 */
public final class InfraNamedWindowInsertFromScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-insert-from";
    private static final String DESCRIPTION =
            "InfraNamedWindowInsertFrom insert-from-window semantics: type-by-window "
                    + "creation after an existing named window with shared insert delivery, seeded "
                    + "keep-all and filtered and unique-index create-window-insert with deploy-time "
                    + "row copying that never reaches create-statement listeners, filtered routing "
                    + "inserts into each window, and lenient partial-column inserts over map and "
                    + "object-array schemas, captured from window listeners and ordered or canonical "
                    + "mode-any snapshots projected to the Java-asserted fields (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowInsertFrom.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-b3f6cb7b36c5211c8822",
            "java-runtime-e601b3cc7f827d578185",
            "java-runtime-46011542d6e9d34a87f5",
            "java-runtime-8138dd777290d00417d1"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraCreateNamedAfterNamed",
            "InfraInsertWhereTypeAndFilter",
            "InfraNamedWindowInsertLenientPropCount{rep=MAP}",
            "InfraNamedWindowInsertLenientPropCount{rep=OBJECTARRAY}"
    };
    private static final String[] STATIC_IDS = {
            "java-b0917219c462cba9d770",
            "java-cff4a3193da2b063a351",
            "java-282d7b64878866ea4709",
            "java-282d7b64878866ea4709"
    };

    private static final String CASE_CREATE_AFTER_NAMED = "create-after-named";
    private static final String CASE_INSERT_WHERE_TYPE_FILTER = "insert-where-type-filter";
    private static final String CASE_LENIENT_MAP = "lenient-map";
    private static final String CASE_LENIENT_OBJECTARRAY = "lenient-objectarray";

    // Transcriptions of InfraNamedWindowInsertFrom lines 89-92, 109-110,
    // 132, 143, 153, 162-165, and 62-65, without the statement-terminating
    // ";\n".  The suite compiles each execution as one module, while this
    // scenario deploys steps as separate modules, so window-referencing
    // creates add @public for cross-module visibility (same convention as the
    // sibling infra-named-window oracles); visibility has no observable
    // effect.  The ord-1 and lenient sources already carry @public on the
    // referencing statements, so those deploys are verbatim source text.  The
    // case pins keep the Java source text of the execution's first statement.
    private static final String EPL_CREATE_WINDOW_ONE =
            "@name('windowOne') @public create window MyWindow#keepall as SupportBean";
    /**
     * Forced whitespace-only transcription: the source writes
     * "@name('windowTwo')create window MyWindowTwo#keepall as MyWindow"
     * (InfraCreateNamedAfterNamed line 90) with no space after the
     * annotation; inserting the @public deployment-convention annotation
     * requires a separating space.  The normalized text is pinned identically
     * in the scenario and this oracle, and whitespace between annotations has
     * no observable effect.
     */
    private static final String EPL_CREATE_WINDOW_TWO =
            "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindow";
    private static final String EPL_INSERT =
            "insert into MyWindow select * from SupportBean";
    private static final String EPL_SELECT_ONE =
            "@name('selectOne') select theString from MyWindow";
    private static final String EPL_CREATE_IWT =
            "@name('window') @public create window MyWindowIWT#keepall as SupportBean";
    private static final String EPL_INSERT_IWT =
            "insert into MyWindowIWT select * from SupportBean(intPrimitive > 0)";
    private static final String EPL_CREATE_IWT_TWO =
            "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindowIWT insert";
    private static final String EPL_CREATE_IWT_THREE =
            "@name('windowThree') @public create window MyWindowThree#keepall"
                    + " as MyWindowIWT insert where theString like 'A%'";
    private static final String EPL_CREATE_IWT_FOUR =
            "@name('windowFour') @public create window MyWindowFour#unique(intPrimitive)"
                    + " as MyWindowIWT insert";
    private static final String EPL_INSERT_IWT_A =
            "insert into MyWindowIWT select * from SupportBean(theString like 'A%')";
    private static final String EPL_INSERT_IWT_B =
            "insert into MyWindowTwo select * from SupportBean(theString like 'B%')";
    private static final String EPL_INSERT_IWT_C =
            "insert into MyWindowThree select * from SupportBean(theString like 'C%')";
    private static final String EPL_INSERT_IWT_D =
            "insert into MyWindowFour select * from SupportBean(theString like 'D%')";
    private static final String EPL_SCHEMA_MAP =
            "@public create MAP schema MyTwoColEvent(c0 string, c1 int)";
    private static final String EPL_SCHEMA_OBJECTARRAY =
            "@public create OBJECTARRAY schema MyTwoColEvent(c0 string, c1 int)";
    private static final String EPL_WINDOW_TWO_COL =
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent";
    private static final String EPL_INSERT_ONE =
            "insert into MyWindow select theString as c0 from SupportBean";
    private static final String EPL_INSERT_TWO =
            "insert into MyWindow select id as c1 from SupportBean_S0";

    // Case pins: the execution's first statement exactly as the Java source
    // writes it, without the @public deployment-convention annotation (the
    // ord-1 and lenient sources already carry @public, so their pins equal
    // their deploys).
    private static final String[] SOURCE_EPLS = {
            "@name('windowOne') create window MyWindow#keepall as SupportBean",
            "@name('window') @public create window MyWindowIWT#keepall as SupportBean",
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent",
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent"
    };

    /**
     * Listener discipline: the create-after-named case listens only to
     * windowOne and selectOne (mirroring addListener("selectOne")
     * .addListener("windowOne") at InfraNamedWindowInsertFrom line 93) and the
     * insert-where-type-filter case only to window, windowTwo, windowThree,
     * and windowFour (lines 111, 133, 144, and 154).  The lenient executions
     * never attach listeners.  Listeners attach when their deployment
     * completes: the seeded create-window-insert deploys copy rows during
     * deployment, before their listeners attach, so those deliveries never
     * reach the listener (the source pins this with assertListenerNotInvoked
     * at lines 135, 146, and 156).
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        listened.put(CASE_CREATE_AFTER_NAMED,
                new HashSet<>(Arrays.asList("selectOne", "windowOne")));
        listened.put(CASE_INSERT_WHERE_TYPE_FILTER,
                new HashSet<>(Arrays.asList("window", "windowTwo", "windowThree", "windowFour")));
        listened.put(CASE_LENIENT_MAP, Collections.emptySet());
        listened.put(CASE_LENIENT_OBJECTARRAY, Collections.emptySet());
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    /**
     * Row projections: listener rows of the two listening cases render
     * intPrimitive and theString sorted (the rows that decide the filters and
     * the unique index; the Java asserts read theString), except the ord-0
     * selectOne row, whose statement selects only theString so the projection
     * is theString alone.  The ord-1 seed snapshots project to theString (the
     * Java iterator asserts are theString-only, lines 134, 145, and 155) and
     * the lenient snapshots project to c0 and c1 (lines 70 and 75).
     */
    private static final String[] FIELDS_BEAN = {"intPrimitive", "theString"};
    private static final String[] FIELDS_STRING = {"theString"};
    private static final String[] FIELDS_TWO_COL = {"c0", "c1"};

    private static final int EXPECTED_RECORDS = 18;
    private static final int EXPECTED_STEPS = 50;

    private InfraNamedWindowInsertFromScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowInsertFromScenarioOracle <scenario.json>");
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
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_CREATE_AFTER_NAMED, configuration, runtime, allSteps, records);
            runCase(CASE_INSERT_WHERE_TYPE_FILTER, configuration, runtime, allSteps, records);
            runCase(CASE_LENIENT_MAP, configuration, runtime, allSteps, records);
            runCase(CASE_LENIENT_OBJECTARRAY, configuration, runtime, allSteps, records);
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

    /**
     * Replays one case's steps on the shared runtime; sequences restart per case.
     * Compiles with CompilerArguments(configuration) plus the runtime path (the
     * sibling-oracle convention): the configuration makes the base event types
     * resolvable to the compiler, and the runtime path carries the prior
     * deployments' public types so cross-module named-window and schema
     * references (create-window-as, insert-from, two-column windows) resolve.
     * No plugin function is needed for these executions.
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
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
            switch (string(step, "op")) {
                case "deploy": {
                    CompilerArguments compilerArgs = new CompilerArguments(configuration);
                    compilerArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        statementsByName.put(statement.getName(), statement);
                        if (LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet())
                                .contains(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                    }
                    break;
                }
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
                    JsonValue modeValue = step.get("mode");
                    boolean canonical = modeValue != null && "any".equals(string(step, "mode"));
                    records.add(snapshot(runtime, statement, caseName, canonical));
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty.  Window insert
     * deliveries are new-only; the projection follows the listened statement
     * (selectOne renders theString alone, the create-window statements render
     * intPrimitive and theString).
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents, listenerFields(caseName, statement.getName()));
            JsonArray oldRows = rows(oldEvents, listenerFields(caseName, statement.getName()));
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
     * Snapshot of the statement iterator: one record with sequence 0.  Ordered
     * snapshots emit the engine iterator order that the Java exact-order
     * iterator asserts pin (keepall iteration is insertion order); the single
     * mode-any snapshot (windowFour, asserted any-order at InfraNamedWindowInsertFrom
     * line 155) emits the canonical row order — the ascending marshaled-fields
     * string with sorted keys, identical to the Go runner and to
     * compat.CanonicalTrace's mode-any normalization — so the checked-in trace
     * is stable against engine iteration order on both sides.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       boolean canonical) {
        String[] fields = snapshotFields(caseName);
        List<JsonObject> projected = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            projected.add(projectedRow(iterator.next(), fields));
        }
        if (canonical) {
            projected.sort(Comparator.comparing(row -> row.get("fields").asObject().toString()));
        }
        JsonArray rows = new JsonArray();
        for (JsonObject row : projected) {
            rows.add(row);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("new", rows);
        return record;
    }

    private static String[] listenerFields(String caseName, String statementName) {
        if (CASE_CREATE_AFTER_NAMED.equals(caseName)) {
            return "selectOne".equals(statementName) ? FIELDS_STRING : FIELDS_BEAN;
        }
        if (CASE_INSERT_WHERE_TYPE_FILTER.equals(caseName)) {
            return FIELDS_BEAN;
        }
        throw new IllegalStateException("case " + caseName + " attaches no listeners");
    }

    private static String[] snapshotFields(String caseName) {
        if (CASE_INSERT_WHERE_TYPE_FILTER.equals(caseName)) {
            return FIELDS_STRING;
        }
        if (CASE_LENIENT_MAP.equals(caseName) || CASE_LENIENT_OBJECTARRAY.equals(caseName)) {
            return FIELDS_TWO_COL;
        }
        throw new IllegalStateException("case " + caseName + " takes no snapshots");
    }

    /**
     * Row rendering projected to exactly the fields the Java assertions read,
     * in sorted order; a full window row carries many properties the Java
     * tests never read, so both engines render the projected fields only and
     * the pinned contract is the Java test's projection.
     */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(projectedRow(event, fields));
        }
        return array;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object.
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull()
                        ? null : string(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0((int) longInteger(payload.get("id"), "id")), type);
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
        String[] expectedCases = {
                CASE_CREATE_AFTER_NAMED, CASE_INSERT_WHERE_TYPE_FILTER,
                CASE_LENIENT_MAP, CASE_LENIENT_OBJECTARRAY};
        int[] expectedOrdinals = {0, 1, 5, 6};
        int[] expectedSnapshots = {0, 3, 1, 1};
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != expectedSnapshots[index]
                    || !SOURCE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCreateAfterNamedCase(steps, offset);
        offset = validateInsertWhereTypeFilterCase(steps, offset);
        offset = validateLenientCase(steps, offset, CASE_LENIENT_MAP);
        offset = validateLenientCase(steps, offset, CASE_LENIENT_OBJECTARRAY);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact create-after-named step sequence mirroring InfraCreateNamedAfterNamed
     * lines 89-100: a keepall window created after nothing, a second window
     * typed by the first, the shared insert, and a select over the window; one
     * SupportBean delivers to both the windowOne and selectOne listeners.
     */
    private static int validateCreateAfterNamedCase(JsonArray steps, int offset) {
        String caseName = CASE_CREATE_AFTER_NAMED;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "windowOne", EPL_CREATE_WINDOW_ONE);
        validateDeploy(steps.get(offset++), caseName, "windowTwo", EPL_CREATE_WINDOW_TWO);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        validateDeploy(steps.get(offset++), caseName, "selectOne", EPL_SELECT_ONE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact insert-where-type-filter step sequence mirroring
     * InfraInsertWhereTypeAndFilter lines 109-224: five filtered seed sends,
     * the keep-all, filtered, and unique-index seeded creates (whose listeners
     * see nothing at deploy time), and four filtered routing inserts each
     * delivering exactly one row to its target window's listener.
     */
    private static int validateInsertWhereTypeFilterCase(JsonArray steps, int offset) {
        String caseName = CASE_INSERT_WHERE_TYPE_FILTER;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_IWT);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_IWT);
        validateBeanSend(steps.get(offset++), caseName, "A1", 1);
        validateBeanSend(steps.get(offset++), caseName, "B2", 1);
        validateBeanSend(steps.get(offset++), caseName, "C3", 1);
        validateBeanSend(steps.get(offset++), caseName, "A4", 4);
        validateBeanSend(steps.get(offset++), caseName, "C5", 4);
        validateDeploy(steps.get(offset++), caseName, "windowTwo", EPL_CREATE_IWT_TWO);
        validateSnapshot(steps.get(offset++), caseName, "windowTwo", "ordered");
        validateDeploy(steps.get(offset++), caseName, "windowThree", EPL_CREATE_IWT_THREE);
        validateSnapshot(steps.get(offset++), caseName, "windowThree", "ordered");
        validateDeploy(steps.get(offset++), caseName, "windowFour", EPL_CREATE_IWT_FOUR);
        validateSnapshot(steps.get(offset++), caseName, "windowFour", "any");
        validateDeploy(steps.get(offset++), caseName, "insertA", EPL_INSERT_IWT_A);
        validateDeploy(steps.get(offset++), caseName, "insertB", EPL_INSERT_IWT_B);
        validateDeploy(steps.get(offset++), caseName, "insertC", EPL_INSERT_IWT_C);
        validateDeploy(steps.get(offset++), caseName, "insertD", EPL_INSERT_IWT_D);
        // InfraInsertWhereTypeAndFilter line 171 sends ("B9", -9); the B-prefixed
        // routing row lands in MyWindowTwo only.
        validateBeanSend(steps.get(offset++), caseName, "B9", -9);
        validateBeanSend(steps.get(offset++), caseName, "A8", -8);
        validateBeanSend(steps.get(offset++), caseName, "C7", -7);
        validateBeanSend(steps.get(offset++), caseName, "D6", -6);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact lenient step sequence mirroring InfraNamedWindowInsertLenientPropCount
     * lines 61-77: a two-column schema and keepall window, two partial-column
     * inserts, and one send per insert, each observed through an ordered
     * iterator snapshot showing the unassigned column as null.
     */
    private static int validateLenientCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "schema",
                CASE_LENIENT_MAP.equals(caseName) ? EPL_SCHEMA_MAP : EPL_SCHEMA_OBJECTARRAY);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_WINDOW_TWO_COL);
        validateDeploy(steps.get(offset++), caseName, "insertOne", EPL_INSERT_ONE);
        validateDeploy(steps.get(offset++), caseName, "insertTwo", EPL_INSERT_TWO);
        validateBeanSend(steps.get(offset++), caseName, "E1", 0);
        validateSnapshot(steps.get(offset++), caseName, "window", "ordered");
        validateS0Send(steps.get(offset++), caseName, 10);
        validateSnapshot(steps.get(offset++), caseName, "window", "ordered");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
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

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        JsonValue theString = payload.get("theString");
        boolean stringMatches = expectedString == null
                ? theString.isNull()
                : theString instanceof JsonString && expectedString.equals(theString.asString());
        if (!stringMatches
                || longInteger(payload.get("intPrimitive"), "intPrimitive")
                != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, long expectedId) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (longInteger(payload.get("id"), "id") != expectedId) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
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
