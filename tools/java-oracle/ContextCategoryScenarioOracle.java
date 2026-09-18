import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.context.ContextPartitionCollection;
import com.espertech.esper.common.client.context.ContextPartitionIdentifier;
import com.espertech.esper.common.client.context.ContextPartitionIdentifierCategory;
import com.espertech.esper.common.client.context.ContextPartitionSelector;
import com.espertech.esper.common.client.context.ContextPartitionSelectorAll;
import com.espertech.esper.common.client.context.ContextPartitionSelectorById;
import com.espertech.esper.common.client.context.ContextPartitionSelectorCategory;
import com.espertech.esper.common.client.context.ContextPartitionSelectorFiltered;
import com.espertech.esper.common.client.context.ContextPartitionSelectorSegmented;
import com.espertech.esper.common.client.context.EPContextPartitionService;
import com.espertech.esper.common.client.context.InvalidContextPartitionSelector;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.internal.context.airegistry.StatementAIResourceRegistry;
import com.espertech.esper.common.internal.context.util.StatementContext;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.filtersvcimpl.FilterServiceSPI;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;


import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.Iterator;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Replays the context-category scenario against the pinned Esper runtime and
 * emits the normalized trace the Go runner mirrors. Deploy steps compile the
 * byte-exact EPL with the accumulated module path (mode "soda" routes through
 * eplToModel like the ord-5 second cycle), listener records capture IR pairs
 * on the statements the Java execution listens on (s0 in every case except
 * partition-selection), snapshot steps iterate the named statement (mode
 * "any" sorts rows canonically; mode "admin:<probe>" emits context-partition
 * service reads and statement-property assertions instead of rows), and
 * snapshot-selector steps exercise by-id, category, filtered and segmented
 * selectors. Build-error probes compile with or without the path per
 * compileWithoutPath and assert the pinned Java prefix; the record value
 * carries the asserted prefix. Selector-error probes assert the pinned
 * InvalidContextPartitionSelector prefix. The w-context-props internal
 * assertions (filterSvcCountApprox and eager agent-instance counts) run
 * inside the oracle only, matching the Java execution's assertThat blocks.
 */
