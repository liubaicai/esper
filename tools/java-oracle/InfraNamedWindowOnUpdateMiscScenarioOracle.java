import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
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
import java.io.Serializable;
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
 * Java oracle for InfraNamedWindowOnUpdate misc executions: the on-update
 * variants the sibling infra-named-window-on-update unit does not cover,
 * namely the method-call set-clause update with the update statement's own
 * old/new delivery, the subclass-typed named-window update, the copy-method
 * bean update, and the wrapper-select window update with an extra projected
 * property.
 *
 * Replays the four pinned executions (ordinals 0, 3, 4, and 5 of the suite's
 * executions()) on one runtime with undeployAll between cases, mirroring the
 * regression-suite harness: SupportBean and SupportBean_S0 from esper-common
 * plus local Serializable mirrors of the regression SupportBeanAbstractSub
 * and SupportBeanCopyMethod (regression-lib is not on the oracle classpath;
 * the mirrors keep the real serialVersionUIDs and collapse the inherited v1
 * of SupportBeanAbstractBase into the subclass mirror), internal timer
 * disabled, and the rethrowing exception handler so statement failures
 * surface to the sender thread.  The copy-method event type registers with
 * the same legacy ConfigurationCommonEventTypeBean copy-method descriptor as
 * TestSuiteInfraNamedWindow, and the setBeanLongPrimitive999 plugin
 * single-row function binds to the oracle's own static method exactly like
 * the suite binds the suite-class method.  Listeners attach only to the
 * deployment statement named "update" in the non-property-set case (its own
 * update delivery, asserting both images) and to the statement named "create"
 * in the subclass case; the copy-method and wrapper executions never listen.
 * Listener and snapshot rows are projected to exactly the fields the Java
 * assertions read: intPrimitive and longPrimitive for the update delivery
 * (InfraUpdateNonPropertySet line 149), v1 and v2 for the subclass delivery
 * (InfraSubclass line 221), valOne for the copy-method window
 * (InfraUpdateCopyMethodBean line 126), and theString plus p0 for the
 * wrapper window (InfraUpdateWrapper line 112), since a full window row
 * carries many properties the Java tests never read.  The wrapper create and
 * insert statements deploy as one module because the engine cannot deploy a
 * separate insert module against a wildcard-plus-extra create-window type
 * (see EPL_CREATE_INSERT_WRAPPER).  Snapshot rows are mode-any pins whose
 * rows the differential canonicalization sorts.
 */
