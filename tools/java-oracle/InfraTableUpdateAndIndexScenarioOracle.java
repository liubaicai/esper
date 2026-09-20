import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EPException;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
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
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
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
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableUpdateAndIndex ordinals 0-4: the table update and
 * unique-index surface. Five executions on five runtimes.
 *
 * InfraEarlyUniqueIndexViolation (ord 0) deploys a two-primary-key
 * MyTableEUIV(pkey0 string, pkey1 int, thecnt count(*)) plus a grouped
 * into-table count feed, sends SupportBean("E1",10) and ("E1",20) so pkey0
 * collides, then exercises four failure phases: a late `create unique index
 * SecIndex on MyTableEUIV(pkey0)` that compiles but deploy-fails with the
 * unique-violation message; a fire-and-forget `update MyTableEUIV set pkey1 =
 * 0` that fails atomically and leaves the table unchanged; an on-update
 * trigger whose SupportBean_S1 send fails through the rethrowing exception
 * handler; and an on-merge compile that is rejected because when-matched
 * updates may not touch unique keys.
 *
 * InfraLateUniqueIndexViolation (ord 1) deploys MyTableLUIV(pkey0, pkey1,
 * col0, thecnt) with distinct pkey0 rows, deploys an on-merge that updates
 * col0, then proves `create unique index MyUniqueSecondary on MyTableLUIV
 * (col0)` deploy-fails because col0 is merge-updated. After
 * undeployModuleContaining("on-merge") an on-update of pkey1 plus a unique
 * index on pkey1 both deploy, and the SupportBean_S1 send fails with the
 * unique-violation message naming MyUniqueSecondary.
 *
 * InfraFAFUpdate (ord 2) deploys MyTableFAFU(pkey0, col0, col1, thecnt) with
 * a non-unique secondary hash index MyIndex on col0 and a grouped into-table
 * feed keyed by theString, then runs the compileExecuteFAF sequence: col0=1
 * for E1, col0=2 for E2, a one-row select on col0=1 through MyIndex, col1=100
 * for E1 and a one-row select on col1=100.
 *
 * InfraTableKeyUpdateSingleKey (ord 3) deploys MyTableSingleKey(pkey0, c0),
 * an insert-into feed and an on-SupportBean_S0 update that renames pkey0;
 * three SupportBean rows then three S0 renames interleaved with iterator
 * any-order assertions (milestone checkpoints are harness no-ops).
 *
 * InfraTableKeyUpdateMultiKey (ord 4) repeats ord 3 against
 * MyTableMultiKey(pkey0, pkey1, c0 long) where only pkey0 of the composite
 * primary key is renamed.
 */
public final class InfraTableUpdateAndIndexScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-update-and-index";
    private static final String DESCRIPTION =
            "InfraTableUpdateAndIndex ordinals 0-4: InfraEarlyUniqueIndexViolation "
                    + "deploys a two-primary-key MyTableEUIV fed by a grouped "
                    + "into-table count, then pins four failure phases — a "
                    + "deploy-time unique-index violation on pkey0, an atomic "
                    + "fire-and-forget update failure on pkey1, an on-update "
                    + "send failure surfaced through the exception handler, and "
                    + "a compile-rejected on-merge unique-key update; "
                    + "InfraLateUniqueIndexViolation pins the merge-updated-"
                    + "column create-index rejection on col0, the "
                    + "undeployModuleContaining boundary, and a late unique "
                    + "index on pkey1 whose on-update send violates it; "
                    + "InfraFAFUpdate runs the fire-and-forget update/select "
                    + "sequence over MyTableFAFU exercising the MyIndex "
                    + "secondary hash index; InfraTableKeyUpdateSingleKey and "
                    + "InfraTableKeyUpdateMultiKey rename the single and "
                    + "composite primary keys through on-SupportBean_S0 "
                    + "updates with iterator any-order assertions. Java "
                    + "milestone checkpoints are harness no-ops and carry no "
                    + "steps (Java source regression-lib/src/main/java/com/"
                    + "espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableUpdateAndIndex.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableUpdateAndIndex.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-878326b2aef272d9ef78",
            "java-runtime-59f3f0884fc9ae9d749f",
            "java-runtime-1118b36d6f38fa1c78a8",
            "java-runtime-16e0f7011601678fa5df",
            "java-runtime-0b3580bcd42327b7d2bb"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraEarlyUniqueIndexViolation",
            "InfraLateUniqueIndexViolation",
            "InfraFAFUpdate",
            "InfraTableKeyUpdateSingleKey",
            "InfraTableKeyUpdateMultiKey"
    };
    private static final String[] STATIC_IDS = {
            "java-c8309a6f377c34ffcdb4",
            "java-8db9b9cd27d04bb86d69",
            "java-4a2462a159e9d648257a",
            "java-ad9c04a3fc0d98d4e4dd",
            "java-97fc9971820f1e5b947d"
    };
    private static final String[] CASES = {
            "early-unique-violation",
            "late-unique-violation",
            "faf-update",
            "key-update-single",
            "key-update-multi"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy-error+faf-error+send-error+build-error; two-row "
                    + "MyTableEUIV colliding on pkey0: late unique index "
                    + "SecIndex deploy-fails on key 'E1', FAF update of pkey1 "
                    + "to 0 fails atomically on MultiKey[E1,0], the on-update "
                    + "S1 send fails through the exception handler, and the "
                    + "on-merge compile is rejected for updating a unique key",
            "deploy-error+send-error; MyTableLUIV with distinct pkey0 rows: "
                    + "unique index on merge-updated col0 deploy-fails, "
                    + "undeployModuleContaining('on-merge') clears the block, "
                    + "on-update plus unique index MyUniqueSecondary on pkey1 "
                    + "deploy and the S1 send violates key '0'",
            "faf; MyTableFAFU with non-unique MyIndex on col0: FAF updates "
                    + "set col0=1/col0=2/col1=100 and the col0=1/col1=100 FAF "
                    + "selects each return the single pkey0 E1 row",
            "snapshot; MyTableSingleKey(pkey0,c0) fed by insert-into; three "
                    + "SupportBean_S0 renames re-key E2->E20, E1->E10, E3->E30 "
                    + "with c0 preserved across iterator any-order asserts",
            "snapshot; MyTableMultiKey(pkey0,pkey1,c0) fed by insert-into; "
                    + "the same three renames update only pkey0 of the "
                    + "composite key while pkey1 and c0 stay pinned"
    };

    // Verbatim transcriptions of InfraTableUpdateAndIndex lines 49-50, 56, 65,
    // 74, 86 (ord 0); 103-109, 114, 117, 127-128 (ord 1); 151-154, 158-163
    // (ord 2); 213-215 (ord 3); and 177-179 (ord 4). Each deploy step compiles
    // its EPL as one module; Faf* deploy labels are fire-and-forget queries
    // (compileExecuteFAF semantics) and register nothing.
    private static final String EPL_EARLY_CREATE =
            "@name('create') @public create table MyTableEUIV as (pkey0 string primary key, pkey1 int primary key, thecnt count(*))";
    private static final String EPL_EARLY_INTO =
            "into table MyTableEUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive";
    private static final String EPL_EARLY_SEC_INDEX =
            "create unique index SecIndex on MyTableEUIV(pkey0)";
    private static final String EPL_EARLY_FAF_UPDATE =
            "update MyTableEUIV set pkey1 = 0";
    private static final String EPL_EARLY_ON_UPDATE =
            "@name('on-update') on SupportBean_S1 update MyTableEUIV set pkey1 = 0";
    private static final String EPL_EARLY_ON_MERGE =
            "@name('on-merge') on SupportBean_S1 merge MyTableEUIV when matched then update set pkey1 = 0";

    private static final String EPL_LATE_CREATE =
            "@name('create') @public create table MyTableLUIV as (pkey0 string primary key, pkey1 int primary key, col0 int, thecnt count(*))";
    private static final String EPL_LATE_INTO =
            "into table MyTableLUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive";
    private static final String EPL_LATE_ON_MERGE =
            "@name('on-merge') on SupportBean_S1 merge MyTableLUIV when matched then update set col0 = 0";
    private static final String EPL_LATE_SEC_INDEX_COL0 =
            "create unique index MyUniqueSecondary on MyTableLUIV (col0)";
    private static final String EPL_LATE_ON_UPDATE =
            "@name('on-update') on SupportBean_S1 update MyTableLUIV set pkey1 = 0";
    private static final String EPL_LATE_SEC_INDEX_PKEY1 =
            "create unique index MyUniqueSecondary on MyTableLUIV (pkey1)";

    private static final String EPL_FAF_CREATE =
            "@public create table MyTableFAFU as (pkey0 string primary key, col0 int, col1 int, thecnt count(*))";
    private static final String EPL_FAF_INDEX =
            "create index MyIndex on MyTableFAFU(col0)";
    private static final String EPL_FAF_INTO =
            "into table MyTableFAFU select count(*) as thecnt from SupportBean group by theString";
    private static final String EPL_FAF_UPDATE_COL0_E1 =
            "update MyTableFAFU set col0 = 1 where pkey0='E1'";
    private static final String EPL_FAF_UPDATE_COL0_E2 =
            "update MyTableFAFU set col0 = 2 where pkey0='E2'";
    private static final String EPL_FAF_SELECT_COL0 =
            "select pkey0 from MyTableFAFU where col0=1";
    private static final String EPL_FAF_UPDATE_COL1_E1 =
            "update MyTableFAFU set col1 = 100 where pkey0='E1'";
    private static final String EPL_FAF_SELECT_COL1 =
            "select pkey0 from MyTableFAFU where col1=100";

    private static final String EPL_SINGLE_CREATE =
            "@name('s0') @public create table MyTableSingleKey(pkey0 string primary key, c0 int)";
    private static final String EPL_SINGLE_INSERT =
            "insert into MyTableSingleKey select theString as pkey0, intPrimitive as c0 from SupportBean";
    private static final String EPL_SINGLE_ON_UPDATE =
            "on SupportBean_S0 update MyTableSingleKey set pkey0 = p01 where pkey0 = p00";

    private static final String EPL_MULTI_CREATE =
            "@name('s1') @public create table MyTableMultiKey(pkey0 string primary key, pkey1 int primary key, c0 long)";
    private static final String EPL_MULTI_INSERT =
            "insert into MyTableMultiKey select theString as pkey0, intPrimitive as pkey1, longPrimitive as c0 from SupportBean";
    private static final String EPL_MULTI_ON_UPDATE =
            "on SupportBean_S0 update MyTableMultiKey set pkey0 = p01 where pkey0 = p00";

    // Pinned Java message prefixes asserted by SupportMessageAssertUtil
    // (startsWith semantics); the error steps emit this text as the record
    // value after the real exception fires with the right class.
    private static final String ERR_EARLY_SEC_INDEX =
            "Failed to deploy: Unique index violation, index 'SecIndex' is a unique index and key 'E1' already exists";
    private static final String ERR_EARLY_FAF_UPDATE =
            "Unique index violation, index 'MyTableEUIV' is a unique index and key 'MultiKey[E1,0]' already exists";
    private static final String ERR_EARLY_ON_UPDATE =
            "Unexpected exception in statement 'on-update': Unique index violation, index 'MyTableEUIV' is a unique index and key 'MultiKey[E1,0]' already exists";
    private static final String ERR_ON_MERGE_UNIQUE =
            "Validation failed in when-matched (clause 1): On-merge statements may not update unique keys of tables";
    private static final String ERR_LATE_SEC_INDEX_COL0 =
            "Failed to deploy: Create-index adds a unique key on columns that are updated by one or more on-merge statements";
    private static final String ERR_LATE_ON_UPDATE =
            "Unexpected exception in statement 'on-update': Unique index violation, index 'MyUniqueSecondary' is a unique index and key '0' already exists";

    private static final String[] SNAPSHOT_KEY_FIELDS = {"pkey0", "pkey1"};
    private static final String[] SNAPSHOT_SINGLE_FIELDS = {"pkey0", "c0"};
    private static final String[] SNAPSHOT_MULTI_FIELDS = {"pkey0", "pkey1", "c0"};
    private static final String[] SNAPSHOT_FAF_FIELDS = {"pkey0"};

    private static final int EXPECTED_STEPS = 84;
    private static final int EXPECTED_RECORDS = 34;

    private InfraTableUpdateAndIndexScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableUpdateAndIndexScenarioOracle <scenario.json>");
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
     * its own runtime). The three event types are preconfigured, the internal
     * timer is disabled and the rethrowing exception handler surfaces
     * statement failures to the sender thread. Every non-FAF deploy registers
     * its deployment by step label and its statements by name so deployed
     * markers, snapshots and undeployModuleContaining resolve both; Faf*
     * deploy labels are compileExecuteFAF queries and register nothing.
     * Error steps attempt the operation, assert the pinned exception class
     * and message prefix, then emit the pinned expectError text as the record
     * value.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
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
                        if (label.startsWith("Faf")) {
                            // Fire-and-forget update, mirroring
                            // RegressionEnvironmentBase.compileExecuteFAF:
                            // compiled as a query with the runtime path and
                            // executed on demand; no deployment, no marker.
                            executeFaf(runtime, configuration, epl);
                            break;
                        }
                        EPDeployment deployment = compileDeploy(runtime, configuration, epl);
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
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
                        // The on-update trigger failure surfaces through the
                        // rethrowing exception handler: sendEventBean throws
                        // EPException whose cause carries the handler's
                        // "Unexpected exception in statement ..." text.
                        String label = string(step, "statement");
                        String expected = string(step, "expectError");
                        try {
                            sendEvent(runtime, string(step, "eventType"),
                                    object(step.get("payload"), "payload"));
                            throw new IllegalStateException(
                                    "send-error step " + label + " unexpectedly succeeded");
                        } catch (EPException ex) {
                            Throwable cause = ex.getCause();
                            if (cause == null) {
                                throw new IllegalStateException(
                                        "send-error step " + label + " carries no cause", ex);
                            }
                            assertMessagePrefix(cause.getMessage(), expected, label);
                        }
                        records.add(errorRecord(caseName, operation, label, sequences, runtime,
                                expected));
                        break;
                    }
                    case "snapshot": {
                        String label = string(step, "statement");
                        String[] fields = stringArray(step.get("fields"), "fields");
                        JsonArray rows = new JsonArray();
                        JsonValue eplValue = step.get("epl");
                        if (eplValue instanceof JsonString) {
                            // FAF select (compileExecuteFAF): the result array
                            // rows are projected to the pinned fields.
                            EPFireAndForgetQueryResult result =
                                    executeFaf(runtime, configuration, eplValue.asString());
                            for (EventBean event : result.getArray()) {
                                rows.add(projectedRow(event, fields));
                            }
                        } else {
                            EPStatement statement = statements.get(label);
                            if (statement == null) {
                                throw new IllegalStateException("snapshot statement " + label
                                        + " was not deployed in case " + caseName);
                            }
                            for (Iterator<EventBean> iterator = statement.iterator();
                                 iterator.hasNext(); ) {
                                rows.add(projectedRow(iterator.next(), fields));
                            }
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
                    case "deploy-error": {
                        // env.compile succeeds; only the deploy throws
                        // EPDeployException with the pinned message prefix.
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String expected = string(step, "expectError");
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        try {
                            runtime.getDeploymentService().deploy(compiled);
                            throw new IllegalStateException(
                                    "deploy-error step " + label + " unexpectedly deployed");
                        } catch (EPDeployException ex) {
                            assertMessagePrefix(ex.getMessage(), expected, label);
                        }
                        records.add(errorRecord(caseName, operation, label, sequences, runtime,
                                expected));
                        break;
                    }
                    case "build-error": {
                        // compileWCheckedEx semantics: the compile itself is
                        // rejected and the EPCompileException cause carries
                        // the pinned validation message.
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String expected = string(step, "expectError");
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        try {
                            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                            throw new IllegalStateException(
                                    "build-error step " + label + " unexpectedly compiled");
                        } catch (EPCompileException ex) {
                            Throwable cause = ex.getCause();
                            if (cause == null) {
                                throw new IllegalStateException(
                                        "build-error step " + label + " carries no cause", ex);
                            }
                            assertMessagePrefix(cause.getMessage(), expected, label);
                        }
                        records.add(errorRecord(caseName, operation, label, sequences, runtime,
                                expected));
                        break;
                    }
                    case "faf-error": {
                        // compileExecuteFAF: the query compiles and the
                        // execution throws EPException with the pinned
                        // message prefix.
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String expected = string(step, "expectError");
                        try {
                            executeFaf(runtime, configuration, epl);
                            throw new IllegalStateException(
                                    "faf-error step " + label + " unexpectedly succeeded");
                        } catch (EPException ex) {
                            assertMessagePrefix(ex.getMessage(), expected, label);
                        }
                        records.add(errorRecord(caseName, operation, label, sequences, runtime,
                                expected));
                        break;
                    }
                    case "undeploy": {
                        // undeployModuleContaining: the step label resolves
                        // the deployment that carries the named statement.
                        String label = string(step, "statement");
                        EPDeployment deployment = deployments.remove(label);
                        if (deployment == null) {
                            throw new IllegalStateException("undeploy of unknown deployment "
                                    + label + " in case " + caseName);
                        }
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.remove(statement.getName());
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
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

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * against the runtime path followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration,
                                              String epl) throws EPCompileException,
            EPDeployException {
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

    /** Error record carrying the pinned expectError text, emitted only after
     * the real exception fired with the asserted class and message prefix. */
    private static JsonObject errorRecord(String caseName, String operation, String label,
                                          Map<String, Integer> sequences, EPRuntime runtime,
                                          String expected) {
        int sequence = sequences.merge(label + ":" + operation, 1, Integer::sum);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", operation);
        record.add("statement", label);
        record.add("sequence", sequence);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", expected);
        return record;
    }

    /** SupportMessageAssertUtil.assertMessage semantics: a prefix check for
     * messages longer than ten characters. */
    private static void assertMessagePrefix(String actual, String expected, String label) {
        if (actual == null || !actual.startsWith(expected)) {
            throw new IllegalStateException("error step " + label + " message drift:\nExpected:"
                    + expected + "\nReceived:" + actual);
        }
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
     * Sends one pinned event: SupportBean carries theString/intPrimitive and
     * the optional longPrimitive setter value (ord 4 inserts),
     * SupportBean_S0 carries id/p00/p01 (ord 3-4 renames) and
     * SupportBean_S1 carries id alone (ord 0-1 on-update triggers).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean(
                        string(payload, "theString"),
                        intField(payload.get("intPrimitive"), "intPrimitive"));
                JsonValue longPrimitive = payload.get("longPrimitive");
                if (longPrimitive instanceof JsonNumber) {
                    event.setLongPrimitive(longInteger(longPrimitive, "longPrimitive"));
                }
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(
                        intField(payload.get("id"), "id"),
                        stringOrNull(payload.get("p00")),
                        stringOrNull(payload.get("p01")));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(
                        intField(payload.get("id"), "id"));
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
                case "early-unique-violation":
                    offset = validateEarlyCase(steps, offset, caseName);
                    break;
                case "late-unique-violation":
                    offset = validateLateCase(steps, offset, caseName);
                    break;
                case "faf-update":
                    offset = validateFafCase(steps, offset, caseName);
                    break;
                case "key-update-single":
                    offset = validateKeyUpdateCase(steps, offset, caseName, "s0",
                            EPL_SINGLE_CREATE, EPL_SINGLE_INSERT, EPL_SINGLE_ON_UPDATE,
                            SNAPSHOT_SINGLE_FIELDS);
                    break;
                default:
                    offset = validateKeyUpdateCase(steps, offset, caseName, "s1",
                            EPL_MULTI_CREATE, EPL_MULTI_INSERT, EPL_MULTI_ON_UPDATE,
                            SNAPSHOT_MULTI_FIELDS);
                    break;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "early-unique-violation":
                return String.join("\n", EPL_EARLY_CREATE, EPL_EARLY_INTO,
                        EPL_EARLY_SEC_INDEX, EPL_EARLY_FAF_UPDATE, EPL_EARLY_ON_UPDATE,
                        EPL_EARLY_ON_MERGE);
            case "late-unique-violation":
                return String.join("\n", EPL_LATE_CREATE, EPL_LATE_INTO, EPL_LATE_ON_MERGE,
                        EPL_LATE_SEC_INDEX_COL0, EPL_LATE_ON_UPDATE, EPL_LATE_SEC_INDEX_PKEY1);
            case "faf-update":
                return String.join("\n", EPL_FAF_CREATE, EPL_FAF_INDEX, EPL_FAF_INTO,
                        EPL_FAF_UPDATE_COL0_E1, EPL_FAF_UPDATE_COL0_E2, EPL_FAF_SELECT_COL0,
                        EPL_FAF_UPDATE_COL1_E1, EPL_FAF_SELECT_COL1);
            case "key-update-single":
                return String.join("\n", EPL_SINGLE_CREATE, EPL_SINGLE_INSERT,
                        EPL_SINGLE_ON_UPDATE);
            default:
                return String.join("\n", EPL_MULTI_CREATE, EPL_MULTI_INSERT,
                        EPL_MULTI_ON_UPDATE);
        }
    }

    /**
     * Exact step sequence of InfraEarlyUniqueIndexViolation.run (lines
     * 47-92): the create-table and into-table deploys, the two colliding
     * SupportBean sends, the deploy-failing unique index on pkey0, the
     * atomically-failing FAF update plus unchanged-table snapshot, the
     * on-update deploy and failing S1 send plus snapshot, the
     * compile-rejected on-merge, and undeployAll.
     */
    private static int validateEarlyCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_EARLY_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_EARLY_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateBeanSend(steps.get(offset++), caseName, "E1", 10, null);
        validateBeanSend(steps.get(offset++), caseName, "E1", 20, null);
        validateErrorStep(steps.get(offset++), caseName, "deploy-error", "sec-index-pkey0",
                EPL_EARLY_SEC_INDEX, ERR_EARLY_SEC_INDEX);
        validateErrorStep(steps.get(offset++), caseName, "faf-error", "faf-update-pkey1",
                EPL_EARLY_FAF_UPDATE, ERR_EARLY_FAF_UPDATE);
        validateSnapshot(steps.get(offset++), caseName, "create", "any",
                SNAPSHOT_KEY_FIELDS, null);
        validateDeploy(steps.get(offset++), caseName, "on-update", EPL_EARLY_ON_UPDATE);
        validateDeployed(steps.get(offset++), caseName, "on-update");
        validateSendError(steps.get(offset++), caseName, "on-update-send", "SupportBean_S1",
                ERR_EARLY_ON_UPDATE);
        validateSnapshot(steps.get(offset++), caseName, "create", "any",
                SNAPSHOT_KEY_FIELDS, null);
        validateErrorStep(steps.get(offset++), caseName, "build-error", "on-merge-pkey1",
                EPL_EARLY_ON_MERGE, ERR_ON_MERGE_UNIQUE);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraLateUniqueIndexViolation.run (lines
     * 101-141): the create-table and into-table deploys, two distinct-key
     * sends, the on-merge deploy, the deploy-failing unique index on
     * merge-updated col0, the undeployModuleContaining boundary, the
     * on-update and unique-index-on-pkey1 deploys, the failing S1 send plus
     * unchanged-table snapshot, and the undeploy/undeployAll teardown.
     */
    private static int validateLateCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_LATE_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_LATE_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateBeanSend(steps.get(offset++), caseName, "E1", 10, null);
        validateBeanSend(steps.get(offset++), caseName, "E2", 20, null);
        validateDeploy(steps.get(offset++), caseName, "on-merge", EPL_LATE_ON_MERGE);
        validateDeployed(steps.get(offset++), caseName, "on-merge");
        validateErrorStep(steps.get(offset++), caseName, "deploy-error", "sec-index-col0",
                EPL_LATE_SEC_INDEX_COL0, ERR_LATE_SEC_INDEX_COL0);
        validateUndeploy(steps.get(offset++), caseName, "on-merge");
        validateDeploy(steps.get(offset++), caseName, "on-update", EPL_LATE_ON_UPDATE);
        validateDeployed(steps.get(offset++), caseName, "on-update");
        validateDeploy(steps.get(offset++), caseName, "sec-index-pkey1",
                EPL_LATE_SEC_INDEX_PKEY1);
        validateDeployed(steps.get(offset++), caseName, "sec-index-pkey1");
        validateSendError(steps.get(offset++), caseName, "on-update-send", "SupportBean_S1",
                ERR_LATE_ON_UPDATE);
        validateSnapshot(steps.get(offset++), caseName, "create", "any",
                SNAPSHOT_KEY_FIELDS, null);
        validateUndeploy(steps.get(offset++), caseName, "on-update");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraFAFUpdate.run (lines 149-166): the
     * create-table, create-index and into-table deploys, the two
     * SupportBean sends, the four compileExecuteFAF updates and the two
     * one-row FAF selects (carried as snapshot steps with epl), and
     * undeployAll.
     */
    private static int validateFafCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "create", EPL_FAF_CREATE);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "create-index", EPL_FAF_INDEX);
        validateDeployed(steps.get(offset++), caseName, "create-index");
        validateDeploy(steps.get(offset++), caseName, "into", EPL_FAF_INTO);
        validateDeployed(steps.get(offset++), caseName, "into");
        validateBeanSend(steps.get(offset++), caseName, "E1", 0, null);
        validateBeanSend(steps.get(offset++), caseName, "E2", 0, null);
        validateDeploy(steps.get(offset++), caseName, "FafUpdateCol0E1", EPL_FAF_UPDATE_COL0_E1);
        validateDeploy(steps.get(offset++), caseName, "FafUpdateCol0E2", EPL_FAF_UPDATE_COL0_E2);
        validateSnapshot(steps.get(offset++), caseName, "FafSelectCol0", "ordered",
                SNAPSHOT_FAF_FIELDS, EPL_FAF_SELECT_COL0);
        validateDeploy(steps.get(offset++), caseName, "FafUpdateCol1E1", EPL_FAF_UPDATE_COL1_E1);
        validateSnapshot(steps.get(offset++), caseName, "FafSelectCol1", "ordered",
                SNAPSHOT_FAF_FIELDS, EPL_FAF_SELECT_COL1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraTableKeyUpdateSingleKey.run (lines
     * 209-241) and InfraTableKeyUpdateMultiKey.run (lines 174-205): the
     * create-table, insert-into and on-update deploys, the three SupportBean
     * sends, the milestone(0) no-op, then three rename sends interleaved
     * with iterator any-order snapshots and milestone no-ops, and
     * undeployAll.
     */
    private static int validateKeyUpdateCase(JsonArray steps, int offset, String caseName,
                                             String createLabel, String createEpl,
                                             String insertEpl, String onUpdateEpl,
                                             String[] snapshotFields) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, createLabel, createEpl);
        validateDeployed(steps.get(offset++), caseName, createLabel);
        validateDeploy(steps.get(offset++), caseName, "insert", insertEpl);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "on-update", onUpdateEpl);
        validateDeployed(steps.get(offset++), caseName, "on-update");
        if ("key-update-single".equals(caseName)) {
            validateBeanSend(steps.get(offset++), caseName, "E1", 10, null);
            validateBeanSend(steps.get(offset++), caseName, "E2", 20, null);
            validateBeanSend(steps.get(offset++), caseName, "E3", 30, null);
        } else {
            validateBeanSend(steps.get(offset++), caseName, "E1", 10, 100L);
            validateBeanSend(steps.get(offset++), caseName, "E2", 20, 200L);
            validateBeanSend(steps.get(offset++), caseName, "E3", 30, 300L);
        }
        validateS0Send(steps.get(offset++), caseName, "E2", "E20");
        validateSnapshot(steps.get(offset++), caseName, createLabel, "any", snapshotFields, null);
        validateS0Send(steps.get(offset++), caseName, "E1", "E10");
        validateSnapshot(steps.get(offset++), caseName, createLabel, "any", snapshotFields, null);
        validateS0Send(steps.get(offset++), caseName, "E3", "E30");
        validateSnapshot(steps.get(offset++), caseName, createLabel, "any", snapshotFields, null);
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

    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement, String expectedMode,
                                         String[] expectedFields, String expectedEpl) {
        JsonObject step = object(value, "snapshot step");
        if (expectedEpl == null) {
            requireFields(step, "op", "case", "statement", "mode", "fields");
        } else {
            requireFields(step, "op", "case", "statement", "mode", "fields", "epl");
            if (!expectedEpl.equals(string(step, "epl"))) {
                throw new IllegalArgumentException("snapshot step EPL is not pinned for "
                        + caseName + "/" + expectedStatement);
            }
        }
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

    /** Pins one deploy-error/build-error/faf-error step: op, case, label,
     * byte-exact EPL and the pinned expectError prefix. */
    private static void validateErrorStep(JsonValue value, String caseName, String operation,
                                          String expectedStatement, String expectedEpl,
                                          String expectedError) {
        JsonObject step = object(value, operation + " step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!operation.equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException(operation + " step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
    }

    /** Pins one send-error step: op, case, label, event type, the
     * {"id":0} payload and the pinned expectError prefix. */
    private static void validateSendError(JsonValue value, String caseName,
                                          String expectedStatement, String expectedEventType,
                                          String expectedError) {
        JsonObject step = object(value, "send-error step");
        requireFields(step, "op", "case", "statement", "eventType", "payload", "expectError");
        if (!"send-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEventType.equals(string(step, "eventType"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("send-error step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
        JsonObject payload = object(step.get("payload"), "send-error payload");
        requireFields(payload, "id");
        if (intField(payload.get("id"), "id") != 0) {
            throw new IllegalArgumentException("send-error payload is not pinned for " + caseName);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String expectedString, int expectedInt,
                                         Long expectedLong) {
        JsonObject payload = sendPayload(value, caseName, "SupportBean");
        if (expectedLong == null) {
            requireFields(payload, "theString", "intPrimitive");
        } else {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            if (longInteger(payload.get("longPrimitive"), "longPrimitive") != expectedLong) {
                throw new IllegalArgumentException(
                        "SupportBean longPrimitive is not pinned for " + caseName);
            }
        }
        if (!expectedString.equals(string(payload, "theString"))
                || intField(payload.get("intPrimitive"), "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException(
                    "SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName,
                                       String p00, String p01) {
        JsonObject payload = sendPayload(value, caseName, "SupportBean_S0");
        requireFields(payload, "id", "p00", "p01");
        if (intField(payload.get("id"), "id") != 0
                || !p00.equals(string(payload, "p00"))
                || !p01.equals(string(payload, "p01"))) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 payload is not pinned for " + caseName);
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
