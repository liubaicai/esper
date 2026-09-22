import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the view-systime-trio chain: three
 * executions from the time-view subdomain replayed as three cases, each
 * against its own fresh runtime.
 *
 * timebatch-refpoint replays ViewTimeBatchRefPoint (ViewTimeBatch ord 9):
 * time_batch(10 minutes, 10L) over SupportBean. The second time_batch
 * argument is an absolute epoch-ms reference point; boundaries sit at
 * refPoint + k*interval under the strictly-greater rule (a boundary exactly
 * at now is skipped) and the anchor latches on first-event arrival. The
 * bare SupportBean sent at 10ms flushes at 600010 and not at 600009.
 *
 * timebatch-uni-systime replays ViewTimeBatchWSystemTime (ord 0):
 * select * over SupportMarketDataBean(symbol='CSCO.O')#time_batch(2)
 * #uni(volume). The Java execution is wall-clock (Thread.sleep plus the
 * internal timer); the oracle disables the internal timer and replays each
 * sleep as an advance-time step at the cumulative instant
 * (1000/2500/3500/5000/7000). The #uni iterator always yields one stats row
 * (NaN zero-state before the first release) and each 2-second batch
 * boundary posts one listener update with new=[stats] and old=[prev stats]
 * at 925.0, 575.0 and 1200.0.
 *
 * timewin-weightedavg-systime replays ViewTimeWinWSystemTime (ord 0):
 * select * over SupportMarketDataBean(symbol='CSCO.O')#time(3.0)
 * #weighted_avg(price, volume, symbol, feed). Sleeps replay as advance-time
 * at cumulative instants 1500/3500/6000/7000. A types step pins the Java
 * assertStatement surface (property average is Double). Each send posts one
 * listener record and each expiry advance posts one (E1+E2 expire together
 * in a single update); weighted_avg params >=3 are passthrough props
 * evaluated on the last new event and retained on the final NaN row.
 *
 * Configuration follows the pinned regression schema: SupportMarketDataBean
 * is a map type {symbol string, price double, volume long, feed string}
 * (sends set all four fields, mirroring makeBean/sendEvent) and SupportBean
 * is mirrored locally as {theString string, intPrimitive int, doubleBoxed
 * Double} because the pinned beans live outside the oracle classpath. The
 * internal timer is disabled and the runtime clock initializes at epoch,
 * mirroring the Java sendTimer(0)/deploy ordering; every case still begins
 * with its own advance-time step.
 *
 * Records follow the standard protocol: one listener record per delivered
 * update with a per-case sequence counter starting at 1 and time rendered
 * from the current engine time; snapshot records carry sequence 0 and the
 * current engine time; the types record carries sequence 0 and the pinned
 * property-type surface. NaN renders as {"state":"nan"} (minimal-json
 * cannot encode raw NaN — ViewGroupMergeViewScenarioOracle precedent) and
 * null as {"state":"null"}.
 */
