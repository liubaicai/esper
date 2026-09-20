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
 * Java oracle for InfraNWTableOnMerge ordinals 32-39: the InfraFlow
 * insert-into/delete/merge lifecycle and the InfraInnerTypeAndVariable
 * tri-state-variable merge executions.  InfraFlow runs twice
 * (namedWindow=true first, then namedWindow=false) and
 * InfraInnerTypeAndVariable runs over the OBJECTARRAY, MAP and DEFAULT
 * representations for each infra kind, giving eight executions on eight
 * runtimes.
 *
 * InfraFlow deploys the MyMergeInfra unique-key named window or
 * primary-key table ('Window'), a filtered SupportBean insert-into feeder
 * ('Insert') and an unconditional SupportBean_A delete-all trigger
 * ('Delete'), then the four-branch 'Merge' (matched delete on
 * intPrimitive&lt;0, matched reset on intPrimitive=0, matched fallback
 * update accumulating intBoxed, not-matched insert).  runAssertionFlow
 * executes twice: between passes the merge module is undeployed, a
 * SupportBean_A event clears the infra and the same EPL redeploys.  After
 * pass two a wildcard merge (insert select up.* for the named window,
 * explicit columns for the table) inserts E99, and a final
 * ambiguous-columns module (TypeOne merging into MyInfraTwo) compiles and
 * deploys without sends.  Listeners attach to 'Window' and 'Merge'; the
 * 'Window' listener never fires for the table variant.
 *
 * InfraInnerTypeAndVariable deploys the MyInnerSchema/MyEventSchema
 * create-schema module, the MyInfraITV keepall named window or
 * primary-key table, the 'createvar' boolean variable module and the
 * 'Merge' on-merge whose three not-matched branches select on the
 * tri-state myvar (null inserts 'B'/me.col2, true inserts col1/col2,
 * false inserts 'A'/null) while matched rows delete.  The Java execution
 * asserts through the 'Merge' listener; this scenario observes the same
 * state through iterator snapshots on the infra (contract: itv cases are
 * iterator-only, no listeners).  milestone(0/1) calls are HA recovery
 * checkpoints that emit no steps; the scenario drops them.  Each
 * execution ends with undeployAll.
 */
public final class InfraNWTableOnMergeFlowITVScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-flow-itv";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 32-39: InfraFlow wires a filtered "
                    + "SupportBean insert-into feeder, an unconditional "
                    + "SupportBean_A delete-all trigger and a four-branch "
                    + "on-merge (matched delete on intPrimitive<0, matched "
                    + "reset on intPrimitive=0, matched fallback update "
                    + "accumulating intBoxed, not-matched insert) over the "
                    + "MyMergeInfra unique-key named window or primary-key "
                    + "table, runs the assertion flow twice across an "
                    + "undeploy/redeploy of the merge module, then exercises "
                    + "a wildcard merge tail and an ambiguous-columns module; "
                    + "InfraInnerTypeAndVariable merges MyEventSchema events "
                    + "into the MyInfraITV keepall named window or "
                    + "primary-key table under the tri-state myvar variable "
                    + "selecting among three not-matched insert branches "
                    + "with a nested-fragment c2 column and a matched-delete, "
                    + "over OBJECTARRAY, MAP and DEFAULT representations "
                    + "(Java source regression-lib/src/main/java/com/"
                    + "espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-403bba8c6b29e32b1a8f",
            "java-runtime-ae05c015767242106de7",
            "java-runtime-3303d0922bd2d722fa3c",
            "java-runtime-f20972a347aabfcc0260",
            "java-runtime-484a8e636d3734b87673",
            "java-runtime-76d16e1334c83e6d4002",
            "java-runtime-462d190f20742a0c266d",
            "java-runtime-8495f57749c2105b15a8"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraFlow{namedWindow=true}",
            "InfraFlow{namedWindow=false}",
            "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}",
            "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}",
            "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=MAP}",
            "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=MAP}",
            "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=DEFAULT}",
            "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=DEFAULT}"
    };
    private static final String[] STATIC_IDS = {
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763",
            "java-097f9b8edb46959e0763"
    };
    private static final String[] CASES = {
            "flow-nw", "flow-table",
            "itv-nw-objectarray", "itv-table-objectarray",
            "itv-nw-map", "itv-table-map",
            "itv-nw-default", "itv-table-default"
    };
    private static final int[] ORDINALS = {32, 33, 34, 35, 36, 37, 38, 39};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+iterator; filtered insert-into feeder, delete-all "
                    + "trigger and four-branch merge over the MyMergeInfra "
                    + "unique-key named window, run twice across merge-module "
                    + "undeploy/redeploy, then a wildcard merge tail and an "
                    + "ambiguous-columns module",
            "listener+iterator; same flow over the MyMergeInfra primary-key "
                    + "table (the 'Window' listener is attached but never "
                    + "fires for a table)",
            "iterator; tri-state myvar selects among three not-matched "
                    + "insert branches over the MyInfraITV keepall named "
                    + "window with a nested-fragment c2 column, matched "
                    + "deletes, OBJECTARRAY representation",
            "iterator; same tri-state merge over the MyInfraITV primary-key "
                    + "table, OBJECTARRAY representation",
            "iterator; same tri-state merge over the MyInfraITV keepall "
                    + "named window, MAP representation",
            "iterator; same tri-state merge over the MyInfraITV primary-key "
                    + "table, MAP representation",
            "iterator; same tri-state merge over the MyInfraITV keepall "
                    + "named window, DEFAULT representation",
            "iterator; same tri-state merge over the MyInfraITV primary-key "
                    + "table, DEFAULT representation"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 538-596
    // (InfraFlow) and 906-925 (InfraInnerTypeAndVariable).  The flow merge
    // and wildcard EPLs end without a semicolon; the ambiguous module
    // carries literal newlines and a trailing newline.
    private static final String EPL_FLOW_CREATE_NW =
            "@Name('Window') @public create window MyMergeInfra#unique(theString) as SupportBean";
    private static final String EPL_FLOW_CREATE_TABLE =
            "@Name('Window') @public create table MyMergeInfra (theString string primary key, "
                    + "intPrimitive int, intBoxed int)";
    private static final String EPL_FLOW_INSERT =
            "@Name('Insert') insert into MyMergeInfra select theString, intPrimitive, intBoxed "
                    + "from SupportBean(boolPrimitive)";
    private static final String EPL_FLOW_DELETE =
            "@Name('Delete') on SupportBean_A delete from MyMergeInfra";
    private static final String EPL_FLOW_MERGE_HEAD =
            "@Name('Merge') on SupportBean(boolPrimitive=false) as up "
                    + "merge MyMergeInfra as mv "
                    + "where mv.theString=up.theString "
                    + "when matched and up.intPrimitive<0 then "
                    + "delete "
                    + "when matched and up.intPrimitive=0 then "
                    + "update set intPrimitive=0, intBoxed=0 "
                    + "when matched then "
                    + "update set intPrimitive=up.intPrimitive, intBoxed=up.intBoxed+mv.intBoxed "
                    + "when not matched then "
                    + "insert select ";
    private static final String EPL_FLOW_MERGE_NW = EPL_FLOW_MERGE_HEAD + "*";
    private static final String EPL_FLOW_MERGE_TABLE =
            EPL_FLOW_MERGE_HEAD + "theString, intPrimitive, intBoxed";
    private static final String EPL_FLOW_WILDCARD_HEAD =
            "@name('Merge') on SupportBean(boolPrimitive = false) as up "
                    + "merge MyMergeInfra as mv "
                    + "where mv.theString = up.theString "
                    + "when not matched then "
                    + "insert select ";
    private static final String EPL_FLOW_WILDCARD_NW = EPL_FLOW_WILDCARD_HEAD + "up.*";
    private static final String EPL_FLOW_WILDCARD_TABLE =
            EPL_FLOW_WILDCARD_HEAD + "theString, intPrimitive, intBoxed";
    private static final String EPL_FLOW_AMBIGUOUS_HEAD =
            "create schema TypeOne (id long, mylong long, mystring long);\n";
    private static final String EPL_FLOW_AMBIGUOUS_NW =
            "@public create window MyInfraTwo#unique(id) as select * from TypeOne;\n";
    private static final String EPL_FLOW_AMBIGUOUS_TABLE =
            "@public create table MyInfraTwo (id long, mylong long, mystring long);\n";
    private static final String EPL_FLOW_AMBIGUOUS_TAIL =
            "on TypeOne as t1 merge MyInfraTwo nm where nm.id = t1.id\n"
                    + "  when not matched and mystring = 0 then insert select *\n"
                    + "  when not matched then insert (id, mylong, mystring) select 0L, 0L, 0L\n";

    private static final String EPL_ITV_CREATEVAR =
            "@name('createvar') @public create variable boolean myvar";
    private static final String EPL_ITV_MERGE =
            "@name('Merge') on MyEventSchema me "
                    + "merge MyInfraITV mw "
                    + "where me.col1 = mw.c1 "
                    + " when not matched and myvar then "
                    + "  insert select col1 as c1, col2 as c2 "
                    + " when not matched and myvar = false then "
                    + "  insert select 'A' as c1, null as c2 "
                    + " when not matched and myvar is null then "
                    + "  insert select 'B' as c1, me.col2 as c2 "
                    + " when matched then "
                    + "  delete";

    private static final String[] FLOW_FIELDS = {"theString", "intPrimitive", "intBoxed"};
    private static final String[] ITV_FIELDS = {"c1", "c2.in1", "c2.in2"};

    // Module statement labels in EPL order for the multi-statement deploys:
    // the ITV schema module binds its two create-schema statements
    // positionally; the ambiguous module binds schema, infra and merge.
    private static final String[] MODULE_LABELS_ITV_SCHEMA = {
            "schema-inner", "schema-event"
    };
    private static final String[] MODULE_LABELS_AMBIGUOUS = {
            "schema-typeone", "infra-two", "merge-two"
    };

    private static final int EXPECTED_STEPS = 282;
    private static final int EXPECTED_RECORDS = 195;

    private InfraNWTableOnMergeFlowITVScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeFlowITVScenarioOracle <scenario.json>");
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
     * snapshots, set-variable and undeploy steps can resolve them; the
     * "schema" and "module" labels bind their module's statements
     * positionally in EPL order.  The listener attaches only to the
     * statements named 'Window' and 'Merge' of the flow cases, mirroring
     * env.compileDeploy(epl).addListener(...); the itv cases are
     * iterator-only per the scenario contract.
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
                        if ("schema".equals(label)) {
                            labels = MODULE_LABELS_ITV_SCHEMA;
                        } else if ("module".equals(label)) {
                            labels = MODULE_LABELS_AMBIGUOUS;
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
                            if (caseName.startsWith("flow-")
                                    && ("Window".equals(statement.getName())
                                    || "Merge".equals(statement.getName()))) {
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
                        sendEvent(runtime, caseName, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "set-variable": {
                        // Mirrors env.runtime().getVariableService()
                        // .setVariableValue(env.deploymentId("createvar"),
                        // "myvar", value): the step's statement label
                        // resolves the deployment that owns the variable.
                        String label = string(step, "statement");
                        String deploymentId = deploymentIds.get(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "no deployment for statement " + label);
                        }
                        String name = string(step, "name");
                        JsonValue rawValue = step.get("payload");
                        Object value = rawValue == null || rawValue.isNull()
                                ? null : Boolean.valueOf(rawValue.asBoolean());
                        runtime.getVariableService().setVariableValue(
                                deploymentId, name, value);
                        int sequence = sequences.merge(label + ":set-variable", 1, Integer::sum);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "set-variable");
                        record.add("statement", label);
                        record.add("sequence", sequence);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("name", name);
                        record.add("value", normalize(value));
                        records.add(record);
                        break;
                    }
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
     * This mirrors assertPropsNew/assertPropsIRPair/assertPropsOld on
     * 'Window' and 'Merge': merge actions deliver the inserted, updated or
     * deleted row to the merge statement's own listener, and named-window
     * mutations reach the 'Window' listener first.
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
     * ordering matches the Go runner's fields-JSON sort.  Nested paths
     * such as c2.in1 resolve through the fragment's schema. */
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
     * Sends one pinned event: SupportBean carries theString/intPrimitive/
     * intBoxed/boolPrimitive (sendSupportBeanEvent), SupportBean_A carries
     * id, and MyEventSchema carries col1 plus the nested col2 fragment in
     * the case's representation (sendMyInnerSchemaEvent: positional object
     * array for OBJECTARRAY, name/value map for MAP and DEFAULT).
     */
    private static void sendEvent(EPRuntime runtime, String caseName, String type,
                                  JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                JsonValue theString = payload.get("theString");
                JsonValue intBoxed = payload.get("intBoxed");
                SupportBean bean = new SupportBean(
                        theString == null || theString.isNull() ? null : theString.asString(),
                        (int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                bean.setIntBoxed(intBoxed == null || intBoxed.isNull()
                        ? null : Integer.valueOf(
                        (int) longInteger(intBoxed, "intBoxed")));
                bean.setBoolPrimitive(payload.get("boolPrimitive") != null
                        && payload.get("boolPrimitive").asBoolean());
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "SupportBean_A": {
                SupportBean_A bean = new SupportBean_A(string(payload, "id"));
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "MyEventSchema": {
                String col1 = string(payload, "col1");
                JsonObject col2 = object(payload.get("col2"), "col2");
                String in1 = string(col2, "in1");
                int in2 = (int) longInteger(col2.get("in2"), "in2");
                if (caseName.endsWith("-objectarray")) {
                    runtime.getEventService().sendEventObjectArray(
                            new Object[]{col1, new Object[]{in1, in2}}, type);
                    return;
                }
                // MAP and DEFAULT both send the name/value map
                // (EventRepresentationChoice.isMapEvent covers DEFAULT).
                Map<String, Object> inner = new LinkedHashMap<>();
                inner.put("in1", in1);
                inner.put("in2", in2);
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("col1", col1);
                event.put("col2", inner);
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
        validateStringArray(scenario.get("javaFlags"), new String[]{"OBSERVEROPS"}, "javaFlags");

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
        offset = validateFlowCase(steps, offset, "flow-nw", false);
        offset = validateFlowCase(steps, offset, "flow-table", true);
        for (String caseName : CASES) {
            if (!caseName.startsWith("itv-")) {
                continue;
            }
            offset = validateITVCase(steps, offset, caseName);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the first deploy EPL of each case. */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "flow-nw":
                return EPL_FLOW_CREATE_NW;
            case "flow-table":
                return EPL_FLOW_CREATE_TABLE;
            default:
                return itvSchemaModuleEpl(caseName);
        }
    }

    /**
     * The InfraInnerTypeAndVariable schema module (lines 906-909): the
     * representation annotation prefixes each create-schema; DEFAULT's
     * empty annotation leaves a leading space.
     */
    private static String itvSchemaModuleEpl(String caseName) {
        String annotation;
        if (caseName.endsWith("-objectarray")) {
            annotation = "@EventRepresentation('objectarray')";
        } else if (caseName.endsWith("-map")) {
            annotation = "@EventRepresentation('map')";
        } else {
            annotation = "";
        }
        return annotation + " @public create schema MyInnerSchema(in1 string, in2 int);\n"
                + annotation + " @public @buseventtype @public create schema MyEventSchema"
                + "(col1 string, col2 MyInnerSchema)";
    }

    /**
     * The InfraInnerTypeAndVariable infra create EPL (lines 911-914): the
     * representation annotation applies to the named window only.
     */
    private static String itvInfraEpl(String caseName) {
        if (caseName.startsWith("itv-table-")) {
            return "@public create table MyInfraITV as (c1 string primary key, c2 MyInnerSchema)";
        }
        String annotation;
        if (caseName.endsWith("-objectarray")) {
            annotation = "@EventRepresentation('objectarray')";
        } else if (caseName.endsWith("-map")) {
            annotation = "@EventRepresentation('map')";
        } else {
            annotation = "";
        }
        return annotation + " @public create window MyInfraITV#keepall as (c1 string, "
                + "c2 MyInnerSchema)";
    }

    private static String flowMergeEpl(boolean isTable) {
        return isTable ? EPL_FLOW_MERGE_TABLE : EPL_FLOW_MERGE_NW;
    }

    private static String flowWildcardEpl(boolean isTable) {
        return isTable ? EPL_FLOW_WILDCARD_TABLE : EPL_FLOW_WILDCARD_NW;
    }

    private static String flowAmbiguousEpl(boolean isTable) {
        return EPL_FLOW_AMBIGUOUS_HEAD
                + (isTable ? EPL_FLOW_AMBIGUOUS_TABLE : EPL_FLOW_AMBIGUOUS_NW)
                + EPL_FLOW_AMBIGUOUS_TAIL;
    }

    /**
     * Exact step sequence of InfraFlow.run (lines 536-596): the Window,
     * Insert and Delete deploys, the four-branch Merge deploy, two
     * runAssertionFlow passes around an undeploy/SupportBean_A-clear/
     * redeploy cycle, the wildcard merge tail and the ambiguous-columns
     * module, then undeployAll; milestoneInc calls are HA checkpoint
     * no-ops that emit no steps.
     */
    private static int validateFlowCase(JsonArray steps, int offset, String caseName,
                                        boolean isTable) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "Window",
                isTable ? EPL_FLOW_CREATE_TABLE : EPL_FLOW_CREATE_NW);
        offset = validateDeployPair(steps, offset, caseName, "Insert", EPL_FLOW_INSERT);
        offset = validateDeployPair(steps, offset, caseName, "Delete", EPL_FLOW_DELETE);
        offset = validateDeployPair(steps, offset, caseName, "Merge", flowMergeEpl(isTable));
        offset = validateFlowPass(steps, offset, caseName);
        validateUndeploy(steps.get(offset++), caseName, "Merge");
        validateASend(steps.get(offset++), caseName, "A1");
        offset = validateDeployPair(steps, offset, caseName, "Merge", flowMergeEpl(isTable));
        offset = validateFlowPass(steps, offset, caseName);
        validateASend(steps.get(offset++), caseName, "A2");
        validateUndeploy(steps.get(offset++), caseName, "Merge");
        offset = validateDeployPair(steps, offset, caseName, "Merge", flowWildcardEpl(isTable));
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E99", 2, 3);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateDeploy(steps.get(offset++), caseName, "module", flowAmbiguousEpl(isTable));
        for (String label : MODULE_LABELS_AMBIGUOUS) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of runAssertionFlow (lines 606-682): nine
     * SupportBean sends each followed by an any-order iterator read on
     * 'Window' — insert-into seed, matched update, not-matched insert, two
     * accumulating updates, insert, reset update and two matched deletes.
     */
    private static int validateFlowPass(JsonArray steps, int offset, String caseName) {
        validateFlowBeanSend(steps.get(offset++), caseName, true, "E1", 10, 200);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E1", 11, 201);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E2", 13, 300);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E2", 14, 301);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E2", 15, 302);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E3", 40, 400);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E3", 0, 1000);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E2", -1, 1000);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        validateFlowBeanSend(steps.get(offset++), caseName, false, "E1", -1, 1000);
        validateSnapshot(steps.get(offset++), caseName, "Window", "any", FLOW_FIELDS);
        return offset;
    }

    /**
     * Exact step sequence of InfraInnerTypeAndVariable.run (lines 905-971):
     * the schema, infra, createvar and merge deploys, then the pinned
     * send/set-variable sequence with an iterator read on the infra after
     * each send — myvar null inserts 'B'/me.col2, the matched send deletes,
     * myvar true inserts col1/col2, myvar false inserts 'A'/null, and after
     * the merge-module undeploy/redeploy myvar true inserts X4.  milestone
     * (0/1) are HA checkpoint no-ops that emit no steps.
     */
    private static int validateITVCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "schema", itvSchemaModuleEpl(caseName));
        for (String label : MODULE_LABELS_ITV_SCHEMA) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        offset = validateDeployPair(steps, offset, caseName, "infra", itvInfraEpl(caseName));
        offset = validateDeployPair(steps, offset, caseName, "createvar", EPL_ITV_CREATEVAR);
        offset = validateDeployPair(steps, offset, caseName, "Merge", EPL_ITV_MERGE);
        validateITVSend(steps.get(offset++), caseName, "X1", "Y1", 10);
        validateSnapshot(steps.get(offset++), caseName, "infra", "any", ITV_FIELDS);
        validateITVSend(steps.get(offset++), caseName, "B", "0", 0);
        validateSnapshot(steps.get(offset++), caseName, "infra", "any", ITV_FIELDS);
        validateSetVariable(steps.get(offset++), caseName, "createvar", "myvar", true);
        validateITVSend(steps.get(offset++), caseName, "X2", "Y2", 11);
        validateSnapshot(steps.get(offset++), caseName, "infra", "any", ITV_FIELDS);
        validateSetVariable(steps.get(offset++), caseName, "createvar", "myvar", false);
        validateITVSend(steps.get(offset++), caseName, "X3", "Y3", 12);
        validateSnapshot(steps.get(offset++), caseName, "infra", "any", ITV_FIELDS);
        validateUndeploy(steps.get(offset++), caseName, "Merge");
        offset = validateDeployPair(steps, offset, caseName, "Merge", EPL_ITV_MERGE);
        validateSetVariable(steps.get(offset++), caseName, "createvar", "myvar", true);
        validateITVSend(steps.get(offset++), caseName, "X4", "Y4", 11);
        validateSnapshot(steps.get(offset++), caseName, "infra", "any", ITV_FIELDS);
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

    private static void validateFlowBeanSend(JsonValue value, String caseName,
                                             boolean expectedBool, String expectedString,
                                             long expectedIntPrimitive, long expectedIntBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive", "intBoxed", "boolPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != expectedIntPrimitive
                || longInteger(payload.get("intBoxed"), "intBoxed") != expectedIntBoxed
                || payload.get("boolPrimitive").asBoolean() != expectedBool) {
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

    private static void validateITVSend(JsonValue value, String caseName, String expectedCol1,
                                        String expectedIn1, long expectedIn2) {
        JsonObject step = object(value, "MyEventSchema step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyEventSchema".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("MyEventSchema step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "MyEventSchema payload");
        requireFields(payload, "col1", "col2");
        JsonObject col2 = object(payload.get("col2"), "col2");
        requireFields(col2, "in1", "in2");
        if (!expectedCol1.equals(string(payload, "col1"))
                || !expectedIn1.equals(string(col2, "in1"))
                || longInteger(col2.get("in2"), "in2") != expectedIn2) {
            throw new IllegalArgumentException(
                    "MyEventSchema payload is not pinned for " + caseName);
        }
    }

    private static void validateSetVariable(JsonValue value, String caseName,
                                            String expectedStatement, String expectedName,
                                            boolean expectedValue) {
        JsonObject step = object(value, "set-variable step");
        requireFields(step, "op", "case", "statement", "name", "payload");
        if (!"set-variable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedName.equals(string(step, "name"))
                || step.get("payload") == null || step.get("payload").isNull()
                || step.get("payload").asBoolean() != expectedValue) {
            throw new IllegalArgumentException("set-variable step is not pinned for " + caseName);
        }
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
