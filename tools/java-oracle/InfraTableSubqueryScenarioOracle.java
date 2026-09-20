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
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableSubquery ordinals 0-3: correlated scalar
 * subqueries and subquery-in-filter against tables. Four executions on four
 * runtimes.
 *
 * InfraTableSubqueryAgainstKeyed (ord 0) deploys keyed varagg(key string
 * primary key, total sum(int)) plus a grouped into-table sum feed and a
 * correlated scalar subquery `(select total from varagg where key = s0.p00)`
 * over SupportBean_S0; the listener pins value=null for the missing G1 row
 * and 200 for G2, then after milestone(0) and SupportBean("G1",100) pins
 * 100/200.
 *
 * InfraTableSubqueryAgainstUnkeyed (ord 1) deploys the unkeyed InfraOne
 * (string string, intPrimitive int), the correlated subquery statement
 * BEFORE the insert-into feed, then sends SupportBean("E1",10), milestone(0)
 * and S0(0,"E1") pinning c0=10 through a full-scan subquery.
 *
 * InfraTableSubquerySecondaryIndex (ord 2) deploys composite-key MyTable
 * (k0,k1 primary keys, p2, value), a secondary index on p2 before any rows,
 * an on-SupportBean_S0 merge that inserts/updates p2 and value, and a
 * correlated subquery `where sb.theString = tbl.p2`; the merge update of
 * the indexed column moves the row from P2_1 to P2_2 across milestones.
 *
 * InfraTableSubqueryInFilter (ord 3, defined first in the file but ordinal
 * 3 in executions()) deploys one three-statement module: module-private
 * MyTable(tablecol primary key), an insert-into feed from SupportBean_S0.p00,
 * and `select * from SupportBean(theString=(select tablecol from
 * MyTable).orderBy().firstOf())` — an uncorrelated enum
 * orderBy().firstOf() subquery inside the stream filter. Filtered-out sends
 * produce no listener record.
 *
 * Java milestone checkpoints are harness no-ops and carry no steps.
 */
