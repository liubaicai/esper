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
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableOnMerge ordinals 26-31: the pattern-multimatch,
 * no-where-clause and multiple-insert merge executions.  Each of the three
 * execution classes (InfraPatternMultimatch, InfraNoWhereClause,
 * InfraMultipleInsert) runs twice — namedWindow=true first, then
 * namedWindow=false — giving six executions on six runtimes.
 *
 * InfraPatternMultimatch deploys the MyInfraPM keepall named window or
 * composite-primary-key table in one compileDeploy and the 'Merge' on-pattern
 * merge in a second.  `every a` spawns one waiting branch per A-event, so one
 * matching B-event completes every pending branch and fires one merge per
 * match; the composite-key where-clause makes re-matches no-ops.  No listener
 * is attached and the merge EPL ends with a trailing space and no semicolon.
 *
 * InfraNoWhereClause deploys one five-statement module (the MyEvent and
 * MySchema create-schemas, the MyInfraNWC keepall window or UNKEYED table,
 * the SupportBean_A delete-all trigger and the merge).  A merge without a
 * where-clause treats every existing target row as matched, so the
 * not-matched insert branches fire only while the target is empty and the
 * 'C%' matched update rewrites all existing rows.
 *
 * InfraMultipleInsert deploys one four-statement module (the two schemas, the
 * MyInfraMI keepall window or primary-key table and the 'Merge' on-merge) and
 * listens on 'Merge': four ordered when-not-matched insert clauses fire on
 * the first matching condition and deliver each inserted row to the merge
 * statement's own listener as new data.
 *
 * milestone(0/1/2) calls are HA recovery checkpoints that emit no steps; the
 * scenario drops them.  Each execution ends with undeployAll.
 */
