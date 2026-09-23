import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierInitiatedTerminated;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
import com.espertech.esper.common.client.context.ContextPartitionSelectorById;
import com.espertech.esper.common.client.context.ContextPartitionSelectorFiltered;
import com.espertech.esper.common.client.context.ContextPartitionSelectorSegmented;
import com.espertech.esper.common.client.context.InvalidContextPartitionSelector;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.filtersvcimpl.FilterServiceSPI;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.runtime.internal.schedulesvcimpl.ScheduleVisitor;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the context-start-end-trio parity scenario:
 * three initiated/terminated-context executions replayed as three cases, each
 * against its own fresh runtime.
 *
 * startend-partition-selection replays
 * ContextInitTermTemporalFixed.ContextStartEndContextPartitionSelection (ord
 * 0): a non-overlapping "start SupportBean_S0 s0 end SupportBean_S1(id=s0.id)"
 * context (byte-exact doubled @public annotation) with a grouped keepall
 * statement projecting context.id, the initiating event's p00, theString and
 * sum(intPrimitive). The S0 start event opens partition id=0 at startTime
 * 1000; no end event is ever sent so the partition stays open through
 * undeploy-all. The grouped keepall fires one listener row per SupportBean
 * send while open (six rows). snapshot-selector probes pin the by-id,
 * filtered (match and no-match) and segmented selectors; the always-false
 * filtered selector emits an empty-rows record while the oracle asserts
 * internally that the callback still observed every partition (Java pins
 * startTimes=[1000L] and p00=["S0_1"], mirrored here as set-equality against
 * the all-selector partition view). The segmented selector is rejected with
 * InvalidContextPartitionSelector, normalized to
 * invalid-context-partition-selector.
 *
 * startend-prev-prior-agg replays
 * ContextInitTermTemporalFixed.ContextStartEndPrevPriorAndAggregation (ord
 * 7): the NineToFive crontab context (start 09:00, end 17:00) with a keepall
 * window projecting prev/prevwindow/prevtail/prior/sum (byte-exact #keepall
 * without parens). Sends outside the window are silent; the 17:00 advance
 * destroys the partition (empty iterator, no listener event); the next day's
 * 09:00 start creates a fresh partition whose prev/prior state is fully
 * reset. prevwindow renders the window newest-first as an event array.
 *
 * initterm-schedule-filter-resources replays
 * ContextInitTerm.ContextInitTermScheduleFilterResources (ord 10): phase A
 * pins the plain #time(30) expiry schedule (1 while deployed, 0 after
 * undeployModuleContaining); phase B deploys an initiated-by/terminated-
 * after-1-minutes context plus an unnamed S0#time(2 min) statement on the
 * same RegressionPath and pins schedule-count-overall 0/1/2/0/0 across
 * initiation, the S0 send, the 08:01 termination (which preempts the S0
 * view's 2-minute expiry) and undeploy-all. The paired filterCount step
 * field is asserted internally against
 * SupportFilterServiceHelper.getFilterSvcCountApprox (reimplemented through
 * FilterServiceSPI because RegressionEnvironment is not on the oracle
 * classpath); no filter-count record is emitted because Go has no
 * filter-service analog (ContextCategoryScenarioOracle precedent).
 *
 * Deploy steps compile the byte-exact EPL with the accumulated module path
 * (env.compileDeploy(epl, path) semantics); undeploy resolves the deployment
 * recorded under the step label and evicts its module from the path,
 * mirroring undeployModuleContaining plus a fresh RegressionPath.
 * schedule-count-overall records follow the count-record protocol (no time,
 * no sequence, observed count only) and verify the step's declared count
 * before emitting.
 */
public final class ContextStartEndTrioScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-start-end-trio";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java";

    private static final String DESCRIPTION =
            "ContextInitTermTemporalFixed start/end executions plus "
                    + "ContextInitTermScheduleFilterResources (three "
                    + "executions, the context start/end trio): "
                    + "startend-partition-selection replays "
                    + "ContextStartEndContextPartitionSelection (ord 0) — a "
                    + "non-overlapping start SupportBean_S0/end "
                    + "SupportBean_S1(id=s0.id) context (byte-exact doubled "
                    + "@public) whose single partition stays open through "
                    + "undeploy-all; the grouped keepall s0 fires one "
                    + "listener row per SupportBean send and the "
                    + "snapshot-selector probes pin ids=[0], filtered "
                    + "p00=S0_1 (3 rows), p00=S0_2 (empty), always-false "
                    + "(empty; the oracle asserts internally that the "
                    + "callback observed every partition) and segmented "
                    + "(selector-error invalid-context-partition-selector). "
                    + "startend-prev-prior-agg replays "
                    + "ContextStartEndPrevPriorAndAggregation (ord 7) — the "
                    + "NineToFive crontab context projecting "
                    + "prev/prevwindow/prevtail/prior/sum over #keepall "
                    + "(byte-exact, no parens); sends outside 09:00-17:00 "
                    + "are silent, the 17:00 end empties the iterator and "
                    + "the next day's 09:00 start resets prev/prior state. "
                    + "initterm-schedule-filter-resources replays "
                    + "ContextInitTermScheduleFilterResources "
                    + "(ContextInitTerm ord 10) — phase A pins the plain "
                    + "#time(30) expiry schedule; phase B pins the "
                    + "initiated-by/terminated-after-1-minutes lifecycle "
                    + "where the termination schedule is created at "
                    + "partition initiation and the S0#time(2 min) expiry "
                    + "is preempted by partition destruction at 1 min; "
                    + "schedule-count-overall steps emit records while the "
                    + "paired filterCount field is asserted internally "
                    + "against getFilterSvcCountApprox (no Go analog).";

    private static final String[] CASES = {
            "startend-partition-selection", "startend-prev-prior-agg",
            "initterm-schedule-filter-resources"};
    private static final int[] ORDINALS = {0, 7, 10};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-0319b4a966a4103e44ef",
            "java-runtime-2bc9a5098743149a306e",
            "java-runtime-10686594f4fde577bfed"};
    private static final String[] EXECUTIONS = {
            "ContextStartEndContextPartitionSelection",
            "ContextStartEndPrevPriorAndAggregation",
            "ContextInitTermScheduleFilterResources"};
    private static final String[] STATIC_IDS = {
            "java-06954b45a1979f495425",
            "java-06954b45a1979f495425",
            "java-025e65b1524c37205df0"};
    private static final String[] CONTEXT_NAMES = {
            "MyCtx", "NineToFive", "EverySupportBean"};
    private static final Set<String> LISTENED_CASES = new HashSet<>(Arrays.asList(
            "startend-partition-selection", "startend-prev-prior-agg"));

    private static final String EPL_STARTEND_CTX =
            "@public @public create context MyCtx as start SupportBean_S0 s0 end SupportBean_S1(id=s0.id)";
    private static final String EPL_STARTEND_S0 =
            "@name('s0') context MyCtx select context.id as c0, context.s0.p00 as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString";
    private static final String EPL_NINETOFIVE_CTX =
            "@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)";
    private static final String EPL_NINETOFIVE_S0 =
            "@name('s0') context NineToFive select prev(theString) as col1, prevwindow(sb) as col2, prevtail(theString) as col3, prior(1, theString) as col4, sum(intPrimitive) as col5 from SupportBean#keepall as sb";
    private static final String EPL_RESOURCES_S0 =
            "@name('s0') select * from SupportBean#time(30)";
    private static final String EPL_RESOURCES_CTX =
            "@name('ctx') @public create context EverySupportBean as initiated by SupportBean as sb terminated after 1 minutes";
    private static final String EPL_RESOURCES_STMT =
            "context EverySupportBean select * from SupportBean_S0#time(2 min) sb0";

    private static final String[] CASE_EPLS = {
            EPL_STARTEND_S0, EPL_NINETOFIVE_S0, EPL_RESOURCES_CTX};

    private static final String[] OBSERVATIONS = {
            "listener+snapshot+snapshot-selector+selector-error; "
                    + "non-overlapping start/end context (S0 starts the "
                    + "partition at startTime 1000, the correlated S1 end is "
                    + "never sent) keeps one partition open through "
                    + "undeploy-all; grouped keepall fires one listener row "
                    + "per SupportBean send; ids/filtered/always-false/"
                    + "segmented selector probes pin iterator rows and "
                    + "InvalidContextPartitionSelector",
            "listener+snapshot; NineToFive crontab context projects "
                    + "prev/prevwindow/prevtail/prior/sum over keepall; the "
                    + "17:00 end destroys the partition (empty iterator) "
                    + "and the next day's 09:00 start resets prev/prior "
                    + "state",
            "schedule-count-overall; phase A pins the plain #time(30) "
                    + "expiry schedule; phase B pins the initiated-by/"
                    + "terminated-after-1-minutes lifecycle — termination "
                    + "schedule created at initiation, S0#time(2 min) "
                    + "expiry preempted by partition destruction at 1 min; "
                    + "filterSvcCountApprox asserted internally (no Go "
                    + "analog)"};

    // Pinned step keys per case (after the case marker), rendered as
    // op|statement|eventType|at|epl|payload|selector|ids|filterProperty|
    // filterValue|expectError|count|filterCount with payload/ids rendered as
    // compacted JSON, a JSON-null filterValue as <null> and absent fields
    // empty. filterCount is the Java-asserted getFilterSvcCountApprox value
    // checked internally by the oracle; it emits no record.
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();

    static {
        CASE_STEPS.put("startend-partition-selection", new String[]{
                "advance-time|||1970-01-01T00:00:00.000Z|||||||||",
                "deploy|ctx|||" + EPL_STARTEND_CTX + "||||||||",
                "deploy|s0|||" + EPL_STARTEND_S0 + "||||||||",
                "advance-time|||1970-01-01T00:00:01.000Z|||||||||",
                "send||SupportBean_S0|||{\"id\":1,\"p00\":\"S0_1\",\"p01\":null}|||||||",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
                "send||SupportBean|||{\"theString\":\"E2\",\"intPrimitive\":10}|||||||",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":2}|||||||",
                "send||SupportBean|||{\"theString\":\"E3\",\"intPrimitive\":100}|||||||",
                "send||SupportBean|||{\"theString\":\"E3\",\"intPrimitive\":101}|||||||",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":3}|||||||",
                "snapshot|s0|||||||||||",
                "snapshot-selector|s0|||||ids|[0]|||||",
                "snapshot-selector|s0|||||filtered||p00|S0_1|||",
                "snapshot-selector|s0|||||filtered||p00|S0_2|||",
                "snapshot-selector|s0|||||filtered||p00|<null>|||",
                "snapshot-selector|s0|||||segmented||||invalid-context-partition-selector||",
                "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("startend-prev-prior-agg", new String[]{
                "advance-time|||2002-05-01T08:00:00.000Z|||||||||",
                "deploy|ctx|||" + EPL_NINETOFIVE_CTX + "||||||||",
                "deploy|s0|||" + EPL_NINETOFIVE_S0 + "||||||||",
                "send||SupportBean|||{\"theString\":null,\"intPrimitive\":0}|||||||",
                "advance-time|||2002-05-01T09:00:00.000Z|||||||||",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
                "snapshot|s0|||||||||||",
                "send||SupportBean|||{\"theString\":\"E2\",\"intPrimitive\":2}|||||||",
                "advance-time|||2002-05-01T17:00:00.000Z|||||||||",
                "snapshot|s0|||||||||||",
                "send||SupportBean|||{\"theString\":null,\"intPrimitive\":0}|||||||",
                "advance-time|||2002-05-02T09:00:00.000Z|||||||||",
                "send||SupportBean|||{\"theString\":\"E3\",\"intPrimitive\":9}|||||||",
                "snapshot|s0|||||||||||",
                "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("initterm-schedule-filter-resources", new String[]{
                "deploy|s0|||" + EPL_RESOURCES_S0 + "||||||||",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
                "schedule-count-overall|||||||||||1|",
                "undeploy|s0|||||||||||",
                "schedule-count-overall|||||||||||0|",
                "advance-time|||2002-05-01T08:00:00.000Z|||||||||",
                "deploy|ctx|||" + EPL_RESOURCES_CTX + "||||||||",
                "deploy|stmt|||" + EPL_RESOURCES_STMT + "||||||||",
                "schedule-count-overall|||||||||||0|1",
                "send||SupportBean|||{\"theString\":\"E1\",\"intPrimitive\":0}|||||||",
                "schedule-count-overall|||||||||||1|2",
                "send||SupportBean_S0|||{\"id\":0,\"p00\":\"S0_1\",\"p01\":null}|||||||",
                "schedule-count-overall|||||||||||2|2",
                "advance-time|||2002-05-01T08:01:00.000Z|||||||||",
                "schedule-count-overall|||||||||||0|1",
                "undeploy-all||||||||||||",
                "schedule-count-overall|||||||||||0|0",
        });
    }

    private static final int EXPECTED_STEPS = 53;
    private static final int EXPECTED_RECORDS = 25;

    private ContextStartEndTrioScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ContextStartEndTrioScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0])));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, CASES[caseIndex], caseIndex, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    /**
     * Replays the case's steps on a fresh runtime with the internal timer
     * disabled and the clock initialized at epoch. Deploy steps compile
     * with the accumulated module path; the s0 listener is attached after
     * deploy for the two listened cases (the Java partition-selection
     * execution attaches none, but the grouped keepall still fires one row
     * per send and the Go runner must match them).
     */
    private static void runCase(JsonArray steps, String caseName,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        s0Type.put("p01", String.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("p10", String.class);
        s1Type.put("p11", String.class);
        configuration.getCommon().addEventType("SupportBean_S1", s1Type);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, EPCompiled> modulesByLabel = new HashMap<>();
            Map<String, TraceWriter> writersByLabel = new HashMap<>();
            Map<String, String> contextDeploymentIds = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            boolean active = false;
            for (JsonValue stepValue : steps) {
                JsonObject step = stepValue.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (op) {
                    case "deploy":
                        deployStep(runtime, configuration, caseName, caseIndex, step,
                                deploymentIds, modulesByLabel, writersByLabel,
                                contextDeploymentIds, deployedModules, records);
                        break;
                    case "send":
                        send(runtime, step);
                        break;
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "snapshot":
                        requireWriter(writersByLabel, step, caseName).snapshot("snapshot", null);
                        break;
                    case "snapshot-selector":
                        snapshotSelector(runtime, caseName, caseIndex, step,
                                writersByLabel, records);
                        break;
                    case "schedule-count-overall": {
                        // Mirrors SupportScheduleHelper.scheduleCountOverall;
                        // the optional filterCount field is asserted
                        // internally against getFilterSvcCountApprox and
                        // emits no record (no Go filter-service analog).
                        countRecord(caseName, step, records, "schedule-count-overall",
                                scheduleCountOverall(runtime));
                        JsonValue filterCount = step.get("filterCount");
                        if (filterCount != null) {
                            assertFilterCount(runtime, filterCount.asInt());
                        }
                        break;
                    }
                    case "undeploy": {
                        String label = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "no deployment for statement " + label);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        deploymentIds.remove(label);
                        writersByLabel.remove(label);
                        // undeployModuleContaining retires the deployment;
                        // the Java execution continues on a fresh
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
                        writersByLabel.clear();
                        deployedModules.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys one labeled module with the accumulated module
     * path (env.compileDeploy(epl, path)). Named statements are registered
     * under their EPL name; the s0 statement of a listened case gets the
     * trace listener.
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, int caseIndex, JsonObject step,
                                   Map<String, String> deploymentIds,
                                   Map<String, EPCompiled> modulesByLabel,
                                   Map<String, TraceWriter> writersByLabel,
                                   Map<String, String> contextDeploymentIds,
                                   List<EPCompiled> deployedModules,
                                   JsonArray records) throws Exception {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        for (EPCompiled deployed : deployedModules) {
            compilerArgs.getPath().add(deployed);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        deployedModules.add(compiled);
        modulesByLabel.put(label, compiled);
        deploymentIds.put(label, deployment.getDeploymentId());
        if ("ctx".equals(label)) {
            contextDeploymentIds.put(CONTEXT_NAMES[caseIndex], deployment.getDeploymentId());
        }
        for (EPStatement candidate : deployment.getStatements()) {
            if (candidate.getName() == null) {
                continue;
            }
            TraceWriter writer = new TraceWriter(records, caseName, candidate,
                    runtime, CONTEXT_NAMES[caseIndex],
                    contextDeploymentIds.get(CONTEXT_NAMES[caseIndex]));
            writersByLabel.put(candidate.getName(), writer);
            if (LISTENED_CASES.contains(caseName) && "s0".equals(candidate.getName())) {
                candidate.addListener(writer);
            }
        }
    }

    private static TraceWriter requireWriter(Map<String, TraceWriter> writersByLabel,
                                             JsonObject step, String caseName) {
        String label = step.getString("statement", "");
        TraceWriter writer = writersByLabel.get(label);
        if (writer == null) {
            throw new IllegalStateException("no deployed statement " + label
                    + " for case " + caseName);
        }
        return writer;
    }

    /**
     * Dispatches the pinned selector kinds. The always-false filtered
     * selector (filterValue JSON null, mirroring
     * SupportSelectorFilteredInitTerm(null)) first runs a probe selector
     * that asserts the filter callback observed every partition — the Java
     * execution pins startTimes=[1000L] and p00=["S0_1"] — then emits the
     * empty-rows record through a fresh selector. The segmented selector is
     * rejected with InvalidContextPartitionSelector, normalized to
     * invalid-context-partition-selector.
     */
    private static void snapshotSelector(EPRuntime runtime, String caseName,
                                         int caseIndex, JsonObject step,
                                         Map<String, TraceWriter> writersByLabel,
                                         JsonArray records) {
        TraceWriter writer = requireWriter(writersByLabel, step, caseName);
        String selector = step.getString("selector", "");
        switch (selector) {
            case "ids": {
                JsonArray ids = step.get("ids").asArray();
                writer.snapshot("snapshot-selector", new ContextPartitionSelectorById() {
                    public java.util.Set<Integer> getContextPartitionIds() {
                        java.util.Set<Integer> set = new java.util.LinkedHashSet<>();
                        for (JsonValue value : ids) {
                            set.add(value.asInt());
                        }
                        return set;
                    }
                });
                break;
            }
            case "filtered": {
                String value = nullableString(step, "filterValue");
                if (value == null) {
                    ObservedSelector probe = new ObservedSelector(null);
                    drain(writer.statement.iterator(probe));
                    assertObservedAllPartitions(runtime, writer.statement,
                            writer.contextDeploymentId, CONTEXT_NAMES[caseIndex], probe);
                }
                writer.snapshot("snapshot-selector", new ObservedSelector(value));
                break;
            }
            case "segmented":
                try {
                    writer.statement.iterator(new ContextPartitionSelectorSegmented() {
                        public List<Object[]> getPartitionKeys() {
                            return null;
                        }
                    });
                    throw new IllegalStateException("expected InvalidContextPartitionSelector");
                } catch (InvalidContextPartitionSelector expected) {
                    writer.selectorError(expected.getMessage());
                }
                break;
            default:
                throw new IllegalArgumentException("unsupported selector " + selector);
        }
    }

    private static void drain(Iterator<EventBean> iterator) {
        while (iterator.hasNext()) {
            iterator.next();
        }
    }

    /**
     * Asserts the always-false filtered selector's callback visited every
     * partition: the observed startTimes and initiating-event p00 values
     * must equal the all-selector partition view (Java pins exactly
     * [1000L] and ["S0_1"] for the single open partition).
     */
    private static void assertObservedAllPartitions(EPRuntime runtime,
                                                    EPStatement statement,
                                                    String contextDeploymentId,
                                                    String contextName,
                                                    ObservedSelector selector) {
        ContextPartitionCollection collection = runtime.getContextPartitionService()
                .getContextPartitions(contextDeploymentId, contextName,
                        ContextPartitionSelectorAll.INSTANCE);
        Set<Long> expectedStartTimes = new HashSet<>();
        Set<String> expectedP00 = new HashSet<>();
        for (Map.Entry<Integer, ContextPartitionIdentifier> entry
                : collection.getIdentifiers().entrySet()) {
            ContextPartitionIdentifierInitiatedTerminated id =
                    (ContextPartitionIdentifierInitiatedTerminated) entry.getValue();
            expectedStartTimes.add(id.getStartTime());
            expectedP00.add(initiatingP00(id));
        }
        if (!expectedStartTimes.equals(selector.startTimes)
                || !expectedP00.equals(selector.p00Values)) {
            throw new IllegalStateException("filtered selector observed startTimes "
                    + selector.startTimes + " p00 " + selector.p00Values
                    + ", want " + expectedStartTimes + " / " + expectedP00);
        }
    }

    private static String initiatingP00(ContextPartitionIdentifierInitiatedTerminated id) {
        Map<String, Object> properties = id.getProperties();
        Object initiating = properties == null ? null : properties.get("s0");
        return initiating == null ? null : (String) ((EventBean) initiating).get("p00");
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", nullableString(payload, "theString"));
            event.put("intPrimitive", payload.get("intPrimitive").asInt());
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", nullableString(payload, "p00"));
            event.put("p01", nullableString(payload, "p01"));
        } else if ("SupportBean_S1".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p10", nullableString(payload, "p10"));
            event.put("p11", nullableString(payload, "p11"));
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    /**
     * Mirrors the suite's SupportScheduleHelper.scheduleCountOverall(runtime):
     * counts every schedule handle across the runtime through the services
     * context scheduling service. Reimplemented locally because the run
     * script classpath excludes regression-lib (ViewGroupMergeViewScenarioOracle
     * precedent).
     */
    private static int scheduleCountOverall(EPRuntime runtime) {
        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
        int[] count = new int[]{0};
        ScheduleVisitor visitor = visit -> count[0]++;
        spi.getServicesContext().getSchedulingService().visitSchedules(visitor);
        return count[0];
    }

    /**
     * Mirrors the suite's SupportFilterServiceHelper.getFilterSvcCountApprox:
     * the runtime-wide approximate filter-service entry count, asserted
     * internally (ContextCategoryScenarioOracle precedent) because Go has
     * no filter-service analog.
     */
    private static void assertFilterCount(EPRuntime runtime, int expected) {
        FilterServiceSPI filterService =
                (FilterServiceSPI) ((EPRuntimeSPI) runtime).getServicesContext().getFilterService();
        int actual = filterService.getFilterCountApprox();
        if (actual != expected) {
            throw new IllegalStateException("filterSvcCountApprox = " + actual
                    + ", want " + expected);
        }
    }

    /**
     * Emits one count record for the pinned introspection probes, mirroring
     * the lifecycle count-record protocol (no time, no sequence, observed
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

    private static String nullableString(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds",
                "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            JsonObject marker = object(steps.get(offset++), "case marker");
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker is not pinned for " + caseName);
            }
            for (String key : CASE_STEPS.get(caseName)) {
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

    /**
     * Renders one step as its pinned key:
     * op|statement|eventType|at|epl|payload|selector|ids|filterProperty|
     * filterValue|expectError|count|filterCount with payload, ids, count and
     * filterCount rendered as compacted JSON and a JSON-null filterValue as
     * <null>. Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "at", "epl", "payload",
                "selector", "ids", "filterProperty", "filterValue", "expectError",
                "count", "filterCount"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        JsonValue ids = step.get("ids");
        JsonValue count = step.get("count");
        JsonValue filterCount = step.get("filterCount");
        JsonValue filterValue = step.get("filterValue");
        String filterValueText = filterValue == null ? ""
                : filterValue.isNull() ? "<null>" : filterValue.asString();
        return string(step, "op") + "|" + string(step, "statement") + "|"
                + string(step, "eventType") + "|" + string(step, "at") + "|"
                + string(step, "epl") + "|" + (payload == null ? "" : payload.toString())
                + "|" + string(step, "selector") + "|" + (ids == null ? "" : ids.toString())
                + "|" + string(step, "filterProperty") + "|" + filterValueText
                + "|" + string(step, "expectError")
                + "|" + (count == null ? "" : count.toString())
                + "|" + (filterCount == null ? "" : filterCount.toString());
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
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be a JSON number");
        }
        return value.asInt();
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    /**
     * Filtered selector that records the startTime and initiating-event p00
     * of every visited partition, mirroring
     * SupportSelectorFilteredInitTerm: a null match value is always-false
     * yet still observes each partition.
     */
    private static final class ObservedSelector implements ContextPartitionSelectorFiltered {
        private final String matchP00;
        private final Set<Long> startTimes = new LinkedHashSet<>();
        private final Set<String> p00Values = new LinkedHashSet<>();

        private ObservedSelector(String matchP00) {
            this.matchP00 = matchP00;
        }

        public boolean filter(ContextPartitionIdentifier identifier) {
            ContextPartitionIdentifierInitiatedTerminated id =
                    (ContextPartitionIdentifierInitiatedTerminated) identifier;
            startTimes.add(id.getStartTime());
            p00Values.add(initiatingP00(id));
            return matchP00 != null && matchP00.equals(initiatingP00(id));
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String contextName;
        private final String contextDeploymentId;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime, String contextName,
                            String contextDeploymentId) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.contextName = contextName;
            this.contextDeploymentId = contextDeploymentId;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private void snapshot(String operation, ContextPartitionSelector selector) {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = selector == null
                    ? statement.iterator() : statement.iterator(selector);
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(events.toArray(new EventBean[0]));
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            record.add("partitions", partitions(selector));
            records.add(record);
        }

        private void selectorError(String message) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "selector-error")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("value", message.startsWith("Invalid context partition selector")
                            ? "invalid-context-partition-selector" : message);
            records.add(record);
        }

        private JsonArray partitions(ContextPartitionSelector selector) {
            ContextPartitionSelector effective = selector == null
                    ? ContextPartitionSelectorAll.INSTANCE : selector;
            ContextPartitionCollection collection = runtime.getContextPartitionService()
                    .getContextPartitions(contextDeploymentId, contextName, effective);
            List<Map.Entry<Integer, ContextPartitionIdentifier>> entries =
                    new ArrayList<>(collection.getIdentifiers().entrySet());
            entries.sort(java.util.Comparator.comparingInt(Map.Entry::getKey));
            JsonArray output = new JsonArray();
            for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
                ContextPartitionIdentifierInitiatedTerminated id =
                        (ContextPartitionIdentifierInitiatedTerminated) entry.getValue();
                JsonObject properties = new JsonObject()
                        .add("startTime", id.getStartTime())
                        .add("endTime", id.getEndTime() == null
                                ? Json.NULL : Json.value(id.getEndTime()));
                String p00 = initiatingP00(id);
                if (p00 != null) {
                    properties.add("initiating.p00", normalize(p00));
                }
                output.add(new JsonObject()
                        .add("id", entry.getKey())
                        .add("key", "start:" + id.getStartTime())
                        .add("properties", properties));
            }
            return output;
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                java.util.Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean[]) {
                // prevwindow projects the window as an event array; normalize
                // each element to the same row shape as a result stream.
                EventBean[] events = (EventBean[]) value;
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    JsonObject fields = new JsonObject();
                    String[] names = event.getEventType().getPropertyNames().clone();
                    java.util.Arrays.sort(names);
                    for (String name : names) {
                        fields.add(name, normalize(event.get(name)));
                    }
                    array.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
                return array;
            }
            if (value instanceof Map[]) {
                // Map-backed event types project prevwindow as the underlying
                // map array; normalize each map the same way.
                Map<?, ?>[] maps = (Map<?, ?>[]) value;
                JsonArray array = new JsonArray();
                for (Map<?, ?> map : maps) {
                    JsonObject fields = new JsonObject();
                    List<String> names = new ArrayList<>();
                    for (Object key : map.keySet()) {
                        names.add(String.valueOf(key));
                    }
                    java.util.Collections.sort(names);
                    for (String name : names) {
                        fields.add(name, normalize(map.get(name)));
                    }
                    array.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
                return array;
            }
            if (value instanceof Object[]) {
                Object[] objects = (Object[]) value;
                JsonArray array = new JsonArray();
                for (Object element : objects) {
                    array.add(normalize(element));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Long) {
                return Json.value(((Long) value).longValue());
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
    }
}
