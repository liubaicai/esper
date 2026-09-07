import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_A;
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
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNamedWindowOnUpdate: on-update statements over named
 * windows, covering the intersect and union composite windows with update
 * old/new listener delivery and iterator state, and multikey-with-array
 * update matching through single-field and two-field int[] deep-equality
 * where clauses.
 *
 * Replays the four pinned executions (ordinals 1, 2, 6, and 7 of the suite's
 * executions()) on one runtime with undeployAll between cases, mirroring the
 * regression-suite harness: SupportBean and SupportBean_A from esper-common
 * plus local mirrors of the regression SupportEventWithManyArray and
 * SupportEventWithIntArray (regression-lib is not on the oracle classpath),
 * internal timer disabled, and the rethrowing exception handler so statement
 * failures surface to the sender thread.  Listeners attach to create-named
 * statements only in the cases whose Java executions call
 * addListener("create") (intersect line 160 and union line 187; the multikey
 * executions never listen) with per-case per-statement sequence counters.
 * Listener and snapshot rows are projected to exactly the fields the Java
 * assertions read: theString and intPrimitive for intersect and union, id and
 * value for the multikey cases, since a full SupportEventWithManyArray row
 * has ~27 mostly-null properties the Java tests never read.  The union
 * snapshot sorts the projected rows by theString like
 * EPAssertionUtil.sort(iterator, "theString") in InfraMultipleDataWindowUnion,
 * while the intersect and multikey snapshots are mode-any pins whose rows the
 * differential canonicalization sorts.
 */
public final class InfraNamedWindowOnUpdateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-on-update";
    private static final String DESCRIPTION =
            "InfraNamedWindowOnUpdate on-update statements over named windows: intersect and union "
                    + "composite windows with update old/new listener delivery and iterator state, plus "
                    + "multikey-with-array update matching through single-field and two-field int[] "
                    + "deep-equality where clauses, captured from create-statement listeners and window "
                    + "snapshots projected to the Java-asserted fields (Java source regression-lib/src/main/"
                    + "java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnUpdate.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnUpdate.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-f921cf2543cbb10b4150",
            "java-runtime-0947ec873ea298b60209",
            "java-runtime-9870ee394d3099ef85e8",
            "java-runtime-a25a18f2754aeae08015"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraMultipleDataWindowIntersect",
            "InfraMultipleDataWindowUnion",
            "InfraUpdateMultikeyWArrayPrimitiveArray",
            "InfraUpdateMultikeyWArrayTwoFields"
    };
    private static final String[] STATIC_IDS = {
            "java-b0afef60c0bc90fc51d8",
            "java-a5f1d32f26cb39f39266",
            "java-0b7af7284d24833e1753",
            "java-cdea340e2ff2cabfd0f3"
    };

    private static final String CASE_INTERSECT = "intersect";
    private static final String CASE_UNION = "union";
    private static final String CASE_MULTIKEY_ARRAY = "multikey-array";
    private static final String CASE_MULTIKEY_TWO_FIELDS = "multikey-two-fields";

    // Transcriptions of InfraNamedWindowOnUpdate lines 157-159, 184-186,
    // 78-82, and 49-53, without the statement-terminating ";\n".  The suite
    // compiles create, insert, and update as one module, while this scenario
    // deploys them as separate modules, so the create-window statements add
    // @public for cross-module visibility (same convention as the
    // infra-named-window-join oracle); visibility has no observable effect.
    private static final String EPL_CREATE_INTERSECT =
            "@name('create') @public create window MyWindowMDW#unique(theString)#length(2) "
                    + "as select * from SupportBean";
    private static final String EPL_INSERT_INTERSECT = "insert into MyWindowMDW select * from SupportBean";
    private static final String EPL_UPDATE_INTERSECT =
            "on SupportBean_A update MyWindowMDW set intPrimitive=intPrimitive*100 where theString=id";
    private static final String EPL_CREATE_UNION =
            "@name('create') @public create window MyWindowMU#unique(theString)#length(2) retain-union "
                    + "as select * from SupportBean";
    private static final String EPL_INSERT_UNION = "insert into MyWindowMU select * from SupportBean";
    private static final String EPL_UPDATE_UNION =
            "on SupportBean_A update MyWindowMU mw set mw.intPrimitive=intPrimitive*100 where theString=id";
    private static final String EPL_CREATE_MULTIKEY =
            "@name('create') @public create window MyWindow#keepall as SupportEventWithManyArray";
    private static final String EPL_INSERT_MULTIKEY =
            "insert into MyWindow select * from SupportEventWithManyArray";
    private static final String EPL_UPDATE_MULTIKEY_ARRAY =
            "on SupportEventWithIntArray as sewia update MyWindow as mw set value = sewia.value "
                    + "where mw.intOne = sewia.array";
    private static final String EPL_UPDATE_MULTIKEY_TWO_FIELDS =
            "on SupportEventWithIntArray as sewia update MyWindow as mw set value = sewia.value "
                    + "where mw.id = sewia.id and mw.intOne = sewia.array";

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("create"));
    private static final Set<String> LISTENED_CASES = new HashSet<>(
            Arrays.asList(CASE_INTERSECT, CASE_UNION));
    private static final int EXPECTED_RECORDS = 10;
    private static final int EXPECTED_STEPS = 46;

    private InfraNamedWindowOnUpdateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOnUpdateScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportEventWithManyArray.class);
        configuration.getCommon().addEventType(SupportEventWithIntArray.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_INTERSECT, runtime, allSteps, records);
            runCase(CASE_UNION, runtime, allSteps, records);
            runCase(CASE_MULTIKEY_ARRAY, runtime, allSteps, records);
            runCase(CASE_MULTIKEY_TWO_FIELDS, runtime, allSteps, records);
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

    /** Replays one case's steps on the shared runtime; sequences restart per case. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
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
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        statementsByName.put(statement.getName(), statement);
                        if (LISTENED_CASES.contains(caseName)
                                && LISTENED_STATEMENTS.contains(statement.getName())) {
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
     * counter; new and old arrays render only when non-empty.
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
     * per-statement sequences and snapshot records do not.  The union snapshot
     * sorts by theString (the ordered pin); mode-any snapshots sort rows by
     * their ascending marshaled-fields string — the same canonical key the
     * differential canonicalization applies — so the checked-in traces are
     * stable against engine iteration order on both sides.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        List<JsonObject> projected = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            projected.add(projectedRow(iterator.next(), caseName));
        }
        if (CASE_UNION.equals(caseName)) {
            // Mirrors EPAssertionUtil.sort(iterator, "theString") in
            // InfraMultipleDataWindowUnion lines 201-204: the union snapshot
            // pin is ordered by theString ascending, unlike the mode-any pins.
            projected.sort(Comparator.comparing(
                    InfraNamedWindowOnUpdateScenarioOracle::theStringOf));
        } else {
            // Mode-any pins: emit the canonical row order (ascending marshaled
            // fields string with sorted keys), identical to the Go runner and
            // to compat.CanonicalTrace's mode-any normalization.
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

    /**
     * Row rendering projected to only the fields the Java test asserts: the
     * intersect and union executions assert theString and intPrimitive
     * (InfraMultipleDataWindowIntersect lines 170-176 and
     * InfraMultipleDataWindowUnion lines 197-203) while the multikey
     * executions assert id and value (lines 68-69 and 97-98).  A full
     * SupportEventWithManyArray row has ~27 mostly-null properties the Java
     * assertions never read, so both engines render the projected fields only
     * and the pinned contract is the Java test's projection.  Field names are
     * sorted for a stable order.
     */
    private static JsonObject projectedRow(EventBean event, String caseName) {
        JsonObject fields = new JsonObject();
        if (CASE_INTERSECT.equals(caseName) || CASE_UNION.equals(caseName)) {
            fields.add("intPrimitive", normalize(event.get("intPrimitive")));
            fields.add("theString", normalize(event.get("theString")));
        } else {
            fields.add("id", normalize(event.get("id")));
            fields.add("value", normalize(event.get("value")));
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

    private static String theStringOf(JsonObject row) {
        return row.get("fields").asObject().get("theString").asString();
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
                bean.setTheString(string(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(string(payload, "id")), type);
                break;
            }
            case "SupportEventWithManyArray": {
                // value defaults to 0 on construction exactly like the
                // regression bean, so the payload omits it.
                SupportEventWithManyArray bean = new SupportEventWithManyArray(string(payload, "id"));
                bean.setIntOne(intArray(payload.get("intOne"), "intOne"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportEventWithIntArray": {
                SupportEventWithIntArray bean = new SupportEventWithIntArray(
                        string(payload, "id"),
                        intArray(payload.get("array"), "array"),
                        (int) longInteger(payload.get("value"), "value"));
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
        String[] expectedCases = {
                CASE_INTERSECT, CASE_UNION, CASE_MULTIKEY_ARRAY, CASE_MULTIKEY_TWO_FIELDS};
        int[] expectedOrdinals = {1, 2, 6, 7};
        int[] expectedSnapshots = {1, 1, 1, 1};
        String[] expectedEpls = {
                EPL_CREATE_INTERSECT, EPL_CREATE_UNION, EPL_CREATE_MULTIKEY, EPL_CREATE_MULTIKEY};
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
        offset = validateIntersectCase(steps, offset);
        offset = validateUnionCase(steps, offset);
        offset = validateMultikeyArrayCase(steps, offset);
        offset = validateTwoFieldsCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact intersect-case step sequence mirroring InfraMultipleDataWindowIntersect
     * lines 157-178: two SupportBean inserts then the SupportBean_A update
     * trigger, with a mode-any iterator snapshot of the create window.
     */
    private static int validateIntersectCase(JsonArray steps, int offset) {
        String caseName = CASE_INTERSECT;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_INTERSECT);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_INTERSECT);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_INTERSECT);
        validateBeanSend(steps.get(offset++), caseName, "E1", 2);
        validateBeanSend(steps.get(offset++), caseName, "E2", 3);
        validateASend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact union-case step sequence mirroring InfraMultipleDataWindowUnion
     * lines 184-206: same send pattern as the intersect case but with the
     * retain-union window and an ordered iterator snapshot.
     */
    private static int validateUnionCase(JsonArray steps, int offset) {
        String caseName = CASE_UNION;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_UNION);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_UNION);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_UNION);
        validateBeanSend(steps.get(offset++), caseName, "E1", 2);
        validateBeanSend(steps.get(offset++), caseName, "E2", 3);
        validateASend(steps.get(offset++), caseName, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create", null);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact multikey-array case step sequence mirroring
     * InfraUpdateMultikeyWArrayPrimitiveArray lines 78-100: four
     * SupportEventWithManyArray inserts (value defaulting to 0) and four
     * SupportEventWithIntArray update triggers, then a mode-any iterator
     * snapshot of the single-field int[] update results.
     */
    private static int validateMultikeyArrayCase(JsonArray steps, int offset) {
        String caseName = CASE_MULTIKEY_ARRAY;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_MULTIKEY);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_MULTIKEY);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_MULTIKEY_ARRAY);
        validateManyArraySend(steps.get(offset++), caseName, "E1", new int[]{1, 2});
        validateManyArraySend(steps.get(offset++), caseName, "E2", new int[]{3, 4});
        validateManyArraySend(steps.get(offset++), caseName, "E3", new int[]{1});
        validateManyArraySend(steps.get(offset++), caseName, "E4", new int[]{});
        validateWithIntArraySend(steps.get(offset++), caseName, "U1", new int[]{3, 4}, 10);
        validateWithIntArraySend(steps.get(offset++), caseName, "U2", new int[]{1}, 11);
        validateWithIntArraySend(steps.get(offset++), caseName, "U3", new int[]{}, 12);
        validateWithIntArraySend(steps.get(offset++), caseName, "U4", new int[]{1, 2}, 13);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact two-fields case step sequence mirroring
     * InfraUpdateMultikeyWArrayTwoFields lines 49-71: three
     * SupportEventWithManyArray inserts followed by five
     * SupportEventWithIntArray update triggers, then a mode-any iterator
     * snapshot.
     */
    private static int validateTwoFieldsCase(JsonArray steps, int offset) {
        String caseName = CASE_MULTIKEY_TWO_FIELDS;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_CREATE_MULTIKEY);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_MULTIKEY);
        validateDeploy(steps.get(offset++), caseName, "update", EPL_UPDATE_MULTIKEY_TWO_FIELDS);
        validateManyArraySend(steps.get(offset++), caseName, "ID1", new int[]{1, 2});
        validateManyArraySend(steps.get(offset++), caseName, "ID2", new int[]{3, 4});
        validateManyArraySend(steps.get(offset++), caseName, "ID3", new int[]{1});
        validateWithIntArraySend(steps.get(offset++), caseName, "ID2", new int[]{3, 4}, 10);
        validateWithIntArraySend(steps.get(offset++), caseName, "ID3", new int[]{1}, 11);
        validateWithIntArraySend(steps.get(offset++), caseName, "ID1", new int[]{1, 2}, 12);
        validateWithIntArraySend(steps.get(offset++), caseName, "IDX", new int[]{1}, 14);
        validateWithIntArraySend(steps.get(offset++), caseName, "ID1", new int[]{1, 2, 3}, 15);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
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
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateASend(JsonValue value, String caseName, String expectedId) {
        JsonObject step = object(value, "SupportBean_A step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_A".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_A step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_A payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException("SupportBean_A payload is not pinned for " + caseName);
        }
    }

    private static void validateManyArraySend(JsonValue value, String caseName, String expectedId,
                                              int[] expectedIntOne) {
        JsonObject step = object(value, "SupportEventWithManyArray step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportEventWithManyArray".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportEventWithManyArray step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportEventWithManyArray payload");
        requireFields(payload, "id", "intOne");
        if (!expectedId.equals(string(payload, "id"))
                || !Arrays.equals(expectedIntOne, intArray(payload.get("intOne"), "intOne"))) {
            throw new IllegalArgumentException(
                    "SupportEventWithManyArray payload is not pinned for " + caseName);
        }
    }

    private static void validateWithIntArraySend(JsonValue value, String caseName,
                                                 String expectedId, int[] expectedArray,
                                                 long expectedValue) {
        JsonObject step = object(value, "SupportEventWithIntArray step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportEventWithIntArray".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportEventWithIntArray payload");
        requireFields(payload, "id", "array", "value");
        if (!expectedId.equals(string(payload, "id"))
                || !Arrays.equals(expectedArray, intArray(payload.get("array"), "array"))
                || longInteger(payload.get("value"), "value") != expectedValue) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray payload is not pinned for " + caseName);
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

    private static int[] intArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        int[] result = new int[items.size()];
        for (int index = 0; index < result.length; index++) {
            result[index] = (int) longInteger(items.get(index), label + "[" + index + "]");
        }
        return result;
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
     * Local mirror of the regression SupportEventWithManyArray (regression-lib
     * is not on the oracle classpath); keeps the id, intOne, and value
     * properties the scenario updates and projects, with value defaulting to
     * 0 on construction exactly like the regression bean.  Serializable like
     * the regression bean: the engine's update copy-on-write for bean event
     * types requires a Serializable underlying or a configured copy method.
     */
    public static final class SupportEventWithManyArray implements Serializable {
        private static final long serialVersionUID = 4377584306530614243L;
        private String id;
        private int[] intOne;
        private int value;

        public SupportEventWithManyArray() {
        }

        public SupportEventWithManyArray(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }

        public void setId(String id) {
            this.id = id;
        }

        public int[] getIntOne() {
            return intOne;
        }

        public void setIntOne(int[] intOne) {
            this.intOne = intOne;
        }

        public int getValue() {
            return value;
        }

        public void setValue(int value) {
            this.value = value;
        }
    }

    /**
     * Local mirror of the regression SupportEventWithIntArray (regression-lib
     * is not on the oracle classpath).  Serializable like the regression bean
     * for the same update copy-on-write reason.
     */
    public static final class SupportEventWithIntArray implements Serializable {
        private static final long serialVersionUID = -6607427982340045404L;
        private String id;
        private int[] array;
        private int value;

        public SupportEventWithIntArray(String id, int[] array, int value) {
            this.id = id;
            this.array = array;
            this.value = value;
        }

        public String getId() {
            return id;
        }

        public int[] getArray() {
            return array;
        }

        public int getValue() {
            return value;
        }
    }
}
