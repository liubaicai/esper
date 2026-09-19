import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.extend.vdw.SupportVirtualDW;
import com.espertech.esper.regressionlib.support.extend.vdw.SupportVirtualDWFactory;
import com.espertech.esper.regressionlib.support.extend.vdw.SupportVirtualDWForge;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.regressionlib.support.util.SupportScheduleHelper;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.EPUndeployException;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for ContextLifecycle (ords 0-4). Replays the five
 * executions on one fresh runtime per case:
 *
 * split-stream (ord 0, ContextLifecycleSplitStream): part A deploys the
 * CtxSegmentedByTarget keyed context plus an on-trigger routing
 * SupportBean(intPrimitive=100) into NewSupportBean; s0 stays silent for
 * E1/1 and emits one row for E1/100. Part B adds the context-bound
 * NewEvent#unique(theString) named window and an output-all on-trigger
 * whose second branch projects a max(intPrimitive) subquery over NewEvent
 * into NewEventTwo; the subquery observes the pre-trigger window state, so
 * E1/1 and E1/100 emit {mymax:null} and E1/0 emits {mymax:100}.
 *
 * vdw-unrepresentable (ord 1, ContextLifecycleVirtualDataWindow): deploys
 * the CtxSegmented context, the TestVDWWindow.test:vdw() named window and
 * its consumer, sends E1/E2, undeploys all, then verifies the SPI contract
 * (every per-partition window and the factory destroyed) before emitting
 * the pinned "unrepresentable" record.
 *
 * nw-other-context-onexpr (ord 2, ContextLifecycleNWOtherContextOnExpr):
 * the NineToFive/TenToFive cron contexts plus the NineToFive-bound
 * MyWindow#keepall named window pin three on-trigger merge rejections.
 *
 * invalid (ord 3, ContextLifecycleInvalid): six probes pin the duplicate
 * context, the referenced-context undeploy precondition, the unknown
 * context, update-istream under a context, the statement-form
 * create-context under a context, and the context-field reference without
 * a statement context.
 *
 * simple (ord 4, ContextLifecycleSimple): a deploy/undeploy sequence
 * asserting context-count (ContextManagementService.getContextCount) and
 * schedule-count-overall (SupportScheduleHelper.scheduleCountOverall).
 */
