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
import java.util.Iterator;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the view-expression-variable-571 bundle:
 * ViewExpressionBatch ords 11/12 + ViewExpressionWindow ords 11/12 — the
 * variable quartet. Each execution deploys a create-variable module under
 * the virtual clock (advanceTime(0) first) and mutates the variable
 * through runtimeSetVariable; variable-driven re-evaluation surfaces on
 * the next clock advance, never at the assignment itself.
 *
 * dynamic-time-batch replays ViewExpressionBatchDynamicTimeBatch
 * (batch ord 11): {@code create variable long SIZE = 1000; @name('s0')
 * select irstream * from SupportBean#expr_batch(newest_timestamp -
 * oldest_timestamp > SIZE)} (strict {@code >}, trailing newline). E1@1000
 * and E2@1900 accumulate silently, runtimeSetVariable(s0,SIZE,500) plus
 * the 1901 advance flushes new {E1,E2} WITHOUT a new send, E3/E4
 * accumulate through the silent 2500 advance, E5@2500 flushes new
 * {E3,E4,E5}/old {E1,E2}, E6@3100 and E7@3700 accumulate under SIZE=999
 * and E8@4100 flushes new {E6,E7,E8}/old {E3,E4,E5}.
 *
 * variable-batch replays ViewExpressionBatchVariableBatch (batch ord 12):
 * {@code create variable boolean POST = false; @name('s0') select
 * irstream * from SupportBean#expr_batch(POST)} (trailing newline).
 * POST=false keeps E1@1000 silent, POST=true plus the 1001 advance
 * flushes new {E1} without a send, E2 and E3 each flush the pending
 * batch, POST=false silences E4/E5 and the 2000 advance, POST=true plus
 * the 2001 advance flushes new {E4,E5}/old {E3} and E6 flushes new
 * {E6}/old {E4,E5}.
 *
 * variable-window replays ViewExpressionWindowVariable (window ord 11):
 * {@code create variable boolean KEEP = true; @name('s0') select irstream
 * * from SupportBean#expr(KEEP)} (trailing newline). E1@1000 is retained,
 * KEEP=false alone does not evict (the iterator still reads {E1} — the
 * Java test's listenerReset is an assert-helper reset only and carries
 * no trace step), the 1001 advance delivers the old-only {E1} expel,
 * E2@1001 self-expires as an {E2}/{E2} pair and after KEEP=true is
 * restored E3@1001 is retained. Every Java iterator assertion posts an
 * in-order snapshot record.
 *
 * dynamic-time-window replays ViewExpressionWindowDynamicTimeWindow
 * (window ord 12): {@code create variable long SIZE = 1000; @name('s0')
 * select irstream * from SupportBean#expr(newest_timestamp -
 * oldest_timestamp < SIZE)} (strict {@code <}, NO trailing
 * semicolon/newline — the byte-exact asymmetry against the other three
 * EPLs is pinned verbatim). E1@1000 accumulates, E2@2000 expires E1 (the
 * span 1000 is not {@code <} 1000), SIZE=10000 keeps {E2,E3} at E3@5000
 * and shrinking to SIZE=2000 lets the 6000 advance lazily evict E2 so
 * E4@6000 leaves {E3,E4}.
 *
 * The Java regression milestone() calls are regression-harness savepoints
 * with identical restored state for these non-contextual executions, so
 * the scenario omits them (they pin no observable). Batch cases pin
 * listener deliveries only (assertPropsPerRowIRPair /
 * assertListenerNotInvoked); window cases pin in-order iterator snapshots
 * at every assertPropsPerRowIterator plus the listener deliveries.
 *
 * SupportBean is the real common/internal support bean ((theString,
 * intPrimitive) ctor), so the bean property resolution matches the
 * regression suite exactly. The internal timer is disabled and the
 * runtime initializes at epoch 0 like the regression environment.
 * Records follow the standard protocol: one listener record per
 * delivered update with a per-statement sequence counter starting at 1
 * and time rendered from the current engine time; deployed markers carry
 * their own per-statement sequence; snapshots carry sequence 0. The
 * set-variable step itself is silent in Java (the variable service's
 * setVariableValue emits no listener event), so no record is emitted for
 * it — the re-evaluation delivery rides the following advance.
 */
public final class ViewExpressionVariable571ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-expression-variable-571";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "ViewExpressionBatch ords 11/12 + ViewExpressionWindow ords "
                    + "11/12 — the variable quartet. dynamic-time-batch "
                    + "(ViewExpressionBatchDynamicTimeBatch, ord 11) "
                    + "replays `create variable long SIZE = 1000; "
                    + "@name('s0') select irstream * from "
                    + "SupportBean#expr_batch(newest_timestamp - "
                    + "oldest_timestamp > SIZE)` (strict `>`, trailing "
                    + "newline): E1@1000 and E2@1900 accumulate silently, "
                    + "runtimeSetVariable(s0,SIZE,500) plus the 1901 "
                    + "advance flushes new {E1,E2} WITHOUT a new send, "
                    + "E3/E4 accumulate through the silent 2500 advance, "
                    + "E5@2500 flushes new {E3,E4,E5}/old {E1,E2}, "
                    + "E6@3100 and E7@3700 accumulate under SIZE=999 and "
                    + "E8@4100 flushes new {E6,E7,E8}/old {E3,E4,E5}. "
                    + "variable-batch (ViewExpressionBatchVariableBatch, "
                    + "ord 12) replays `create variable boolean POST = "
                    + "false; @name('s0') select irstream * from "
                    + "SupportBean#expr_batch(POST)` (trailing newline): "
                    + "POST=false keeps E1@1000 silent, POST=true plus "
                    + "the 1001 advance flushes new {E1} without a send, "
                    + "E2 and E3 each flush the pending batch, POST=false "
                    + "silences E4/E5 and the 2000 advance, POST=true "
                    + "plus the 2001 advance flushes new {E4,E5}/old "
                    + "{E3} and E6 flushes new {E6}/old {E4,E5}. "
                    + "variable-window (ViewExpressionWindowVariable, "
                    + "ord 11) replays `create variable boolean KEEP = "
                    + "true; @name('s0') select irstream * from "
                    + "SupportBean#expr(KEEP)` (trailing newline): "
                    + "E1@1000 is retained, KEEP=false alone does not "
                    + "evict (iterator still {E1}), the 1001 advance "
                    + "delivers the old-only {E1} expel, E2@1001 "
                    + "self-expires as an {E2}/{E2} pair and after "
                    + "KEEP=true is restored E3@1001 is retained. "
                    + "dynamic-time-window "
                    + "(ViewExpressionWindowDynamicTimeWindow, ord 12) "
                    + "replays `create variable long SIZE = 1000; "
                    + "@name('s0') select irstream * from "
                    + "SupportBean#expr(newest_timestamp - "
                    + "oldest_timestamp < SIZE)` (strict `<`, NO "
                    + "trailing semicolon/newline — byte-exact asymmetry "
                    + "pinned): E1@1000 accumulates, E2@2000 expires E1 "
                    + "(span 1000 is not < 1000), SIZE=10000 keeps "
                    + "{E2,E3} at E3@5000 and shrinking to SIZE=2000 "
                    + "lets the 6000 advance lazily evict E2 so E4@6000 "
                    + "leaves {E3,E4}. Java milestone() calls are "
                    + "regression-harness savepoints with identical "
                    + "restored state for these executions, so the "
                    + "scenario omits them.";

    private static final String[] CASES = {
            "dynamic-time-batch", "variable-batch",
            "variable-window", "dynamic-time-window"};
    private static final int[] ORDINALS = {11, 12, 11, 12};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-ace804f86e16f62ae8f5",
            "java-runtime-afb34438de3f25c964cb",
            "java-runtime-8f167cedfd9b5d56fe67",
            "java-runtime-5ccd88d79fe264701059"};
    private static final String[] EXECUTIONS = {
            "ViewExpressionBatchDynamicTimeBatch",
            "ViewExpressionBatchVariableBatch",
            "ViewExpressionWindowVariable",
            "ViewExpressionWindowDynamicTimeWindow"};
    // Deduplicated inventory ids: the two batch runtime rows share static
    // id java-20551a17cb2af08c67fc and the two window runtime rows share
    // java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-20551a17cb2af08c67fc",
            "java-20551a17cb2af08c67fc",
            "java-06e6b1f6c905b8f12b82",
            "java-06e6b1f6c905b8f12b82"};
    private static final String[] OBSERVATIONS = {
            "deployed+listener; create variable long SIZE = 1000 with "
                    + "#expr_batch(newest_timestamp - oldest_timestamp > "
                    + "SIZE): E1@1000 and E2@1900 accumulate silently, "
                    + "runtimeSetVariable(s0,SIZE,500) plus the 1901 "
                    + "advance flushes new {E1,E2} WITHOUT a new send, "
                    + "E3/E4 accumulate through the silent 2500 advance, "
                    + "E5@2500 flushes new {E3,E4,E5}/old {E1,E2}, "
                    + "E6@3100 and E7@3700 accumulate under SIZE=999 and "
                    + "E8@4100 flushes new {E6,E7,E8}/old {E3,E4,E5}; the "
                    + "Java execution pins flattened IR pairs only, so no "
                    + "snapshots are recorded",
            "deployed+listener; create variable boolean POST = false with "
                    + "#expr_batch(POST): POST=false keeps E1@1000 "
                    + "silent, POST=true plus the 1001 advance flushes "
                    + "new {E1} without a send, E2 and E3 each flush the "
                    + "pending batch (new {E2}/old {E1}, new {E3}/old "
                    + "{E2}), POST=false silences E4/E5 and the 2000 "
                    + "advance, POST=true plus the 2001 advance flushes "
                    + "new {E4,E5}/old {E3} and E6 flushes new {E6}/old "
                    + "{E4,E5}; the Java execution pins flattened IR "
                    + "pairs only, so no snapshots are recorded",
            "deployed+listener+snapshot; create variable boolean KEEP = "
                    + "true with #expr(KEEP): E1@1000 is retained, "
                    + "KEEP=false alone does not evict (the iterator "
                    + "still reads {E1}), the 1001 advance delivers the "
                    + "old-only {E1} expel and an empty iterator, E2@1001 "
                    + "self-expires as an {E2}/{E2} pair with an empty "
                    + "iterator, and after KEEP=true is restored E3@1001 "
                    + "is retained (new-only delivery, iterator {E3}); "
                    + "in-order iterator pins project theString at every "
                    + "assertPropsPerRowIterator",
            "deployed+listener+snapshot; create variable long SIZE = "
                    + "1000 with #expr(newest_timestamp - "
                    + "oldest_timestamp < SIZE) (strict `<`, no trailing "
                    + "semicolon): E1@1000 accumulates, E2@2000 expires "
                    + "E1 (span 1000 is not < 1000), SIZE=10000 keeps "
                    + "{E2,E3} at E3@5000 and shrinking to SIZE=2000 "
                    + "lets the 6000 advance lazily evict E2 so E4@6000 "
                    + "leaves {E3,E4}; in-order iterator pins project "
                    + "theString at every assertPropsPerRowIterator"};

    private static final String EPL_DYNAMIC_TIME_BATCH =
            "create variable long SIZE = 1000;\n"
                    + "@name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > SIZE);\n";
    private static final String EPL_VARIABLE_BATCH =
            "create variable boolean POST = false;\n"
                    + "@name('s0') select irstream * from SupportBean#expr_batch(POST);\n";
    private static final String EPL_VARIABLE_WINDOW =
            "create variable boolean KEEP = true;\n"
                    + "@name('s0') select irstream * from SupportBean#expr(KEEP);\n";
    private static final String EPL_DYNAMIC_TIME_WINDOW =
            "create variable long SIZE = 1000;\n"
                    + "@name('s0') select irstream * from SupportBean#expr(newest_timestamp - oldest_timestamp < SIZE)";

    private static final String TIME_EPOCH = "1970-01-01T00:00:00.000Z";
    private static final String TIME_1000 = "1970-01-01T00:00:01.000Z";
    private static final String TIME_1001 = "1970-01-01T00:00:01.001Z";
    private static final String TIME_1900 = "1970-01-01T00:00:01.900Z";
    private static final String TIME_1901 = "1970-01-01T00:00:01.901Z";
    private static final String TIME_2000 = "1970-01-01T00:00:02.000Z";
    private static final String TIME_2001 = "1970-01-01T00:00:02.001Z";
    private static final String TIME_2300 = "1970-01-01T00:00:02.300Z";
    private static final String TIME_2500 = "1970-01-01T00:00:02.500Z";
    private static final String TIME_3100 = "1970-01-01T00:00:03.100Z";
    private static final String TIME_3700 = "1970-01-01T00:00:03.700Z";
    private static final String TIME_4100 = "1970-01-01T00:00:04.100Z";
    private static final String TIME_5000 = "1970-01-01T00:00:05.000Z";
    private static final String TIME_6000 = "1970-01-01T00:00:06.000Z";

    // Case-level EPL pin: the execution's compileDeploy text — the full
    // create-variable module, byte-exact including the missing trailing
    // `;\n` on dynamic-time-window.
    private static final String[] CASE_EPLS = {
            EPL_DYNAMIC_TIME_BATCH, EPL_VARIABLE_BATCH,
            EPL_VARIABLE_WINDOW, EPL_DYNAMIC_TIME_WINDOW};

    // Pinned row projections: every case projects theString.
    private static final String[][] CASE_FIELDS = {
            {"theString"}, {"theString"}, {"theString"}, {"theString"}};

    // Pinned op sequences per case (after the case marker). milestone()
    // savepoints are omitted; advance-time steps sit exactly where the
    // Java execution calls env.advanceTime, set-variable steps mirror
    // runtimeSetVariable(s0, name, value) and snapshot steps sit where
    // Java asserts the iterator.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send",
                    "advance-time", "send",
                    "set-variable",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "set-variable",
                    "advance-time", "send",
                    "advance-time", "send",
                    "undeploy-all"},
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send",
                    "set-variable",
                    "advance-time", "send", "send",
                    "set-variable",
                    "send", "send",
                    "advance-time",
                    "set-variable",
                    "advance-time", "send",
                    "undeploy-all"},
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send", "snapshot",
                    "set-variable", "snapshot",
                    "advance-time", "snapshot",
                    "send", "snapshot",
                    "set-variable",
                    "send", "snapshot",
                    "undeploy-all"},
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send", "snapshot",
                    "advance-time", "send", "snapshot",
                    "set-variable",
                    "advance-time", "send", "snapshot",
                    "set-variable",
                    "advance-time", "send", "snapshot",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "SB|theString|intPrimitive".
    private static final String[][] CASE_SENDS = {
            {"SB|E1|0", "SB|E2|0", "SB|E3|0", "SB|E4|0", "SB|E5|0",
                    "SB|E6|0", "SB|E7|0", "SB|E8|0"},
            {"SB|E1|1", "SB|E2|1", "SB|E3|1", "SB|E4|1", "SB|E5|2",
                    "SB|E6|1"},
            {"SB|E1|1", "SB|E2|2", "SB|E3|3"},
            {"SB|E1|0", "SB|E2|0", "SB|E3|0", "SB|E4|0"}
    };

    // Pinned advance-time instants per case, in advance order.
    private static final String[][] CASE_ADVANCES = {
            {TIME_EPOCH, TIME_1000, TIME_1900, TIME_1901, TIME_2300,
                    TIME_2500, TIME_3100, TIME_3700, TIME_4100},
            {TIME_EPOCH, TIME_1000, TIME_1001, TIME_2000, TIME_2001},
            {TIME_EPOCH, TIME_1000, TIME_1001},
            {TIME_EPOCH, TIME_1000, TIME_2000, TIME_5000, TIME_6000}
    };

    // Pinned variable assignments per case, in set order, encoded
    // "statement|name|value". Long variables decode as integer payloads;
    // boolean variables decode as JSON booleans.
    private static final String[][] CASE_VARIABLE_SETS = {
            {"s0|SIZE|500", "s0|SIZE|999"},
            {"s0|POST|true", "s0|POST|false", "s0|POST|true"},
            {"s0|KEEP|false", "s0|KEEP|true"},
            {"s0|SIZE|10000", "s0|SIZE|2000"}
    };

    // Pinned snapshot modes per case; all iterator pins are in-order
    // (assertPropsPerRowIterator).
    private static final String[][] CASE_SNAPSHOT_MODES = {
            {},
            {},
            {"ordered", "ordered", "ordered", "ordered", "ordered"},
            {"ordered", "ordered", "ordered", "ordered"}
    };

    private static final int EXPECTED_STEPS = 77;
    private static final int EXPECTED_RECORDS = 30;

    private ViewExpressionVariable571ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewExpressionVariable571ScenarioOracle <scenario.json>");
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
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                new CompilerArguments(runtime.getRuntimePath()));
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
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "set-variable": {
                        if (deployment == null) {
                            throw new IllegalStateException(
                                    "set-variable without a deployment");
                        }
                        // Mirrors env.runtimeSetVariable("s0", name, value):
                        // the variable service's deployment-scoped write
                        // emits no record — the re-evaluation delivery
                        // rides the following advance-time step.
                        JsonValue payload = step.get("payload");
                        Object value;
                        if (payload != null && payload.isBoolean()) {
                            value = payload.asBoolean();
                        } else if (payload instanceof JsonNumber) {
                            value = longNumber(payload);
                        } else {
                            throw new IllegalStateException(
                                    "set-variable payload is not pinned");
                        }
                        runtime.getVariableService().setVariableValue(
                                deployment.getDeploymentId(),
                                step.getString("name", ""), value);
                        break;
                    }
                    case "snapshot": {
                        if (writer == null) {
                            throw new IllegalStateException(
                                    "snapshot without a deployed statement");
                        }
                        writer.snapshot();
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
     * by the case's pinned ops — advance-time instants, deploy steps
     * carrying the byte-exact create-variable module text, deployed
     * markers, sends with pinned payloads, set-variable writes with
     * pinned statement/name/value, snapshot reads with pinned modes and
     * undeploy-all. Unknown step fields are rejected.
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
            int advances = 0;
            int variableSets = 0;
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
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_ADVANCES[caseIndex][advances++].equals(string(step, "at"))) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "set-variable": {
                        requireFields(step, "op", "case", "statement", "name", "payload");
                        String expected = CASE_VARIABLE_SETS[caseIndex][variableSets++];
                        String actual = string(step, "statement") + "|"
                                + string(step, "name") + "|"
                                + variableValue(step.get("payload"));
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
                    || advances != CASE_ADVANCES[caseIndex].length
                    || variableSets != CASE_VARIABLE_SETS[caseIndex].length
                    || snapshots != CASE_SNAPSHOT_MODES[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex
                        + " step counts are not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static String variableValue(JsonValue value) {
        if (value != null && value.isBoolean()) {
            return Boolean.toString(value.asBoolean());
        }
        return Long.toString(longNumber(value));
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

        private void snapshot() {
            JsonArray rows = new JsonArray();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                rows.add(row(iterator.next(), fields));
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "snapshot")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (rows.size() > 0) {
                record.add("new", rows);
            }
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
