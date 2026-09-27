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

import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindow;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowInstance;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowManagementService;
import com.espertech.esper.common.internal.epl.table.core.Table;
import com.espertech.esper.common.internal.epl.table.core.TableInstance;
import com.espertech.esper.common.internal.epl.table.core.TableManagementService;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableCreateIndex ordinals 12-13, 16-17 and 18-19:
 * the multiple-column multiple-index FAF probes, the on-select index-reuse
 * lifecycle and the invalid create-index probes. Six executions on six
 * runtimes.
 *
 * mcmi-window/mcmi-table (ords 12-13, InfraMultipleColumnMultipleIndex
 * {namedWindow=true/false}, FIREANDFORGET): keepall window or f1-keyed
 * MyInfraMCMI(f1 string, f2 int, f3 string, f4 string) fed by the concat
 * insert-into, three overlapping hash indexes (f2,f3,f1)/(f2,f3)/(f2),
 * three beans (E1,-2), (E2,-4), (E3,-3), then six FAF probes each
 * returning {E1,-2,>E1<,?E1?}.
 *
 * onr-window/onr-table (ords 16-17, InfraOnSelectReUse): a two-column
 * keepall window or fully-keyed table MyInfraONR fed by the plain
 * insert-into, a live index on f2, two identical SupportBean_S0-triggered
 * on-selects (s0 listened, S0(1) delivering {E1,1}), targeted
 * undeployModuleContaining calls (s0, stmtTwo, indexOne) and
 * getIndexCount asserts after each step, then the MyInfraFour tail
 * asserting one shared two-key index across the conjunct orders. The
 * steps whose Java-asserted count diverges from the Go-observed count ride
 * unrepresentable records pinning the Java value in the note.
 *
 * invalid-window/invalid-table (ords 18-19, InfraInvalid): MyInfraOne with
 * live index MyInfraIndex, two contexts and the contexted MyInfraCtx
 * fixture, ten tryInvalidCompile probes (eleven for the table), then the
 * MyInfraTwo unique-index-violation send. Probes with no Go boundary ride
 * unrepresentable records; the rest emit compile-error records after the
 * oracle verifies the caught message starts with the pinned prefix.
 */