public final class InfraNWTableOnMergePatternNoWhereScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-pattern-nowhere";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 26-31: InfraPatternMultimatch merges every "
                    + "completed every-A-then-B pattern match into the MyInfraPM keepall "
                    + "named window or composite-primary-key table under a composite-key "
                    + "where-clause so re-matches are no-ops; InfraNoWhereClause merges "
                    + "without a where-clause over the MyInfraNWC keepall named window or "
                    + "unkeyed table so every existing row is matched and the not-matched "
                    + "insert branches fire only while the target is empty; "
                    + "InfraMultipleInsert evaluates four ordered when-not-matched insert "
                    + "clauses over the MyInfraMI keepall named window or primary-key "
                    + "table and delivers each inserted row to the 'Merge' listener; each "
                    + "execution runs over a named window and a table (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/infra/nwtable/InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-d28146beedb9cfdf9dbb",
            "java-runtime-ac7258306c8be9b66f8f",
            "java-runtime-0dc6b8eb05a1d3989b23",
            "java-runtime-c6f6bda40f34d29acb8c",
            "java-runtime-2c4851ea026cc835d39b",
            "java-runtime-f075aa28d64b1ae78d2e"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraPatternMultimatch{namedWindow=true}",
            "InfraPatternMultimatch{namedWindow=false}",
            "InfraNoWhereClause{namedWindow=true}",
            "InfraNoWhereClause{namedWindow=false}",
            "InfraMultipleInsert{namedWindow=true}",
            "InfraMultipleInsert{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-74cbcb4f6ad520a0f3fd",
            "java-74cbcb4f6ad520a0f3fd",
            "java-2b5b9c17389359bc78f7",
            "java-2b5b9c17389359bc78f7",
            "java-227e4a044ce4b9c1de81",
            "java-227e4a044ce4b9c1de81"
    };
    private static final String[] CASES = {
            "patternmultimatch-nw", "patternmultimatch-table",
            "nowhere-nw", "nowhere-table",
            "multipleinsert-nw", "multipleinsert-table"
    };
    private static final int[] ORDINALS = {26, 27, 28, 29, 30, 31};
    private static final String[] CASE_OBSERVATIONS = {
            "iterator; every-A-then-B pattern merge into the MyInfraPM keepall named "
                    + "window: one B event completes every pending A branch and the "
                    + "composite-key where-clause makes re-matches no-ops",
            "iterator; same every-A-then-B pattern merge into the MyInfraPM "
                    + "composite-primary-key table",
            "iterator; merge without a where-clause over the MyInfraNWC keepall named "
                    + "window: every existing row is matched, the not-matched insert "
                    + "branches fire only while the target is empty and the 'C%' matched "
                    + "update rewrites all rows",
            "iterator; same no-where merge over the MyInfraNWC unkeyed table",
            "listener; four ordered when-not-matched insert clauses over the MyInfraMI "
                    + "keepall named window deliver each inserted row to the 'Merge' "
                    + "listener as new data",
            "listener; same four ordered when-not-matched insert clauses over the "
                    + "MyInfraMI primary-key table"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 1002-1011
    // (InfraPatternMultimatch), 756-771 (InfraNoWhereClause) and 695-710
    // (InfraMultipleInsert).  The pattern merge ends with a trailing space
    // and no semicolon; the module deploys end with ";\n".
    private static final String EPL_CREATE_PM_NW =
            "@name('create') @public create window MyInfraPM#keepall as (c1 string, c2 string)";
    private static final String EPL_CREATE_PM_TABLE =
            "@name('create') @public create table MyInfraPM as (c1 string primary key, "
                    + "c2 string primary key)";
    private static final String EPL_MERGE_PM =
            "@name('Merge') on pattern[every a=SupportBean(theString like 'A%') -> "
                    + "b=SupportBean(theString like 'B%', intPrimitive = a.intPrimitive)] me "
                    + "merge MyInfraPM mw "
                    + "where me.a.theString = mw.c1 and me.b.theString = mw.c2 "
                    + "when not matched then "
                    + "insert select me.a.theString as c1, me.b.theString as c2 ";

    private static final String EPL_SCHEMA_HEAD =
            "@public @buseventtype create schema MyEvent as (in1 string, in2 int);\n"
                    + "create schema MySchema as (col1 string, col2 int);\n";
    private static final String EPL_NWC_NW =
            "@name('create') @public create window MyInfraNWC#keepall as MySchema;\n";
    private static final String EPL_NWC_TABLE =
            "@name('create') @public create table MyInfraNWC (col1 string, col2 int);\n";
    private static final String EPL_NWC_TAIL =
            "on SupportBean_A delete from MyInfraNWC;\n"
                    + "on MyEvent me merge MyInfraNWC mw "
                    + "when not matched and me.in1 like \"A%\" then "
                    + "insert(col1, col2) select me.in1, me.in2 "
                    + "when not matched and me.in1 like \"B%\" then "
                    + "insert select me.in1 as col1, me.in2 as col2 "
                    + "when matched and me.in1 like \"C%\" then "
                    + "update set col1='Z', col2=-1 "
                    + "when not matched then "
                    + "insert select \"x\" || me.in1 || \"x\" as col1, me.in2 * -1 as col2;\n";
    private static final String EPL_MODULE_NWC_NW = EPL_SCHEMA_HEAD + EPL_NWC_NW + EPL_NWC_TAIL;
    private static final String EPL_MODULE_NWC_TABLE =
            EPL_SCHEMA_HEAD + EPL_NWC_TABLE + EPL_NWC_TAIL;

    private static final String EPL_MI_NW =
            "@public create window MyInfraMI#keepall as MySchema;\n";
    private static final String EPL_MI_TABLE =
            "@public create table MyInfraMI (col1 string primary key, col2 int);\n";
    private static final String EPL_MI_TAIL =
            "@name('Merge') on MyEvent merge MyInfraMI "
                    + "where col1=in1 "
                    + "when not matched and in1 like \"A%\" then "
                    + "insert(col1, col2) select in1, in2 "
                    + "when not matched and in1 like \"B%\" then "
                    + "insert select in1 as col1, in2 as col2 "
                    + "when not matched and in1 like \"C%\" then "
                    + "insert select \"Z\" as col1, -1 as col2 "
                    + "when not matched and in1 like \"D%\" then "
                    + "insert select \"x\"||in1||\"x\" as col1, in2*-1 as col2;\n";
    private static final String EPL_MODULE_MI_NW = EPL_SCHEMA_HEAD + EPL_MI_NW + EPL_MI_TAIL;
    private static final String EPL_MODULE_MI_TABLE =
            EPL_SCHEMA_HEAD + EPL_MI_TABLE + EPL_MI_TAIL;

    private static final String[] CASE_EPLS = {
            EPL_CREATE_PM_NW, EPL_CREATE_PM_TABLE,
            EPL_MODULE_NWC_NW, EPL_MODULE_NWC_TABLE,
            EPL_MODULE_MI_NW, EPL_MODULE_MI_TABLE
    };

    // Module statement labels in EPL order for the single compileDeploy of
    // InfraNoWhereClause and InfraMultipleInsert; deployment.getStatements()
    // binds positionally.
    private static final String[] MODULE_LABELS_NWC = {
            "schema-myevent", "schema-myschema", "create", "delete", "merge"
    };
    private static final String[] MODULE_LABELS_MI = {
            "schema-myevent", "schema-myschema", "infra", "merge"
    };

    private static final String[] PM_FIELDS = {"c1", "c2"};
    private static final String[] NWC_FIELDS = {"col1", "col2"};

    private static final int EXPECTED_STEPS = 102;
    private static final int EXPECTED_RECORDS = 50;

    private InfraNWTableOnMergePatternNoWhereScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergePatternNoWhereScenarioOracle <scenario.json>");
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
     * handler surfaces statement failures to the sender thread.  Deploys
     * register their statements under the step labels so deployed markers,
     * snapshots and undeploy steps can resolve them; the "module" label binds
     * the module's statements positionally in EPL order.  The listener
     * attaches only to the statement named 'Merge' of the multipleinsert
     * cases, mirroring env.compileDeploy(epl).addListener("Merge").
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

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Map<String, String> deploymentIds = new HashMap<>();
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
                        String[] labels;
                        if ("module".equals(label)) {
                            labels = caseName.startsWith("multipleinsert")
                                    ? MODULE_LABELS_MI : MODULE_LABELS_NWC;
                        } else {
                            labels = new String[]{label};
                        }
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != labels.length) {
                            throw new IllegalStateException("deployment of " + caseName + "/"
                                    + label + " has " + deployed.length + " statements, want "
                                    + labels.length);
                        }
                        for (int index = 0; index < deployed.length; index++) {
                            EPStatement statement = deployed[index];
                            if (caseName.startsWith("multipleinsert")
                                    && "Merge".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                            statements.put(labels[index], statement);
                        }
                        for (String bound : labels) {
                            deploymentIds.put(bound, deployment.getDeploymentId());
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
                        record.add("statement", statement.getName());
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        if (rows.size() > 0) {
                            record.add("new", rows);
                        }
                        records.add(record);
                        break;
                    }
                    case "undeploy": {
                        // Mirrors env.undeployModuleContaining: the label
                        // resolves the deployment that owns the statement.
                        String label = string(step, "statement");
                        String deploymentId = deploymentIds.get(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "no deployment for statement " + label);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        deploymentIds.remove(label);
                        statements.remove(label);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deploymentIds.clear();
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
     * This mirrors assertPropsNew on 'Merge': each ordered not-matched
     * insert action delivers the inserted row as new data.
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
     * Sends one pinned event: SupportBean carries theString/intPrimitive,
     * SupportBean_A carries id, and MyEvent is sent as a LinkedHashMap in
     * property order mirroring sendMyEvent's default-representation map.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull()
                        ? null : theString.asString());
                bean.setIntPrimitive(
                        (int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "SupportBean_A": {
                SupportBean_A bean = new SupportBean_A(string(payload, "id"));
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "MyEvent": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("in1", string(payload, "in1"));
                event.put("in2", (int) longInteger(payload.get("in2"), "in2"));
                runtime.getEventService().sendEventMap(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
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
        offset = validatePatternCase(steps, offset, "patternmultimatch-nw", EPL_CREATE_PM_NW);
        offset = validatePatternCase(steps, offset, "patternmultimatch-table",
                EPL_CREATE_PM_TABLE);
        offset = validateNoWhereCase(steps, offset, "nowhere-nw", EPL_MODULE_NWC_NW);
        offset = validateNoWhereCase(steps, offset, "nowhere-table", EPL_MODULE_NWC_TABLE);
        offset = validateMultipleInsertCase(steps, offset, "multipleinsert-nw",
                EPL_MODULE_MI_NW);
        offset = validateMultipleInsertCase(steps, offset, "multipleinsert-table",
                EPL_MODULE_MI_TABLE);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of InfraPatternMultimatch.run (lines 998-1030):
     * the create and merge deploys, the A1/A2 seeds, the B1 match completing
     * both pending branches, the A3/A4 seeds and the B2 match, each followed
     * by an any-order iterator read, then undeployAll; milestone(0/1) are HA
     * checkpoint no-ops that emit no steps.
     */
    private static int validatePatternCase(JsonArray steps, int offset, String caseName,
                                           String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "merge", EPL_MERGE_PM);
        validateBeanSend(steps.get(offset++), caseName, "A1", 1);
        validateBeanSend(steps.get(offset++), caseName, "A2", 1);
        validateBeanSend(steps.get(offset++), caseName, "B1", 1);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", PM_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "A3", 2);
        validateBeanSend(steps.get(offset++), caseName, "A4", 2);
        validateBeanSend(steps.get(offset++), caseName, "B2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", PM_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraNoWhereClause.run (lines 752-804): one
     * five-statement module deploy, then the MyEvent/SupportBean_A sends
     * each followed by an any-order iterator read on 'create'; milestone
     * (0/1/2) are HA checkpoint no-ops that emit no steps.
     */
    private static int validateNoWhereCase(JsonArray steps, int offset, String caseName,
                                           String moduleEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "module", moduleEpl);
        for (String label : MODULE_LABELS_NWC) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateMyEventSend(steps.get(offset++), caseName, "E1", 2);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateMyEventSend(steps.get(offset++), caseName, "A1", 3);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateASend(steps.get(offset++), caseName, "Ax1");
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateMyEventSend(steps.get(offset++), caseName, "A1", 4);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateMyEventSend(steps.get(offset++), caseName, "B1", 5);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateASend(steps.get(offset++), caseName, "Ax1");
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateMyEventSend(steps.get(offset++), caseName, "B1", 5);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateMyEventSend(steps.get(offset++), caseName, "C", 6);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", NWC_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraMultipleInsert.run (lines 691-735): one
     * four-statement module deploy with the listener on 'Merge', then the
     * MyEvent sends whose not-matched inserts surface as listener records;
     * milestone(0/1) are HA checkpoint no-ops that emit no steps.
     */
    private static int validateMultipleInsertCase(JsonArray steps, int offset, String caseName,
                                                  String moduleEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "module", moduleEpl);
        for (String label : MODULE_LABELS_MI) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateMyEventSend(steps.get(offset++), caseName, "E1", 0);
        validateMyEventSend(steps.get(offset++), caseName, "A1", 1);
        validateMyEventSend(steps.get(offset++), caseName, "B1", 2);
        validateMyEventSend(steps.get(offset++), caseName, "C1", 3);
        validateMyEventSend(steps.get(offset++), caseName, "D1", 4);
        validateMyEventSend(steps.get(offset++), caseName, "B1", 2);
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

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedIntPrimitive) {
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
                || longInteger(payload.get("intPrimitive"), "intPrimitive")
                != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateASend(JsonValue value, String caseName, String expectedId) {
        JsonObject step = object(value, "SupportBean_A step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_A".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBean_A step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_A payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException(
                    "SupportBean_A payload is not pinned for " + caseName);
        }
    }

    private static void validateMyEventSend(JsonValue value, String caseName, String expectedIn1,
                                            long expectedIn2) {
        JsonObject step = object(value, "MyEvent step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("MyEvent step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "MyEvent payload");
        requireFields(payload, "in1", "in2");
        if (!expectedIn1.equals(string(payload, "in1"))
                || longInteger(payload.get("in2"), "in2") != expectedIn2) {
            throw new IllegalArgumentException("MyEvent payload is not pinned for " + caseName);
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
