import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.view.ViewExpressionWindow;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the view-expression-udf-prev-572
 * bundle: ViewExpressionBatch ords 3/5 + ViewExpressionWindow ords 4/6 —
 * the UDF+Prev quartet. Each execution replays one regression run under
 * its own runtime initialized at epoch 0; the UDF cases register the
 * same `udf` plug-in single-row function the TestSuiteView session
 * configuration installs
 * (ViewExpressionWindow.LocalUDF.evaluateExpiryUDF(String key,
 * Object viewref, Integer expiryCount)) and drive its static result
 * toggle through the pinned udf-result carrier steps.
 * Note: all four pinned EPLs are plain {@code select} (istream only),
 * so remove-stream rows never reach the listener: the expr_batch
 * prior-batch {E1} rstream and the expr mass-expiry {E1,E2,E3}
 * rstream flow but surface no "old" rows in the trace.
 *
 * udf-batch replays ViewExpressionBatchUDFBuiltin (batch ord 3):
 * {@code @name('s0') select * from
 * SupportBean#expr_batch(udf(theString, view_reference,
 * expired_count))}. With result=true E1 flushes its single-row batch
 * (new {E1}) and E2 flushes new {E2} (the prior batch's {E1} rstream
 * row never surfaces on an istream statement); with result=false E3
 * stays silent. The pinned udf-mode observations read the LocalUDF
 * statics after E1 and E3 — where the Java execution's env.assertThat
 * reads them — and report key=E1 then key=E3, expiryCount=0 at both
 * (expr_batch evaluates the arriving event only: batch expiry is a
 * whole-batch flush, so expired_count never counts retained rows) and
 * viewref non-null.
 *
 * prev-batch replays ViewExpressionBatchPrev (batch ord 5):
 * {@code @name('s0') select prev(1, theString) as val0 from
 * SupportBean#expr_batch(current_count > 2)}. E1 and E2 accumulate
 * silently; E3 flushes the whole batch as ONE 3-row delivery with
 * val0 {null,E1,E2} — prev is evaluated per output row inside the
 * flushed batch.
 *
 * udf-window replays ViewExpressionWindowUDFBuiltin (window ord 4):
 * {@code @name('s0') select * from
 * SupportBean#expr(udf(theString, view_reference, expired_count))}.
 * E1 and E2 are retained as new-only deliveries; with result=false E3
 * delivers new {E3} alone — the mass-expiry {E1,E2,E3} rstream never
 * surfaces on an istream statement — while the keep predicate fails
 * for both
 * retained rows before the arriving row's own evaluation, so the
 * pinned observation reads key=E3 expiryCount=2 viewref non-null.
 *
 * prev-window replays ViewExpressionWindowPrev (window ord 6):
 * {@code @name('s0') select prev(1, theString) as val0 from
 * SupportBean#expr(true)}. Deliveries are per-row, val0 null at E1 and
 * val0 E1 at E2; expr(true) never expires so no old data flows.
 *
 * The Java regression milestone()/mileZero calls are regression-harness
 * savepoints with identical restored state for these non-contextual
 * executions, so the scenario omits them (they pin no observable). The
 * Java executions assert the LocalUDF statics via env.assertThat after
 * E1 and E3 only — never after E2 — and never assert listener shapes;
 * the udf-mode snapshot steps pin the two asserted observation points
 * per UDF case while listener deliveries are recorded symmetrically.
 *
 * Records follow the standard protocol: one listener record per
 * delivered update with a per-statement sequence counter starting at 1
 * and time rendered from the current engine time; deployed markers and
 * udf observations carry their own per-statement counters. Observation
 * records carry name "udf" and a value object {key, expiryCount,
 * viewref} mirroring the LocalUDF getters (viewref is pinned as its
 * non-null assertion, matching assertNotNull(LocalUDF.getViewref())).
 */
public final class ViewExpressionUDFPrev572ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-expression-udf-prev-572";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
            "regression-run/src/test/java/com/espertech/esper/regressionrun/suite/view/TestSuiteView.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "ViewExpressionBatch ords 3/5 + ViewExpressionWindow ords 4/6 — " +
            "the UDF+Prev quartet. udf-batch (ViewExpressionBatchUDFBuiltin, " +
            "ord 3) replays `@name('s0') select * from " +
            "SupportBean#expr_batch(udf(theString, view_reference, " +
            "expired_count))` with the TestSuiteView udf plug-in single-row " +
            "function (ViewExpressionWindow.LocalUDF.evaluateExpiryUDF(String " +
            "key, Object viewref, Integer expiryCount) -> bool, registered " +
            "via addPlugInSingleRowFunction with " +
            "threadPoolCompilerNumThreads=0): result=true flushes E1 as new " +
            "{E1} and E2 as new {E2} — plain-select istream statements " +
            "surface NO remove stream, so the prior batch's {E1} rstream row " +
            "never reaches the listener — and result=false keeps E3 silent; " +
            "pinned UDF observations read key=E1 then key=E3, expiryCount=0 " +
            "at both (expr_batch evaluates the arriving event only — batch " +
            "expiry is a whole-batch flush, so expired_count never counts " +
            "retained rows), viewref non-null. udf-window " +
            "(ViewExpressionWindowUDFBuiltin, ord 4) replays `select * from " +
            "SupportBean#expr(udf(theString, view_reference, " +
            "expired_count))`: E1 and E2 are retained as new-only deliveries, " +
            "then result=false at E3 delivers new {E3} alone — the " +
            "mass-expiry {E1,E2,E3} flows on the remove stream the istream " +
            "statement never surfaces — while the pinned observation reads " +
            "key=E3 expiryCount=2 viewref non-null (the keep predicate fails " +
            "for both retained rows before the arriving row's own " +
            "evaluation). prev-batch (ViewExpressionBatchPrev, ord 5) replays " +
            "`select prev(1, theString) as val0 from " +
            "SupportBean#expr_batch(current_count > 2)`: E1 and E2 accumulate " +
            "silently and E3 flushes the whole batch as one 3-row delivery " +
            "with val0 {null,E1,E2} — prev is evaluated per output row inside " +
            "the flushed batch. prev-window (ViewExpressionWindowPrev, ord 6) " +
            "replays `select prev(1, theString) as val0 from " +
            "SupportBean#expr(true)`: per-row deliveries carry val0 null at " +
            "E1 then val0 E1 at E2; expr(true) never expires so no old data " +
            "flows. Java milestone()/mileZero calls are regression-harness " +
            "savepoints with identical restored state for these executions, " +
            "so the scenario omits them; the Java executions assert LocalUDF " +
            "statics only after E1 and E3, so udf-mode snapshots pin those " +
            "two positions per UDF case.";

    private static final String[] CASES = {
            "udf-batch", "prev-batch",
            "udf-window", "prev-window"};
    private static final int[] ORDINALS = {3, 5, 4, 6};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-5804edaff708d2d83a5e",
            "java-runtime-3bec407608d81eebcc12",
            "java-runtime-2ea3302a92f786153790",
            "java-runtime-9171e672cfde4fa06cb5"};
    private static final String[] EXECUTIONS = {
            "ViewExpressionBatchUDFBuiltin",
            "ViewExpressionBatchPrev",
            "ViewExpressionWindowUDFBuiltin",
            "ViewExpressionWindowPrev"};
    // Deduplicated inventory ids: the two batch runtime rows share static
    // id java-20551a17cb2af08c67fc and the two window runtime rows share
    // java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-20551a17cb2af08c67fc",
            "java-20551a17cb2af08c67fc",
            "java-06e6b1f6c905b8f12b82",
            "java-06e6b1f6c905b8f12b82"};
    private static final String[] OBSERVATIONS = {
            "deployed+listener+udf; #expr_batch(udf(theString, " +
            "view_reference, expired_count)) with the TestSuiteView udf " +
            "plug-in: result=true flushes E1 as new {E1} and E2 as new {E2} " +
            "(plain-select istream statements surface no remove stream, so " +
            "the prior batch's {E1} rstream row never reaches the listener), " +
            "result=false keeps E3 silent; the pinned observations read " +
            "key=E1/key=E3 with expiryCount=0 at both (expr_batch evaluates " +
            "the arriving event only, so expired_count never counts retained " +
            "rows) and viewref non-null; the Java execution asserts listener " +
            "shape never, so no iterator snapshots are recorded",
            "deployed+listener; select prev(1, theString) as val0 over " +
            "#expr_batch(current_count > 2): E1 and E2 accumulate silently " +
            "and E3 flushes the whole batch as ONE 3-row delivery with val0 " +
            "{null,E1,E2} — prev is evaluated per output row inside the " +
            "flushed batch; the Java execution pins the flattened IR pair " +
            "only, so no snapshots are recorded",
            "deployed+listener+udf; #expr(udf(theString, view_reference, " +
            "expired_count)) with the TestSuiteView udf plug-in: E1 and E2 " +
            "are retained as new-only deliveries, result=false at E3 delivers " +
            "new {E3} alone — the mass-expiry {E1,E2,E3} flows on the remove " +
            "stream the istream statement never surfaces; the pinned " +
            "observations read key=E1 then key=E3 expiryCount=2 viewref " +
            "non-null — the keep predicate fails for both retained rows " +
            "before the arriving row's own evaluation; the Java execution " +
            "asserts the statics only after E1 and E3, so no iterator " +
            "snapshots are recorded",
            "deployed+listener; select prev(1, theString) as val0 over " +
            "#expr(true): deliveries are per-row, val0 null at E1 and val0 E1 " +
            "at E2; expr(true) never expires so no old data flows; the Java " +
            "execution pins assertPropsNew only, so no snapshots are recorded"
    };

    private static final String EPL_UDF_BATCH =
            "@name('s0') select * from SupportBean#expr_batch(udf(theString, view_reference, expired_count))";
    private static final String EPL_PREV_BATCH =
            "@name('s0') select prev(1, theString) as val0 from SupportBean#expr_batch(current_count > 2)";
    private static final String EPL_UDF_WINDOW =
            "@name('s0') select * from SupportBean#expr(udf(theString, view_reference, expired_count))";
    private static final String EPL_PREV_WINDOW =
            "@name('s0') select prev(1, theString) as val0 from SupportBean#expr(true)";

    // Case-level EPL pin: the execution's compileDeploy text (none
    // carries a trailing `;\n`).
    private static final String[] CASE_EPLS = {
            EPL_UDF_BATCH, EPL_PREV_BATCH,
            EPL_UDF_WINDOW, EPL_PREV_WINDOW};

    // Pinned row projections: the UDF cases project theString (the
    // execution's asserted/UDF-key field), the Prev cases project val0.
    private static final String[][] CASE_FIELDS = {
            {"theString"}, {"val0"}, {"theString"}, {"val0"}};

    // Pinned op sequences per case (after the case marker). milestone()
    // savepoints are omitted; set-variable steps named "udf-result"
    // mirror LocalUDF.setResult calls and udf-mode snapshot steps sit
    // exactly where the Java execution's env.assertThat reads the
    // LocalUDF statics (after E1 and after E3 — never after E2).
    private static final String[][] CASE_OPS = {
            {"deploy", "deployed",
                    "set-variable", "send", "snapshot",
                    "send",
                    "set-variable", "send", "snapshot",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "send", "send", "send",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "set-variable", "send", "snapshot",
                    "send",
                    "set-variable", "send", "snapshot",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "SB|theString|intPrimitive".
    private static final String[][] CASE_SENDS = {
            {"SB|E1|0", "SB|E2|0", "SB|E3|0"},
            {"SB|E1|1", "SB|E2|2", "SB|E3|3"},
            {"SB|E1|0", "SB|E2|0", "SB|E3|0"},
            {"SB|E1|1", "SB|E2|2"}
    };

    // Pinned udf-result toggles per case, in set order (LocalUDF.
    // setResult calls); the Prev cases carry none.
    private static final String[][] CASE_UDF_RESULTS = {
            {"true", "false"},
            {},
            {"true", "false"},
            {}
    };

    // Pinned snapshot modes per case; every UDF observation step is
    // mode "udf" (an env.assertThat read of the LocalUDF statics, not
    // an iterator pin).
    private static final String[][] CASE_SNAPSHOT_MODES = {
            {"udf", "udf"},
            {},
            {"udf", "udf"},
            {}
    };

    private static final int EXPECTED_STEPS = 35;
    private static final int EXPECTED_RECORDS = 16;

    private ViewExpressionUDFPrev572ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewExpressionUDFPrev572ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        // Mirror TestSuiteView.configure: the `udf` plug-in single-row
        // function backed by ViewExpressionWindow.LocalUDF.
        // evaluateExpiryUDF (the regression session also sets
        // threadPoolCompilerNumThreads=0; the default compiler already
        // runs single-threaded).
        configuration.getCompiler().addPlugInSingleRowFunction("udf",
                ViewExpressionWindow.LocalUDF.class.getName(), "evaluateExpiryUDF");

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
            EPDeployment deployment = null;
            boolean active = false;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String epl = step.getString("epl", "");
                        // Mirrors RegressionEnvironment.compileDeploy:
                        // the CompilerArguments carry the Configuration
                        // (not just the path) so the `udf` plug-in
                        // single-row function resolves.
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                new CompilerArguments(configuration));
                        deployment = runtime.getDeploymentService().deploy(compiled,
                                new com.espertech.esper.runtime.client.DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                        writer = new TraceWriter(records, caseName, findStatement(deployment),
                                runtime, CASE_FIELDS[caseIndex]);
                        writer.statement.addListener(writer);
                        break;
                    }
                    case "deployed": {
                        String label = step.getString("statement", "");
                        if (writer == null || !label.equals(writer.statement.getName())) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        writer.deployed(label);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        writer = null;
                        deployment = null;
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "set-variable": {
                        // Host-neutral carrier for LocalUDF.setResult:
                        // the static toggle lives on the plug-in class,
                        // not on a statement, so the step is addressed
                        // by name only. The write itself emits no
                        // record — its effect surfaces on the next
                        // send's expiry evaluation.
                        String name = step.getString("name", "");
                        if (!"udf-result".equals(name) || deployment == null) {
                            throw new IllegalStateException(
                                    "set-variable " + name + " is not pinned");
                        }
                        JsonValue payload = step.get("payload");
                        if (payload == null || !payload.isBoolean()) {
                            throw new IllegalStateException(
                                    "udf-result payload must be a boolean");
                        }
                        ViewExpressionWindow.LocalUDF.setResult(payload.asBoolean());
                        break;
                    }
                    case "snapshot": {
                        // udf-mode snapshots mirror the Java execution's
                        // env.assertThat reads of the LocalUDF statics
                        // (after E1 and after E3, never after E2).
                        String mode = step.getString("mode", "");
                        if (writer == null || !"udf".equals(mode)) {
                            throw new IllegalStateException(
                                    "snapshot mode " + mode + " is not pinned");
                        }
                        writer.udfObservation();
                        break;
                    }
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }

            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            String theString = payload.getString("theString", null);
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            int intPrimitive = intPrimitiveVal instanceof JsonNumber
                    ? ((JsonNumber) intPrimitiveVal).asInt() : 0;
            runtime.getEventService().sendEventBean(
                    new SupportBean(theString, intPrimitive), "SupportBean");
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds",
                "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
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
                    "observation", "epl", "deploys");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
            validateStringArray(definition.get("deploys"), new String[]{"s0"}, "deploys");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence per case: each case marker is followed
     * by the case's pinned ops — deploy steps carrying the byte-exact
     * statement text, deployed markers, sends with pinned payloads,
     * udf-result toggles, udf-mode snapshots and undeploy-all. Unknown
     * step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            int deploys = 0;
            int deployeds = 0;
            int udfResults = 0;
            int snapshots = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        deploys++;
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "deployed":
                        requireFields(step, "op", "case", "statement");
                        deployeds++;
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deployed step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "send": {
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        if (!"SupportBean".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "intPrimitive");
                        String actual = "SB|" + string(payload, "theString") + "|"
                                + integer(payload, "intPrimitive");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "set-variable": {
                        requireFields(step, "op", "case", "name", "payload");
                        String expected = CASE_UDF_RESULTS[caseIndex][udfResults++];
                        JsonValue payload = step.get("payload");
                        String actual = "udf-result".equals(string(step, "name"))
                                && payload != null && payload.isBoolean()
                                ? Boolean.toString(payload.asBoolean()) : "?";
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("set-variable step " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "snapshot":
                        requireFields(step, "op", "case", "statement", "mode");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_SNAPSHOT_MODES[caseIndex][snapshots++]
                                .equals(string(step, "mode"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != 1 || deployeds != 1
                    || udfResults != CASE_UDF_RESULTS[caseIndex].length
                    || snapshots != CASE_SNAPSHOT_MODES[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex
                        + " step counts are not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object.get(name));
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonValue value) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException("value must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException("value is outside the Java long range", ex);
        }
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

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String[] fields;
        private long sequence;
        private long deployedSequence;
        private long observationSequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime, String[] fields) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.fields = fields;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private void deployed(String label) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "deployed")
                    .add("statement", label)
                    .add("sequence", ++deployedSequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            records.add(record);
        }

        // udfObservation mirrors the Java execution's env.assertThat
        // reads of ViewExpressionWindow.LocalUDF: key is the
        // last-evaluated event's theString, expiryCount is the
        // expired_count argument of that invocation and viewref pins
        // the non-null view_reference assertion.
        private void udfObservation() {
            if (ViewExpressionWindow.LocalUDF.getViewref() == null) {
                throw new IllegalStateException(
                        "udf observation before any udf invocation or with a null view_reference");
            }
            JsonObject value = new JsonObject()
                    .add("key", ViewExpressionWindow.LocalUDF.getKey())
                    .add("expiryCount", ViewExpressionWindow.LocalUDF.getExpiryCount())
                    .add("viewref", true);
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "observation")
                    .add("statement", statement.getName())
                    .add("sequence", ++observationSequence)
                    .add("name", "udf")
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("value", value);
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(row(event, fields));
            }
            return output;
        }
    }

    private static JsonObject row(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Double && ((Double) value).isNaN()
                || value instanceof Float && ((Float) value).isNaN()) {
            return new JsonObject().add("state", "nan");
        }
        if (value instanceof BigDecimal) {
            return Json.value(((BigDecimal) value).toPlainString());
        }
        if (value instanceof BigInteger) {
            return Json.value(value.toString());
        }
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean event : (EventBean[]) value) {
                array.add(normalize(event));
            }
            return array;
        }
        if (value instanceof EventBean) {
            JsonObject fields = new JsonObject();
            EventBean event = (EventBean) value;
            for (String name : new java.util.TreeSet<>(
                    Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            java.util.TreeSet<String> keys = new java.util.TreeSet<>();
            Map<?, ?> mapValue = (Map<?, ?>) value;
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject object = new JsonObject();
            for (String key : keys) {
                object.add(key, normalize(mapValue.get(key)));
            }
            return object;
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
        return Json.value(String.valueOf(value));
    }
}