public final class InfraTableSubqueryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-subquery";
    private static final String DESCRIPTION =
            "InfraTableSubquery ordinals 0-3: InfraTableSubqueryAgainstKeyed "
                    + "deploys keyed varagg fed by a grouped into-table sum and "
                    + "pins the correlated scalar subquery value null/200 then "
                    + "100/200 across milestone(0); "
                    + "InfraTableSubqueryAgainstUnkeyed deploys the unkeyed "
                    + "InfraOne with the subquery statement before the "
                    + "insert-into feed and pins c0=10; "
                    + "InfraTableSubquerySecondaryIndex deploys composite-key "
                    + "MyTable with a secondary index on p2 and an on-merge "
                    + "that moves the indexed column from P2_1 to P2_2, "
                    + "pinning c0=10 then null/11; InfraTableSubqueryInFilter "
                    + "deploys one module whose select * filters SupportBean "
                    + "on theString=(select tablecol from MyTable).orderBy()"
                    + ".firstOf(), emitting listener records only for "
                    + "matching sends (Java source regression-lib/src/main/"
                    + "java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableSubquery.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableSubquery.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-7b449dd45dd6961c5d61",
            "java-runtime-9cde668ef5b4781b068a",
            "java-runtime-8ece65643b6ec15616f2",
            "java-runtime-a839574d871f88c2fdbf"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraTableSubqueryAgainstKeyed",
            "InfraTableSubqueryAgainstUnkeyed",
            "InfraTableSubquerySecondaryIndex",
            "InfraTableSubqueryInFilter"
    };
    private static final String[] STATIC_IDS = {
            "java-84c3e4b24f1621c4e20f",
            "java-8a837e4f238a838a719c",
            "java-22c9a3e16812af54b583",
            "java-05c24f1601cc75b1683e"
    };
    private static final String[] CASES = {
            "subquery-keyed",
            "subquery-unkeyed",
            "subquery-secondary-index",
            "subquery-in-filter"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};

    // Verbatim transcriptions of InfraTableSubquery lines 78-83 (ord 0);
    // 102-104 (ord 1); 121-133 (ord 2, the merge EPL keeps its trailing
    // space after `id `); and 41-43 (ord 3, a single three-statement module
    // whose create table is module-private and therefore cannot deploy
    // separately). Each deploy step compiles its EPL as one module.
    private static final String EPL_KEYED_CREATE =
            "@public create table varagg as (key string primary key, total sum(int))";
    private static final String EPL_KEYED_INTO =
            "into table varagg select sum(intPrimitive) as total from SupportBean group by theString";
    private static final String EPL_KEYED_S0 =
            "@name('s0') select (select total from varagg where key = s0.p00) as value "
                    + "from SupportBean_S0 as s0";

    private static final String EPL_UNKEYED_CREATE =
            "@public create table InfraOne (string string, intPrimitive int)";
    private static final String EPL_UNKEYED_S0 =
            "@name('s0') select (select intPrimitive from InfraOne where string = s0.p00) as c0 "
                    + "from SupportBean_S0 as s0";
    private static final String EPL_UNKEYED_INSERT =
            "insert into InfraOne select theString as string, intPrimitive from SupportBean";

    private static final String EPL_SECIDX_CREATE =
            "@public create table MyTable(k0 string primary key, k1 string primary key, "
                    + "p2 string, value int)";
    private static final String EPL_SECIDX_INDEX =
            "create index MyIndex on MyTable(p2)";
    private static final String EPL_SECIDX_MERGE =
            "on SupportBean_S0 merge MyTable where p00 = k0 and p01 = k1 "
                    + "when not matched then insert select p00 as k0, p01 as k1, p02 as p2, "
                    + "id as value when matched then update set p2 = p02, value = id ";
    private static final String EPL_SECIDX_S0 =
            "@Name('s0') select (select value from MyTable as tbl where sb.theString = tbl.p2) "
                    + "as c0 from SupportBean as sb";

    private static final String EPL_FILTER_MODULE =
            "create table MyTable(tablecol string primary key);\n"
                    + "insert into MyTable select p00 as tablecol from SupportBean_S0;\n"
                    + "@name('s0') select * from SupportBean(theString=(select tablecol "
                    + "from MyTable).orderBy().firstOf())";

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(Arrays.asList("s0"));
    private static final int EXPECTED_STEPS = 54;
    private static final int EXPECTED_RECORDS = 23;

    private InfraTableSubqueryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableSubqueryScenarioOracle <scenario.json>");
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
     * its own runtime). SupportBean and SupportBean_S0 are preconfigured,
     * the internal timer is disabled and the rethrowing exception handler
     * surfaces statement failures to the sender thread. Every deploy
     * registers its deployment by step label and its statements by name so
     * deployed markers resolve both; the s0 statement carries the listener
     * that captures assertPropsNew/assertEventNew/assertListenerInvokedFlag
     * observations (a filtered-out send emits no record).
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
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
        Map<String, EPStatement> statements = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
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
                        EPDeployment deployment = compileDeploy(runtime, epl);
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
                            if (LISTENED_STATEMENTS.contains(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.containsKey(label) && !statements.containsKey(label)) {
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

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * against the runtime path followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, String epl)
            throws EPCompileException, EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    /**
     * Listener emitting one record per invocation with a per-case sequence
     * counter; the default istream selector means only a new array renders
     * and only when non-empty.
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

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, and Object[]/int[] row underlyings as JSON
     * arrays.
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
     * Sends one pinned event: SupportBean carries theString/intPrimitive
     * (the two-arg constructor leaves every other property at its Java
     * default) and SupportBean_S0 carries id plus p00/p01/p02 (absent keys
     * stay null, matching the two-arg constructor the Java executions use).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean(
                        string(payload, "theString"),
                        intField(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(
                        intField(payload.get("id"), "id"),
                        stringOrNull(payload.get("p00")),
                        stringOrNull(payload.get("p01")),
                        stringOrNull(payload.get("p02")));
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
                    "observation", "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
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
        offset = validateKeyedCase(steps, offset, CASES[0]);
        offset = validateUnkeyedCase(steps, offset, CASES[1]);
        offset = validateSecondaryIndexCase(steps, offset, CASES[2]);
        offset = validateInFilterCase(steps, offset, CASES[3]);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "subquery-keyed":
                return String.join("\n", EPL_KEYED_CREATE, EPL_KEYED_INTO, EPL_KEYED_S0);
            case "subquery-unkeyed":
                return String.join("\n", EPL_UNKEYED_CREATE, EPL_UNKEYED_S0, EPL_UNKEYED_INSERT);
            case "subquery-secondary-index":
                return String.join("\n", EPL_SECIDX_CREATE, EPL_SECIDX_INDEX, EPL_SECIDX_MERGE,
                        EPL_SECIDX_S0);
            default:
                return EPL_FILTER_MODULE;
        }
    }

    /**
     * Exact step sequence of InfraTableSubqueryAgainstKeyed.run (lines
     * 75-93): the create-table, into-table and s0 deploys, SupportBean
     * ("G2",200), the assertValues G1/G2 S0 sends, milestone(0) (no step),
     * SupportBean("G1",100), the second G1/G2 pair and undeployAll.
     */
    private static int validateKeyedCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_KEYED_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_KEYED_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_KEYED_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "G2", 200);
        validateS0Send(steps.get(offset++), caseName, 0, "G1");
        validateS0Send(steps.get(offset++), caseName, 0, "G2");
        validateBeanSend(steps.get(offset++), caseName, "G1", 100);
        validateS0Send(steps.get(offset++), caseName, 0, "G1");
        validateS0Send(steps.get(offset++), caseName, 0, "G2");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSubqueryAgainstUnkeyed.run (lines
     * 99-113): the unkeyed create-table, the s0 subquery deploy BEFORE the
     * insert-into feed, SupportBean("E1",10), milestone(0) (no step),
     * S0(0,"E1") and undeployAll.
     */
    private static int validateUnkeyedCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_UNKEYED_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_UNKEYED_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_UNKEYED_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 10);
        validateS0Send(steps.get(offset++), caseName, 0, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSubquerySecondaryIndex.run (lines
     * 118-148): the composite-key create-table, the secondary index on p2
     * before any rows, the on-merge deploy, the s0 subquery deploy, the
     * S0(10,G1,SG1,P2_1) insert and the P2_1 assert, milestone(0), the
     * S0(11,G1,SG1,P2_2) update, milestone(1), the P2_1/P2_2 asserts and
     * undeployAll.
     */
    private static int validateSecondaryIndexCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_SECIDX_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_SECIDX_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateDeploy(steps.get(offset++), caseName, "merge", EPL_SECIDX_MERGE);
        validateDeployed(steps.get(offset++), caseName, "merge");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SECIDX_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateS0MergeSend(steps.get(offset++), caseName, 10, "G1", "SG1", "P2_1");
        validateBeanSend(steps.get(offset++), caseName, "P2_1", -1);
        validateS0MergeSend(steps.get(offset++), caseName, 11, "G1", "SG1", "P2_2");
        validateBeanSend(steps.get(offset++), caseName, "P2_1", -1);
        validateBeanSend(steps.get(offset++), caseName, "P2_2", -1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableSubqueryInFilter.run (lines 40-61):
     * one module deploy carrying the module-private create table, the
     * insert-into feed and the filtered select *; the sendAssert/sendS0
     * interleaving (filtered-out SupportBean sends emit no record), the
     * milestone(0) no-op and undeployAll.
     */
    private static int validateInFilterCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "module", EPL_FILTER_MODULE);
        validateDeployed(steps.get(offset++), caseName, "module");
        validateBeanSend(steps.get(offset++), caseName, "E", 0);
        validateS0Send(steps.get(offset++), caseName, 0, "E");
        validateBeanSend(steps.get(offset++), caseName, "E", 0);
        validateS0Send(steps.get(offset++), caseName, 0, "C");
        validateBeanSend(steps.get(offset++), caseName, "E", 0);
        validateBeanSend(steps.get(offset++), caseName, "C", 0);
        validateBeanSend(steps.get(offset++), caseName, "A", 0);
        validateBeanSend(steps.get(offset++), caseName, "C", 0);
        validateS0Send(steps.get(offset++), caseName, 0, "A");
        validateBeanSend(steps.get(offset++), caseName, "A", 0);
        validateBeanSend(steps.get(offset++), caseName, "C", 0);
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

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String expectedString, int expectedInt) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || intField(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException(
                    "SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Pins a two-field SupportBean_S0 send (id, p00) — the shape
     * SupportBean_S0(0, key) sends in ords 0-1 and ord 3. */
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
        if (intField(payload.get("id"), "id") != expectedId
                || !expectedP00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    /** Pins the four-field SupportBean_S0 send (id, p00, p01, p02) of
     * sendInsertUpdate in ord 2. */
    private static void validateS0MergeSend(JsonValue value, String caseName, int expectedId,
                                            String expectedP00, String expectedP01,
                                            String expectedP02) {
        JsonObject step = object(value, "SupportBean_S0 merge step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00", "p01", "p02");
        if (intField(payload.get("id"), "id") != expectedId
                || !expectedP00.equals(string(payload, "p00"))
                || !expectedP01.equals(string(payload, "p01"))
                || !expectedP02.equals(string(payload, "p02"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
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
