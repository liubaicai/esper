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
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableContext ordinals 0-2: tables declared under a
 * context (partitioned, non-overlapping initiated/terminated and
 * context-visibility compile errors). Three executions on three runtimes.
 *
 * InfraPartitioned (ord 0) deploys the CtxPerString segmented context
 * (theString from SupportBean, p00 from SupportBean_S0), the unkeyed
 * context-bound MyTable(thesum sum(int)), an into-table sum feed and the
 * listened s0 `select MyTable.thesum as c0 from SupportBean_S0`; the
 * listener pins c0=110 for S0(0,"E1") and c0=20 for S0(0,"E2") across
 * milestone(0).
 *
 * InfraNonOverlapping (ord 1) deploys the CtxNowTillS0 context
 * (start @now end SupportBean_S0), the keyed context-bound
 * MyTable(pkey primary key, thesum sum(int), col0 string), a grouped
 * into-table sum feed keyed on theString and the listened s0
 * `select pkey as c0, thesum as c1 from MyTable output snapshot when
 * terminated`; each SupportBean_S0(-1) terminator emits the whole
 * partition table as one new-data batch ({E1,110},{E2,20} then
 * {E1,30},{E3,100}). A mid-run `create index MyIdx on MyTable(col0)` and
 * a deploy-only `select * from MyTable, SupportBean_S1 where col0 = p11`
 * join carry deployed markers only.
 *
 * InfraTableContextInvalid (ord 2) deploys the SimpleCtx scheduled
 * context (start after 1 sec end after 1 sec) and the keyed context-bound
 * MyTable, then runs three tryInvalidCompile probes whose pinned Java
 * message prefixes record as compile-error records.
 *
 * Java milestone checkpoints are harness no-ops and carry no steps. The
 * ord-1 listener batches sort rows by their compact field rendering
 * because the Java execution asserts them with
 * assertPropsPerRowLastNewAnyOrder.
 */
