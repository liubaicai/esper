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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
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
 * Java oracle for InfraNWTableOnMerge ordinals 40-45: the InfraInvalid
 * tryInvalidCompile probe sequences over the named-window and table
 * fixtures, and the four InfraInsertOnly{namedWindow=true} variants
 * (useEquivalent, plain, useColumnNames, soda).  Six executions on six
 * runtimes.
 *
 * InfraInvalid deploys one multi-statement fixture module (the MergeInfra
 * unique-key named window or keyless table, the ABCSchema type and the
 * ABCInfra keepall window or keyless table) and then compiles the pinned
 * probe EPLs with the runtime path, mirroring
 * env.tryInvalidCompile(path, epl, prefix): each probe must fail to
 * compile and the caught message must start with the pinned expectError
 * prefix (SupportMessageAssertUtil.assertMessage semantics).  The
 * named-window variant runs thirteen probes; the table variant runs
 * twelve (the matched-insert event-type-conversion probe is named-window
 * only) and pins a different message for the unknown insert column probe.
 * After undeployAll and a cleared path, four fixture deploys (Composite
 * map schema, AInfra keepall window, SomeOther map schema, MyEvent map
 * schema) precede the final on-update nested event-type assignment probe.
 * Deploy steps emit no records, mirroring the context-key-segmented-invalid
 * precedent: the Java execution observes only the compile failures.
 *
 * InfraInsertOnly deploys the InsertOnlyInfra unique-key named window
 * ('Window') and one 'on' merge variant, attaches the listener to 'on',
 * sends SupportBean("E1",1) and SupportBean("E2",2) with an iterator
 * snapshot of 'Window' after each, and ends with undeployAll.  The soda
 * variant (ord 45) pins the same EPL as the plain variant; Java's
 * compileDeploy(soda=true, epl, path) asserts the eplToModel toEPL
 * round-trip and is observably identical.  env.milestone(0) and the
 * assertSame(windowType, onType) check are Java-internal and emit no
 * records.
 */
