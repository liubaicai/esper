import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.CreateWindowClause;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.OnClause;
import com.espertech.esper.common.client.soda.SchemaColumnDesc;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.StreamSelector;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
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
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the InfraNamedWindowOM parity bundle —
 * all three executions of InfraNamedWindowOM.java (SODA/eplToModel
 * compile-path over one keepall key/value named window).
 *
 * compile (ord 0, InfraCompile, lines 44-109): five eplToModel deploys in
 * source order — the @public create-window as-select {key,value}, an
 * unconditional on-SupportBean_B select mywin.*, the insert-into, the
 * irstream select filtered on `key is not null` and, after the E1/E2
 * sends, the on-SupportMarketDataBean delete correlated on
 * s0.symbol=s1.key. All five statements carry listeners; create, select
 * and delete additionally assert model.toEPL() after deploy (unrep
 * records). The send matrix runs E1/E2, the E1/E1/E2 delete sequence,
 * the B1 empty-window and B2 filled-window on-select probes, then
 * null-key/null-value boundary beans and null-symbol/key delete probes
 * before the per-statement undeploys (delete/onselect/select/insert/
 * create, mirroring undeployModuleContaining).
 *
 * om (ord 1, InfraOM, lines 119-206): the same five statements built as
 * programmatic EPStatementObjectModels — each toEPL assert precedes its
 * deploy, the select filter reads `value is not null` and the on-select
 * correlates s0.id=s1.key (B1 and the null-id probe stay silent). The
 * insert deploys through the eplToModelCompileDeploy equivalent (parse,
 * toEPL round-trip assert, deploy). Teardown is undeployAll.
 *
 * create-table-syntax (ord 2, InfraOMCreateTableSyntax, lines 212-221):
 * compile-only schema-column create-window OM asserted through toEPL;
 * zero deploys, sends or listeners — the single unrepresentable record
 * pins the asserted text.
 */