public final class ContextLifecycleScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-lifecycle";
    private static final String DESCRIPTION =
            "ContextLifecycle lifecycle surface (ords 0-4): split-stream replays a keyed-context "
                    + "on-trigger routing intPrimitive=100 into an insert-into stream (part A) and an "
                    + "output-all split whose second branch projects a pre-trigger max(intPrimitive) "
                    + "subquery over a context-bound named window into NewEventTwo (part B, mymax "
                    + "null,null,100); vdw-unrepresentable pins the Java virtual-data-window SPI "
                    + "contract (one window per context partition, destroyed on undeploy) with no Go "
                    + "boundary; nw-other-context-onexpr pins three on-trigger merge rejections across "
                    + "the NineToFive/TenToFive cron contexts and the MyWindow/MyWindowTwo named "
                    + "windows; invalid pins six rejection probes (duplicate context, "
                    + "referenced-context undeploy precondition, unknown context, update-istream under "
                    + "a context, unrepresentable statement-form create-context, context field without "
                    + "a statement context); simple pins the context-count/schedule-count-overall "
                    + "deploy-undeploy sequence with one shared schedule per referenced temporal "
                    + "context. compile-error/undeploy-error records carry the pinned Java message "
                    + "prefixes; unrepresentable carries the pinned SPI note; context-count and "
                    + "schedule-count-overall carry observed counts (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextLifecycle.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/"
                    + "ContextLifecycle.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-d59fd16257279a2118a7",
            "java-runtime-38ddfd09bd97e862e438",
            "java-runtime-a80d326a65c440aa438f",
            "java-runtime-f04b99d603d61f3808fb",
            "java-runtime-ba7774dedca1bd391c8c"
    };
    private static final String[] EXECUTION_NAMES = {
            "ContextLifecycleSplitStream",
            "ContextLifecycleVirtualDataWindow",
            "ContextLifecycleNWOtherContextOnExpr",
            "ContextLifecycleInvalid",
            "ContextLifecycleSimple"
    };
    private static final String[] STATIC_IDS = {
            "java-1d9805ba018e9b1bed1d",
            "java-724063e9b5a1a2b3d83a",
            "java-15148b122e0ca21c3681",
            "java-1a96001402ca3a4c074e",
            "java-407112a1151ec755e45b"
    };
    private static final String[] JAVA_FLAGS = {"INVALIDITY", "STATICHOOK"};
    private static final String[] CASES = {
            "split-stream",
            "vdw-unrepresentable",
            "nw-other-context-onexpr",
            "invalid",
            "simple"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4};

    // Verbatim transcriptions of ContextLifecycle lines 44-47, 59-66,
    // 86-88, 107-123, 132, 149, 161-162, 183-212.
    private static final String EPL_SPLIT_CTX =
            "@public create context CtxSegmentedByTarget partition by theString from SupportBean";
    private static final String EPL_SPLIT_OUT_A =
            "@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into "
                    + "NewSupportBean select * where intPrimitive = 100";
    private static final String EPL_SPLIT_S0_A = "@name('s0') select * from NewSupportBean";
    private static final String EPL_SPLIT_WINDOW =
            "@public context CtxSegmentedByTarget create window NewEvent#unique(theString) as "
                    + "SupportBean";
    private static final String EPL_SPLIT_OUT_B =
            "@Name('out') @public context CtxSegmentedByTarget on SupportBean "
                    + "insert into NewEvent select * where intPrimitive = 100 "
                    + "insert into NewEventTwo select (select max(intPrimitive) from NewEvent) as "
                    + "mymax  output all";
    private static final String EPL_SPLIT_S0_B = "@name('s0') select * from NewEventTwo";

    private static final String EPL_VDW_CTX =
            "@public create context CtxSegmented as partition by theString from SupportBean";
    private static final String EPL_VDW_WINDOW =
            "@public context CtxSegmented create window TestVDWWindow.test:vdw() as SupportBean";
    private static final String EPL_VDW_S0 = "select * from TestVDWWindow";

    private static final String EPL_NINE_TO_FIVE =
            "@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)";
    private static final String EPL_TEN_TO_FIVE =
            "@public create context TenToFive as start (0, 10, *, *, *) end (0, 17, *, *, *)";
    private static final String EPL_NW_CREATEWINDOW =
            "@name('createwindow') @public context NineToFive create window MyWindow#keepall as "
                    + "SupportBean";
    private static final String EPL_NW_WINDOW_TWO =
            "@public create window MyWindowTwo#keepall as SupportBean";
    private static final String EPL_PROBE_NO_CONTEXT =
            "on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1";
    private static final String EPL_PROBE_OTHER_CONTEXT =
            "context TenToFive on SupportBean_S0 s0 merge MyWindow mw when matched then update set "
                    + "intPrimitive = 1";
    private static final String EPL_PROBE_CONTEXTLESS_WINDOW =
            "context TenToFive on SupportBean_S0 s0 merge MyWindowTwo mw when matched then update "
                    + "set intPrimitive = 1";

    private static final String EPL_INVALID_CTX =
            "@name('ctx') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, "
                    + "*, *)";
    private static final String EPL_INVALID_S0 = "context NineToFive select * from SupportBean";
    private static final String EPL_INVALID_ABC_STREAM =
            "@public insert into ABCStream select * from SupportBean";
    private static final String EPL_INVALID_CTX_SEGMENTED =
            "@Name('context') @public create context SegmentedByAString partition by theString from "
                    + "ABCStream";
    private static final String EPL_INVALID_CTX_ABC =
            "@public create context ABC start @now end after 5 seconds";
    private static final String EPL_PROBE_CONTEXT_NOT_FOUND =
            "context EightToSix select * from SupportBean";
    private static final String EPL_PROBE_UPDATE_ISTREAM =
            "context SegmentedByAString update istream ABCStream set intPrimitive = (select id from "
                    + "SupportBean_S0#lastevent) where intPrimitive < 0";
    private static final String EPL_PROBE_CREATE_CONTEXT_IN_CONTEXT =
            "context ABC create context DEF start @now end after 5 seconds";
    private static final String EPL_PROBE_CONTEXT_FIELD =
            "select context.sb.theString from SupportBean as sb";

    private static final String EPL_SIMPLE_CTX =
            "@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, "
                    + "*, *, *)";
    private static final String EPL_SIMPLE_S0 =
            "@Name('s0') context NineToFive select * from SupportBean";
    private static final String EPL_SIMPLE_C =
            "@Name('C') context NineToFive select * from SupportBean";
    private static final String EPL_SIMPLE_D =
            "@Name('D') context NineToFive select * from SupportBean";

    private static final String ERR_NO_CONTEXT =
            "Cannot create on-trigger expression: Named window 'MyWindow' was declared with context "
                    + "'NineToFive', please declare the same context name";
    private static final String ERR_OTHER_CONTEXT =
            "Cannot create on-trigger expression: Named window 'MyWindow' was declared with context "
                    + "'NineToFive', please use the same context instead";
    private static final String ERR_CONTEXTLESS_WINDOW =
            "Cannot create on-trigger expression: Named window 'MyWindowTwo' was declared without a "
                    + "context";
    private static final String ERR_DUPLICATE_CONTEXT =
            "Context by name 'NineToFive' already exists";
    private static final String ERR_CONTEXT_IN_USE =
            "A precondition is not satisfied: Context 'NineToFive' cannot be un-deployed as it is "
                    + "referenced by deployment";
    private static final String ERR_CONTEXT_NOT_FOUND =
            "Context by name 'EightToSix' could not be found";
    private static final String ERR_UPDATE_ISTREAM =
            "Update IStream is not supported in conjunction with a context";
    private static final String ERR_CREATE_CONTEXT_IN_CONTEXT =
            "A create-context statement cannot itself be associated to a context, please declare a "
                    + "nested context instead";
    private static final String ERR_CONTEXT_FIELD =
            "Failed to validate select-clause expression 'context.sb.theString': Failed to resolve "
                    + "property 'context.sb.theString' to a stream or nested property in a stream";

    private static final String VDW_NOTE =
            "virtual data window SPI: one VirtualDataWindow instance per context partition (two "
                    + "partitions observed for E1/E2), every window and the factory destroyed on "
                    + "undeploy; no Go plugin-window boundary";

    private static final String[] CASE_OBSERVATIONS = {
            "listener; part A routes SupportBean(intPrimitive=100) into NewSupportBean under the "
                    + "keyed context so s0 emits one row for E1/100 and stays silent for E1/1; part "
                    + "B's output-all split projects the pre-trigger max(intPrimitive) over NewEvent "
                    + "into NewEventTwo so s0 emits mymax null, null, then 100 for E1/1, E1/100, E1/0",
            "unrepresentable; the Java virtual-data-window SPI instantiates one window per context "
                    + "partition (two after E1/E2) and destroys every window plus the factory on "
                    + "undeploy-all; no Go plugin-window boundary exists, so the pinned record "
                    + "documents the contract",
            "compile-error; three on-trigger merge probes pin the named-window context rules: a "
                    + "contextless trigger against NineToFive-bound MyWindow, a TenToFive trigger "
                    + "against MyWindow, and a TenToFive trigger against contextless MyWindowTwo "
                    + "after createwindow is undeployed",
            "compile-error+undeploy-error; six probes pin the duplicate NineToFive registration, the "
                    + "provider undeploy precondition while s0 references the context, the unknown "
                    + "EightToSix context, update-istream under SegmentedByAString, the "
                    + "unrepresentable statement-form create-context under ABC, and the "
                    + "context.sb.theString reference without a statement context",
            "context-count+schedule-count-overall; deploy/undeploy steps pin len(env.Contexts()) and "
                    + "Engine.ScheduleCountOverall: the cron context counts once registered, one "
                    + "shared schedule exists while any statement references it, and undeploy-all "
                    + "returns both counters to zero"
    };
    private static final String[] CASE_EPLS = {
            EPL_SPLIT_OUT_B,
            EPL_VDW_WINDOW,
            EPL_PROBE_OTHER_CONTEXT,
            EPL_PROBE_CONTEXT_NOT_FOUND,
            EPL_SIMPLE_CTX
    };

    private static final int EXPECTED_STEPS = 70;

    /**
     * Pinned per-case step keys rendered as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count.
     * Deploy steps carry the byte-exact EPL text; compileWithoutPath marks
     * the ord-4 first deploy whose Java execution calls the path-less
     * compileDeploy overload and the ord-3 path-less tryInvalidCompile;
     * build-error/undeploy-error steps pin the Java message prefix in
     * expectError; the unrepresentable step pins the SPI note; count steps
     * pin the asserted count.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("split-stream", new String[]{
                "deploy|ctx|||" + EPL_SPLIT_CTX + "||||||",
                "deploy|out|||" + EPL_SPLIT_OUT_A + "||||||",
                "deploy|s0|||" + EPL_SPLIT_S0_A + "||||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}|||||",
                "undeploy-all||||||||||",
                "deploy|ctx|||" + EPL_SPLIT_CTX + "||||||",
                "deploy|window|||" + EPL_SPLIT_WINDOW + "||||||",
                "deploy|out|||" + EPL_SPLIT_OUT_B + "||||||",
                "deploy|s0|||" + EPL_SPLIT_S0_B + "||||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}|||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":0}|||||",
                "undeploy-all||||||||||",
        });
        CASE_STEPS.put("vdw-unrepresentable", new String[]{
                "deploy|ctx|||" + EPL_VDW_CTX + "||||||",
                "deploy|window|||" + EPL_VDW_WINDOW + "||||||",
                "deploy|s0|||" + EPL_VDW_S0 + "||||||",
                "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
                "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":2}|||||",
                "undeploy-all||||||||||",
                "unrepresentable|virtual-data-window|||||" + VDW_NOTE + "||||",
        });
        CASE_STEPS.put("nw-other-context-onexpr", new String[]{
                "deploy|ctx-nine|||" + EPL_NINE_TO_FIVE + "||||||",
                "deploy|ctx-ten|||" + EPL_TEN_TO_FIVE + "||||||",
                "deploy|createwindow|||" + EPL_NW_CREATEWINDOW + "||||||",
                "build-error|trigger-no-context|||" + EPL_PROBE_NO_CONTEXT + "||" + ERR_NO_CONTEXT
                        + "||||",
                "build-error|trigger-other-context|||" + EPL_PROBE_OTHER_CONTEXT + "||"
                        + ERR_OTHER_CONTEXT + "||||",
                "undeploy|createwindow|||||||||",
                "deploy|window-two|||" + EPL_NW_WINDOW_TWO + "||||||",
                "build-error|trigger-contextless-window|||" + EPL_PROBE_CONTEXTLESS_WINDOW + "||"
                        + ERR_CONTEXTLESS_WINDOW + "||||",
                "undeploy-all||||||||||",
        });
        CASE_STEPS.put("invalid", new String[]{
                "deploy|ctx|||" + EPL_INVALID_CTX + "||||||",
                "build-error|duplicate-context|||" + EPL_INVALID_CTX + "||" + ERR_DUPLICATE_CONTEXT
                        + "||||",
                "deploy|s0|||" + EPL_INVALID_S0 + "||||||",
                "undeploy-error|ctx|||||" + ERR_CONTEXT_IN_USE + "||||",
                "build-error|context-not-found|||" + EPL_PROBE_CONTEXT_NOT_FOUND + "||"
                        + ERR_CONTEXT_NOT_FOUND + "||||",
                "deploy|abc-stream|||" + EPL_INVALID_ABC_STREAM + "||||||",
                "deploy|ctx-segmented|||" + EPL_INVALID_CTX_SEGMENTED + "||||||",
                "build-error|update-istream-context|||" + EPL_PROBE_UPDATE_ISTREAM + "||"
                        + ERR_UPDATE_ISTREAM + "||||",
                "deploy|ctx-abc|||" + EPL_INVALID_CTX_ABC + "||||||",
                "build-error|create-context-in-context|||" + EPL_PROBE_CREATE_CONTEXT_IN_CONTEXT
                        + "||" + ERR_CREATE_CONTEXT_IN_CONTEXT + "||||",
                "build-error|context-field-no-context|||" + EPL_PROBE_CONTEXT_FIELD + "||"
                        + ERR_CONTEXT_FIELD + "|1|||",
                "undeploy-all||||||||||",
        });
        CASE_STEPS.put("simple", new String[]{
                "context-count||||||||||0",
                "schedule-count-overall||||||||||0",
                "deploy|ctx|||" + EPL_SIMPLE_CTX + "|||1|||",
                "context-count||||||||||1",
                "schedule-count-overall||||||||||0",
                "undeploy|ctx|||||||||",
                "context-count||||||||||0",
                "deploy|ctx|||" + EPL_SIMPLE_CTX + "||||||",
                "context-count||||||||||1",
                "deploy|s0|||" + EPL_SIMPLE_S0 + "||||||",
                "schedule-count-overall||||||||||1",
                "undeploy|s0|||||||||",
                "schedule-count-overall||||||||||0",
                "undeploy|ctx|||||||||",
                "context-count||||||||||0",
                "deploy|ctx|||" + EPL_SIMPLE_CTX + "||||||",
                "deploy|c|||" + EPL_SIMPLE_C + "||||||",
                "deploy|d|||" + EPL_SIMPLE_D + "||||||",
                "schedule-count-overall||||||||||1",
                "undeploy-all||||||||||",
                "context-count||||||||||0",
                "schedule-count-overall||||||||||0",
                "undeploy-all||||||||||",
        });
    }

    private ContextLifecycleScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextLifecycleScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0])));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, steps, records);
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
     * Replays the case's steps on a fresh runtime. Deploy steps compile
     * with the accumulated module path (env.compileDeploy(epl, path));
     * compileWithoutPath deploys compile without it, mirroring the
     * path-less compileDeploy overload. build-error steps compile with or
     * without the path per their marker, mirroring env.tryInvalidCompile's
     * two forms. undeploy resolves the deployment recorded under the step
     * label, mirroring undeployModuleContaining.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCompiler().addPlugInVirtualDataWindow("test", "vdw",
                SupportVirtualDWForge.class.getName(), SupportVirtualDW.ITERATE);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            if ("vdw-unrepresentable".equals(caseName)) {
                SupportVirtualDWFactory.getWindows().clear();
                SupportVirtualDWFactory.setDestroyed(false);
            }
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPCompiled> modulesByLabel = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
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
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, deploymentIds,
                                modulesByLabel, deployedModules, records);
                        break;
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, deployedModules, records);
                        break;
                    case "undeploy-error":
                        undeployErrorStep(runtime, caseName, step, deploymentIds, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(caseName, step, records);
                        break;
                    case "undeploy": {
                        String label = string(step, "statement");
                        String deploymentId = deploymentIds.get(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "no deployment for statement " + label);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        // undeployModuleContaining retires the deployment;
                        // the Java executions continue on a fresh or cleared
                        // RegressionPath, so the label's module leaves the
                        // accumulated compile path here.
                        EPCompiled evicted = modulesByLabel.remove(label);
                        if (evicted != null) {
                            deployedModules.remove(evicted);
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        modulesByLabel.clear();
                        deployedModules.clear();
                        break;
                    case "context-count": {
                        int observed = ((EPRuntimeSPI) runtime).getServicesContext()
                                .getContextManagementService().getContextCount();
                        countRecord(caseName, step, records, "context-count", observed);
                        break;
                    }
                    case "schedule-count-overall": {
                        // Mirrors SupportScheduleHelper.scheduleCountOverall.
                        countRecord(caseName, step, records, "schedule-count-overall",
                                SupportScheduleHelper.scheduleCountOverall(runtime));
                        break;
                    }
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module. compileWithoutPath steps
     * compile without the accumulated module path, mirroring the Java
     * execution's path-less compileDeploy; other deploys accumulate into
     * the path like RegressionPath. The split-stream s0 deploys attach the
     * trace listener, mirroring addListener("s0").
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration, String caseName,
                                   JsonObject step, Map<String, String> deploymentIds,
                                   Map<String, EPCompiled> modulesByLabel,
                                   List<EPCompiled> deployedModules, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        boolean withoutPath = step.getBoolean("compileWithoutPath", false);
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        if (!withoutPath) {
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        if (!withoutPath) {
            deployedModules.add(compiled);
        }
        modulesByLabel.put(label, compiled);
        deploymentIds.put(label, deployment.getDeploymentId());
        if ("split-stream".equals(caseName) && "s0".equals(label)) {
            for (EPStatement statement : deployment.getStatements()) {
                if ("s0".equals(statement.getName())) {
                    statement.addListener(new TraceWriter(records, caseName, statement, runtime));
                }
            }
        }
    }

    /** Sends one event bean decoded from the step payload. */
    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = string(step, "eventType");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                JsonValue theString = payload.get("theString");
                JsonValue intPrimitive = payload.get("intPrimitive");
                runtime.getEventService().sendEventBean(new SupportBean(
                        theString == null || theString.isNull() ? null : theString.asString(),
                        intPrimitive == null || intPrimitive.isNull() ? 0 : intPrimitive.asInt()),
                        type);
                break;
            }
            case "SupportBean_S0": {
                JsonValue p00 = payload.get("p00");
                runtime.getEventService().sendEventBean(new SupportBean_S0(
                        payload.get("id").asInt(),
                        p00 == null || p00.isNull() ? null : p00.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). Probes marked compileWithoutPath compile without the
     * accumulated module path, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx; the rest compile with the path.
     */
    private static void buildErrorStep(Configuration configuration, String caseName, JsonObject step,
                                       List<EPCompiled> deployedModules, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                for (EPCompiled deployed : deployedModules) {
                    compilerArgs.getPath().add(deployed);
                }
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
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
     * Mirrors the Java execution's undeployModuleContaining precondition
     * probe: undeploying the provider deployment while s0 references its
     * context throws EPUndeployException. Emits
     * {"operation":"undeploy-error"} carrying the pinned prefix after
     * verifying the caught message starts with it.
     */
    private static void undeployErrorStep(EPRuntime runtime, String caseName, JsonObject step,
                                          Map<String, String> deploymentIds, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String deploymentId = deploymentIds.get(label);
        if (deploymentId == null) {
            throw new IllegalStateException("no deployment for statement " + label);
        }
        String caught;
        try {
            runtime.getDeploymentService().undeploy(deploymentId);
            caught = null;
        } catch (EPUndeployException ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException("undeploy-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("undeploy-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "undeploy-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Emits the pinned "unrepresentable" record for the virtual data
     * window case after verifying the SPI contract the note documents:
     * every per-partition window and the factory are destroyed by the
     * preceding undeploy-all (the Java execution's assertThat block).
     */
    private static void unrepresentableStep(String caseName, JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        if (!VDW_NOTE.equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " carries an unpinned note");
        }
        if (SupportVirtualDWFactory.getWindows().size() != 2) {
            throw new IllegalStateException("expected two per-partition virtual data windows, got "
                    + SupportVirtualDWFactory.getWindows().size());
        }
        for (SupportVirtualDW window : SupportVirtualDWFactory.getWindows()) {
            if (!window.isDestroyed()) {
                throw new IllegalStateException(
                        "virtual data window was not destroyed on undeploy");
            }
        }
        if (!SupportVirtualDWFactory.isDestroyed()) {
            throw new IllegalStateException(
                    "virtual data window factory was not destroyed on undeploy");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    /**
     * Emits one count record for the pinned introspection probes, mirroring
     * the view-group count-record protocol (no time, no sequence, observed
     * value only). The step's declared count is the Java-asserted value and
     * is verified before the record is emitted.
     */
    private static void countRecord(String caseName, JsonObject step, JsonArray records,
                                    String operation, int observed) {
        JsonValue declared = step.get("count");
        if (declared == null || !declared.isNumber() || declared.asInt() != observed) {
            throw new IllegalStateException(operation + " drift for " + caseName
                    + ": declared " + (declared == null ? "<none>" : declared.toString())
                    + " observed " + observed);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", operation);
        record.add("count", observed);
        records.add(record);
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
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
     * with the payload, ids and count rendered as compacted JSON. Unknown
     * fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "name", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "ids", "count"));
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
        JsonValue count = step.get("count");
        String countText = count == null ? "" : count.toString();
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + idsText + "|" + countText;
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException(
                            "duplicate JSON object key: " + member.getName());
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
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            long parsed = Long.parseLong(text, 10);
            if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " is outside the Java int range");
            }
            return (int) parsed;
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
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

    private static JsonArray results(EventBean[] events) {
        JsonArray output = new JsonArray();
        if (events == null) {
            return output;
        }
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            output.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return output;
    }

    private static JsonValue eventFields(String[] names,
                                         java.util.function.Function<String, Object> getter) {
        JsonObject fields = new JsonObject();
        String[] sorted = names.clone();
        Arrays.sort(sorted);
        for (String name : sorted) {
            fields.add(name, normalize(getter.apply(name)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof EventBean) {
            return eventFields(((EventBean) value).getEventType().getPropertyNames(),
                    key -> ((EventBean) value).get(key));
        }
        if (value instanceof Map) {
            Map<?, ?> map = (Map<?, ?>) value;
            String[] keys = map.keySet().toArray(new String[0]);
            Arrays.sort(keys);
            return eventFields(keys, map::get);
        }
        if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).intValue());
        }
        if (value instanceof Double || value instanceof Float) {
            double d = ((Number) value).doubleValue();
            if (d == Math.rint(d) && !Double.isInfinite(d)) {
                return Json.value((long) d);
            }
            return Json.value(d);
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime())
                            .toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }
    }
}