public final class InfraTableContextScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-context";
    private static final String DESCRIPTION =
            "InfraTableContext ordinals 0-2: InfraPartitioned deploys the "
                    + "CtxPerString partitioned context, an unkeyed context-bound "
                    + "MyTable, an into-table sum feed and an s0 reading "
                    + "MyTable.thesum per SupportBean_S0 partition key, pinning "
                    + "c0=110 then c0=20; InfraNonOverlapping deploys the "
                    + "CtxNowTillS0 start-@now/end-SupportBean_S0 context, a keyed "
                    + "context-bound MyTable fed by a grouped into-table sum and "
                    + "an s0 emitting the partition table as one "
                    + "output-snapshot-when-terminated batch per "
                    + "SupportBean_S0(-1) terminator ({E1,110},{E2,20} then "
                    + "{E1,30},{E3,100}), with a late create index on col0 and a "
                    + "deploy-only MyTable/SupportBean_S1 join; "
                    + "InfraTableContextInvalid deploys the SimpleCtx scheduled "
                    + "context and a context-bound keyed MyTable, then pins three "
                    + "tryInvalidCompile prefixes for a contextless select, a "
                    + "contextless subquery and a contextless insert-into (Java "
                    + "source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/tbl/InfraTableContext.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableContext.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-8b5b2d92d108da7e8fb2",
            "java-runtime-5c625828c160a26df78b",
            "java-runtime-df03d93aca6b6b5ccd59"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraPartitioned",
            "InfraNonOverlapping",
            "InfraTableContextInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-62ab3ac014745d5ab5c5",
            "java-969b28d4058f010a19af",
            "java-69b99206b5ca50456737"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "context-partitioned",
            "context-nonoverlapping",
            "context-invalid"
    };
    private static final int[] ORDINALS = {0, 1, 2};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; the partitioned context keys MyTable per theString/p00 so "
                    + "S0(0,E1) reads the E1-partition sum 110 and S0(0,E2) reads 20",
            "listener; s0 emits the whole partition table as one any-order new "
                    + "batch per SupportBean_S0(-1) terminator ({E1,110},{E2,20} "
                    + "then {E1,30},{E3,100}); the mid-run create index on col0 "
                    + "and the deploy-only MyTable/SupportBean_S1 join carry "
                    + "deployed markers only",
            "compile-error; three tryInvalidCompile probes pin the "
                    + "context-visibility prefixes for a contextless select, a "
                    + "contextless subquery and a contextless insert-into "
                    + "against SimpleCtx-bound MyTable"
    };

    // Verbatim transcriptions of InfraTableContext lines 90-94 (ord 0);
    // 55-58 and 70-71 (ord 1); and 38-39 plus 41-46 (ord 2, the probe EPLs
    // and their assertMessage prefixes). Each deploy step compiles its EPL
    // as one module against the runtime path.
    private static final String EPL_PART_CTX =
            "@public create context CtxPerString "
                    + "partition by theString from SupportBean, p00 from SupportBean_S0";
    private static final String EPL_PART_CREATE =
            "@public context CtxPerString create table MyTable(thesum sum(int))";
    private static final String EPL_PART_INTO =
            "context CtxPerString into table MyTable select sum(intPrimitive) as thesum "
                    + "from SupportBean";
    private static final String EPL_PART_S0 =
            "@name('s0') context CtxPerString select MyTable.thesum as c0 from SupportBean_S0";

    private static final String EPL_NONOVERLAP_CTX =
            "@public create context CtxNowTillS0 start @now end SupportBean_S0";
    private static final String EPL_NONOVERLAP_CREATE =
            "@public context CtxNowTillS0 create table MyTable(pkey string primary key, "
                    + "thesum sum(int), col0 string)";
    private static final String EPL_NONOVERLAP_INTO =
            "context CtxNowTillS0 into table MyTable select sum(intPrimitive) as thesum "
                    + "from SupportBean group by theString";
    private static final String EPL_NONOVERLAP_S0 =
            "@name('s0') context CtxNowTillS0 select pkey as c0, thesum as c1 from MyTable "
                    + "output snapshot when terminated";
    private static final String EPL_NONOVERLAP_INDEX =
            "context CtxNowTillS0 create index MyIdx on MyTable(col0)";
    private static final String EPL_NONOVERLAP_JOIN =
            "context CtxNowTillS0 select * from MyTable, SupportBean_S1 where col0 = p11";

    private static final String EPL_INVALID_CTX =
            "@public create context SimpleCtx start after 1 sec end after 1 sec";
    private static final String EPL_INVALID_CREATE =
            "@public context SimpleCtx create table MyTable(pkey string primary key, "
                    + "thesum sum(int), col0 string)";

    private static final String EPL_PROBE_SELECT = "select * from MyTable";
    private static final String EPL_PROBE_SUBQUERY =
            "select (select * from MyTable) from SupportBean";
    private static final String EPL_PROBE_INSERT =
            "insert into MyTable select theString as pkey from SupportBean";

    private static final String ERR_TABLE_VISIBILITY =
            "Table by name 'MyTable' has been declared for context 'SimpleCtx' and can "
                    + "only be used within the same context [";
    private static final String ERR_SUBQUERY =
            "Failed to plan subquery number 1 querying MyTable: Mismatch in context "
                    + "specification, the context for the table 'MyTable' is 'SimpleCtx' "
                    + "and the query specifies no context  [select (select * from MyTable) "
                    + "from SupportBean]";

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(Arrays.asList("s0"));
    private static final int EXPECTED_STEPS = 46;
    private static final int EXPECTED_RECORDS = 19;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * build-error steps carry the pinned expectError prefix; send payloads
     * render as their compact JSON.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("context-partitioned", new String[]{
                "deploy|context-partitioned|ctx||" + EPL_PART_CTX + "|||||||",
                "deployed|context-partitioned|ctx|||||||||",
                "deploy|context-partitioned|create||" + EPL_PART_CREATE + "|||||||",
                "deployed|context-partitioned|create|||||||||",
                "deploy|context-partitioned|into||" + EPL_PART_INTO + "|||||||",
                "deployed|context-partitioned|into|||||||||",
                "deploy|context-partitioned|s0||" + EPL_PART_S0 + "|||||||",
                "deployed|context-partitioned|s0|||||||||",
                "send|context-partitioned||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":50}||||||",
                "send|context-partitioned||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}||||||",
                "send|context-partitioned||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":60}||||||",
                "send|context-partitioned||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}||||||",
                "send|context-partitioned||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}||||||",
                "undeploy-all|context-partitioned||||||||||",
        });
        CASE_STEPS.put("context-nonoverlapping", new String[]{
                "deploy|context-nonoverlapping|ctx||" + EPL_NONOVERLAP_CTX + "|||||||",
                "deployed|context-nonoverlapping|ctx|||||||||",
                "deploy|context-nonoverlapping|create||" + EPL_NONOVERLAP_CREATE + "|||||||",
                "deployed|context-nonoverlapping|create|||||||||",
                "deploy|context-nonoverlapping|into||" + EPL_NONOVERLAP_INTO + "|||||||",
                "deployed|context-nonoverlapping|into|||||||||",
                "deploy|context-nonoverlapping|s0||" + EPL_NONOVERLAP_S0 + "|||||||",
                "deployed|context-nonoverlapping|s0|||||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":50}||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":60}||||||",
                "send|context-nonoverlapping||SupportBean_S0||{\"id\":-1}||||||",
                "deploy|context-nonoverlapping|index||" + EPL_NONOVERLAP_INDEX + "|||||||",
                "deployed|context-nonoverlapping|index|||||||||",
                "deploy|context-nonoverlapping|join||" + EPL_NONOVERLAP_JOIN + "|||||||",
                "deployed|context-nonoverlapping|join|||||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":90}||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":30}||||||",
                "send|context-nonoverlapping||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":10}||||||",
                "send|context-nonoverlapping||SupportBean_S0||{\"id\":-1}||||||",
                "undeploy-all|context-nonoverlapping||||||||||",
        });
        CASE_STEPS.put("context-invalid", new String[]{
                "deploy|context-invalid|ctx||" + EPL_INVALID_CTX + "|||||||",
                "deployed|context-invalid|ctx|||||||||",
                "deploy|context-invalid|create||" + EPL_INVALID_CREATE + "|||||||",
                "deployed|context-invalid|create|||||||||",
                "build-error|context-invalid|select-table||" + EPL_PROBE_SELECT + "||"
                        + ERR_TABLE_VISIBILITY + "|||||",
                "build-error|context-invalid|subquery-table||" + EPL_PROBE_SUBQUERY + "||"
                        + ERR_SUBQUERY + "|||||",
                "build-error|context-invalid|insert-table||" + EPL_PROBE_INSERT + "||"
                        + ERR_TABLE_VISIBILITY + "|||||",
                "undeploy-all|context-invalid||||||||||",
        });
    }

    private InfraTableContextScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableContextScenarioOracle <scenario.json>");
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
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). SupportBean, SupportBean_S0 and SupportBean_S1 are
     * preconfigured, the internal timer is disabled and the rethrowing
     * exception handler surfaces statement failures to the sender thread.
     * Every deploy registers its deployment by step label and its
     * statements by name so deployed markers resolve both; the s0
     * statement carries the listener that captures assertEqualsNew and
     * assertPropsPerRowLastNewAnyOrder observations. Build-error steps
     * compile against the runtime path, mirroring
     * env.tryInvalidCompile(path, epl, message).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
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
                    case "build-error":
                        buildErrorStep(runtime, configuration, caseName, step, records);
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
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). All three probes compile against the runtime path,
     * mirroring env.tryInvalidCompile(path, epl, message).
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; the default istream selector means only a new
     * array renders and only when non-empty. Rows sort by their compact
     * field rendering because the ord-1 batches are asserted with
     * assertPropsPerRowLastNewAnyOrder.
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
            JsonArray newRows = sortedRows(rows(newEvents));
            JsonArray oldRows = sortedRows(rows(oldEvents));
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

    /** Sorts rendered rows by their compact field JSON so the any-order
     * Java assertions pin one canonical order on both traces. */
    private static JsonArray sortedRows(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue item : rows) {
            items.add(item);
        }
        items.sort((left, right) -> left.asObject().get("fields").toString()
                .compareTo(right.asObject().get("fields").toString()));
        JsonArray sorted = new JsonArray();
        for (JsonValue item : items) {
            sorted.add(item);
        }
        return sorted;
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
     * default), SupportBean_S0 carries id plus p00/p01/p02 (absent keys
     * stay null, matching the constructors the Java executions use) and
     * SupportBean_S1 carries id plus p10/p11 (never sent by the pinned
     * scenario; registered so the ord-1 join compiles).
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
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(
                        intField(payload.get("id"), "id"),
                        stringOrNull(payload.get("p10")),
                        stringOrNull(payload.get("p11")));
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
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

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
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
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

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order (deploys and build-error probes). */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "context-partitioned":
                return String.join("\n", EPL_PART_CTX, EPL_PART_CREATE, EPL_PART_INTO,
                        EPL_PART_S0);
            case "context-nonoverlapping":
                return String.join("\n", EPL_NONOVERLAP_CTX, EPL_NONOVERLAP_CREATE,
                        EPL_NONOVERLAP_INTO, EPL_NONOVERLAP_S0, EPL_NONOVERLAP_INDEX,
                        EPL_NONOVERLAP_JOIN);
            default:
                return String.join("\n", EPL_INVALID_CTX, EPL_INVALID_CREATE,
                        EPL_PROBE_SELECT, EPL_PROBE_SUBQUERY, EPL_PROBE_INSERT);
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
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText;
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
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
