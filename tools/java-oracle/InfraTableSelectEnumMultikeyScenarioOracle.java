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
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithIntArray;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithManyArray;
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
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableSelect ordinals 1-4: the "select from table"
 * surface covering the enum firstOf() read and the array/composite multikey
 * table joins. Four executions on four runtimes.
 *
 * InfraTableSelectEnum (ord 1) deploys one module — an unkeyed
 * MyTable(p string) plus the 's0' select of t.firstOf() as c0 — attaches NO
 * listener, seeds {'a'} through a fire-and-forget insert
 * (compileExecuteFAFNoResult semantics: compileQuery + executeQuery, no
 * deployment), and asserts the 's0' iterator: next().get("c0") is the first
 * table row's UNDERLYING Object[]{'a'}, not an EventBean.
 *
 * InfraTableSelectMultikeyWArraySingleArray (ord 2) deploys one module — a
 * MyTable keyed by the primitive int array column k, an insert-into fed by
 * SupportEventWithIntArray.array, and the 's0' join of
 * SupportEventWithManyArray.intOne to k — attaches the listener to 's0',
 * sends E1/E2/E3 inserts, crosses the milestone(0) serde checkpoint (a
 * harness no-op carrying no step), and probes [2], [1,3] and [1,2] for
 * c0 = 30/20/10. Array primary-key equality is by content (Arrays.equals).
 *
 * InfraTableSelectMultikeyWArrayTwoArray (ord 3) deploys one module — a
 * MyTable keyed by the two primitive int array columns k1/k2, an insert-into
 * fed by SupportEventWithManyArray(id='I'), and the 's0' join of
 * SupportEventWithManyArray(id='Q') on k1=intOne and k2=intTwo — attaches
 * the listener to 's0', sends three 'I' inserts (including the empty-array
 * key component []), crosses milestone(0), and probes ([2],[]),
 * ([1,2],[3,4]) and ([1,3],[1]) for c0 = 30/10/20.
 *
 * InfraTableSelectMultikeyWArrayComposite (ord 4) deploys one module — a
 * MyTable keyed by three string columns k0/k1/k2 with a planning-only btree
 * secondary index on (k0,k1,v), an insert-into fed by SupportBean_S0, and
 * the 's0' join of SupportBean_S1 on k0=p10 and k1=p11 with the
 * lexicographic range predicate v>p12 — attaches the listener to 's0',
 * sends four S0 rows, crosses milestone(0), and probes ('A','CC',''),
 * ('C','CC',''), ('A','BB','X3') and ('A','BB','Z'); the last leaves the
 * listener uninvoked (assertListenerNotInvoked) because no row satisfies
 * v>'Z'.
 */
public final class InfraTableSelectEnumMultikeyScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-select-enum-multikey";
    private static final String DESCRIPTION =
            "InfraTableSelect ordinals 1-4: InfraTableSelectEnum deploys an "
                    + "unkeyed MyTable(p string) plus a 's0' select of "
                    + "t.firstOf() as c0, seeds {'a'} via fire-and-forget "
                    + "insert and observes the iterator row whose c0 is the "
                    + "first table row's Object[] underlying; "
                    + "InfraTableSelectMultikeyWArraySingleArray joins "
                    + "SupportEventWithManyArray.intOne to the "
                    + "int[primitive] primary key of a table filled from "
                    + "SupportEventWithIntArray; "
                    + "InfraTableSelectMultikeyWArrayTwoArray joins "
                    + "intOne/intTwo to a two-array-key table filled and "
                    + "probed through the same event type filtered on id "
                    + "'I'/'Q'; InfraTableSelectMultikeyWArrayComposite "
                    + "joins SupportBean_S1 to a three-string-key table with "
                    + "a btree secondary index and a lexicographic v > p12 "
                    + "range predicate. Java milestone(0) checkpoints are "
                    + "harness no-ops and carry no steps (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/tbl/InfraTableSelect.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableSelect.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-27e7ce929b90e4009b48",
            "java-runtime-d52d06b4618e30451543",
            "java-runtime-83451626aeee59b34ac2",
            "java-runtime-b8b3e8c1f04c0c918ea2"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraTableSelectEnum",
            "InfraTableSelectMultikeyWArraySingleArray",
            "InfraTableSelectMultikeyWArrayTwoArray",
            "InfraTableSelectMultikeyWArrayComposite"
    };
    private static final String[] STATIC_IDS = {
            "java-ece967984c50b6e02d47",
            "java-2ae2dfa34214fa74ffc8",
            "java-79057cf89b8898584f05",
            "java-2a4767401dba7efd8f97"
    };
    private static final String[] CASES = {
            "enum-firstof",
            "multikey-warray-single",
            "multikey-warray-two",
            "multikey-warray-composite"
    };
    private static final int[] ORDINALS = {1, 2, 3, 4};
    private static final String[] CASE_OBSERVATIONS = {
            "iterator; unkeyed MyTable(p string) seeded {'a'} by "
                    + "fire-and-forget insert; the s0 iterator row carries "
                    + "c0 = Object[]{'a'} — firstOf() yields the first table "
                    + "row's underlying, not an EventBean; no listener is "
                    + "attached",
            "listener; int[primitive] primary key k filled from "
                    + "SupportEventWithIntArray.array by content equality; "
                    + "SupportEventWithManyArray.intOne probes "
                    + "[2]/[1,3]/[1,2] yield c0 30/20/10",
            "listener; two int[primitive] primary keys k1/k2 filled from "
                    + "SupportEventWithManyArray(id='I') and probed by id='Q' "
                    + "sends; the empty array [] is a valid key component; "
                    + "probes ([2],[])/([1,2],[3,4])/([1,3],[1]) yield c0 "
                    + "30/10/20",
            "listener; three string primary keys k0/k1/k2 plus a "
                    + "planning-only btree index on (k0,k1,v); "
                    + "SupportBean_S1 probes join on k0=p10 and k1=p11 with "
                    + "lexicographic v>p12 — ('A','CC','') yields X3, "
                    + "('C','CC','') yields X4, ('A','BB','X3') yields X4 "
                    + "because 'X1' is not > 'X3', and ('A','BB','Z') leaves "
                    + "the listener uninvoked"
    };

    // Verbatim transcriptions of InfraTableSelect lines 150-151 (ord 1),
    // 119-121 (ord 2), 88-90 (ord 3) and 50-53 (ord 4); each case deploys
    // its module in a single compileDeploy call.
    private static final String EPL_ENUM_MODULE =
            "@public create table MyTable(p string);\n"
                    + "@name('s0') select t.firstOf() as c0 from MyTable as t;\n";
    private static final String EPL_ENUM_FAF_INSERT =
            "insert into MyTable select 'a' as p";
    private static final String EPL_SINGLE_MODULE =
            "@public create table MyTable(k int[primitive] primary key, value int);\n"
                    + "insert into MyTable select array as k, value from SupportEventWithIntArray;\n"
                    + "@name('s0') select t.value as c0 from SupportEventWithManyArray, MyTable as t where k = intOne;\n";
    private static final String EPL_TWO_MODULE =
            "@public create table MyTable(k1 int[primitive] primary key, k2 int[primitive] primary key, value int);\n"
                    + "insert into MyTable select intOne as k1, intTwo as k2, value from SupportEventWithManyArray(id = 'I');\n"
                    + "@name('s0') select t.value as c0 from SupportEventWithManyArray(id='Q'), MyTable as t where k1 = intOne and k2 = intTwo;\n";
    private static final String EPL_COMPOSITE_MODULE =
            "@public create table MyTable(k0 string primary key, k1 string primary key, k2 string primary key, v string);\n"
                    + "create index MyIndex on MyTable(k0, k1, v btree);\n"
                    + "insert into MyTable select p00 as k0, p01 as k1, p02 as k2, p03 as v from SupportBean_S0;\n"
                    + "@name('s0') select t.v as v from SupportBean_S1, MyTable as t where k0 = p10 and k1 = p11 and v > p12;\n";

    private static final String FAF_INSERT_LABEL = "FafInsert";
    private static final String[] SNAPSHOT_FIELDS = {"c0"};

    private static final int EXPECTED_STEPS = 42;
    private static final int EXPECTED_RECORDS = 18;

    private InfraTableSelectEnumMultikeyScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableSelectEnumMultikeyScenarioOracle <scenario.json>");
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
        for (String caseName : CASES) {
            runCase(caseName, allSteps, records);
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
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). The four event types are preconfigured, the internal
     * timer is disabled and the rethrowing exception handler surfaces
     * statement failures to the sender thread. Every deploy registers its
     * statements by name and its deployment by step label so deployed
     * markers and snapshots resolve both; the FafInsert label is a
     * fire-and-forget insert (compileExecuteFAFNoResult semantics) and
     * registers nothing. The recording listener attaches to 's0' for the
     * three multikey cases (compileDeploy(...).addListener("s0")); the enum
     * case mirrors compileDeploy(epl, path) with no listener.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportEventWithIntArray.class);
        configuration.getCommon().addEventType(SupportEventWithManyArray.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Set<String> deployments = new HashSet<>();
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
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        if (FAF_INSERT_LABEL.equals(label)) {
                            // Fire-and-forget insert, mirroring
                            // RegressionEnvironmentBase.compileExecuteFAFNoResult:
                            // compiled as a query with the runtime path and
                            // executed on demand; no deployment, no marker.
                            CompilerArguments fafArgs = new CompilerArguments(configuration);
                            fafArgs.getPath().add(runtime.getRuntimePath());
                            EPCompiled query = EPCompilerProvider.getCompiler()
                                    .compileQuery(epl, fafArgs);
                            runtime.getFireAndForgetService().executeQuery(query);
                            break;
                        }
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        deployments.add(label);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
                            // The multikey executions attach the listener to
                            // 's0'; the enum execution attaches none.
                            if ("s0".equals(statement.getName())
                                    && !"enum-firstof".equals(caseName)) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.contains(label) && !statements.containsKey(label)) {
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
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException("snapshot statement " + label
                                    + " was not deployed in case " + caseName);
                        }
                        String[] fields = stringArray(step.get("fields"), "fields");
                        JsonArray rows = new JsonArray();
                        for (Iterator<EventBean> iterator = statement.iterator();
                             iterator.hasNext(); ) {
                            rows.add(projectedRow(iterator.next(), fields));
                        }
                        if ("any".equals(string(step, "mode"))) {
                            sortRowsCanonical(rows);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "snapshot");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        if (rows.size() > 0) {
                            record.add("new", rows);
                        }
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deployments.clear();
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
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty. This
     * mirrors the 's0' listener assertions of the three multikey executions
     * (assertEqualsNew on c0/v); the assertListenerNotInvoked probe of the
     * composite case is the absence of a record.
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
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
    }

    /** Row projected to exactly the fields the step's assertions read,
     * with property names sorted alphabetically so the canonical "any"-mode
     * ordering matches the Go runner's fields-JSON sort. */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = fields.clone();
        Arrays.sort(names);
        JsonObject values = new JsonObject();
        for (String field : names) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /**
     * Canonical row ordering for "any"-mode snapshots, mirroring the Go
     * runner's sortRowsCanonical freeze of Java's assertEqualsAnyOrder: rows
     * sort by their fields JSON (keys already sorted alphabetically).
     */
    private static void sortRowsCanonical(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue row : rows) {
            items.add(row);
        }
        items.sort(Comparator.comparing(row -> row.asObject().get("fields").toString()));
        while (rows.size() > 0) {
            rows.remove(0);
        }
        for (JsonValue item : items) {
            rows.add(item);
        }
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, and Object[]/int[] row underlyings (the enum
     * case's firstOf() value) as JSON arrays.
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
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Sends one pinned event: SupportEventWithIntArray carries id/array/value
     * (ord 2 inserts), SupportEventWithManyArray carries id/value/intOne/
     * intTwo (ord 2 probes and ord 3 'I'/'Q' sends), SupportBean_S0 carries
     * id/p00..p03 (ord 4 inserts) and SupportBean_S1 carries id/p10..p12
     * (ord 4 probes).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportEventWithIntArray": {
                SupportEventWithIntArray event = new SupportEventWithIntArray(
                        stringOrNull(payload.get("id")), intArray(payload.get("array")),
                        intField(payload.get("value"), "value"));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportEventWithManyArray": {
                SupportEventWithManyArray event =
                        new SupportEventWithManyArray(stringOrNull(payload.get("id")));
                JsonValue value = payload.get("value");
                if (value instanceof JsonNumber) {
                    event.withValue(value.asInt());
                }
                JsonValue intOne = payload.get("intOne");
                if (intOne != null) {
                    event.withIntOne(intArray(intOne));
                }
                JsonValue intTwo = payload.get("intTwo");
                if (intTwo != null) {
                    event.withIntTwo(intArray(intTwo));
                }
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(
                        intField(payload.get("id"), "id"),
                        stringOrNull(payload.get("p00")),
                        stringOrNull(payload.get("p01")),
                        stringOrNull(payload.get("p02")),
                        stringOrNull(payload.get("p03")));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(
                        intField(payload.get("id"), "id"),
                        stringOrNull(payload.get("p10")),
                        stringOrNull(payload.get("p11")),
                        stringOrNull(payload.get("p12")));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static String stringOrNull(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    private static int intField(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        return (int) longInteger(value, label);
    }

    private static int[] intArray(JsonValue value) {
        JsonArray items = array(value, "int array payload member");
        int[] result = new int[items.size()];
        for (int index = 0; index < items.size(); index++) {
            result[index] = (int) longInteger(items.get(index), "int array element");
        }
        return result;
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
        validateStringArray(scenario.get("javaFlags"), new String[]{}, "javaFlags");

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
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
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
            switch (caseName) {
                case "enum-firstof":
                    offset = validateEnumCase(steps, offset, caseName);
                    break;
                case "multikey-warray-single":
                    offset = validateSingleArrayCase(steps, offset, caseName);
                    break;
                case "multikey-warray-two":
                    offset = validateTwoArrayCase(steps, offset, caseName);
                    break;
                default:
                    offset = validateCompositeCase(steps, offset, caseName);
                    break;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the module EPL of each case. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "enum-firstof":
                return EPL_ENUM_MODULE;
            case "multikey-warray-single":
                return EPL_SINGLE_MODULE;
            case "multikey-warray-two":
                return EPL_TWO_MODULE;
            default:
                return EPL_COMPOSITE_MODULE;
        }
    }

    /**
     * Exact step sequence of InfraTableSelectEnum.run (lines 148-162): the
     * module deploy with deployed markers for the deployment label and the
     * named 's0' statement, the fire-and-forget seed insert (no marker), the
     * ordered c0 snapshot mirroring assertIterator, and undeployAll. The
     * execution attaches no listener.
     */
    private static int validateEnumCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateModuleDeploy(steps, offset, caseName, EPL_ENUM_MODULE);
        validateDeploy(steps.get(offset++), caseName, FAF_INSERT_LABEL, EPL_ENUM_FAF_INSERT);
        validateSnapshot(steps.get(offset++), caseName, "s0", "ordered", SNAPSHOT_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSelectMultikeyWArraySingleArray.run
     * (lines 117-135): the module deploy with deployed markers, the three
     * SupportEventWithIntArray inserts, the milestone(0) no-op (no step),
     * the three SupportEventWithManyArray probes and undeployAll.
     */
    private static int validateSingleArrayCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateModuleDeploy(steps, offset, caseName, EPL_SINGLE_MODULE);
        validateIntArraySend(steps.get(offset++), caseName, "E1", new int[]{1, 2}, 10);
        validateIntArraySend(steps.get(offset++), caseName, "E2", new int[]{1, 3}, 20);
        validateIntArraySend(steps.get(offset++), caseName, "E3", new int[]{2}, 30);
        validateManyArraySend(steps.get(offset++), caseName, null, new int[]{2}, null, null);
        validateManyArraySend(steps.get(offset++), caseName, null, new int[]{1, 3}, null, null);
        validateManyArraySend(steps.get(offset++), caseName, null, new int[]{1, 2}, null, null);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSelectMultikeyWArrayTwoArray.run
     * (lines 86-104): the module deploy with deployed markers, the three
     * id='I' inserts, the milestone(0) no-op (no step), the three id='Q'
     * probes sent with value -1 by sendManyArrayAssert, and undeployAll.
     */
    private static int validateTwoArrayCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateModuleDeploy(steps, offset, caseName, EPL_TWO_MODULE);
        validateManyArraySend(steps.get(offset++), caseName, "I", new int[]{1, 2},
                new int[]{3, 4}, 10);
        validateManyArraySend(steps.get(offset++), caseName, "I", new int[]{1, 3},
                new int[]{1}, 20);
        validateManyArraySend(steps.get(offset++), caseName, "I", new int[]{2},
                new int[]{}, 30);
        validateManyArraySend(steps.get(offset++), caseName, "Q", new int[]{2},
                new int[]{}, -1);
        validateManyArraySend(steps.get(offset++), caseName, "Q", new int[]{1, 2},
                new int[]{3, 4}, -1);
        validateManyArraySend(steps.get(offset++), caseName, "Q", new int[]{1, 3},
                new int[]{1}, -1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSelectMultikeyWArrayComposite.run
     * (lines 48-69): the module deploy with deployed markers, the four
     * SupportBean_S0 inserts, the milestone(0) no-op (no step), the four
     * SupportBean_S1 probes (the last is assertListenerNotInvoked), and
     * undeployAll.
     */
    private static int validateCompositeCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateModuleDeploy(steps, offset, caseName, EPL_COMPOSITE_MODULE);
        validateS0Send(steps.get(offset++), caseName, "A", "BB", "CCC", "X1");
        validateS0Send(steps.get(offset++), caseName, "A", "BB", "DDDD", "X4");
        validateS0Send(steps.get(offset++), caseName, "A", "CC", "CCC", "X3");
        validateS0Send(steps.get(offset++), caseName, "C", "CC", "CCC", "X4");
        validateS1Send(steps.get(offset++), caseName, "A", "CC", "");
        validateS1Send(steps.get(offset++), caseName, "C", "CC", "");
        validateS1Send(steps.get(offset++), caseName, "A", "BB", "X3");
        validateS1Send(steps.get(offset++), caseName, "A", "BB", "Z");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /** The module deploy plus its two deployed markers: the deployment
     * label and the named 's0' statement (the inner-join-on precedent). */
    private static int validateModuleDeploy(JsonArray steps, int offset, String caseName,
                                            String moduleEpl) {
        validateDeploy(steps.get(offset++), caseName, "module", moduleEpl);
        validateDeployed(steps.get(offset++), caseName, "module");
        validateDeployed(steps.get(offset++), caseName, "s0");
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

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode, String[] expectedFields) {
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
                "snapshot fields for " + caseName);
    }

    private static void validateIntArraySend(JsonValue value, String caseName,
                                             String expectedId, int[] expectedArray,
                                             int expectedValue) {
        JsonObject payload = sendPayload(value, caseName, "SupportEventWithIntArray");
        requireFields(payload, "id", "array", "value");
        if (!expectedId.equals(string(payload, "id"))
                || !Arrays.equals(expectedArray, intArray(payload.get("array")))
                || intField(payload.get("value"), "value") != expectedValue) {
            throw new IllegalArgumentException(
                    "SupportEventWithIntArray payload is not pinned for " + caseName);
        }
    }

    /**
     * Pins a SupportEventWithManyArray send. The ord-2 probes carry only
     * intOne (id/intTwo/value absent, matching new SupportEventWithManyArray()
     * .withIntOne(ints)); the ord-3 sends carry all four fields.
     */
    private static void validateManyArraySend(JsonValue value, String caseName,
                                              String expectedId, int[] expectedIntOne,
                                              int[] expectedIntTwo, Integer expectedValue) {
        JsonObject payload = sendPayload(value, caseName, "SupportEventWithManyArray");
        if (expectedId == null) {
            requireFields(payload, "intOne");
        } else {
            requireFields(payload, "id", "intOne", "intTwo", "value");
            if (!expectedId.equals(string(payload, "id"))
                    || intField(payload.get("value"), "value") != expectedValue) {
                throw new IllegalArgumentException(
                        "SupportEventWithManyArray payload is not pinned for " + caseName);
            }
            if (!Arrays.equals(expectedIntTwo, intArray(payload.get("intTwo")))) {
                throw new IllegalArgumentException(
                        "SupportEventWithManyArray intTwo is not pinned for " + caseName);
            }
        }
        if (!Arrays.equals(expectedIntOne, intArray(payload.get("intOne")))) {
            throw new IllegalArgumentException(
                    "SupportEventWithManyArray intOne is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName,
                                       String p00, String p01, String p02, String p03) {
        JsonObject payload = sendPayload(value, caseName, "SupportBean_S0");
        requireFields(payload, "id", "p00", "p01", "p02", "p03");
        if (intField(payload.get("id"), "id") != 0
                || !p00.equals(string(payload, "p00"))
                || !p01.equals(string(payload, "p01"))
                || !p02.equals(string(payload, "p02"))
                || !p03.equals(string(payload, "p03"))) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    private static void validateS1Send(JsonValue value, String caseName,
                                       String p10, String p11, String p12) {
        JsonObject payload = sendPayload(value, caseName, "SupportBean_S1");
        requireFields(payload, "id", "p10", "p11", "p12");
        if (intField(payload.get("id"), "id") != 0
                || !p10.equals(string(payload, "p10"))
                || !p11.equals(string(payload, "p11"))
                || !p12.equals(string(payload, "p12"))) {
            throw new IllegalArgumentException(
                    "SupportBean_S1 payload is not pinned for " + caseName);
        }
    }

    private static JsonObject sendPayload(JsonValue value, String caseName,
                                          String expectedEventType) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedEventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName + "/"
                    + expectedEventType);
        }
        return object(step.get("payload"), expectedEventType + " payload");
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

    private static String[] stringArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        String[] names = new String[items.size()];
        for (int index = 0; index < items.size(); index++) {
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException(label + " must be a string array");
            }
            names[index] = item.asString();
        }
        return names;
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