public final class InfraNamedWindowOnUpdateMiscScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-on-update-misc";
    private static final String DESCRIPTION =
            "InfraNamedWindowOnUpdate on-update variants: method-call set clauses ported as "
                    + "approved-difference literal assignments with the update statement's own old/new "
                    + "delivery, subclass-typed named-window updates through a collapsed Go struct, "
                    + "copy-method bean updates under the field-wise-copy approved difference, and "
                    + "wrapper-select windows with an extra projected property, captured from "
                    + "update/create listeners and window snapshots projected to the Java-asserted "
                    + "fields (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnUpdate.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-5d041a3958410a90fa9a",
            "java-runtime-9ea51cb84d163be79693",
            "java-runtime-43feff597147c7867ca7",
            "java-runtime-09f56f49bab53388bb2d"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraUpdateNonPropertySet",
            "InfraSubclass",
            "InfraUpdateCopyMethodBean",
            "InfraUpdateWrapper"
    };
    private static final String[] STATIC_IDS = {
            "java-26306de901985e70904f",
            "java-d920e63f2081f1d72d2f",
            "java-6eb1ea7dc80c3078f47f",
            "java-6b566143dbbb00ea202b"
    };

    private static final String CASE_NON_PROPERTY_SET = "non-property-set";
    private static final String CASE_SUBCLASS = "subclass";
    private static final String CASE_COPY_METHOD = "copy-method";
    private static final String CASE_WRAPPER = "wrapper";

    // Transcriptions of InfraNamedWindowOnUpdate lines 138-143, 212-214,
    // 120-122, and 106-108, without the statement-terminating ";\n".  The
    // suite compiles create, insert, and update as one module, while this
    // scenario deploys them as separate modules, so the create-window
    // statements add @public for cross-module visibility (same convention as
    // the sibling infra-named-window-on-update oracle); visibility has no
    // observable effect.  The case pins keep the Java source text without
    // @public.
    private static final String EPL_CREATE_NON_PROPERTY_SET =
            "@public create window MyWindowUNP#keepall as SupportBean";
    private static final String EPL_INSERT_NON_PROPERTY_SET =
            "insert into MyWindowUNP select * from SupportBean";
    private static final String EPL_UPDATE_NON_PROPERTY_SET =
            "@name('update') on SupportBean_S0 as sb update MyWindowUNP as mywin"
                    + " set mywin.setIntPrimitive(10),"
                    + "     setBeanLongPrimitive999(mywin)";
    private static final String EPL_CREATE_SUBCLASS =
            "@name('create') @public create window MyWindowSC#keepall"
                    + " as select * from SupportBeanAbstractSub";
    private static final String EPL_INSERT_SUBCLASS =
            "insert into MyWindowSC select * from SupportBeanAbstractSub";
    private static final String EPL_UPDATE_SUBCLASS =
            "on SupportBean update MyWindowSC set v1=theString, v2=theString";
    private static final String EPL_CREATE_COPY_METHOD =
            "@name('window') @public create window MyWindowBeanCopyMethod#keepall"
                    + " as SupportBeanCopyMethod";
    private static final String EPL_INSERT_COPY_METHOD =
            "insert into MyWindowBeanCopyMethod select * from SupportBeanCopyMethod";
    private static final String EPL_UPDATE_COPY_METHOD =
            "on SupportBean update MyWindowBeanCopyMethod set valOne = 'x'";
    private static final String EPL_CREATE_WRAPPER =
            "@name('window') @public create window MyWindow#keepall"
                    + " as select *, 1 as p0 from SupportBean";
    private static final String EPL_INSERT_WRAPPER =
            "insert into MyWindow select *, 2 as p0 from SupportBean";
    private static final String EPL_UPDATE_WRAPPER =
            "on SupportBean_S0 update MyWindow set theString = 'x', p0 = 2";

    /**
     * Forced deviation from the sibling units' one-statement-per-deploy
     * convention: the wrapper create and insert deploy as one module.  With a
     * separate insert module the engine records a public event-type dependency
     * on MyWindow, and a create-window-as-select with a wildcard plus extra
     * projected properties never registers its type in the bus event-type
     * repository, so the insert deploy fails ("Failed to find event type
     * 'MyWindow' ..."); without @public the insert deploy succeeds but the
     * cross-module update statement can no longer resolve the named window.
     * The suite itself compiles all three statements as one module, and the
     * merged module keeps the update statement separate (it resolves the named
     * window through the path), so this bundling has no observable effect.
     */
    private static final String EPL_CREATE_INSERT_WRAPPER =
            EPL_CREATE_WRAPPER + ";\n" + EPL_INSERT_WRAPPER;

    // Case pins: the observed statement's EPL exactly as the Java source
    // writes it, without the @public deployment-convention annotation.
    private static final String[] SOURCE_EPLS = {
            EPL_UPDATE_NON_PROPERTY_SET,
            "@name('create') create window MyWindowSC#keepall as select * from SupportBeanAbstractSub",
            "@name('window') create window MyWindowBeanCopyMethod#keepall as SupportBeanCopyMethod",
            "@name('window') create window MyWindow#keepall as select *, 1 as p0 from SupportBean"
    };

    /**
     * Listener discipline: only the non-property-set case listens, and only
     * to its "update" statement (its own new/old delivery, mirroring
     * addListener("update") at InfraNamedWindowOnUpdate line 144), and the
     * subclass case listens only to its "create" statement (mirroring
     * addListener("create") at line 215).  The copy-method and wrapper
     * executions never attach listeners.
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        listened.put(CASE_NON_PROPERTY_SET, new HashSet<>(Arrays.asList("update")));
        listened.put(CASE_SUBCLASS, new HashSet<>(Arrays.asList("create")));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    private static final int EXPECTED_RECORDS = 5;
    private static final int EXPECTED_STEPS = 29;

    private InfraNamedWindowOnUpdateMiscScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOnUpdateMiscScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBeanAbstractSub.class);
        // Transcription of TestSuiteInfraNamedWindow: the copy-method bean
        // event type registers with a legacy bean-type descriptor naming the
        // copy method the engine's update copy-on-write invokes.
        ConfigurationCommonEventTypeBean copyMethodLegacy = new ConfigurationCommonEventTypeBean();
        copyMethodLegacy.setCopyMethod("myCopyMethod");
        configuration.getCommon().addEventType(
                "SupportBeanCopyMethod", SupportBeanCopyMethod.class.getName(), copyMethodLegacy);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        // Transcription of TestSuiteInfraNamedWindow: the suite registers the
        // suite class's static setBeanLongPrimitive999 as a plugin single-row
        // function; the oracle binds the same function name to its own static
        // method below.
        configuration.getCompiler().addPlugInSingleRowFunction("setBeanLongPrimitive999",
                InfraNamedWindowOnUpdateMiscScenarioOracle.class.getName(),
                "setBeanLongPrimitive999");
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_NON_PROPERTY_SET, configuration, runtime, allSteps, records);
            runCase(CASE_SUBCLASS, configuration, runtime, allSteps, records);
            runCase(CASE_COPY_METHOD, configuration, runtime, allSteps, records);
            runCase(CASE_WRAPPER, configuration, runtime, allSteps, records);
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

    /** Plugin single-row function backing setBeanLongPrimitive999(mywin). */
    public static void setBeanLongPrimitive999(SupportBean event) {
        event.setLongPrimitive(999);
    }

    /**
     * Replays one case's steps on the shared runtime; sequences restart per case.
     * Compiles against the full configuration, not just the runtime path: the
     * runtime-path-only CompilerArguments constructor carries an empty
     * configuration, so the configured plugin single-row function would not be
     * visible (the same convention the transpose-stream oracle uses).
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
                    records.add(snapshot(runtime, statement, caseName));
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
     * counter; new and old arrays render only when non-empty.  The update
     * statement's own delivery carries the post-image as new and the
     * pre-image as old; whatever the engine delivers on the subclass create
     * statement is recorded verbatim.
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
            JsonArray newRows = rows(newEvents, caseName);
            JsonArray oldRows = rows(oldEvents, caseName);
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
     * Snapshot of the statement iterator: one record with sequence 0 holding
     * the rows in the differential protocol's canonical order, mirroring the
     * generic differential replay convention where listener records carry
     * per-statement sequences and snapshot records do not.  Both snapshots in
     * this scenario are mode-any pins whose rows sort by their ascending
     * marshaled-fields string — the same canonical key the differential
     * canonicalization applies — so the checked-in traces are stable against
     * engine iteration order on both sides.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        List<JsonObject> projected = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            projected.add(projectedRow(iterator.next(), caseName));
        }
        // Mode-any pins: emit the canonical row order (ascending marshaled
        // fields string with sorted keys), identical to the Go runner and to
        // compat.CanonicalTrace's mode-any normalization.
        projected.sort(Comparator.comparing(row -> row.get("fields").asObject().toString()));
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

    /**
     * Row rendering projected to only the fields the Java test asserts: the
     * non-property-set update delivery asserts intPrimitive and longPrimitive
     * (InfraUpdateNonPropertySet line 149), the subclass delivery asserts v1
     * and v2 (InfraSubclass line 221), the copy-method window asserts valOne
     * (InfraUpdateCopyMethodBean line 126), and the wrapper window asserts
     * theString and p0 (InfraUpdateWrapper line 112).  A full window row
     * carries many properties the Java assertions never read, so both engines
     * render the projected fields only and the pinned contract is the Java
     * test's projection.  Field names are sorted for a stable order (the
     * wrapper Java assertion order is theString,p0; rows render p0 first).
     */
    private static JsonObject projectedRow(EventBean event, String caseName) {
        JsonObject fields = new JsonObject();
        if (CASE_NON_PROPERTY_SET.equals(caseName)) {
            fields.add("intPrimitive", normalize(event.get("intPrimitive")));
            fields.add("longPrimitive", normalize(event.get("longPrimitive")));
        } else if (CASE_SUBCLASS.equals(caseName)) {
            fields.add("v1", normalize(event.get("v1")));
            fields.add("v2", normalize(event.get("v2")));
        } else if (CASE_COPY_METHOD.equals(caseName)) {
            fields.add("valOne", normalize(event.get("valOne")));
        } else {
            fields.add("p0", normalize(event.get("p0")));
            fields.add("theString", normalize(event.get("theString")));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(EventBean[] events, String caseName) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(projectedRow(event, caseName));
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
            case "SupportBeanAbstractSub": {
                // The regression constructor takes only v2 and leaves the
                // inherited v1 null, so the payload must carry v1: null.
                if (!payload.get("v1").isNull()) {
                    throw new IllegalArgumentException(
                            "SupportBeanAbstractSub payload v1 must be null");
                }
                runtime.getEventService().sendEventBean(
                        new SupportBeanAbstractSub(string(payload, "v2")), type);
                break;
            }
            case "SupportBeanCopyMethod": {
                runtime.getEventService().sendEventBean(new SupportBeanCopyMethod(
                        string(payload, "valOne"), string(payload, "valTwo")), type);
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
                CASE_NON_PROPERTY_SET, CASE_SUBCLASS, CASE_COPY_METHOD, CASE_WRAPPER};
        int[] expectedOrdinals = {0, 3, 4, 5};
        int[] expectedSnapshots = {0, 0, 1, 1};
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
        offset = validateNonPropertySetCase(steps, offset);
        offset = validateSubclassCase(steps, offset);
        offset = validateCopyMethodCase(steps, offset);
        offset = validateWrapperCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact non-property-set step sequence mirroring InfraUpdateNonPropertySet
     * lines 137-152: a SupportBean insert then the SupportBean_S0 trigger of
     * the method-call set-clause update whose own listener receives the
     * post-image as new and the pre-image as old.
     */
    private static int validateNonPropertySetCase(JsonArray steps, int offset) {
        String caseName = CASE_NON_PROPERTY_SET;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_NON_PROPERTY_SET);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_NON_PROPERTY_SET);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_NON_PROPERTY_SET);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateS0Send(steps.get(offset++), caseName, 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact subclass step sequence mirroring InfraSubclass lines 211-224: a
     * SupportBeanAbstractSub insert (inherited v1 null) then the SupportBean
     * trigger of the v1/v2 update; the create-statement listener receives the
     * insert delivery and then the update delivery.
     */
    private static int validateSubclassCase(JsonArray steps, int offset) {
        String caseName = CASE_SUBCLASS;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_SUBCLASS);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_SUBCLASS);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_SUBCLASS);
        validateAbstractSubSend(steps.get(offset++), caseName, "value2");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact copy-method step sequence mirroring InfraUpdateCopyMethodBean
     * lines 119-129: a SupportBeanCopyMethod insert then the default
     * SupportBean trigger of the valOne update under the configured
     * myCopyMethod copy, observed through a mode-any iterator snapshot.
     */
    private static int validateCopyMethodCase(JsonArray steps, int offset) {
        String caseName = CASE_COPY_METHOD;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_COPY_METHOD);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_COPY_METHOD);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_COPY_METHOD);
        validateCopyMethodSend(steps.get(offset++), caseName, "a", "b");
        validateBeanSend(steps.get(offset++), caseName, null, 0);
        validateSnapshot(steps.get(offset++), caseName, "window", "any");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact wrapper step sequence mirroring InfraUpdateWrapper lines 105-114:
     * the create and insert statements deploy as one module (see
     * EPL_CREATE_INSERT_WRAPPER for why the bundling is forced), the
     * SupportBean_S0-triggered theString/p0 update deploys separately, and a
     * mode-any iterator snapshot observes the updated window.
     */
    private static int validateWrapperCase(JsonArray steps, int offset) {
        String caseName = CASE_WRAPPER;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_INSERT_WRAPPER);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_WRAPPER);
        validateBeanSend(steps.get(offset++), caseName, "E1", 100);
        validateS0Send(steps.get(offset++), caseName, -1);
        validateSnapshot(steps.get(offset++), caseName, "window", "any");
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

    /** The theString pin is null for the bare copy-method trigger bean. */
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

    private static void validateAbstractSubSend(JsonValue value, String caseName,
                                                String expectedV2) {
        JsonObject step = object(value, "SupportBeanAbstractSub step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBeanAbstractSub".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBeanAbstractSub step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBeanAbstractSub payload");
        requireFields(payload, "v1", "v2");
        if (!payload.get("v1").isNull() || !expectedV2.equals(string(payload, "v2"))) {
            throw new IllegalArgumentException(
                    "SupportBeanAbstractSub payload is not pinned for " + caseName);
        }
    }

    private static void validateCopyMethodSend(JsonValue value, String caseName,
                                               String expectedValOne, String expectedValTwo) {
        JsonObject step = object(value, "SupportBeanCopyMethod step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBeanCopyMethod".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBeanCopyMethod step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBeanCopyMethod payload");
        requireFields(payload, "valOne", "valTwo");
        if (!expectedValOne.equals(string(payload, "valOne"))
                || !expectedValTwo.equals(string(payload, "valTwo"))) {
            throw new IllegalArgumentException(
                    "SupportBeanCopyMethod payload is not pinned for " + caseName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        if (expectedMode == null) {
            requireFields(step, "op", "case", "statement");
        } else {
            requireFields(step, "op", "case", "statement", "mode");
        }
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        if (expectedMode != null && !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot mode is not pinned for " + caseName + "/"
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

    /**
     * Local mirror of the regression SupportBeanAbstractSub (regression-lib is
     * not on the oracle classpath) with the regression bean's real
     * serialVersionUID; the inherited v1 of SupportBeanAbstractBase collapses
     * into this mirror because the subclass execution only reads and writes
     * v1 and v2.  Serializable like the regression bean: the engine's update
     * copy-on-write for bean event types requires a Serializable underlying or
     * a configured copy method.
     */
    public static final class SupportBeanAbstractSub implements Serializable {
        private static final long serialVersionUID = -196620234553104260L;
        private String v1;
        private String v2;

        public SupportBeanAbstractSub(String v2) {
            this.v2 = v2;
        }

        public String getV1() {
            return v1;
        }

        public void setV1(String v1) {
            this.v1 = v1;
        }

        public String getV2() {
            return v2;
        }

        public void setV2(String v2) {
            this.v2 = v2;
        }
    }

    /**
     * Local mirror of the regression SupportBeanCopyMethod (regression-lib is
     * not on the oracle classpath) with the regression bean's real
     * serialVersionUID and the same myCopyMethod copy method the suite's
     * legacy ConfigurationCommonEventTypeBean descriptor names; the copy is
     * field-wise exactly like the regression bean's.
     */
    public static final class SupportBeanCopyMethod implements Serializable {
        private static final long serialVersionUID = -5033276410791065014L;
        private String valOne;
        private String valTwo;

        public SupportBeanCopyMethod(String valOne, String valTwo) {
            this.valOne = valOne;
            this.valTwo = valTwo;
        }

        public String getValOne() {
            return valOne;
        }

        public void setValOne(String valOne) {
            this.valOne = valOne;
        }

        public String getValTwo() {
            return valTwo;
        }

        public void setValTwo(String valTwo) {
            this.valTwo = valTwo;
        }

        public SupportBeanCopyMethod myCopyMethod() {
            return new SupportBeanCopyMethod(valOne, valTwo);
        }
    }
}
