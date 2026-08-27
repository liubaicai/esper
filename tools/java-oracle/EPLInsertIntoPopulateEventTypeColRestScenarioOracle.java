import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Oracle for the epl.insert-into-populate-eventcol remainder work unit:
 * EPLInsertIntoPopulateEventTypeColumnBean ordinals 8-11 and
 * EPLInsertIntoPopulateEventTypeColumnNonBean ordinals 8-15, pinned to the
 * regression-lib executions verbatim (EPLInsertIntoPopulateEventTypeColumnBean
 * .java:45-213 and EPLInsertIntoPopulateEventTypeColumnNonBean.java:54-244
 * plus :201-217).
 *
 * Runtime IDs cited from testdata/compat/java-execution-inventory.jsonl:
 * - bean-singletomulti           java-runtime-5a7dbebf11b467444d22 (bean ord8)
 * - bean-multitosingle           java-runtime-d9b52924796472db83cb (bean ord9)
 * - bean-invalid                 java-runtime-10c2cd110438e2422b62 (bean ord10)
 * - bean-context-prop            java-runtime-63b9e9149d6ec7890aba (bean ord11)
 * - nonbean-new-doc-objectarray  java-runtime-6883c88a8250fd587c89 (ord8)
 * - nonbean-new-doc-map          java-runtime-7f2273f9d9a40f6c3dc2 (ord9)
 * - nonbean-case-new-map         java-runtime-2ea417f6f55b2d3543ad (ord10)
 * - nonbean-case-new-objectarray java-runtime-ba3f88c4715292f3432e (ord11)
 * - nonbean-case-new-json        java-runtime-b121584c40a08d48229d (ord12)
 * - nonbean-singlecol-named-window java-runtime-97ac55e0199723988adc (ord13)
 * - nonbean-single-to-multi      java-runtime-f8d3d61670e7dae24003 (ord14)
 * - nonbean-invalid              java-runtime-05f38146a2f28887769c (ord15)
 *
 * Compilation mirrors env.compileDeploy(epl, path): every case carries one
 * RegressionPath-shaped {@code List<EPCompiled>} whose members join the
 * CompilerArguments module path exactly like getArgsWithExportToPath. The
 * deployed-runtime path must NOT be used: verification against the live
 * runtime path skips the insert-into route diagnostics this unit pins
 * (measured: with getRuntimePath the B10/N15 shapes compile and deploy,
 * with the module path they reject with the pinned prefixes). Each case
 * runs in a fresh runtime "parity-eventcol-rest-&lt;case&gt;" initialized at
 * epoch zero with the internal timer disabled, so trace times read
 * "1970-01-01T00:00:00Z".
 *
 * Milestones sit between sends inside replay order and observe nothing
 * themselves; iterator assertions that follow them in the pinned suite are
 * emitted as trailing "snapshot" records (B8/B9/N14).
 *
 * Identity notes (not representable in the value-only trace; asserted
 * Go-side and cited in evidence notes): bean-singletomulti,
 * bean-multitosingle, bean-context-prop and nonbean-single-to-multi pin
 * underlying identity (assertSame/getUnderlying; suite lines 96/98, 75,
 * 58, 68). Bean-typed columns surface the raw SupportBean underlying on
 * direct get(); the trace renders its pinned value projection.
 *
 * Record protocol (esper-parity/v1): rows carry sorted property names,
 * nested fragments render {__type:&lt;event type name&gt;,&lt;fields...&gt;}
 * sorted, null renders {"state":"null"} and integral doubles gain a ".0"
 * suffix via the Go javaDoubleString convention (10d -> "10.0").
 */
