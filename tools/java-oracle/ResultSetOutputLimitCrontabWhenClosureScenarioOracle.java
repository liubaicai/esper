import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.OutputLimitClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for the ResultSetOutputLimitCrontabWhen closure
 * executions (ords 9, 10 and 13 — the last unreferenced executions of
 * ResultSetOutputLimitCrontabWhen.java). Replays each case on one fresh
 * runtime:
 *
 * when-then-soda (ord 9, ResultSetOutputWhenThenExpressionSODA): sets
 * myvar=0, advances to 2008-02-01T08:00, deploys the inert
 * `on SupportBean set myvar = intPrimitive` trigger and the s0 length(2)
 * select whose output clause is
 * `when myvar=1 then set myvar=0, count_insert_var=count_insert`, then
 * undeploys. The suite's only assertion is the SODA model's toEPL text;
 * the unrepresentable step rebuilds the model and asserts toEPL equals
 * the pinned EPL (the Go API has no statement object model, so the Go
 * side pins the record verbatim). No events are sent and the listener
 * never fires, so the trace carries deployed markers plus the
 * unrepresentable record only.
 *
 * same-var-twice (ord 10, ResultSetOutputWhenThenSameVarTwice, JIRA-386):
 * deploys s1 and s2 as separate modules with the identical
 * `select * from SupportMarketDataBean output last when myvar=100`
 * clause, sends E1/E2, advances one second (neither listener fires),
 * sets myvar=100 and advances one more second so both listeners emit the
 * single last buffered row E2, then undeploys both modules.
 *
 * invalid (ord 13, ResultSetInvalid): eight path-less tryInvalidCompile
 * probes pin the output-clause rejection prefixes — a non-boolean when
 * expression, a then-set type mismatch, an aggregate in then-set, a
 * then-set literal without a variable name (emitted as the pinned
 * "unrepresentable" record because SetOutputVariable requires a name in
 * the Go API), a stream property inside an aggregate in when, an
 * aggregate over count_insert in when, prev(count_insert) in when, and a
 * zero-second every interval. Every probe compiles without the runtime
 * path, mirroring env.tryInvalidCompile's path-less compileWCheckedEx.
 */
