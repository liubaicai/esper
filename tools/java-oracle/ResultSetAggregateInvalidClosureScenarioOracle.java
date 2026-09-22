import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
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
 * Scenario oracle for the ResultSetAggregate invalid-closure executions
 * pinned by Draft 4.496 — four invalid-form executions across four Java
 * files, all compile-time validation (10 probes, no events, no listeners,
 * no milestones, no virtual time). Replays each case on one fresh runtime:
 *
 * ext-invalid (ResultSetAggregateExtInvalid, single-execution file): one
 * path-less tryInvalidCompile probe pins `select rate(10) from SupportBean`
 * rejected as an unknown function. The suite runs under
 * setExtendedAggregation(false) (TestSuiteResultSetAggregateWConfig), so
 * the case configuration disables extended aggregation before compiling;
 * without it rate(10) would compile. The Go API has no such toggle and
 * Rate compiles, so the step is emitted as the pinned "unrepresentable"
 * record.
 *
 * aggregate-invalid (ResultSetAggregateSortedMinMaxBy ord 7,
 * ResultSetAggregateInvalid): two path-less tryInvalidCompile probes pin
 * the sorted/minmax-by declaration rejections — maxBy over a criteria
 * expression spanning SupportBean_S0 and SupportBean_S1 (same-stream
 * check), and sorted over a stream with no declared data window.
 *
 * window-invalid (ResultSetAggregationMethodWindow ord 4,
 * ResultSetAggregateWindowInvalid): deploys the @public table
 * MyTable(windowcol window(*) @type('SupportBean')) so the module joins
 * the accumulated path, then runs two path-carrying tryInvalidCompile
 * probes — windowcol.first(id), whose argument validates against the
 * column's contained type (SupportBean has no id property), and
 * windowcol.listReference(intPrimitive), whose zero-parameter arity check
 * rejects any argument. Both steps are emitted as pinned "unrepresentable"
 * records (the typed Go API has no first(property)/listReference(property)
 * forms). The case ends with undeploy-all.
 *
 * filter-named-param-invalid (ResultSetAggregateFilterNamedParameter ord
 * 20, ResultSetAggregateFilterNamedParamInvalid): five path-less
 * tryInvalidCompile probes — a multi-value filter tuple, multiple filter
 * expressions and a non-boolean filter (all unrepresentable in the typed
 * Go API), a create-table column declaring filter:true, and a correlated
 * subquery whose aggregate filter reads an outer-stream property.
 */
public final class ResultSetAggregateInvalidClosureScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-invalid-closure";
    private static final String DESCRIPTION =
            "ResultSetAggregate invalid-closure surface (4 executions across 4 "
                    + "files, 10 compile-time probes): ext-invalid replays "
                    + "ResultSetAggregateExtInvalid — `select rate(10) from "
                    + "SupportBean` under extendedAggregation=false fails name "
                    + "resolution (unrepresentable: Go has no extended-aggregation "
                    + "toggle and Rate compiles); aggregate-invalid replays "
                    + "ResultSetAggregateSortedMinMaxBy ord 7 "
                    + "ResultSetAggregateInvalid — `maxBy(p00||p10)` spanning "
                    + "SupportBean_S0/SupportBean_S1 fails the same-stream "
                    + "criteria check and `sorted(p00)` over a windowless stream "
                    + "fails the data-window requirement; window-invalid replays "
                    + "ResultSetAggregationMethodWindow ord 4 "
                    + "ResultSetAggregateWindowInvalid — the @public "
                    + "MyTable(windowcol window(*) @type('SupportBean')) deploy "
                    + "precedes two path-carrying probes, `windowcol.first(id)` "
                    + "(argument validated against the contained type, "
                    + "SupportBean has no id) and "
                    + "`windowcol.listReference(intPrimitive)` (zero-parameter "
                    + "arity check), both unrepresentable in the typed API; "
                    + "filter-named-param-invalid replays "
                    + "ResultSetAggregateFilterNamedParameter ord 20 "
                    + "ResultSetAggregateFilterNamedParamInvalid — multi-value, "
                    + "multiple and non-boolean filter named parameters are "
                    + "unrepresentable (FilterAggregate takes one "
                    + "Expression[bool]), `create table MyTable(totals sum(int, "
                    + "filter:true))` is rejected by CreateTable, and the "
                    + "correlated `filter:s0.p00='a'` subquery aggregate is "
                    + "rejected by the existing subselect check. compile-error "
                    + "records carry the pinned Java message prefixes; "
                    + "unrepresentable records pin the probe EPL and prefix; "
                    + "the deployed record marks the MyTable module deploy "
                    + "(Java sources "
                    + "regression-lib/.../ResultSetAggregateExtInvalid.java, "
                    + "ResultSetAggregateSortedMinMaxBy.java, "
                    + "ResultSetAggregationMethodWindow.java, "
                    + "ResultSetAggregateFilterNamedParameter.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/aggregate/ResultSetAggregateExtInvalid.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java";
    private static final String JAVA_SOURCE3 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/aggregate/ResultSetAggregationMethodWindow.java";
    private static final String JAVA_SOURCE4 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/aggregate/ResultSetAggregateFilterNamedParameter.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-98f0ac5299780e2d6656",
            "java-runtime-09530eec55a704b01c96",
            "java-runtime-fbbdc48ea2da6d4bc426",
            "java-runtime-50935b7efcc1fdced314"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateExtInvalid",
            "ResultSetAggregateInvalid",
            "ResultSetAggregateWindowInvalid",
            "ResultSetAggregateFilterNamedParamInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-9c4ed42f2a9b55c3fd1c",
            "java-553516b9d01c12a13172",
            "java-313287657d14b5c686f8",
            "java-0c29efb6d43971aba5c4"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "ext-invalid",
            "aggregate-invalid",
            "window-invalid",
            "filter-named-param-invalid"
    };
    private static final int[] ORDINALS = {0, 7, 4, 20};

    // Verbatim transcriptions of ResultSetAggregateExtInvalid line 21,
    // ResultSetAggregateSortedMinMaxBy lines 427-431,
    // ResultSetAggregationMethodWindow lines 156-162 and
    // ResultSetAggregateFilterNamedParameter lines 258-276.
    private static final String EPL_TABLE =
            "@public create table MyTable(windowcol window(*) @type('SupportBean'));\n";
    private static final String EPL_PROBE_RATE =
            "select rate(10) from SupportBean";
    private static final String EPL_PROBE_MAXBY_CROSS =
            "select maxBy(p00||p10) from SupportBean_S0#lastevent, SupportBean_S1#lastevent";
    private static final String EPL_PROBE_SORTED_NO_WINDOW =
            "select sorted(p00) from SupportBean_S0";
    private static final String EPL_PROBE_WINDOWCOL_FIRST =
            "select MyTable.windowcol.first(id) from SupportBean_S0";
    private static final String EPL_PROBE_WINDOWCOL_LISTREF =
            "select MyTable.windowcol.listReference(intPrimitive) from SupportBean_S0";
    private static final String EPL_PROBE_FILTER_MULTI_VALUE =
            "select sum(intPrimitive, filter:(intPrimitive, doublePrimitive)) from SupportBean";
    private static final String EPL_PROBE_FILTER_MULTIPLE =
            "select sum(intPrimitive, intPrimitive > 0, filter:intPrimitive < 0) from SupportBean";
    private static final String EPL_PROBE_FILTER_NON_BOOL =
            "select sum(intPrimitive, filter:intPrimitive) from SupportBean";
    private static final String EPL_PROBE_CREATE_TABLE_FILTER =
            "create table MyTable(totals sum(int, filter:true))";
    private static final String EPL_PROBE_CORRELATED_SUBQUERY =
            "select (select sum(intPrimitive, filter:s0.p00='a') from SupportBean) "
                    + "from SupportBean_S0 as s0";

    // The pinned Java message prefixes (SupportMessageAssertUtil.assertMessage
    // startsWith semantics). ERR_RATE is the complete message: the suite pins
    // the ' [<original EPL>]' suffix literally.
    private static final String ERR_RATE =
            "Failed to validate select-clause expression 'rate(10)': Unknown "
                    + "single-row function, aggregation function or mapped or "
                    + "indexed property named 'rate' could not be resolved "
                    + "[select rate(10) from SupportBean]";
    private static final String ERR_MAXBY_CROSS =
            "Failed to validate select-clause expression 'maxby(p00||p10)': The "
                    + "'maxby' aggregation function requires that any parameter "
                    + "expressions evaluate properties of the same stream";
    private static final String ERR_SORTED_NO_WINDOW =
            "Failed to validate select-clause expression 'sorted(p00)': The "
                    + "'sorted' aggregation function requires that a data window "
                    + "is declared for the stream";
    private static final String ERR_WINDOWCOL_FIRST =
            "Failed to validate select-clause expression "
                    + "'MyTable.windowcol.first(id)': Failed to validate "
                    + "aggregation function parameter expression 'id': Property "
                    + "named 'id' is not valid in any stream";
    private static final String ERR_WINDOWCOL_LISTREF =
            "Failed to validate select-clause expression "
                    + "'MyTable.windowcol.listReference(int...(45 chars)': Invalid "
                    + "number of parameters";
    private static final String ERR_FILTER_MULTI_VALUE =
            "Failed to validate select-clause expression "
                    + "'sum(intPrimitive,filter:(intPrimiti...(55 chars)': Filter "
                    + "named parameter requires a single expression returning a "
                    + "boolean-typed value";
    private static final String ERR_FILTER_MULTIPLE =
            "Failed to validate select-clause expression "
                    + "'sum(intPrimitive,intPrimitive>0,fil...(54 chars)': Only a "
                    + "single filter expression can be provided";
    private static final String ERR_FILTER_NON_BOOL =
            "Failed to validate select-clause expression "
                    + "'sum(intPrimitive,filter:intPrimitive)': Filter named "
                    + "parameter requires a single expression returning a "
                    + "boolean-typed value";
    private static final String ERR_CREATE_TABLE_FILTER =
            "Failed to validate table-column expression 'sum(int,filter:true)': "
                    + "The 'group_by' and 'filter' parameter is not allowed in "
                    + "create-table statements";
    private static final String ERR_CORRELATED_SUBQUERY =
            "Failed to plan subquery number 1 querying SupportBean: Subselect "
                    + "aggregation functions cannot aggregate across correlated "
                    + "properties";

    private static final String[] CASE_OBSERVATIONS = {
            "unrepresentable; the single path-less tryInvalidCompile probe pins "
                    + "the rate(10) unknown-function rejection the suite produces "
                    + "under extendedAggregation=false (Go has no toggle and Rate "
                    + "compiles, so the record carries the pinned message verbatim)",
            "compile-error; two path-less tryInvalidCompile probes pin the "
                    + "sorted/minmax-by rejections: maxBy over a criteria "
                    + "expression spanning SupportBean_S0 and SupportBean_S1 "
                    + "(same-stream check), and sorted over a stream with no "
                    + "declared data window",
            "deployed+unrepresentable; the @public MyTable(windowcol window(*) "
                    + "@type('SupportBean')) deploy precedes two path-carrying "
                    + "probes — windowcol.first(id) validates the argument "
                    + "against the contained type (SupportBean has no id) and "
                    + "windowcol.listReference(intPrimitive) fails the "
                    + "zero-parameter arity check; both pin verbatim (no "
                    + "typed-API form)",
            "compile-error+unrepresentable; five path-less tryInvalidCompile "
                    + "probes pin the filter named-parameter rejections: "
                    + "multi-value filter tuple, multiple filter expressions and "
                    + "non-boolean filter (pinned-only; FilterAggregate takes one "
                    + "Expression[bool]), create-table filter:true rejected by "
                    + "CreateTable, and the correlated filter:s0.p00='a' subquery "
                    + "aggregate rejected by the subselect-correlation check"
    };
    private static final String[] CASE_EPLS = {
            EPL_PROBE_RATE,
            EPL_PROBE_MAXBY_CROSS,
            EPL_TABLE,
            EPL_PROBE_FILTER_MULTI_VALUE
    };

    private static final int EXPECTED_STEPS = 17;
    private static final int EXPECTED_RECORDS = 11;

    /**
     * Pinned per-case step keys rendered as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at.
     * The deploy step carries the byte-exact table EPL; build-error and
     * unrepresentable steps carry the byte-exact probe EPL and the pinned
     * expectError prefix. The window-invalid probes compile against the
     * accumulated path (no compileWithoutPath marker), mirroring
     * env.tryInvalidCompile(path, epl, ...); every other probe is path-less
     * (compileWithoutPath=1), mirroring env.tryInvalidCompile(epl, ...).
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("ext-invalid", new String[]{
                "unrepresentable|rate-unknown-function|||" + EPL_PROBE_RATE + "||"
                        + ERR_RATE + "|1|",
        });
        CASE_STEPS.put("aggregate-invalid", new String[]{
                "build-error|maxby-cross-stream|||" + EPL_PROBE_MAXBY_CROSS + "||"
                        + ERR_MAXBY_CROSS + "|1|",
                "build-error|sorted-no-window|||" + EPL_PROBE_SORTED_NO_WINDOW + "||"
                        + ERR_SORTED_NO_WINDOW + "|1|",
        });
        CASE_STEPS.put("window-invalid", new String[]{
                "deploy|create-table|||" + EPL_TABLE + "||||",
                "deployed|create-table|||||||",
                "unrepresentable|windowcol-first-id|||" + EPL_PROBE_WINDOWCOL_FIRST
                        + "||" + ERR_WINDOWCOL_FIRST + "||",
                "unrepresentable|windowcol-listreference-arity|||"
                        + EPL_PROBE_WINDOWCOL_LISTREF + "||" + ERR_WINDOWCOL_LISTREF
                        + "||",
                "undeploy-all||||||||",
        });
        CASE_STEPS.put("filter-named-param-invalid", new String[]{
                "unrepresentable|filter-multi-value|||" + EPL_PROBE_FILTER_MULTI_VALUE
                        + "||" + ERR_FILTER_MULTI_VALUE + "|1|",
                "unrepresentable|filter-multiple|||" + EPL_PROBE_FILTER_MULTIPLE
                        + "||" + ERR_FILTER_MULTIPLE + "|1|",
                "unrepresentable|filter-non-bool|||" + EPL_PROBE_FILTER_NON_BOOL
                        + "||" + ERR_FILTER_NON_BOOL + "|1|",
                "unrepresentable|create-table-filter|||" + EPL_PROBE_CREATE_TABLE_FILTER
                        + "||" + ERR_CREATE_TABLE_FILTER + "|1|",
                "build-error|filter-correlated-subquery|||"
                        + EPL_PROBE_CORRELATED_SUBQUERY + "||"
                        + ERR_CORRELATED_SUBQUERY + "|1|",
        });
    }

    private ResultSetAggregateInvalidClosureScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateInvalidClosureScenarioOracle <scenario.json>");
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
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
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
     * Replays the case's steps on a fresh runtime. The ext-invalid case
     * disables extended aggregation like TestSuiteResultSetAggregateWConfig
     * (without it rate(10) resolves and the probe would compile); the other
     * cases use the default configuration. Deploy steps compile against the
     * accumulated module path like env.compileDeploy(epl, path); probes
     * compile with or without the path per their compileWithoutPath marker,
     * mirroring env.tryInvalidCompile's two forms.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        if (!"ext-invalid".equals(caseName)) {
            configuration.getCommon().addEventType(SupportBean_S0.class);
            configuration.getCommon().addEventType(SupportBean_S1.class);
        } else {
            // TestSuiteResultSetAggregateWConfig disables extended
            // aggregation for this execution; rate(10) then fails name
            // resolution instead of compiling.
            configuration.getCompiler().getExpression().setExtendedAggregation(false);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Set<String> deployedLabels = new HashSet<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            Map<String, Integer> sequences = new HashMap<>();
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
                        deployStep(runtime, configuration, caseName, step, deployedLabels,
                                deployedModules);
                        break;
                    case "deployed":
                        deployedStep(runtime, caseName, step, deployedLabels, sequences, records);
                        break;
                    case "build-error":
                        probeStep(configuration, caseName, step, deployedModules, records,
                                "compile-error");
                        break;
                    case "unrepresentable":
                        probeStep(configuration, caseName, step, deployedModules, records,
                                "unrepresentable");
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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
     * Compiles and deploys the step's single-statement module against the
     * accumulated module path, mirroring env.compileDeploy(epl, path): the
     * compiled module joins the path so the path-carrying probes resolve
     * the @public table. The deployed statement label registers so the
     * deployed marker resolves.
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, JsonObject step, Set<String> deployedLabels,
                                   List<EPCompiled> deployedModules) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().addAll(deployedModules);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        deployedModules.add(compiled);
        EPStatement[] statements = deployment.getStatements();
        if (statements.length != 1) {
            throw new IllegalStateException(caseName + " deploy " + label + " produced "
                    + statements.length + " statements, want 1");
        }
        deployedLabels.add(label);
    }

    /**
     * Emits the deployed marker for a statement label the preceding deploy
     * registered, mirroring the per-statement deployed record.
     */
    private static void deployedStep(EPRuntime runtime, String caseName, JsonObject step,
                                     Set<String> deployedLabels, Map<String, Integer> sequences,
                                     JsonArray records) {
        String label = string(step, "statement");
        if (!deployedLabels.contains(label)) {
            throw new IllegalStateException("deployed marker for unknown statement " + label);
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
    }

    /**
     * Compiles an expected-invalid probe and emits the pinned record
     * carrying the expectError prefix after verifying the caught message
     * starts with it (SupportMessageAssertUtil.assertMessage semantics).
     * Probes marked compileWithoutPath compile without the accumulated
     * module path, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx; the rest compile with the path like the
     * window-invalid probes. build-error steps emit "compile-error"
     * records; unrepresentable steps emit "unrepresentable" records (the
     * typed Go API has no boundary for them, so the Go side pins the value
     * verbatim).
     */
    private static void probeStep(Configuration configuration, String caseName,
                                  JsonObject step, List<EPCompiled> deployedModules,
                                  JsonArray records, String operation) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().addAll(deployedModules);
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException(operation + " probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException(operation + " message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", operation);
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaSource3", "javaSource4", "javaRuntimes", "javaNames",
                "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))
                || !JAVA_SOURCE3.equals(string(scenario, "javaSource3"))
                || !JAVA_SOURCE4.equals(string(scenario, "javaSource4"))) {
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
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
     * with the payload rendered as compacted JSON. Unknown fields are
     * rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "name", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "at"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp + "|" + string(step, "at");
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
}