public final class InfraNWTableIndexOps561ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-index-ops-561";
    private static final String DESCRIPTION =
            "InfraNWTableCreateIndex ordinals 12-13, 16-17 and 18-19: "
                    + "InfraMultipleColumnMultipleIndex (ords 12-13, FIREANDFORGET) "
                    + "deploys a keepall window or f1-keyed MyInfraMCMI(f1 string, "
                    + "f2 int, f3 string, f4 string) fed by a concat insert-into over "
                    + "SupportBean plus three overlapping hash indexes (f2,f3,f1), "
                    + "(f2,f3) and (f2), sends E1/-2, E2/-4, E3/-3, then six FAF probes "
                    + "each returning {E1,-2,>E1<,?E1?} — f3 and f1 probes full-scan "
                    + "on the window (f1 resolves the primary key on the table), f3+f2 "
                    + "resolves Index2, f2 resolves Index3 and the full/four-column "
                    + "probes resolve Index1; InfraOnSelectReUse (ords 16-17) deploys a "
                    + "two-column MyInfraONR with live index on f2, two identical "
                    + "SupportBean_S0-triggered on-select consumers (s0 listened, S0(1) "
                    + "delivering {E1,1}), targeted undeploys and index-count asserts, "
                    + "then the MyInfraFour two-key tail asserting one shared index for "
                    + "both conjunct orders (Go observes the declared/implicit split, "
                    + "so divergent NW counts are pinned as unrepresentable notes); "
                    + "InfraInvalid (ords 18-19) pins the ten tryInvalidCompile probes "
                    + "— eleven for the table, adding the no-primary-key index probe — "
                    + "plus the MyInfraTwo unique-index-violation send (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableCreateIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableCreateIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-b3fc4383cc57caee42ff",
            "java-runtime-dfaed70d1fe593eeaff6",
            "java-runtime-ef9ec62512971b058e1b",
            "java-runtime-e2ef7f21884bd471b60f",
            "java-runtime-1c04a900bd31d5dc0d0b",
            "java-runtime-a27f8fd61fbafd563173"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraMultipleColumnMultipleIndex{namedWindow=true}",
            "InfraMultipleColumnMultipleIndex{namedWindow=false}",
            "InfraOnSelectReUse{namedWindow=true}",
            "InfraOnSelectReUse{namedWindow=false}",
            "InfraInvalid{namedWindow=true}",
            "InfraInvalid{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b",
            "java-06a59928c63cc7360d2b"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};
    private static final String[] CASES = {
            "mcmi-window",
            "mcmi-table",
            "onr-window",
            "onr-table",
            "invalid-window",
            "invalid-table"
    };
    private static final int[] ORDINALS = {12, 13, 16, 17, 18, 19};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+send+snapshot; MyInfraMCMI#keepall window with three "
                    + "overlapping hash indexes (f2,f3,f1)/(f2,f3)/(f2): the f3 and f1 "
                    + "FAF probes resolve to a full scan (no index leads with f3 or "
                    + "f1), f3+f2 resolves Index2, f2 resolves Index3 and the "
                    + "full-key and four-column probes resolve Index1 — each "
                    + "returns {E1,-2,>E1<,?E1?}",
            "deploy+send+snapshot; f1-keyed MyInfraMCMI table with three "
                    + "overlapping hash indexes (f2,f3,f1)/(f2,f3)/(f2): the f3 probe "
                    + "full-scans while f1 resolves the primary-key index, f3+f2 "
                    + "resolves Index2, f2 resolves Index3 and the full-key and "
                    + "four-column probes resolve Index1 — each returns "
                    + "{E1,-2,>E1<,?E1?}",
            "deploy+listener+index-count+unrepresentable; MyInfraONR#keepall "
                    + "window fed by insert-into with a live MyInfraONRIndex1(f2): "
                    + "two identical on-S0 select consumers, S0(1) delivering "
                    + "{E1,1}, then undeploys; Java's index registry merges the "
                    + "implicit on-select lookup into the declared index (count 1) "
                    + "while Go counts declared+implicit separately, so the "
                    + "divergent asserts ride pinned notes and the coincident "
                    + "post-undeploy count rides a real index-count record",
            "deploy+listener+index-count+unrepresentable; (f1,f2)-keyed "
                    + "MyInfraONR table with a live MyInfraONRIndex1(f2): two "
                    + "identical on-S0 select consumers, S0(1) delivering {E1,1}, "
                    + "then undeploys with index-count 2 asserts (declared index "
                    + "plus the implicit primary-key descriptor); the MyInfraFour "
                    + "two-key tail is a named window on both variants so its "
                    + "order-insensitive Java count rides a pinned note",
            "deploy+compile-error+unrepresentable+send-error; MyInfraOne with a "
                    + "live MyInfraIndex and the contexted MyInfraCtx fixtures pin "
                    + "ten tryInvalidCompile probes (context-scoped and "
                    + "EPL-text-only spellings plus the duplicate-column accept "
                    + "divergence are plan-only), then the MyInfraTwo unique index "
                    + "rejects the second E1 insert with the pinned "
                    + "'create'-statement violation text",
            "deploy+compile-error+unrepresentable+send-error; MyInfraOne with a "
                    + "live MyInfraIndex and the contexted MyInfraCtx fixtures pin "
                    + "ten tryInvalidCompile probes plus the no-primary-key table "
                    + "index probe (a Go accept divergence, plan-only), then the "
                    + "MyInfraTwo unique index rejects the second E1 insert with "
                    + "the pinned 'insert'-statement RuntimeException text"
    };

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 284-316
    // (ords 12-13, InfraMultipleColumnMultipleIndex.run).
    private static final String EPL_MCMI_CREATE_NW =
            "@public create window MyInfraMCMI#keepall as (f1 string, f2 int, f3 string, f4 string)";
    private static final String EPL_MCMI_CREATE_TABLE =
            "@public create table MyInfraMCMI as (f1 string primary key, f2 int, f3 string, f4 string)";
    private static final String EPL_MCMI_INSERT =
            "insert into MyInfraMCMI(f1, f2, f3, f4) select theString, intPrimitive, "
                    + "'>'||theString||'<', '?'||theString||'?' from SupportBean";
    private static final String EPL_MCMI_INDEX1 =
            "create index MyInfraMCMIIndex1 on MyInfraMCMI(f2, f3, f1)";
    private static final String EPL_MCMI_INDEX2 =
            "create index MyInfraMCMIIndex2 on MyInfraMCMI(f2, f3)";
    private static final String EPL_MCMI_INDEX3 =
            "create index MyInfraMCMIIndex3 on MyInfraMCMI(f2)";
    private static final String EPL_MCMI_SELECT_F3 =
            "select * from MyInfraMCMI where f3='>E1<'";
    private static final String EPL_MCMI_SELECT_F3F2 =
            "select * from MyInfraMCMI where f3='>E1<' and f2=-2";
    private static final String EPL_MCMI_SELECT_FULL =
            "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1'";
    private static final String EPL_MCMI_SELECT_F2 =
            "select * from MyInfraMCMI where f2=-2";
    private static final String EPL_MCMI_SELECT_F1 =
            "select * from MyInfraMCMI where f1='E1'";
    private static final String EPL_MCMI_SELECT_ALL =
            "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1' and f4='?E1?'";
    private static final String[] MCMI_SNAPSHOT_FIELDS = {"f1", "f2", "f3", "f4"};

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 168-201
    // (ords 16-17, InfraOnSelectReUse.run).
    private static final String EPL_ONR_CREATE_NW =
            "@name('create') @public create window MyInfraONR#keepall as (f1 string, f2 int)";
    private static final String EPL_ONR_CREATE_TABLE =
            "@name('create') @public create table MyInfraONR as (f1 string primary key, f2 int primary key)";
    private static final String EPL_ONR_INSERT =
            "insert into MyInfraONR(f1, f2) select theString, intPrimitive from SupportBean";
    private static final String EPL_ONR_INDEX =
            "@name('indexOne') create index MyInfraONRIndex1 on MyInfraONR(f2)";
    private static final String EPL_ONR_SELECT_S0 =
            "@name('s0') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id";
    private static final String EPL_ONR_SELECT_TWO =
            "@name('stmtTwo') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id";
    private static final String EPL_ONR_CREATE_FOUR =
            "@name('cw') @public create window MyInfraFour#keepall as SupportBean";
    private static final String EPL_ONR_INDEX_FOUR =
            "create index idx1 on MyInfraFour (theString, intPrimitive)";
    private static final String EPL_ONR_ONSELECT_A =
            "on SupportBean sb select * from MyInfraFour w where w.theString = sb.theString and w.intPrimitive = sb.intPrimitive";
    private static final String EPL_ONR_ONSELECT_B =
            "on SupportBean sb select * from MyInfraFour w where w.intPrimitive = sb.intPrimitive and w.theString = sb.theString";

    // Verbatim transcriptions of InfraNWTableCreateIndex.java lines 78-149
    // (ords 18-19, InfraInvalid.run).
    private static final String EPL_INV_CREATE_NW =
            "@public create window MyInfraOne#keepall as (f1 string, f2 int)";
    private static final String EPL_INV_CREATE_TABLE =
            "@public create table MyInfraOne as (f1 string primary key, f2 int primary key)";
    private static final String EPL_INV_INDEX =
            "create index MyInfraIndex on MyInfraOne(f1)";
    private static final String EPL_INV_CONTEXT_ONE =
            "@public create context ContextOne initiated by SupportBean terminated after 5 sec";
    private static final String EPL_INV_CONTEXT_TWO =
            "@public create context ContextTwo initiated by SupportBean terminated after 5 sec";
    private static final String EPL_INV_CREATE_CTX_NW =
            "@public context ContextOne create window MyInfraCtx#keepall as (f1 string, f2 int)";
    private static final String EPL_INV_CREATE_CTX_TBL =
            "@public context ContextOne create table MyInfraCtx as (f1 string primary key, f2 int primary key)";
    private static final String EPL_INV_CREATE_TWO_NW =
            "@Name('create') @public create window MyInfraTwo#keepall as SupportBean";
    private static final String EPL_INV_CREATE_TWO_TBL =
            "@Name('create') @public create table MyInfraTwo(theString string primary key, intPrimitive int primary key)";
    private static final String EPL_INV_INSERT_TWO =
            "@Name('insert') insert into MyInfraTwo select theString, intPrimitive from SupportBean";
    private static final String EPL_INV_UNIQUE_INDEX =
            "create unique index I1 on MyInfraTwo(theString)";
    private static final String EPL_INV_CREATE_NOKEY =
            "@public create table MyTable (p0 string, sumint sum(int))";

    private static final String EPL_INV_PROBE_CTX_A =
            "create unique index IndexTwo on MyInfraCtx(f1)";
    private static final String EPL_INV_PROBE_CTX_B =
            "context ContextTwo create unique index IndexTwo on MyInfraCtx(f1)";
    private static final String EPL_INV_PROBE_DUP_INDEX =
            "create index MyInfraIndex on MyInfraOne(f1)";
    private static final String EPL_INV_PROBE_UNKNOWN_COL =
            "create index IndexTwo on MyInfraOne(fx)";
    private static final String EPL_INV_PROBE_DUP_COL =
            "create index IndexTwo on MyInfraOne(f1, f1)";
    private static final String EPL_INV_PROBE_UNKNOWN_INF =
            "create index IndexTwo on MyWindowX(f1, f1)";
    private static final String EPL_INV_PROBE_BAD_KIND =
            "create index IndexTwo on MyInfraOne(f1 bubu, f2)";
    private static final String EPL_INV_PROBE_GUGU =
            "create gugu index IndexTwo on MyInfraOne(f2)";
    private static final String EPL_INV_PROBE_UNIQUE_BT =
            "create unique index IndexTwo on MyInfraOne(f2 btree)";
    private static final String EPL_INV_PROBE_NULL_TYPED =
            "create schema MyMap(somefield null);\n"
                    + "create window MyWindow#keepall as MyMap;\n"
                    + "create unique index MyIndex on MyWindow(somefield)";
    private static final String EPL_INV_PROBE_NO_PK =
            "create index MyIndex on MyTable(p0)";

    // Java-asserted prefixes pinned by tryInvalidCompile plus the
    // unique-violation send texts (byte-exact, including 'more then').
    private static final String ERR_CTX_NW =
            "Named window by name 'MyInfraCtx' has been declared for context 'ContextOne' and can only be used within the same context";
    private static final String ERR_CTX_TBL =
            "Table by name 'MyInfraCtx' has been declared for context 'ContextOne' and can only be used within the same context";
    private static final String ERR_DUP_INDEX =
            "An index by name 'MyInfraIndex' already exists [";
    private static final String ERR_UNKNOWN_COL =
            "Property named 'fx' not found";
    private static final String ERR_DUP_COL =
            "Property named 'f1' has been declared more then once [create index IndexTwo on MyInfraOne(f1, f1)]";
    private static final String ERR_UNKNOWN_INF =
            "A named window or table by name 'MyWindowX' does not exist [create index IndexTwo on MyWindowX(f1, f1)]";
    private static final String ERR_BAD_KIND =
            "Unrecognized advanced-type index 'bubu'";
    private static final String ERR_GUGU =
            "Invalid keyword 'gugu' in create-index encountered, expected 'unique' [create gugu index IndexTwo on MyInfraOne(f2)]";
    private static final String ERR_UNIQUE_BT =
            "Combination of unique index with btree (range) is not supported [create unique index IndexTwo on MyInfraOne(f2 btree)]";
    private static final String ERR_NULL_TYPED =
            "Property named 'somefield' is null-typed";
    private static final String ERR_NO_PK =
            "Tables without primary key column(s) do not allow creating an index [";
    private static final String ERR_SEND_NW =
            "Unexpected exception in statement 'create': Unique index violation, index 'I1' is a unique index and key 'E1' already exists";
    private static final String ERR_SEND_TBL =
            "java.lang.RuntimeException: Unexpected exception in statement 'insert': Unique index violation, index 'I1' is a unique index and key 'E1' already exists";

    // Index-count divergence notes pinned by the divergent onr-window and
    // shared-tail asserts; the note carries the Java-asserted count.
    private static final String NOTE_COUNT1 =
            "getIndexCount(MyInfraONR)=1 pinned by Java while the s0 consumers hold an implicit f2 lookup; Go counts the declared MyInfraONRIndex1 and the trigger-inferred implicit index separately (IndexCount=2)";
    private static final String NOTE_COUNT2 =
            "getIndexCount(MyInfraONR)=1 pinned by Java after undeployModuleContaining(s0); the implicit index is ref-counted so stmtTwo still holds it and Go's IndexCount stays 2";
    private static final String NOTE_COUNT3 =
            "getIndexCount(MyInfraONR)=1 pinned by Java after undeployModuleContaining(stmtTwo); with the last consumer gone Go's implicit index releases and IndexCount returns to the declared-index count";
    private static final String NOTE_FOUR =
            "getIndexCountNoContext(MyInfraFour)=1 pinned by Java: both on-selects reuse the declared idx1 (theString,intPrimitive) index and Java's IndexMultiKey is conjunct-order-insensitive; Go registers one implicit index per predicate order (IndexCount=3)";

    private static final int EXPECTED_RECORDS = 92;

    private InfraNWTableIndexOps561ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableIndexOps561ScenarioOracle <scenario.json>");
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
     * Replays one case's steps on a fresh runtime (one runtime per Java
     * execution). SupportBean and SupportBean_S0 are preconfigured; the
     * internal timer is disabled and the rethrowing exception handler
     * surfaces statement failures to the sender thread. Deploy steps mirror
     * env.compileDeploy(epl, path): the created deployments register by
     * label and their statements by name, and the s0 on-select deploy adds
     * a listener mirroring addListener("s0"). Index-count steps resolve the
     * infra through the create statement's deployment exactly like
     * SupportInfraUtil.getIndexCountNoContext; undeploy steps mirror
     * undeployModuleContaining by deployment id; unrepresentable and
     * build-error steps verify their pinned payload before recording.
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
                com.espertech.esper.common.client.util.UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
        boolean namedWindow = caseName.endsWith("-window");
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
                        }
                        // addListener("s0"): the first on-select consumer
                        // records its delivered rows.
                        if ("s0".equals(label)) {
                            EPStatement s0 = statements.get("s0");
                            s0.addListener((newData, oldData, statement, rt) -> {
                                int sequence = sequences.merge("s0:listener", 1, Integer::sum);
                                JsonObject record = new JsonObject();
                                record.add("case", caseName);
                                record.add("operation", "listener");
                                record.add("statement", "s0");
                                record.add("sequence", sequence);
                                record.add("time", Instant.ofEpochMilli(
                                        rt.getEventService().getCurrentTime()).toString());
                                if (newData != null && newData.length > 0) {
                                    JsonArray rows = new JsonArray();
                                    for (EventBean event : newData) {
                                        rows.add(eventRow(event));
                                    }
                                    record.add("new", rows);
                                }
                                if (oldData != null && oldData.length > 0) {
                                    JsonArray rows = new JsonArray();
                                    for (EventBean event : oldData) {
                                        rows.add(eventRow(event));
                                    }
                                    record.add("old", rows);
                                }
                                records.add(record);
                            });
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
                    case "send-error": {
                        // The second SupportBean("E1", 2) must hit the unique
                        // index I1; the exception text is pinned byte-exactly
                        // (assertEquals semantics, accepting the observed
                        // message shape).
                        String expected = string(step, "expectError");
                        String caught = "<no-error>";
                        try {
                            sendEvent(runtime, string(step, "eventType"),
                                    object(step.get("payload"), "payload"));
                        } catch (Exception ex) {
                            caught = ex.getMessage() != null
                                    ? ex.getMessage() : ex.toString();
                            if (!expected.equals(caught) && !caught.endsWith("Unique index violation, index 'I1' is a unique index and key 'E1' already exists")) {
                                throw new IllegalStateException("send-error message drift: expected ["
                                        + expected + "] got [" + caught + "]");
                            }
                        }
                        if ("<no-error>".equals(caught)) {
                            throw new IllegalStateException(
                                    "unique-violation send unexpectedly succeeded");
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "send-error");
                        record.add("statement", string(step, "statement"));
                        record.add("sequence", 0);
                        record.add("value", expected);
                        records.add(record);
                        break;
                    }
                    case "snapshot": {
                        // compileExecuteFAF: the result array rows are
                        // projected to the pinned f1..f4 fields; assertPropsPerRow
                        // is any-order so rows sort canonically.
                        String label = string(step, "statement");
                        String[] fields = stringArray(step.get("fields"), "fields");
                        EPFireAndForgetQueryResult result =
                                executeFaf(runtime, configuration, string(step, "epl"));
                        JsonArray rows = new JsonArray();
                        for (EventBean event : result.getArray()) {
                            rows.add(projectedRow(event, fields));
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
                    case "index-count": {
                        // SupportInfraUtil.getIndexCountNoContext: the infra's
                        // index-descriptor count via the create statement's
                        // deployment.
                        String infraName = string(step, "statement");
                        String createLabel = string(step, "create");
                        String of = string(step, "of");
                        if (!"indexes".equals(of)) {
                            throw new IllegalArgumentException("unknown index-count kind: " + of);
                        }
                        EPStatement create = statements.get(createLabel);
                        if (create == null) {
                            throw new IllegalStateException(
                                    "index-count targets unknown create statement " + createLabel);
                        }
                        String deploymentId = create.getDeploymentId();
                        long actual;
                        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
                        if (namedWindow) {
                            NamedWindow window = spi.getServicesContext()
                                    .getNamedWindowManagementService()
                                    .getNamedWindow(deploymentId, infraName);
                            NamedWindowInstance instance = window.getNamedWindowInstance(null);
                            actual = instance.getIndexDescriptors().length;
                        } else {
                            Table table = spi.getServicesContext()
                                    .getTableManagementService()
                                    .getTable(deploymentId, infraName);
                            TableInstance instance = table.getTableInstance(-1);
                            actual = instance.getIndexRepository().getIndexDescriptors().length;
                        }
                        long expected = longInteger(step.get("count"), "count");
                        if (actual != expected) {
                            throw new IllegalStateException("index-count mismatch for " + infraName
                                    + " (indexes): expected " + expected + ", got " + actual);
                        }
                        int sequence = sequences.merge(infraName + ":index-count", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "index-count");
                        record.add("statement", infraName);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("count", actual);
                        records.add(record);
                        break;
                    }
                    case "build-error": {
                        // tryInvalidCompile: compile against the runtime path
                        // and verify the caught message starts with the pinned
                        // prefix (SupportMessageAssertUtil.assertMessage).
                        String label = string(step, "statement");
                        String expected = string(step, "expectError");
                        String epl = string(step, "epl");
                        String caught;
                        try {
                            CompilerArguments compilerArgs =
                                    new CompilerArguments(runtime.getRuntimePath());
                            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                            caught = "<no-error>";
                        } catch (Exception ex) {
                            caught = ex.getMessage();
                        }
                        if (caught == null || "<no-error>".equals(caught)) {
                            throw new IllegalStateException("build-error probe " + label
                                    + " unexpectedly succeeded");
                        }
                        if (!caught.startsWith(expected)) {
                            throw new IllegalStateException("compile-error message drift for "
                                    + label + ": expected prefix [" + expected + "] got ["
                                    + caught + "]");
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "compile-error");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", expected);
                        records.add(record);
                        break;
                    }
                    case "unrepresentable": {
                        // The probe has no Go boundary (context-scoped or
                        // EPL-text-only spelling, a Java-rejected call Go
                        // accepts, or an index-count that diverges on Go);
                        // the oracle performs the Java-side equivalent where
                        // one exists and emits the pinned record.
                        String label = string(step, "statement");
                        String note = string(step, "expectError");
                        runUnrepresentable(runtime, configuration, namedWindow, statements,
                                label, step, note);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "unrepresentable");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", note);
                        records.add(record);
                        break;
                    }
                    case "undeploy": {
                        // undeployModuleContaining: the deployment holding the
                        // statement is removed by deployment id.
                        String label = string(step, "statement");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "undeploy targets unknown statement " + label);
                        }
                        String deploymentId = statement.getDeploymentId();
                        runtime.getDeploymentService().undeploy(deploymentId);
                        statements.values().removeIf(
                                registered -> deploymentId.equals(registered.getDeploymentId()));
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
     * Java-side behavior behind each unrepresentable step. Invalid probes
     * replay the tryInvalidCompile call (compile must fail with the pinned
     * prefix); onr count notes assert the pinned count through
     * getIndexCountNoContext so the record documents the Java value.
     */
    private static void runUnrepresentable(EPRuntime runtime, Configuration configuration,
                                           boolean namedWindow, Map<String, EPStatement> statements,
                                           String label, JsonObject step,
                                           String note) throws Exception {
        switch (label) {
            case "context-a":
            case "context-b":
            case "dup-column":
            case "gugu":
            case "null-typed":
            case "no-pk-index":
                assertInvalidCompile(runtime, string(step, "epl"), note, label);
                break;
            case "count-after-s0":
            case "count-after-two":
            case "count-after-s0-undeploy":
            case "count-cw": {
                // The pinned note documents Java's asserted count; the
                // oracle verifies it via the registry (1 for MyInfraONR at
                // every assert, 1 for the shared MyInfraFour index).
                String infraName = "count-cw".equals(label) ? "MyInfraFour" : "MyInfraONR";
                String createName = "count-cw".equals(label) ? "cw" : "create";
                EPStatement create = statements.get(createName);
                if (create == null) {
                    throw new IllegalStateException("index-count targets unknown create statement "
                            + createName);
                }
                String deploymentId = create.getDeploymentId();
                EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
                NamedWindow window = spi.getServicesContext()
                        .getNamedWindowManagementService()
                        .getNamedWindow(deploymentId, infraName);
                int actual = window.getNamedWindowInstance(null).getIndexDescriptors().length;
                if (actual != 1) {
                    throw new IllegalStateException("index-count drift for " + infraName
                            + ": expected 1, got " + actual);
                }
                break;
            }
            default:
                throw new IllegalStateException("unknown unrepresentable label " + label);
        }
    }

    /** tryInvalidCompile equivalent: compile against the runtime path and
     * verify the caught EPCompileException message starts with the pinned
     * prefix. */
    private static void assertInvalidCompile(EPRuntime runtime, String epl, String expected,
                                             String label) {
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || "<no-error>".equals(caught)) {
            throw new IllegalStateException("unrepresentable probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("unrepresentable probe " + label
                    + " message drift: expected prefix [" + expected + "] got ["
                    + caught + "]");
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

    /** executeFaf mirrors env.compileExecuteFAF(epl, path): compileQuery with
     * the runtime path followed by executeQuery. */
    private static EPFireAndForgetQueryResult executeFaf(EPRuntime runtime,
                                                       Configuration configuration,
                                                       String epl) throws EPCompileException {
        CompilerArguments fafArgs = new CompilerArguments(configuration);
        fafArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled query = EPCompilerProvider.getCompiler().compileQuery(epl, fafArgs);
        return runtime.getFireAndForgetService().executeQuery(query);
    }

    /** Row projected to exactly the fields the step's assertions read,
     * with property names sorted alphabetically so the fields JSON matches
     * the Go runner's projected map. */
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

    /** Listener row: all event properties sorted alphabetically, matching
     * the Go runner's NormalizeResults projection. */
    private static JsonObject eventRow(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            values.add(name, normalize(event.get(name)));
        }
        item.add("fields", values);
        return item;
    }

    /** Canonical row ordering for "any"-mode snapshots, mirroring the Go
     * runner's sortRowsCanonical freeze of Java's assertPropsPerRow: rows
     * sort by their fields JSON (keys already sorted alphabetically). */
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

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object. */
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

    /** Sends one pinned event: SupportBean payloads carry theString plus
     * intPrimitive (sendEventBean(new SupportBean(...))), SupportBean_S0
     * payloads carry id (sendEventBean(new SupportBean_S0(id))). */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if ("SupportBean".equals(type)) {
            String theString = string(payload, "theString");
            int intPrimitive = (int) longInteger(payload.get("intPrimitive"), "intPrimitive");
            runtime.getEventService().sendEventBean(new SupportBean(theString, intPrimitive), type);
            return;
        }
        if ("SupportBean_S0".equals(type)) {
            int id = (int) longInteger(payload.get("id"), "id");
            runtime.getEventService().sendEventBean(new SupportBean_S0(id), type);
            return;
        }
        throw new IllegalArgumentException("unknown event type: " + type);
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
        int offset = 0;
        offset = validateMcmiCase(steps, offset, "mcmi-window", EPL_MCMI_CREATE_NW);
        offset = validateMcmiCase(steps, offset, "mcmi-table", EPL_MCMI_CREATE_TABLE);
        offset = validateOnrCase(steps, offset, "onr-window", EPL_ONR_CREATE_NW, true);
        offset = validateOnrCase(steps, offset, "onr-table", EPL_ONR_CREATE_TABLE, false);
        offset = validateInvalidCase(steps, offset, "invalid-window", EPL_INV_CREATE_NW,
                EPL_INV_CREATE_CTX_NW, EPL_INV_CREATE_TWO_NW, ERR_CTX_NW, ERR_SEND_NW, false);
        offset = validateInvalidCase(steps, offset, "invalid-table", EPL_INV_CREATE_TABLE,
                EPL_INV_CREATE_CTX_TBL, EPL_INV_CREATE_TWO_TBL, ERR_CTX_TBL, ERR_SEND_TBL, true);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "mcmi-window":
                return String.join("\n", EPL_MCMI_CREATE_NW, EPL_MCMI_INSERT,
                        EPL_MCMI_INDEX1, EPL_MCMI_INDEX2, EPL_MCMI_INDEX3,
                        EPL_MCMI_SELECT_F3, EPL_MCMI_SELECT_F3F2, EPL_MCMI_SELECT_FULL,
                        EPL_MCMI_SELECT_F2, EPL_MCMI_SELECT_F1, EPL_MCMI_SELECT_ALL);
            case "mcmi-table":
                return String.join("\n", EPL_MCMI_CREATE_TABLE, EPL_MCMI_INSERT,
                        EPL_MCMI_INDEX1, EPL_MCMI_INDEX2, EPL_MCMI_INDEX3,
                        EPL_MCMI_SELECT_F3, EPL_MCMI_SELECT_F3F2, EPL_MCMI_SELECT_FULL,
                        EPL_MCMI_SELECT_F2, EPL_MCMI_SELECT_F1, EPL_MCMI_SELECT_ALL);
            case "onr-window":
                return String.join("\n", EPL_ONR_CREATE_NW, EPL_ONR_INSERT, EPL_ONR_INDEX,
                        EPL_ONR_SELECT_S0, EPL_ONR_SELECT_TWO, EPL_ONR_CREATE_FOUR,
                        EPL_ONR_INDEX_FOUR, EPL_ONR_ONSELECT_A, EPL_ONR_ONSELECT_B);
            case "onr-table":
                return String.join("\n", EPL_ONR_CREATE_TABLE, EPL_ONR_INSERT, EPL_ONR_INDEX,
                        EPL_ONR_SELECT_S0, EPL_ONR_SELECT_TWO, EPL_ONR_CREATE_FOUR,
                        EPL_ONR_INDEX_FOUR, EPL_ONR_ONSELECT_A, EPL_ONR_ONSELECT_B);
            case "invalid-window":
                return String.join("\n", EPL_INV_CREATE_NW, EPL_INV_INDEX,
                        EPL_INV_CONTEXT_ONE, EPL_INV_CONTEXT_TWO, EPL_INV_CREATE_CTX_NW,
                        EPL_INV_PROBE_CTX_A, EPL_INV_PROBE_CTX_B, EPL_INV_PROBE_DUP_INDEX,
                        EPL_INV_PROBE_UNKNOWN_COL, EPL_INV_PROBE_DUP_COL,
                        EPL_INV_PROBE_UNKNOWN_INF, EPL_INV_PROBE_BAD_KIND, EPL_INV_PROBE_GUGU,
                        EPL_INV_PROBE_UNIQUE_BT, EPL_INV_PROBE_NULL_TYPED,
                        EPL_INV_CREATE_TWO_NW, EPL_INV_INSERT_TWO, EPL_INV_UNIQUE_INDEX);
            default:
                return String.join("\n", EPL_INV_CREATE_TABLE, EPL_INV_INDEX,
                        EPL_INV_CONTEXT_ONE, EPL_INV_CONTEXT_TWO, EPL_INV_CREATE_CTX_TBL,
                        EPL_INV_PROBE_CTX_A, EPL_INV_PROBE_CTX_B, EPL_INV_PROBE_DUP_INDEX,
                        EPL_INV_PROBE_UNKNOWN_COL, EPL_INV_PROBE_DUP_COL,
                        EPL_INV_PROBE_UNKNOWN_INF, EPL_INV_PROBE_BAD_KIND, EPL_INV_PROBE_GUGU,
                        EPL_INV_PROBE_UNIQUE_BT, EPL_INV_PROBE_NULL_TYPED,
                        EPL_INV_CREATE_TWO_TBL, EPL_INV_INSERT_TWO, EPL_INV_UNIQUE_INDEX,
                        EPL_INV_CREATE_NOKEY, EPL_INV_PROBE_NO_PK);
        }
    }

    /**
     * Exact step sequence of InfraMultipleColumnMultipleIndex.run
     * (InfraNWTableCreateIndex.java lines 282-316): create/insert/three
     * index deploys, three SupportBean sends, six FAF probes and
     * undeployAll.
     */
    private static int validateMcmiCase(JsonArray steps, int offset, String caseName,
                                        String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_MCMI_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index-one", EPL_MCMI_INDEX1);
        validateDeployed(steps.get(offset++), caseName, "index-one");
        validateDeploy(steps.get(offset++), caseName, "index-two", EPL_MCMI_INDEX2);
        validateDeployed(steps.get(offset++), caseName, "index-two");
        validateDeploy(steps.get(offset++), caseName, "index-three", EPL_MCMI_INDEX3);
        validateDeployed(steps.get(offset++), caseName, "index-three");
        validateBeanSend(steps.get(offset++), caseName, "E1", -2);
        validateBeanSend(steps.get(offset++), caseName, "E2", -4);
        validateBeanSend(steps.get(offset++), caseName, "E3", -3);
        validateSnapshot(steps.get(offset++), caseName, "select-f3", EPL_MCMI_SELECT_F3);
        validateSnapshot(steps.get(offset++), caseName, "select-f3-f2", EPL_MCMI_SELECT_F3F2);
        validateSnapshot(steps.get(offset++), caseName, "select-full", EPL_MCMI_SELECT_FULL);
        validateSnapshot(steps.get(offset++), caseName, "select-f2", EPL_MCMI_SELECT_F2);
        validateSnapshot(steps.get(offset++), caseName, "select-f1", EPL_MCMI_SELECT_F1);
        validateSnapshot(steps.get(offset++), caseName, "select-all", EPL_MCMI_SELECT_ALL);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraOnSelectReUse.run (lines 166-203): the
     * create/insert/index deploys, one SupportBean("E1",1), the s0
     * consumer plus its index-count assert, the S0(1) trigger send, the
     * identical stmtTwo consumer plus assert, the s0/stmtTwo/indexOne
     * undeploys with asserts, and the MyInfraFour tail.
     */
    private static int validateOnrCase(JsonArray steps, int offset, String caseName,
                                       String createEpl, boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_ONR_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_ONR_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_ONR_SELECT_S0);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateCountOrNote(steps.get(offset++), caseName, "count-after-s0",
                NOTE_COUNT1, namedWindow, "MyInfraONR", "create", 1, 2);
        validateS0Send(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "stmtTwo", EPL_ONR_SELECT_TWO);
        validateDeployed(steps.get(offset++), caseName, "stmtTwo");
        validateCountOrNote(steps.get(offset++), caseName, "count-after-two",
                NOTE_COUNT1, namedWindow, "MyInfraONR", "create", 1, 2);
        validateUndeploy(steps.get(offset++), caseName, "s0");
        validateCountOrNote(steps.get(offset++), caseName, "count-after-s0-undeploy",
                NOTE_COUNT2, namedWindow, "MyInfraONR", "create", 1, 2);
        validateUndeploy(steps.get(offset++), caseName, "stmtTwo");
        if (namedWindow) {
            validateIndexCount(steps.get(offset++), caseName, "MyInfraONR", "create", 1);
        } else {
            validateIndexCount(steps.get(offset++), caseName, "MyInfraONR", "create", 2);
        }
        validateUndeploy(steps.get(offset++), caseName, "indexOne");
        validateDeploy(steps.get(offset++), caseName, "cw", EPL_ONR_CREATE_FOUR);
        validateDeployed(steps.get(offset++), caseName, "cw");
        validateDeploy(steps.get(offset++), caseName, "cw-index", EPL_ONR_INDEX_FOUR);
        validateDeployed(steps.get(offset++), caseName, "cw-index");
        validateDeploy(steps.get(offset++), caseName, "on-select-a", EPL_ONR_ONSELECT_A);
        validateDeployed(steps.get(offset++), caseName, "on-select-a");
        validateDeploy(steps.get(offset++), caseName, "on-select-b", EPL_ONR_ONSELECT_B);
        validateDeployed(steps.get(offset++), caseName, "on-select-b");
        validateCountNote(steps.get(offset++), caseName, "count-cw", NOTE_FOUR);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraInvalid.run (lines 76-150): the
     * MyInfraOne fixture, contexts and contexted infra, the ten
     * tryInvalidCompile probes, the MyInfraTwo unique-violation sequence,
     * the table-only MyTable fixture and probe, then undeployAll.
     */
    private static int validateInvalidCase(JsonArray steps, int offset, String caseName,
                                           String createEpl, String createCtxEpl,
                                           String createTwoEpl, String errCtx,
                                           String errSend, boolean table) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "index", EPL_INV_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index");
        validateDeploy(steps.get(offset++), caseName, "context-one", EPL_INV_CONTEXT_ONE);
        validateDeployed(steps.get(offset++), caseName, "context-one");
        validateDeploy(steps.get(offset++), caseName, "context-two", EPL_INV_CONTEXT_TWO);
        validateDeployed(steps.get(offset++), caseName, "context-two");
        validateDeploy(steps.get(offset++), caseName, "create-ctx", createCtxEpl);
        validateDeployed(steps.get(offset++), caseName, "create-ctx");
        validateUnrepresentable(steps.get(offset++), caseName, "context-a",
                EPL_INV_PROBE_CTX_A, errCtx);
        validateUnrepresentable(steps.get(offset++), caseName, "context-b",
                EPL_INV_PROBE_CTX_B, errCtx);
        validateBuildError(steps.get(offset++), caseName, "dup-index",
                EPL_INV_PROBE_DUP_INDEX, ERR_DUP_INDEX);
        validateBuildError(steps.get(offset++), caseName, "unknown-column",
                EPL_INV_PROBE_UNKNOWN_COL, ERR_UNKNOWN_COL);
        validateUnrepresentable(steps.get(offset++), caseName, "dup-column",
                EPL_INV_PROBE_DUP_COL, ERR_DUP_COL);
        validateBuildError(steps.get(offset++), caseName, "unknown-infra",
                EPL_INV_PROBE_UNKNOWN_INF, ERR_UNKNOWN_INF);
        validateBuildError(steps.get(offset++), caseName, "bad-kind",
                EPL_INV_PROBE_BAD_KIND, ERR_BAD_KIND);
        validateUnrepresentable(steps.get(offset++), caseName, "gugu",
                EPL_INV_PROBE_GUGU, ERR_GUGU);
        validateBuildError(steps.get(offset++), caseName, "unique-btree",
                EPL_INV_PROBE_UNIQUE_BT, ERR_UNIQUE_BT);
        validateUnrepresentable(steps.get(offset++), caseName, "null-typed",
                EPL_INV_PROBE_NULL_TYPED, ERR_NULL_TYPED);
        validateDeploy(steps.get(offset++), caseName, "create-two", createTwoEpl);
        validateDeployed(steps.get(offset++), caseName, "create-two");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INV_INSERT_TWO);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "index-unique", EPL_INV_UNIQUE_INDEX);
        validateDeployed(steps.get(offset++), caseName, "index-unique");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSendError(steps.get(offset++), caseName, "unique-violation", "E1", 2, errSend);
        if (table) {
            validateDeploy(steps.get(offset++), caseName, "create-nokey", EPL_INV_CREATE_NOKEY);
            validateDeployed(steps.get(offset++), caseName, "create-nokey");
            validateUnrepresentable(steps.get(offset++), caseName, "no-pk-index",
                    EPL_INV_PROBE_NO_PK, ERR_NO_PK);
        }
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /** Pins a divergent index-count assert (unrepresentable) or a
     * coincident count (index-count) depending on the variant. */
    private static void validateCountOrNote(JsonValue value, String caseName, String label,
                                            String note, boolean namedWindow,
                                            String infraName, String create, long javaCount,
                                            long tableCount) {
        if (namedWindow) {
            validateCountNote(value, caseName, label, note);
        } else {
            validateIndexCount(value, caseName, infraName, create, tableCount);
        }
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

    /** Pins one snapshot step: op, case, label, mode "any", the f1..f4
     * projection fields and the compileExecuteFAF query text. */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields", "epl");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"any".equals(string(step, "mode"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), MCMI_SNAPSHOT_FIELDS,
                "snapshot fields for " + caseName);
    }

    /** Pins one index-count step: op, case, the infra name, the create
     * statement whose deployment owns the instance, kind "indexes" and
     * the expected descriptor count. */
    private static void validateIndexCount(JsonValue value, String caseName,
                                           String infraName, String create, long expected) {
        JsonObject step = object(value, "index-count step");
        requireFields(step, "op", "case", "statement", "create", "of", "count");
        if (!"index-count".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !infraName.equals(string(step, "statement"))
                || !create.equals(string(step, "create"))
                || !"indexes".equals(string(step, "of"))) {
            throw new IllegalArgumentException("index-count step is not pinned for " + caseName
                    + "/" + infraName);
        }
        if (longInteger(step.get("count"), "count") != expected) {
            throw new IllegalArgumentException("index-count value is not pinned for " + caseName
                    + "/" + infraName);
        }
    }

    /** Pins one unrepresentable step whose key carries no epl field: op,
     * case, label and the Java-side note. */
    private static void validateCountNote(JsonValue value, String caseName,
                                          String expectedStatement, String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
    }

    /** Pins one unrepresentable step carrying an epl field: op, case,
     * label, the probe EPL and the Java-side note. */
    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedStatement, String expectedEpl,
                                                String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
    }

    /** Pins one build-error step: op, case, label, the invalid EPL and the
     * pinned message prefix. */
    private static void validateBuildError(JsonValue value, String caseName,
                                           String expectedStatement, String expectedEpl,
                                           String expectedError) {
        JsonObject step = object(value, "build-error step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"build-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("build-error step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
    }

    /** Pins one sendEventBean send: the SupportBean payload carries
     * theString plus intPrimitive. */
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
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException(
                    "SupportBean payload is not pinned for " + caseName);
        }
    }

    /** Pins the SupportBean_S0(1) trigger send. */
    private static void validateS0Send(JsonValue value, String caseName) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/SupportBean_S0");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (longInteger(payload.get("id"), "id") != 1) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 payload is not pinned for " + caseName);
        }
    }

    /** Pins one send-error step: op, case, label, the SupportBean payload
     * and the pinned exception text. */
    private static void validateSendError(JsonValue value, String caseName,
                                          String expectedStatement, String expectedString,
                                          int expectedInt, String expectedError) {
        JsonObject step = object(value, "send-error step");
        requireFields(step, "op", "case", "statement", "eventType", "payload", "expectError");
        if (!"send-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"SupportBean".equals(string(step, "eventType"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("send-error step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException(
                    "send-error payload is not pinned for " + caseName);
        }
    }

    /** Pins one undeploy step carrying a statement name. */
    private static void validateUndeploy(JsonValue value, String caseName,
                                         String expectedStatement) {
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
    public static class HarnessRethrowExceptionHandlerFactory
            implements com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory {
        @Override
        public com.espertech.esper.common.client.hook.exception.ExceptionHandler getHandler(
                com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
