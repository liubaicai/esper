import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.context.SupportSelectorById;
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
 * Java oracle for EPLOtherFromClauseOptional: source-less (no from-clause)
 * EPL semantics.  Replays five of the six executions on one runtime with
 * undeployAll between cases, mirroring the regression-suite harness:
 * SupportBean, SupportBean_S0 and SupportBean_S1 from esper-common, internal
 * timer disabled, and the rethrowing exception handler so statement failures
 * surface to the sender thread.
 *
 * Ords 0/1 (context-soda-false/context-soda-true) deploy MyContext plus the
 * s0/s1 source-less selects; s0's listener fires at partition initiation and
 * s1's at termination via output-when-terminated, and iterator snapshots
 * yield one row per live partition.  The soda case runs the identical step
 * stream; only the s0/s1 deploys compile through the eplToModel round-trip
 * (env.compileDeploy(soda, epl, path)).  Ord 2 (no-context) deploys
 * 'select 1 as value' and snapshots its single projected row.  Ord 4
 * (faf-context) deploys MyContext plus the count(*) feeder (no listener),
 * initiates two partitions, and runs six fire-and-forget probes covering
 * all-partitions, a SupportSelectorById(1) selector, distinct, and
 * where/having filters.  Ord 5 (invalid) deploys the context then records
 * five compile-error records: the two plain probes compile without the
 * module path (env.tryInvalidCompile), the faf-* probes compile via
 * compileQuery with the runtime path (tryInvalidFAFCompile), and
 * faf-multi-selector compiles successfully then executes with a two-element
 * ContextPartitionSelector array to capture the IllegalArgumentException.
 *
 * Listeners attach to statements named s0/s1 in every case (the runner
 * convention); the feeder and context deployments get none.  ctxs0 renders
 * the initiating SupportBean_S0 as a {"kind":"row"} object over its five
 * properties so Java/Go traces compare field-for-field.
 */
public final class EplOtherFromClauseOptionalScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-from-clause-optional";
    private static final String DESCRIPTION =
            "EPLOtherFromClauseOptional source-less (no from-clause) EPL semantics: initiated/terminated "
                    + "context partitions delivering context.s0 at initiation (s0) and at termination via "
                    + "output-when-terminated (s1) in EPL and SODA compile paths (ords 0/1, byte-identical "
                    + "steps since soda only changes env.compileDeploy(soda, epl, path)); a no-context "
                    + "'select 1 as value' deployed statement whose iterator yields one projected row "
                    + "(ord 2); fire-and-forget source-less queries over live context partitions covering "
                    + "all-partitions, a by-id partition selector, distinct, and where/having filters "
                    + "(ord 4); and the invalid probes (ord 5, intentionally-different) recording the "
                    + "pinned Java message prefixes for a source-less subselect, wildcard select and FAF "
                    + "wildcard, a multi-selector FAF execute rejection, and context+order-by FAF compile "
                    + "rejection. Deploy attaches listeners only to statements named s0/s1 (the count(*) "
                    + "feeder in faf-context deploys without listener semantics). The faf-multi-selector "
                    + "build-error step is special: the oracle compiles the FAF successfully, then executes "
                    + "with a two-element ContextPartitionSelector array and records the thrown "
                    + "IllegalArgumentException; the Go runner pins its own nearest boundary. The two "
                    + "plain-compile probes (subselect-no-from, wildcard) carry compileWithoutPath mirroring "
                    + "env.tryInvalidCompile's path-less compile; the faf-* probes compile via compileQuery "
                    + "with the runtime path mirroring tryInvalidFAFCompile.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/"
                    + "EPLOtherFromClauseOptional.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-0e96acf48376ed71c690",
            "java-runtime-f54b77f9c8381d0cc12c",
            "java-runtime-00d22c5518b7c57f1ba7",
            "java-runtime-4f6e15a0c30a5e95b1f6",
            "java-runtime-6d948697a80bca6a0dac"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherFromOptionalContext{soda=false}",
            "EPLOtherFromOptionalContext{soda=true}",
            "EPLOtherFromOptionalNoContext",
            "EPLOtherFromOptionalFAFContext",
            "EPLOtherFromOptionalInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-0c9a8913a4dafbcd91e2",
            "java-0c9a8913a4dafbcd91e2",
            "java-0c9a8913a4dafbcd91e2",
            "java-0c9a8913a4dafbcd91e2",
            "java-0c9a8913a4dafbcd91e2"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET", "INVALIDITY"};
    private static final String[] CASES = {
            "context-soda-false", "context-soda-true", "no-context", "faf-context", "invalid"
    };
    private static final int[] ORDINALS = {0, 1, 2, 4, 5};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+snapshot; s0 fires ctxs0=<initiating S0> at partition initiation and s1 fires at "
                    + "termination via output-when-terminated; iterators yield one row per live partition",
            "listener+snapshot; identical steps to context-soda-false; soda only changes the Java compile "
                    + "path (eplToModel round-trip for the s0/s1 deploys)",
            "snapshot; deployed 'select 1 as value' yields exactly one row {value:1} from its iterator "
                    + "with no events sent",
            "faf; source-less FAF over two live partitions yields one row per partition, honors a by-id "
                    + "selector, distinct, and where/having filters",
            "compile-error; source-less subselect, wildcard select/FAF, multi-selector FAF execute "
                    + "(compileFAF succeeds, execute with two selectors throws), and context+order-by FAF "
                    + "probes record the pinned Java message prefixes"
    };

    // Verbatim transcriptions of EPLOtherFromClauseOptional lines 56, 58-59,
    // 61-62, 146, 156, 160, 163-164, 167, 176-177, 190-192, 196, 201, 211,
    // 216-218.
    private static final String EPL_CONTEXT =
            "@public create context MyContext initiated by SupportBean_S0 as s0 "
                    + "terminated by SupportBean_S1(id=s0.id)";
    private static final String EPL_CONTEXT_SEMI = EPL_CONTEXT + ";";
    private static final String EPL_S0 =
            "@name('s0') context MyContext select context.s0 as ctxs0";
    private static final String EPL_S1 =
            "@name('s1') context MyContext select context.s0 as ctxs0 output when terminated";
    private static final String EPL_NO_CONTEXT = "@name('s0') select 1 as value";
    private static final String EPL_FEEDER = "context MyContext select count(*) from SupportBean";
    private static final String EPL_FAF_ALL = "context MyContext select context.s0.p00 as id";
    private static final String EPL_FAF_DISTINCT =
            "context MyContext select distinct context.s0.p01 as p01";
    private static final String EPL_FAF_WHERE_FALSE =
            "context MyContext select 1 as value where 'a'='b'";
    private static final String EPL_FAF_WHERE_ID =
            "context MyContext select context.s0.p00 as value where context.s0.id=10";
    private static final String EPL_FAF_HAVING_ID =
            "context MyContext select context.s0.p00 as value having context.s0.id=10";
    private static final String EPL_SUBSELECT = "select (select 1)";
    private static final String EPL_WILDCARD = "select *";
    private static final String EPL_FAF_ORDER_BY =
            "context MyContext select context.s0.p00 as p00 order by p00 desc";

    private static final String ERR_SUBSELECT = "Incorrect syntax near ')'";
    private static final String ERR_WILDCARD =
            "Wildcard cannot be used when the from-clause is not provided";
    private static final String ERR_MULTI_SELECTOR =
            "Fire-and-forget queries without a from-clause allow only a single context partition selector";
    private static final String ERR_ORDER_BY =
            "Fire-and-forget queries without a from-clause and with context do not allow order-by";

    private static final String[] CASE_EPLS = {
            EPL_CONTEXT, EPL_CONTEXT, EPL_NO_CONTEXT,
            EPL_CONTEXT_SEMI + "\n" + EPL_FEEDER + ";\n", EPL_CONTEXT_SEMI
    };

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("s0", "s1"));
    private static final String SODA_CASE = "context-soda-true";
    private static final String MULTI_SELECTOR_STATEMENT = "faf-multi-selector";

    private static final int EXPECTED_STEPS = 56;
    private static final int EXPECTED_RECORDS = 34;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * send payloads render as their compact JSON.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        String[] contextSteps = {
                "deploy|context-soda-false|ctx||" + EPL_CONTEXT + "|||||||",
                "deploy|context-soda-false|s0||" + EPL_S0 + "|||||||",
                "deploy|context-soda-false|s1||" + EPL_S1 + "|||||||",
                "send|context-soda-false||SupportBean_S0||{\"id\":10,\"p00\":\"A\",\"p01\":null,\"p02\":null,\"p03\":null}||||||",
                "snapshot|context-soda-false|s0||||||ordered|||ctxs0",
                "send|context-soda-false||SupportBean_S0||{\"id\":20,\"p00\":\"B\",\"p01\":null,\"p02\":null,\"p03\":null}||||||",
                "snapshot|context-soda-false|s0||||||ordered|||ctxs0",
                "snapshot|context-soda-false|s1||||||ordered|||ctxs0",
                "send|context-soda-false||SupportBean_S1||{\"id\":10,\"p10\":\"A\",\"p11\":null,\"p12\":null,\"p13\":null}||||||",
                "snapshot|context-soda-false|s0||||||ordered|||ctxs0",
                "snapshot|context-soda-false|s1||||||ordered|||ctxs0",
                "send|context-soda-false||SupportBean_S1||{\"id\":20,\"p10\":\"A\",\"p11\":null,\"p12\":null,\"p13\":null}||||||",
                "snapshot|context-soda-false|s0||||||ordered|||ctxs0",
                "snapshot|context-soda-false|s1||||||ordered|||ctxs0",
                "undeploy-all|context-soda-false||||||||||",
        };
        CASE_STEPS.put("context-soda-false", contextSteps);
        String[] sodaSteps = contextSteps.clone();
        for (int i = 0; i < sodaSteps.length; i++) {
            sodaSteps[i] = sodaSteps[i].replace("context-soda-false", "context-soda-true");
        }
        CASE_STEPS.put("context-soda-true", sodaSteps);
        CASE_STEPS.put("no-context", new String[]{
                "deploy|no-context|s0||" + EPL_NO_CONTEXT + "|||||||",
                "snapshot|no-context|s0||||||ordered|||value",
                "undeploy-all|no-context||||||||||",
        });
        CASE_STEPS.put("faf-context", new String[]{
                "deploy|faf-context|ctx||" + EPL_CONTEXT + "|||||||",
                "deploy|faf-context|feeder||" + EPL_FEEDER + "|||||||",
                "send|faf-context||SupportBean_S0||{\"id\":10,\"p00\":\"A\",\"p01\":\"x\",\"p02\":null,\"p03\":null}||||||",
                "send|faf-context||SupportBean_S0||{\"id\":20,\"p00\":\"B\",\"p01\":\"x\",\"p02\":null,\"p03\":null}||||||",
                "faf|faf-context|faf-all||" + EPL_FAF_ALL + "|||||||id",
                "faf|faf-context|faf-by-id||" + EPL_FAF_ALL + "|||||ids|[1]|id",
                "faf|faf-context|faf-distinct||" + EPL_FAF_DISTINCT + "|||||||p01",
                "faf|faf-context|faf-where-false||" + EPL_FAF_WHERE_FALSE + "|||||||value",
                "faf|faf-context|faf-where-id||" + EPL_FAF_WHERE_ID + "|||||||value",
                "faf|faf-context|faf-having-id||" + EPL_FAF_HAVING_ID + "|||||||value",
                "undeploy-all|faf-context||||||||||",
        });
        CASE_STEPS.put("invalid", new String[]{
                "deploy|invalid|ctx||" + EPL_CONTEXT_SEMI + "|||||||",
                "build-error|invalid|subselect-no-from||" + EPL_SUBSELECT + "||" + ERR_SUBSELECT + "|1||||",
                "build-error|invalid|wildcard||" + EPL_WILDCARD + "||" + ERR_WILDCARD + "|1||||",
                "build-error|invalid|faf-wildcard||" + EPL_WILDCARD + "||" + ERR_WILDCARD + "|||||",
                "build-error|invalid|faf-multi-selector||" + EPL_FAF_ALL + "||" + ERR_MULTI_SELECTOR + "|||||",
                "build-error|invalid|faf-order-by||" + EPL_FAF_ORDER_BY + "||" + ERR_ORDER_BY + "|||||",
                "undeploy-all|invalid||||||||||",
        });
    }

    private EplOtherFromClauseOptionalScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EplOtherFromClauseOptionalScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASES) {
                runCase(caseName, configuration, runtime, allSteps, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
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

    /**
     * Replays one case's steps on the shared runtime; sequences restart per
     * case.  Deploys register their statement under the step label so
     * snapshots can resolve it; listeners attach to statements named s0/s1
     * (the runner convention).  Deploys in the soda case compile the s0/s1
     * EPL through the eplToModel round-trip, mirroring
     * env.compileDeploy(soda, epl, path).
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
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
                    boolean soda = SODA_CASE.equals(caseName)
                            && LISTENED_STATEMENTS.contains(label);
                    EPCompiled compiled = soda
                            ? compileSoda(epl, configuration, runtime)
                            : compileModule(epl, configuration, runtime);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != 1) {
                        throw new IllegalStateException("deployment of " + label + " has "
                                + deployed.length + " statements, want 1");
                    }
                    EPStatement statement = deployed[0];
                    if (LISTENED_STATEMENTS.contains(statement.getName())) {
                        statement.addListener(listener(caseName, sequences, records, runtime));
                    }
                    statements.put(label, statement);
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
                    for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
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
                case "faf": {
                    // Fire-and-forget mirroring RegressionEnvironmentBase
                    // .compileExecuteFAF: compileQuery with the runtime path,
                    // then executeQuery (with a by-id selector when the step
                    // carries one).
                    EPCompiled query = compileFaf(string(step, "epl"), configuration, runtime);
                    EventBean[] result;
                    if ("ids".equals(string(step, "selector"))) {
                        int[] ids = intArray(step.get("ids"), "ids");
                        Set<Integer> idSet = new HashSet<>();
                        for (int id : ids) {
                            idSet.add(id);
                        }
                        result = runtime.getFireAndForgetService().executeQuery(query,
                                new ContextPartitionSelector[]{new SupportSelectorById(idSet)})
                                .getArray();
                    } else {
                        result = runtime.getFireAndForgetService().executeQuery(query).getArray();
                    }
                    String[] fields = stringArray(step.get("fields"), "fields");
                    JsonArray rows = new JsonArray();
                    if (result != null) {
                        for (EventBean row : result) {
                            rows.add(projectedRow(row, fields));
                        }
                    }
                    int sequence = sequences.merge("faf", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "faf");
                    record.add("statement", string(step, "statement"));
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "build-error":
                    buildErrorStep(runtime, configuration, caseName, step, records);
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        runtime.getDeploymentService().undeployAll();
        statements.clear();
    }

    /**
     * Module compile with the runtime path, mirroring
     * RegressionEnvironmentBase.compileDeploy(epl, path): the compiler sees
     * the configuration plus the already-deployed module path.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * soda=true compile path: EPL to model, serializability copy, byte-exact
     * toEPL round-trip guard, then compile the model (precedent
     * EPLFromClauseMethodVariableScenarioOracle.deployMaybeSoda).
     */
    private static EPCompiled compileSoda(String epl, Configuration configuration,
                                          EPRuntime runtime) throws Exception {
        EPStatementObjectModel model = EPCompilerProvider.getCompiler()
                .eplToModel(epl, configuration);
        model = SerializableObjectCopier.copyMayFail(model);
        if (!epl.equals(model.toEPL())) {
            throw new IllegalStateException("soda toEPL round-trip mismatch: expected [" + epl
                    + "] got [" + model.toEPL() + "]");
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        return EPCompilerProvider.getCompiler().compile(module, compilerArgs);
    }

    /**
     * FAF compile with the runtime path, mirroring
     * RegressionEnvironmentBase.compileFAF / SupportMessageAssertUtil
     * .compileFAFInternal.
     */
    private static EPCompiled compileFaf(String epl, Configuration configuration,
                                         EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compileQuery(epl, compilerArgs);
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics).  Steps whose statement starts with "faf-" compile via
     * compileQuery with the runtime path (tryInvalidFAFCompile); the
     * faf-multi-selector step compiles successfully then executes with a
     * two-element ContextPartitionSelector array, mirroring the Java
     * execution's IllegalArgumentException capture.  Steps carrying
     * compileWithoutPath compile without the module path
     * (env.tryInvalidCompile).
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        if (MULTI_SELECTOR_STATEMENT.equals(label)) {
            EPCompiled compiled = compileFaf(epl, configuration, runtime);
            try {
                runtime.getFireAndForgetService().executeQuery(compiled,
                        new ContextPartitionSelector[2]);
                caught = "<no-error>";
            } catch (IllegalArgumentException ex) {
                caught = ex.getMessage();
            }
        } else {
            try {
                if (label.startsWith("faf-")) {
                    compileFaf(epl, configuration, runtime);
                } else {
                    CompilerArguments compilerArgs = new CompilerArguments(configuration);
                    if (!step.getBoolean("compileWithoutPath", false)) {
                        compilerArgs.getPath().add(runtime.getRuntimePath());
                    }
                    EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                }
                caught = "<no-error>";
            } catch (Exception ex) {
                caught = ex.getMessage();
            }
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
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

    /** Row projected to exactly the fields the step's assertions read. */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
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
     * {"state":"null"} object.  EventBean values and the SupportBean_S0
     * initiating-event underlying render as {"kind":"row","fields":{...}}
     * over their sorted properties so the context.s0 column compares
     * field-for-field with the Go initiating-event rendering.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean eventBean) {
            JsonObject fields = new JsonObject();
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(eventBean.get(name)));
            }
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fields);
            return row;
        }
        if (value instanceof SupportBean_S0 bean) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
            fields.add("p00", normalize(bean.getP00()));
            fields.add("p01", normalize(bean.getP01()));
            fields.add("p02", normalize(bean.getP02()));
            fields.add("p03", normalize(bean.getP03()));
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fields);
            return row;
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
                SupportBean bean = new SupportBean();
                bean.setTheString(nullableString(payload, "theString"));
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null && !intPrimitive.isNull()) {
                    bean.setIntPrimitive((int) longInteger(intPrimitive, "intPrimitive"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 bean = new SupportBean_S0(
                        (int) longInteger(payload.get("id"), "id"),
                        nullableString(payload, "p00"),
                        nullableString(payload, "p01"),
                        nullableString(payload, "p02"),
                        nullableString(payload, "p03"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S1": {
                SupportBean_S1 bean = new SupportBean_S1(
                        (int) longInteger(payload.get("id"), "id"),
                        nullableString(payload, "p10"),
                        nullableString(payload, "p11"),
                        nullableString(payload, "p12"),
                        nullableString(payload, "p13"));
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

    private static int[] intArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        int[] result = new int[items.size()];
        for (int index = 0; index < items.size(); index++) {
            result[index] = (int) longInteger(items.get(index), label);
        }
        return result;
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
