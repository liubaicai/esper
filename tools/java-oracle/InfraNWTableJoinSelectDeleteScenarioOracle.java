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
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
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
 * Direct Esper 9.0.0 oracle for the infra-nwtable-join-select-delete parity
 * scenario. Mirrors two infra-nwtable executions that each run once over a
 * keepall named window and once over a primary-key table:
 * InfraNWTableJoin.InfraNWTableJoinSimple (ords 0/1, the join cases) deploys
 * the @public @buseventtype MyEvent schema and the MyInfra store in ONE
 * module, routes every MyEvent into the store through a separate insert
 * deployment and joins each arriving SupportBean against the store;
 * InfraNWTableOnSelectWDelete.InfraNWTableOnSelectWDeleteAssertion (ords
 * 0/1, the seldel cases) preloads a SupportBean-shaped store, fires the
 * on-SupportBean_S0 select-and-delete window aggregate that removes the
 * matched rows, asserts the store iterator between triggers and finally
 * re-deploys the identical select-delete EPL through
 * eplToModelCompileDeploy for the SODA check.
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own;
 * every execution ends with undeployAll). Every deploy step compiles and
 * deploys its own module: the join cases' first deploy carries the
 * schema+infra statements newline-joined exactly like the source's single
 * stmtTextCreate compileDeploy, while all other deploy steps carry a
 * single statement each. Deployed markers emit one record per deploy step.
 * Listener records carry the new-data rows (the join's c0/c1 pair, the
 * select-delete's single c0 sum). Snapshot steps read the create
 * statement's iterator like the source's assertPropsPerRowIterator
 * assertions; the emitted new array mirrors the engine iterator order
 * projected onto theString/intPrimitive — including the empty array the
 * final probe yields once the second select-delete emptied the store —
 * while the step's mode field carries the ordered/any-order comparison
 * each Java assertion used (the first window probe is ordered, every
 * other probe is any-order).
 */
public final class InfraNWTableJoinSelectDeleteScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-join-select-delete";
    private static final String DESCRIPTION =
            "InfraNWTableJoin and InfraNWTableOnSelectWDelete sibling slices: a"
                    + " continuous join of a keepall named window or primary-key table"
                    + " against a SupportBean#keepall stream on cid/theString equality,"
                    + " then an on-trigger select-and-delete whose ungrouped aggregate"
                    + " fold projects window(win.*) over the matched store rows and"
                    + " removes exactly that match set, with iterator probes on the"
                    + " create statement and a SODA re-deploy of the same select-delete"
                    + " text (Java sources"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableJoin.java and"
                    + " InfraNWTableOnSelectWDelete.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableJoin.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/"
                    + "nwtable/InfraNWTableJoin.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/"
                    + "nwtable/InfraNWTableOnSelectWDelete.java",
    };

    private static final String[] CASES = {
            "join-nw", "join-table", "seldel-nw", "seldel-table"};
    private static final int[] ORDINALS = {0, 1, 0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9aad0c9a0b81e251f6d4",
            "java-runtime-333f1a440da03d4b266a",
            "java-runtime-27d8980edc91c4593d34",
            "java-runtime-60c74e5e717dc6d9330e",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNWTableJoinSimple{namedWindow=true}",
            "InfraNWTableJoinSimple{namedWindow=false}",
            "InfraNWTableOnSelectWDeleteAssertion{namedWindow=true}",
            "InfraNWTableOnSelectWDeleteAssertion{namedWindow=false}",
    };
    private static final String[] STATIC_IDS = {
            "java-675ca69dbc976b4c4448",
            "java-675ca69dbc976b4c4448",
            "java-5a29ff903cb7fd7a100d",
            "java-5a29ff903cb7fd7a100d",
    };

    // Transcriptions of InfraNWTableJoin.java lines 43-55 (schema+infra
    // module, insert and join texts) and InfraNWTableOnSelectWDelete.java
    // lines 44-52 (create, insert and select-delete texts). The join module
    // text is the source's stmtTextCreate verbatim: schema line, "\n", then
    // the infra line, joined here with ";\n" as the module separator.
    private static final String EPL_JOIN_SCHEMA =
            "@public @buseventtype create schema MyEvent(cid string)";
    private static final String EPL_JOIN_CREATE_NW =
            "@public create window MyInfra.win:keepall() as MyEvent";
    private static final String EPL_JOIN_CREATE_TBL =
            "@public create table MyInfra(cid string primary key)";
    private static final String EPL_JOIN_INSERT =
            "insert into MyInfra select * from MyEvent";
    private static final String EPL_JOIN_SELECT =
            "@name('s0') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce,"
                    + " SupportBean#keepall() as sb where sb.theString = ce.cid";

    private static final String EPL_SELDEL_CREATE_NW =
            "@name('create') @public create window MyInfra#keepall as SupportBean";
    private static final String EPL_SELDEL_CREATE_TBL =
            "@name('create') @public create table MyInfra (theString string primary key,"
                    + " intPrimitive int primary key)";
    private static final String EPL_SELDEL_INSERT =
            "insert into MyInfra select theString, intPrimitive from SupportBean";
    private static final String EPL_SELDEL_SELECT =
            "@name('s0') on SupportBean_S0 as s0 select and delete"
                    + " window(win.*).aggregate(0,(result,value) => result+value.intPrimitive)"
                    + " as c0 from MyInfra as win where s0.p00=win.theString";

    private static final String[] CASE_EPLS = {
            EPL_JOIN_SCHEMA + ";\n" + EPL_JOIN_CREATE_NW + ";\n" + EPL_JOIN_INSERT
                    + ";\n" + EPL_JOIN_SELECT + ";\n",
            EPL_JOIN_SCHEMA + ";\n" + EPL_JOIN_CREATE_TBL + ";\n" + EPL_JOIN_INSERT
                    + ";\n" + EPL_JOIN_SELECT + ";\n",
            EPL_SELDEL_CREATE_NW + ";\n" + EPL_SELDEL_INSERT + ";\n" + EPL_SELDEL_SELECT
                    + ";\n",
            EPL_SELDEL_CREATE_TBL + ";\n" + EPL_SELDEL_INSERT + ";\n" + EPL_SELDEL_SELECT
                    + ";\n",
    };

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+listener; keepall window MyInfra fed by the MyEvent insert and"
                    + " declared in one module with the @buseventtype MyEvent schema;"
                    + " the join select ce.cid,sb.intPrimitive emits {c0='C2',c1=1} on"
                    + " SupportBean(C2,1) and {c0='C1',c1=4} on SupportBean(C1,4) once"
                    + " C1/C2/C3 sit in the store",
            "deployed+listener; cid-PK table MyInfra fed by the MyEvent insert and"
                    + " declared in one module with the @buseventtype MyEvent schema;"
                    + " the join select ce.cid,sb.intPrimitive emits {c0='C2',c1=1} on"
                    + " SupportBean(C2,1) and {c0='C1',c1=4} on SupportBean(C1,4) once"
                    + " C1/C2/C3 sit in the store",
            "deployed+listener+snapshot; keepall window MyInfra over SupportBean fed by"
                    + " the theString/intPrimitive insert: the create iterator probes"
                    + " {E1,E2} then {E2} then {E2,E2,E2} then empty while s0 emits"
                    + " c0=1 then c0=9",
            "deployed+listener+snapshot; composite-PK table MyInfra(theString,"
                    + " intPrimitive) fed by the same insert: the create iterator probes"
                    + " {E1,E2} then {E2} then {E2,E2,E2} then empty while s0 emits"
                    + " c0=1 then c0=9",
    };

    /** Statements carrying the case's update listener. */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        listened.put("join-nw", Set.of("s0"));
        listened.put("join-table", Set.of("s0"));
        listened.put("seldel-nw", Set.of("s0"));
        listened.put("seldel-table", Set.of("s0"));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    private static final int[] EXPECTED_CASE_RECORDS = {5, 5, 10, 10};
    private static final int EXPECTED_RECORDS = 30;
    private static final int EXPECTED_STEPS = 66;

    private InfraNWTableJoinSelectDeleteScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableJoinSelectDeleteScenarioOracle <scenario.json>");
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
        for (int index = 0; index < CASES.length; index++) {
            int before = records.size();
            runCase(index, allSteps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASES[index] + " emitted "
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

    /** Replays one case's steps on a fresh runtime (one runtime per Java execution). */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        try {
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
                        // Every deploy step is one compileDeploy: the step epl
                        // carries the module text verbatim (the join cases'
                        // schema+infra deploy is the source's two-statement
                        // stmtTextCreate). CompilerArguments(runtimePath) makes
                        // prior deployments' @public artifacts visible, mirroring
                        // the suite's RegressionPath. The step's label binds to
                        // the module's last statement — the infra select or the
                        // store create — while earlier statements (the schema)
                        // stay unreferenced by later ops.
                        String label = string(step, "statement");
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length == 0) {
                            throw new IllegalStateException("deploy of case " + caseName
                                    + " label " + label + " produced no statements");
                        }
                        EPStatement statement = deployed[deployed.length - 1];
                        // Mirroring addListener: Java attaches the listener on
                        // the initial s0 deployment only; the SODA
                        // eplToModelCompileDeploy leaves the redeployed
                        // statement unlistened, so an already-bound label
                        // skips attachment here.
                        if (LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet())
                                .contains(label)
                                && !statementsByName.containsKey(label)) {
                            statement.addListener(listener(caseName, sequences, records,
                                    runtime));
                        }
                        statementsByName.put(label, statement);
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statementsByName.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "snapshot": {
                        String label = string(step, "statement");
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "snapshot targets unknown statement " + label
                                            + " in case " + caseName);
                        }
                        records.add(snapshot(runtime, statement, caseName,
                                string(step, "mode"), fieldsOf(step)));
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean":
                runtime.getEventService().sendEventBean(
                        new SupportBean(string(payload, "theString"),
                                integer(payload, "intPrimitive")), type);
                break;
            case "SupportBean_S0":
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(integer(payload, "id"),
                                string(payload, "p00")), type);
                break;
            case "MyEvent":
                // env.sendEventMap(Collections.singletonMap("cid", c1), "MyEvent"):
                // the map event type comes from the @buseventtype schema deploy.
                runtime.getEventService().sendEventMap(
                        Collections.singletonMap("cid", string(payload, "cid")), type);
                break;
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
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
     * Snapshot of the store statement's iterator at the current virtual clock:
     * one record with sequence 0 holding the engine iterator order projected
     * onto the step's field list. The new array is emitted even when empty —
     * the final probe of each seldel case reads an emptied store, and the
     * pinned empty array is the assertion the Java source made. For the
     * step's mode field (ordered vs any) the oracle mirrors the differential
     * comparator's canonicalization: any-order probes emit rows sorted by
     * their canonical field encoding so the checked-in trace matches the
     * canonicalized evidence record-for-record, while the ordered probe
     * (seldel-nw's first iterator assertion) keeps the engine iterator order.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement,
                                       String caseName, String mode, String[] fields) {
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            rows.add(projectedRow(iterator.next(), fields));
        }
        if ("any".equals(mode)) {
            List<JsonValue> sorted = new ArrayList<>(rows.values());
            sorted.sort(Comparator.comparing(InfraNWTableJoinSelectDeleteScenarioOracle
                    ::canonicalRowKey));
            rows = new JsonArray();
            for (JsonValue row : sorted) {
                rows.add(row);
            }
        } else if (!"ordered".equals(mode)) {
            throw new IllegalArgumentException(
                    "unknown snapshot mode '" + mode + "' in case " + caseName);
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
     * Canonical row key mirroring canonicalizeAnyModeRows in the differential
     * comparator: the fields object's JSON text with members in name order —
     * the same representation Go's encoding/json emits for a map.
     */
    private static String canonicalRowKey(JsonValue row) {
        JsonObject fields = row.asObject().get("fields").asObject();
        List<String> names = new ArrayList<>();
        for (Member member : fields) {
            names.add(member.getName());
        }
        Collections.sort(names);
        JsonObject canonical = new JsonObject();
        for (String name : names) {
            canonical.add(name, fields.get(name));
        }
        return canonical.toString();
    }

    /** The snapshot step's projection list: the source's fieldsWin pair. */
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

    /** Projected row rendering in the step's declared field order. */
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

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

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
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, EventBean fragments as nested row objects, and
     * Java arrays as JSON arrays — the same shapes the Go normalizer emits.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
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

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds",
                "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES,
                "javaSourceFiles");
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

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
        offset = validateJoinCase(steps, offset, CASES[0],
                EPL_JOIN_SCHEMA + ";\n" + EPL_JOIN_CREATE_NW);
        offset = validateJoinCase(steps, offset, CASES[1],
                EPL_JOIN_SCHEMA + ";\n" + EPL_JOIN_CREATE_TBL);
        offset = validateSeldelCase(steps, offset, CASES[2], EPL_SELDEL_CREATE_NW,
                "ordered");
        offset = validateSeldelCase(steps, offset, CASES[3], EPL_SELDEL_CREATE_TBL,
                "any");
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact join step sequence mirroring InfraNWTableJoinSimple lines 42-70:
     * the schema+infra module deployment, the insert deployment, the s0 join
     * deployment, the C1/C2/C3 MyEvent sends and the two joining SupportBean
     * sends before undeploy-all.
     */
    private static int validateJoinCase(JsonArray steps, int offset, String caseName,
                                        String moduleEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", moduleEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_JOIN_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_JOIN_SELECT);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateMapSend(steps.get(offset++), caseName, "MyEvent", "cid", "C1");
        validateMapSend(steps.get(offset++), caseName, "MyEvent", "cid", "C2");
        validateMapSend(steps.get(offset++), caseName, "MyEvent", "cid", "C3");
        validateBeanSend(steps.get(offset++), caseName, "C2", 1);
        validateBeanSend(steps.get(offset++), caseName, "C1", 4);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact select-delete step sequence mirroring
     * InfraNWTableOnSelectWDeleteAssertion lines 43-83: the create, insert
     * and s0 deployments, the E1/E2 preload, the iterator probe, the E1
     * select-delete and second probe, the E2/3 E2/4 refill and third probe,
     * the E2 select-delete and empty probe, the SODA redeploy of the same
     * select-delete text and undeploy-all. The first probe mode is ordered
     * on the window case and any-order on the table case; the later probes
     * are any-order in both, exactly like the source assertions.
     */
    private static int validateSeldelCase(JsonArray steps, int offset, String caseName,
                                          String createEpl, String firstMode) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_SELDEL_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELDEL_SELECT);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create", firstMode);
        validateS0Send(steps.get(offset++), caseName, 100, "E1");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateBeanSend(steps.get(offset++), caseName, "E2", 3);
        validateBeanSend(steps.get(offset++), caseName, "E2", 4);
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateS0Send(steps.get(offset++), caseName, 101, "E2");
        validateSnapshot(steps.get(offset++), caseName, "create", "any");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELDEL_SELECT);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName,
                                       String expectedStatement, String expectedEpl) {
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

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
        String[] fields = fieldsOf(step);
        if (fields.length != 2 || !"theString".equals(fields[0])
                || !"intPrimitive".equals(fields[1])) {
            throw new IllegalArgumentException("snapshot fields are not pinned for " + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String theString, int intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(payload.get("theString").asString())
                || payload.get("intPrimitive").asInt() != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName,
                                       int id, String p00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (payload.get("id").asInt() != id
                || !p00.equals(payload.get("p00").asString())) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateMapSend(JsonValue value, String caseName,
                                        String eventType, String field, String expected) {
        JsonObject step = object(value, "map send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("map send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "map payload");
        requireFields(payload, field);
        if (!expected.equals(payload.get(field).asString())) {
            throw new IllegalArgumentException("map payload is not pinned for " + caseName);
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
