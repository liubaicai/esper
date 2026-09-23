import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
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
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.context.SupportSelectorById;
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
 * Java oracle for the context-key-segmented-infra-prioritized scenario:
 * three ContextKeySegmentedInfra/ContextKeySegmentedPrioritized executions
 * replayed as five cases, each against its own fresh runtime.
 *
 * infra-aggregated-subquery-nw and infra-aggregated-subquery-table replay
 * ContextKeySegmentedInfra.ContextKeySegmentedInfraAggregatedSubquery
 * (inventory ordinal 0, runtime java-runtime-f297e13be96337235ae0), which
 * runs tryAssertionAggregatedSubquery twice under one runtime ID:
 * namedWindow=true then namedWindow=false (table).  Each variant deploys
 * the byte-exact four-statement module — the SegmentedByString keyed
 * context (theString from SupportBean, p00 from SupportBean_S0), the
 * @public contexted MyInfra infra (keepall named window, or table with
 * theString primary key and the byte-exact space before the column-list
 * paren), the contexted insert and the @Audit @name('s0') select that
 * projects select * plus the uncorrelated (select max(intPrimitive) from
 * MyInfra) as mymax over SupportBean_S0.  SupportBean("E1",10) and
 * ("E2",20) feed the infra silently (the trigger stream is S0);
 * S0(0,"E2") and S0(0,"E1") each emit one listener row carrying the S0
 * columns plus mymax 20 and 10; S0(0,"E3") allocates a fresh partition
 * whose MyInfra is empty so mymax renders null (empty-set aggregation,
 * not 0).
 *
 * infra-create-index-nw and infra-create-index-table replay
 * ContextKeySegmentedInfra.ContextKeySegmentedInfraCreateIndex (ordinal
 * 2, runtime java-runtime-f49875a427fb00323404), which runs
 * tryAssertionCreateIndex twice: namedWindow=true then false.  Each
 * variant deploys the byte-exact single module — the OVERLAPPING
 * SegmentedByCustomer context (initiated by SupportBean_S0 s0, terminated
 * by SupportBean_S1(p00 = p10), no partition-by), the contexted MyInfra
 * infra, the insert and the contexted create index.  The named-window
 * insert carries NO context clause (inherited through the contexted
 * window) while the table insert REQUIRES explicit context
 * SegmentedByCustomer; the table variant also drops the space before the
 * column-list paren.  S0(1,"A") and S0(2,"B") initiate partitions 0 and
 * 1; SupportBean("E1",1) broadcasts into BOTH live partitions' MyInfra;
 * the fire-and-forget query select * from MyInfra where intPrimitive = 1
 * under ContextPartitionSelectorById {1} (SupportSelectorById(1),
 * partition B) returns exactly one row {theString=E1, intPrimitive=1};
 * S1(3,"A") terminates partition A only (p10="A" correlates to the
 * initiating s0.p00).
 *
 * keyed-prioritized replays ContextKeySegmentedPrioritized (variant
 * direct, runtime java-runtime-3b57cf3ad453555fb0b5): three separate
 * compileDeploy calls — the @public SegmentedByMessage keyed context,
 * @name('s0') @Drop @Priority(1) select 'test1' and @name('s1')
 * @Priority(0) select 'test2' — under a runtime configured with
 * configuration.getRuntime().getExecution().setPrioritized(true)
 * (TestSuiteContextWConfig precedent).  SupportBean("test msg",1) invokes
 * the s0 listener once (constant 'test1'); s1 is NOT invoked because the
 * higher-priority @Drop consumes the event for that partition — the
 * absence of an s1 record is the assertion.
 *
 * Deploy steps compile the byte-exact EPL with the accumulated module
 * path (env.compileDeploy(epl, path) semantics); the faf step compiles
 * through compileQuery with the same path (env.compileFAF) and executes
 * with the pinned by-id selector.  Java's milestone calls are documented
 * no-ops and carry no steps.  Listener records carry case/operation/
 * statement/sequence/time plus new/old rows; the faf record carries the
 * result rows under "new".
 */
