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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the view-expression-batch-core-570 bundle:
 * ViewExpressionBatch ords 0/1/2/6 — four statement-level non-aggregate
 * #expr_batch executions whose trigger predicate is evaluated over the
 * accumulating batch and whose flush delivers the whole batch as new data
 * plus the prior delivered batch as old data, each replayed against its
 * own fresh runtime.
 *
 * newest-oldest replays ViewExpressionBatchNewestEventOldestEvent (ord 0):
 * TWO deployments of {@code @name('s0') select irstream * from
 * SupportBean#expr_batch(newest_event.intPrimitive !=
 * oldest_event.intPrimitive, ...)} — first the explicit
 * exclude-trigger-event EPL ({@code ..., false}), then after undeployAll
 * the include-trigger-event EPL ({@code ..., true}). Excluding the
 * trigger event the flushed batch is the accumulation BEFORE the
 * triggering send: E1/1 accumulates, E2/1 stays silent, E3/2 flushes
 * new {E1,E2}, E4/3 flushes new {E3}/old {E1,E2}, E5/3 and E6/3 stay
 * silent and E7/2 flushes new {E4,E5,E6}/old {E3}. Including the
 * trigger event the flushed batch carries the triggering send: E1/1 and
 * E2/1 stay silent, E3/2 flushes new {E1,E2,E3}, E4/3, E5/3 and E6/3
 * stay silent and E7/2 flushes new {E4,E5,E6,E7}/old {E1,E2,E3}.
 *
 * length-batch replays ViewExpressionBatchLengthBatch (ord 1):
 * {@code @name('s0') select irstream * from SupportBean#expr_batch(
 * current_count >= 3, true)}. E1/1 and E2/2 stay silent, then each
 * third send flushes the triple — E3 posts new {E1,E2,E3}, E6 posts
 * new {E4,E5,E6}/old {E1,E2,E3} and E9 posts new {E7,E8,E9}/old
 * {E4,E5,E6}.
 *
 * time-batch replays ViewExpressionBatchTimeBatch (ord 2):
 * {@code @name('s0') select irstream * from SupportBean#expr_batch(
 * newest_timestamp - oldest_timestamp > 2000)} under a virtual clock
 * starting at advanceTime(0). Sends land at E1@1000, E2@1500, E3@1500,
 * E4@3000; the lone advance to t=3100 flushes nothing, E5@3100 flushes
 * new {E1,E2,E3,E4,E5}, E6@3100 starts the next batch, E7@5100 queues,
 * the lone advance to t=5101 flushes nothing and E8@5101 flushes new
 * {E6,E7,E8}/old {E1,E2,E3,E4,E5}. A clock advance alone never
 * evaluates the trigger.
 *
 * event-prop-batch replays ViewExpressionBatchEventPropBatch (ord 6):
 * {@code @name('s0') select irstream theString as val0 from
 * SupportBean#expr_batch(intPrimitive > 0)}. E1/1 flushes new {E1},
 * E2/1 flushes new {E2}/old {E1}, E3/-1 stays silent but is RETAINED in
 * the pending batch, and E4/2 flushes new {E3,E4}/old {E2}.
 *
 * The Java regression milestone() calls are regression-harness savepoints
 * with identical restored state for these non-contextual executions, so
 * the scenario omits them (they pin no observable). All four cases pin
 * listener deliveries only: the executions assert
 * assertPropsPerRowIRPair / assertPropsPerRowLastNew /
 * assertListenerNotInvoked and never the iterator.
 *
 * SupportBean is the real common/internal support bean ((theString,
 * intPrimitive) ctor), so the bean property resolution matches the
 * regression suite exactly. The internal timer is disabled and the
 * runtime initializes at epoch 0 like the regression environment.
 * Records follow the standard protocol: one listener record per
 * delivered update with a per-statement sequence counter starting at 1
 * and time rendered from the current engine time; deployed markers
 * carry their own per-statement sequence. Both counters persist across
 * undeployAll within a case (the newest-oldest include phase deploys
 * a fresh statement whose deployed marker continues at 2 and whose
 * listener deliveries continue at 4..5).
 */
public final class ViewExpressionBatchCore570ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-expression-batch-core-570";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"};

    private static final String DESCRIPTION =
            "ViewExpressionBatch ords 0/1/2/6 — the non-aggregate-trigger "
                    + "quartet. newest-oldest "
                    + "(ViewExpressionBatchNewestEventOldestEvent, ord 0) "
                    + "runs TWO deployments of `@name('s0') select "
                    + "irstream * from SupportBean#expr_batch("
                    + "newest_event.intPrimitive != "
                    + "oldest_event.intPrimitive, ...)`: the "
                    + "exclude-trigger-event (`false`) phase accumulates "
                    + "E1/1, keeps E2/1 silent, flushes new {E1,E2} at "
                    + "E3/2 and new {E3}/old {E1,E2} at E4/3, keeps "
                    + "E5/3+E6/3 silent and flushes new {E4,E5,E6}/old "
                    + "{E3} at E7/2; after undeployAll the "
                    + "include-trigger-event (`true`) phase keeps "
                    + "E1/1+E2/1 silent, flushes new {E1,E2,E3} at "
                    + "E3/2, keeps E4/3+E5/3+E6/3 silent and flushes new "
                    + "{E4,E5,E6,E7}/old {E1,E2,E3} at E7/2. length-batch "
                    + "(ord 1) replays `@name('s0') select irstream * "
                    + "from SupportBean#expr_batch(current_count >= 3, "
                    + "true)`: E1/1+E2/2 stay silent, E3/3 flushes new "
                    + "{E1,E2,E3}, E4/4+E5/5 stay silent, E6/6 flushes "
                    + "new {E4,E5,E6}/old {E1,E2,E3}, E7/7+E8/8 stay "
                    + "silent and E9/9 flushes new {E7,E8,E9}/old "
                    + "{E4,E5,E6}. time-batch (ord 2) replays "
                    + "`@name('s0') select irstream * from "
                    + "SupportBean#expr_batch(newest_timestamp - "
                    + "oldest_timestamp > 2000)` under a virtual clock "
                    + "starting at advanceTime(0): sends land at "
                    + "E1@1000, E2@1500, E3@1500, E4@3000, the lone "
                    + "advance to t=3100 flushes nothing, E5@3100 "
                    + "flushes new {E1,E2,E3,E4,E5}, E6@3100 starts the "
                    + "next batch, E7@5100 queues, the lone advance to "
                    + "t=5101 flushes nothing and E8@5101 flushes new "
                    + "{E6,E7,E8}/old {E1,E2,E3,E4,E5}; a clock advance "
                    + "alone never evaluates the trigger. "
                    + "event-prop-batch (ord 6) replays `@name('s0') "
                    + "select irstream theString as val0 from "
                    + "SupportBean#expr_batch(intPrimitive > 0)`: E1/1 "
                    + "flushes new {E1}, E2/1 flushes new {E2}/old "
                    + "{E1}, E3/-1 stays silent but is retained in the "
                    + "pending batch and E4/2 flushes new {E3,E4}/old "
                    + "{E2}. Java milestone() calls are "
                    + "regression-harness savepoints with identical "
                    + "restored state for these executions, so the "
                    + "scenario omits them.";

    private static final String[] CASES = {
            "newest-oldest", "length-batch", "time-batch", "event-prop-batch"};
    private static final int[] ORDINALS = {0, 1, 2, 6};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-a4012797b6db56fc35ed",
            "java-runtime-bc9403566ad19ca73ed9",
            "java-runtime-d68cff5ac536057a77b8",
            "java-runtime-cfb5c3a23c3ab8bf2029"};
    private static final String[] EXECUTIONS = {
            "ViewExpressionBatchNewestEventOldestEvent",
            "ViewExpressionBatchLengthBatch",
            "ViewExpressionBatchTimeBatch",
            "ViewExpressionBatchEventPropBatch"};
    // Deduplicated inventory id: all four runtime rows share static id
    // java-20551a17cb2af08c67fc, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-20551a17cb2af08c67fc",
            "java-20551a17cb2af08c67fc",
            "java-20551a17cb2af08c67fc",
            "java-20551a17cb2af08c67fc"};
    private static final String[] OBSERVATIONS = {
            "deployed+listener; TWO deployments of "
                    + "#expr_batch(newest_event.intPrimitive != "
                    + "oldest_event.intPrimitive): the explicit "
                    + "exclude-trigger-event (`false`) phase flushes the "
                    + "batch accumulated BEFORE the triggering send "
                    + "(E3/2 posts new {E1,E2}, E4/3 posts new {E3}/old "
                    + "{E1,E2}, E7/2 posts new {E4,E5,E6}/old {E3}), "
                    + "then undeployAll swaps in the explicit "
                    + "include-trigger-event (`true`) EPL which folds "
                    + "the triggering send into the flushed batch (E3/2 "
                    + "posts new {E1,E2,E3}, E7/2 posts new "
                    + "{E4,E5,E6,E7}/old {E1,E2,E3}); the Java execution "
                    + "pins flattened IR pairs only, so no snapshots "
                    + "are recorded",
            "deployed+listener; #expr_batch(current_count >= 3, true) "
                    + "flushes every third send as one batch: E1/1 and "
                    + "E2/2 stay silent, E3/3 posts new {E1,E2,E3}, "
                    + "E4/4 and E5/5 stay silent, E6/6 posts new "
                    + "{E4,E5,E6}/old {E1,E2,E3}, E7/7 and E8/8 stay "
                    + "silent and E9/9 posts new {E7,E8,E9}/old "
                    + "{E4,E5,E6}; the Java execution pins flattened "
                    + "IR pairs only, so no snapshots are recorded",
            "deployed+listener; advanceTime(0) then "
                    + "#expr_batch(newest_timestamp - oldest_timestamp "
                    + "> 2000) under the virtual clock: sends land at "
                    + "E1@1000, E2@1500, E3@1500, E4@3000, the lone "
                    + "advance to t=3100 delivers nothing, E5@3100 "
                    + "posts new {E1,E2,E3,E4,E5}, E6@3100 starts the "
                    + "next batch, E7@5100 queues, the lone advance to "
                    + "t=5101 delivers nothing and E8@5101 posts new "
                    + "{E6,E7,E8}/old {E1,E2,E3,E4,E5}; a clock advance "
                    + "alone never evaluates the trigger",
            "deployed+listener; #expr_batch(intPrimitive > 0) over a "
                    + "theString as val0 projection: E1/1 posts new "
                    + "{E1}, E2/1 posts new {E2}/old {E1}, E3/-1 stays "
                    + "silent but is RETAINED in the pending batch and "
                    + "E4/2 posts new {E3,E4}/old {E2}; the Java "
                    + "execution pins flattened IR pairs only, so no "
                    + "snapshots are recorded"};

    private static final String EPL_NEWEST_OLDEST_EXCLUDE =
            "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, false)";
    private static final String EPL_NEWEST_OLDEST_INCLUDE =
            "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, true)";
    private static final String EPL_LENGTH_BATCH =
            "@name('s0') select irstream * from SupportBean#expr_batch(current_count >= 3, true)";
    private static final String EPL_TIME_BATCH =
            "@name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > 2000)";
    private static final String EPL_EVENT_PROP =
            "@name('s0') select irstream theString as val0 from SupportBean#expr_batch(intPrimitive > 0)";

    private static final String TIME_EPOCH = "1970-01-01T00:00:00.000Z";
    private static final String TIME_1000 = "1970-01-01T00:00:01.000Z";
    private static final String TIME_1500 = "1970-01-01T00:00:01.500Z";
    private static final String TIME_3000 = "1970-01-01T00:00:03.000Z";
    private static final String TIME_3100 = "1970-01-01T00:00:03.100Z";
    private static final String TIME_5100 = "1970-01-01T00:00:05.100Z";
    private static final String TIME_5101 = "1970-01-01T00:00:05.101Z";

    // Case-level EPL pin: the execution's first (or only) compileDeploy
    // text. newest-oldest's second deployment is pinned positionally in
    // CASE_DEPLOY_EPLS below.
    private static final String[] CASE_EPLS = {
            EPL_NEWEST_OLDEST_EXCLUDE, EPL_LENGTH_BATCH,
            EPL_TIME_BATCH, EPL_EVENT_PROP};

    // Pinned positional deploy labels plus each deploy's byte-exact
    // statement text; newest-oldest redeploys s0 once per phase.
    private static final String[][] CASE_DEPLOYS = {
            {"s0", "s0"}, {"s0"}, {"s0"}, {"s0"}};
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_NEWEST_OLDEST_EXCLUDE, EPL_NEWEST_OLDEST_INCLUDE},
            {EPL_LENGTH_BATCH},
            {EPL_TIME_BATCH},
            {EPL_EVENT_PROP}};

    // Pinned row projections: the wildcard-select cases pin theString;
    // event-prop-batch pins the val0 alias.
    private static final String[][] CASE_FIELDS = {
            {"theString"}, {"theString"}, {"theString"}, {"val0"}};

    // Pinned op sequences per case (after the case marker). milestone()
    // savepoints are omitted; advance-time steps sit exactly where the
    // Java execution calls env.advanceTime (the t=3100 and t=5101
    // advances pin that a clock advance alone never flushes).
    private static final String[][] CASE_OPS = {
            {"deploy", "deployed",
                    "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all",
                    "deploy", "deployed",
                    "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "send", "send", "send", "send", "send", "send",
                    "send", "send", "send",
                    "undeploy-all"},
            {"advance-time", "deploy", "deployed",
                    "advance-time", "send",
                    "advance-time", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send", "send",
                    "advance-time", "send",
                    "advance-time", "send",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "SB|theString|intPrimitive" (the negative E3 of event-prop-batch
    // is pinned verbatim: it stays silent but is retained pending).
    private static final String[][] CASE_SENDS = {
            {"SB|E1|1", "SB|E2|1", "SB|E3|2", "SB|E4|3", "SB|E5|3",
                    "SB|E6|3", "SB|E7|2",
                    "SB|E1|1", "SB|E2|1", "SB|E3|2", "SB|E4|3", "SB|E5|3",
                    "SB|E6|3", "SB|E7|2"},
            {"SB|E1|1", "SB|E2|2", "SB|E3|3", "SB|E4|4", "SB|E5|5",
                    "SB|E6|6", "SB|E7|7", "SB|E8|8", "SB|E9|9"},
            {"SB|E1|1", "SB|E2|2", "SB|E3|3", "SB|E4|4", "SB|E5|5",
                    "SB|E6|6", "SB|E7|7", "SB|E8|8"},
            {"SB|E1|1", "SB|E2|1", "SB|E3|-1", "SB|E4|2"}
    };

    // Pinned advance-time instants per case, in advance order (only
    // time-batch advances the virtual clock; it starts at epoch like
    // the execution's advanceTime(0)).
    private static final String[][] CASE_ADVANCES = {
            {},
            {},
            {TIME_EPOCH, TIME_1000, TIME_1500, TIME_3000, TIME_3100,
                    TIME_5100, TIME_5101},
            {}
    };

    private static final int EXPECTED_STEPS = 61;
    private static final int EXPECTED_RECORDS = 18;

    private ViewExpressionBatchCore570ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewExpressionBatchCore570ScenarioOracle <scenario.json>");
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
            Map<String, TraceWriter> writers = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            // Sequence counters persist across undeployAll: the
            // newest-oldest include phase redeploys s0 and continues the
            // deployed/listener sequences exactly like the Go runner's
            // per-case sequence map.
            Map<String, Integer> sequences = new HashMap<>();
            int deployIndex = 0;
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
                        String label = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                                epl, new CompilerArguments(runtime.getRuntimePath()));
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("deploy of case " + caseName
                                    + " produced " + deployed.length + " statements");
                        }
                        EPStatement statement = deployed[0];
                        statementsByName.put(label, statement);
                        TraceWriter writer = new TraceWriter(records, caseName, statement,
                                runtime, CASE_FIELDS[caseIndex], sequences);
                        writers.put(label, writer);
                        // Every statement carries Java's addListener("s0").
                        statement.addListener(writer);
                        deployIndex++;
                        break;
                    }
                    case "deployed": {
                        String label = step.getString("statement", "");
                        if (!statementsByName.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        String key = label + ":deployed";
                        int sequence = sequences.getOrDefault(key, 0) + 1;
                        sequences.put(key, sequence);
                        JsonObject record = new JsonObject()
                                .add("case", caseName)
                                .add("operation", "deployed")
                                .add("statement", label)
                                .add("sequence", sequence)
                                .add("time", Instant.ofEpochMilli(
                                        runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        writers.clear();
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            if (deployIndex != CASE_DEPLOYS[caseIndex].length) {
                throw new IllegalStateException("case " + caseName + " deployed "
                        + deployIndex + " statements, want "
                        + CASE_DEPLOYS[caseIndex].length);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
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
            validateStringArray(definition.get("deploys"), CASE_DEPLOYS[index], "deploys");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence per case: each case marker is followed
     * by the case's pinned ops — advance-time instants, deploy steps
     * carrying the byte-exact statement text per deployment, deployed
     * markers, sends with pinned payloads and undeploy-all. Unknown step
     * fields are rejected.
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
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        int deployIndex = deploys++;
                        if (!CASE_DEPLOYS[caseIndex][deployIndex].equals(string(step, "statement"))
                                || !CASE_DEPLOY_EPLS[caseIndex][deployIndex]
                                .equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "deployed":
                        requireFields(step, "op", "case", "statement");
                        if (!CASE_DEPLOYS[caseIndex][deployeds++].equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deployed step " + cursor + " is not pinned");
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
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length
                    || deploys != CASE_DEPLOYS[caseIndex].length
                    || deployeds != CASE_DEPLOYS[caseIndex].length
                    || advances != CASE_ADVANCES[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " step counts are not pinned");
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
        private final Map<String, Integer> sequences;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime, String[] fields,
                            Map<String, Integer> sequences) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.fields = fields;
            this.sequences = sequences;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            int sequence = sequences.getOrDefault(statement.getName(), 0) + 1;
            sequences.put(statement.getName(), sequence);
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
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
