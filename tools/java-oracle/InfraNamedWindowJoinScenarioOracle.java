import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the InfraNamedWindowJoin work-unit Draft 4.249 candidate
 * extended by the Draft 4.250 four-case addition (pinned Esper 9.0.0 commit
 * 9e1b9f1cc9117fea4bf33ab043762c045d73839c,
 * regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/
 * namedwindow/InfraNamedWindowJoin.java).
 *
 * Covers executions of executions() at inventory ordinals 0-2 (Draft 4.249
 * lane) and 3-6 (Draft 4.250 lane):
 *
 * index-choice (InfraJoinIndexChoice, java-runtime-e138d2fc24010a22dbb1)
 * replays all five datawindow/index-set combos of assertIndexChoice
 * behaviorally: each combo re-creates MyWindow over SupportSimpleBeanOne with
 * std:unique(s1) or win:keepall(), adds zero to two optional indexes, preloads
 * (E1,10,11,12)/(E2,20,21,22), then for every where-clause assertion deploys
 * the unidirectional join projecting the pinned observable scalars
 * "select ssb2.s2 as s2, ssb1.s1 as s1, ssb1.i1 as i1 from
 * SupportSimpleBeanTwo as ssb2, MyWindow as ssb1 where ..." named s0, sends
 * SupportSimpleBeanTwo vectors
 * (E2,50,21,22) and (E1,60,11,12) and undeploys the s0 module again. Per the
 * frozen contract this lane is behavioral only: the pinned
 * INDEX_CALLBACK_HOOK/@Hook(INTERNAL_QUERY_PLAN) instrumentation and the
 * SupportQueryPlanIndexHook.assertJoinOneStreamAndReset plan-uniqueness flag
 * are not replayed (Go has no query-plan hook), so every deploy succeeds and
 * the trace carries the join output rows only.
 *
 * right-outer-late-start (InfraRightOuterJoinLateStart,
 * java-runtime-26e5704237191a42acf4) fills WindowLeave#time(6000) with the
 * eight literal SupportQueueLeave vectors (id 1..8, location "0".."3",
 * timeLeave 247) and WindowEnter#time(6000) with the ten literal
 * SupportQueueEnter vectors (id 1..10, location "0".."4", sku alternating
 * 166583/169254, timeEnter 123), deploys the grouped right-outer s1 and the
 * mirrored left-outer s2 statements (output every 1.0 seconds never fires
 * because the internal timer is disabled and time stays at zero), then takes
 * iterator snapshots of s2 first and s1 second at the pinned
 * assertIterator positions; both must hold the identical ten ordered rows.
 *
 * full-outer-named-agg-late-start (InfraFullOuterJoinNamedAggregationLateStart,
 * java-runtime-38e260a335688bf62391) fills MyWindowFO#groupwin(theString,
 * intPrimitive)#length(3) with the eighteen literal SupportBean vectors
 * (theString in c0/c1/c2 x intPrimitive 0..2 x two events, boolPrimitive true
 * explicitly carried in every payload) plus the nineteenth (c1,2,true) bean,
 * snapshots the create-window statement iterator (19 rows), deploys the full
 * outer join aggregation named select, sends SupportMarketDataBean c0 and c3
 * and snapshots the select iterator (ten rows beginning with the unmatched
 * [null,null,0,c3] group).
 *
 * named-and-stream (InfraJoinNamedAndStream, java-runtime-479eabd476ce403c4ab8)
 * deploys keepall MyWindowJNS projecting (theString, intPrimitive) to (a, b)
 * together with its on-SupportBean_A delete and the insert-into feed as one
 * setup module, then the consumer named s0 selecting irstream symbol, a, b
 * from SupportMarketDataBean#length(10) as s0 joined to MyWindowJNS as s1 on
 * s1.a = symbol, and replays the eleven-send vector of market data, beans and
 * bean-A deletions: single-row NEW output when the matching bean arrives,
 * single-row OLD output on the matching deletion, silent otherwise except the
 * two-row NEW batch for market S3 against both buffered S3 beans and its
 * mirrored two-row OLD batch when SB_A(S3) empties the window again.
 *
 * between-named (InfraJoinBetweenNamed, java-runtime-342d46139a3313b382b7)
 * routes boolPrimitive-split bean feeds into keepall MyWindowOne (a1, b1) and
 * MyWindowTwo (a2, b2), deletes from MyWindowOne on market volume=1 and from
 * MyWindowTwo on volume=0 matching symbol, joins the two windows irstream on
 * s0.a1 = s1.a2 as s0, and replays the ten-send vector asserting the pairwise
 * NEW rows as each side fills in, the two-row NEW batch for SB(false,S1,6),
 * then the delete-and-reinsert cycle around S0 ending with NEW
 * {a1=S0,b1=8,a2=S0,b2=7}.
 *
 * between-same-named (InfraJoinBetweenSameNamed,
 * java-runtime-8f4b0394ee32a5524464) self-joins keepall MyWindowJSN as s0 and
 * s1 on s0.a = s1.a projecting renamed a0/b0/a1/b1 columns, feeds the E1 and
 * E2 beans (one NEW row each), deletes on every market event matching symbol
 * and asserts the E1 removal yields exactly ONE old row despite both stream
 * aliases, while market E0 stays silent.
 *
 * single-insert-one-window (InfraJoinSingleInsertOneWindow,
 * java-runtime-e7320476230a3903e2ec) is structurally identical to
 * between-named over windows MyWindowJSIOne/MyWindowJSITwo with the consumer
 * statement named select; it replays the identical ten-send vector producing
 * the same listener records under that statement name.
 *
 * Conventions and deviations: SupportSimpleBeanOne/SupportSimpleBeanTwo are
 * local mirrors of the regression-lib beans (same field names and primitive
 * types) because regression-lib is not on the oracle classpath;
 * SupportMarketDataBean and SupportQueueLeave/Enter are registered as map
 * event types with the same property surface (same rationale as prior infra
 * oracles). Scenario steps carry complete literal field vectors matching the
 * Java sends. The pinned listener on the full-outer create statement is not
 * attached because the suite never reads it; state is observed through the
 * snapshot op instead. The SERDEREQUIRED flag of the right-outer execution is
 * a harness serialization check with no observable effect on a plain runtime.
 * The Draft 4.250 lanes register SupportBean_A as a map event type with the
 * String id field mirroring the regression-lib bean, and add @public to the
 * MyWindowJSN/MyWindowJSIOne/MyWindowJSITwo create-window statements because
 * the suite compiled setup and consumer as one module while this oracle
 * deploys them as separate modules.
 * Records follow the standard protocol: listener rows (sorted property names,
 * normalized values, new before old), snapshot rows over the statement
 * iterator in engine order, sequence numbers per case, epoch-zero timestamps
 * with the internal timer disabled.
 */