public final class ContextKeySegmentedInfraPrioritizedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-key-segmented-infra-prioritized";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextKeySegmentedInfra.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextKeySegmentedPrioritized.java";

    private static final String DESCRIPTION =
            "ContextKeySegmentedInfra aggregated-subquery and create-index executions plus "
                    + "ContextKeySegmentedPrioritized (three executions, five cases): "
                    + "infra-aggregated-subquery-nw/-table replay "
                    + "ContextKeySegmentedInfraAggregatedSubquery (ord 0, both namedWindow "
                    + "variants under one runtime ID) — a SegmentedByString keyed context, a "
                    + "contexted MyInfra keepall window or theString-primary-key table fed by "
                    + "the contexted insert, and s0 projecting select * plus an uncorrelated "
                    + "max(intPrimitive) subquery over MyInfra; S0 triggers yield mymax 20, 10 "
                    + "and null for the empty E3 partition. infra-create-index-nw/-table replay "
                    + "ContextKeySegmentedInfraCreateIndex (ord 2, both variants) — an "
                    + "OVERLAPPING initiated/terminated SegmentedByCustomer context (no "
                    + "partition-by) whose named-window insert omits the context clause while "
                    + "the table insert requires it; SupportBean E1 broadcasts into both live "
                    + "partitions, the FAF select under by-id selector {1} returns {E1,1}, and "
                    + "S1(3,'A') terminates only partition A. keyed-prioritized replays "
                    + "ContextKeySegmentedPrioritized (direct execution) under "
                    + "setPrioritized(true): s0 @Drop @Priority(1) fires 'test1' while s1 "
                    + "@Priority(0) stays silent.";

    private static final String[] CASES = {
            "infra-aggregated-subquery-nw", "infra-aggregated-subquery-table",
            "infra-create-index-nw", "infra-create-index-table",
            "keyed-prioritized"
    };
    private static final int[] ORDINALS = {0, 0, 2, 2, 0};
    private static final String[] CASE_RUNTIME_IDS = {
            "java-runtime-f297e13be96337235ae0",
            "java-runtime-f297e13be96337235ae0",
            "java-runtime-f49875a427fb00323404",
            "java-runtime-f49875a427fb00323404",
            "java-runtime-3b57cf3ad453555fb0b5"
    };
    private static final String[] CASE_EXECUTION_NAMES = {
            "ContextKeySegmentedInfraAggregatedSubquery",
            "ContextKeySegmentedInfraAggregatedSubquery",
            "ContextKeySegmentedInfraCreateIndex",
            "ContextKeySegmentedInfraCreateIndex",
            "ContextKeySegmentedPrioritized"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f297e13be96337235ae0",
            "java-runtime-f49875a427fb00323404",
            "java-runtime-3b57cf3ad453555fb0b5"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextKeySegmentedInfraAggregatedSubquery",
            "ContextKeySegmentedInfraCreateIndex",
            "ContextKeySegmentedPrioritized"
    };
    private static final String[] STATIC_IDS = {
            "java-46365e0a7205d91894a0",
            "java-4124e1ac4a7995762796",
            "java-fc89ee858ab88751c211"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String PRIORITIZED_CASE = "keyed-prioritized";

    // Verbatim transcriptions of ContextKeySegmentedInfra lines 435-441
    // (tryAssertionAggregatedSubquery): the module keeps the trailing
    // newline on every statement and the table variant keeps the space
    // before the column-list paren.
    private static final String EPL_AGG_MODULE_NW =
            "create context SegmentedByString partition by theString from SupportBean, p00 from SupportBean_S0;\n"
                    + "@public context SegmentedByString create window MyInfra#keepall as SupportBean;\n"
                    + "@Name('insert') context SegmentedByString insert into MyInfra select theString, intPrimitive from SupportBean;\n"
                    + "@Audit @name('s0') context SegmentedByString select *, (select max(intPrimitive) from MyInfra) as mymax from SupportBean_S0;\n";
    private static final String EPL_AGG_MODULE_TABLE =
            "create context SegmentedByString partition by theString from SupportBean, p00 from SupportBean_S0;\n"
                    + "@public context SegmentedByString create table MyInfra (theString string primary key, intPrimitive int);\n"
                    + "@Name('insert') context SegmentedByString insert into MyInfra select theString, intPrimitive from SupportBean;\n"
                    + "@Audit @name('s0') context SegmentedByString select *, (select max(intPrimitive) from MyInfra) as mymax from SupportBean_S0;\n";

    // Verbatim transcriptions of ContextKeySegmentedInfra lines 295-308
    // (tryAssertionCreateIndex): the literal spacing/newline artifacts are
    // part of the pinned text; the named-window insert has NO context
    // clause while the table insert requires context SegmentedByCustomer.
    private static final String EPL_IDX_MODULE_NW =
            "@name('create-ctx') @public create context SegmentedByCustomer   initiated by SupportBean_S0 s0   terminated by SupportBean_S1(p00 = p10);"
                    + "@name('create-infra') @public context SegmentedByCustomer\n"
                    + "create window MyInfra#keepall as SupportBean;"
                    + "@name('insert-into-window') insert into MyInfra select theString, intPrimitive from SupportBean;"
                    + "@name('create-index') context SegmentedByCustomer create index MyIndex on MyInfra(intPrimitive);";
    private static final String EPL_IDX_MODULE_TABLE =
            "@name('create-ctx') @public create context SegmentedByCustomer   initiated by SupportBean_S0 s0   terminated by SupportBean_S1(p00 = p10);"
                    + "@name('create-infra') @public context SegmentedByCustomer\n"
                    + "create table MyInfra(theString string primary key, intPrimitive int);"
                    + "@name('insert-into-table') context SegmentedByCustomer insert into MyInfra select theString, intPrimitive from SupportBean;"
                    + "@name('create-index') context SegmentedByCustomer create index MyIndex on MyInfra(intPrimitive);";
    private static final String EPL_FAF =
            "select * from MyInfra where intPrimitive = 1";

    // Verbatim transcriptions of ContextKeySegmentedPrioritized lines
    // 22-27: three separate compileDeploy calls.
    private static final String EPL_PRIO_CTX =
            "@public create context SegmentedByMessage partition by theString from SupportBean";
    private static final String EPL_PRIO_S0 =
            "@name('s0') @Drop @Priority(1) context SegmentedByMessage select 'test1' from SupportBean";
    private static final String EPL_PRIO_S1 =
            "@name('s1') @Priority(0) context SegmentedByMessage select 'test2' from SupportBean";

    private static final String[] CASE_EPLS = {
            EPL_AGG_MODULE_NW, EPL_AGG_MODULE_TABLE,
            EPL_IDX_MODULE_NW, EPL_IDX_MODULE_TABLE,
            EPL_PRIO_S0
    };

    private static final String[] CASE_OBSERVATIONS = {
            "listener; keyed SegmentedByString context over a keepall named window fed by the "
                    + "contexted insert; SupportBean sends are silent (trigger stream is S0), "
                    + "S0(0,E2)/S0(0,E1) emit one row each carrying the S0 columns plus mymax "
                    + "20/10, and S0(0,E3) allocates an empty partition whose mymax is null",
            "listener; identical observable sequence over the theString-primary-key table "
                    + "variant (byte-exact space before the column-list paren)",
            "faf; overlapping initiated/terminated SegmentedByCustomer context (no "
                    + "partition-by) over a keepall named window whose insert inherits the "
                    + "context; S0(1,A)/S0(2,B) initiate partitions 0/1, SupportBean E1 "
                    + "broadcasts into both, the by-id {1} FAF returns {E1,1}, and S1(3,A) "
                    + "terminates only partition A",
            "faf; identical sequence over the table variant whose insert requires the explicit "
                    + "context SegmentedByCustomer clause",
            "listener; prioritized runtime (setPrioritized(true)) with three separate deploys; "
                    + "s0 @Drop @Priority(1) fires 'test1' for SupportBean(test msg,1) while s1 "
                    + "@Priority(0) is not invoked"
    };

    private static final int EXPECTED_STEPS = 38;
    private static final int EXPECTED_RECORDS = 9;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * send payloads render as their compact JSON; the faf step carries the
     * query EPL plus selector "ids" and the pinned id array.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();

    static {
        for (String caseName : new String[]{
                "infra-aggregated-subquery-nw", "infra-aggregated-subquery-table"}) {
            String module = caseName.endsWith("-nw") ? EPL_AGG_MODULE_NW : EPL_AGG_MODULE_TABLE;
            CASE_STEPS.put(caseName, new String[]{
                    "deploy|" + caseName + "|module||" + module + "|||||||",
                    "send|" + caseName + "||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||||||",
                    "send|" + caseName + "||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}||||||",
                    "send|" + caseName + "||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}||||||",
                    "send|" + caseName + "||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}||||||",
                    "send|" + caseName + "||SupportBean_S0||{\"id\":0,\"p00\":\"E3\"}||||||",
                    "undeploy-all|" + caseName + "||||||||||",
            });
        }
        for (String caseName : new String[]{
                "infra-create-index-nw", "infra-create-index-table"}) {
            String module = caseName.endsWith("-nw") ? EPL_IDX_MODULE_NW : EPL_IDX_MODULE_TABLE;
            CASE_STEPS.put(caseName, new String[]{
                    "deploy|" + caseName + "|module||" + module + "|||||||",
                    "send|" + caseName + "||SupportBean_S0||{\"id\":1,\"p00\":\"A\"}||||||",
                    "send|" + caseName + "||SupportBean_S0||{\"id\":2,\"p00\":\"B\"}||||||",
                    "send|" + caseName + "||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||||||",
                    "faf|" + caseName + "|faf||" + EPL_FAF + "|||||ids|[1]|",
                    "send|" + caseName + "||SupportBean_S1||{\"id\":3,\"p10\":\"A\"}||||||",
                    "undeploy-all|" + caseName + "||||||||||",
            });
        }
        CASE_STEPS.put("keyed-prioritized", new String[]{
                "deploy|keyed-prioritized|ctx||" + EPL_PRIO_CTX + "|||||||",
                "deploy|keyed-prioritized|s0||" + EPL_PRIO_S0 + "|||||||",
                "deploy|keyed-prioritized|s1||" + EPL_PRIO_S1 + "|||||||",
                "send|keyed-prioritized||SupportBean||{\"theString\":\"test msg\",\"intPrimitive\":1}||||||",
                "undeploy-all|keyed-prioritized||||||||||",
        });
    }

    private ContextKeySegmentedInfraPrioritizedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextKeySegmentedInfraPrioritizedScenarioOracle <scenario.json>");
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
     * Replays the case's steps on a fresh runtime with the internal timer
     * disabled and the clock initialized at epoch.  The prioritized case
     * additionally sets configuration.getRuntime().getExecution()
     * .setPrioritized(true) before the runtime is created, mirroring
     * TestSuiteContextWConfig.configurePrioritized.  Deploy steps compile
     * with the accumulated module path; listeners attach to statements
     * named s0 and s1, mirroring env.addListener (the create-index module
     * has neither name so it deploys unlistened).
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
        if (PRIORITIZED_CASE.equals(caseName)) {
            configuration.getRuntime().getExecution().setPrioritized(true);
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
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
                        String epl = string(step, "epl");
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        compilerArgs.getPath().getCompileds().addAll(deployedModules);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        deployedModules.add(compiled);
                        for (EPStatement statement : deployment.getStatements()) {
                            if ("s0".equals(statement.getName())
                                    || "s1".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime, statement));
                            }
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "faf":
                        fafStep(runtime, configuration, caseName, step, records, deployedModules);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployedModules.clear();
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
     * Replays the fire-and-forget step, mirroring the Java execution's
     * env.compileFAF(query, path) plus executeQuery with
     * SupportSelectorById: the query compiles through compileQuery with
     * the accumulated module path and executes under the pinned by-id
     * selector {1} (partition B).  The emitted faf record carries the
     * result rows in result order under "new".
     */
    private static void fafStep(EPRuntime runtime, Configuration configuration, String caseName,
                                JsonObject step, JsonArray records,
                                List<EPCompiled> deployedModules) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        if (!"ids".equals(string(step, "selector"))) {
            throw new IllegalStateException("unsupported faf selector "
                    + string(step, "selector"));
        }
        Set<Integer> ids = new HashSet<>();
        for (JsonValue id : array(step.get("ids"), "ids")) {
            ids.add(integer(id));
        }
        CompilerArguments fafArgs = new CompilerArguments(configuration);
        fafArgs.getPath().getCompileds().addAll(deployedModules);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery(epl, fafArgs);
        EPFireAndForgetQueryResult result = runtime.getFireAndForgetService()
                .executeQuery(compiled, new ContextPartitionSelector[]{new SupportSelectorById(ids)});
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "faf");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = new JsonArray();
        for (EventBean event : result.getArray()) {
            rows.add(fullRow(event));
        }
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * Each delivery must be a single new-only row: the aggregated-subquery
     * s0 rows carry the S0 columns plus mymax, and the prioritized s0 row
     * carries the constant 'test1' projection.  s1 must never be invoked.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           EPStatement statement) {
        return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + caseName
                        + " listener callback must contain one new-only row");
            }
            EventBean event = newEvents[0];
            JsonObject row = fullRow(event);
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", new JsonArray().add(row));
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject fullRow(EventBean event) {
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
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
                requireFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                requireFields(payload, "id", "p00");
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"),
                        nullableString(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S1": {
                requireFields(payload, "id", "p10");
                SupportBean_S1 bean = new SupportBean_S1(
                        integer(payload, "id"),
                        nullableString(payload, "p10"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
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
                    || !CASE_RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !CASE_EXECUTION_NAMES[index].equals(string(definition, "executionName"))
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
        return integer(object.get(name), name);
    }

    private static int integer(JsonValue value) {
        long number = longInteger(value, "value");
        if (number < Integer.MIN_VALUE || number > Integer.MAX_VALUE) {
            throw new IllegalArgumentException("value is outside the Java int range");
        }
        return (int) number;
    }

    private static int integer(JsonValue value, String name) {
        long number = longInteger(value, name);
        if (number < Integer.MIN_VALUE || number > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) number;
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