public class ContextCategoryScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "context-category";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextCategory.java";
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5, 6, 7, 8};
    private static final String[] RUNTIME_IDS = {
        "java-runtime-edb899dd318dc4e8711e",
        "java-runtime-440efde2e13b0063a968",
        "java-runtime-cbe8b6c2ba86887f6807",
        "java-runtime-ad038edc0893e86eba46",
        "java-runtime-fe484daf28031390497b",
        "java-runtime-96843edb4ce366e1d74e",
        "java-runtime-51b59dd76ca097972003",
        "java-runtime-629da2acd413b0688d68",
        "java-runtime-76b9f0c6ca0a53be2ab0",
    };
    private static final String[] EXECUTION_NAMES = {
        "ContextCategorySceneOne",
        "ContextCategorySceneTwo",
        "ContextCategoryWContextProps",
        "ContextCategoryBooleanExprFilter",
        "ContextCategoryContextPartitionSelection",
        "ContextCategorySingleCategorySODAPrior",
        "ContextCategoryInvalid",
        "ContextCategoryDeclaredExpr{isAlias=true}",
        "ContextCategoryDeclaredExpr{isAlias=false}",
    };
    private static final String[] STATIC_IDS = {
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
        "java-2275d4c280d0acadad30",
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
        "scene-one",
        "scene-two",
        "w-context-props",
        "boolean-expr-filter",
        "partition-selection",
        "single-category-soda-prior",
        "invalid",
        "declared-expr-alias",
        "declared-expr-call",
    };
    private static final String[] CASE_OBSERVATIONS = {
        "listener+admin; one module deploys the context (trailing space after 'cat2 ') and s0; admin pins statement names [s0], nesting level 1, partition ids {0,1} and count 2, null context properties on 'context' and CategoryContext/module-deployment properties on 's0'; A and B deliver per-category counts, C matches nothing",
        "listener+admin; 'group by' spelling; partition identifiers carry labels {cat1,cat2} with id 0 = cat1 and per-id context properties; c1..c5 project theString, sum, context.label, context.name and context.id; the cumulative sum is per category",
        "listener+snapshot+admin; three categories (between 10 and 20 inclusive) allocate eagerly; iterators over all partitions include empty ones with null sums; the oracle asserts filterSvcCountApprox==3 and 3 eager agent instances; context count drops to 0 after undeploying the module containing s0",
        "listener+admin; two deployments share the module path; 'like' patterns A%/B%/C% route to agroup/bgroup/cgroup; s0's CONTEXTDEPLOYMENTID is the ctx deployment from the earlier module",
        "snapshot+snapshot-selector+selector-error; keepall group-by-theString statement with no listener; selectors pin by-id, by-category {grp1,grp3}, filtered label match, null and empty category sets, an always-false collecting selector observing all three labels, and a segmented selector rejected as an invalid context partition selector",
        "listener+admin; single category; prior(1) is per-partition so E1(5) yields null then E1(4) yields 5 while non-matching E2(20) neither fires nor advances prior; the second deploy+send cycle (mode 'soda') mirrors the Java eplToModel round-trip; context count returns to 0 after each undeploy-all",
        "compile-error; probes pin a bad filter property (dummy=1), a non-boolean predicate (intPrimitive), and a statement stream type (SupportBean_S0) not listed in the category context; the record value pins the Go wording while the oracle asserts the Java prefix",
        "listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the alias-for form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
        "listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the script-call form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
    };
    private static final String[] CASE_EPLS = {
        "@name('s0') context CategoryContext select count(*) as c0, context.label as c1 from SupportBean",
        "@Name('s0') context CtxCategory select theString as c1, sum(intPrimitive) as c2, context.label as c3, context.name as c4, context.id as c5 from SupportBean",
        "@name('s0') context CategorizedContext select context.name as c0, context.label as c1, sum(intPrimitive) as c2 from SupportBean",
        "@name('s0') context Ctx600a select context.label as c0, count(*) as c1 from SupportBean",
        "@name('s0') context MyCtx select context.id as c0, context.label as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString",
        "@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean",
        "context ACtx select * from SupportBean_S0",
        "@name('s0') expression getLabelThree alias for { context.label } context MyCtx select getLabelOne as c0, getLabelTwo as c1, getLabelThree as c2 from SupportBean",
        "@name('s0') expression getLabelThree { context.label } context MyCtx select getLabelOne() as c0, getLabelTwo() as c1, getLabelThree() as c2 from SupportBean",
    };

    /**
     * The complete step sequence per case as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|ids|selector|filterProperty|filterValue keys. Deploy steps carry
     * the byte-exact EPL text; mode "soda" marks the ord-5 second cycle whose
     * Java execution compiles through eplToModel; mode "clear-path" on
     * undeploy-all marks the RegressionPath.clear() between the two ord-5
     * cycles; mode "admin:<probe>" on snapshot pins context-partition service
     * reads instead of statement iterators; mode "any" pins an any-order
     * iterator assertion. Java's milestone calls are documented no-ops and
     * carry no steps.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("scene-one", new String[]{
            "deploy|module|||@name('context') create context CategoryContext\ngroup theString = 'A' as cat1,\ngroup theString = 'B' as cat2 \nfrom SupportBean;\n@name('s0') context CategoryContext select count(*) as c0, context.label as c1 from SupportBean;\n||||||||",
            "deployed|module|||||||||||",
            "snapshot|context|CategoryContext||||||admin:context-admin||||",
            "snapshot|context|||||||admin:statement-props||||",
            "snapshot|s0|||||||admin:statement-props||||",
            "send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":1}|||||||",
            "send|||SupportBean||{\"theString\":\"C\",\"intPrimitive\":2}|||||||",
            "send|||SupportBean||{\"theString\":\"B\",\"intPrimitive\":3}|||||||",
            "send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":4}|||||||",
            "send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":6}|||||||",
            "send|||SupportBean||{\"theString\":\"B\",\"intPrimitive\":5}|||||||",
            "send|||SupportBean||{\"theString\":\"C\",\"intPrimitive\":7}|||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("scene-two", new String[]{
            "deploy|module|||@Name('CTX') create context CtxCategory group by intPrimitive > 0 as cat1,group by intPrimitive < 0 as cat2 from SupportBean;\n@Name('s0') context CtxCategory select theString as c1, sum(intPrimitive) as c2, context.label as c3, context.name as c4, context.id as c5 from SupportBean;\n||||||||",
            "deployed|module|||||||||||",
            "snapshot|CTX|CtxCategory||||||admin:partition-info||||",
            "send|||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":1}|||||||",
            "snapshot|CTX|CtxCategory||||||admin:partition-info||||",
            "send|||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":-2}|||||||",
            "send|||SupportBean||{\"theString\":\"G3\",\"intPrimitive\":3}|||||||",
            "send|||SupportBean||{\"theString\":\"G4\",\"intPrimitive\":-4}|||||||",
            "send|||SupportBean||{\"theString\":\"G5\",\"intPrimitive\":5}|||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("w-context-props", new String[]{
            "deploy|module|||@Name('context') create context CategorizedContext group intPrimitive < 10 as cat1, group intPrimitive between 10 and 20 as cat2, group intPrimitive > 20 as cat3 from SupportBean;\n@name('s0') context CategorizedContext select context.name as c0, context.label as c1, sum(intPrimitive) as c2 from SupportBean;\n||||||||",
            "deployed|module|||||||||||",
            "snapshot|context|CategorizedContext||||||admin:partition-info||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
            "snapshot|s0|||||||any||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":4}|||||||",
            "send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":11}|||||||",
            "send|||SupportBean||{\"theString\":\"E4\",\"intPrimitive\":25}|||||||",
            "send|||SupportBean||{\"theString\":\"E5\",\"intPrimitive\":25}|||||||",
            "send|||SupportBean||{\"theString\":\"E6\",\"intPrimitive\":3}|||||||",
            "snapshot|s0|||||||any||||",
            "snapshot|context|||||||admin:context-count||||",
            "undeploy|module|||||||||||",
            "snapshot|context|||||||admin:context-count||||",
        });
        CASE_STEPS.put("boolean-expr-filter", new String[]{
            "deploy|ctx|||@name('ctx') @public create context Ctx600a group by theString like 'A%' as agroup, group by theString like 'B%' as bgroup, group by theString like 'C%' as cgroup from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|s0|||@name('s0') context Ctx600a select context.label as c0, count(*) as c1 from SupportBean||||||||",
            "deployed|s0|||||||||||",
            "snapshot|s0|||||||admin:statement-props||||",
            "send|||SupportBean||{\"theString\":\"B1\",\"intPrimitive\":1}|||||||",
            "send|||SupportBean||{\"theString\":\"A1\",\"intPrimitive\":1}|||||||",
            "send|||SupportBean||{\"theString\":\"B171771\",\"intPrimitive\":1}|||||||",
            "send|||SupportBean||{\"theString\":\"A  x\",\"intPrimitive\":1}|||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("partition-selection", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < -5 as grp1, group by intPrimitive between -5 and +5 as grp2, group by intPrimitive > 5 as grp3 from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|s0|||@name('s0') context MyCtx select context.id as c0, context.label as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString||||||||",
            "deployed|s0|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":-5}|||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":2}|||||||",
            "send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":-100}|||||||",
            "send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":-8}|||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":60}|||||||",
            "snapshot|s0|||||||any||||",
            "snapshot|ctx|MyCtx||||||admin:context-props||||",
            "snapshot-selector|s0||||||||[1]|ids||",
            "snapshot-selector|s0||||[\"grp1\",\"grp3\"]|||||filtered|labels|",
            "snapshot-selector|s0|||||||||filtered|label|grp1",
            "snapshot-selector|s0||||null|||||filtered|labels|",
            "snapshot-selector|s0||||[]|||||filtered|labels|",
            "snapshot-selector|s0|||||||||filtered|label|",
            "snapshot-selector|s0|||||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered, ContextPartitionSelectorById, ContextPartitionSelectorCategory] interfaces but received ||||segmented||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("single-category-soda-prior", new String[]{
            "deploy|ctx|||@Name('context') @public create context CategorizedContext as group intPrimitive<10 as cat1 from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|s0|||@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean||||||||",
            "deployed|s0|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":4}|||||||",
            "snapshot|context|||||||admin:context-count||||",
            "undeploy-all||||||||clear-path||||",
            "snapshot|context|||||||admin:context-count||||",
            "deploy|ctx-soda|||@Name('context') @public create context CategorizedContext as group intPrimitive<10 as cat1 from SupportBean||||soda||||",
            "deployed|ctx-soda|||||||||||",
            "deploy|s0-soda|||@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean||||soda||||",
            "deployed|s0-soda|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":4}|||||||",
            "snapshot|context|||||||admin:context-count||||",
            "undeploy-all||||||||||||",
            "snapshot|context|||||||admin:context-count||||",
        });
        CASE_STEPS.put("invalid", new String[]{
            "build-error|bad-filter-prop|||create context ACtx group theString is not null as cat1 from SupportBean(dummy = 1)||Failed to validate filter expression 'dummy=1': Property named 'dummy' is not valid in any stream [|1|||||",
            "build-error|non-boolean-predicate|||create context ACtx group intPrimitive as grp1 from SupportBean||Filter expression not returning a boolean value: 'intPrimitive' [|1|||||",
            "deploy|ctx|||@public create context ACtx group intPrimitive < 10 as cat1 from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "build-error|statement-stream-type|||context ACtx select * from SupportBean_S0||Category context 'ACtx' requires that any of the events types that are listed in the category context also appear in any of the filter expressions of the statement [||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("declared-expr-alias", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|expr-1|||@name('expr-1') @public create expression getLabelOne { context.label }||||||||",
            "deployed|expr-1|||||||||||",
            "deploy|expr-2|||@name('expr-2') @public create expression getLabelTwo { 'x'||context.label||'x' }||||||||",
            "deployed|expr-2|||||||||||",
            "deploy|s0|||@name('s0') expression getLabelThree alias for { context.label } context MyCtx select getLabelOne as c0, getLabelTwo as c1, getLabelThree as c2 from SupportBean||||||||",
            "deployed|s0|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":-2}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":1}|||||||",
            "undeploy-all||||||||||||",
        });
        CASE_STEPS.put("declared-expr-call", new String[]{
            "deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean||||||||",
            "deployed|ctx|||||||||||",
            "deploy|expr-1|||@name('expr-1') @public create expression getLabelOne { context.label }||||||||",
            "deployed|expr-1|||||||||||",
            "deploy|expr-2|||@name('expr-2') @public create expression getLabelTwo { 'x'||context.label||'x' }||||||||",
            "deployed|expr-2|||||||||||",
            "deploy|s0|||@name('s0') expression getLabelThree { context.label } context MyCtx select getLabelOne() as c0, getLabelTwo() as c1, getLabelThree() as c2 from SupportBean||||||||",
            "deployed|s0|||||||||||",
            "send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":-2}|||||||",
            "send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":1}|||||||",
            "undeploy-all||||||||||||",
        });
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextCategoryScenarioOracle <scenario.json>");
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        validateScenario(scenario);
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ContextCategoryScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, String> deploymentIds = new HashMap<>();
            Map<String, String> deploymentLabels = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            Map<String, String> contextDeploymentIds = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
            Set<EPStatement> listened = Collections.newSetFromMap(new IdentityHashMap<>());

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (op) {
                    case "deploy":
                        deployStep(runtime, config, caseName, step, records,
                            deploymentIds, deploymentLabels, statementsByName,
                            contextDeploymentIds, deployedModules, sequences, listened);
                        break;
                    case "deployed": {
                        String label = step.getString("statement", "");
                        String key = label + ":deployed";
                        int seq = sequences.getOrDefault(key, 0) + 1;
                        sequences.put(key, seq);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", seq);
                        record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "snapshot":
                        snapshotStep(runtime, caseName, step, records, statementsByName,
                            deploymentIds, deploymentLabels, contextDeploymentIds);
                        break;
                    case "snapshot-selector":
                        snapshotSelectorStep(runtime, caseName, step, records, statementsByName,
                            contextDeploymentIds);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, config, caseName, step, records, deployedModules);
                        break;
                    case "undeploy": {
                        String owner = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(owner);
                        if (deploymentId == null) {
                            throw new IllegalStateException("no deployment for statement " + owner);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        deploymentIds.remove(owner);
                        deploymentLabels.remove(deploymentId);
                        statementsByName.remove(owner);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        deploymentLabels.clear();
                        statementsByName.clear();
                        contextDeploymentIds.clear();
                        // clear-path marks the RegressionPath.clear() between
                        // the two ord-5 cycles; plain undeploy-all leaves the
                        // accumulated module path intact like Java's.
                        if ("clear-path".equals(step.getString("mode", ""))) {
                            deployedModules.clear();
                        }
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
     * Compiles and deploys one labeled module. Deploys accumulate into the
     * module path like RegressionPath; mode "soda" compiles through
     * eplToModel like the ord-5 second cycle. The create-context statement's
     * context name is parsed from the EPL so the context-partition service
     * reads can resolve the context's deployment id. Listeners attach to s0
     * for the cases whose Java execution calls addListener.
     */
    private static void deployStep(EPRuntime runtime, Configuration config, String caseName, JsonObject step,
                                   List<JsonObject> records, Map<String, String> deploymentIds,
                                   Map<String, String> deploymentLabels,
                                   Map<String, EPStatement> statementsByName,
                                   Map<String, String> contextDeploymentIds,
                                   List<EPCompiled> deployedModules,
                                   Map<String, Integer> sequences, Set<EPStatement> listened) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompiled compiled;
            if ("soda".equals(step.getString("mode", ""))) {
                EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, config);
                if (!epl.equals(model.toEPL())) {
                    throw new IllegalStateException("soda round-trip drift for " + label
                        + ": [" + model.toEPL() + "]");
                }
                com.espertech.esper.common.client.module.Module module =
                    new com.espertech.esper.common.client.module.Module();
                module.getItems().add(new com.espertech.esper.common.client.module.ModuleItem(model));
                module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module, compilerArgs);
            } else {
                compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            deployedModules.add(compiled);
            deploymentIds.put(label, deployment.getDeploymentId());
            deploymentLabels.put(deployment.getDeploymentId(), label);
            String contextName = parseContextName(epl);
            if (contextName != null) {
                contextDeploymentIds.put(contextName, deployment.getDeploymentId());
            }
            for (EPStatement stmt : deployment.getStatements()) {
                statementsByName.put(stmt.getName(), stmt);
                String name = stmt.getName();
                if (listenedNames(caseName).contains(name) && !listened.contains(stmt)) {
                    listened.add(stmt);
                    stmt.addListener(listener(caseName, name, sequences, records, runtime));
                }
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    /** The statement names each case listens on, mirroring env.addListener calls. */
    private static Set<String> listenedNames(String caseName) {
        if ("partition-selection".equals(caseName)) {
            return Collections.emptySet();
        }
        return new HashSet<>(Collections.singletonList("s0"));
    }

    /** Parses the context name out of a create-context EPL module. */
    private static String parseContextName(String epl) {
        Matcher matcher = Pattern.compile("create context\\s+(\\w+)").matcher(epl);
        return matcher.find() ? matcher.group(1) : null;
    }

    /** Sends one event bean decoded from the step payload. */
    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull() ? null : theString.asString());
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null && !intPrimitive.isNull()) {
                    bean.setIntPrimitive(intPrimitive.asInt());
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                JsonValue p00 = payload.get("p00");
                runtime.getEventService().sendEventBean(
                    new SupportBean_S0(payload.get("id").asInt(),
                        p00 == null || p00.isNull() ? null : p00.asString()), type);
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    /**
     * Dispatches on the step mode: "admin:<probe>" emits an admin record
     * mirroring the Java execution's context-partition service reads and
     * statement-property assertions; "any" (or empty) emits one snapshot
     * record mirroring the statement iterator with rows in canonical
     * any-order and the statement's category partitions.
     */
    private static void snapshotStep(EPRuntime runtime, String caseName, JsonObject step,
                                     List<JsonObject> records, Map<String, EPStatement> statementsByName,
                                     Map<String, String> deploymentIds,
                                     Map<String, String> deploymentLabels,
                                     Map<String, String> contextDeploymentIds) {
        String mode = step.getString("mode", "");
        if (mode.startsWith("admin:")) {
            adminStep(runtime, caseName, step, records, statementsByName,
                deploymentIds, deploymentLabels, contextDeploymentIds);
            return;
        }
        String name = step.getString("statement", "");
        EPStatement statement = statementsByName.get(name);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot " + name);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = sortedRows(iterateRows(statement.iterator()));
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        JsonArray partitions = partitions(runtime, statement, contextDeploymentIds, ContextPartitionSelectorAll.INSTANCE);
        if (partitions.size() > 0) {
            record.add("partitions", partitions);
        }
        records.add(record);
    }

    /**
     * Emits one {"operation":"admin"} record for the pinned probe:
     * context-admin mirrors getContextStatementNames/getContextNestingLevel/
     * getContextPartitionIds/getContextPartitionCount; statement-props
     * mirrors the CONTEXTNAME/CONTEXTDEPLOYMENTID statement properties
     * (deployment ids normalize to the deploy-step label); partition-info
     * and context-props mirror getContextPartitions/getIdentifier/
     * getContextProperties; and context-count mirrors
     * SupportContextMgmtHelper.getContextCount.
     */
    private static void adminStep(EPRuntime runtime, String caseName, JsonObject step,
                                  List<JsonObject> records, Map<String, EPStatement> statementsByName,
                                  Map<String, String> deploymentIds,
                                  Map<String, String> deploymentLabels,
                                  Map<String, String> contextDeploymentIds) {
        String name = step.getString("statement", "");
        String contextName = step.getString("name", "");
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "admin");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        if (!contextName.isEmpty()) {
            record.add("name", contextName);
        }
        EPContextPartitionService service = runtime.getContextPartitionService();
        switch (mode(step)) {
            case "admin:context-admin": {
                String depId = contextDeploymentIds.get(contextName);
                String[] statementNames = service.getContextStatementNames(depId, contextName);
                int level = service.getContextNestingLevel(depId, contextName);
                Set<Integer> ids = service.getContextPartitionIds(depId, contextName, ContextPartitionSelectorAll.INSTANCE);
                long count = service.getContextPartitionCount(depId, contextName);
                JsonObject value = new JsonObject();
                JsonArray names = new JsonArray();
                for (String statementName : statementNames) {
                    names.add(statementName);
                }
                value.add("statementNames", names);
                value.add("nestingLevel", level);
                value.add("partitionCount", (int) count);
                record.add("value", value);
                record.add("partitions", partitions(service, depId, contextName, ContextPartitionSelectorAll.INSTANCE));
                break;
            }
            case "admin:statement-props": {
                EPStatement statement = statementsByName.get(name);
                JsonObject value = new JsonObject();
                Object ctxName = statement == null ? null : statement.getProperty(StatementProperty.CONTEXTNAME);
                Object ctxDepId = statement == null ? null : statement.getProperty(StatementProperty.CONTEXTDEPLOYMENTID);
                if (ctxName == null) {
                    value.add("contextName", new JsonObject().add("state", "null"));
                } else {
                    value.add("contextName", String.valueOf(ctxName));
                }
                if (ctxDepId == null) {
                    value.add("contextDeploymentId", new JsonObject().add("state", "null"));
                } else {
                    String label = deploymentLabels.get(String.valueOf(ctxDepId));
                    value.add("contextDeploymentId", label == null ? String.valueOf(ctxDepId) : label);
                }
                record.add("value", value);
                break;
            }
            case "admin:partition-info":
            case "admin:context-props": {
                String depId = contextDeploymentIds.get(contextName);
                record.add("partitions", partitionProperties(service, depId, contextName));
                if ("w-context-props".equals(caseName)) {
                    // The Java execution's assertThat pins three eager filter
                    // entries and three eager agent instances at deploy.
                    assertFilterAndAgentCounts(runtime, statementsByName, 3, 3);
                }
                break;
            }
            case "admin:context-count": {
                EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
                int count = spi.getServicesContext().getContextManagementService().getContextCount();
                record.add("value", count);
                if ("w-context-props".equals(caseName)) {
                    // The Java execution's assertThat pins the filter count
                    // alongside the context count (3 live, 0 after undeploy).
                    assertFilterAndAgentCounts(runtime, statementsByName, count == 0 ? 0 : 3, -1);
                }
                break;
            }

            default:
                throw new IllegalStateException("unknown admin probe " + mode(step));
        }
        records.add(record);
    }

    /**
     * Replays one selector-targeted iterator step, mirroring the Java
     * execution's statement.iterator(selector) calls: "ids" maps to
     * ContextPartitionSelectorById, "filtered" with filterProperty "labels"
     * maps to ContextPartitionSelectorCategory over the payload label array
     * (null and empty sets both select nothing), "filtered" with
     * filterProperty "label" maps to MySelectorFilteredCategory collecting
     * observed labels, and "segmented" asserts the invalid-selector
     * rejection.
     */
    private static void snapshotSelectorStep(EPRuntime runtime, String caseName, JsonObject step,
                                             List<JsonObject> records,
                                             Map<String, EPStatement> statementsByName,
                                             Map<String, String> contextDeploymentIds) {
        String name = step.getString("statement", "");
        EPStatement statement = statementsByName.get(name);
        if (statement == null) {
            throw new IllegalStateException("no statement for snapshot-selector " + name);
        }
        String selectorKind = step.getString("selector", "");
        ContextPartitionSelector selector;
        MySelectorFilteredCategory collector = null;
        switch (selectorKind) {
            case "ids": {
                Set<Integer> ids = new HashSet<>();
                JsonValue raw = step.get("ids");
                if (raw != null && raw.isArray()) {
                    for (JsonValue id : raw.asArray()) {
                        ids.add(id.asInt());
                    }
                }
                selector = (ContextPartitionSelectorById) () -> ids;
                break;
            }
            case "filtered": {
                String property = step.getString("filterProperty", "");
                if ("labels".equals(property)) {
                    Set<String> labels = null;
                    JsonValue payload = step.get("payload");
                    if (payload != null && payload.isArray()) {
                        labels = new HashSet<>();
                        for (JsonValue label : payload.asArray()) {
                            labels.add(label.asString());
                        }
                    }
                    final Set<String> labelSet = labels;
                    selector = (ContextPartitionSelectorCategory) () -> labelSet;
                } else if ("label".equals(property)) {
                    String match = step.getString("filterValue", "");
                    collector = new MySelectorFilteredCategory(match.isEmpty() ? null : match);
                    selector = collector;
                } else {
                    throw new IllegalStateException("filtered selector property " + property + " is not pinned");
                }
                break;
            }
            case "segmented":
                selector = new ContextPartitionSelectorSegmented() {
                    public List<Object[]> getPartitionKeys() {
                        return null;
                    }
                };
                break;
            default:
                throw new IllegalStateException("unsupported selector " + selectorKind);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot-selector");
        record.add("statement", name);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
            runtime.getEventService().getCurrentTime()).toString());
        try {
            Iterator<EventBean> it = statement.iterator(selector);
            JsonArray rows = sortedRows(iterateRows(it));
            if (rows.size() > 0) {
                record.add("new", rows);
            }
            // The null-match collector throws on a second filter pass (the
            // Java execution only iterates it once), so partitions are only
            // collected when the selector can be re-applied safely.
            if (collector == null || collector.matchCategory != null) {
                JsonArray partitions = partitions(runtime, statement, contextDeploymentIds, selector);
                if (partitions.size() > 0) {
                    record.add("partitions", partitions);
                }
            }
            if (collector != null) {
                JsonObject value = new JsonObject();
                JsonArray observed = new JsonArray();
                for (Object label : new TreeSet<>(Arrays.asList(collector.getCategories()))) {
                    observed.add(String.valueOf(label));
                }
                value.add("observedLabels", observed);
                record.add("value", value);
            }
        } catch (InvalidContextPartitionSelector ex) {
            String expected = step.getString("expectError", "");
            if (!expected.isEmpty() && !ex.getMessage().startsWith(expected)) {
                throw new IllegalStateException("selector-error message drift for " + selectorKind
                    + ": expected prefix [" + expected + "] got [" + ex.getMessage() + "]");
            }
            record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "selector-error");
            record.add("statement", name);
            record.add("sequence", 0);
            record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
            if (!expected.isEmpty()) {
                record.add("value", expected);
            }
        }
        records.add(record);
    }

    /**
     * Compiles an expected-invalid statement, mirroring env.tryInvalidCompile:
     * compileWithoutPath steps compile without the accumulated module path
     * (the first two invalid probes), other probes compile with the path.
     * The compile must fail and the message must start with the pinned
     * prefix; the record value carries the asserted prefix.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration config, String caseName,
                                       JsonObject step, List<JsonObject> records,
                                       List<EPCompiled> deployedModules) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            if (!step.getBoolean("compileWithoutPath", false)) {
                for (EPCompiled deployed : deployedModules) {
                    compilerArgs.getPath().add(deployed);
                }
            }
            EPCompilerProvider.getCompiler().compile(step.getString("epl", ""), compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
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

    /** Iterates one statement iterator into a row list. */
    private static List<JsonObject> iterateRows(Iterator<EventBean> it) {
        List<JsonObject> rows = new ArrayList<>();
        while (it.hasNext()) {
            rows.add(row(it.next()));
        }
        return rows;
    }

    /** Sorts rows by their compact field rendering for the any-order pins. */
    private static JsonArray sortedRows(List<JsonObject> rows) {
        rows.sort((left, right) -> left.get("fields").toString().compareTo(right.get("fields").toString()));
        JsonArray array = new JsonArray();
        for (JsonObject row : rows) {
            array.add(row);
        }
        return array;
    }

    /**
     * Renders the statement-scoped partition identifiers for snapshot
     * records: one record per ContextPartitionIdentifierCategory carrying
     * its id, category key and label.
     */
    private static JsonArray partitions(EPRuntime runtime, EPStatement statement,
                                        Map<String, String> contextDeploymentIds,
                                        ContextPartitionSelector selector) {
        String contextName = String.valueOf(statement.getProperty(StatementProperty.CONTEXTNAME));
        String depId = contextDeploymentIds.get(contextName);
        return partitions(runtime.getContextPartitionService(), depId, contextName, selector);
    }

    private static JsonArray partitions(EPContextPartitionService service, String depId, String contextName,
                                        ContextPartitionSelector selector) {
        ContextPartitionCollection collection = service.getContextPartitions(depId, contextName, selector);
        JsonArray array = new JsonArray();
        List<Map.Entry<Integer, ContextPartitionIdentifier>> entries =
            new ArrayList<>(collection.getIdentifiers().entrySet());
        entries.sort(Map.Entry.comparingByKey());
        for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
            ContextPartitionIdentifierCategory id = (ContextPartitionIdentifierCategory) entry.getValue();
            JsonObject partition = new JsonObject();
            partition.add("id", entry.getKey());
            partition.add("key", "category:" + id.getLabel());
            JsonObject properties = new JsonObject();
            properties.add("label", id.getLabel());
            partition.add("properties", properties);
            array.add(partition);
        }
        return array;
    }

    /**
     * Asserts the w-context-props internal counts the Java execution pins in
     * its assertThat blocks: the filter-service count approximation and the
     * eager agent-instance count on s0 (skipped when agentCount is negative).
     */
    private static void assertFilterAndAgentCounts(EPRuntime runtime,
                                                   Map<String, EPStatement> statementsByName,
                                                   int filterCount, int agentCount) {
        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
        FilterServiceSPI filterService = (FilterServiceSPI) spi.getServicesContext().getFilterService();
        int actualFilter = filterService.getFilterCountApprox();
        if (actualFilter != filterCount) {
            throw new IllegalStateException("filterSvcCountApprox = " + actualFilter
                + ", want " + filterCount);
        }
        if (agentCount < 0) {
            return;
        }
        EPStatement statement = statementsByName.get("s0");
        if (statement == null) {
            throw new IllegalStateException("no s0 statement for agent-instance count");
        }
        StatementContext context = ((EPStatementSPI) statement).getStatementContext();
        StatementAIResourceRegistry registry = context.getStatementAIResourceRegistry();
        int actualAgents = registry.getAgentInstanceAggregationService().getInstanceCount();
        if (actualAgents != agentCount) {
            throw new IllegalStateException("agent-instance count = " + actualAgents
                + ", want " + agentCount);
        }
    }

    /**
     * Renders the admin getContextProperties view: name, id and label per
     * partition, mirroring SupportContextPropUtil.assertContextProps.
     */
    private static JsonArray partitionProperties(EPContextPartitionService service, String depId,
                                                 String contextName) {
        ContextPartitionCollection collection =
            service.getContextPartitions(depId, contextName, ContextPartitionSelectorAll.INSTANCE);
        JsonArray array = new JsonArray();
        List<Map.Entry<Integer, ContextPartitionIdentifier>> entries =
            new ArrayList<>(collection.getIdentifiers().entrySet());
        entries.sort(Map.Entry.comparingByKey());
        for (Map.Entry<Integer, ContextPartitionIdentifier> entry : entries) {
            Map<String, Object> props = service.getContextProperties(depId, contextName, entry.getKey());
            JsonObject partition = new JsonObject();
            partition.add("id", entry.getKey());
            partition.add("key", "category:" + props.get("label"));
            JsonObject properties = new JsonObject();
            properties.add("id", (Integer) props.get("id"));
            properties.add("label", String.valueOf(props.get("label")));
            properties.add("name", String.valueOf(props.get("name")));
            partition.add("properties", properties);
            array.add(partition);
        }
        return array;
    }

    /** Per-statement listener emitting one IR-pair record per callback. */
    private static UpdateListener listener(String caseName, String statementName,
                                           Map<String, Integer> sequences, List<JsonObject> records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            int sequence = sequences.getOrDefault(statementName, 0) + 1;
            sequences.put(statementName, sequence);
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

    /** Canonical row rendering with sorted property names for stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: null as the tagged {"state":"null"} object,
     * integral numbers as JSON numbers, other numbers as doubles, booleans
     * and characters passthrough, everything else stringified.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        if (value instanceof Iterable) {
            JsonArray array = new JsonArray();
            for (Object element : (Iterable<?>) value) {
                array.add(normalize(element));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Deepest non-null cause message, the canonical cross-runtime error text. */
    private static String rootCauseMessage(Throwable throwable) {
        Throwable current = throwable;
        while (true) {
            Throwable cause = current.getCause();
            if (cause == null || cause == current) {
                break;
            }
            current = cause;
        }
        return current.getMessage();
    }

    private static String mode(JsonObject step) {
        return step.getString("mode", "");
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
            "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = scenario.get("cases").asArray();
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("cases must contain exactly " + CASES.length + " entries");
        }
        for (int i = 0; i < CASES.length; i++) {
            JsonObject entry = cases.get(i).asObject();
            requireFields(entry, "case", "ordinal", "runtimeId", "executionName", "observation", "epl");
            if (!CASES[i].equals(string(entry, "case"))
                    || ORDINALS[i] != entry.getInt("ordinal", -1)
                    || !RUNTIME_IDS[i].equals(string(entry, "runtimeId"))
                    || !EXECUTION_NAMES[i].equals(string(entry, "executionName"))
                    || !CASE_OBSERVATIONS[i].equals(string(entry, "observation"))
                    || !CASE_EPLS[i].equals(string(entry, "epl"))) {
                throw new IllegalArgumentException("case " + i + " metadata is not pinned");
            }
        }

        JsonArray steps = scenario.get("steps").asArray();
        int offset = 0;
        for (String caseName : CASES) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("missing case " + caseName);
            }
            JsonObject marker = steps.get(offset).asObject();
            if (!"case".equals(string(marker, "op")) || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("step " + offset + " is not the " + caseName + " case marker");
            }
            offset++;
            String[] want = CASE_STEPS.get(caseName);
            if (offset + want.length > steps.size()) {
                throw new IllegalArgumentException("case " + caseName + " is truncated");
            }
            for (String pinned : want) {
                String key = stepKey(steps.get(offset).asObject());
                if (!pinned.equals(key)) {
                    throw new IllegalArgumentException("case " + caseName + " step " + offset
                        + " = [" + key + "], want [" + pinned + "]");
                }
                offset++;
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has " + (steps.size() - offset) + " trailing steps");
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|ids|selector|filterProperty|filterValue with the payload compacted
     * and ids rendered as a JSON array. Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
            "op", "case", "statement", "name", "eventType", "epl", "payload",
            "expectError", "compileWithoutPath", "mode", "ids",
            "selector", "filterProperty", "filterValue"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        String payload = "";
        JsonValue payloadValue = step.get("payload");
        if (payloadValue != null) {
            payload = payloadValue.toString();
        }
        String ids = "";
        JsonValue idsValue = step.get("ids");
        if (idsValue != null) {
            ids = idsValue.toString();
        }
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
            + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payload
            + "|" + string(step, "expectError") + "|" + cwp
            + "|" + string(step, "mode") + "|" + ids
            + "|" + string(step, "selector") + "|" + string(step, "filterProperty")
            + "|" + string(step, "filterValue");
    }

    private static void requireFields(JsonObject object, String... names) {
        if (object.names().size() != names.length) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("JSON object is missing field " + name);
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be a string array");
        }
        JsonArray array = value.asArray();
        if (array.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(array.get(i).asString())) {
                throw new IllegalArgumentException(name + " is not pinned");
            }
        }
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        return value == null || value.isNull() ? "" : value.asString();
    }

    /**
     * Mirrors MySelectorFilteredCategory: visits every partition descriptor,
     * records the observed labels, and accepts only the pinned match label
     * (null means never match).
     */
    private static class MySelectorFilteredCategory implements ContextPartitionSelectorFiltered {
        private final String matchCategory;
        private final List<Object> categories = new ArrayList<>();
        private final LinkedHashSet<Integer> cpids = new LinkedHashSet<>();

        private MySelectorFilteredCategory(String matchCategory) {
            this.matchCategory = matchCategory;
        }

        public boolean filter(ContextPartitionIdentifier contextPartitionIdentifier) {
            ContextPartitionIdentifierCategory id = (ContextPartitionIdentifierCategory) contextPartitionIdentifier;
            if (matchCategory == null && cpids.contains(id.getContextPartitionId())) {
                throw new RuntimeException("Already exists context id: " + id.getContextPartitionId());
            }
            cpids.add(id.getContextPartitionId());
            categories.add(id.getLabel());
            return matchCategory != null && matchCategory.equals(id.getLabel());
        }

        Object[] getCategories() {
            return categories.toArray();
        }
    }
}
