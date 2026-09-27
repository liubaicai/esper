import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
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
 * Direct Esper 9.0.0 oracle for the view-expression-window-agg-568 bundle:
 * ViewExpressionWindow ords 7/8/9/10 — two statement-level aggregate #expr
 * keep-predicate executions and two three-statement named-window modules
 * with #expr retention plus an on-delete trigger, each replayed against its
 * own fresh runtime.
 *
 * aggregation-ungrouped replays ViewExpressionWindowAggregationUngrouped
 * (ord 7): {@code @name('s0') select irstream theString from
 * SupportBean#expr(sum(intPrimitive) < 10)}. The full nine-send sequence
 * runs: E1/1 keeps, E2/9 evicts {E1}, the E3/11 and E4/12 inserts
 * self-expire (each posts its own event on both the insert and remove
 * streams while the iterator is empty), E5/1-E7/3 accumulate {E5,E6,E7},
 * E8/6 evicts {E5,E6} and E9/9 evicts {E7,E8}. Every Java iterator
 * assertion posts an ordered snapshot record (the empty post-E3/post-E4
 * windows included).
 *
 * aggregation-groupwin replays ViewExpressionWindowAggregationWGroupwin
 * (ord 8): {@code @name('s0') select irstream theString from
 * SupportBean#groupwin(intPrimitive)#expr(sum(longPrimitive) < 10)}. The
 * keep predicate holds per intPrimitive group: E5/2/6 evicts the whole
 * group-2 pair {E2,E4} in one delivery and E6/1/2 evicts the group-1 head
 * {E1}. The Java execution pins assertPropsPerRowIRPairFlattened only, so
 * this case records listener deliveries without snapshots.
 *
 * named-window-delete (ord 9) and aggregation-on-delete (ord 10) replay
 * their three-statement modules {@code @name('s0') create window
 * NW#expr(true)} / {@code NW#expr(sum(intPrimitive) < 10) as SupportBean}
 * plus a wildcard insert-into and an on-SupportBean_A delete
 * (theString = id). The module deploys as ONE compileDeploy exactly like
 * the regression execution; the s0 create-window statement carries
 * addListener and the asserted iterator. ord 9's A(E2) delete posts old
 * {E2} between ordered iterator snapshots {E1,E2,E3} and {E1,E3}; ord 10's
 * keep predicate re-evaluates after the delete-triggered removal so E4/2
 * posts new {E4} while the retention pops {E1} (sum(intPrimitive) = 10
 * fails < 10).
 *
 * The Java regression milestone() calls and the ord-9 listenerReset are
 * regression-harness savepoints with identical restored state for these
 * non-contextual executions, so the scenario omits them (they pin no
 * observable).
 *
 * SupportBean is the real common/internal support bean ((theString,
 * intPrimitive) ctor plus setLongPrimitive for the groupwin sends) and
 * SupportBean_A carries the delete trigger id, so the bean property
 * resolution matches the regression suite exactly. Records follow the
 * standard protocol: one listener record per delivered update with a
 * per-statement sequence counter starting at 1 and time rendered from the
 * current engine time; deployed markers carry the deploy ordinal;
 * snapshots carry sequence 0, the pinned theString projection and "new"
 * only when rows exist.
 */
public final class ViewExpressionWindowAgg568ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-expression-window-agg-568";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java"};

    private static final String DESCRIPTION =
            "ViewExpressionWindow ords 7/8/9/10 — the aggregate-keep and "
                    + "named-window-delete quartet. aggregation-ungrouped "
                    + "(ViewExpressionWindowAggregationUngrouped, ord 7) "
                    + "replays `@name('s0') select irstream theString from "
                    + "SupportBean#expr(sum(intPrimitive) < 10)`: sends E1/1 "
                    + "through E9/9 post the full nine-delivery sequence "
                    + "including both self-expiring inserts (E3/11 posts new "
                    + "{E3} with old {E2,E3} and an empty iterator; E4/12 "
                    + "posts new {E4} with old {E4}) before E5-E7 accumulate "
                    + "{E5,E6,E7}, E8/6 evicts {E5,E6} and E9/9 evicts "
                    + "{E7,E8}; every Java iterator assertion posts an "
                    + "ordered snapshot. aggregation-groupwin (ord 8) "
                    + "replays `@name('s0') select irstream theString from "
                    + "SupportBean#groupwin(intPrimitive)#expr("
                    + "sum(longPrimitive) < 10)`: E1-E4 accumulate per "
                    + "group, E5/2/6 evicts the whole group-2 pair {E2,E4} "
                    + "and E6/1/2 evicts the group-1 head {E1}; the Java "
                    + "execution pins flattened IR pairs only, so the case "
                    + "records listener deliveries without snapshots. "
                    + "named-window-delete (ord 9) and aggregation-on-delete "
                    + "(ord 10) replay their three-statement modules "
                    + "`@name('s0') create window NW#expr(true)` / "
                    + "`NW#expr(sum(intPrimitive) < 10) as SupportBean` "
                    + "plus a wildcard insert-into and an on-SupportBean_A "
                    + "delete (theString = id): ord 9's A(E2) delete posts "
                    + "old {E2} between ordered iterator snapshots "
                    + "{E1,E2,E3} and {E1,E3}; ord 10's keep predicate "
                    + "re-evaluates after the delete-triggered removal so "
                    + "E4/2 pops {E1} (sum(intPrimitive) = 10 fails < 10). "
                    + "Java milestone() calls and listenerReset are "
                    + "regression-harness savepoints with identical "
                    + "restored state, so the scenario omits them.";

    private static final String[] CASES = {
            "aggregation-ungrouped", "aggregation-groupwin",
            "named-window-delete", "aggregation-on-delete"};
    private static final int[] ORDINALS = {7, 8, 9, 10};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-0201ff1e8d8eaabe883f",
            "java-runtime-bef7f02bfc8cb8b18b86",
            "java-runtime-d9281b1cc6d48c4984e1",
            "java-runtime-e4c4569b44a3e479c37e"};
    private static final String[] EXECUTIONS = {
            "ViewExpressionWindowAggregationUngrouped",
            "ViewExpressionWindowAggregationWGroupwin",
            "ViewExpressionWindowNamedWindowDelete",
            "ViewExpressionWindowAggregationWOnDelete"};
    // Deduplicated inventory id: all four runtime rows share static id
    // java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
    private static final String[] STATIC_IDS = {
            "java-06e6b1f6c905b8f12b82",
            "java-06e6b1f6c905b8f12b82",
            "java-06e6b1f6c905b8f12b82",
            "java-06e6b1f6c905b8f12b82"};
    private static final String[] OBSERVATIONS = {
            "deployed+listener+snapshot; ungrouped sum(intPrimitive) < 10 "
                    + "keep over the full nine-send sequence: E1/1 keeps, "
                    + "E2/9 evicts {E1}, E3/11 and E4/12 self-expire "
                    + "(new {E3}/old {E2,E3} and new {E4}/old {E4} with "
                    + "empty iterators), E5/1-E7/3 accumulate {E5,E6,E7}, "
                    + "E8/6 evicts {E5,E6} and E9/9 evicts {E7,E8}; ordered "
                    + "iterator snapshots pin theString after every send",
            "deployed+listener; #groupwin(intPrimitive)#expr("
                    + "sum(longPrimitive) < 10) keeps rows per group: "
                    + "E1/1/5-E4/2/4 accumulate, E5/2/6 evicts the whole "
                    + "group-2 pair {E2,E4} in one delivery and E6/1/2 "
                    + "evicts the group-1 head {E1}; the Java execution "
                    + "pins flattened IR pairs only, so no snapshots are "
                    + "recorded",
            "deployed+listener+snapshot; one three-statement module: "
                    + "@name('s0') create window NW#expr(true) as "
                    + "SupportBean, a wildcard insert-into and an "
                    + "on-SupportBean_A delete (theString = id); sends "
                    + "E1-E3 accumulate, the ordered iterator pins "
                    + "{E1,E2,E3}, the A(E2) delete posts old {E2} and the "
                    + "iterator pins {E1,E3}",
            "deployed+listener; the named-window-delete module with "
                    + "NW#expr(sum(intPrimitive) < 10) retention: E1/1 and "
                    + "E2/8 insert, the A(E2) delete posts old {E2} with "
                    + "the keep predicate re-evaluated over {E1}, E3/7 "
                    + "inserts and E4/2 posts new {E4} while the retention "
                    + "pops {E1} (sum(intPrimitive) = 10 fails < 10); the "
                    + "Java execution pins flattened IR pairs only, so no "
                    + "snapshots are recorded"};

    private static final String EPL_AGG_UNGROUPED =
            "@name('s0') select irstream theString from SupportBean#expr(sum(intPrimitive) < 10)";
    private static final String EPL_AGG_GROUPWIN =
            "@name('s0') select irstream theString from SupportBean#groupwin(intPrimitive)#expr(sum(longPrimitive) < 10)";
    private static final String EPL_NW_CREATE_KEEP =
            "@name('s0') create window NW#expr(true) as SupportBean";
    private static final String EPL_NW_CREATE_AGG =
            "@name('s0') create window NW#expr(sum(intPrimitive) < 10) as SupportBean";
    private static final String EPL_NW_INSERT =
            "insert into NW select * from SupportBean";
    private static final String EPL_NW_DELETE =
            "on SupportBean_A delete from NW where theString = id";

    // Byte-exact module text the Java executions pass to compileDeploy:
    // each statement terminated by ';' plus a newline.
    private static String moduleEpl(String createEpl) {
        return createEpl + ";\n" + EPL_NW_INSERT + ";\n" + EPL_NW_DELETE + ";\n";
    }

    private static final String[] CASE_EPLS = {
            EPL_AGG_UNGROUPED, EPL_AGG_GROUPWIN,
            moduleEpl(EPL_NW_CREATE_KEEP), moduleEpl(EPL_NW_CREATE_AGG)};

    // Pinned deploy labels per case plus each label's byte-exact statement
    // text; the named-window cases deploy their three labels as one
    // compileDeploy module.
    private static final String[][] CASE_DEPLOYS = {
            {"s0"}, {"s0"}, {"s0", "insert", "delete"}, {"s0", "insert", "delete"}};
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_AGG_UNGROUPED},
            {EPL_AGG_GROUPWIN},
            {EPL_NW_CREATE_KEEP, EPL_NW_INSERT, EPL_NW_DELETE},
            {EPL_NW_CREATE_AGG, EPL_NW_INSERT, EPL_NW_DELETE}};

    // Pinned row projections: every case pins theString.
    private static final String[][] CASE_FIELDS = {
            {"theString"}, {"theString"}, {"theString"}, {"theString"}};

    // Pinned op sequences per case (after the case marker). Every Java
    // iterator assertion posts a snapshot step; milestone() savepoints and
    // the ord-9 listenerReset are omitted.
    private static final String[][] CASE_OPS = {
            {"deploy", "deployed",
                    "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "send", "snapshot", "send", "snapshot", "send", "snapshot",
                    "undeploy-all"},
            {"deploy", "deployed",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy", "deploy", "deploy", "deployed", "deployed", "deployed",
                    "send", "send", "send", "snapshot",
                    "send", "snapshot",
                    "undeploy-all"},
            {"deploy", "deploy", "deploy", "deployed", "deployed", "deployed",
                    "send", "send", "send", "send", "send",
                    "undeploy-all"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "SB|theString|intPrimitive", "SBL|theString|intPrimitive|longPrimitive"
    // for the groupwin sends or "A|id" for the SupportBean_A triggers.
    private static final String[][] CASE_SENDS = {
            {"SB|E1|1", "SB|E2|9", "SB|E3|11", "SB|E4|12", "SB|E5|1",
                    "SB|E6|2", "SB|E7|3", "SB|E8|6", "SB|E9|9"},
            {"SBL|E1|1|5", "SBL|E2|2|2", "SBL|E3|1|3",
                    "SBL|E4|2|4", "SBL|E5|2|6", "SBL|E6|1|2"},
            {"SB|E1|1", "SB|E2|2", "SB|E3|3", "A|E2"},
            {"SB|E1|1", "SB|E2|8", "A|E2", "SB|E3|7", "SB|E4|2"}
    };

    // Pinned snapshot modes per case, in snapshot order (all in-order;
    // groupwin and on-delete assert no iterator).
    private static final String[][] CASE_SNAPSHOT_MODES = {
            {"ordered", "ordered", "ordered", "ordered", "ordered",
                    "ordered", "ordered", "ordered", "ordered"},
            {},
            {"ordered", "ordered"},
            {}
    };

    private static final int EXPECTED_STEPS = 59;
    private static final int EXPECTED_RECORDS = 43;

    private ViewExpressionWindowAgg568ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewExpressionWindowAgg568ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            Map<String, TraceWriter> writers = new HashMap<>();
            Map<String, EPStatement> statementsByName = new HashMap<>();
            List<String> pendingStatements = new ArrayList<>();
            List<String> pendingEpls = new ArrayList<>();
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
                if ("deploy".equals(operation)) {
                    pendingStatements.add(step.getString("statement", ""));
                    pendingEpls.add(step.getString("epl", ""));
                    continue;
                }
                // Every non-deploy step flushes the queued labels into one
                // module deploy, matching the Java executions' single
                // compileDeploy call.
                if (!pendingStatements.isEmpty()) {
                    deployModule(runtime, caseName, caseIndex, pendingStatements,
                            pendingEpls, statementsByName, writers, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                switch (operation) {
                    case "deployed": {
                        String label = step.getString("statement", "");
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        JsonObject record = new JsonObject()
                                .add("case", caseName)
                                .add("operation", "deployed")
                                .add("statement", label)
                                .add("sequence", writers.get(label).deployed())
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
                    case "snapshot": {
                        String label = step.getString("statement", "");
                        TraceWriter writer = writers.get(label);
                        if (writer == null) {
                            throw new IllegalStateException(
                                    "snapshot without a deployed statement " + label);
                        }
                        writer.snapshot();
                        break;
                    }
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(runtime, caseName, caseIndex, pendingStatements,
                        pendingEpls, statementsByName, writers, records);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Compiles and deploys the queued labels as ONE module joined with
     * ";\n" — the whitespace-normalized form of the execution's module
     * text — then registers each statement under its scenario label and
     * attaches the listener to the s0 statement exactly like the Java
     * executions' addListener("s0").
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     int caseIndex, List<String> statementNames,
                                     List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, TraceWriter> writers,
                                     JsonArray records) throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(moduleEpl.toString(),
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        EPStatement[] deployed = deployment.getStatements();
        if (deployed.length != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.length + " statements");
        }
        for (int index = 0; index < statementNames.size(); index++) {
            String label = statementNames.get(index);
            EPStatement statement = deployed[index];
            statementsByName.put(label, statement);
            TraceWriter writer = new TraceWriter(records, caseName, statement,
                    runtime, CASE_FIELDS[caseIndex]);
            writers.put(label, writer);
            if ("s0".equals(label)) {
                statement.addListener(writer);
            }
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
            SupportBean bean = new SupportBean(theString, intPrimitive);
            JsonValue longPrimitiveVal = payload.get("longPrimitive");
            if (longPrimitiveVal != null) {
                bean.setLongPrimitive(longNumber(longPrimitiveVal));
            }
            runtime.getEventService().sendEventBean(bean, "SupportBean");
            return;
        }
        if ("SupportBean_A".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportBean_A(payload.getString("id", null)), "SupportBean_A");
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
     * by the case's pinned ops — deploy steps carrying the byte-exact
     * statement text per label, deployed markers, sends with pinned
     * payloads, ordered snapshots and undeploy-all. Unknown step fields
     * are rejected.
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
                        String eventType = string(step, "eventType");
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        String actual;
                        if (expected.startsWith("SBL|")) {
                            if (!"SupportBean".equals(eventType)) {
                                throw new IllegalArgumentException("send step " + cursor
                                        + " is not pinned");
                            }
                            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
                            actual = "SBL|" + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive") + "|"
                                    + longNumber(payload.get("longPrimitive"));
                        } else if (expected.startsWith("SB|")) {
                            if (!"SupportBean".equals(eventType)) {
                                throw new IllegalArgumentException("send step " + cursor
                                        + " is not pinned");
                            }
                            requireFields(payload, "theString", "intPrimitive");
                            actual = "SB|" + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive");
                        } else {
                            if (!"SupportBean_A".equals(eventType)) {
                                throw new IllegalArgumentException("send step " + cursor
                                        + " is not pinned");
                            }
                            requireFields(payload, "id");
                            actual = "A|" + string(payload, "id");
                        }
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "snapshot":
                        requireFields(step, "op", "case", "statement", "mode");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_SNAPSHOT_MODES[caseIndex][snapshots++]
                                .equals(string(step, "mode"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
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
                    || snapshots != CASE_SNAPSHOT_MODES[caseIndex].length) {
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

        private long deployed() {
            return ++deployedSequence;
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