public final class EPLInsertIntoPopulateEventTypeColRestScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "insertinto-eventcol-col-rest";

    /** Inventory/frozen-contract order; compile-invalid units sit with their family. */
    private static final String[] CASES = {
        "bean-singletomulti",
        "bean-multitosingle",
        "bean-context-prop",
        "bean-invalid",
        "nonbean-new-doc-objectarray",
        "nonbean-new-doc-map",
        "nonbean-case-new-map",
        "nonbean-case-new-objectarray",
        "nonbean-case-new-json",
        "nonbean-singlecol-named-window",
        "nonbean-single-to-multi",
        "nonbean-invalid",
    };

    /** Compile-phase units: no scenario sends, recorded via compile-rejected. */
    private static final java.util.Set<String> SENDLESS_CASES = new java.util.HashSet<>(
            java.util.Arrays.asList("bean-invalid", "nonbean-invalid"));

    private EPLInsertIntoPopulateEventTypeColRestScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1 && args.length != 2) {
            throw new IllegalArgumentException(
                "usage: EPLInsertIntoPopulateEventTypeColRestScenarioOracle <scenario.json> [caseFilter]");
        }
        String caseFilter = args.length >= 2 ? args[1] : null;
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unexpected scenario id " + scenario.getString("id", ""));
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        for (String caseName : CASES) {
            if (caseFilter != null && !caseFilter.isEmpty() && !caseFilter.equals(caseName)) {
                continue;
            }
            // Compile-invalid units carry no sends, so no scenario segment
            // gates them: they always run (also reachable via caseFilter).
            if (!SENDLESS_CASES.contains(caseName) && !hasCase(steps, caseName)) {
                continue;
            }
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    /** Ordered steps of one op kind inside one case marker segment. */
    private static List<JsonObject> caseStepsOfOp(JsonArray allSteps, String caseName, String op) {
        List<JsonObject> found = new ArrayList<>();
        boolean active = false;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String stepOp = step.getString("op", "");
            if ("case".equals(stepOp)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (active && op.equals(stepOp)) {
                found.add(step);
            }
        }
        return found;
    }

    private static void runCase(JsonArray steps, String caseName, JsonArray records) throws Exception {
        switch (caseName) {
            case "bean-singletomulti":
                runBeanSingleToMulti(caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "bean-multitosingle":
                runBeanMultiToSingle(caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "bean-context-prop":
                runBeanContextProp(caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "bean-invalid":
                runBeanInvalid(records);
                break;
            case "nonbean-new-doc-objectarray":
                runNewOperatorDocSample(caseName, "objectarray", caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-new-doc-map":
                runNewOperatorDocSample(caseName, "map", caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-case-new-map":
                runCaseNew(caseName, Rep.MAP, caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-case-new-objectarray":
                runCaseNew(caseName, Rep.OBJECTARRAY, caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-case-new-json":
                runCaseNew(caseName, Rep.JSONCLASSPROVIDED, caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-singlecol-named-window":
                runSingleColNamedWindow(caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-single-to-multi":
                runNonBeanSingleToMulti(caseStepsOfOp(steps, caseName, "send"), records);
                break;
            case "nonbean-invalid":
                runNonBeanInvalid(records);
                break;
            default:
                throw new IllegalArgumentException("unknown case " + caseName);
        }
    }

    /**
     * Fresh zero-clock configuration carrying the support beans as
     * preconfigured types (mirrors the engine defaults the pinned runner
     * configures) plus the regression factory quirks relevant here.
     */
    private static Configuration newConfiguration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        configuration.getCommon().addEventType("SupportBean_S1", SupportBean_S1.class);
        return configuration;
    }

    /** Fresh runtime "parity-eventcol-rest-<case>" pinned to epoch zero. */
    private static EPRuntime newRuntime(String caseName, Configuration configuration) {
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-eventcol-rest-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        return runtime;
    }

    /**
     * Mirrors env.compileDeploy(epl, path): compile against the
     * configuration plus the accumulated EPCompiled module path, add the
     * result to the path (path.add), then deploy to the runtime.
     */
    private static EPDeployment compileDeploy(Configuration configuration, List<EPCompiled> path,
                                              EPRuntime runtime, String epl, String deploymentId) throws Exception {
        CompilerArguments arguments = compilerArguments(configuration, path);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, arguments);
        path.add(compiled);
        return runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId));
    }

    /** getArgsWithExportToPath: configuration plus every prior path member. */
    private static CompilerArguments compilerArguments(Configuration configuration, List<EPCompiled> path) {
        CompilerArguments arguments = new CompilerArguments(configuration);
        arguments.getPath().getCompileds().addAll(path);
        return arguments;
    }

    /**
     * Mirrors env.tryInvalidCompile(path, epl, message): rejects unless the
     * EPCompileException message starts with {@code prefix} (exact assertMessage
     * semantics; the tail may carry the rendered module text in brackets).
     */
    private static void tryInvalidCompile(Configuration configuration, List<EPCompiled> path,
                                          String epl, String prefix) {
        try {
            EPCompilerProvider.getCompiler().compile(epl, compilerArguments(configuration, path));
        } catch (EPCompileException ex) {
            String message = ex.getMessage();
            if (message == null || !message.startsWith(prefix)) {
                throw new IllegalStateException(
                    "expected error prefix <" + prefix + "> for <" + epl + "> got <" + message + ">", ex);
            }
            return;
        }
        throw new IllegalStateException("expected compile failure for <" + epl + ">");
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("deployment has no statement named " + name);
    }

    /**
     * Sends one scenario step. Payload projections reproduce the pinned
     * constructors: SupportBean(theString, intPrimitive),
     * SupportBean_S0(id, p00, p01), bare map sends for TriggerEvent /
     * AEvent / EventA exactly like env.sendEventMap.
     */
    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean bean = new SupportBean(
                        payload.getString("theString", null),
                        payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBean_S0" -> {
                SupportBean_S0 bean = new SupportBean_S0(
                        payload.getInt("id", 0),
                        payload.getString("p00", null),
                        payload.getString("p01", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "TriggerEvent" -> runtime.getEventService().sendEventMap(new LinkedHashMap<>(), eventType);
            case "AEvent" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("symbol", payload.getString("symbol", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "EventA" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getString("id", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            default -> throw new IllegalArgumentException("unsupported event type " + eventType);
        }
    }

    /**
     * B8 EPLInsertIntoColBeanSingleToMulti. Three module deployments on one
     * RegressionPath; s0 rides EventOne#keepall. sbarr renders as an array
     * of SupportBean value projections; identity asserted Go-side.
     */
    private static void runBeanSingleToMulti(List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 1) {
            throw new IllegalArgumentException("bean-singletomulti needs exactly one send");
        }
        String caseName = "bean-singletomulti";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            int deploy = 0;
            compileDeploy(configuration, path, runtime,
                    "@public create schema EventOne(sbarr SupportBean[])",
                    caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                    "insert into EventOne select maxby(intPrimitive) as sbarr from SupportBean as sb",
                    caseName + "-" + deploy++);
            EPDeployment s0Deployment = compileDeploy(configuration, path, runtime,
                    "@name('s0') select * from EventOne#keepall",
                    caseName + "-" + deploy);
            EPStatement s0 = findStatement(s0Deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            // send SB(E1,1) -> one listener record; milestone(0);
            // assertPropsPerRowIterator(s0, sbarr[0].theString) {"E1"} snapshot.
            replaySends(runtime, sends);
            writer.appendSnapshot("s0", s0);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * B9 EPLInsertIntoColBeanMultiToSingle. The insert fires on S0; the
     * one-row SupportBean#keepall window supplies the sb column (identity
     * asserted Go-side); one trailing keepall iteration snapshot.
     */
    private static void runBeanMultiToSingle(List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 2) {
            throw new IllegalArgumentException("bean-multitosingle needs exactly two sends");
        }
        String caseName = "bean-multitosingle";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            int deploy = 0;
            compileDeploy(configuration, path, runtime,
                    "@public create schema EventOne(sb SupportBean)",
                    caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                    "insert into EventOne select (select * from SupportBean#keepall) as sb from SupportBean_S0",
                    caseName + "-" + deploy++);
            EPDeployment s0Deployment = compileDeploy(configuration, path, runtime,
                    "@name('s0') select * from EventOne#keepall",
                    caseName + "-" + deploy);
            EPStatement s0 = findStatement(s0Deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            // SB(E1,1); S0(1) -> one listener record; milestone(0); snapshot {"E1"}.
            replaySends(runtime, sends);
            writer.appendSnapshot("s0", s0);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * B11 EPLInsertIntoColBeanContextProp, no milestone, single module. The
     * context frame initiates on the SupportBean send; the S0 delivery
     * carries context.sb into OutStream.col.
     */
    private static void runBeanContextProp(List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 2) {
            throw new IllegalArgumentException("bean-context-prop needs exactly two sends");
        }
        String caseName = "bean-context-prop";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            String epl = "create context MyContext initiated by SupportBean as sb;" +
                "create schema OutStream(col SupportBean);" +
                "context MyContext on SupportBean_S0 " +
                "  insert into OutStream select context.sb as col;" +
                "@name('s0') select * from OutStream;";
            EPDeployment deployment = compileDeploy(configuration, path, runtime, epl, caseName + "-0");
            EPStatement s0 = findStatement(deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            replaySends(runtime, sends);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * B10 EPLInsertIntoColBeanInvalid, compile-phase only. Both rejected
     * attempts become {"operation":"compile-rejected"} records whose
     * errorPrefix equals the pinned message prefix (startsWith semantics,
     * identical to SupportMessageAssertUtil.assertMessage).
     */
    private static void runBeanInvalid(JsonArray records) throws Exception {
        String caseName = "bean-invalid";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            String prefix = "Incompatible type detected attempting to insert into column 'sbs' type '" +
                SupportBean.class.getName() + "' compared to selected type 'SupportBean_S0'";
            String epl = "insert into TypeOne select (select * from SupportBean_S0#keepall) as sbs from SupportBean_S1";

            compileDeploy(configuration, path, runtime,
                    "@public create schema TypeOne(sbs SupportBean[])", caseName + "-0");
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            writer.appendCompileRejected("TypeOne", epl, prefix, configuration, path);

            compileDeploy(configuration, path, runtime,
                    "@public create schema TypeTwo(sbs SupportBean)", caseName + "-1");
            writer.appendCompileRejected("TypeTwo", epl, prefix, configuration, path);

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * N8/N9 EPLInsertIntoColNonBeanNewOperatorDocSample. Five module
     * deployments chained over one path; s0 records both firings (the
     * second trigger is part of the observable stream), items[i] fragments
     * render as Item rows with ".0"-suffixed prices.
     */
    private static void runNewOperatorDocSample(String caseName, String typeType,
                                                List<JsonObject> sends, JsonArray records) throws Exception {
        if (!typeType.equals("objectarray") && !typeType.equals("map")) {
            throw new IllegalArgumentException("unsupported typeType " + typeType);
        }
        if (sends.size() != 2) {
            throw new IllegalArgumentException(caseName + " needs exactly two sends");
        }
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            int deploy = 0;
            compileDeploy(configuration, path, runtime,
                "@public create " + typeType + " schema Item(name string, price double)",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@public create " + typeType + " schema PurchaseOrder(orderId string, items Item[])",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@public @buseventtype create schema TriggerEvent()",
                caseName + "-" + deploy++);
            EPDeployment s0Deployment = compileDeploy(configuration, path, runtime,
                "@name('s0') insert into PurchaseOrder select '001' as orderId, new {name= 'i1', price=10} as items from TriggerEvent",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@name('s1') select * from PurchaseOrder#keepall",
                caseName + "-" + deploy);
            EPStatement s0 = findStatement(s0Deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            // Trigger -> record; milestone(0); Trigger -> record (no assertions pinned).
            replaySends(runtime, sends);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * N10-N12 EPLInsertIntoColNonBeanCaseNew over map/objectarray/
     * json-provided representations. Schema annotations mirror
     * EventRepresentationChoice.getAnnotationTextWJsonProvided; out records
     * b/2 then a/1 across the silent milestone.
     */
    private static void runCaseNew(String caseName, Rep rep,
                                   List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 2) {
            throw new IllegalArgumentException(caseName + " needs exactly two sends");
        }
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            int deploy = 0;
            compileDeploy(configuration, path, runtime,
                annotationWJsonProvided(rep, "MyLocalJsonProvidedNested") +
                    "@public create schema Nested(p0 string, p1 int)",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                annotationWJsonProvided(rep, "MyLocalJsonProvidedOuterType") +
                    "@public create schema OuterType(n0 Nested)",
                caseName + "-" + deploy++);

            String epl = "@Name('out') " +
                "expression computeNested {\n" +
                "  sb => case\n" +
                "  when intPrimitive = 1 \n" +
                "    then new { p0 = 'a', p1 = 1}\n" +
                "  else new { p0 = 'b', p1 = 2 }\n" +
                "  end\n" +
                "}\n" +
                "insert into OuterType select computeNested(sb) as n0 from SupportBean as sb;\n" +
                "@name('s1') select * from OuterType#keepall;\n";
            EPDeployment deployment = compileDeploy(configuration, path, runtime, epl, caseName + "-" + deploy);
            EPStatement out = findStatement(deployment, "out");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            out.addListener(writer);

            replaySends(runtime, sends);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * N13 EPLInsertIntoColNonBeanSingleColNamedWindow. Producer1 routes the
     * last-event subquery into the named-window column e; producer2 (s0,
     * listened) projects (select e from MyEventWindow) into BEvent.e. Only
     * the B-side firing records; e renders as an AEvent fragment.
     */
    private static void runSingleColNamedWindow(List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 3) {
            throw new IllegalArgumentException("nonbean-singlecol-named-window needs exactly three sends");
        }
        String caseName = "nonbean-singlecol-named-window";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            int deploy = 0;
            compileDeploy(configuration, path, runtime,
                "@public @buseventtype create schema AEvent (symbol string)",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@public create window MyEventWindow#lastevent (e AEvent)",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "insert into MyEventWindow select (select * from AEvent#lastevent) as e from SupportBean(theString = 'A')",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@public create schema BEvent (e AEvent)",
                caseName + "-" + deploy++);
            EPDeployment s0Deployment = compileDeploy(configuration, path, runtime,
                "@name('s0') insert into BEvent select (select e from MyEventWindow) as e from SupportBean(theString = 'B')",
                caseName + "-" + deploy++);
            compileDeploy(configuration, path, runtime,
                "@name('s1') select * from BEvent#keepall",
                caseName + "-" + deploy);
            EPStatement s0 = findStatement(s0Deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            // Sends: AEvent{GE}, SB(A,1); milestone(0); SB(B,2) -> one record.
            replaySends(runtime, sends);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * N14 EPLInsertIntoColNonBeanSingleToMulti. One batch module carries
     * schema + bare consumer + producer + s0 consumer together (compiled
     * and deployed once). aArray renders as EventA fragment array and the
     * keepall iteration closes the case as a snapshot record.
     */
    private static void runNonBeanSingleToMulti(List<JsonObject> sends, JsonArray records) throws Exception {
        if (sends.size() != 1) {
            throw new IllegalArgumentException("nonbean-single-to-multi needs exactly one send");
        }
        String caseName = "nonbean-single-to-multi";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            String epl = "@public @buseventtype create schema EventA(id string);\n" +
                    "select * from EventA#keepall;\n" +
                    "@public create schema EventB(aArray EventA[]);\n" +
                    "insert into EventB select maxby(id) as aArray from EventA;\n" +
                    "@name('s0') select * from EventB#keepall;\n";
            EPDeployment deployment = compileDeploy(configuration, path, runtime, epl, caseName + "-0");
            EPStatement s0 = findStatement(deployment, "s0");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            s0.addListener(writer);

            // Send {id=x1} -> record; milestone(0); snapshot aArray[0].id=x1.
            replaySends(runtime, sends);
            writer.appendSnapshot("s0", s0);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * N15 EPLInsertIntoColNonBeanBeanInvalid, compile-phase only. The two
     * StructOf-typed new{} routing rejects record in pinned order with
     * their exact message prefixes.
     */
    private static void runNonBeanInvalid(JsonArray records) throws Exception {
        String caseName = "nonbean-invalid";
        Configuration configuration = newConfiguration();
        EPRuntime runtime = newRuntime(caseName, configuration);
        List<EPCompiled> path = new ArrayList<>();
        try {
            compileDeploy(configuration, path, runtime,
                    "@public create schema N1_1(p0 int)", caseName + "-0");
            compileDeploy(configuration, path, runtime,
                    "@public create schema N1_2(p1 N1_1)", caseName + "-1");

            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            String epl1 = "insert into N1_2 select new {p0='a'} as p1 from SupportBean";
            String prefix1 = "Invalid assignment of column 'p0' of type 'String' to event property 'p0' typed as 'Integer', column and parameter types mismatch";
            writer.appendCompileRejected("invalid-1", epl1, prefix1, configuration, path);

            String epl2 = "insert into N1_2 select new {xxx='a'} as p1 from SupportBean";
            String prefix2 = "Failed to find property 'xxx' among properties for target event type 'N1_1'";
            writer.appendCompileRejected("invalid-2", epl2, prefix2, configuration, path);

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void replaySends(EPRuntime runtime, List<JsonObject> sends) {
        for (JsonObject send : sends) {
            sendStep(runtime, send);
        }
    }

    /**
     * Mirrors EventRepresentationChoice.getAnnotationTextWJsonProvided:
     * only the json-provided rep differs from the plain annotation text,
     * prefixing "@JsonSchema(className='...')" pointing at the local
     * MyLocalJsonProvided* mirror classes declared at the bottom of this
     * file (field names/types equal the pinned suite declarations).
     */
    private static String annotationWJsonProvided(Rep rep, String providedClassSimpleName) {
        if (rep == Rep.JSONCLASSPROVIDED) {
            return "@JsonSchema(className='" + EPLInsertIntoPopulateEventTypeColRestScenarioOracle.class.getName()
                + "$" + providedClassSimpleName + "') @EventRepresentation('json')";
        }
        if (rep == Rep.MAP) {
            return "@EventRepresentation('map')";
        }
        if (rep == Rep.OBJECTARRAY) {
            return "@EventRepresentation('objectarray')";
        }
        throw new IllegalStateException("unhandled representation " + rep);
    }

    private enum Rep {
        OBJECTARRAY, MAP, JSONCLASSPROVIDED
    }

    /**
     * Listener/snapshot/compile-rejected trace writer emitting
     * esper-parity/v1 records. Nested EventBean fragments render
     * {__type:&lt;name&gt;,fields sorted}; raw SupportBean underlyings
     * render the pinned value projection; doubles go through
     * javaDoubleString and absent properties render {"state":"null"},
     * matching the Go runner's compat normalizer byte for byte.
     */
    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            append(++sequence, "listener", statement.getName(), newEvents, oldEvents);
        }

        /**
         * Window/"keepall" iterator assertion rendered as a snapshot-style
         * record occupying the post-milestone position of the case stream.
         */
        private void appendSnapshot(String statementName, EPStatement statement) {
            List<EventBean> events = new ArrayList<>();
            for (java.util.Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
                events.add(it.next());
            }
            append(++sequence, "snapshot", statementName, events.toArray(new EventBean[0]), null);
        }

        /** Frozen compile-failure record; prefix enforced by tryInvalidCompile. */
        private void appendCompileRejected(String statementLabel, String epl, String prefix,
                                           Configuration configuration, List<EPCompiled> path) {
            tryInvalidCompile(configuration, path, epl, prefix);
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "compile-rejected")
                    .add("statement", statementLabel)
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("errorPrefix", prefix);
            records.add(record);
        }

        private void append(long seq, String operation, String statementName,
                            EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statementName)
                    .add("sequence", seq)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = results(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(new JsonObject().add("kind", "row").add("fields", fieldsOf(event)));
            }
            return output;
        }

        private JsonObject fieldsOf(EventBean eventBean) {
            JsonObject fields = new JsonObject();
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                Object value;
                try {
                    value = eventBean.get(name);
                } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                    continue;
                }
                fields.add(name, normalize(value));
            }
            return fields;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean eventBean) {
                if (eventBean.getUnderlying() instanceof SupportBean bean) {
                    return normalizeSupportBean(bean);
                }
                JsonObject object = new JsonObject();
                object.add("__type", eventBean.getEventType().getName());
                String[] names = eventBean.getEventType().getPropertyNames().clone();
                java.util.Arrays.sort(names);
                for (String name : names) {
                    Object inner;
                    try {
                        inner = eventBean.get(name);
                    } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                        continue;
                    }
                    object.add(name, normalize(inner));
                }
                return object;
            }
            if (value instanceof SupportBean bean) {
                return normalizeSupportBean(bean);
            }
            if (value instanceof MyLocalJsonProvidedNested nested) {
                // json-provided representation surfaces the raw provided-class
                // POJO; project it like an event-type-name-pinned fragment.
                JsonObject object = new JsonObject();
                object.add("__type", "Nested");
                object.add("p0", normalize(nested.p0));
                object.add("p1", normalize(nested.p1));
                return object;
            }
            if (value instanceof Double || value instanceof Float) {
                return Json.value(javaDoubleString(((Number) value).doubleValue()));
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Map<?, ?> map) {
                JsonObject object = new JsonObject();
                List<String> keys = new ArrayList<>();
                for (Object key : map.keySet()) {
                    keys.add(String.valueOf(key));
                }
                java.util.Collections.sort(keys);
                for (String key : keys) {
                    object.add(key, normalize(map.get(key)));
                }
                return object;
            }
            if (value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            return Json.value(String.valueOf(value));
        }

        /**
         * Pinned SupportBean value projection (identity lives in the
         * execution/assertions, not the trace): assertion-visible scalar
         * surface of the beans routed through these work units.
         */
        private JsonValue normalizeSupportBean(SupportBean bean) {
            JsonObject object = new JsonObject();
            object.add("__type", "SupportBean");
            object.add("intPrimitive", bean.getIntPrimitive());
            object.add("theString", bean.getTheString());
            return object;
        }

        /**
         * Mirrors the Go runner's javaDoubleString convention: integral,
         * finite doubles gain a ".0" suffix (pinned cases produce
         * 10d -> "10.0"); others take Double.toString.
         */
        private static String javaDoubleString(double value) {
            if (value == Math.rint(value) && !Double.isInfinite(value)) {
                return (long) value + ".0";
            }
            return Double.toString(value);
        }
    }

    // Local mirrors of the suite's json-provided underlyings
    // (EPLInsertIntoPopulateEventTypeColumnNonBean.MyLocalJsonProvidedNested /
    // MyLocalJsonProvidedOuterType); field names/types match the pinned
    // declarations so @JsonSchema(className=...) resolves identically.

    public static class MyLocalJsonProvidedNested implements java.io.Serializable {
        private static final long serialVersionUID = 7686625266568140928L;
        public String p0;
        public int p1;
    }

    public static class MyLocalJsonProvidedOuterType implements java.io.Serializable {
        private static final long serialVersionUID = 9111321466824957997L;
        public MyLocalJsonProvidedNested n0;
    }
}
