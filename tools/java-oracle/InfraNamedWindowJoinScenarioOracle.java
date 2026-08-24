import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
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

import java.io.Serializable;
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
 * extended by the Draft 4.250 four-case addition and the Draft 4.251
 * seven-case addition (pinned Esper 9.0.0 commit
 * 9e1b9f1cc9117fea4bf33ab043762c045d73839c,
 * regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/
 * namedwindow/InfraNamedWindowJoin.java).
 *
 * Covers executions of executions() at inventory ordinals 0-2 (Draft 4.249
 * lane), 3-6 (Draft 4.250 lane) and 7-9 (Draft 4.251 lane):
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
 * unidirectional (InfraUnidirectional, java-runtime-98c12887d1e25c26c101)
 * deploys the pinned three-statement module (keepall MyWindowU over the full
 * SupportBean surface with its insert feed and the consumer named select
 * projecting w.* over MyWindowU w unidirectional joined to
 * SupportBean_A#lastevent on id = theString) and replays the four-send vector
 * E1 bean, A(E1), A(E2), E2 bean: only the E2 bean arrival joins, so exactly
 * one record carries the full twenty-property SupportBean row while A-side
 * arrivals against an already-matching window stay silent.
 *
 * window-unidirectional-join (InfraWindowUnidirectionalJoin,
 * java-runtime-09ca3e1b6ac4f52b0b7e) deploys the pinned four-statement module
 * (@public keepall MyWindowWUJ over SupportBean, its insert feed, the
 * on-SupportBean_S1 delete keyed by p10 = theString, and the s0 consumer
 * selecting window(win.*) as c0, window(win.*).where(v => v.intPrimitive < 2)
 * as c1 and window(win.*).toMap(k=>k.theString,v=>v.intPrimitive) as c2 from
 * unidirectional SupportBean_S0 joined to MyWindowWUJ) and replays the
 * eleven-send vector of three beans, five S0 triggers and three deletes: the
 * first four triggers each produce one record while the window fills and
 * shrinks (c1 shrinks to [] ahead of c0/c2 because 2 < 2 is false), the fifth
 * trigger finds the window empty so zero join rows reach the listener guard;
 * three compile-only deployments then prove the non-unidirectional selection,
 * join and subquery forms of window(win.*) without listeners or sends.
 *
 * inner-join-late-start-{objectarray,map,json,jsonprovided,default}
 * (InfraInnerJoinLateStart, java-runtime-2a245ed9721b6b840746) replay five of
 * the six pinned EventRepresentationChoice iterations with byte-identical
 * records apart from the case name: the annotated @public @buseventtype
 * Product/Portfolio schemas (jsonprovided prefixes "@JsonSchema(
 * className='<oracle mirror FQCN>') @EventRepresentation('json')" mirroring
 * EventRepresentationChoice.JSONCLASSPROVIDED.getAnnotationTextWJsonProvided
 * and resolving the local MyLocalJsonProvided* mirror classes exactly like
 * the pinned suite resolves its own nested mirrors), the keepall windows over
 * both schemas with their insert feeds, the productA/productB/productA
 * preload vector, the Query2 unidirectional inner join carrying the listener,
 * and the assertion vector: matching portfolio productB emits one
 * {portfolio, ProductWin.product, size} row, unmatched portfolio productC and
 * product-only insert productC stay silent, and the retrying portfolio
 * productC emits the second row.
 *
 * Draft 4.251 documented deviations: (1) window(win.*) evaluates to raw
 * SupportBean UNDERLYINGS which the pinned assertions compare element-wise
 * via SupportBean.equals; the renderer therefore defines the protocol that a
 * value whose class equals the underlying class of a REGISTERED event type
 * renders row-shaped over the engine-resolved property surface, consistent
 * with the EventBean branch and preserving the full property surface that the
 * String.valueOf fallback would flatten into a "SupportBean(...)" string;
 * resolution uses a case-local underlying-class-to-event-type map populated
 * from the preconfigured registration. (2) The AVRO representation iteration
 * of InfraInnerJoinLateStart is skipped as an approved infrastructure
 * difference: this oracle lane has no Avro precedent, and the pinned
 * underlying-class matrix assertions are harness-internal checks that emit no
 * records either way.
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
 * The Draft 4.251 lanes register SupportBean_S0/SupportBean_S1 as map event
 * types with id int and p00..p03/p10..p13 String mirrors (the regression
 * beans' private value field has no getter and is therefore not an event
 * property).
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
        if (args.length != 1 && args.length != 2) {
            System.err.println("usage: InfraNamedWindowJoinScenarioOracle <scenario.json> [case-filter]");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        String caseFilter = args.length == 2 ? args[1] : null;
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            if (caseFilter != null && !caseFilter.equals(caseName)) {
                continue;
            }
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
            case "unidirectional" -> {
                config.getCommon().addEventType(SupportBean.class);
                Map<String, Object> beanAType = new HashMap<>();
                beanAType.put("id", String.class);
                config.getCommon().addEventType("SupportBean_A", beanAType);
            }
            case "window-unidirectional-join" -> {
                config.getCommon().addEventType(SupportBean.class);
                Map<String, Object> s0Type = new LinkedHashMap<>();
                s0Type.put("id", int.class);
                s0Type.put("p00", String.class);
                s0Type.put("p01", String.class);
                s0Type.put("p02", String.class);
                s0Type.put("p03", String.class);
                config.getCommon().addEventType("SupportBean_S0", s0Type);
                Map<String, Object> s1Type = new LinkedHashMap<>();
                s1Type.put("id", int.class);
                s1Type.put("p10", String.class);
                s1Type.put("p11", String.class);
                s1Type.put("p12", String.class);
                s1Type.put("p13", String.class);
                config.getCommon().addEventType("SupportBean_S1", s1Type);
            }
            case "inner-join-late-start-objectarray",
                 "inner-join-late-start-map",
                 "inner-join-late-start-json",
                 "inner-join-late-start-jsonprovided",
                 "inner-join-late-start-default" -> {
                // Product/Portfolio types come from the pinned schema deployment itself.
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
        private int sZeroSends = 0;
        private int sOneSends = 0;
        private int productSends = 0;
        private int portfolioSends = 0;

        // Registered-underlying protocol support (see class Javadoc):
        // underlying classes of preconfigured bean event types mapped to
        // their engine-resolved event types.
        private final Map<Class<?>, EventType> underlyingEventTypes = new HashMap<>();
        private int snapshots = 0;
        private int seq = 0;

        CaseContext(String caseName, EPRuntime runtime, List<JsonObject> records, Configuration config) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
            this.config = config;
            EventType preconfiguredSupportBean =
                runtime.getEventTypeService().getEventTypePreconfigured("SupportBean");
            if (preconfiguredSupportBean != null) {
                underlyingEventTypes.put(preconfiguredSupportBean.getUnderlyingType(), preconfiguredSupportBean);
            }
        }

        private void deploy(String key) throws Exception {
            if (jsonPopulationModule(key)) {
                // The plain-json representation is the only lane whose path
                // carries a GENERATED (non-provided) JSON event type; the
                // pinned compiler re-adds generated underlyings from every
                // path registry entry, so a second compilation whose runtime
                // path holds more than one deployment aborts with a duplicate
                // class error. The five population statements therefore
                // compile and deploy as ONE module at the first population
                // deploy step (single pathable) and the remaining population
                // deploy steps become no-ops. Module grouping is already an
                // established deviation of this lane; late-start semantics
                // are preserved because the population statements emit no
                // records and query2 still deploys after the preload sends.
                deployJsonPopulationModule();
                return;
            }
            if (jsonPopulationConsumed(key)) {
                return;
            }
            String epl = eplFor(key);
            CompilerArguments compilerArgs = new CompilerArguments(config);
            if (jsonQuery2(key)) {
                // Compile against the single population module pathable; the
                // accumulating runtime path would carry the generated JSON
                // type once per deployment.
                compilerArgs.getPath().add(populationCompiled);
            } else {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
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

        private EPCompiled populationCompiled;

        private boolean jsonQuery2(String key) {
            return "inner-join-late-start-json".equals(caseName) && "query2".equals(key);
        }

        private boolean jsonPopulationModule(String key) {
            return "inner-join-late-start-json".equals(caseName) && "schema".equals(key);
        }

        private boolean jsonPopulationConsumed(String key) {
            return "inner-join-late-start-json".equals(caseName)
                && ("window".equals(key) || "insert-product".equals(key)
                || "portfolio-window".equals(key) || "insert-portfolio".equals(key));
        }

        private void deployJsonPopulationModule() throws Exception {
            String epl = innerJoinLateStartEpl("schema") + innerJoinLateStartEpl("window") + ";\n"
                + innerJoinLateStartEpl("insert-product") + ";\n"
                + innerJoinLateStartEpl("portfolio-window") + ";\n"
                + innerJoinLateStartEpl("insert-portfolio") + ";\n";
            CompilerArguments compilerArgs = new CompilerArguments(config);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            populationCompiled = compiled;
            lastDeploymentId = deployment.getDeploymentId();
            lastDeploymentStatements.clear();
            for (EPStatement statement : deployment.getStatements()) {
                statements.put(statement.getName(), statement);
                lastDeploymentStatements.add(statement.getName());
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
                case "unidirectional":
                    return unidirectionalEpl(key);
                case "window-unidirectional-join":
                    return windowUnidirectionalJoinEpl(key);
                case "inner-join-late-start-objectarray":
                case "inner-join-late-start-map":
                case "inner-join-late-start-json":
                case "inner-join-late-start-jsonprovided":
                case "inner-join-late-start-default":
                    return innerJoinLateStartEpl(key);
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

        /**
         * Pinned InfraUnidirectional module: three statements deployed as
         * one module exactly as the suite compiles them.
         */
        private String unidirectionalEpl(String key) {
            switch (key) {
                case "setup":
                    return "create window MyWindowU#keepall select * from SupportBean;\n" +
                        "insert into MyWindowU select * from SupportBean;\n" +
                        "@name('select') select w.* from MyWindowU w unidirectional, SupportBean_A#lastevent s where s.id = w.theString;\n";
                default:
                    throw new IllegalStateException("unknown unidirectional deploy key " + key);
            }
        }

        /**
         * Pinned InfraWindowUnidirectionalJoin module plus the three trailing
         * compile-only window(win.*) forms (selection, join, subquery).
         */
        private String windowUnidirectionalJoinEpl(String key) {
            switch (key) {
                case "setup":
                    return "@public create window MyWindowWUJ#keepall as SupportBean;\n" +
                        "insert into MyWindowWUJ select * from SupportBean;\n" +
                        "on SupportBean_S1 as s1 delete from MyWindowWUJ where s1.p10 = theString;\n" +
                        "@name('s0') select window(win.*) as c0," +
                        "window(win.*).where(v => v.intPrimitive < 2) as c1, " +
                        "window(win.*).toMap(k=>k.theString,v=>v.intPrimitive) as c2 " +
                        "from SupportBean_S0 as s0 unidirectional, MyWindowWUJ as win";
                case "compile-a":
                    return "select window(win.*) from MyWindowWUJ as win";
                case "compile-b":
                    return "select window(win.*) as c0 from SupportBean_S0#lastevent as s0, MyWindowWUJ as win";
                case "compile-c":
                    return "select (select window(win.*) from MyWindowWUJ as win) from SupportBean_S0";
                default:
                    throw new IllegalStateException("unknown window-unidirectional-join deploy key " + key);
            }
        }

        /**
         * Mirrors EventRepresentationChoice.getAnnotationTextWJsonProvided(Class)
         * per representation; only JSONCLASSPROVIDED differs from the plain
         * annotation text, prefixing "@JsonSchema(className='...')" that
         * resolves the local MyLocalJsonProvided* mirror class named by the
         * caller (same shape as the pinned suite's own nested mirrors).
         */
        private String representationAnnotation(String providedClassSimpleName) {
            switch (caseName) {
                case "inner-join-late-start-objectarray":
                    return "@EventRepresentation('objectarray')";
                case "inner-join-late-start-map":
                    return "@EventRepresentation('map')";
                case "inner-join-late-start-json":
                    return "@EventRepresentation('json')";
                case "inner-join-late-start-jsonprovided":
                    return "@JsonSchema(className='" + InfraNamedWindowJoinScenarioOracle.class.getName() +
                        "$" + providedClassSimpleName + "') @EventRepresentation('json')";
                case "inner-join-late-start-default":
                    return "";
                default:
                    throw new IllegalStateException("representation annotation undefined for case " + caseName);
            }
        }

        /**
         * Pinned InfraInnerJoinLateStart deployments; the annotation text of
         * the schema module varies per representation while every other
         * deployment is identical across the five replayed representations.
         */
        private String innerJoinLateStartEpl(String key) {
            switch (key) {
                case "schema":
                    return representationAnnotation("MyLocalJsonProvidedProduct") +
                        "@name('schema') @public @buseventtype create schema Product (product string, size int);\n" +
                        representationAnnotation("MyLocalJsonProvidedPortfolio") +
                        " @public @buseventtype create schema Portfolio (portfolio string, product string);\n";
                case "window":
                    return "@name('window') @public create window ProductWin#keepall as Product";
                case "insert-product":
                    return "insert into ProductWin select * from Product";
                case "portfolio-window":
                    return "@public create window PortfolioWin#keepall as Portfolio";
                case "insert-portfolio":
                    return "insert into PortfolioWin select * from Portfolio";
                case "query2":
                    return "@Name(\"Query2\") select portfolio, ProductWin.product, size " +
                        "from PortfolioWin unidirectional inner join ProductWin on PortfolioWin.product=ProductWin.product";
                default:
                    throw new IllegalStateException("unknown inner-join-late-start deploy key " + key);
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
                case "single-insert-one-window", "unidirectional":
                    return "select";
                case "window-unidirectional-join":
                    return "s0";
                case "inner-join-late-start-objectarray", "inner-join-late-start-map",
                        "inner-join-late-start-json", "inner-join-late-start-jsonprovided",
                        "inner-join-late-start-default":
                    return "Query2";
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
            JsonValue rawPayload = step.get("payload");
            // Representation sends carry positional JSON arrays; every other
            // event type carries an object payload.
            JsonObject payload = type.equals("Product") || type.equals("Portfolio")
                ? null
                : rawPayload.asObject();
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
                case "SupportBean_S0" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("id", payload.getInt("id", 0));
                    event.put("p00", payload.getString("p00", null));
                    event.put("p01", payload.getString("p01", null));
                    event.put("p02", payload.getString("p02", null));
                    event.put("p03", payload.getString("p03", null));
                    runtime.getEventService().sendEventMap(event, "SupportBean_S0");
                    sZeroSends++;
                }
                case "SupportBean_S1" -> {
                    Map<String, Object> event = new LinkedHashMap<>();
                    event.put("id", payload.getInt("id", 0));
                    event.put("p10", payload.getString("p10", null));
                    event.put("p11", payload.getString("p11", null));
                    event.put("p12", payload.getString("p12", null));
                    event.put("p13", payload.getString("p13", null));
                    runtime.getEventService().sendEventMap(event, "SupportBean_S1");
                    sOneSends++;
                }
                case "Product", "Portfolio" -> sendRepresentationEvent(type, rawPayload);
                default -> throw new IllegalStateException("unknown eventType " + type + " in case " + caseName);
            }
        }

        /**
         * Sends a Product or Portfolio event using the pinned helper semantics
         * for the representation under test: positional object array in schema
         * declaration order, LinkedHashMap map send, or JsonObject JSON send;
         * values come from the scenario payload.
         */
        private void sendRepresentationEvent(String type, JsonValue payload) {
            boolean product = type.equals("Product");
            if (caseName.equals("inner-join-late-start-objectarray")) {
                JsonArray values = payload.asArray();
                Object[] event = product
                    ? new Object[]{values.get(0).asString(), values.get(1).asInt()}
                    : new Object[]{values.get(0).asString(), values.get(1).asString()};
                runtime.getEventService().sendEventObjectArray(event, type);
            } else if (caseName.equals("inner-join-late-start-map")
                || caseName.equals("inner-join-late-start-default")) {
                JsonObject payloadObject = payload.asObject();
                Map<String, Object> event = new LinkedHashMap<>();
                if (product) {
                    event.put("product", payloadObject.getString("product", null));
                    event.put("size", payloadObject.getInt("size", 0));
                } else {
                    event.put("portfolio", payloadObject.getString("portfolio", null));
                    event.put("product", payloadObject.getString("product", null));
                }
                runtime.getEventService().sendEventMap(event, type);
            } else {
                JsonObject payloadObject = payload.asObject();
                JsonObject event = new JsonObject();
                if (product) {
                    event.add("product", payloadObject.getString("product", null));
                    event.add("size", payloadObject.getInt("size", 0));
                } else {
                    event.add("portfolio", payloadObject.getString("portfolio", null));
                    event.add("product", payloadObject.getString("product", null));
                }
                runtime.getEventService().sendEventJson(event.toString(), type);
            }
            if (product) {
                productSends++;
            } else {
                portfolioSends++;
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
                case "unidirectional" -> {
                    if (fillBeanSends != 2 || beanASends != 2) {
                        throw new IllegalStateException("unidirectional requires two bean sends and two bean-A sends");
                    }
                }
                case "window-unidirectional-join" -> {
                    if (fillBeanSends != 3 || sZeroSends != 5 || sOneSends != 3) {
                        throw new IllegalStateException(
                            "window-unidirectional-join requires three bean sends, five S0 sends and three S1 sends");
                    }
                }
                case "inner-join-late-start-objectarray", "inner-join-late-start-map",
                        "inner-join-late-start-json", "inner-join-late-start-jsonprovided",
                        "inner-join-late-start-default" -> {
                    if (productSends != 3 || portfolioSends != 4) {
                        throw new IllegalStateException(caseName + " requires three product sends and four portfolio sends");
                    }
                }
                default -> throw new IllegalStateException("unknown case " + caseName);
            }
        }

        private JsonArray renderRows(EventBean[] events) {
            JsonArray rows = new JsonArray();
            if (events == null) {
                return rows;
            }
            for (EventBean event : events) {
                rows.add(renderRow(event));
            }
            return rows;
        }

        private JsonObject renderRow(EventBean event) {
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(event.get(prop)));
            }
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            item.add("fields", fields);
            return item;
        }

        private JsonValue normalize(Object value) {
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
            // Registered-underlying protocol: a raw underlying whose class is
            // the underlying class of a registered event type renders
            // row-shaped exactly like the EventBean branch instead of the
            // String.valueOf fallback (see class Javadoc).
            if (underlyingEventTypes.containsKey(value.getClass())) {
                return renderRow(new UnderlyingEventBean(value, underlyingEventTypes.get(value.getClass())));
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

        /** Minimal engine-independent EventBean view over a raw underlying. */
        private static final class UnderlyingEventBean implements EventBean {
            private final Object underlying;
            private final EventType eventType;

            UnderlyingEventBean(Object underlying, EventType eventType) {
                this.underlying = underlying;
                this.eventType = eventType;
            }

            public EventType getEventType() {
                return eventType;
            }

            public Object get(String propertyExpression) {
                return eventType.getGetter(propertyExpression).get(this);
            }

            public Object getUnderlying() {
                return underlying;
            }

            public Object getFragment(String propertyExpression) {
                return eventType.getGetter(propertyExpression).getFragment(this);
            }
        }
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

    /** Local mirrors of the pinned InfraNamedWindowJoin.MyLocalJsonProvided* json-provided classes (regression-lib is not on the oracle classpath). */
    public static class MyLocalJsonProvidedProduct implements Serializable {
        public String product;
        public int size;
    }

    /** Local mirrors of the pinned InfraNamedWindowJoin.MyLocalJsonProvided* json-provided classes (regression-lib is not on the oracle classpath). */
    public static class MyLocalJsonProvidedPortfolio implements Serializable {
        public String portfolio;
        public String product;
    }
}