public class InfraNamedWindowJoinScenarioOracle {

    private static final String[] DATAWINDOWS = {
        "std:unique(s1)",
        "std:unique(s1)",
        "std:unique(s1)",
        "std:unique(s1)",
        "win:keepall()"
    };

    private static final String[][] INDEX_SETS = {
        {},
        {"create unique index One on MyWindow (s1)"},
        {"create unique index One on MyWindow (s1, l1)"},
        {"create index One on MyWindow (s1)", "create unique index Two on MyWindow (s1, d1)"},
        {"create index One on MyWindow (s1)", "create unique index Two on MyWindow (s1, d1)"}
    };

    private static final String[][] WHERE_SETS = {
        {"s1 = s2", "s1 = s2 and l1 = l2"},
        {"s1 = s2", "s1 = s2 and l1 = l2"},
        {"s1 = s2", "d1 = d2", "s1 = s2 and l1 = l2"},
        {"d1 = d2", "s1 = s2", "s1 = s2 and l1 = l2", "s1 = s2 and d1 = d2 and l1 = l2"},
        {"d1 = d2", "s1 = s2", "s1 = s2 and l1 = l2", "s1 = s2 and d1 = d2 and l1 = l2", "d1 = d2 and s1 = s2"}
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: InfraNamedWindowJoinScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

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
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        switch (caseName) {
            case "index-choice" -> {
                config.getCommon().addEventType(SupportSimpleBeanOne.class);
                config.getCommon().addEventType(SupportSimpleBeanTwo.class);
            }
            case "right-outer-late-start" -> {
                Map<String, Object> leaveType = new HashMap<>();
                leaveType.put("id", int.class);
                leaveType.put("location", String.class);
                leaveType.put("timeLeave", long.class);
                config.getCommon().addEventType("SupportQueueLeave", leaveType);
                Map<String, Object> enterType = new HashMap<>();
                enterType.put("id", int.class);
                enterType.put("location", String.class);
                enterType.put("sku", String.class);
                enterType.put("timeEnter", long.class);
                config.getCommon().addEventType("SupportQueueEnter", enterType);
            }
            case "full-outer-named-agg-late-start" -> {
                config.getCommon().addEventType(SupportBean.class);
                Map<String, Object> marketType = new HashMap<>();
                marketType.put("symbol", String.class);
                marketType.put("price", double.class);
                marketType.put("volume", Long.class);
                marketType.put("feed", String.class);
                config.getCommon().addEventType("SupportMarketDataBean", marketType);
            }
            case "named-and-stream", "between-named", "between-same-named", "single-insert-one-window" -> {
                config.getCommon().addEventType(SupportBean.class);
                Map<String, Object> marketType = new HashMap<>();
                marketType.put("symbol", String.class);
                marketType.put("price", double.class);
                marketType.put("volume", Long.class);
                marketType.put("feed", String.class);
                config.getCommon().addEventType("SupportMarketDataBean", marketType);
                Map<String, Object> beanAType = new HashMap<>();
                beanAType.put("id", String.class);
                config.getCommon().addEventType("SupportBean_A", beanAType);
            }
            default -> throw new IllegalStateException("unknown case: " + caseName);
        }
        EPRuntime runtime = EPRuntimeProvider.getRuntime("InfraNamedWindowJoinScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);

        CaseContext ctx = new CaseContext(caseName, runtime, records, config);
        try {
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
                    case "send" -> ctx.send(step);
                    case "deploy" -> ctx.deploy(step.getString("statement", ""));
                    case "undeploy" -> ctx.undeployLast();
                    case "undeploy-all" -> ctx.undeployAll();
                    case "snapshot" -> ctx.snapshot(step.getString("statement", ""));
                    default -> throw new IllegalStateException("unsupported op " + op + " in case " + caseName);
                }
            }
            ctx.validateEnd();
        } finally {
            runtime.destroy();
        }
    }

    private static final class CaseContext {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private final Configuration config;
        private final Map<String, EPStatement> statements = new LinkedHashMap<>();
        private final List<String> lastDeploymentStatements = new ArrayList<>();
        private String lastDeploymentId;

        // index-choice cursor state
        private int comboPtr = -1;
        private int indexPtr = 0;
        private int wherePtr = 0;

        // send accounting for end-of-case validation
        private int queueLeaveSends = 0;
        private int queueEnterSends = 0;
        private int fillBeanSends = 0;
        private int beanASends = 0;
        private int marketSends = 0;
        private int snapshots = 0;
        private int seq = 0;

        CaseContext(String caseName, EPRuntime runtime, List<JsonObject> records, Configuration config) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
            this.config = config;
        }

        private void deploy(String key) throws Exception {
            String epl = eplFor(key);
            CompilerArguments compilerArgs = new CompilerArguments(config);
            compilerArgs.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            lastDeploymentId = deployment.getDeploymentId();
            lastDeploymentStatements.clear();
            for (EPStatement statement : deployment.getStatements()) {
                statements.put(statement.getName(), statement);
                lastDeploymentStatements.add(statement.getName());
                if (listenerStatement().equals(statement.getName())) {
                    attachListener(statement);
                }
            }
        }

        private String eplFor(String key) {
            switch (caseName) {
                case "index-choice":
                    return indexChoiceEpl(key);
                case "right-outer-late-start":
                    return rightOuterEpl(key);
                case "full-outer-named-agg-late-start":
                    return fullOuterEpl(key);
                case "named-and-stream":
                case "between-named":
                case "between-same-named":
                case "single-insert-one-window":
                    return joinExecEpl(key);
                default:
                    throw new IllegalStateException("unknown case " + caseName);
            }
        }

        private String indexChoiceEpl(String key) {
            switch (key) {
                case "window": {
                    comboPtr++;
                    if (comboPtr >= DATAWINDOWS.length) {
                        throw new IllegalStateException("index-choice has more window deployments than combos");
                    }
                    indexPtr = 0;
                    wherePtr = 0;
                    return "@public create window MyWindow." + DATAWINDOWS[comboPtr] + " as SupportSimpleBeanOne";
                }
                case "insert":
                    requireCombo();
                    return "insert into MyWindow select * from SupportSimpleBeanOne";
                case "index": {
                    requireCombo();
                    if (indexPtr >= INDEX_SETS[comboPtr].length) {
                        throw new IllegalStateException("index-choice combo " + comboPtr + " has more index deployments than declared");
                    }
                    return INDEX_SETS[comboPtr][indexPtr++];
                }
                case "s0": {
                    requireCombo();
                    if (wherePtr >= WHERE_SETS[comboPtr].length) {
                        throw new IllegalStateException("index-choice combo " + comboPtr + " has more s0 deployments than declared");
                    }
                    String where = WHERE_SETS[comboPtr][wherePtr];
                    wherePtr++;
                    return "@name('s0')" +
                        "select ssb2.s2 as s2, ssb1.s1 as s1, ssb1.i1 as i1 " +
                        "from SupportSimpleBeanTwo as ssb2 unidirectional, MyWindow as ssb1 " +
                        "where " + where;
                }
                default:
                    throw new IllegalStateException("unknown index-choice deploy key " + key);
            }
        }

        private void requireCombo() {
            if (comboPtr < 0 || comboPtr >= DATAWINDOWS.length) {
                throw new IllegalStateException("index-choice deploy outside a combo");
            }
        }

        private String rightOuterEpl(String key) {
            switch (key) {
                case "window-leave":
                    return "@public create window WindowLeave#time(6000) as select timeLeave, id, location from SupportQueueLeave;\n" +
                        "insert into WindowLeave select timeLeave, id, location from SupportQueueLeave;";
                case "window-enter":
                    return "@public create window WindowEnter#time(6000) as select location, sku, timeEnter, id from SupportQueueEnter;\n" +
                        "insert into WindowEnter select location, sku, timeEnter, id from SupportQueueEnter;";
                case "s1":
                    return "@name('s1') select s1.location as loc, sku, avg((coalesce(timeLeave, 250) - timeEnter)) as avgTime, " +
                        "count(timeEnter) as cntEnter, count(timeLeave) as cntLeave, (count(timeEnter) - count(timeLeave)) as diff " +
                        "from WindowLeave as s0 right outer join WindowEnter as s1 " +
                        "on s0.id = s1.id and s0.location = s1.location " +
                        "group by s1.location, sku " +
                        "output every 1.0 seconds " +
                        "order by s1.location, sku";
                case "s2":
                    return "@name('s2') select s1.location as loc, sku, avg((coalesce(timeLeave, 250) - timeEnter)) as avgTime, " +
                        "count(timeEnter) as cntEnter, count(timeLeave) as cntLeave, (count(timeEnter) - count(timeLeave)) as diff " +
                        "from WindowEnter as s1 left outer join WindowLeave as s0 " +
                        "on s0.id = s1.id and s0.location = s1.location " +
                        "group by s1.location, sku " +
                        "output every 1.0 seconds " +
                        "order by s1.location, sku";
                default:
                    throw new IllegalStateException("unknown right-outer deploy key " + key);
            }
        }

        private String fullOuterEpl(String key) {
            switch (key) {
                case "create":
                    return "@name('create') @public create window MyWindowFO#groupwin(theString, intPrimitive)#length(3) as select theString, intPrimitive, boolPrimitive from SupportBean;\n" +
                        "insert into MyWindowFO select theString, intPrimitive, boolPrimitive from SupportBean;\n";
                case "select":
                    return "@name('select') select theString, intPrimitive, count(boolPrimitive) as cntBool, symbol " +
                        "from MyWindowFO full outer join SupportMarketDataBean#keepall " +
                        "on theString = symbol " +
                        "group by theString, intPrimitive, symbol order by theString, intPrimitive, symbol";
                default:
                    throw new IllegalStateException("unknown full-outer deploy key " + key);
            }
        }

        private String joinExecEpl(String key) {
            switch (caseName + "/" + key) {
                case "named-and-stream/setup":
                    return "@name('create') @public create window MyWindowJNS#keepall as select theString as a, intPrimitive as b from SupportBean;\n" +
                        "on SupportBean_A delete from MyWindowJNS where id = a;\n" +
                        "insert into MyWindowJNS select theString as a, intPrimitive as b from SupportBean;\n";
                case "named-and-stream/s0":
                    return "@name('s0') select irstream symbol, a, b " +
                        "from SupportMarketDataBean#length(10) as s0," +
                        "MyWindowJNS as s1 where s1.a = symbol";
                case "between-named/setup":
                    return "@name('createOne') @public create window MyWindowOne#keepall as select theString as a1, intPrimitive as b1 from SupportBean;\n" +
                        "@name('createTwo') @public create window MyWindowTwo#keepall as select theString as a2, intPrimitive as b2 from SupportBean;\n" +
                        "on SupportMarketDataBean(volume=1) delete from MyWindowOne where symbol = a1;\n" +
                        "on SupportMarketDataBean(volume=0) delete from MyWindowTwo where symbol = a2;\n" +
                        "insert into MyWindowOne select theString as a1, intPrimitive as b1 from SupportBean(boolPrimitive = true);\n" +
                        "insert into MyWindowTwo select theString as a2, intPrimitive as b2 from SupportBean(boolPrimitive = false);\n";
                case "between-named/s0":
                    return "@name('s0') select irstream a1, b1, a2, b2 from MyWindowOne as s0, MyWindowTwo as s1 where s0.a1 = s1.a2";
                case "between-same-named/setup":
                    return "@name('create') @public create window MyWindowJSN#keepall as select theString as a, intPrimitive as b from SupportBean;\n" +
                        "on SupportMarketDataBean delete from MyWindowJSN where symbol = a;\n" +
                        "insert into MyWindowJSN select theString as a, intPrimitive as b from SupportBean;\n";
                case "between-same-named/s0":
                    return "@name('s0') select irstream s0.a as a0, s0.b as b0, s1.a as a1, s1.b as b1 " +
                        "from MyWindowJSN as s0, MyWindowJSN as s1 where s0.a = s1.a";
                case "single-insert-one-window/setup":
                    return "@name('create') @public create window MyWindowJSIOne#keepall as select theString as a1, intPrimitive as b1 from SupportBean;\n" +
                        "@name('createTwo') @public create window MyWindowJSITwo#keepall as select theString as a2, intPrimitive as b2 from SupportBean;\n" +
                        "on SupportMarketDataBean(volume=1) delete from MyWindowJSIOne where symbol = a1;\n" +
                        "on SupportMarketDataBean(volume=0) delete from MyWindowJSITwo where symbol = a2;\n" +
                        "insert into MyWindowJSIOne select theString as a1, intPrimitive as b1 from SupportBean(boolPrimitive = true);\n" +
                        "insert into MyWindowJSITwo select theString as a2, intPrimitive as b2 from SupportBean(boolPrimitive = false);\n";
                case "single-insert-one-window/select":
                    return "@name('select') select irstream a1, b1, a2, b2 from MyWindowJSIOne as s0, MyWindowJSITwo as s1 where s0.a1 = s1.a2";
                default:
                    throw new IllegalStateException("unknown " + caseName + " deploy key " + key);
            }
        }

        private void undeployLast() throws Exception {
            if (lastDeploymentId == null) {
                throw new IllegalStateException("undeploy without a previous deploy in case " + caseName);
            }
            runtime.getDeploymentService().undeploy(lastDeploymentId);
            for (String name : lastDeploymentStatements) {
                statements.remove(name);
            }
            lastDeploymentId = null;
            lastDeploymentStatements.clear();
        }

        private void undeployAll() throws Exception {
            runtime.getDeploymentService().undeployAll();
            statements.clear();
            lastDeploymentId = null;
            lastDeploymentStatements.clear();
        }

        private String listenerStatement() {
            switch (caseName) {
                case "index-choice":
                case "named-and-stream":
                case "between-named":
                case "between-same-named":
                    return "s0";
                case "single-insert-one-window":
                    return "select";
                default:
                    return "";
            }
        }

        private void attachListener(EPStatement statement) {
            statement.addListener((newData, oldData, stmt, rt) -> {
                boolean hasNew = newData != null && newData.length > 0;
                boolean hasOld = oldData != null && oldData.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                seq++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", stmt.getName());
                record.add("sequence", seq);
                record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                record.add("new", renderRows(hasNew ? newData : null));
                if (hasOld) {
                    record.add("old", renderRows(oldData));
                }
                records.add(record);
            });
        }

        private void snapshot(String wanted) {
            EPStatement target = statements.get(wanted);
            if (target == null) {
                throw new IllegalStateException("no statement named " + wanted + " in case " + caseName);
            }
            snapshots++;
            JsonArray rows = new JsonArray();
            java.util.Iterator<EventBean> it = target.iterator();
            while (it.hasNext()) {
                rows.add(renderRow(it.next()));
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "snapshot");
            record.add("statement", target.getName());
            record.add("sequence", 0);
            record.add("time", java.time.Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", rows);
            records.add(record);
        }

        private void send(JsonObject step) {
            String type = step.getString("eventType", "");
            JsonObject payload = step.get("payload").asObject();
            switch (type) {
                case "SupportSimpleBeanOne" -> {
                    SupportSimpleBeanOne event = new SupportSimpleBeanOne(
                        payload.getString("s1", null),
                        payload.getInt("i1", 0),
                        payload.getDouble("d1", 0),
                        payload.getLong("l1", 0));
                    runtime.getEventService().sendEventBean(event, "SupportSimpleBeanOne");
                }
                case "SupportSimpleBeanTwo" -> {
                    SupportSimpleBeanTwo event = new SupportSimpleBeanTwo(
                        payload.getString("s2", null),
                        payload.getInt("i2", 0),
                        payload.getDouble("d2", 0),
                        payload.getLong("l2", 0));
                    runtime.getEventService().sendEventBean(event, "SupportSimpleBeanTwo");
                }
                case "SupportQueueLeave" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("id", payload.getInt("id", 0));
                    event.put("location", payload.getString("location", null));
                    event.put("timeLeave", payload.getLong("timeLeave", 0));
                    runtime.getEventService().sendEventMap(event, "SupportQueueLeave");
                    queueLeaveSends++;
                }
                case "SupportQueueEnter" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("id", payload.getInt("id", 0));
                    event.put("location", payload.getString("location", null));
                    event.put("sku", payload.getString("sku", null));
                    event.put("timeEnter", payload.getLong("timeEnter", 0));
                    runtime.getEventService().sendEventMap(event, "SupportQueueEnter");
                    queueEnterSends++;
                }
                case "SupportBean_A" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("id", payload.getString("id", null));
                    runtime.getEventService().sendEventMap(event, "SupportBean_A");
                    beanASends++;
                }
                case "SupportBean" -> {
                    SupportBean event = new SupportBean();
                    event.setTheString(payload.getString("theString", null));
                    event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                    event.setBoolPrimitive(payload.getBoolean("boolPrimitive", false));
                    runtime.getEventService().sendEventBean(event, "SupportBean");
                    fillBeanSends++;
                }
                case "SupportMarketDataBean" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("symbol", payload.getString("symbol", null));
                    event.put("price", payload.getDouble("price", 0));
                    event.put("volume", payload.getLong("volume", 0));
                    event.put("feed", payload.getString("feed", null));
                    runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
                    marketSends++;
                }
                default -> throw new IllegalStateException("unknown eventType " + type + " in case " + caseName);
            }
        }

        private void validateEnd() {
            switch (caseName) {
                case "index-choice" -> {
                    if (comboPtr != DATAWINDOWS.length - 1 || wherePtr != WHERE_SETS[DATAWINDOWS.length - 1].length) {
                        throw new IllegalStateException("index-choice did not consume all five combos and their assertions");
                    }
                }
                case "right-outer-late-start" -> {
                    if (queueLeaveSends != 8 || queueEnterSends != 10 || snapshots != 2) {
                        throw new IllegalStateException("right-outer requires eight leave sends, ten enter sends and two snapshots");
                    }
                }
                case "full-outer-named-agg-late-start" -> {
                    if (fillBeanSends != 19 || marketSends != 2 || snapshots != 2) {
                        throw new IllegalStateException("full-outer requires nineteen bean sends, two market sends and two snapshots");
                    }
                }
                case "named-and-stream" -> {
                    if (fillBeanSends != 4 || marketSends != 5 || beanASends != 2) {
                        throw new IllegalStateException("named-and-stream requires four bean sends, five market sends and two bean-A sends");
                    }
                }
                case "between-named", "single-insert-one-window" -> {
                    if (fillBeanSends != 8 || marketSends != 2) {
                        throw new IllegalStateException(caseName + " requires eight bean sends and two market sends");
                    }
                }
                case "between-same-named" -> {
                    if (fillBeanSends != 2 || marketSends != 2) {
                        throw new IllegalStateException("between-same-named requires two bean sends and two market sends");
                    }
                }
                default -> throw new IllegalStateException("unknown case " + caseName);
            }
        }
    }

    private static JsonArray renderRows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(renderRow(event));
        }
        return rows;
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
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
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Collection<?>) {
            JsonArray array = new JsonArray();
            for (Object item : (Collection<?>) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the regression-lib SupportSimpleBeanOne (regression-lib is not on the oracle classpath). */
    public static class SupportSimpleBeanOne {
        private final String s1;
        private final int i1;
        private final double d1;
        private final long l1;

        public SupportSimpleBeanOne(String s1, int i1, double d1, long l1) {
            this.s1 = s1;
            this.i1 = i1;
            this.d1 = d1;
            this.l1 = l1;
        }

        public String getS1() {
            return s1;
        }

        public int getI1() {
            return i1;
        }

        public double getD1() {
            return d1;
        }

        public long getL1() {
            return l1;
        }
    }

    /** Local mirror of the regression-lib SupportSimpleBeanTwo (regression-lib is not on the oracle classpath). */
    public static class SupportSimpleBeanTwo {
        private final String s2;
        private final int i2;
        private final double d2;
        private final long l2;

        public SupportSimpleBeanTwo(String s2, int i2, double d2, long l2) {
            this.s2 = s2;
            this.i2 = i2;
            this.d2 = d2;
            this.l2 = l2;
        }

        public String getS2() {
            return s2;
        }

        public int getI2() {
            return i2;
        }

        public double getD2() {
            return d2;
        }

        public long getL2() {
            return l2;
        }
    }
}
