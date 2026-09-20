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
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0;
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
 * Java oracle for InfraNWTableOnMerge ordinals 20-25: the multi-action and
 * ordered-assignment merge executions.  Each of the three execution classes
 * (InfraMultiactionDeleteUpdate, InfraUpdateOrderOfFields,
 * InfraSubqueryNotMatched) runs twice — namedWindow=true first, then
 * namedWindow=false — giving six executions on six runtimes.
 *
 * InfraMultiactionDeleteUpdate deploys the WinMDU keepall named window or
 * primary-key table, the SupportBean insert feeder, and a merge with six
 * ordered matched actions whose where-clauses observe the row state left by
 * earlier actions in the same match: the E5/E6 rows survive because their
 * update actions run before the trailing delete clauses re-evaluate the
 * updated value.  The case ends with undeployModuleContaining("merge") and
 * an EPL-to-model redeploy of the same merge text.
 *
 * InfraUpdateOrderOfFields deploys one three-statement module (the
 * MyInfraUOF infra, the SupportBean insert feeder, and the 'Merge' on-merge)
 * and listens on 'Merge': the update-set assignments evaluate left-to-right,
 * so intBoxed=mywin.intPrimitive reads the already-updated value while
 * doublePrimitive=initial.intPrimitive reads the pre-update value.
 *
 * InfraSubqueryNotMatched deploys the InfraOne and InfraTwo infras, the
 * SupportBean_S0 insert feeder for InfraTwo, and a merge whose not-matched
 * insert assignment carries a correlated subquery over InfraTwo
 * (w2.val0 = sb.theString).  The named-window variant sends a second
 * SupportBean_S0/SupportBean pair so the unique-key InfraTwo row updates and
 * the not-matched insert observes the new val1.
 *
 * assertStatelessStmt("Create") in InfraSubqueryNotMatched and the
 * milestone(0/1/2) ordering markers are harness-internal and emit no steps.
 */
public final class InfraNWTableOnMergeMultiactionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-multiaction";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 20-25: InfraMultiactionDeleteUpdate runs six "
                    + "ordered matched actions over WinMDU where later action where-clauses "
                    + "observe earlier action results (E5/E6 survive their trailing delete "
                    + "clauses); InfraUpdateOrderOfFields evaluates update-set assignments "
                    + "left-to-right so intBoxed reads the already-updated intPrimitive while "
                    + "initial.intPrimitive reads the pre-update value; InfraSubqueryNotMatched "
                    + "evaluates a correlated subquery over InfraTwo in the not-matched insert "
                    + "assignment; each execution runs over a named window and a primary-key "
                    + "table (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-dfa83c0593d19172be7d",
            "java-runtime-ffbda0563d50878dafd8",
            "java-runtime-3034e5da517c1235da22",
            "java-runtime-cb5eefe84a486b090e53",
            "java-runtime-905164d98662721e5509",
            "java-runtime-d3e1f3aa50c4375f657f"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraMultiactionDeleteUpdate{namedWindow=true}",
            "InfraMultiactionDeleteUpdate{namedWindow=false}",
            "InfraUpdateOrderOfFields{namedWindow=true}",
            "InfraUpdateOrderOfFields{namedWindow=false}",
            "InfraSubqueryNotMatched{namedWindow=true}",
            "InfraSubqueryNotMatched{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-d718ed89dce189e3cb3b",
            "java-d718ed89dce189e3cb3b",
            "java-71292c39e6d9067bb77d",
            "java-71292c39e6d9067bb77d",
            "java-1b30fec400708094c9a1",
            "java-1b30fec400708094c9a1"
    };
    private static final String[] CASES = {
            "multiaction-nw", "multiaction-table",
            "orderoffields-nw", "orderoffields-table",
            "subquery-nw", "subquery-table"
    };
    private static final int[] ORDINALS = {20, 21, 22, 23, 24, 25};
    private static final String[] CASE_OBSERVATIONS = {
            "iterator; six ordered matched actions over the WinMDU keepall named window where "
                    + "later action where-clauses observe earlier action results, ending with an "
                    + "undeployModuleContaining('merge') and an EPL-to-model redeploy",
            "iterator; same six ordered matched actions over the WinMDU primary-key table",
            "listener; left-to-right update-set assignments over the MyInfraUOF keepall named "
                    + "window: intBoxed reads the already-updated intPrimitive while "
                    + "doublePrimitive reads initial.intPrimitive",
            "listener; same left-to-right update-set assignments over the MyInfraUOF "
                    + "primary-key table",
            "iterator; correlated subquery over the InfraTwo unique-key named window in the "
                    + "not-matched insert assignment into the InfraOne unique-key named window",
            "iterator; same correlated subquery over the InfraTwo composite-primary-key table "
                    + "into the InfraOne primary-key table"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 1096-1109
    // (InfraMultiactionDeleteUpdate), 1210-1216 (InfraUpdateOrderOfFields)
    // and 1165-1179 (InfraSubqueryNotMatched).
    private static final String EPL_CREATE_MDU_NW =
            "@name('Create') @public create window WinMDU#keepall as SupportBean";
    private static final String EPL_CREATE_MDU_TABLE =
            "@name('Create') @public create table WinMDU (theString string primary key, "
                    + "intPrimitive int)";
    private static final String EPL_INSERT_MDU =
            "insert into WinMDU select theString, intPrimitive from SupportBean";
    private static final String EPL_MERGE_MDU =
            "@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where "
                    + "st0.key0=win.theString when matched "
                    + "then delete where intPrimitive<0 "
                    + "then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 "
                    + "then update set intPrimitive=999 where intPrimitive=1000 "
                    + "then delete where intPrimitive=1000 "
                    + "then update set intPrimitive=1999 where intPrimitive=2000 "
                    + "then delete where intPrimitive=2000";

    private static final String EPL_UOF_TAIL =
            "insert into MyInfraUOF select theString, intPrimitive, intBoxed, doublePrimitive "
                    + "from SupportBean;\n"
                    + "@name('Merge') on SupportBean_S0 as sb merge MyInfraUOF as mywin where "
                    + "mywin.theString = sb.p00 when matched then update set intPrimitive=id, "
                    + "intBoxed=mywin.intPrimitive, doublePrimitive=initial.intPrimitive;\n";
    private static final String EPL_MODULE_UOF_NW =
            "@public create window MyInfraUOF#keepall as SupportBean;\n" + EPL_UOF_TAIL;
    private static final String EPL_MODULE_UOF_TABLE =
            "@public create table MyInfraUOF(theString string primary key, intPrimitive int, "
                    + "intBoxed int, doublePrimitive double);\n" + EPL_UOF_TAIL;

    private static final String EPL_CREATE_ONE_NW =
            "@name('Create') @public create window InfraOne#unique(string) (string string, "
                    + "intPrimitive int)";
    private static final String EPL_CREATE_ONE_TABLE =
            "@name('Create') @public create table InfraOne (string string primary key, "
                    + "intPrimitive int)";
    private static final String EPL_CREATE_TWO_NW =
            "@public create window InfraTwo#unique(val0) (val0 string, val1 int)";
    private static final String EPL_CREATE_TWO_TABLE =
            "@public create table InfraTwo (val0 string primary key, val1 int primary key)";
    private static final String EPL_INSERT_TWO =
            "insert into InfraTwo select 'W2' as val0, id as val1 from SupportBean_S0";
    private static final String EPL_MERGE_SUBQUERY =
            "on SupportBean sb merge InfraOne w1 where sb.theString = w1.string "
                    + "when not matched then insert select 'Y' as string, "
                    + "(select val1 from InfraTwo as w2 where w2.val0 = sb.theString) as "
                    + "intPrimitive";

    private static final String[] CASE_EPLS = {
            EPL_CREATE_MDU_NW, EPL_CREATE_MDU_TABLE,
            EPL_MODULE_UOF_NW, EPL_MODULE_UOF_TABLE,
            EPL_CREATE_ONE_NW, EPL_CREATE_ONE_TABLE
    };

    // Module statement labels in EPL order for the InfraUpdateOrderOfFields
    // single compileDeploy; deployment.getStatements() binds positionally.
    private static final String[] MODULE_LABELS_UOF = {"create", "insert", "merge"};

    private static final String[] MDU_FIELDS = {"theString", "intPrimitive"};
    private static final String[] SUBQUERY_FIELDS = {"string", "intPrimitive"};

    private static final int EXPECTED_STEPS = 109;
    private static final int EXPECTED_RECORDS = 43;

    private InfraNWTableOnMergeMultiactionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeMultiactionScenarioOracle <scenario.json>");
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
     * its own runtime).  SupportBean, SupportBean_ST0 and SupportBean_S0 are
     * preconfigured event types; internal timer is disabled and the
     * rethrowing exception handler surfaces statement failures to the sender
     * thread.  Deploys register their statements under the step labels so
     * deployed markers, snapshots and undeploy steps can resolve them; a
     * repeated label deploys through the EPL-to-model roundtrip, mirroring
     * env.eplToModelCompileDeploy.  The listener attaches only to the
     * statement named 'Merge' (the InfraUpdateOrderOfFields cases).
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_ST0.class);
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
        Map<String, String> deploymentIds = new HashMap<>();
        Set<String> deployedOnce = new HashSet<>();
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
                        String[] labels =
                                "module".equals(label) ? MODULE_LABELS_UOF : new String[]{label};
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled;
                        boolean modelRoundtrip = !deployedOnce.add(label);
                        if (modelRoundtrip) {
                            // Mirrors env.eplToModelCompileDeploy: parse to
                            // the object model, assert the EPL roundtrip and
                            // compile the model.
                            EPStatementObjectModel model = EPCompilerProvider.getCompiler()
                                    .eplToModel(epl, configuration);
                            if (!epl.trim().equals(model.toEPL())) {
                                throw new IllegalStateException(
                                        "EPL-to-model roundtrip mismatch for " + label);
                            }
                            Module module = new Module();
                            module.getItems().add(new ModuleItem(model));
                            module.setModuleText(model.toEPL());
                            compiled = EPCompilerProvider.getCompiler()
                                    .compile(module, compilerArgs);
                        } else {
                            compiled = EPCompilerProvider.getCompiler()
                                    .compile(epl, compilerArgs);
                        }
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != labels.length) {
                            throw new IllegalStateException("deployment of " + caseName + "/"
                                    + label + " has " + deployed.length + " statements, want "
                                    + labels.length);
                        }
                        if (modelRoundtrip && !epl.trim().equals(
                                deployed[0].getProperty(StatementProperty.EPL))) {
                            throw new IllegalStateException(
                                    "redeployed statement EPL mismatch for " + label);
                        }
                        for (int index = 0; index < deployed.length; index++) {
                            EPStatement statement = deployed[index];
                            if ("Merge".equals(statement.getName())) {
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
     * This mirrors assertPropsPerRowLastNew on 'Merge': the merge update
     * delivers the updated row as an insert-remove pair.
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
     * Sends one pinned event: SupportBean carries theString/intPrimitive
     * plus doublePrimitive when the payload sets it (mirroring
     * makeSupportBean), SupportBean_ST0 carries id/key0/p00, and
     * SupportBean_S0 carries id plus p00 when present.
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
                JsonValue doublePrimitive = payload.get("doublePrimitive");
                if (doublePrimitive != null && !doublePrimitive.isNull()) {
                    bean.setDoublePrimitive(
                            doubleValue(doublePrimitive, "doublePrimitive"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "SupportBean_ST0": {
                SupportBean_ST0 bean = new SupportBean_ST0(
                        string(payload, "id"), string(payload, "key0"),
                        (int) longInteger(payload.get("p00"), "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "SupportBean_S0": {
                int id = (int) longInteger(payload.get("id"), "id");
                JsonValue p00 = payload.get("p00");
                SupportBean_S0 bean = p00 == null || p00.isNull()
                        ? new SupportBean_S0(id)
                        : new SupportBean_S0(id, p00.asString());
                runtime.getEventService().sendEventBean(bean, type);
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
        offset = validateMultiactionCase(steps, offset, "multiaction-nw", EPL_CREATE_MDU_NW);
        offset = validateMultiactionCase(steps, offset, "multiaction-table",
                EPL_CREATE_MDU_TABLE);
        offset = validateOrderOfFieldsCase(steps, offset, "orderoffields-nw", EPL_MODULE_UOF_NW);
        offset = validateOrderOfFieldsCase(steps, offset, "orderoffields-table",
                EPL_MODULE_UOF_TABLE);
        offset = validateSubqueryCase(steps, offset, "subquery-nw", EPL_CREATE_ONE_NW,
                EPL_CREATE_TWO_NW, true);
        offset = validateSubqueryCase(steps, offset, "subquery-table", EPL_CREATE_ONE_TABLE,
                EPL_CREATE_TWO_TABLE, false);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of InfraMultiactionDeleteUpdate.run (lines
     * 1094-1146): create/insert/merge deploys, six SupportBean +
     * SupportBean_ST0 send pairs each followed by an iterator read (ordered
     * for the single-row asserts, any-order for the multi-row asserts), then
     * undeployModuleContaining("merge"), the EPL-to-model redeploy and
     * undeployAll; milestone(0..2) are HA checkpoint no-ops that emit no
     * steps.
     */
    private static int validateMultiactionCase(JsonArray steps, int offset, String caseName,
                                               String createEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createEpl);
        offset = validateDeployPair(steps, offset, caseName, "insert", EPL_INSERT_MDU);
        offset = validateDeployPair(steps, offset, caseName, "merge", EPL_MERGE_MDU);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E1", 0);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", MDU_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E2", -1, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E2", 0);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", MDU_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3000, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E3", 3);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", MDU_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E4", 4, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E4", 3000);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", MDU_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E5", 1000, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E5", 0);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", MDU_FIELDS);
        validateBeanSend(steps.get(offset++), caseName, "E6", 2000, null);
        validateST0Send(steps.get(offset++), caseName, "ST0", "E6", 0);
        validateSnapshot(steps.get(offset++), caseName, "create", "any", MDU_FIELDS);
        validateUndeploy(steps.get(offset++), caseName, "merge");
        offset = validateDeployPair(steps, offset, caseName, "merge", EPL_MERGE_MDU);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraUpdateOrderOfFields.run (lines 1209-1234):
     * one three-statement module deploy, the makeSupportBean(E1,1,2) seed
     * and SupportBean_S0(5,"E1") trigger, the makeSupportBean(E2,10,20) seed
     * and SupportBean_S0(6,"E2") trigger, the SupportBean_S0(7,"E1")
     * re-trigger of the updated E1 row, and undeployAll; milestone(0) is an
     * HA checkpoint no-op.
     */
    private static int validateOrderOfFieldsCase(JsonArray steps, int offset, String caseName,
                                                 String moduleEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "module", moduleEpl);
        for (String label : MODULE_LABELS_UOF) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, 2.0);
        validateS0Send(steps.get(offset++), caseName, 5, "E1");
        validateBeanSend(steps.get(offset++), caseName, "E2", 10, 20.0);
        validateS0Send(steps.get(offset++), caseName, 6, "E2");
        validateS0Send(steps.get(offset++), caseName, 7, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraSubqueryNotMatched.run (lines 1163-1192):
     * the InfraOne and InfraTwo creates, the InfraTwo insert feeder and the
     * merge deploys, then SupportBean_S0(50) seeding {W2,50} and
     * SupportBean("W2",1) triggering the not-matched insert before the
     * iterator read; the named-window variant repeats with
     * SupportBean_S0(51) and SupportBean("W2",2); undeployAll ends the case.
     */
    private static int validateSubqueryCase(JsonArray steps, int offset, String caseName,
                                            String createOneEpl, String createTwoEpl,
                                            boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName);
        offset = validateDeployPair(steps, offset, caseName, "create", createOneEpl);
        offset = validateDeployPair(steps, offset, caseName, "create-two", createTwoEpl);
        offset = validateDeployPair(steps, offset, caseName, "insert", EPL_INSERT_TWO);
        offset = validateDeployPair(steps, offset, caseName, "merge", EPL_MERGE_SUBQUERY);
        validateS0Send(steps.get(offset++), caseName, 50, null);
        validateBeanSend(steps.get(offset++), caseName, "W2", 1, null);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", SUBQUERY_FIELDS);
        if (namedWindow) {
            validateS0Send(steps.get(offset++), caseName, 51, null);
            validateBeanSend(steps.get(offset++), caseName, "W2", 2, null);
            validateSnapshot(steps.get(offset++), caseName, "create", "ordered", SUBQUERY_FIELDS);
        }
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
                                         long expectedIntPrimitive, Double expectedDouble) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (expectedDouble == null) {
            requireFields(payload, "theString", "intPrimitive");
        } else {
            requireFields(payload, "theString", "intPrimitive", "doublePrimitive");
            if (doubleValue(payload.get("doublePrimitive"), "doublePrimitive")
                    != expectedDouble) {
                throw new IllegalArgumentException(
                        "SupportBean payload is not pinned for " + caseName);
            }
        }
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive")
                != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateST0Send(JsonValue value, String caseName, String expectedId,
                                        String expectedKey0, long expectedP00) {
        JsonObject step = object(value, "SupportBean_ST0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_ST0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBean_ST0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_ST0 payload");
        requireFields(payload, "id", "key0", "p00");
        if (!expectedId.equals(string(payload, "id"))
                || !expectedKey0.equals(string(payload, "key0"))
                || longInteger(payload.get("p00"), "p00") != expectedP00) {
            throw new IllegalArgumentException(
                    "SupportBean_ST0 payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, long expectedId,
                                       String expectedP00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        if (expectedP00 == null) {
            requireFields(payload, "id");
        } else {
            requireFields(payload, "id", "p00");
            if (!expectedP00.equals(string(payload, "p00"))) {
                throw new IllegalArgumentException(
                        "SupportBean_S0 payload is not pinned for " + caseName);
            }
        }
        if (longInteger(payload.get("id"), "id") != expectedId) {
            throw new IllegalArgumentException(
                    "SupportBean_S0 payload is not pinned for " + caseName);
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

    private static double doubleValue(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON number");
        }
        try {
            return Double.parseDouble(value.toString());
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is not a JSON number", ex);
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