public final class ResultSetOutputLimitCrontabWhenClosureScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-crontab-when-closure";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitCrontabWhen closure surface (ords 9, 10, 13): "
                    + "when-then-soda replays ResultSetOutputWhenThenExpressionSODA — "
                    + "runtimeSetVariable(myvar,0), advance to 2008-02-01T08:00, deploy "
                    + "the inert `on SupportBean set myvar = intPrimitive` trigger and "
                    + "the s0 `select symbol from SupportMarketDataBean#length(2) "
                    + "output when myvar=1 then set myvar=0, "
                    + "count_insert_var=count_insert` statement, then undeploy-all; "
                    + "the SODA toEPL assertion pins as an unrepresentable record "
                    + "carrying the asserted EPL (no Go object model); same-var-twice "
                    + "replays ResultSetOutputWhenThenSameVarTwice (JIRA-386) — s1 and "
                    + "s2 deploy the identical `select * from SupportMarketDataBean "
                    + "output last when myvar=100` module, E1/E2 sends plus a "
                    + "one-second advance fire neither listener, then myvar=100 and a "
                    + "second advance fire both with the single last buffered row E2 "
                    + "before both modules undeploy; invalid replays ResultSetInvalid's "
                    + "eight path-less tryInvalidCompile probes pinning the "
                    + "output-clause rejection prefixes (non-boolean when, then-set "
                    + "type mismatch, aggregate in then-set, then-set literal — "
                    + "unrepresentable in Go, aggregate over a stream property in "
                    + "when, aggregate over count_insert in when, prev in when, "
                    + "zero-second every interval). compile-error records carry the "
                    + "pinned Java message prefixes; unrepresentable records pin the "
                    + "asserted EPL and the then-set-literal prefix; deployed records "
                    + "mark each module statement; listener records carry the s1/s2 "
                    + "last-row emissions (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/resultset/outputlimit/ResultSetOutputLimitCrontabWhen.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/outputlimit/ResultSetOutputLimitCrontabWhen.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-c8af5811fef1d7f582d6",
            "java-runtime-feee4a26544010fc7fed",
            "java-runtime-54ed0a4e11d7714b8289"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOutputWhenThenExpressionSODA",
            "ResultSetOutputWhenThenSameVarTwice",
            "ResultSetInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-1ebf6ec5d07196df180e",
            "java-40115e717917aa787855",
            "java-ecf17baa44d87226cc82"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "when-then-soda",
            "same-var-twice",
            "invalid"
    };
    private static final int[] ORDINALS = {9, 10, 13};

    // Verbatim transcriptions of ResultSetOutputLimitCrontabWhen lines 132,
    // 134, 155-156 and 362-384.
    private static final String EPL_ON_SET = "on SupportBean set myvar = intPrimitive";
    private static final String EPL_SODA =
            "@name('s0') select symbol from SupportMarketDataBean#length(2) output "
                    + "when myvar=1 then set myvar=0, count_insert_var=count_insert";
    private static final String EPL_S1 =
            "@name('s1') select * from SupportMarketDataBean output last when myvar=100";
    private static final String EPL_S2 =
            "@name('s2') select * from SupportMarketDataBean output last when myvar=100";
    private static final String EPL_PROBE_WHEN_NON_BOOL =
            "select * from SupportMarketDataBean output when myvardummy";
    private static final String EPL_PROBE_THEN_TYPE =
            "select * from SupportMarketDataBean output when true then set myvardummy = 'b'";
    private static final String EPL_PROBE_THEN_AGGREGATE =
            "select * from SupportMarketDataBean output when true then set myvardummy = sum(myvardummy)";
    private static final String EPL_PROBE_THEN_LITERAL =
            "select * from SupportMarketDataBean output when true then set 1";
    private static final String EPL_PROBE_WHEN_AGG_PROPERTY =
            "select * from SupportMarketDataBean output when sum(price) > 0";
    private static final String EPL_PROBE_WHEN_AGG_COUNT =
            "select * from SupportMarketDataBean output when sum(count_insert) > 0";
    private static final String EPL_PROBE_WHEN_PREV =
            "select * from SupportMarketDataBean output when prev(1, count_insert) = 0";
    private static final String EPL_PROBE_EVERY_ZERO =
            "select theString, count(*) from SupportBean#length(2) group by theString "
                    + "output all every 0 seconds";

    private static final String ERR_WHEN_NON_BOOL =
            "The when-trigger expression in the OUTPUT WHEN clause must return a "
                    + "boolean-type value [select * from SupportMarketDataBean output "
                    + "when myvardummy]";
    private static final String ERR_THEN_TYPE =
            "Failed to validate the output rate limiting clause: Failed to validate "
                    + "assignment expression 'myvardummy=\"b\"': Variable 'myvardummy' "
                    + "of declared type Integer cannot be assigned a value of type "
                    + "String [select * from SupportMarketDataBean output when true "
                    + "then set myvardummy = 'b']";
    private static final String ERR_THEN_AGGREGATE =
            "Aggregation functions may not be used within update-set [select * from "
                    + "SupportMarketDataBean output when true then set myvardummy = "
                    + "sum(myvardummy)]";
    private static final String ERR_THEN_LITERAL =
            "Failed to validate the output rate limiting clause: Failed to validate "
                    + "assignment expression '1': Assignment expression must receive a "
                    + "single variable value";
    private static final String ERR_WHEN_AGG_PROPERTY =
            "Failed to validate output limit expression '(sum(price))>0': Property "
                    + "named 'price' is not valid in any stream [select * from "
                    + "SupportMarketDataBean output when sum(price) > 0]";
    private static final String ERR_WHEN_AGG_COUNT =
            "An aggregate function may not appear in a OUTPUT LIMIT clause [select * "
                    + "from SupportMarketDataBean output when sum(count_insert) > 0]";
    private static final String ERR_WHEN_PREV =
            "Failed to validate output limit expression 'prev(1,count_insert)=0': "
                    + "Previous function cannot be used in this context [select * from "
                    + "SupportMarketDataBean output when prev(1, count_insert) = 0]";
    private static final String ERR_EVERY_ZERO =
            "Invalid time period expression returns a zero or negative time interval "
                    + "[select theString, count(*) from SupportBean#length(2) group by "
                    + "theString output all every 0 seconds]";

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+unrepresentable; myvar=0 and the 2008-02-01T08:00 advance "
                    + "precede the inert on-set deploy and the s0 length(2) "
                    + "output-when-then deploy; the SODA model's toEPL assertion pins "
                    + "the byte-exact EPL as an unrepresentable record (no Go object "
                    + "model); no events are sent so the listener never fires",
            "deployed+listener; s1 and s2 deploy the identical `output last when "
                    + "myvar=100` module, E1/E2 sends and a one-second advance fire "
                    + "neither listener, then myvar=100 and a second advance fire "
                    + "both listeners with the single last buffered row E2 before "
                    + "both modules undeploy",
            "compile-error+unrepresentable; eight path-less tryInvalidCompile probes "
                    + "pin the output-clause rejections: non-boolean when, then-set "
                    + "type mismatch, aggregate in then-set, then-set literal without "
                    + "a variable name (pinned-only; SetOutputVariable requires a "
                    + "name), aggregate over a stream property in when, aggregate "
                    + "over count_insert in when, prev in when, and a zero-second "
                    + "every interval"
    };
    private static final String[] CASE_EPLS = {
            EPL_SODA,
            EPL_S1,
            EPL_PROBE_WHEN_NON_BOOL
    };

    // Statement labels whose deployed statements attach a TraceWriter,
    // mirroring addListener("s0"/"s1"/"s2"). The inert on-set trigger gets
    // no listener.
    private static final Set<String> LISTENED_LABELS =
            new HashSet<>(Arrays.asList("s0", "s1", "s2"));

    private static final int EXPECTED_STEPS = 32;
    private static final int EXPECTED_RECORDS = 15;

    /**
     * Pinned per-case step keys rendered as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at.
     * deploy steps carry the byte-exact EPL text; build-error steps carry
     * the byte-exact probe EPL and the pinned expectError prefix; every
     * probe is path-less (compileWithoutPath=1) mirroring
     * env.tryInvalidCompile(epl, ...). The soda-to-epl unrepresentable step
     * pins the asserted toEPL text in epl and expectError; the
     * then-set-literal unrepresentable step pins its probe EPL and prefix.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("when-then-soda", new String[]{
                "set-variable||myvar|||0|||",
                "advance-time||||||||2008-02-01T08:00:00Z",
                "deploy|on-set|||" + EPL_ON_SET + "||||",
                "deployed|on-set|||||||",
                "deploy|s0|||" + EPL_SODA + "||||",
                "deployed|s0|||||||",
                "unrepresentable|soda-to-epl|||" + EPL_SODA + "||" + EPL_SODA + "||",
                "set-variable||myvar|||0|||",
                "undeploy-all||||||||",
        });
        CASE_STEPS.put("same-var-twice", new String[]{
                "advance-time||||||||1970-01-01T00:00:00Z",
                "deploy|s1|||" + EPL_S1 + "||||",
                "deployed|s1|||||||",
                "deploy|s2|||" + EPL_S2 + "||||",
                "deployed|s2|||||||",
                "send|||SupportMarketDataBean||{\"symbol\":\"ABC\",\"id\":\"E1\","
                        + "\"price\":100}|||",
                "send|||SupportMarketDataBean||{\"symbol\":\"ABC\",\"id\":\"E2\","
                        + "\"price\":100}|||",
                "advance-time||||||||1970-01-01T00:00:01Z",
                "set-variable||myvar|||100|||",
                "advance-time||||||||1970-01-01T00:00:02Z",
                "undeploy|s1|||||||",
                "undeploy|s2|||||||",
        });
        CASE_STEPS.put("invalid", new String[]{
                "build-error|when-non-bool|||" + EPL_PROBE_WHEN_NON_BOOL + "||"
                        + ERR_WHEN_NON_BOOL + "|1|",
                "build-error|then-set-type-mismatch|||" + EPL_PROBE_THEN_TYPE + "||"
                        + ERR_THEN_TYPE + "|1|",
                "build-error|then-set-aggregate|||" + EPL_PROBE_THEN_AGGREGATE + "||"
                        + ERR_THEN_AGGREGATE + "|1|",
                "unrepresentable|then-set-literal|||" + EPL_PROBE_THEN_LITERAL + "||"
                        + ERR_THEN_LITERAL + "|1|",
                "build-error|when-aggregate-property|||" + EPL_PROBE_WHEN_AGG_PROPERTY
                        + "||" + ERR_WHEN_AGG_PROPERTY + "|1|",
                "build-error|when-aggregate-count-insert|||" + EPL_PROBE_WHEN_AGG_COUNT
                        + "||" + ERR_WHEN_AGG_COUNT + "|1|",
                "build-error|when-prev-count-insert|||" + EPL_PROBE_WHEN_PREV + "||"
                        + ERR_WHEN_PREV + "|1|",
                "build-error|every-zero-seconds|||" + EPL_PROBE_EVERY_ZERO + "||"
                        + ERR_EVERY_ZERO + "|1|",
        });
    }

    private ResultSetOutputLimitCrontabWhenClosureScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitCrontabWhenClosureScenarioOracle <scenario.json>");
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
     * Replays the case's steps on a fresh runtime. Deploy steps compile
     * with the runtime path (env.compileDeploy(epl, path)); the probes
     * compile without it, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx. The suite configuration declares myvar and
     * myvardummy as int and count_insert_var as int (Java's assignment
     * widener accepts the long count_insert; Go registers int64 because
     * narrowing is rejected).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        configuration.getCommon().addVariable("myvar", int.class, 0);
        configuration.getCommon().addVariable("myvardummy", int.class, 0);
        configuration.getCommon().addVariable("count_insert_var", int.class, 0);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Set<String> deployedLabels = new HashSet<>();
            Map<String, String> deploymentIds = new HashMap<>();
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
                                deploymentIds, records);
                        break;
                    case "deployed":
                        deployedStep(runtime, caseName, step, deployedLabels, sequences, records);
                        break;
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "set-variable":
                        setVariableStep(runtime, step);
                        break;
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(string(step, "at")).toEpochMilli());
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(configuration, caseName, step, records);
                        break;
                    case "undeploy":
                        undeployStep(runtime, step, deploymentIds);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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
     * Compiles and deploys the step's single-statement module with the
     * runtime path, mirroring env.compileDeploy(epl, path). The deployed
     * statement label registers so the deployed marker resolves; the
     * listened labels (s0/s1/s2) attach the TraceWriter the suite
     * registers via addListener.
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, JsonObject step, Set<String> deployedLabels,
                                   Map<String, String> deploymentIds, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        EPStatement[] statements = deployment.getStatements();
        if (statements.length != 1) {
            throw new IllegalStateException(caseName + " deploy " + label + " produced "
                    + statements.length + " statements, want 1");
        }
        deployedLabels.add(label);
        deploymentIds.put(label, deployment.getDeploymentId());
        if (LISTENED_LABELS.contains(label)) {
            EPStatement statement = statements[0];
            if (!label.equals(statement.getName())) {
                throw new IllegalStateException(caseName + " deploy " + label
                        + " produced statement " + statement.getName());
            }
            statement.addListener(new TraceWriter(records, caseName, label, runtime));
        }
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
     * Replays one send step: a SupportMarketDataBean built through the
     * three-argument (symbol, id, price) constructor the suite uses, so
     * volume and feed stay null. Sends emit no trace record; the listener
     * records carry the observable output.
     */
    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String eventType = string(step, "eventType");
        if (!"SupportMarketDataBean".equals(eventType)) {
            throw new IllegalStateException("unknown send event type: " + eventType);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        runtime.getEventService().sendEventBean(new SupportMarketDataBean(
                string(payload, "symbol"), string(payload, "id"),
                payload.getDouble("price", 0)), eventType);
    }

    /**
     * Replays runtimeSetVariable(null, name, value); the pinned variables
     * are int-typed so the payload must be a JSON integer.
     */
    private static void setVariableStep(EPRuntime runtime, JsonObject step) {
        String name = string(step, "name");
        JsonValue payload = step.get("payload");
        if (payload == null || !payload.isNumber()) {
            throw new IllegalStateException("set-variable " + name + " payload is not a number");
        }
        runtime.getVariableService().setVariableValue(null, name, payload.asInt());
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). Every probe compiles without the runtime path, mirroring
     * env.tryInvalidCompile's path-less compileWCheckedEx.
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                throw new IllegalStateException("build-error probe " + label
                        + " is not marked compileWithoutPath");
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
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Emits the pinned "unrepresentable" record for the two surfaces the
     * typed Go API cannot express. soda-to-epl rebuilds the SODA model the
     * suite constructs and asserts toEPL equals the pinned EPL (the record
     * carries the asserted text). then-set-literal compiles the probe EPL
     * and verifies the pinned prefix like a build-error probe (the record
     * carries the prefix; SetOutputVariable requires a variable name in
     * Go, so no Go rejection boundary exists).
     */
    private static void unrepresentableStep(Configuration configuration, String caseName,
                                            JsonObject step, JsonArray records) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        if ("soda-to-epl".equals(label)) {
            EPStatementObjectModel model = new EPStatementObjectModel();
            model.setSelectClause(SelectClause.create("symbol"));
            model.setFromClause(FromClause.create(
                    FilterStream.create("SupportMarketDataBean")
                            .addView("length", Expressions.constant(2))));
            model.setOutputLimitClause(OutputLimitClause.create(Expressions.eq("myvar", 1))
                    .addThenAssignment(Expressions.eq(Expressions.property("myvar"),
                            Expressions.constant(0)))
                    .addThenAssignment(Expressions.eq(Expressions.property("count_insert_var"),
                            Expressions.property("count_insert"))));
            model.setAnnotations(Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
            String rendered = model.toEPL();
            if (!epl.equals(rendered)) {
                throw new IllegalStateException("SODA toEPL drift for " + label
                        + ": expected [" + epl + "] got [" + rendered + "]");
            }
        } else if ("then-set-literal".equals(label)) {
            String caught;
            try {
                CompilerArguments compilerArgs = new CompilerArguments(configuration);
                if (!step.getBoolean("compileWithoutPath", false)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " is not marked compileWithoutPath");
                }
                EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                caught = null;
            } catch (Exception ex) {
                caught = ex.getMessage();
            }
            if (caught == null) {
                throw new IllegalStateException("unrepresentable probe " + label
                        + " unexpectedly succeeded");
            }
            if (!caught.startsWith(expected)) {
                throw new IllegalStateException("unrepresentable message drift for " + label
                        + ": expected prefix [" + expected + "] got [" + caught + "]");
            }
        } else {
            throw new IllegalStateException("unknown unrepresentable step " + label);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Undeploys one labeled deployment, mirroring
     * env.undeployModuleContaining.
     */
    private static void undeployStep(EPRuntime runtime, JsonObject step,
                                     Map<String, String> deploymentIds) throws Exception {
        String label = string(step, "statement");
        String deploymentId = deploymentIds.remove(label);
        if (deploymentId == null) {
            throw new IllegalStateException("undeploy of unknown statement " + label);
        }
        runtime.getDeploymentService().undeploy(deploymentId);
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

    /**
     * UpdateListener that writes one listener record per delivered batch
     * with a per-statement sequence, mirroring the Go runner's Subscribe
     * recorder. Rows render every event property sorted by name; nulls
     * normalize to {"state":"null"}.
     */
    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final String statementName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, String statementName,
                            EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statementName = statementName;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statementName)
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
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

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean[] events) {
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    array.add(normalize(event));
                }
                return array;
            }
            if (value instanceof EventBean event) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
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
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