public final class ViewSystimeTrioScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "view-systime-trio";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeBatch.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeBatchWSystemTime.java";
    private static final String JAVA_SOURCE3 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewTimeWinWSystemTime.java";

    private static final String DESCRIPTION =
            "ViewTimeBatchRefPoint (ViewTimeBatch ord 9) plus the two "
                    + "wall-clock executions ViewTimeBatchWSystemTime and "
                    + "ViewTimeWinWSystemTime (three executions, the time-view "
                    + "system-time trio): timebatch-refpoint anchors "
                    + "time_batch(10 minutes, 10L) at the absolute epoch-ms "
                    + "reference point latched on first-event arrival, so the "
                    + "bare SupportBean flushes at 600010 and not one "
                    + "millisecond earlier; timebatch-uni-systime replays the "
                    + "Thread.sleep schedule of ViewTimeBatchWSystemTime as "
                    + "advance-time steps at cumulative instants "
                    + "1000/2500/3500/5000/7000 over #time_batch(2)#uni(volume), "
                    + "the iterator always yielding one stats row (NaN "
                    + "zero-state) and each 2-second batch boundary posting "
                    + "one listener update with new=[stats] and "
                    + "old=[prev stats] at 925.0, 575.0 and 1200.0; "
                    + "timewin-weightedavg-systime replays "
                    + "ViewTimeWinWSystemTime's sleeps as advance-time steps "
                    + "at cumulative instants 1500/3500/6000/7000 over "
                    + "#time(3.0)#weighted_avg(price, volume, symbol, feed), "
                    + "asserting property average is Double, posting one "
                    + "listener record per send and one per expiry advance "
                    + "(E1+E2 expire together in a single update) and ending "
                    + "on the NaN row that retains the last event's "
                    + "symbol/feed passthrough props. Each case deploys s0 "
                    + "with the verbatim Java EPL, advances the clock where "
                    + "Java sleeps or sends timers, sends events and "
                    + "snapshots the statement iterator where Java asserts "
                    + "it.";

    private static final String[] CASES = {
            "timebatch-refpoint", "timebatch-uni-systime",
            "timewin-weightedavg-systime"};
    private static final int[] ORDINALS = {9, 0, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7d3c38fa4d2477a0be87",
            "java-runtime-90902912cde58a38fafd",
            "java-runtime-486b77a63e1a3518f362"};
    private static final String[] EXECUTIONS = {
            "ViewTimeBatchRefPoint",
            "ViewTimeBatchWSystemTime",
            "ViewTimeWinWSystemTime"};
    private static final String[] STATIC_IDS = {
            "java-890952520f19d33fb6d9",
            "java-6020404113b2fcd36186",
            "java-949fe2671907c0c3e961"};
    private static final String[] OBSERVATIONS = {
            "listener; time_batch(10 minutes, 10L) anchors boundaries at "
                    + "refPoint + k*interval with the strictly-greater rule "
                    + "(a boundary exactly at now is skipped) and the anchor "
                    + "latches on first-event arrival; the bare SupportBean "
                    + "sent at 10ms flushes at 600010 and not at 600009",
            "listener+iterator; Java wall-clock sleeps replayed as "
                    + "advance-time at cumulative instants "
                    + "1000/2500/3500/5000/7000; the #uni(volume) iterator "
                    + "always yields one stats row (NaN zero-state before "
                    + "the first release) and each 2-second batch boundary "
                    + "posts one listener update new=[stats] "
                    + "old=[prev stats] at 925.0, 575.0 and 1200.0",
            "listener+iterator+types; Java wall-clock sleeps replayed as "
                    + "advance-time at cumulative instants "
                    + "1500/3500/6000/7000; property average is Double; "
                    + "each send and each expiry advance posts one listener "
                    + "record (E1+E2 expire together in a single update) "
                    + "with weighted averages 10.0, 10.5, 10.25, 10.375, "
                    + "10.333333333, 10.3, 10.2 and NaN; params >=3 are "
                    + "passthrough props evaluated on the last new event "
                    + "and retained on the NaN row"};

    private static final String EPL_REFPOINT =
            "@name('s0') select * from SupportBean#time_batch(10 minutes, 10L)";
    private static final String EPL_UNI =
            "@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time_batch(2)#uni(volume)";
    private static final String EPL_WAVG =
            "@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time(3.0)#weighted_avg(price, volume, symbol, feed)";

    private static final String[] CASE_EPLS = {EPL_REFPOINT, EPL_UNI, EPL_WAVG};

    // Pinned op sequences per case (after the case marker). The two
    // systime cases replay each Java Thread.sleep as an advance-time step
    // at the cumulative instant and each iterator/listener assertion as a
    // snapshot; timewin-weightedavg-systime's checkValue asserts iterator
    // plus listener so every send and expiry advance is followed by a
    // snapshot.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "advance-time", "send",
                    "advance-time", "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "snapshot", "send", "send",
                    "snapshot", "advance-time", "send", "send", "snapshot",
                    "advance-time", "snapshot", "send", "send", "send",
                    "snapshot", "advance-time", "send", "snapshot",
                    "advance-time", "snapshot", "send", "snapshot",
                    "advance-time", "snapshot", "undeploy-all"},
            {"advance-time", "deploy", "types", "send", "snapshot", "send",
                    "snapshot", "advance-time", "send", "snapshot", "send",
                    "snapshot", "advance-time", "snapshot", "send",
                    "snapshot", "advance-time", "snapshot", "advance-time",
                    "snapshot", "undeploy-all"}
    };

    // Pinned advance-time targets per case, in advance order.
    private static final String[][] CASE_TIMES = {
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:00.010Z",
                    "1970-01-01T00:10:00.009Z", "1970-01-01T00:10:00.010Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.000Z",
                    "1970-01-01T00:00:02.500Z", "1970-01-01T00:00:03.500Z",
                    "1970-01-01T00:00:05.000Z", "1970-01-01T00:00:07.000Z"},
            {"1970-01-01T00:00:00.000Z", "1970-01-01T00:00:01.500Z",
                    "1970-01-01T00:00:03.500Z", "1970-01-01T00:00:06.000Z",
                    "1970-01-01T00:00:07.000Z"}
    };

    // Pinned send payloads per case, in send order, encoded
    // "eventType|field|field|..." with <null> for a JSON-null field.
    // SupportBean sends encode theString|intPrimitive;
    // SupportMarketDataBean sends encode symbol|price|volume|feed with the
    // price/volume JSON literal verbatim.
    private static final String[][] CASE_SENDS = {
            {"SupportBean|<null>|0"},
            {"SupportMarketDataBean|CSCO.O|0|500|",
                    "SupportMarketDataBean|CSCO.O|0|1000|",
                    "SupportMarketDataBean|CSCO.O|0|1000|",
                    "SupportMarketDataBean|CSCO.O|0|1200|",
                    "SupportMarketDataBean|CSCO.O|0|500|",
                    "SupportMarketDataBean|CSCO.O|0|600|",
                    "SupportMarketDataBean|CSCO.O|0|1000|",
                    "SupportMarketDataBean|CSCO.O|0|200|",
                    "SupportMarketDataBean|CSCO.O|0|1200|"},
            {"SupportMarketDataBean|CSCO.O|10|500|feed1",
                    "SupportMarketDataBean|CSCO.O|11|500|feed1",
                    "SupportMarketDataBean|CSCO.O|10|1000|feed1",
                    "SupportMarketDataBean|CSCO.O|10.5|2000|feed1",
                    "SupportMarketDataBean|CSCO.O|10.2|1000|feed1"}
    };

    // Pinned types surfaces per case: the Java-asserted property
    // name-to-type pairs (env.assertStatement semantics). Only
    // timewin-weightedavg-systime asserts a property type in Java.
    private static final Map<String, String[][]> TYPES_ASSERTIONS = Map.of(
            "timewin-weightedavg-systime", new String[][]{{"average", "Double"}});

    private static final int EXPECTED_STEPS = 57;
    private static final int EXPECTED_RECORDS = 30;

    private ViewSystimeTrioScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewSystimeTrioScenarioOracle <scenario.json>");
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        configuration.getCommon().addEventType("SupportBean", LocalSupportBean.class);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
            int deployCount = 0;
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
                    String epl = step.getString("epl", "");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(
                                    SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                    deployCount++;
                    writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                    writer.statement.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    writer = null;
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("advance-time".equals(operation)) {
                    runtime.getEventService().advanceTime(
                            Instant.parse(step.getString("at", "")).toEpochMilli());
                } else if ("snapshot".equals(operation)) {
                    if (writer == null) {
                        throw new IllegalStateException("snapshot without a deployed statement");
                    }
                    writer.snapshot();
                } else if ("types".equals(operation)) {
                    if (writer == null) {
                        throw new IllegalStateException("types without a deployed statement");
                    }
                    writer.types();
                } else {
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
        if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            JsonValue priceVal = payload.get("price");
            event.put("price", priceVal instanceof JsonNumber ? ((JsonNumber) priceVal).asDouble() : 0.0d);
            JsonValue volumeVal = payload.get("volume");
            event.put("volume", volumeVal instanceof JsonNumber ? ((JsonNumber) volumeVal).asLong() : 0L);
            event.put("feed", payload.getString("feed", null));
            runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
            return;
        }
        if ("SupportBean".equals(eventType)) {
            LocalSupportBean event = new LocalSupportBean();
            // theString is set unconditionally from the payload, including
            // explicit JSON null for the bare bean of the refpoint case
            JsonValue theStringVal = payload.get("theString");
            event.setTheString(theStringVal instanceof JsonString ? ((JsonString) theStringVal).asString() : null);
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            if (intPrimitiveVal instanceof JsonNumber) {
                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
            }
            JsonValue doubleBoxedVal = payload.get("doubleBoxed");
            if (doubleBoxedVal instanceof JsonNumber) {
                event.setDoubleBoxed(((JsonNumber) doubleBoxedVal).asDouble());
            }
            runtime.getEventService().sendEventBean(event, "SupportBean");
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
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
                "javaSource2", "javaSource3", "javaRuntimes", "javaNames", "javaStaticIds",
                "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))
                || !JAVA_SOURCE3.equals(string(scenario, "javaSource3"))) {
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
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned ops — deploy s0 with the verbatim EPL, advance-time
     * steps at the pinned clock targets (the wall-clock sleeps replayed at
     * cumulative instants), the assertion sends with pinned payloads,
     * iterator snapshots, the weighted_avg types assertion and
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
                        deploys++;
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String eventType = string(step, "eventType");
                        String actual;
                        if ("SupportBean".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "theString", "intPrimitive");
                            String theString = nullableString(payload, "theString");
                            actual = eventType + "|"
                                    + (theString == null ? "<null>" : theString) + "|"
                                    + integer(payload, "intPrimitive");
                        } else if ("SupportMarketDataBean".equals(eventType)) {
                            JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                            requireFields(payload, "symbol", "price", "volume", "feed");
                            String feed = nullableString(payload, "feed");
                            actual = eventType + "|" + string(payload, "symbol") + "|"
                                    + numberLiteral(payload.get("price")) + "|"
                                    + numberLiteral(payload.get("volume")) + "|"
                                    + (feed == null ? "<null>" : feed);
                        } else {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " is not pinned");
                        }
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_TIMES[caseIndex][advances++].equals(string(step, "at"))) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " is not pinned");
                        }
                        break;
                    case "snapshot":
                    case "types":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException(operation + " step " + cursor + " is not pinned");
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
                    || deploys != 1
                    || advances != CASE_TIMES[caseIndex].length) {
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

    private static String numberLiteral(JsonValue value) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("value must be a JSON number");
        }
        return value.toString();
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
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            append("listener", ++sequence, newEvents, oldEvents);
        }

        private void snapshot() {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append("snapshot", 0, events.toArray(new EventBean[0]), null);
        }

        /**
         * Java-asserted event-type surface for the statement: the pinned
         * property name-to-type pairs. The actual event type is verified
         * against the pinned surface before recording so a drift fails the
         * oracle (env.assertStatement semantics).
         */
        private void types() {
            String[][] properties = TYPES_ASSERTIONS.get(caseName);
            if (properties == null) {
                throw new IllegalStateException("types step for case " + caseName
                        + " pins no asserted surface");
            }
            EventType eventType = statement.getEventType();
            JsonObject pinned = new JsonObject();
            for (String[] pair : properties) {
                Class<?> propertyType = eventType.getPropertyType(pair[0]);
                String actual = propertyType == null ? "null" : propertyType.getSimpleName();
                if (!pair[1].equals(actual)) {
                    throw new IllegalStateException("property type drift for " + caseName + "."
                            + pair[0] + ": expected " + pair[1] + " got " + actual);
                }
                pinned.add(pair[0], pair[1]);
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "types")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("value", new JsonObject().add("properties", pinned));
            records.add(record);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
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
                output.add(row(event));
            }
            return output;
        }

        private JsonObject row(EventBean event) {
            JsonObject fields = new JsonObject();
            for (String name : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Double && ((Double) value).isNaN()
                    || value instanceof Float && ((Float) value).isNaN()) {
                // NaN marker: {"state":"nan"}, symmetric with the null
                // marker; plain JSON that jq and every parser accept.
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
                return row((EventBean) value);
            }
            if (value instanceof Object[]) {
                JsonArray array = new JsonArray();
                for (Object item : (Object[]) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Map<?, ?>) {
                // map projections surface as bare JSON objects with
                // TreeSet-sorted String.valueOf keys
                Map<?, ?> mapValue = (Map<?, ?>) value;
                TreeSet<String> keys = new TreeSet<>();
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

    /**
     * Local mirror of the pinned SupportBean regression bean members in
     * use (com.espertech.esper.common.internal.support.SupportBean): the
     * no-arg constructor leaves theString null, intPrimitive 0 and
     * doubleBoxed null, matching the bare bean the refpoint case sends.
     */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private Double doubleBoxed;

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }

        public Double getDoubleBoxed() {
            return doubleBoxed;
        }

        public void setDoubleBoxed(Double doubleBoxed) {
            this.doubleBoxed = doubleBoxed;
        }
    }
}