public final class InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-invalid-insertonly";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 40-45: InfraInvalid replays the "
                    + "tryInvalidCompile probe sequence over the "
                    + "MergeInfra/ABCInfra named-window and table fixtures, "
                    + "pinning the Java compile-error prefixes (the nw "
                    + "variant carries one extra probe, the matched-insert "
                    + "event-type conversion; probe 2 pins divergent "
                    + "nw/table wording); InfraInsertOnly deploys the "
                    + "InsertOnlyInfra unique-key named window and one of "
                    + "four insert-only on-merge variants (useEquivalent "
                    + "where 1=2, plain, useColumnNames, soda) and observes "
                    + "the 'on' listener rows and window iterator across "
                    + "two SupportBean sends (Java source regression-lib/"
                    + "src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-99b2d413519187b56232",
            "java-runtime-33b1e837bc8b373bb198",
            "java-runtime-651148621e89ec5465b0",
            "java-runtime-e51caa89fbde4ab34197",
            "java-runtime-8900ac7e3d063d8b8842",
            "java-runtime-b581753558f219b16a2a"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraInvalid{namedWindow=true}",
            "InfraInvalid{namedWindow=false}",
            "InfraInsertOnly{namedWindow=true, useEquivalent=true, soda=false, useColumnNames=false}",
            "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=false}",
            "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=true}",
            "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=true, useColumnNames=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-42d23ac20541998f0a55",
            "java-42d23ac20541998f0a55",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140"
    };
    private static final String[] CASES = {
            "invalid-nw", "invalid-table",
            "insertonly-nw-equivalent", "insertonly-nw",
            "insertonly-nw-colnames", "insertonly-nw-soda"
    };
    private static final int[] ORDINALS = {40, 41, 42, 43, 44, 45};
    private static final String[] CASE_OBSERVATIONS = {
            "compile-error; thirteen probes record the pinned Java message "
                    + "prefixes over the MergeInfra unique-key named window "
                    + "and ABCInfra keepall window fixtures, then the "
                    + "Composite/AInfra/SomeOther/MyEvent fixture rejects "
                    + "the nested event-type assignment",
            "compile-error; twelve probes record the pinned Java message "
                    + "prefixes over the MergeInfra and ABCInfra keyless "
                    + "tables (the nw-only matched-insert probe is absent "
                    + "and probe 2 pins the column-assignment wording), "
                    + "then the Composite/AInfra/SomeOther/MyEvent fixture "
                    + "rejects the nested event-type assignment",
            "listener+iterator; on-merge with the equivalent where 1=2 / "
                    + "when not matched then insert form over the "
                    + "InsertOnlyInfra unique-key named window",
            "listener+iterator; plain insert-only on-merge (bare merge ... "
                    + "insert select) over the InsertOnlyInfra unique-key "
                    + "named window",
            "listener+iterator; insert-only on-merge with an explicit "
                    + "insert(p0, p1) column list over the InsertOnlyInfra "
                    + "unique-key named window",
            "listener+iterator; the plain insert-only on-merge compiled "
                    + "through the soda object-model round-trip over the "
                    + "InsertOnlyInfra unique-key named window "
                    + "(observably identical EPL to insertonly-nw)"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 823-885
    // (InfraInvalid) and 480-494 (InfraInsertOnly).  The fixture modules
    // and the variable/indexed probes carry literal newlines exactly as
    // the Java string concatenation produces them.
    private static final String EPL_INVALID_FIXTURE_NW =
            "@public create window MergeInfra#unique(theString) as SupportBean;\n"
                    + "create schema ABCSchema as (val int);\n"
                    + "@public create window ABCInfra#keepall as ABCSchema;\n";
    private static final String EPL_INVALID_FIXTURE_TABLE =
            "@public create table MergeInfra as (theString string, intPrimitive int, "
                    + "boolPrimitive bool);\n"
                    + "create schema ABCSchema as (val int);\n"
                    + "@public create table ABCInfra (val int);\n";
    private static final String EPL_INVALID_COMPOSITE =
            "@public create map schema Composite as (c0 int)";
    private static final String EPL_INVALID_AINFRA =
            "@public create window AInfra#keepall as (c Composite)";
    private static final String EPL_INVALID_SOMEOTHER =
            "@public create map schema SomeOther as (c1 int)";
    private static final String EPL_INVALID_MYEVENT =
            "@public create map schema MyEvent as (so SomeOther)";

    private static final String EPL_PROBE_FILTER_WINDOWEVENT =
            "on SupportBean_A merge MergeInfra as windowevent where id = theString "
                    + "when not matched and exists(select * from MergeInfra mw where "
                    + "mw.theString = windowevent.theString) is not null then insert "
                    + "into ABC select '1'";
    private static final String EPL_PROBE_UNKNOWN_COLUMN =
            "on SupportBean_A as up merge ABCInfra as mv when not matched then "
                    + "insert (col) select 1";
    private static final String EPL_PROBE_NOTMATCHED_UPDATE =
            "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true "
                    + "when not matched then update set intPrimitive = 1";
    private static final String EPL_PROBE_MATCHED_INSERT =
            "on SupportBean_A as up merge MergeInfra as mv where mv.theString=id "
                    + "when matched then insert select *";
    private static final String EPL_PROBE_MISSING_CLAUSES =
            "on SupportBean as up merge MergeInfra as mv";
    private static final String EPL_PROBE_MISSING_THEN =
            "on SupportBean as up merge MergeInfra as mv where a=b when matched";
    private static final String EPL_PROBE_AND_THEN_DELETE =
            "on SupportBean as up merge MergeInfra as mv where a=b when matched "
                    + "and then delete";
    private static final String EPL_PROBE_AMBIGUOUS_WHERE =
            "on SupportBean as up merge MergeInfra as mv where boolPrimitive=true "
                    + "when not matched then insert select *";
    private static final String EPL_PROBE_INVALID_SELECT =
            "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true "
                    + "when not matched then insert select intPrimitive";
    private static final String EPL_PROBE_MATCH_WHERE =
            "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true "
                    + "when not matched then insert select * where theString = 'A'";
    private static final String EPL_PROBE_VARIABLE_LHS =
            "@public create variable int myvariable;\n"
                    + "on SupportBean_A merge MergeInfra when matched then update set "
                    + "myvariable = 1;\n";
    private static final String EPL_PROBE_INDEXED_LHS =
            "on SupportBean_A merge MergeInfra when matched then update set "
                    + "theString[1][2] = 1;\n";
    private static final String EPL_PROBE_NESTED_ASSIGN =
            "on MyEvent as me update AInfra set c = me.so";

    private static final String ERR_FILTER_WINDOWEVENT =
            "On-Merge not-matched filter expression may not use properties that are "
                    + "provided by the named window event [" + EPL_PROBE_FILTER_WINDOWEVENT + "]";
    private static final String ERR_UNKNOWN_COLUMN_NW =
            "Validation failed in when-not-matched (clause 1): Event type named "
                    + "'ABCInfra' has already been declared with differing column name "
                    + "or type information: Type by name 'ABCInfra' in property 'col' "
                    + "property name not found in target";
    private static final String ERR_UNKNOWN_COLUMN_TABLE =
            "Validation failed in when-not-matched (clause 1): Column 'col' could not "
                    + "be assigned to any of the properties of the underlying type "
                    + "(missing column names, event property, setter method or "
                    + "constructor?) [";
    private static final String ERR_NOTMATCHED_UPDATE =
            "Incorrect syntax near 'update' (a reserved keyword) expecting 'insert' "
                    + "but found 'update' at line 1 column 9";
    private static final String ERR_MATCHED_INSERT_NW =
            "Validation failed in when-not-matched (clause 1): Expression-returned "
                    + "event type 'SupportBean_A' with underlying type 'com.espertech."
                    + "esper.regressionlib.support.bean.SupportBean_A' cannot be "
                    + "converted to target event type 'MergeInfra' with underlying "
                    + "type 'com.espertech.esper.common.internal.support.SupportBean' "
                    + "[" + EPL_PROBE_MATCHED_INSERT + "]";
    private static final String ERR_MISSING_CLAUSES =
            "Unexpected end-of-input at line 1 column 4";
    private static final String ERR_MISSING_THEN =
            "Incorrect syntax near end-of-input ('matched' is a reserved keyword) "
                    + "expecting 'then' but found end-of-input at line 1 column 66 [";
    private static final String ERR_AND_THEN_DELETE =
            "Incorrect syntax near 'then' (a reserved keyword) at line 1 column 71 "
                    + "[" + EPL_PROBE_AND_THEN_DELETE + "]";
    private static final String ERR_AMBIGUOUS_WHERE =
            "Failed to validate where-clause expression 'boolPrimitive=true': "
                    + "Property named 'boolPrimitive' is ambiguous as is valid for "
                    + "more then one stream [" + EPL_PROBE_AMBIGUOUS_WHERE + "]";
    private static final String ERR_INVALID_SELECT =
            "Failed to validate select-clause expression 'intPrimitive': Property "
                    + "named 'intPrimitive' is not valid in any stream ["
                    + EPL_PROBE_INVALID_SELECT + "]";
    private static final String ERR_MATCH_WHERE =
            "Failed to validate match where-clause expression 'theString=\"A\"': "
                    + "Property named 'theString' is not valid in any stream ["
                    + EPL_PROBE_MATCH_WHERE + "]";
    private static final String ERR_VARIABLE_LHS =
            "Left-hand-side does not allow variables for variable 'myvariable'";
    private static final String ERR_INDEXED_LHS =
            "Unrecognized left-hand-side assignment 'theString[1][2]'";
    private static final String ERR_NESTED_ASSIGN =
            "Failed to validate assignment expression 'c=me.so': Invalid assignment "
                    + "to property 'c' event type 'Composite' from event type "
                    + "'SomeOther' [" + EPL_PROBE_NESTED_ASSIGN + "]";

    private static final String[] INVALID_PROBE_LABELS_NW = {
            "notmatched-filter-windowevent", "insert-unknown-column",
            "notmatched-update-action", "matched-insert-wrong-type",
            "missing-clauses", "missing-then", "and-then-delete",
            "ambiguous-where-prop", "invalid-select-prop",
            "invalid-match-where-prop", "variable-lhs", "indexed-lhs"
    };
    private static final String[] INVALID_PROBE_LABELS_TABLE = {
            "notmatched-filter-windowevent", "insert-unknown-column",
            "notmatched-update-action", "missing-clauses", "missing-then",
            "and-then-delete", "ambiguous-where-prop", "invalid-select-prop",
            "invalid-match-where-prop", "variable-lhs", "indexed-lhs"
    };
    private static final String[] INVALID_PROBE_EPLS_NW = {
            EPL_PROBE_FILTER_WINDOWEVENT, EPL_PROBE_UNKNOWN_COLUMN,
            EPL_PROBE_NOTMATCHED_UPDATE, EPL_PROBE_MATCHED_INSERT,
            EPL_PROBE_MISSING_CLAUSES, EPL_PROBE_MISSING_THEN,
            EPL_PROBE_AND_THEN_DELETE, EPL_PROBE_AMBIGUOUS_WHERE,
            EPL_PROBE_INVALID_SELECT, EPL_PROBE_MATCH_WHERE,
            EPL_PROBE_VARIABLE_LHS, EPL_PROBE_INDEXED_LHS
    };
    private static final String[] INVALID_PROBE_EPLS_TABLE = {
            EPL_PROBE_FILTER_WINDOWEVENT, EPL_PROBE_UNKNOWN_COLUMN,
            EPL_PROBE_NOTMATCHED_UPDATE, EPL_PROBE_MISSING_CLAUSES,
            EPL_PROBE_MISSING_THEN, EPL_PROBE_AND_THEN_DELETE,
            EPL_PROBE_AMBIGUOUS_WHERE, EPL_PROBE_INVALID_SELECT,
            EPL_PROBE_MATCH_WHERE, EPL_PROBE_VARIABLE_LHS, EPL_PROBE_INDEXED_LHS
    };
    private static final String[] INVALID_PROBE_ERRORS_NW = {
            ERR_FILTER_WINDOWEVENT, ERR_UNKNOWN_COLUMN_NW, ERR_NOTMATCHED_UPDATE,
            ERR_MATCHED_INSERT_NW, ERR_MISSING_CLAUSES, ERR_MISSING_THEN,
            ERR_AND_THEN_DELETE, ERR_AMBIGUOUS_WHERE, ERR_INVALID_SELECT,
            ERR_MATCH_WHERE, ERR_VARIABLE_LHS, ERR_INDEXED_LHS
    };
    private static final String[] INVALID_PROBE_ERRORS_TABLE = {
            ERR_FILTER_WINDOWEVENT, ERR_UNKNOWN_COLUMN_TABLE, ERR_NOTMATCHED_UPDATE,
            ERR_MISSING_CLAUSES, ERR_MISSING_THEN, ERR_AND_THEN_DELETE,
            ERR_AMBIGUOUS_WHERE, ERR_INVALID_SELECT, ERR_MATCH_WHERE,
            ERR_VARIABLE_LHS, ERR_INDEXED_LHS
    };

    private static final String EPL_INSERTONLY_CREATE =
            "@Name('Window') @public create window InsertOnlyInfra#unique(p0) as "
                    + "(p0 string, p1 int)";
    private static final String EPL_INSERTONLY_EQUIVALENT =
            "@name('on') on SupportBean merge InsertOnlyInfra where 1=2 when not "
                    + "matched then insert select theString as p0, intPrimitive as p1";
    private static final String EPL_INSERTONLY_PLAIN =
            "@name('on') on SupportBean merge InsertOnlyInfra insert select "
                    + "theString as p0, intPrimitive as p1";
    private static final String EPL_INSERTONLY_COLNAMES =
            "@name('on') on SupportBean as provider merge InsertOnlyInfra "
                    + "insert(p0, p1) select provider.theString, intPrimitive";

    private static final String[] INSERTONLY_FIELDS = {"p0", "p1"};

    private static final int EXPECTED_STEPS = 81;
    private static final int EXPECTED_RECORDS = 49;

    private InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle <scenario.json>");
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
     * its own runtime).  SupportBean and SupportBean_A are preconfigured
     * event types; internal timer is disabled and the rethrowing exception
     * handler surfaces statement failures to the sender thread.  Invalid
     * cases compile and deploy their fixture modules without registering
     * statement labels (the execution never looks statements up) and emit
     * one compile-error record per probe.  Insert-only cases register the
     * 'Window' and 'on' statements so deployed markers and snapshots can
     * resolve them, and attach the listener to 'on', mirroring
     * env.addListener("on").
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        boolean insertOnly = caseName.startsWith("insertonly-");
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
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
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        if (insertOnly) {
                            EPStatement[] deployed = deployment.getStatements();
                            if (deployed.length != 1) {
                                throw new IllegalStateException("deployment of " + caseName
                                        + "/" + label + " has " + deployed.length
                                        + " statements, want 1");
                            }
                            EPStatement statement = deployed[0];
                            if ("on".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                            statements.put(label, statement);
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statements.containsKey(label)) {
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
                    case "build-error":
                        buildErrorStep(runtime, caseName, step, records);
                        break;
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
     * Compiles an expected-invalid probe with the runtime path, mirroring
     * env.tryInvalidCompile(path, epl, prefix): the caught message must
     * start with the pinned expectError prefix
     * (SupportMessageAssertUtil.assertMessage semantics for messages
     * longer than ten characters) before the compile-error record carries
     * the pinned prefix verbatim.
     */
    private static void buildErrorStep(EPRuntime runtime, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs =
                    new CompilerArguments(runtime.getRuntimePath());
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException("build-error probe " + caseName + "/" + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + caseName
                    + "/" + label + ": expected prefix [" + expected + "] got ["
                    + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * This mirrors the 'on' listener assertions of InfraInsertOnly: the
     * merge delivers each inserted row to the merge statement's listener.
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

    /**
     * Sends one pinned event: SupportBean carries theString and
     * intPrimitive (sendEventBean(new SupportBean(theString, intPrimitive))).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        if ("SupportBean".equals(type)) {
            JsonValue theString = payload.get("theString");
            SupportBean bean = new SupportBean(
                    theString == null || theString.isNull() ? null : theString.asString(),
                    (int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
            runtime.getEventService().sendEventBean(bean, type);
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
        offset = validateInvalidCase(steps, offset, "invalid-nw", false);
        offset = validateInvalidCase(steps, offset, "invalid-table", true);
        for (String caseName : CASES) {
            if (!caseName.startsWith("insertonly-")) {
                continue;
            }
            offset = validateInsertOnlyCase(steps, offset, caseName);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the first deploy EPL of each case. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "invalid-nw":
                return EPL_INVALID_FIXTURE_NW;
            case "invalid-table":
                return EPL_INVALID_FIXTURE_TABLE;
            default:
                return EPL_INSERTONLY_CREATE;
        }
    }

    /** The pinned 'on' merge EPL of each insert-only case (lines 485-492). */
    private static String insertOnlyMergeEpl(String caseName) {
        switch (caseName) {
            case "insertonly-nw-equivalent":
                return EPL_INSERTONLY_EQUIVALENT;
            case "insertonly-nw-colnames":
                return EPL_INSERTONLY_COLNAMES;
            default:
                // insertonly-nw and insertonly-nw-soda share the plain EPL;
                // soda only changes the Java compile path.
                return EPL_INSERTONLY_PLAIN;
        }
    }

    /**
     * Exact step sequence of InfraInvalid.run (lines 821-888): the
     * multi-statement fixture deploy, the twelve or thirteen
     * tryInvalidCompile probes (the matched-insert probe is named-window
     * only), undeployAll, the four block-2 fixture deploys, the nested
     * event-type assignment probe and the final undeployAll.
     */
    private static int validateInvalidCase(JsonArray steps, int offset, String caseName,
                                           boolean isTable) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "fixture",
                isTable ? EPL_INVALID_FIXTURE_TABLE : EPL_INVALID_FIXTURE_NW);
        String[] labels = isTable ? INVALID_PROBE_LABELS_TABLE : INVALID_PROBE_LABELS_NW;
        String[] epls = isTable ? INVALID_PROBE_EPLS_TABLE : INVALID_PROBE_EPLS_NW;
        String[] errors = isTable ? INVALID_PROBE_ERRORS_TABLE : INVALID_PROBE_ERRORS_NW;
        for (int index = 0; index < labels.length; index++) {
            validateBuildError(steps.get(offset++), caseName, labels[index], epls[index],
                    errors[index]);
        }
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "composite-schema", EPL_INVALID_COMPOSITE);
        validateDeploy(steps.get(offset++), caseName, "ainfra-window", EPL_INVALID_AINFRA);
        validateDeploy(steps.get(offset++), caseName, "someother-schema", EPL_INVALID_SOMEOTHER);
        validateDeploy(steps.get(offset++), caseName, "myevent-schema", EPL_INVALID_MYEVENT);
        validateBuildError(steps.get(offset++), caseName, "nested-event-assign",
                EPL_PROBE_NESTED_ASSIGN, ERR_NESTED_ASSIGN);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraInsertOnly.run (lines 477-517): the
     * Window and 'on' deploys with deployed markers, the E1 send and
     * snapshot, the E2 send and snapshot across the milestone(0) no-op,
     * and undeployAll.
     */
    private static int validateInsertOnlyCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "Window", EPL_INSERTONLY_CREATE);
        offset = validateDeployPair(steps, offset, caseName, "on", insertOnlyMergeEpl(caseName));
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", INSERTONLY_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", INSERTONLY_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static int validateDeployPair(JsonArray steps, int offset, String caseName,
                                          String statement, String epl) {
        validateDeploy(steps.get(offset++), caseName, statement, epl);
        validateDeployed(steps.get(offset++), caseName, statement);
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

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String expectedString, long expectedIntPrimitive) {
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
