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
 * Java oracle for InfraNWTableOnMerge ordinals 46-53: the six remaining
 * InfraInsertOnly executions and the two InfraDeleteThenUpdate executions.
 * Eight executions on eight runtimes.
 *
 * InfraInsertOnly deploys the InsertOnlyInfra unique-key named window (ord
 * 46) or primary-key table (ords 47-51) and one 'on' merge variant,
 * attaches the listener to 'on', sends SupportBean("E1",1) and
 * SupportBean("E2",2) with an iterator snapshot of 'Window' after each,
 * and ends with undeployAll.  Ord 46 pairs soda with useColumnNames; ord
 * 47 is the useEquivalent where-1=2 form; ords 49 and 51 use the explicit
 * insert(p0, p1) column list; ords 50 and 51 compile through the soda
 * object-model round-trip.  The soda variants pin the same EPL as their
 * non-soda twins; Java's compileDeploy(soda=true, epl, path) asserts the
 * eplToModel toEPL round-trip and is observably identical.  env.milestone(0)
 * and the assertSame(windowType, onType) check are Java-internal and emit
 * no records.
 *
 * InfraDeleteThenUpdate deploys the MyInfra keepall named window (ord 52)
 * or primary-key table (ord 53) and the 'merge' statement — a single
 * matched clause carrying ordered delete then update actions — attaches
 * the listener to 'merge' (mirroring addListener("merge"); the Java
 * execution never asserts it), seeds {A,1} through a fire-and-forget
 * insert (compileExecuteFAFNoResult semantics: compileQuery + executeQuery,
 * no deployment), snapshots 'create', sends SupportBean("A",10), snapshots
 * again and undeploys.  The named-window iterator reads {A,10} (update
 * wins); the table iterator is empty (delete wins).
 */