public final class InfraNamedWindowOM564ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-om-564";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOM.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOM.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportMarketDataBean.java"
    };

    private static final String[] RUNTIME_IDS = {
            "java-runtime-2fadc5dd18bce73a28d5",
            "java-runtime-2d6d5656b8c1bbec11ac",
            "java-runtime-7521a52d49a6bc033264"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraCompile",
            "InfraOM",
            "InfraOMCreateTableSyntax"
    };
    private static final String[] STATIC_IDS = {
            "java-10c000a768d7eb5776f7",
            "java-10c000a768d7eb5776f7",
            "java-10c000a768d7eb5776f7"
    };
    private static final String[] CASES = {"compile", "om", "create-table-syntax"};
    private static final int[] ORDINALS = {0, 1, 2};

    // Verbatim transcriptions of InfraNamedWindowOM.java: the InfraCompile
    // string literals (lines 44/49/53/57/72) and the InfraOM assertEquals
    // literals (lines 126/140/161/189) — create's assert runs after the
    // annotations are set; select/ondelete/onselect assert the unannotated
    // model text while the deployed module carries the annotated toEPL.
    private static final String EPL_CREATE =
            "@name('create') @public create window MyWindow#keepall as "
                    + "select theString as key, longBoxed as value from SupportBean";
    private static final String EPL_ONSELECT =
            "@name('onselect') on SupportBean_B select mywin.* from MyWindow as mywin";
    private static final String EPL_INSERT =
            "@name('insert') insert into MyWindow select theString as key, "
                    + "longBoxed as value from SupportBean";
    private static final String EPL_SELECT =
            "@name('select') select irstream key, value*2 as value from "
                    + "MyWindow(key is not null)";
    private static final String EPL_DELETE =
            "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow "
                    + "as s1 where s0.symbol=s1.key";

    private static final String OM_INSERT =
            "insert into MyWindow select theString as key, longBoxed as value "
                    + "from SupportBean";
    private static final String OM_SELECT_TEXT =
            "select irstream key, value*2 as value from MyWindow(value is not null)";
    private static final String OM_SELECT_EPL = "@name('select') " + OM_SELECT_TEXT;
    private static final String OM_DELETE_TEXT =
            "on SupportMarketDataBean as s0 delete from MyWindow as s1 "
                    + "where s0.symbol=s1.key";
    private static final String OM_DELETE_EPL = "@name('ondelete') " + OM_DELETE_TEXT;
    private static final String OM_ONSELECT_TEXT =
            "on SupportBean_B as s0 select s1.* from MyWindow as s1 "
                    + "where s0.id=s1.key";
    private static final String OM_ONSELECT_EPL = "@name('onselect') " + OM_ONSELECT_TEXT;
    private static final String CREATE_TABLE_TEXT =
            "create window MyWindowOM#keepall as (a1 string, a2 double, a3 int)";

    // Deploy labels per case in Java source order, with the pinned EPL and
    // the listener name the deployed statement reports (InfraOM's insert is
    // unnamed and unlistened; its delete/on-select carry ondelete/onselect).
    private static final String[][] COMPILE_DEPLOYS = {
            {"create", EPL_CREATE, "create"},
            {"onselect", EPL_ONSELECT, "onselect"},
            {"insert", EPL_INSERT, "insert"},
            {"select", EPL_SELECT, "select"},
            {"delete", EPL_DELETE, "delete"}
    };
    private static final String[][] OM_DEPLOYS = {
            {"create", EPL_CREATE, "create"},
            {"insert", OM_INSERT, ""},
            {"select", OM_SELECT_EPL, "select"},
            {"delete", OM_DELETE_EPL, "ondelete"},
            {"onselect", OM_ONSELECT_EPL, "onselect"}
    };

    // toEPL-assert pins per case: label, asserted text, and whether the
    // assert sees the annotated model (compile asserts post-deploy on the
    // eplToModel text; om asserts the unannotated model except create).
    private static final String[][] COMPILE_UNREP = {
            {"toepl-create", EPL_CREATE},
            {"toepl-select", EPL_SELECT},
            {"toepl-delete", EPL_DELETE}
    };
    private static final String[][] OM_UNREP = {
            {"toepl-create", EPL_CREATE},
            {"toepl-select-om", OM_SELECT_TEXT},
            {"toepl-ondelete", OM_DELETE_TEXT},
            {"toepl-onselect", OM_ONSELECT_TEXT}
    };

    // Send pins per case: event type plus payload JSON literal.
    private static final String[][] COMPILE_SENDS = {
            {"SupportBean", "{\"theString\":\"E1\",\"longBoxed\":10}"},
            {"SupportBean", "{\"theString\":\"E2\",\"longBoxed\":20}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E1\"}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E1\"}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E2\"}"},
            {"SupportBean_B", "{\"id\":\"B1\"}"},
            {"SupportBean", "{\"theString\":\"E3\",\"longBoxed\":30}"},
            {"SupportBean_B", "{\"id\":\"B2\"}"},
            {"SupportBean", "{\"theString\":null,\"longBoxed\":99}"},
            {"SupportBean", "{\"theString\":\"BND\",\"longBoxed\":null}"},
            {"SupportMarketDataBean", "{\"symbol\":null}"},
            {"SupportMarketDataBean", "{\"symbol\":\"BND\"}"}
    };
    private static final String[][] OM_SENDS = {
            {"SupportBean", "{\"theString\":\"E1\",\"longBoxed\":10}"},
            {"SupportBean", "{\"theString\":\"E2\",\"longBoxed\":20}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E1\"}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E1\"}"},
            {"SupportMarketDataBean", "{\"symbol\":\"E2\"}"},
            {"SupportBean", "{\"theString\":\"E3\",\"longBoxed\":30}"},
            {"SupportBean", "{\"theString\":\"E4\",\"longBoxed\":40}"},
            {"SupportBean_B", "{\"id\":\"B1\"}"},
            {"SupportBean_B", "{\"id\":\"E3\"}"},
            {"SupportBean", "{\"theString\":null,\"longBoxed\":99}"},
            {"SupportBean", "{\"theString\":\"BND\",\"longBoxed\":null}"},
            {"SupportBean_B", "{\"id\":null}"},
            {"SupportMarketDataBean", "{\"symbol\":null}"},
            {"SupportMarketDataBean", "{\"symbol\":\"BND\"}"}
    };

    private static final int EXPECTED_STEPS = 63;
    private static final int EXPECTED_RECORDS = 62;

    private InfraNamedWindowOM564ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOM564ScenarioOracle <scenario.json>");
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
     * its own runtime). The internal timer is disabled and the rethrowing
     * exception handler surfaces statement failures to the sender thread;
     * deployed modules accumulate on the path exactly like
     * env.compileDeploy(epl, path). create-table-syntax never touches the
     * runtime — the Java execution is compile-only.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_B.class);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
        List<EPCompiled> path = new ArrayList<>();
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
                        EPDeployment deployment = deploy(runtime, configuration,
                                caseName, label, epl, path);
                        deployments.put(label, deployment);
                        String listener = listenerFor(caseName, label);
                        if (!listener.isEmpty()) {
                            findStatement(deployment, listener).addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.containsKey(label)) {
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
                    case "unrepresentable": {
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String note = string(step, "expectError");
                        assertPinnedUnrepresentable(caseName, label, epl, note);
                        runToEPLAssert(configuration, caseName, label, epl);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "unrepresentable");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", note);
                        records.add(record);
                        break;
                    }
                    case "undeploy":
                        undeployModuleContaining(runtime, string(step, "statement"));
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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
     * Mirrors env.compileDeploy(model, path): compile the pinned statement
     * against the accumulated path and deploy it. The compile leg replays
     * env.eplToModel(epl) -> module compile; the om leg rebuilds the
     * programmatic EPStatementObjectModel of InfraOM and asserts the
     * annotated toEPL rendering equals the pinned deploy text before the
     * model compile (mirroring the assertEquals calls around setAnnotations).
     */
    private static EPDeployment deploy(EPRuntime runtime, Configuration configuration,
                                       String caseName, String label, String epl,
                                       List<EPCompiled> path) throws Exception {
        CompilerArguments arguments = new CompilerArguments(configuration);
        arguments.getPath().getCompileds().addAll(path);
        Module module = new Module();
        EPStatementObjectModel model;
        if ("om".equals(caseName) && !"insert".equals(label)) {
            // env.compileDeploy(model, path): the programmatic OM with its
            // annotations set; the pinned deploy text is the annotated
            // toEPL rendering (mirroring the assertEquals calls around
            // setAnnotations).
            model = omStatementModel(label, true);
            if (!epl.equals(model.toEPL())) {
                throw new IllegalStateException("annotated toEPL does not pin the deploy "
                        + "text for " + label + ": got " + model.toEPL());
            }
        } else {
            // env.eplToModel(epl) -> compileDeploy for the compile leg, and
            // env.eplToModelCompileDeploy(epl, path) for om's insert: parse
            // the pinned text, assert the toEPL round-trip (mirroring
            // eplToModelCompileDeploy's assertEquals), compile the model.
            model = SerializableObjectCopier.copyMayFail(
                    EPCompilerProvider.getCompiler().eplToModel(epl, configuration));
            if (model == null || !epl.equals(model.toEPL())) {
                throw new IllegalStateException("eplToModel round-trip failed for "
                        + caseName + "/" + label);
            }
        }
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, arguments);
        path.add(compiled);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        if ("om".equals(caseName) && "insert".equals(label)
                && (deployment.getStatements().length != 1
                || !epl.equals(deployment.getStatements()[0]
                .getProperty(StatementProperty.EPL)))) {
            throw new IllegalStateException("deployed insert EPL does not pin the text");
        }
        return deployment;
    }

    /**
     * Mirrors one assertEquals(text, model.toEPL()) of the Java executions:
     * the compile leg re-parses the pinned EPL through eplToModel and
     * round-trips it; the om and create-table-syntax legs rebuild the
     * programmatic model (unannotated, except create which asserts the
     * annotated model) and assert toEPL equals the pinned text.
     */
    private static void runToEPLAssert(Configuration configuration, String caseName,
                                       String label, String epl) throws Exception {
        switch (caseName) {
            case "compile": {
                EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(
                        EPCompilerProvider.getCompiler().eplToModel(epl, configuration));
                if (model == null || !epl.equals(model.toEPL())) {
                    throw new IllegalStateException("eplToModel round-trip failed for " + label);
                }
                break;
            }
            case "om": {
                String base = label.substring("toepl-".length());
                if ("select-om".equals(base)) {
                    base = "select";
                } else if ("ondelete".equals(base)) {
                    base = "delete";
                }
                EPStatementObjectModel model = omStatementModel(base, "create".equals(base));
                if (!epl.equals(model.toEPL())) {
                    throw new IllegalStateException("toEPL assert failed for " + label
                            + ": got " + model.toEPL());
                }
                break;
            }
            case "create-table-syntax": {
                EPStatementObjectModel model = createTableModel();
                if (!epl.equals(model.toEPL())) {
                    throw new IllegalStateException("toEPL assert failed for " + label
                            + ": got " + model.toEPL());
                }
                break;
            }
            default:
                throw new IllegalStateException("unexpected unrepresentable case " + caseName);
        }
    }

    /**
     * Rebuilds the programmatic EPStatementObjectModel of InfraOM.java for
     * one statement: create carries @name('create')+@public when annotated,
     * insert parses through the eplToModelCompileDeploy equivalent, select
     * projects key + value*2 over MyWindow(value is not null), ondelete is
     * the correlated market-delete and onselect the correlated SupportBean_B
     * select. Unannotated builds omit every annotation (mirroring where the
     * Java asserts run).
     */
    private static EPStatementObjectModel omStatementModel(String label, boolean annotated)
            throws Exception {
        EPStatementObjectModel model = new EPStatementObjectModel();
        switch (label) {
            case "create": {
                if (annotated) {
                    model.setAnnotations(Arrays.asList(
                            AnnotationPart.nameAnnotation("create"),
                            new AnnotationPart("public")));
                }
                model.setCreateWindow(CreateWindowClause.create("MyWindow")
                        .addView("keepall").setAsEventTypeName("SupportBean"));
                model.setSelectClause(SelectClause.create()
                        .addWithAsProvidedName("theString", "key")
                        .addWithAsProvidedName("longBoxed", "value"));
                return model;
            }
            case "insert": {
                // eplToModelCompileDeploy pins the unannotated insert text;
                // the deploy builds it through eplToModel.
                throw new IllegalStateException("insert has no programmatic model");
            }
            case "select": {
                model.setSelectClause(SelectClause.create()
                        .streamSelector(StreamSelector.RSTREAM_ISTREAM_BOTH)
                        .add("key")
                        .add(Expressions.multiply(Expressions.property("value"),
                                Expressions.constant(2)), "value"));
                model.setFromClause(FromClause.create(
                        FilterStream.create("MyWindow", Expressions.isNotNull("value"))));
                if (annotated) {
                    model.setAnnotations(Collections.singletonList(
                            AnnotationPart.nameAnnotation("select")));
                }
                return model;
            }
            case "delete": {
                model.setOnExpr(OnClause.createOnDelete("MyWindow", "s1"));
                model.setFromClause(FromClause.create(
                        FilterStream.create("SupportMarketDataBean", "s0")));
                model.setWhereClause(Expressions.eqProperty("s0.symbol", "s1.key"));
                if (annotated) {
                    model.setAnnotations(Collections.singletonList(
                            AnnotationPart.nameAnnotation("ondelete")));
                }
                return model;
            }
            case "onselect": {
                model.setOnExpr(OnClause.createOnSelect("MyWindow", "s1"));
                model.setWhereClause(Expressions.eqProperty("s0.id", "s1.key"));
                model.setFromClause(FromClause.create(
                        FilterStream.create("SupportBean_B", "s0")));
                model.setSelectClause(SelectClause.createStreamWildcard("s1"));
                if (annotated) {
                    model.setAnnotations(Collections.singletonList(
                            AnnotationPart.nameAnnotation("onselect")));
                }
                return model;
            }
            default:
                throw new IllegalStateException("no OM statement model for " + label);
        }
    }

    /** The schema-column create-window OM of InfraOMCreateTableSyntax (lines
     * 215-220). */
    private static EPStatementObjectModel createTableModel() {
        EPStatementObjectModel model = new EPStatementObjectModel();
        CreateWindowClause clause = CreateWindowClause.create("MyWindowOM").addView("keepall");
        clause.addColumn(new SchemaColumnDesc("a1", "string"));
        clause.addColumn(new SchemaColumnDesc("a2", "double"));
        clause.addColumn(new SchemaColumnDesc("a3", "int"));
        model.setCreateWindow(clause);
        return model;
    }

    /** Mirrors RegressionEnvironment.undeployModuleContaining: scan the
     * live deployments for the statement name and undeploy its module. */
    private static void undeployModuleContaining(EPRuntime runtime, String statementName)
            throws Exception {
        for (String deploymentId : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deploymentId);
            for (EPStatement statement : info.getStatements()) {
                if (statementName.equals(statement.getName())) {
                    runtime.getDeploymentService().undeploy(deploymentId);
                    return;
                }
            }
        }
        throw new IllegalStateException("no deployment contains statement " + statementName);
    }

    private static String listenerFor(String caseName, String label) {
        for (String[] entry : "compile".equals(caseName) ? COMPILE_DEPLOYS : OM_DEPLOYS) {
            if (entry[0].equals(label)) {
                return entry[2];
            }
        }
        throw new IllegalStateException("unknown deploy label " + caseName + "/" + label);
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("deployment lacks statement named " + name);
    }

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable
     * field order. */
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

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
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

    /**
     * Replays sendSupportBean(theString, longBoxed), the SupportBean_B
     * triggers and sendMarketBean(symbol): each payload field accepts a
     * JSON string, number or literal null, mirroring the boxed Java
     * properties.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                requireFields(payload, "theString", "longBoxed");
                SupportBean bean = new SupportBean();
                bean.setTheString(nullableString(payload.get("theString"), "theString"));
                bean.setLongBoxed(nullableLong(payload.get("longBoxed"), "longBoxed"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_B": {
                requireFields(payload, "id");
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(nullableString(payload.get("id"), "id")), type);
                break;
            }
            case "SupportMarketDataBean": {
                requireFields(payload, "symbol");
                runtime.getEventService().sendEventBean(new SupportMarketDataBean(
                        nullableString(payload.get("symbol"), "symbol"), 0, 0L, ""), type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static String nullableString(JsonValue value, String label) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (!value.isString()) {
            throw new IllegalArgumentException(label + " must be a string or null");
        }
        return value.asString();
    }

    private static Long nullableLong(JsonValue value, String label) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer or null");
        }
        try {
            return Long.parseLong(value.toString(), 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    /** The pinned note every unrepresentable record carries: the asserted
     * toEPL text plus the fixed suffix documenting the SODA surface and its
     * Go mapping. */
    private static String note(String epl) {
        return "soda-EPStatementObjectModel.toEPL: " + epl
                + " - Java builds or parses a statement object model and asserts "
                + "assertEquals(text, model.toEPL()); the Go surface has no "
                + "statement object model or EPL-text parser (the immutable Plan "
                + "is the object model), so the asserted text is pinned verbatim "
                + "with no Go counterpart";
    }

    private static void assertPinnedUnrepresentable(String caseName, String label,
                                                    String epl, String note) {
        String[][] pins = "compile".equals(caseName) ? COMPILE_UNREP : OM_UNREP;
        if ("create-table-syntax".equals(caseName)) {
            if (!"toepl-create-window".equals(label) || !CREATE_TABLE_TEXT.equals(epl)
                    || !note.equals(note(CREATE_TABLE_TEXT))) {
                throw new IllegalStateException("unrepresentable step is not pinned for "
                        + caseName + "/" + label);
            }
            return;
        }
        for (String[] pin : pins) {
            if (pin[0].equals(label)) {
                if (!pin[1].equals(epl) || !note.equals(note(epl))) {
                    throw new IllegalStateException("unrepresentable step is not pinned for "
                            + caseName + "/" + label);
                }
                return;
            }
        }
        throw new IllegalStateException("unknown unrepresentable label " + caseName + "/" + label);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
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
                    || string(definition, "observation").isEmpty()
                    || string(definition, "epl").isEmpty()) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCompileSteps(steps, offset);
        offset = validateOmSteps(steps, offset);
        offset = validateCreateTableSteps(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** Pins the compile-case step sequence (InfraCompile.run): deploy +
     * deployed + the post-deploy toEPL pin for create, the onselect/insert/
     * select deploys with select's post-deploy pin, the E1/E2 sends, the
     * delete deploy + pin, the delete matrix, the B1/B2 on-select probes,
     * the boundary sends and the five per-statement undeploys. */
    private static int validateCompileSteps(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "compile");
        offset = validateDeployTriple(steps, offset, "compile", COMPILE_DEPLOYS[0], "toepl-create");
        offset = validateDeployPair(steps, offset, "compile", COMPILE_DEPLOYS[1]);
        offset = validateDeployPair(steps, offset, "compile", COMPILE_DEPLOYS[2]);
        offset = validateDeployTriple(steps, offset, "compile", COMPILE_DEPLOYS[3], "toepl-select");
        offset = validateSend(steps, offset, "compile", COMPILE_SENDS[0]);
        offset = validateSend(steps, offset, "compile", COMPILE_SENDS[1]);
        offset = validateDeployTriple(steps, offset, "compile", COMPILE_DEPLOYS[4], "toepl-delete");
        for (int index = 2; index <= 4; index++) {
            offset = validateSend(steps, offset, "compile", COMPILE_SENDS[index]);
        }
        offset = validateSend(steps, offset, "compile", COMPILE_SENDS[5]);
        offset = validateSend(steps, offset, "compile", COMPILE_SENDS[6]);
        offset = validateSend(steps, offset, "compile", COMPILE_SENDS[7]);
        for (int index = 8; index <= 11; index++) {
            offset = validateSend(steps, offset, "compile", COMPILE_SENDS[index]);
        }
        for (String label : new String[]{"delete", "onselect", "select", "insert", "create"}) {
            validateUndeploy(steps.get(offset++), "compile", label);
        }
        return offset;
    }

    /** Pins the om-case step sequence (InfraOM.run): each toEPL pin
     * precedes its deploy, insert deploys unnamed, delete and on-select
     * deploy mid-scenario, then the E3/E4 sends, the B1/E3 probes, the
     * boundary sends and undeployAll. */
    private static int validateOmSteps(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "om");
        validateUnrepresentableStep(steps.get(offset++), "om", "toepl-create", EPL_CREATE);
        offset = validateDeployPair(steps, offset, "om", OM_DEPLOYS[0]);
        offset = validateDeployPair(steps, offset, "om", OM_DEPLOYS[1]);
        validateUnrepresentableStep(steps.get(offset++), "om", "toepl-select-om", OM_SELECT_TEXT);
        offset = validateDeployPair(steps, offset, "om", OM_DEPLOYS[2]);
        offset = validateSend(steps, offset, "om", OM_SENDS[0]);
        offset = validateSend(steps, offset, "om", OM_SENDS[1]);
        validateUnrepresentableStep(steps.get(offset++), "om", "toepl-ondelete", OM_DELETE_TEXT);
        offset = validateDeployPair(steps, offset, "om", OM_DEPLOYS[3]);
        for (int index = 2; index <= 4; index++) {
            offset = validateSend(steps, offset, "om", OM_SENDS[index]);
        }
        validateUnrepresentableStep(steps.get(offset++), "om", "toepl-onselect", OM_ONSELECT_TEXT);
        offset = validateDeployPair(steps, offset, "om", OM_DEPLOYS[4]);
        for (int index = 5; index <= 13; index++) {
            offset = validateSend(steps, offset, "om", OM_SENDS[index]);
        }
        validateUndeployAll(steps.get(offset++), "om");
        return offset;
    }

    /** Pins the create-table-syntax case: a single unrepresentable step
     * carrying the schema-column toEPL assert. */
    private static int validateCreateTableSteps(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), "create-table-syntax");
        validateUnrepresentableStep(steps.get(offset++), "create-table-syntax",
                "toepl-create-window", CREATE_TABLE_TEXT);
        return offset;
    }

    private static int validateDeployPair(JsonArray steps, int offset, String caseName,
                                          String[] deploy) {
        validateDeploy(steps.get(offset++), caseName, deploy[0], deploy[1]);
        validateDeployed(steps.get(offset++), caseName, deploy[0]);
        return offset;
    }

    /** deploy + deployed + the post-deploy toEPL pin (compile's assert
     * order). */
    private static int validateDeployTriple(JsonArray steps, int offset, String caseName,
                                            String[] deploy, String unrepLabel) {
        offset = validateDeployPair(steps, offset, caseName, deploy);
        validateUnrepresentableStep(steps.get(offset++), caseName, unrepLabel, deploy[1]);
        return offset;
    }

    private static int validateSend(JsonArray steps, int offset, String caseName,
                                    String[] send) {
        JsonObject step = object(steps.get(offset++), "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !send[0].equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/" + send[0]);
        }
        requireJsonEquals(step.get("payload"), send[1], send[0] + " payload");
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

    /** Pins one unrepresentable step: op, case, label, the asserted EPL
     * and the computed record note. */
    private static void validateUnrepresentableStep(JsonValue value, String caseName,
                                                    String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !note(expectedEpl).equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
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

    /** Order-independent JSON equality against a pinned literal. */
    private static void requireJsonEquals(JsonValue value, String expected, String label) {
        JsonValue parsed = Json.parse(expected);
        if (!jsonEquals(parsed, value)) {
            throw new IllegalArgumentException(label + " payload is not pinned: " + value);
        }
    }

    private static boolean jsonEquals(JsonValue left, JsonValue right) {
        if (left == null || right == null) {
            return left == right;
        }
        if (left.isNull() || right.isNull()) {
            return left.isNull() && right.isNull();
        }
        if (left.isObject() && right.isObject()) {
            JsonObject a = left.asObject();
            JsonObject b = right.asObject();
            if (a.size() != b.size()) {
                return false;
            }
            for (Member member : a) {
                if (!jsonEquals(member.getValue(), b.get(member.getName()))) {
                    return false;
                }
            }
            return true;
        }
        if (left.isArray() && right.isArray()) {
            JsonArray a = left.asArray();
            JsonArray b = right.asArray();
            if (a.size() != b.size()) {
                return false;
            }
            for (int index = 0; index < a.size(); index++) {
                if (!jsonEquals(a.get(index), b.get(index))) {
                    return false;
                }
            }
            return true;
        }
        return left.toString().equals(right.toString());
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
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        try {
            return Integer.parseInt(value.toString(), 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java int range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item.isString()) || !expected[index].equals(item.asString())) {
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
