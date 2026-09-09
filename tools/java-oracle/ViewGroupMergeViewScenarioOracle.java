import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;
import com.espertech.esper.runtime.internal.schedulesvcimpl.ScheduleVisitor;
import com.espertech.esper.runtime.internal.schedulesvcimpl.SchedulingServiceSPI;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ViewGroup grouped-window scenarios (merge-view family).
 *
 * Covers fourteen pinned executions of ViewGroup, each replayed as its own
 * case against a fresh runtime with the pinned module text transcribed
 * verbatim (only the byte-exact EPL text; no other annotations are added):
 *
 * merge-view-union-aggregate replays ViewGroupObjectArrayEvent (execution
 * ordinal 0, runtime java-runtime-a3b6bef89e22a122cc7a):
 * select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2),
 * sending A/10, B/11, A/12, A/13 and pinning the per-group Long sum
 * evolution 10, 21, 33, 36 through istream-only listener records.
 *
 * length-win-groups replays ViewGroupLengthWin (ordinal 14, runtime
 * java-runtime-639bc9b69621f3b6a417): select irstream theString as c0,
 * intPrimitive as c1 from SupportBean#groupwin(theString)#length(3),
 * sending E1/1, E2/20, E1/2, E2/21, E2/22, E1/3 (istream-only while the
 * per-group windows are not full), then E2/23 evicting E2/20 and E1/-1
 * evicting E1/1 as irstream pairs. The suite's iterator assertions here
 * use assertPropsPerRowIteratorAnyOrder, i.e. Java itself leaves
 * cross-group iteration order unpinned, so no snapshot steps are recorded
 * for this case.
 *
 * stats-four-views replays ViewGroupStats (ordinal 1, runtime
 * java-runtime-03ed11fd1e5c3a3d11c2) as a multi-module case: the suite
 * makes four separate compileDeploy calls, replayed here as four modules
 * deployed in order before any send — @name('priceLast3Stats') and
 * @name('volumeLast3Stats') over #groupwin(symbol)#length(3)#uni(...),
 * @name('priceAllStats') and @name('volumeAllStats') over
 * #groupwin(symbol)#uni(...), all select * with "order by symbol asc"
 * and the @name(...) annotation directly adjacent to the select keyword
 * as in the suite concatenation. The 14 sends use the suite helper's
 * SupportMarketDataBean(symbol, price, volume, "") shape. The oracle
 * attaches one listener per named statement (sequence numbering restarts
 * per statement) and records every delivered batch; because each send
 * updates exactly one symbol group and #uni posts istream-only, every
 * invocation carries exactly one new row, so each record IS the value the
 * suite pins with assertLastNewRow (its listener.getLastNewData() last
 * event) at sends 5, 7, 9, and 14, and for the other sends the record is
 * the same single-row observable the suite skips asserting. After send 14
 * two snapshot steps pin the suite's ordered assertPropsPerRowIterator
 * state of priceAllStats and priceLast3Stats (rows {symbol, average}
 * among the full select-* properties).
 *
 * correl-groups replays ViewGroupCorrel (ordinal 4, runtime
 * java-runtime-74e476ad16bf62d4a9e0): select * from
 * SupportMarketDataBean#groupwin(symbol)#length(1000000)#correl(price,
 * volume, feed), sending (ABC,10.0,1000,f1), (DEF,1.0,2,f2),
 * (DEF,2.0,4,f3), (ABC,20.0,2000,f4). Each record carries the full
 * select-* row; the Go-pinned surface is symbol/correlation/feed among
 * all fields, with correlation NaN for the two single-datapoint groups
 * rendered through the NaN marker convention.
 *
 * linest-groups replays ViewGroupLinest (ordinal 5, runtime
 * java-runtime-942d359f7bb1302684bb): select * from
 * SupportMarketDataBean#groupwin(symbol)#length(1000000)#linest(price,
 * volume, feed), sending (ABC,10.0,50000,f1), (DEF,1.0,1,f2),
 * (DEF,2.0,2,f3), (ABC,11.0,50100,f4). Full select-* rows again; the
 * Go-pinned surface is symbol/slope/YIntercept/feed among all fields,
 * with the two single-datapoint slope/YIntercept NaNs marked.
 *
 * multi-property-uni replays ViewGroupMultiProperty (ordinal 6, runtime
 * java-runtime-47877af7a1d114850723): select irstream datapoints as size,
 * symbol, feed, volume from SupportMarketDataBean#groupwin(symbol, feed,
 * volume)#uni(price) order by symbol, feed, volume. The suite helper
 * sendEvent(symbol, feed, volume) constructs SupportMarketDataBean with
 * price 0; the replayed sends are (GE,INFO,1) then (GE,INFO,1),
 * (GE,INFO,2), (GE,INFO,1) — where the suite calls listenerReset and
 * asserts nothing, the oracle records every delivered batch as the
 * established assertPropsNew convention — then (GE,REU,99) and
 * (MSFT,INFO,100), each delivering one irstream pair (new current
 * datapoints, old prior datapoints). The suite's final ordered
 * assertPropsPerRowIterator is pinned with a snapshot step.
 *
 * Five further cases are virtual-time driven and replay the engine clock
 * through advance-time steps: each suite env.advanceTime(millis) call maps
 * to one {"op":"advance-time","at":"<RFC3339 UTC>"} step (millisecond
 * precision, always three fractional digits) and the oracle advances the
 * runtime clock to the parsed epoch millis. The suite's pre-deploy
 * advanceTime(1000) replays as the case's leading advance-time step
 * executed after deployment, observably equivalent because grouped time
 * windows arm on the first event, so no timer is scheduled before it.
 *
 * time-batch-groups replays ViewGroupTimeBatch (ordinal 10, runtime
 * java-runtime-7bd36b6fe5567b066794): select irstream * from
 * SupportMarketDataBean#groupwin(symbol)#time_batch(10 sec) (the suite
 * keeps its double space after from). Sends (S1,10) at 1000, (S1,20) at
 * 5000 and (S2,30) at 10000 are listener-silenced; 10999 stays silent;
 * 11000 flushes the S1 group as IRPair new [{10},{20}], 20000 flushes S2
 * as new [{30}], and the re-armed empty S1 batch still posts one update
 * call at 21000 as old [{10},{20}].
 *
 * time-accum-groups replays ViewGroupTimeAccum (ordinal 11, runtime
 * java-runtime-842bde62118b9b8cae2d): the same double-spaced module over
 * #time_accum(10 sec). The three sends deliver istream rows immediately
 * (new 10, 20, 30 as the suite's assertPrice pins), then the delayed
 * exchange releases old [{10},{20}] at 15000 and old [{30}] at 20000.
 *
 * time-order-groups replays ViewGroupTimeOrder (ordinal 12, runtime
 * java-runtime-806120fdd2130ab1f275): select irstream * from
 * SupportBeanTimestamp#groupwin(groupId)#time_order(timestamp, 10 sec)
 * (single space after from in the suite). The four sends (E1,G1,3000),
 * (E2,G2,2000), (E3,G2,3000), (E4,G1,2500) at 1000/2000/3000/4000 each
 * deliver one istream row pinned by assertId; 11999 stays silent, 12000
 * releases old [{E2}], 12499 stays silent, 12500 releases old [{E4}]
 * (E1 and E3 expire at 13000 unobserved).
 *
 * time-length-batch-groups replays ViewGroupTimeLengthBatch (ordinal 13,
 * runtime java-runtime-737a5f1ffd4c6a6c8924): the double-spaced module
 * over #time_length_batch(10 sec, 100) with the identical timeline and
 * deliveries as time-batch-groups.
 *
 * time-win-groups replays ViewGroupTimeWin (ordinal 16, runtime
 * java-runtime-68ef8076bc96595cbfd0): the double-spaced module over
 * #time(10 sec). The three sends deliver istream rows immediately, 10999
 * stays silent, and each group expires exactly: old [{10}] at 11000, old
 * [{20}] at 15000, old [{30}] at 20000.
 *
 * Three further cases replay the reclaim_group_aged/reclaim_group_freq
 * surface and observe it through recorded counts instead of listener rows:
 *
 * reclaim-time-window replays ViewGroupReclaimTimeWindow (ordinal 2,
 * runtime java-runtime-88d7b731431c59d99f3a): select longPrimitive,
 * count(*) from SupportBean#groupwin(theString)#time(3000000) under
 * @Hint('reclaim_group_aged=30,reclaim_group_freq=5'). The ten sends
 * "0".."9" at t=0 arm one time window per group, so a schedule-count step
 * observes 10 scheduled handles; after the advance to 1000000 the aged
 * groups are reclaimed, the E1 send arms exactly one group, and a second
 * schedule-count step observes 1. An explicit undeploy-all step mirrors
 * env.undeployAll() (it removes every deployment and its schedule handles
 * and records nothing itself), and the trailing schedule-count-overall
 * step observes 0 for the whole runtime.
 *
 * reclaim-aged-hint replays ViewGroupReclaimAgedHint (ordinal 3, runtime
 * java-runtime-afc05b1a18402bb17f56): select * from
 * SupportBean#groupwin(theString)#keepall under
 * @Hint('reclaim_group_aged=5,reclaim_group_freq=1'). For each of ten
 * one-second slots the scenario advances to slot*1000+1 and sends one
 * thousand "E<slot>" events; the reclaim passes keep only the groups
 * within the aged horizon, so an iterator-count step observes 6000
 * retained rows (the suite asserts <=6000 there and 6001 after one more
 * E0 send, which the final iterator-count step observes exactly). The
 * iterator-count op counts the statement's iteration without
 * materializing rows.
 *
 * reclaim-flip-time replays the flipTime=5000 instantiation of
 * ViewGroupReclaimWithFlipTime (ordinal 9, runtime
 * java-runtime-33b5cb01913d5d23292b): select * from
 * SupportBean#groupwin(theString)#keepall under
 * @Hint('reclaim_group_aged=1,reclaim_group_freq=5'). Sends E1 at t=0 and
 * E2 at t=4999 observe keepall counts 1 and 2; the advance to t=5000
 * crosses the reclaim flip so group E1 ages out before the E3 send, whose
 * iterator-count step observes 2 again.
 *
 * The three reclaim cases record no listener deliveries: the suite
 * attaches a listener but pins only schedule and iterator counts for
 * these executions, so attaching no recording listener leaves the
 * observed counts unchanged (listeners do not affect view state,
 * scheduling, or iteration) and keeps the trace to the pinned surface.
 * The suite's pre-deploy advanceTime(0) replays through each case's
 * existing initialization advance to epoch 0, in the same before-deploy
 * order, so the reclaim cases carry no leading advance-time step.
 *
 * New count-record protocol (no time, no sequence, observed values
 * only): step {"op":"schedule-count","statement":S} emits
 * {"case":C,"operation":"schedule-count","statement":S,"count":N} with N
 * counted from the scheduling service; step
 * {"op":"schedule-count-overall"} emits
 * {"case":C,"operation":"schedule-count-overall","count":N} counted over
 * the whole runtime; step {"op":"iterator-count","statement":S} emits
 * {"case":C,"operation":"iterator-count","statement":S,"count":N}. The
 * declared count in each step is informational; the record always
 * carries the observed value.
 *
 * Suite milestones are harness ordering markers with no observable engine
 * output and record nothing. The OAEventStringInt event type mirrors the
 * pinned regression-run registration as an object-array type with property
 * names {p1, p2} and types {String, int}, sent through
 * sendEventObjectArray exactly like env.sendEventObjectArray.
 * SupportMarketDataBean is mirrored as a local bean class because the
 * fixed run script classpath excludes regression-lib; the mirror keeps the
 * pinned four-arg constructor (symbol, price, volume, feed), field types
 * (String, double, Long, String) and getters byte-equivalent, and the
 * unused id property does not participate in these scenarios. The ord-12
 * SupportBeanTimestamp is regression-lib as well and is mirrored locally
 * with the pinned three-arg constructor (id, groupId, timestamp), field
 * types (String, long, String) and getters. SupportBean
 * is the common-module class already on the classpath and is constructed
 * with the suite's two-arg (theString, intPrimitive) constructor.
 *
 * NaN convention: no prior oracle renders NaN, so normalize renders
 * Double.NaN and Float.NaN as the JSON object {"state":"nan"}, by
 * symmetry with the existing null marker {"state":"null"}; plain JSON,
 * accepted by jq and every JSON parser.
 *
 * Cross-statement record order: the Java runtime dispatches one event to
 * the stats-four-views statements in reverse deployment order
 * (volumeAllStats, priceAllStats, volumeLast3Stats, priceLast3Stats),
 * while the suite only ever asserts per-listener and never pins
 * cross-statement order. Listener records are therefore buffered per
 * send step and flushed in deployment order (the LinkedHashMap order)
 * at the send boundary, so the trace order is canonical; per-statement
 * sequences and all row values are unchanged. Snapshot steps flush
 * immediately as before.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch (not per row), sequence numbering per statement from 1, time
 * rendered from the current engine time, and new/old row arrays rendered
 * with the scalar normalization rules; a batch is skipped only when the
 * engine delivers neither new nor old rows. Snapshot records carry no
 * time or sequence: step {op:"snapshot", statement:"s0"} iterates the
 * named deployed statement and emits its current window contents.
 */
public class ViewGroupMergeViewScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewGroupMergeViewScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently against its own runtime,
        // matching one runtime ID per execution.
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("scenario", scenario.getString("description", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
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
        // Mirrors the pinned regression-run registration of OAEventStringInt
        // as an object-array event type (TestSuiteView): names {p1, p2},
        // types {String, int}.
        config.getCommon().addEventType("OAEventStringInt",
            new String[] {"p1", "p2"}, new Object[] {String.class, int.class});
        // Local mirror class: regression-lib is outside the oracle classpath.
        config.getCommon().addEventType("SupportMarketDataBean", LocalSupportMarketDataBean.class);
        config.getCommon().addEventType("SupportBean", SupportBean.class);
        // Regression-lib mirror for the ord-12 time-order case.
        config.getCommon().addEventType("SupportBeanTimestamp", LocalSupportBeanTimestamp.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewGroupMergeViewScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            // Mirrors the suite's sequential compileDeploy calls: multi-module
            // cases (stats-four-views) deploy one module per suite call in
            // order; single-module cases deploy exactly the pinned module.
            Map<String, EPStatement> statementsByName = new LinkedHashMap<>();
            for (String module : eplModulesFor(caseName)) {
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(config));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                for (EPStatement statement : deployment.getStatements()) {
                    statementsByName.put(statement.getName(), statement);
                }
            }
            if (!statementsByName.containsKey("s0") && !statementsByName.containsKey("priceLast3Stats")) {
                throw new IllegalStateException("no observable statement deployed for case " + caseName);
            }
            // One listener per named statement; the suite attaches listeners
            // to every statement it deploys before sending events. Per-
            // statement sequence counters keep each statement's pin unambiguous.
            // Deliveries buffer per statement and flush at the send boundary
            // so records appear in canonical deployment order. The reclaim
            // cases pin only schedule and iterator counts, so no recording
            // listener is attached there (listener attachment does not affect
            // view state, scheduling, or iteration).
            Map<String, List<JsonObject>> pending = new LinkedHashMap<>();
            if (!COUNT_ONLY_CASES.contains(caseName)) {
                attachRecordListeners(statementsByName, pending, caseName);
            }

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
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                    // Flush this send's buffered deliveries in deployment
                    // order (the Java runtime dispatches to the four
                    // stats-four-views statements in reverse deployment
                    // order; the suite only asserts per-listener, so
                    // deployment order is the canonical record order).
                    // Single-statement cases are unaffected: one buffer,
                    // one flush, sequences unchanged.
                    flushPending(statementsByName, pending, records);
                    continue;
                }
                if ("advance-time".equals(op)) {
                    // Mirrors env.advanceTime(millis): the pinned at value is
                    // the engine clock in RFC3339 UTC with millisecond
                    // precision. Time-driven deliveries flush at this
                    // boundary in the same canonical deployment order as
                    // send-driven ones.
                    runtime.getEventService().advanceTime(
                        java.time.Instant.parse(step.getString("at", "")).toEpochMilli());
                    flushPending(statementsByName, pending, records);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPStatement snapshotStatement = statementsByName.get(statementName);
                    if (snapshotStatement == null) {
                        throw new IllegalStateException(
                            "snapshot statement " + statementName + " not deployed for case " + caseName);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statementName);
                    record.add("new", rows(snapshotStatement.iterator()));
                    records.add(record);
                    continue;
                }
                if ("schedule-count".equals(op)) {
                    // Observes the suite's SupportScheduleHelper.scheduleCount:
                    // schedule handles whose statement id matches the named
                    // statement. Records the observed count (no time, no
                    // sequence); the step's declared count is informational.
                    String statementName = step.getString("statement", "s0");
                    EPStatement countStatement = requireStatement(statementsByName, statementName, caseName);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "schedule-count");
                    record.add("statement", statementName);
                    record.add("count", scheduleCountForStatement(countStatement));
                    records.add(record);
                    continue;
                }
                if ("schedule-count-overall".equals(op)) {
                    // Observes the suite's
                    // SupportScheduleHelper.scheduleCountOverall over the
                    // whole runtime (used after undeploy-all).
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "schedule-count-overall");
                    record.add("count", scheduleCountOverall(runtime));
                    records.add(record);
                    continue;
                }
                if ("iterator-count".equals(op)) {
                    // Observes the suite's
                    // EPAssertionUtil.iteratorCount(statement.iterator()):
                    // counts the current iteration without materializing rows.
                    String statementName = step.getString("statement", "s0");
                    EPStatement countStatement = requireStatement(statementsByName, statementName, caseName);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "iterator-count");
                    record.add("statement", statementName);
                    record.add("count", iteratorCount(countStatement));
                    records.add(record);
                    continue;
                }
                if ("undeploy-all".equals(op)) {
                    // Mirrors env.undeployAll(): removes every deployment and
                    // its schedule handles; records nothing itself.
                    runtime.getDeploymentService().undeployAll();
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /** Appends buffered per-send records in deployment order, draining them. */
    private static void flushPending(Map<String, EPStatement> statementsByName,
        Map<String, List<JsonObject>> pending, List<JsonObject> records) {
        for (String name : statementsByName.keySet()) {
            List<JsonObject> batch = pending.get(name);
            if (batch != null && !batch.isEmpty()) {
                records.addAll(batch);
                batch.clear();
            }
        }
    }

    /** Cases that observe schedule and iterator counts only, no listener rows. */
    private static final java.util.Set<String> COUNT_ONLY_CASES = java.util.Set.of(
        "reclaim-time-window", "reclaim-aged-hint", "reclaim-flip-time");

    /** Attaches one recording listener per deployed statement (non-count cases). */
    private static void attachRecordListeners(Map<String, EPStatement> statementsByName,
        Map<String, List<JsonObject>> pending, String caseName) {
        for (EPStatement statement : statementsByName.values()) {
            int[] seq = new int[] {0};
            statement.addListener((newData, oldData, stmt, rt) -> {
                boolean hasNew = newData != null && newData.length > 0;
                boolean hasOld = oldData != null && oldData.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", stmt.getName());
                record.add("sequence", seq[0]);
                record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                if (hasNew) {
                    record.add("new", rows(newData));
                }
                if (hasOld) {
                    record.add("old", rows(oldData));
                }
                pending.computeIfAbsent(stmt.getName(), key -> new ArrayList<>()).add(record);
            });
        }
    }

    /** Resolves a deployed statement by name or fails the case. */
    private static EPStatement requireStatement(Map<String, EPStatement> statementsByName,
        String statementName, String caseName) {
        EPStatement statement = statementsByName.get(statementName);
        if (statement == null) {
            throw new IllegalStateException(
                "statement " + statementName + " not deployed for case " + caseName);
        }
        return statement;
    }

    /**
     * Mirrors the suite's
     * SupportScheduleHelper.scheduleCount(statement): casts through the
     * statement SPI to the scheduling service and counts schedule handles
     * whose statement id matches. Reimplemented locally because the fixed run
     * script classpath excludes regression-lib; the visited SPI and the
     * matching predicate are identical.
     */
    private static int scheduleCountForStatement(EPStatement statement) {
        EPStatementSPI spi = (EPStatementSPI) statement;
        SchedulingServiceSPI schedulingServiceSPI =
            (SchedulingServiceSPI) spi.getStatementContext().getSchedulingService();
        int statementId = spi.getStatementId();
        int[] count = new int[] {0};
        ScheduleVisitor visitor = visit -> {
            if (visit.getStatementId() == statementId) {
                count[0]++;
            }
        };
        schedulingServiceSPI.visitSchedules(visitor);
        return count[0];
    }

    /**
     * Mirrors the suite's SupportScheduleHelper.scheduleCountOverall(runtime):
     * counts every schedule handle across the runtime through the services
     * context scheduling service.
     */
    private static int scheduleCountOverall(EPRuntime runtime) {
        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
        int[] count = new int[] {0};
        ScheduleVisitor visitor = visit -> count[0]++;
        spi.getServicesContext().getSchedulingService().visitSchedules(visitor);
        return count[0];
    }

    /**
     * Mirrors the suite's EPAssertionUtil.iteratorCount: counts the
     * statement's current iteration without materializing rows.
     */
    private static int iteratorCount(EPStatement statement) {
        int count = 0;
        Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            iterator.next();
            count++;
        }
        return count;
    }

    /** Verbatim transcriptions of the pinned ViewGroup modules. */
    private static List<String> eplModulesFor(String caseName) {
        List<String> modules = new ArrayList<>();
        switch (caseName) {
            case "merge-view-union-aggregate" ->
                modules.add("@name('s0') select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2)");
            case "length-win-groups" ->
                modules.add("@Name('s0') select irstream theString as c0,intPrimitive as c1 from SupportBean#groupwin(theString)#length(3)");
            case "stats-four-views" -> {
                // Four separate compileDeploy calls in the suite; the filter
                // variable concatenates directly after @name('...').
                String filter = "select * from SupportMarketDataBean";
                modules.add("@name('priceLast3Stats')" + filter + "#groupwin(symbol)#length(3)#uni(price) order by symbol asc");
                modules.add("@name('volumeLast3Stats')" + filter + "#groupwin(symbol)#length(3)#uni(volume) order by symbol asc");
                modules.add("@name('priceAllStats')" + filter + "#groupwin(symbol)#uni(price) order by symbol asc");
                modules.add("@name('volumeAllStats')" + filter + "#groupwin(symbol)#uni(volume) order by symbol asc");
            }
            case "correl-groups" ->
                modules.add("@name('s0') select * from SupportMarketDataBean#groupwin(symbol)#length(1000000)#correl(price, volume, feed)");
            case "linest-groups" ->
                modules.add("@name('s0') select * from SupportMarketDataBean#groupwin(symbol)#length(1000000)#linest(price, volume, feed)");
            case "multi-property-uni" ->
                modules.add("@name('s0') select irstream datapoints as size, symbol, feed, volume " +
                    "from SupportMarketDataBean#groupwin(symbol, feed, volume)#uni(price) order by symbol, feed, volume");
            // Virtual-time grouped windows: the suite transcriptions keep
            // the source's double space after "from" for ords 10/11/13/16
            // and the single space for ord 12.
            case "time-batch-groups" ->
                modules.add("@name('s0') select irstream * from  SupportMarketDataBean#groupwin(symbol)#time_batch(10 sec)");
            case "time-accum-groups" ->
                modules.add("@name('s0') select irstream * from  SupportMarketDataBean#groupwin(symbol)#time_accum(10 sec)");
            case "time-order-groups" ->
                modules.add("@name('s0') select irstream * from SupportBeanTimestamp#groupwin(groupId)#time_order(timestamp, 10 sec)");
            case "time-length-batch-groups" ->
                modules.add("@name('s0') select irstream * from  SupportMarketDataBean#groupwin(symbol)#time_length_batch(10 sec, 100)");
            case "time-win-groups" ->
                modules.add("@name('s0') select irstream * from  SupportMarketDataBean#groupwin(symbol)#time(10 sec)");
            // Reclaim-group executions: byte-exact transcriptions of the
            // suite's concatenated EPL text (hint annotation adjacent to
            // @name, single space before select).
            case "reclaim-time-window" ->
                modules.add("@name('s0') @Hint('reclaim_group_aged=30,reclaim_group_freq=5') " +
                    "select longPrimitive, count(*) from SupportBean#groupwin(theString)#time(3000000)");
            case "reclaim-aged-hint" ->
                modules.add("@name('s0') @Hint('reclaim_group_aged=5,reclaim_group_freq=1') " +
                    "select * from SupportBean#groupwin(theString)#keepall");
            case "reclaim-flip-time" ->
                modules.add("@name('s0') @Hint('reclaim_group_aged=1,reclaim_group_freq=5') select * from SupportBean#groupwin(theString)#keepall");
            default -> throw new IllegalStateException("unknown case: " + caseName);
        }
        return modules;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("OAEventStringInt".equals(eventType)) {
            // Mirrors env.sendEventObjectArray(new Object[]{p1, p2}, "OAEventStringInt").
            runtime.getEventService().sendEventObjectArray(
                new Object[] {payload.getString("p1", null), payload.getInt("p2", 0)}, eventType);
            return;
        }
        if ("SupportMarketDataBean".equals(eventType)) {
            // Ord-1 sends mirror the suite helper sendEvent(env, symbol,
            // price, volume) with feed ""; ord-4/5 sends mirror the suite's
            // direct four-arg construction with feed f1..f4; ord-6 sends
            // mirror sendEvent(env, symbol, feed, volume) with price 0.
            JsonValue priceVal = payload.get("price");
            double price = priceVal instanceof JsonNumber ? ((JsonNumber) priceVal).asDouble() : 0.0d;
            JsonValue volumeVal = payload.get("volume");
            Long volume = volumeVal instanceof JsonNumber ? ((JsonNumber) volumeVal).asLong() : null;
            runtime.getEventService().sendEventBean(
                new LocalSupportMarketDataBean(payload.getString("symbol", null), price, volume,
                    payload.getString("feed", null)),
                eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            // Mirrors the suite helper sendSupportBean(env, theString, intPrimitive).
            runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
                eventType);
            return;
        }
        if ("SupportBeanTimestamp".equals(eventType)) {
            // Mirrors sendEventTS(env, id, groupId, timestamp): the suite's
            // three-arg SupportBeanTimestamp constructor.
            runtime.getEventService().sendEventBean(
                new LocalSupportBeanTimestamp(payload.getString("id", null),
                    payload.getString("groupId", null), payload.getLong("timestamp", 0)),
                eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonArray rows(Iterator<EventBean> iterator) {
        JsonArray array = new JsonArray();
        while (iterator.hasNext()) {
            array.add(row(iterator.next()));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        item.add("fields", fields);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Double && ((Double) value).isNaN()
            || value instanceof Float && ((Float) value).isNaN()) {
            // NaN marker: {"state":"nan"}, symmetric with the null marker;
            // plain JSON that jq and every parser accept.
            JsonObject nanObj = new JsonObject();
            nanObj.add("state", "nan");
            return nanObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Local mirror of the pinned SupportMarketDataBean regression bean
     * (com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean),
     * reduced to the four members the ViewGroup scenarios use. Constructor
     * parameter order, field types, and getters match the pinned bean; the
     * unused id property does not participate in these scenarios.
     */
    public static class LocalSupportMarketDataBean {
        private final String symbol;
        private final double price;
        private final Long volume;
        private final String feed;

        public LocalSupportMarketDataBean(String symbol, double price, Long volume, String feed) {
            this.symbol = symbol;
            this.price = price;
            this.volume = volume;
            this.feed = feed;
        }

        public String getSymbol() {
            return symbol;
        }

        public double getPrice() {
            return price;
        }

        public Long getVolume() {
            return volume;
        }

        public String getFeed() {
            return feed;
        }
    }

    /**
     * Local mirror of the pinned SupportBeanTimestamp regression bean
     * (com.espertech.esper.regressionlib.support.bean.SupportBeanTimestamp),
     * used by the ord-12 time-order case. Constructor parameter order,
     * field types, and getters match the pinned bean.
     */
    public static class LocalSupportBeanTimestamp {
        private final String id;
        private final long timestamp;
        private final String groupId;

        public LocalSupportBeanTimestamp(String id, String groupId, long timestamp) {
            this.id = id;
            this.groupId = groupId;
            this.timestamp = timestamp;
        }

        public String getId() {
            return id;
        }

        public long getTimestamp() {
            return timestamp;
        }

        public String getGroupId() {
            return groupId;
        }
    }
}