public final class InfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-insertonly-deletethenupdate";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 46-53: InfraInsertOnly replays the "
                    + "six remaining insert-only variants — ord 46 the "
                    + "named-window soda+useColumnNames form, ords 47-51 the "
                    + "five namedWindow=false table variants (useEquivalent "
                    + "where 1=2, plain, useColumnNames, soda, "
                    + "soda+useColumnNames) — deploying the InsertOnlyInfra "
                    + "unique-key window or primary-key table plus one 'on' "
                    + "merge and observing the 'on' listener rows and Window "
                    + "iterator across two SupportBean sends; "
                    + "InfraDeleteThenUpdate deploys the MyInfra keepall "
                    + "window or primary-key table plus the 'merge' "
                    + "delete-then-update statement, seeds {A,1} via "
                    + "fire-and-forget insert, sends SupportBean(A,10) and "
                    + "observes the divergent outcome — update-wins {A,10} "
                    + "for the named window, delete-wins empty for the "
                    + "table — with the 'merge' listener attached (Java "
                    + "source regression-lib/src/main/java/com/espertech/"
                    + "esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-5cdc46289e4fac78a0c5",
            "java-runtime-8e9616eb8385c473d45a",
            "java-runtime-af614186a63cbeb33ae5",
            "java-runtime-eb7754c9e46c8c465c14",
            "java-runtime-f21a6fc889f14608828f",
            "java-runtime-f7a73c74e857ffbdfd15",
            "java-runtime-5816ec0ef519ec8a48e1",
            "java-runtime-3cca4ced23a6097b5023"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=true, useColumnNames=true}",
            "InfraInsertOnly{namedWindow=false, useEquivalent=true, soda=false, useColumnNames=false}",
            "InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=false, useColumnNames=false}",
            "InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=false, useColumnNames=true}",
            "InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=true, useColumnNames=false}",
            "InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=true, useColumnNames=true}",
            "InfraDeleteThenUpdate{namedWindow=true}",
            "InfraDeleteThenUpdate{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0b6e7bd4eb235001d140",
            "java-0acf62afc326a95d7216",
            "java-0acf62afc326a95d7216"
    };
    private static final String[] CASES = {
            "insertonly-nw-soda-colnames",
            "insertonly-table-equivalent", "insertonly-table",
            "insertonly-table-colnames", "insertonly-table-soda",
            "insertonly-table-soda-colnames",
            "deletethenupdate-nw", "deletethenupdate-table"
    };
    private static final int[] ORDINALS = {46, 47, 48, 49, 50, 51, 52, 53};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+iterator; the insert(p0, p1) column-names insert-only "
                    + "on-merge compiled through the soda object-model "
                    + "round-trip over the InsertOnlyInfra unique-key named "
                    + "window",
            "listener+iterator; on-merge with the equivalent where 1=2 / "
                    + "when not matched then insert form over the "
                    + "InsertOnlyInfra primary-key table",
            "listener+iterator; plain insert-only on-merge (bare merge ... "
                    + "insert select) over the InsertOnlyInfra primary-key "
                    + "table",
            "listener+iterator; insert-only on-merge with an explicit "
                    + "insert(p0, p1) column list over the InsertOnlyInfra "
                    + "primary-key table",
            "listener+iterator; the plain insert-only on-merge compiled "
                    + "through the soda object-model round-trip over the "
                    + "InsertOnlyInfra primary-key table (observably "
                    + "identical EPL to insertonly-table)",
            "listener+iterator; the insert(p0, p1) column-names insert-only "
                    + "on-merge compiled through the soda object-model "
                    + "round-trip over the InsertOnlyInfra primary-key table",
            "listener+iterator; delete-then-update multi-action merge over "
                    + "the MyInfra keepall named window seeded {A,1} by "
                    + "fire-and-forget insert: the update wins and the "
                    + "iterator reads {A,10}",
            "listener+iterator; delete-then-update multi-action merge over "
                    + "the MyInfra primary-key table seeded {A,1} by "
                    + "fire-and-forget insert: the delete wins and the "
                    + "iterator is empty"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 480-494
    // (InfraInsertOnly) and 319-330 (InfraDeleteThenUpdate).
    private static final String EPL_INSERTONLY_CREATE_NW =
            "@Name('Window') @public create window InsertOnlyInfra#unique(p0) as "
                    + "(p0 string, p1 int)";
    private static final String EPL_INSERTONLY_CREATE_TABLE =
            "@Name('Window') @public create table InsertOnlyInfra (p0 string primary key, p1 int)";
    private static final String EPL_INSERTONLY_EQUIVALENT =
            "@name('on') on SupportBean merge InsertOnlyInfra where 1=2 when not "
                    + "matched then insert select theString as p0, intPrimitive as p1";
    private static final String EPL_INSERTONLY_PLAIN =
            "@name('on') on SupportBean merge InsertOnlyInfra insert select "
                    + "theString as p0, intPrimitive as p1";
    private static final String EPL_INSERTONLY_COLNAMES =
            "@name('on') on SupportBean as provider merge InsertOnlyInfra "
                    + "insert(p0, p1) select provider.theString, intPrimitive";

    private static final String EPL_DTU_CREATE_NW =
            "@name('create') @public create window MyInfra#keepall() as (p0 string, p1 int)";
    private static final String EPL_DTU_CREATE_TABLE =
            "@name('create') @public create table MyInfra(p0 string primary key, p1 int)";
    private static final String EPL_DTU_MERGE =
            "@name('merge') on SupportBean sb merge MyInfra where theString = p0 "
                    + "when matched then delete then update set p1 = intPrimitive";
    private static final String EPL_DTU_FAF_INSERT =
            "insert into MyInfra select 'A' as p0, 1 as p1";

    private static final String FAF_INSERT_LABEL = "FafInsert";
    private static final String[] SNAPSHOT_FIELDS = {"p0", "p1"};

    private static final int EXPECTED_STEPS = 80;
    private static final int EXPECTED_RECORDS = 46;

    private InfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenarioOracle <scenario.json>");
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
     * its own runtime).  SupportBean is a preconfigured event type; the
     * internal timer is disabled and the rethrowing exception handler
     * surfaces statement failures to the sender thread.  Every deploy
     * registers its statement under the step label so deployed markers and
     * snapshots can resolve it; the FafInsert label is a fire-and-forget
     * insert (compileExecuteFAFNoResult semantics) and registers nothing.
     * The recording listener attaches to the statements the Java execution
     * listens on: 'on' for the insert-only cases (env.addListener("on"))
     * and 'merge' for the delete-then-update cases
     * (compileDeploy(...).addListener("merge")).
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

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
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("deployment of " + caseName
                                    + "/" + label + " has " + deployed.length
                                    + " statements, want 1");
                        }
                        EPStatement statement = deployed[0];
                        if ("on".equals(statement.getName())
                                || "merge".equals(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                        statements.put(label, statement);
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
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * This mirrors the 'on' listener assertions of InfraInsertOnly (the
     * merge delivers each inserted row to the merge statement's listener)
     * and the attached-but-unasserted 'merge' listener of
     * InfraDeleteThenUpdate.
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
        for (String caseName : CASES) {
            if (isInsertOnlyCase(caseName)) {
                offset = validateInsertOnlyCase(steps, offset, caseName);
            } else {
                offset = validateDeleteThenUpdateCase(steps, offset, caseName);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static boolean isInsertOnlyCase(String caseName) {
        return caseName.startsWith("insertonly-");
    }

    private static boolean isTableCase(String caseName) {
        return caseName.contains("-table");
    }

    /** The pinned cases[] epl: the first deploy EPL of each case. */
    private static String caseEpl(String caseName) {
        if (isInsertOnlyCase(caseName)) {
            return insertOnlyCreateEpl(caseName);
        }
        return isTableCase(caseName) ? EPL_DTU_CREATE_TABLE : EPL_DTU_CREATE_NW;
    }

    /** The pinned create EPL of each insert-only case (lines 480-482). */
    private static String insertOnlyCreateEpl(String caseName) {
        return isTableCase(caseName) ? EPL_INSERTONLY_CREATE_TABLE : EPL_INSERTONLY_CREATE_NW;
    }

    /** The pinned 'on' merge EPL of each insert-only case (lines 485-492);
     * the soda variants share their non-soda twin's EPL. */
    private static String insertOnlyMergeEpl(String caseName) {
        switch (caseName) {
            case "insertonly-table-equivalent":
                return EPL_INSERTONLY_EQUIVALENT;
            case "insertonly-nw-soda-colnames":
            case "insertonly-table-colnames":
            case "insertonly-table-soda-colnames":
                return EPL_INSERTONLY_COLNAMES;
            default:
                // insertonly-table and insertonly-table-soda share the
                // plain EPL; soda only changes the Java compile path.
                return EPL_INSERTONLY_PLAIN;
        }
    }

    /**
     * Exact step sequence of InfraInsertOnly.run (lines 477-517): the
     * Window and 'on' deploys with deployed markers, the E1 send and
     * snapshot, the E2 send and snapshot across the milestone(0) no-op,
     * and undeployAll.
     */
    private static int validateInsertOnlyCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "Window",
                insertOnlyCreateEpl(caseName));
        offset = validateDeployPair(steps, offset, caseName, "on", insertOnlyMergeEpl(caseName));
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", SNAPSHOT_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", SNAPSHOT_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraDeleteThenUpdate.run (lines 311-342): the
     * 'create' and 'merge' deploys with deployed markers, the
     * fire-and-forget seed insert (no marker), the ordered {A,1} snapshot,
     * the SupportBean("A",10) send, the divergent second snapshot and
     * undeployAll.
     */
    private static int validateDeleteThenUpdateCase(JsonArray steps, int offset,
                                                    String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create",
                isTableCase(caseName) ? EPL_DTU_CREATE_TABLE : EPL_DTU_CREATE_NW);
        offset = validateDeployPair(steps, offset, caseName, "merge", EPL_DTU_MERGE);
        validateDeploy(steps.get(offset++), caseName, FAF_INSERT_LABEL, EPL_DTU_FAF_INSERT);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", SNAPSHOT_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "A", 10);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", SNAPSHOT_FIELDS);
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
