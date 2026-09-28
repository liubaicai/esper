import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expression;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.Filter;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.PatternExpr;
import com.espertech.esper.common.client.soda.PatternStream;
import com.espertech.esper.common.client.soda.Patterns;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportBean_C;
import com.espertech.esper.regressionlib.support.bean.SupportBean_D;
import com.espertech.esper.regressionlib.support.bean.SupportBean_E;
import com.espertech.esper.regressionlib.support.bean.SupportBean_F;
import com.espertech.esper.regressionlib.support.bean.SupportBean_G;
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
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-guard-while-590 unit: the four
 * PatternGuardWhile executions (ords 0-3, static java-2be6b5626499eeff834e,
 * flags []) replayed as four cases on one fresh runtime each.
 *
 * <ul>
 *   <li>simple (ord 0, PatternGuardWhileSimple,
 *       java-runtime-aa6c31e987ec865a8cf2): deploys
 *       {@code @Name('s0') select a.theString as c0 from pattern
 *       [(every a=SupportBean) while (a.theString like 'E%')]} and sends
 *       E1, E2, X, E3, X. E1/E2 deliver new {c0}; the first X match
 *       falsifies the while-guard permanently (ExpressionGuard.inspect
 *       returns FALSE, guardQuit + evaluateFalse to the parent), so E3
 *       and the trailing X stay silent. The env.milestone(0/1/2)
 *       savepoints are unrepresented harness machinery.</li>
 *   <li>pattern-op (ord 1, PatternOp, java-runtime-bf4b4c7d66f189e40a73):
 *       the five-leg PatternTestHarness over
 *       EventCollectionFactory.getEventSetOne(0, 1000): ONE deploy step
 *       (statement "all") expands to the five per-leg deployments of
 *       {@code @name("S<i>") select * from pattern [<atom>]} (S0..S4 stand
 *       in for the name--&lt;atom&gt; labels; leg 3 compiles through the
 *       SODA module path like the harness's model case), then twelve
 *       advance-before-send replays of the mixed set (advanceTime
 *       precedes sendEventBean) and undeploy-all with the kill-resend
 *       silence check. Per (statement, trigger-event) the harness
 *       compares the listener's last delivery as a multiset
 *       (compareLists).</li>
 *   <li>pattern-variable (ord 2, PatternVariable,
 *       java-runtime-8808f29a3bfdd475589f): deploys
 *       {@code @name('var') @public create variable boolean myVariable =
 *       true} then {@code @name('s0') select * from pattern [every
 *       a=SupportBean(theString like 'A%') -&gt; (every b=SupportBean
 *       (theString like 'B%')) while (myVariable)]} — both compiles see
 *       the accumulated module path like RegressionEnvironment's
 *       RegressionPath. B1 delivers ONE listener batch of two rows (one
 *       per live every-a branch); runtimeSetVariable("var", "myVariable",
 *       false) falsifies every live branch so A3/A4/B2 stay silent.</li>
 *   <li>pattern-invalid (ord 3, PatternInvalid,
 *       java-runtime-4416cc3367e38623077a): two tryInvalidCompile probes
 *       compiled without the runtime path record the pinned compile-error
 *       messages for the non-boolean literal guard
 *       {@code while ('abc')} and the unresolvable property guard
 *       {@code while (abc)}.</li>
 * </ul>
 */
public final class PatternGuardWhile590ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-guard-while-590";
    private static final String DESCRIPTION =
            "PatternGuardWhile ords 0-3 — the four expression-guard `while` executions "
                    + "replayed as four cases: (simple) PatternGuardWhileSimple deploys "
                    + "`@Name('s0') select a.theString as c0 from pattern [(every "
                    + "a=SupportBean) while (a.theString like 'E%')]` and sends E1,E2,X,E3,X "
                    + "— E1 and E2 fire new {c0:E1}/{c0:E2} while the first X match falsifies "
                    + "the while-guard permanently (ExpressionGuard.inspect returns FALSE -> "
                    + "guardQuit + evaluateFalse) so E3 and the trailing X stay silent; "
                    + "(pattern-op) PatternOp's five-leg PatternTestHarness over "
                    + "EventCollectionFactory.getEventSetOne(0,1000) replays as ONE deploy-all "
                    + "step (statement \"all\" expands to the five per-leg deployments of "
                    + "`@name(\"S<i>\") select * from pattern [<atom>]`, S3 carrying the SODA "
                    + "model's `select * from pattern [(every b=SupportBean_B) while "
                    + "(b.id!=\"B3\")]` toEPL text with double-quoted literal) then twelve "
                    + "advance-before-send steps (+1000ms each) and undeploy-all — S0 fires "
                    + "B1{a:A1,b:B1} then quits at B2 (b.id!='B2'), S1 fires "
                    + "B1{a:A1,b:B1}/B2{a:A1,b:B2} then quits at B3, S2 and S3 fire "
                    + "B1{b:B1}/B2{b:B2} then quit at B3, S4 stays silent (the very first B1 "
                    + "match fails b.id!='B1'); (pattern-variable) PatternVariable deploys "
                    + "`@name('var') @public create variable boolean myVariable = true` plus "
                    + "`@name('s0') select * from pattern [every a=SupportBean(theString like "
                    + "'A%') -> (every b=SupportBean(theString like 'B%')) while (myVariable)]` "
                    + "— B1 delivers ONE listener batch of two rows (a:A1,b:B1)/(a:A2,b:B1), "
                    + "one per live every-a branch — then "
                    + "runtimeSetVariable('var','myVariable',false) falsifies every live branch "
                    + "so A3/A4/B2 stay silent; (pattern-invalid) PatternInvalid's two "
                    + "tryInvalidCompile probes record the pinned compile-error messages for "
                    + "the non-boolean literal guard `while ('abc')` and the unresolvable "
                    + "property guard `while (abc)`, each compiled without the runtime path. "
                    + "env.milestone savepoints, the COMPILE_TO_MODEL/COMPILE_TO_EPL/"
                    + "consume+suppress replay styles, the @Audit('pattern')/"
                    + "@Audit('pattern-instances') annotations and the post-undeploy statement "
                    + "lookup are unrepresented harness machinery.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternGuardWhile.java";
    private static final String[] JAVA_SOURCE_FILES = {
            JAVA_SOURCE,
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "EventCollectionFactory.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "EventExpressionCase.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/patternassert/"
                    + "PatternTestHarness.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_D.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_E.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_F.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_G.java"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-aa6c31e987ec865a8cf2",
            "java-runtime-bf4b4c7d66f189e40a73",
            "java-runtime-8808f29a3bfdd475589f",
            "java-runtime-4416cc3367e38623077a"
    };
    private static final String[] EXECUTIONS = {
            "PatternGuardWhileSimple",
            "PatternOp",
            "PatternVariable",
            "PatternInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-2be6b5626499eeff834e",
            "java-2be6b5626499eeff834e",
            "java-2be6b5626499eeff834e",
            "java-2be6b5626499eeff834e"
    };

    private static final String[] CASES = {
            "simple", "pattern-op", "pattern-variable", "pattern-invalid"
    };
    private static final int[] CASE_ORDINALS = {0, 1, 2, 3};

    // Byte-exact statement texts per case (case.epls). The pattern-op leg
    // atoms are PatternGuardWhile.java lines 70-107 verbatim — leg 3 is
    // the SODA model leg whose pinned text is the model's toEPL output
    // (`while (b.id!="B3")` keeps the double-quoted literal), wrapped in
    // the harness's `select * from pattern [<atom>]` under the S0..S4
    // names that stand in for the name--<atom> labels.
    private static final String EPL_SIMPLE =
            "@Name('s0') select a.theString as c0 from pattern "
                    + "[(every a=SupportBean) while (a.theString like 'E%')]";
    private static final String[] OP_ATOMS = {
            "a=SupportBean_A -> (every b=SupportBean_B) while(b.id != 'B2')",
            "a=SupportBean_A -> (every b=SupportBean_B) while(b.id != 'B3')",
            "(every b=SupportBean_B) while(b.id != 'B3')",
            "(every b=SupportBean_B) while (b.id!=\"B3\")",
            "(every b=SupportBean_B) while(b.id != 'B1')"
    };
    private static final int SODA_LEG = 3;
    private static final String SODA_TO_EPL =
            "select * from pattern [(every b=SupportBean_B) while (b.id!=\"B3\")]";
    private static final String EPL_VARIABLE_CREATE =
            "@name('var') @public create variable boolean myVariable = true";
    private static final String EPL_VARIABLE_SELECT =
            "@name('s0') select * from pattern [every a=SupportBean(theString like 'A%') -> "
                    + "(every b=SupportBean(theString like 'B%')) while (myVariable)]";
    private static final String EPL_INVALID_LITERAL =
            "select * from pattern [every SupportBean while ('abc')]";
    private static final String EPL_INVALID_PROPERTY =
            "select * from pattern [every SupportBean while (abc)]";
    private static final String ERROR_INVALID_LITERAL =
            "Invalid parameter for pattern guard 'SupportBean while (\"abc\")': Expression "
                    + "pattern guard requires a single expression as a parameter returning a "
                    + "true or false (boolean) value "
                    + "[select * from pattern [every SupportBean while ('abc')]]";
    private static final String ERROR_INVALID_PROPERTY =
            "Failed to validate pattern guard expression 'abc': Property named 'abc' is not "
                    + "valid in any stream "
                    + "[select * from pattern [every SupportBean while (abc)]]";

    private static final String[] OP_LEG_NAMES = {"S0", "S1", "S2", "S3", "S4"};
    private static final String[] OP_CASE_EPLS;

    static {
        OP_CASE_EPLS = new String[OP_ATOMS.length];
        for (int leg = 0; leg < OP_ATOMS.length; leg++) {
            OP_CASE_EPLS[leg] = "@name(\"" + OP_LEG_NAMES[leg]
                    + "\") select * from pattern [" + OP_ATOMS[leg] + "]";
        }
    }

    private static final String[][] CASE_EPLS = {
            {EPL_SIMPLE},
            OP_CASE_EPLS,
            {EPL_VARIABLE_CREATE, EPL_VARIABLE_SELECT},
            {EPL_INVALID_LITERAL, EPL_INVALID_PROPERTY}
    };

    private static final String[] OBSERVATIONS = {
            "listener; deploy s0 then sends E1,E2,X,E3,X at the engine start instant (no "
                    + "external clock): E1 fires new {c0:E1}, E2 fires {c0:E2}, the first X "
                    + "sends' falsifying match quits the guarded every permanently so E3 and "
                    + "the second X deliver nothing; milestone(0/1/2) savepoints and "
                    + "undeployAll carry no records",
            "listener; external clock +1000ms per send; all five legs deploy in ONE step "
                    + "before any send (harness deploy-all), then twelve advance-before-send "
                    + "replays of A1,B1,C1,B2,A2,D1,E1,F1,D2,B3,G1,D3 (advanceTime precedes "
                    + "sendEventBean): S0 B1{a:A1,b:B1} only (`b.id != 'B2'` quits at B2), "
                    + "S1 B1{a:A1,b:B1}+B2{a:A1,b:B2} (`!= 'B3'` quits at B3), S2 B1{b:B1}+"
                    + "B2{b:B2}, S3 the SODA model leg `b.id!=\"B3\"` fires B1{b:B1}+B2{b:B2}, "
                    + "S4 silent (B1 falsifies `!= 'B1'` on the first match); per leg+trigger "
                    + "the fires compare as a multiset like compareLists; undeployAll resend "
                    + "silence",
            "listener; deploy var (`@public create variable boolean myVariable = true`, "
                    + "RegressionPath-scoped) then deploy s0: sends A1,A2 arm two every-a "
                    + "branches, B1 delivers ONE new batch of two rows (a:A1,b:B1)/(a:A2,b:B1) "
                    + "with the guard true; runtimeSetVariable('var','myVariable',false) "
                    + "falsifies every live branch permanently (guardQuit+evaluateFalse) so "
                    + "A3/A4/B2 deliver nothing; milestone(0/1) savepoints and undeployAll "
                    + "carry no records",
            "compile-error; two tryInvalidCompile probes compiled without the runtime path "
                    + "record the pinned Java messages: `while ('abc')` fails with 'Invalid "
                    + "parameter for pattern guard \"SupportBean while (\\\"abc\\\")\": "
                    + "Expression pattern guard requires a single expression as a parameter "
                    + "returning a true or false (boolean) value' and `while (abc)` fails "
                    + "with 'Failed to validate pattern guard expression \"abc\": Property "
                    + "named 'abc' is not valid in any stream'"
    };

    // Pinned op sequence per case (after the case marker).
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "send", "send", "undeploy-all"},
            {"deploy",
                    "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "send", "send",
                    "undeploy-all"},
            {"deploy", "deploy", "send", "send", "send", "set-variable",
                    "send", "send", "send", "undeploy-all"},
            {"build-error", "build-error"}
    };

    // Pinned deploy statement labels per case ("all" is the W-harness
    // deploy-all expander for pattern-op).
    private static final String[][] CASE_DEPLOYS = {
            {"s0"},
            {"all"},
            {"var", "s0"},
            {}
    };
    private static final String[][] CASE_DEPLOY_EPLS = {
            {EPL_SIMPLE},
            {},
            {EPL_VARIABLE_CREATE, EPL_VARIABLE_SELECT},
            {}
    };

    // Pinned sends per case: "simple"/"pattern-variable" carry
    // SupportBean(theString,intPrimitive), "pattern-op" carries the
    // EventCollectionFactory.makeMixedSet "<eventType>|<id>" sends with
    // pinned advance instants in CASE_ADVANCES.
    private static final String[][] CASE_BEAN_SENDS = {
            {"E1|0", "E2|0", "X|0", "E3|0", "X|0"},
            {},
            {"A1|1", "A2|2", "B1|100", "A3|3", "A4|4", "B2|200"},
            {}
    };
    private static final long[][] CASE_ADVANCES = {
            {},
            {1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000, 11000, 12000},
            {},
            {}
    };
    private static final String[][] CASE_MIXED_SENDS = {
            {},
            {"SupportBean_A|A1", "SupportBean_B|B1", "SupportBean_C|C1",
                    "SupportBean_B|B2", "SupportBean_A|A2", "SupportBean_D|D1",
                    "SupportBean_E|E1", "SupportBean_F|F1", "SupportBean_D|D2",
                    "SupportBean_B|B3", "SupportBean_G|G1", "SupportBean_D|D3"},
            {},
            {}
    };

    // Pinned set-variable step for pattern-variable:
    // runtimeSetVariable("var", "myVariable", false).
    private static final String[][] CASE_SET_VARIABLES = {
            {},
            {},
            {"var|myVariable|false"},
            {}
    };

    // Pinned build-error probes for pattern-invalid:
    // "<statement>|<epl>|<expectError>" — tryInvalidCompile compiles
    // without the runtime path.
    private static final String[][] CASE_BUILD_ERRORS = {
            {},
            {},
            {},
            {"non-boolean-literal-guard|" + EPL_INVALID_LITERAL + "|" + ERROR_INVALID_LITERAL,
                    "unresolvable-property-guard|" + EPL_INVALID_PROPERTY + "|"
                            + ERROR_INVALID_PROPERTY}
    };

    private static final int EXPECTED_STEPS = 37;
    private static final int EXPECTED_RECORDS = 12;

    private PatternGuardWhile590ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternGuardWhile590ScenarioOracle <scenario.json>");
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
        switch (caseName) {
            case "simple":
            case "pattern-variable":
            case "pattern-invalid":
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case "pattern-op":
                configuration.getCommon().addEventType(SupportBean_A.class);
                configuration.getCommon().addEventType(SupportBean_B.class);
                configuration.getCommon().addEventType(SupportBean_C.class);
                configuration.getCommon().addEventType(SupportBean_D.class);
                configuration.getCommon().addEventType(SupportBean_E.class);
                configuration.getCommon().addEventType(SupportBean_F.class);
                configuration.getCommon().addEventType(SupportBean_G.class);
                break;
            default:
                throw new IllegalStateException("unknown case " + caseName);
        }
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            boolean active = false;
            int deployIndex = 0;
            // The accumulated module path mirrors the regression suite's
            // RegressionPath: the pattern-variable "var" deployment's
            // public variable resolves in the s0 compile.
            List<EPCompiled> path = new ArrayList<>();
            Map<String, String> deploymentIds = new HashMap<>();
            List<String> legDeployments = new ArrayList<>();
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
                        if ("all".equals(label)) {
                            // The pattern-op deploy-all expands to the
                            // harness's five per-leg deployments —
                            // statement names S0..S4, one listener each,
                            // like env.deploy(unit).addListener(name).
                            for (int leg = 0; leg < OP_CASE_EPLS.length; leg++) {
                                EPCompiled compiled = compileOpLeg(runtime, configuration, leg);
                                EPDeployment deployment = runtime.getDeploymentService().deploy(
                                        compiled, new DeploymentOptions()
                                                .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                                        + "-" + (deployIndex++)));
                                legDeployments.add(deployment.getDeploymentId());
                                EPStatement statement = requireStatement(
                                        deployment, OP_LEG_NAMES[leg]);
                                statement.addListener(new TraceWriter(
                                        records, caseName, statement, runtime));
                            }
                            break;
                        }
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = compileWithPath(configuration, path, epl);
                        path.add(compiled);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(
                                compiled, new DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                                + "-" + (deployIndex++)));
                        deploymentIds.put(label, deployment.getDeploymentId());
                        // Only the select statement carries a listener
                        // (env.addListener("s0")); the create-variable
                        // deployment declares the public variable and has
                        // no subscriber surface.
                        if (!"var".equals(label)) {
                            EPStatement statement = requireStatement(deployment, label);
                            statement.addListener(new TraceWriter(
                                    records, caseName, statement, runtime));
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        // The pattern-op harness resends the whole set
                        // after undeployAll to prove silence and asserts
                        // the statements resolve to null.
                        for (String send : CASE_MIXED_SENDS[caseIndex]) {
                            resendSilently(runtime, send);
                        }
                        for (String deploymentId : legDeployments) {
                            for (String legName : OP_LEG_NAMES) {
                                if (runtime.getDeploymentService().getStatement(
                                        deploymentId, legName) != null) {
                                    throw new IllegalStateException(
                                            "statement " + legName + " survived undeploy-all");
                                }
                            }
                        }
                        break;
                    case "send":
                        String at = step.getString("at", "");
                        if (!at.isEmpty()) {
                            runtime.getEventService().advanceTime(
                                    Instant.parse(at).toEpochMilli());
                        }
                        sendEvent(runtime, step);
                        break;
                    case "set-variable": {
                        // runtimeSetVariable(statementName, name, value):
                        // the label resolves to the deployment that owns
                        // the public variable.
                        String label = step.getString("statement", "");
                        String deploymentId = deploymentIds.get(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException(
                                    "set-variable target " + label + " was not deployed");
                        }
                        JsonValue payload = step.get("payload");
                        Object value;
                        if (payload != null && payload.isBoolean()) {
                            value = payload.asBoolean();
                        } else {
                            throw new IllegalStateException(
                                    "unsupported set-variable payload " + payload);
                        }
                        runtime.getVariableService().setVariableValue(
                                deploymentId, step.getString("name", ""), value);
                        break;
                    }
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    // compileWithPath mirrors RegressionEnvironmentBase
    // .compileWCheckedEx(epl, path): the compiler sees the
    // configuration plus the accumulated compiled modules (the
    // RegressionPath) so the pattern-variable s0 deployment resolves
    // the "var" module's public variable — adding the runtime path too
    // would double-register the variable.
    private static EPCompiled compileWithPath(Configuration configuration,
                                              List<EPCompiled> path, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().addAll(path);
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    // compileOpLeg mirrors the harness's per-case deployment: legs other
    // than S3 compile the pinned `@name("S<i>") select * from pattern
    // [<atom>]` text while S3 compiles the SODA model leg — the harness
    // pins `select * from pattern [(every b=SupportBean_B) while
    // (b.id!="B3")]` as the model's toEPL output.
    private static EPCompiled compileOpLeg(EPRuntime runtime, Configuration configuration,
                                           int leg) throws Exception {
        if (leg == SODA_LEG) {
            EPStatementObjectModel model = new EPStatementObjectModel();
            model.setSelectClause(SelectClause.createWildcard());
            model = SerializableObjectCopier.copyMayFail(model);
            Expression guardExpr = Expressions.neq("b.id", "B3");
            PatternExpr every = Patterns.every(
                    Patterns.filter(Filter.create("SupportBean_B"), "b"));
            PatternExpr patternGuarded = Patterns.whileGuard(every, guardExpr);
            model.setFromClause(FromClause.create(PatternStream.create(patternGuarded)));
            if (!SODA_TO_EPL.equals(model.toEPL())) {
                throw new IllegalStateException(
                        "SODA leg toEPL is not pinned: " + model.toEPL());
            }
            model.setAnnotations(Collections.singletonList(
                    AnnotationPart.nameAnnotation(OP_LEG_NAMES[SODA_LEG])));
            Module module = new Module();
            module.getItems().add(new ModuleItem(model));
            module.setModuleText(model.toEPL());
            return EPCompilerProvider.getCompiler().compile(
                    module, new CompilerArguments(runtime.getRuntimePath()));
        }
        return EPCompilerProvider.getCompiler().compile(
                OP_CASE_EPLS[leg], new CompilerArguments(runtime.getRuntimePath()));
    }

    // buildErrorStep mirrors env.tryInvalidCompile(epl, message): the
    // probe compiles without the runtime path (PatternInvalid's
    // tryInvalidCompile calls compileWCheckedEx path-less), the caught
    // message must start with the pinned expectError
    // (SupportMessageAssertUtil.assertMessage), and the record carries
    // the pinned prefix.
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        String expected = step.getString("expectError", "");
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(
                    epl, new CompilerArguments(configuration));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        records.add(new JsonObject()
                .add("case", caseName)
                .add("operation", "compile-error")
                .add("statement", label)
                .add("sequence", 0)
                .add("value", expected));
    }

    private static EPStatement requireStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static void resendSilently(EPRuntime runtime, String pinned) {
        String eventType = pinned.substring(0, pinned.indexOf('|'));
        String id = pinned.substring(pinned.indexOf('|') + 1);
        sendTyped(runtime, eventType, id);
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportBean(payload.getString("theString", ""),
                            payload.getInt("intPrimitive", 0)), "SupportBean");
            return;
        }
        sendTyped(runtime, eventType, payload.getString("id", null));
    }

    private static void sendTyped(EPRuntime runtime, String eventType, String id) {
        switch (eventType) {
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(new SupportBean_A(id), "SupportBean_A");
                return;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(new SupportBean_B(id), "SupportBean_B");
                return;
            case "SupportBean_C":
                runtime.getEventService().sendEventBean(new SupportBean_C(id), "SupportBean_C");
                return;
            case "SupportBean_D":
                runtime.getEventService().sendEventBean(new SupportBean_D(id), "SupportBean_D");
                return;
            case "SupportBean_E":
                runtime.getEventService().sendEventBean(new SupportBean_E(id), "SupportBean_E");
                return;
            case "SupportBean_F":
                runtime.getEventService().sendEventBean(new SupportBean_F(id), "SupportBean_F");
                return;
            case "SupportBean_G":
                runtime.getEventService().sendEventBean(new SupportBean_G(id), "SupportBean_G");
                return;
            default:
                throw new IllegalStateException("unknown eventType: " + eventType);
        }
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
                    "observation", "epls");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
            validateStringArray(definition.get("epls"), CASE_EPLS[index], "epls");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * pinned ops — deploy/send/set-variable/undeploy-all for the runtime
     * cases and the two build-error probes for pattern-invalid. Unknown
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
            if (!"case".equals(string(marker, "op"))
                    || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int deploys = 0;
            int sends = 0;
            int setVariables = 0;
            int buildErrors = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy": {
                        String label = CASE_DEPLOYS[caseIndex][deploys];
                        if ("all".equals(label)) {
                            requireFields(step, "op", "case", "statement");
                        } else {
                            requireFields(step, "op", "case", "statement", "epl");
                            if (!CASE_DEPLOY_EPLS[caseIndex][deploys]
                                    .equals(string(step, "epl"))) {
                                throw new IllegalArgumentException(
                                        "deploy step " + cursor + " epl is not pinned");
                            }
                        }
                        if (!label.equals(string(step, "statement"))) {
                            throw new IllegalArgumentException(
                                    "deploy step " + cursor + " is not pinned");
                        }
                        deploys++;
                        break;
                    }
                    case "send": {
                        if (caseIndex == 1) {
                            // pattern-op: fused advance-before-send with
                            // pinned instant and mixed-set payload.
                            requireFields(step, "op", "case", "at", "eventType", "payload");
                            long expected = CASE_ADVANCES[caseIndex][sends];
                            long actual = Instant.parse(string(step, "at")).toEpochMilli();
                            if (actual != expected) {
                                throw new IllegalArgumentException("send step " + cursor
                                        + " advance instant is not pinned: expected "
                                        + expected + " got " + actual);
                            }
                            String expectedSend = CASE_MIXED_SENDS[caseIndex][sends];
                            JsonObject payload = object(step.get("payload"),
                                    "send payload " + cursor);
                            requireFields(payload, "id");
                            String eventType = string(step, "eventType");
                            String actualSend = eventType + "|" + string(payload, "id");
                            if (!expectedSend.equals(actualSend)) {
                                throw new IllegalArgumentException("send payload " + cursor
                                        + " is not pinned: expected " + expectedSend
                                        + " got " + actualSend);
                            }
                        } else {
                            // simple/pattern-variable: SupportBean sends
                            // carry theString|intPrimitive.
                            requireFields(step, "op", "case", "eventType", "payload");
                            String expectedSend = CASE_BEAN_SENDS[caseIndex][sends];
                            JsonObject payload = object(step.get("payload"),
                                    "send payload " + cursor);
                            requireFields(payload, "theString", "intPrimitive");
                            String actualSend = string(step, "eventType") + "|"
                                    + string(payload, "theString") + "|"
                                    + integer(payload, "intPrimitive");
                            if (!("SupportBean|" + expectedSend).equals(actualSend)) {
                                throw new IllegalArgumentException("send payload " + cursor
                                        + " is not pinned: expected SupportBean|"
                                        + expectedSend + " got " + actualSend);
                            }
                        }
                        sends++;
                        break;
                    }
                    case "set-variable": {
                        requireFields(step, "op", "case", "statement", "name", "payload");
                        String[] parts = CASE_SET_VARIABLES[caseIndex][setVariables].split("\\|");
                        JsonValue payload = step.get("payload");
                        if (!parts[0].equals(string(step, "statement"))
                                || !parts[1].equals(string(step, "name"))
                                || payload == null || !payload.isBoolean()
                                || payload.asBoolean() != Boolean.parseBoolean(parts[2])) {
                            throw new IllegalArgumentException(
                                    "set-variable step " + cursor + " is not pinned");
                        }
                        setVariables++;
                        break;
                    }
                    case "build-error": {
                        requireFields(step, "op", "case", "statement", "epl",
                                "expectError");
                        String[] parts = CASE_BUILD_ERRORS[caseIndex][buildErrors].split("\\|", 3);
                        if (!parts[0].equals(string(step, "statement"))
                                || !parts[1].equals(string(step, "epl"))
                                || !parts[2].equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException(
                                    "build-error step " + cursor + " is not pinned");
                        }
                        buildErrors++;
                        break;
                    }
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (deploys != CASE_DEPLOYS[caseIndex].length
                    || sends != CASE_BEAN_SENDS[caseIndex].length
                            + CASE_MIXED_SENDS[caseIndex].length
                    || setVariables != CASE_SET_VARIABLES[caseIndex].length
                    || buildErrors != CASE_BUILD_ERRORS[caseIndex].length) {
                throw new IllegalArgumentException(
                        "case " + caseIndex + " step counts are not pinned");
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
        JsonValue value = object.get(name);
        if (!(value instanceof com.espertech.esper.common.client.json.minimaljson.JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
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

        private TraceWriter(JsonArray records, String caseName, EPStatement statement,
                            EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
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
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String name : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            values.add(name, normalize(event.get(name)));
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
                array.add(normalize(event.getUnderlying()));
            }
            return array;
        }
        if (value instanceof EventBean) {
            return normalize(((EventBean) value).getUnderlying());
        }
        if (value instanceof SupportBean) {
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(((SupportBean) value).getTheString()));
            fields.add("intPrimitive", normalize(((SupportBean) value).getIntPrimitive()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_A) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_A) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_B) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_B) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_C) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_C) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_D) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_D) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_E) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_E) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_F) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_F) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBean_G) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((SupportBean_G) value).getId()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Iterable<?>) {
            JsonArray array = new JsonArray();
            for (Object item : (Iterable<?>) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            TreeSet<String> keys = new TreeSet<>();
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

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
