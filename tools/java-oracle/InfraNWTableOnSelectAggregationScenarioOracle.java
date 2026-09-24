import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
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
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the infra-nwtable-on-select-aggregation
 * parity scenario. Mirrors the InfraNWTableOnSelect aggregation slice:
 * InfraSelectAggregationHavingStreamWildcard (ords 6/7, the on-A
 * stream-wildcard select mwc.* as mwcwin grouped by a with having
 * sum(b) = 20 emitting one fragment event per group row),
 * InfraSelectAggregation (ords 16/17, the ungrouped sum(b) snapshot
 * collapse plus the mid-case on-B delete), InfraSelectAggregationCorrelated
 * (ords 18/19, the where a = id correlated sum whose empty match set still
 * emits one null-sum row, with listeners on select and create),
 * InfraSelectAggregationGrouping (ords 20/21, two grouped on-selects in
 * one module — plain group-by ordered desc and the having-filtered
 * variant), and InfraOnSelectMultikeyWArray (ords 26/27, group by an int[]
 * array column loaded through four compileExecuteFAFNoResult inserts).
 * Each class runs once over a keepall named window and once over a
 * primary-key table.
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own;
 * every execution ends with undeployAll). Deploy steps queue per module:
 * the correlated and grouping cases compile their statements as ONE module
 * exactly like the source's single compileDeploy call, while the other
 * cases compile one single-statement module per deploy step. Deployed
 * markers emit one record per statement label. The FafInsert deploy steps
 * run compileExecuteFAFNoResult equivalents (compileQuery + executeQuery,
 * no deployment, no marker). The types steps read the deployed select
 * statement's event type and record positional {name,type} entries with
 * Java class simple names (Integer for the int sums). Listener records
 * carry the new-data rows; the stream-wildcard fragment events render the
 * fragment bean's fields under the mwcwin property.
 */
public final class InfraNWTableOnSelectAggregationScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-select-aggregation";
    private static final String DESCRIPTION =
            "InfraNWTableOnSelect aggregation slice (ords 6/7, 16-21, 26-27): five"
                    + " execution classes run once over a keepall named window and once over a"
                    + " primary-key table. The having-stream-wildcard cases group the correlated"
                    + " rows and emit one fragment event per group row (having-wildcard, ord"
                    + " 6/7); the ungrouped aggregation cases collapse the on-select snapshot to"
                    + " a single sum row, including the correlated where-clause variant whose"
                    + " empty match set still emits one null-sum row (select-aggregation ord"
                    + " 16/17, correlated ord 18/19); the grouping cases run two on-selects in"
                    + " one module — plain group-by ordered desc and the having-filtered variant"
                    + " — over a composite-PK store that retains duplicate a values (grouping,"
                    + " ord 20/21); and the multikey cases group by an int[] array column loaded"
                    + " through fire-and-forget inserts (multikey-w-array, ord 26/27). Deployed"
                    + " markers pin the module fan-out, listener records carry the new-data"
                    + " rows, and types records pin the select statements' output property"
                    + " types (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "infra/nwtable/InfraNWTableOnSelect.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnSelect.java";

    private static final String[] CASES = {
            "having-wildcard-nw", "having-wildcard-table",
            "select-agg-nw", "select-agg-table",
            "correlated-nw", "correlated-table",
            "grouping-nw", "grouping-table",
            "multikey-w-array-nw", "multikey-w-array-table"};
    private static final int[] ORDINALS = {6, 7, 16, 17, 18, 19, 20, 21, 26, 27};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-a83c289aeff7f22c5d10",
            "java-runtime-113be8d12970feaf2fd0",
            "java-runtime-192de61f3c60d848e6c2",
            "java-runtime-99d89e9beadeae52ce8b",
            "java-runtime-fa42452055bf6fa1e4fd",
            "java-runtime-485188699c51be24f2cb",
            "java-runtime-36a62fff425f4c9b8cb6",
            "java-runtime-964dbe543e45c3f14f00",
            "java-runtime-d10a8674ccc6ecc5c70d",
            "java-runtime-5b8dddb5496cfbefa7d7",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraSelectAggregationHavingStreamWildcard{namedWindow=true}",
            "InfraSelectAggregationHavingStreamWildcard{namedWindow=false}",
            "InfraSelectAggregation{namedWindow=true}",
            "InfraSelectAggregation{namedWindow=false}",
            "InfraSelectAggregationCorrelated{namedWindow=true}",
            "InfraSelectAggregationCorrelated{namedWindow=false}",
            "InfraSelectAggregationGrouping{namedWindow=true}",
            "InfraSelectAggregationGrouping{namedWindow=false}",
            "InfraOnSelectMultikeyWArray{namedWindow=true}",
            "InfraOnSelectMultikeyWArray{namedWindow=false}",
    };
    private static final String[] STATIC_IDS = {
            "java-2ec871e1e4d7e32c39a8",
            "java-2ec871e1e4d7e32c39a8",
            "java-36dad03f6eadeaa780c1",
            "java-36dad03f6eadeaa780c1",
            "java-46aadb56f6ef28b6b2bd",
            "java-46aadb56f6ef28b6b2bd",
            "java-acc31838f7d1ff0ce7a6",
            "java-acc31838f7d1ff0ce7a6",
            "java-81de7615b15d1c9671eb",
            "java-81de7615b15d1c9671eb",
    };

    // Transcriptions of InfraNWTableOnSelect.java lines 102-110
    // (multikey), 189-199 (pattern-correlation shape shared with the
    // correlated aggregation), 253-258 (correlation-delete shape),
    // 318-323 (grouping), 395-399 (correlated), 456-467/482 (aggregation)
    // and 736-745 (having-stream-wildcard).
    private static final String EPL_CREATE_SHS_NW =
            "@public create window MyInfraSHS#keepall as (a string, b int)";
    private static final String EPL_CREATE_SHS_TBL =
            "@public create table MyInfraSHS as (a string primary key, b int primary key)";
    private static final String EPL_INSERT_SHS =
            "insert into MyInfraSHS select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_SELECT_SHS =
            "@name('select') on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc"
                    + " where id = a group by a having sum(b) = 20";

    private static final String EPL_CREATE_SA_NW =
            "@name('create') @public create window MyInfraSA#keepall as select theString as a,"
                    + " intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_SA_TBL =
            "@name('create') @public create table MyInfraSA (a string primary key,"
                    + " b int primary key)";
    private static final String EPL_SELECT_SA =
            "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSA";
    private static final String EPL_INSERT_SA =
            "insert into MyInfraSA select theString as a, intPrimitive as b from SupportBean";
    private static final String EPL_DELETE_SA =
            "on SupportBean_B delete from MyInfraSA where id = a";

    private static final String EPL_CREATE_SAC_NW =
            "@name('create') @public create window MyInfraSAC#keepall as select theString as a,"
                    + " intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_SAC_TBL =
            "@name('create') @public create table MyInfraSAC(a string primary key,"
                    + " b int primary key)";
    private static final String EPL_SELECT_SAC =
            "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSAC where a = id";
    private static final String EPL_INSERT_SAC =
            "insert into MyInfraSAC select theString as a, intPrimitive as b from SupportBean";

    private static final String EPL_CREATE_SAG_NW =
            "@name('create') @public create window MyInfraSAG#keepall as select theString as a,"
                    + " intPrimitive as b from SupportBean";
    private static final String EPL_CREATE_SAG_TBL =
            "@name('create') @public create table MyInfraSAG(a string primary key,"
                    + " b int primary key)";
    private static final String EPL_SELECT_SAG =
            "@name('select') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG"
                    + " group by a order by a desc";
    private static final String EPL_SELECT_TWO_SAG =
            "@name('selectTwo') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG"
                    + " group by a having sum(b) > 5 order by a desc";
    private static final String EPL_INSERT_SAG =
            "@name('insert') insert into MyInfraSAG select theString as a, intPrimitive as b"
                    + " from SupportBean";
    private static final String EPL_DELETE_SAG =
            "on SupportBean_B delete from MyInfraSAG where id = a";

    private static final String EPL_CREATE_PC_NW =
            "@name('create') @public create window MyInfraPC#keepall as (id string, array int[],"
                    + " value int)";
    private static final String EPL_CREATE_PC_TBL =
            "@name('create') @public create table MyInfraPC(id string primary key, array int[],"
                    + " value int)";
    private static final String EPL_SELECT_PC =
            "@name('s0') on SupportBean select array, sum(value) as thesum from MyInfraPC"
                    + " group by array";
    private static final String EPL_FAF_E1 = "insert into MyInfraPC values('E1', {1, 2}, 10)";
    private static final String EPL_FAF_E2 = "insert into MyInfraPC values('E2', {1, 2}, 11)";
    private static final String EPL_FAF_E3 = "insert into MyInfraPC values('E3', {1, 2}, 21)";
    private static final String EPL_FAF_E4 = "insert into MyInfraPC values('E4', {1}, 22)";

    private static final String[] CASE_EPLS = {
            EPL_CREATE_SHS_NW + ";\n" + EPL_INSERT_SHS + ";\n" + EPL_SELECT_SHS + ";\n",
            EPL_CREATE_SHS_TBL + ";\n" + EPL_INSERT_SHS + ";\n" + EPL_SELECT_SHS + ";\n",
            EPL_CREATE_SA_NW + ";\n" + EPL_SELECT_SA + ";\n" + EPL_INSERT_SA + ";\n"
                    + EPL_DELETE_SA + ";\n",
            EPL_CREATE_SA_TBL + ";\n" + EPL_SELECT_SA + ";\n" + EPL_INSERT_SA + ";\n"
                    + EPL_DELETE_SA + ";\n",
            EPL_CREATE_SAC_NW + ";\n" + EPL_SELECT_SAC + ";\n" + EPL_INSERT_SAC + ";\n",
            EPL_CREATE_SAC_TBL + ";\n" + EPL_SELECT_SAC + ";\n" + EPL_INSERT_SAC + ";\n",
            EPL_CREATE_SAG_NW + ";\n" + EPL_SELECT_SAG + ";\n" + EPL_SELECT_TWO_SAG + ";\n"
                    + EPL_INSERT_SAG + ";\n" + EPL_DELETE_SAG + ";\n",
            EPL_CREATE_SAG_TBL + ";\n" + EPL_SELECT_SAG + ";\n" + EPL_SELECT_TWO_SAG + ";\n"
                    + EPL_INSERT_SAG + ";\n" + EPL_DELETE_SAG + ";\n",
            EPL_CREATE_PC_NW + ";\n" + EPL_SELECT_PC + ";\n" + EPL_FAF_E1 + ";\n" + EPL_FAF_E2
                    + ";\n" + EPL_FAF_E3 + ";\n" + EPL_FAF_E4 + ";\n",
            EPL_CREATE_PC_TBL + ";\n" + EPL_SELECT_PC + ";\n" + EPL_FAF_E1 + ";\n" + EPL_FAF_E2
                    + ";\n" + EPL_FAF_E3 + ";\n" + EPL_FAF_E4 + ";\n",
    };

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+listener; keepall window MyInfraSHS(a,b) fed by the SupportBean insert;"
                    + " on-A select mwc.* as mwcwin where id = a group by a having sum(b) = 20"
                    + " emits two fragment events (mwcwin.a='E1' each) when A('E1') fires after"
                    + " E1/16, E2/2, E1/4",
            "deployed+listener; composite-PK table MyInfraSHS(a,b) fed by the SupportBean"
                    + " insert; the same on-A stream-wildcard select emits two fragment events"
                    + " when A('E1') fires after E1/16, E2/2, E1/4",
            "deployed+listener+types; keepall window MyInfraSA(a,b) fed by the SupportBean"
                    + " insert; on-A select sum(b) as sumb emits {sumb=6} after E1/1,E2/2,E3/3,"
                    + " {sumb=4} after the on-B delete removes E2, and {sumb=14} after E4/10;"
                    + " the select event type pins one Integer property sumb",
            "deployed+listener+types; composite-PK table MyInfraSA(a,b) fed by the SupportBean"
                    + " insert; the same ungrouped sum select emits {sumb=6}, {sumb=4} after"
                    + " the on-B delete removes E2, and {sumb=14} after E4/10; the select event"
                    + " type pins one Integer property sumb",
            "deployed+listener+types; one module (keepall window MyInfraSAC, on-A select"
                    + " sum(b) as sumb where a = id, SupportBean insert) with listeners on"
                    + " select and create: A('A1') emits {sumb=null} over the empty match set,"
                    + " A('E2') emits {sumb=2} then {sumb=12} once the second E2 row (composite"
                    + " PK allows duplicate a) lands; the select event type pins one Integer"
                    + " property sumb",
            "deployed+listener+types; one module (composite-PK table MyInfraSAC, on-A select"
                    + " sum(b) as sumb where a = id, SupportBean insert) with listeners on"
                    + " select and create: A('A1') emits {sumb=null} over the empty match set,"
                    + " A('E2') emits {sumb=2} then {sumb=12}; the select event type pins one"
                    + " Integer property sumb",
            "deployed+listener+types; one module (keepall window MyInfraSAG, on-A select"
                    + " a,sum(b) group by a order by a desc, on-A selectTwo adding having"
                    + " sum(b) > 5, named SupportBean insert): A('A1') on the empty store fires"
                    + " neither listener; after E1/1,E2/2,E1/5 select emits [{E2,2},{E1,6}]"
                    + " and selectTwo [{E1,6}]; after E4/-1,E2/10,E1/100 select emits"
                    + " [{E4,-1},{E2,12},{E1,106}] and selectTwo [{E2,12},{E1,106}]; the on-B delete"
                    + " removes both E2 rows so A('A3') emits [{E4,-1},{E1,106}] and"
                    + " [{E1,106}]; the select event type pins a=String, sumb=Integer",
            "deployed+listener+types; one module (composite-PK table MyInfraSAG, the same"
                    + " two grouped on-selects, named SupportBean insert): identical trigger"
                    + " sequence — select emits [{E2,2},{E1,6}] then [{E4,-1},{E2,12},{E1,106}]"
                    + " then [{E4,-1},{E1,106}] after the on-B delete; selectTwo emits"
                    + " [{E1,6}] then [{E2,12},{E1,106}] then [{E1,106}]; the select event type pins"
                    + " a=String, sumb=Integer",
            "deployed+listener; keepall window MyInfraPC(id,array int[],value) loaded by"
                    + " four fire-and-forget inserts; on-SupportBean select array, sum(value)"
                    + " as thesum group by array emits {thesum=21} for the shared {1,2} key"
                    + " after E1/E2, then [{thesum=42},{thesum=22}] any-order after"
                    + " E3({1,2},21) and E4({1},22)",
            "deployed+listener; table MyInfraPC(id primary key, array int[], value) loaded"
                    + " by the same four fire-and-forget inserts; the on-SupportBean grouped"
                    + " select emits {thesum=21} then [{thesum=42},{thesum=22}] any-order",
    };

    /**
     * Module grouping: the scenario's deploy steps are per statement so the
     * Go runner can map each onto one plan, while this oracle reproduces the
     * Java fan-out. The correlated and grouping cases compile one module
     * (module key 0); the other cases compile one single-statement module
     * per deploy step (distinct module keys force a flush).
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> shs = new HashMap<>();
        shs.put("create", 0);
        shs.put("insert", 1);
        shs.put("select", 2);
        modules.put("having-wildcard-nw", shs);
        modules.put("having-wildcard-table", shs);
        Map<String, Integer> sa = new HashMap<>();
        sa.put("create", 0);
        sa.put("select", 1);
        sa.put("insert", 2);
        sa.put("delete", 3);
        modules.put("select-agg-nw", sa);
        modules.put("select-agg-table", sa);
        Map<String, Integer> sac = new HashMap<>();
        sac.put("create", 0);
        sac.put("select", 0);
        sac.put("insert", 0);
        modules.put("correlated-nw", sac);
        modules.put("correlated-table", sac);
        Map<String, Integer> sag = new HashMap<>();
        sag.put("create", 0);
        sag.put("select", 0);
        sag.put("selectTwo", 0);
        sag.put("insert", 0);
        sag.put("delete", 1);
        modules.put("grouping-nw", sag);
        modules.put("grouping-table", sag);
        Map<String, Integer> pc = new HashMap<>();
        pc.put("create", 0);
        pc.put("s0", 1);
        modules.put("multikey-w-array-nw", pc);
        modules.put("multikey-w-array-table", pc);
        MODULE_KEYS = Collections.unmodifiableMap(modules);

        Map<String, Set<String>> listened = new HashMap<>();
        listened.put("having-wildcard-nw", Set.of("select"));
        listened.put("having-wildcard-table", Set.of("select"));
        listened.put("select-agg-nw", Set.of("select"));
        listened.put("select-agg-table", Set.of("select"));
        listened.put("correlated-nw", Set.of("select", "create"));
        listened.put("correlated-table", Set.of("select", "create"));
        listened.put("grouping-nw", Set.of("select", "selectTwo"));
        listened.put("grouping-table", Set.of("select", "selectTwo"));
        listened.put("multikey-w-array-nw", Set.of("s0"));
        listened.put("multikey-w-array-table", Set.of("s0"));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    private static final int[] EXPECTED_CASE_RECORDS = {4, 4, 8, 8, 11, 7, 12, 12, 4, 4};
    private static final int EXPECTED_RECORDS = 74;
    private static final int EXPECTED_STEPS = 166;

    private InfraNWTableOnSelectAggregationScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnSelectAggregationScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean_A", idSchema());
        configuration.getCommon().addEventType("SupportBean_B", idSchema());
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        int pendingModule = -1;
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
                if ("deploy".equals(operation)) {
                    String statementName = string(step, "statement");
                    if ("FafInsert".equals(statementName)) {
                        // compileExecuteFAFNoResult: compile the pinned
                        // values-clause insert against the runtime path and
                        // execute it fire-and-forget; no deployment, no
                        // deployed marker.
                        if (!pendingStatements.isEmpty()) {
                            deployModule(runtime, caseName, pendingStatements, pendingEpls,
                                    statementsByName, sequences, records);
                            pendingStatements.clear();
                            pendingEpls.clear();
                        }
                        CompilerArguments fafArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled faf = EPCompilerProvider.getCompiler()
                                .compileQuery(string(step, "epl"), fafArgs);
                        runtime.getFireAndForgetService().executeQuery(faf);
                        continue;
                    }
                    Integer module = MODULE_KEYS.get(caseName).get(statementName);
                    if (module == null) {
                        throw new IllegalStateException("case " + caseName
                                + " deploys unknown statement " + statementName);
                    }
                    if (!pendingStatements.isEmpty() && pendingModule != module.intValue()) {
                        deployModule(runtime, caseName, pendingStatements, pendingEpls,
                                statementsByName, sequences, records);
                        pendingStatements.clear();
                        pendingEpls.clear();
                    }
                    pendingModule = module.intValue();
                    pendingStatements.add(statementName);
                    pendingEpls.add(string(step, "epl"));
                    continue;
                }
                if (!pendingStatements.isEmpty()) {
                    deployModule(runtime, caseName, pendingStatements, pendingEpls,
                            statementsByName, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                switch (operation) {
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
                    case "types": {
                        String label = string(step, "statement");
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "types statement " + label + " was not deployed");
                        }
                        EventType eventType = statement.getEventType();
                        JsonArray entries = new JsonArray();
                        for (String name : eventType.getPropertyNames()) {
                            JsonObject entry = new JsonObject();
                            entry.add("name", name);
                            entry.add("type", eventType.getPropertyType(name).getSimpleName());
                            entries.add(entry);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "types");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("value", entries);
                        records.add(record);
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
            if (!pendingStatements.isEmpty()) {
                deployModule(runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
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
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (a whitespace-normalized form of the suite's module text),
     * then registers the statements by scenario label and attaches the case's
     * listeners. CompilerArguments(runtime.getRuntimePath()) carries the prior
     * deployments' public types so later modules resolve the @public store.
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        EPStatement[] deployed = deployment.getStatements();
        if (deployed.length != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.length + " statements");
        }
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        for (int index = 0; index < statementNames.size(); index++) {
            String label = statementNames.get(index);
            EPStatement statement = deployed[index];
            statementsByName.put(label, statement);
            if (listened.contains(label)) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    /** Map event type for the regression-lib SupportBean_A/_B triggers (id only). */
    private static Map<String, Object> idSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("id", String.class);
        return schema;
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                // The multikey trigger sends a bare `new SupportBean()`
                // (empty payload); the aggregation cases send both fields.
                SupportBean bean = new SupportBean();
                if (payload.get("theString") != null) {
                    bean.setTheString(payload.get("theString").asString());
                }
                if (payload.get("intPrimitive") != null) {
                    bean.setIntPrimitive(integer(payload, "intPrimitive"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_A":
            case "SupportBean_B": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", string(payload, "id"));
                runtime.getEventService().sendEventMap(event, type);
                break;
            }
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
     * {"state":"null"} object, EventBean fragments (the mwcwin stream
     * wildcard) as nested row objects, and Java arrays as JSON arrays —
     * the same shapes the Go normalizer emits.
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
        offset = validateHavingWildcardCase(steps, offset, CASES[0], EPL_CREATE_SHS_NW);
        offset = validateHavingWildcardCase(steps, offset, CASES[1], EPL_CREATE_SHS_TBL);
        offset = validateSelectAggCase(steps, offset, CASES[2], EPL_CREATE_SA_NW);
        offset = validateSelectAggCase(steps, offset, CASES[3], EPL_CREATE_SA_TBL);
        offset = validateCorrelatedCase(steps, offset, CASES[4], EPL_CREATE_SAC_NW);
        offset = validateCorrelatedCase(steps, offset, CASES[5], EPL_CREATE_SAC_TBL);
        offset = validateGroupingCase(steps, offset, CASES[6], EPL_CREATE_SAG_NW);
        offset = validateGroupingCase(steps, offset, CASES[7], EPL_CREATE_SAG_TBL);
        offset = validateMultikeyCase(steps, offset, CASES[8], EPL_CREATE_PC_NW);
        offset = validateMultikeyCase(steps, offset, CASES[9], EPL_CREATE_PC_TBL);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact having-wildcard step sequence mirroring
     * InfraSelectAggregationHavingStreamWildcard lines 734-765: three
     * separately deployed statements, the E1/16, E2/2, E1/4 sends and the
     * A('E1') trigger.
     */
    private static int validateHavingWildcardCase(JsonArray steps, int offset,
                                                  String caseName, String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_SHS);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_SHS);
        validateDeployed(steps.get(offset++), caseName, "select");
        validateBeanSend(steps.get(offset++), caseName, "E1", 16);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E1", 4);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact select-aggregation step sequence mirroring InfraSelectAggregation
     * lines 455-504: three separately deployed statements, the three bean
     * sends, the A1 trigger, the mid-case on-B delete deployment, the B('E2')
     * delete, the A2/A3 triggers around the E4/10 send, the types probe and
     * undeploy-all.
     */
    private static int validateSelectAggCase(JsonArray steps, int offset,
                                             String caseName, String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_SA);
        validateDeployed(steps.get(offset++), caseName, "select");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_SA);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_DELETE_SA);
        validateDeployed(steps.get(offset++), caseName, "delete");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_B", "E2");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A2");
        validateBeanSend(steps.get(offset++), caseName, "E4", 10);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A3");
        validateTypes(steps.get(offset++), caseName, "select");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact correlated step sequence mirroring
     * InfraSelectAggregationCorrelated lines 394-434: the three-statement
     * module (deploy steps queue, deployed markers follow), the E1/E2/E3
     * sends, the A1/E2 triggers around the second E2 insert, the types
     * probe and undeploy-all.
     */
    private static int validateCorrelatedCase(JsonArray steps, int offset,
                                              String caseName, String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_SAC);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_SAC);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeployed(steps.get(offset++), caseName, "select");
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "E2");
        validateBeanSend(steps.get(offset++), caseName, "E2", 10);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "E2");
        validateTypes(steps.get(offset++), caseName, "select");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact grouping step sequence mirroring InfraSelectAggregationGrouping
     * lines 317-375: the four-statement module, the A1 trigger on the empty
     * store, the E1/E2/E1 sends, the A1 trigger, the E4/E2/E1 sends, the A2
     * trigger, the mid-case on-B delete deployment, the B('E2') delete, the
     * A3 trigger, the types probe and undeploy-all.
     */
    private static int validateGroupingCase(JsonArray steps, int offset,
                                            String caseName, String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeploy(steps.get(offset++), caseName, "select", EPL_SELECT_SAG);
        validateDeploy(steps.get(offset++), caseName, "selectTwo", EPL_SELECT_TWO_SAG);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_SAG);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeployed(steps.get(offset++), caseName, "select");
        validateDeployed(steps.get(offset++), caseName, "selectTwo");
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateBeanSend(steps.get(offset++), caseName, "E1", 5);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateBeanSend(steps.get(offset++), caseName, "E4", -1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 10);
        validateBeanSend(steps.get(offset++), caseName, "E1", 100);
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A2");
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_DELETE_SAG);
        validateDeployed(steps.get(offset++), caseName, "delete");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_B", "E2");
        validateIdSend(steps.get(offset++), caseName, "SupportBean_A", "A3");
        validateTypes(steps.get(offset++), caseName, "select");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact multikey step sequence mirroring InfraOnSelectMultikeyWArray
     * lines 100-126: the create and s0 deployments, the four
     * compileExecuteFAFNoResult inserts around two bare SupportBean
     * triggers, and undeploy-all.
     */
    private static int validateMultikeyCase(JsonArray steps, int offset,
                                            String caseName, String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELECT_PC);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateDeploy(steps.get(offset++), caseName, "FafInsert", EPL_FAF_E1);
        validateDeploy(steps.get(offset++), caseName, "FafInsert", EPL_FAF_E2);
        validateEmptyBeanSend(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "FafInsert", EPL_FAF_E3);
        validateDeploy(steps.get(offset++), caseName, "FafInsert", EPL_FAF_E4);
        validateEmptyBeanSend(steps.get(offset++), caseName);
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

    private static void validateTypes(JsonValue value, String caseName,
                                      String expectedStatement) {
        JsonObject step = object(value, "types step");
        requireFields(step, "op", "case", "statement");
        if (!"types".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("types step is not pinned for " + caseName + "/"
                    + expectedStatement);
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

    private static void validateEmptyBeanSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (payload.size() != 0) {
            throw new IllegalArgumentException("SupportBean payload must be empty for " + caseName);
        }
    }

    private static void validateIdSend(JsonValue value, String caseName,
                                       String eventType, String id) {
        JsonObject step = object(value, "id send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("id send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "id payload");
        requireFields(payload, "id");
        if (!id.equals(payload.get("id").asString())) {
            throw new IllegalArgumentException("id payload is not pinned for " + caseName);
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
