import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;

import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the epl-other-create-expression parity
 * scenario. Mirrors EPLOtherCreateExpression: EPLOtherInvalid (ord 0, the
 * duplicate declared-expression compile-error; the js:abc script deploy and
 * its same-arity duplicate rejection verified but unrepresentable),
 * EPLOtherParseSpecialAndMixedExprAndScript (ord 1, the declared-expression
 * halves: myexpr concat select narrowed to the representable c1 column, and
 * the scalarfilter enum chain whose stateless flag is asserted in-process;
 * the js:callIt bean-factory select verified but unrepresentable),
 * EPLOtherExprAndScriptLifecycleAndFilter (ord 2, the declared-expression
 * filter pass: deploy MyFilter=1, filtered select, undeployAll, redeploy =2,
 * rebind proof; the js:MyFilter pass verified but unrepresentable),
 * EPLOtherScriptUse (ord 3, intentionally-different: script overloading by
 * arity plus SODA EPL round-trips verified in-process), and
 * EPLOtherExpressionUse (ord 4, parts A-D run once per case: TwoPi/factorPi
 * selects, the statement-local TwoPi shadow, the JoinMultiplication deploy,
 * and the deferred subquery myexpr over MyInfra as a keepall named window
 * and as a no-key table with the stateful flag asserted in-process).
 *
 * <p>Each case runs on a fresh runtime (each Java execution gets its own;
 * every execution ends with undeployAll). Every deploy step compiles one
 * single-statement module exactly like the source's per-call compileDeploy.
 * Deployed markers emit one record per statement label. Listener records
 * carry the new-data rows. The build-error step compiles the pinned probe
 * and requires the failure message to start with the pinned prefix. The
 * unrepresentable steps verify the Java surface in-process (script
 * deployment and duplicate rejection, isStatelessSelect flags, the callIt
 * select values, the script lifecycle pass, the abc overloads and the SODA
 * EPL property) before emitting the pinned note record.
 */
public final class EPLOtherCreateExpressionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-create-expression";
    private static final String DESCRIPTION =
            "EPLOtherCreateExpression slice (ords 0-4): invalid replays the duplicate"
                    + " declared-expression compile-error (script duplicate-by-arity"
                    + " unrepresentable); parse-mixed-expr replays the declared-expression"
                    + " halves (myexpr concat select, scalarfilter enum chain with the"
                    + " stateless flag pinned as unrepresentable, js:callIt"
                    + " unrepresentable); lifecycle-filter replays the declared-expression"
                    + " filter pass (deploy MyFilter=1, filtered select, undeploy-all,"
                    + " redeploy =2, rebind proof; the js:MyFilter pass unrepresentable);"
                    + " script-use is intentionally-different (script overloading by arity"
                    + " plus SODA round-trips); expression-use replays parts A-D over a"
                    + " keepall named window and a no-key table (TwoPi/factorPi selects,"
                    + " statement-local TwoPi shadow, JoinMultiplication deploy, deferred"
                    + " subquery myexpr over MyInfra with the stateful flag pinned as"
                    + " unrepresentable). Deployed markers pin the module fan-out,"
                    + " listener records carry the new-data rows, compile-error records"
                    + " carry the pinned Java message, and unrepresentable records pin"
                    + " the Java surfaces with no Go boundary (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/epl/other/EPLOtherCreateExpression.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/"
                    + "EPLOtherCreateExpression.java";

    private static final String[] CASES = {
            "invalid", "parse-mixed-expr", "lifecycle-filter", "script-use",
            "expression-use-nw", "expression-use-table"};
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-e3078c54652f7f72cd73",
            "java-runtime-2e66cbcd61d52f258cb4",
            "java-runtime-c0c5df502bf2651e6acf",
            "java-runtime-c2595ccced4ddaea854d",
            "java-runtime-61a0f3f3916ac25443d7",
            "java-runtime-61a0f3f3916ac25443d7",
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherInvalid",
            "EPLOtherParseSpecialAndMixedExprAndScript",
            "EPLOtherExprAndScriptLifecycleAndFilter",
            "EPLOtherScriptUse",
            "EPLOtherExpressionUse{namedWindow=true}",
            "EPLOtherExpressionUse{namedWindow=false}",
    };
    private static final String[] STATIC_IDS = {
            "java-683caba87edd0bf7ecea",
            "java-dd3e693c0a24401dc305",
            "java-e292f8a1c482bb84b1e4",
            "java-ae1621ba24392298ec2f",
            "java-089123ca8c5a8e4a85b6",
            "java-089123ca8c5a8e4a85b6",
    };

    // Transcriptions of EPLOtherCreateExpression.java lines 47-58 (invalid),
    // 68-95 (parse-mixed), 207-214 (lifecycle), 106-128 (script-use) and
    // 138-193 (expression-use).
    private static final String EPL_E1 =
            "@name('s0') @public create expression E1 {''}";
    private static final String EPL_E1_DUP = "create expression E1 {''}";
    private static final String EPL_ABC_TWO =
            "@public create expression int js:abc(p1, p2) [p1*p2]";
    private static final String EPL_ABC_DUP =
            "create expression int js:abc(a, a) [p1*p2]";

    private static final String EPL_MYSCRIPT =
            "@public create expression string js:myscript(p1) [\"--\"+p1+\"--\"]";
    private static final String EPL_MYEXPR =
            "@public create expression myexpr {sb => '--'||theString||'--'}";
    private static final String EPL_SELECT_MYEXPR =
            "@name('s0') select myexpr(sb) as c1 from SupportBean as sb";
    private static final String EPL_SCALARFILTER =
            "@public create expression scalarfilter {s =>    strvals.where(y => y != 'E1') }";
    private static final String EPL_SELECT_SCALAR =
            "@name('s0') select scalarfilter(t).where(x => x != 'E2') as val1"
                    + " from SupportCollection as t";
    private static final String EPL_CALLIT =
            "@public create expression com.espertech.esper.common.internal.support.SupportBean"
                    + " js:callIt() [ new"
                    + " com.espertech.esper.common.internal.support.SupportBean('E1', 10); ]";
    private static final String EPL_SELECT_CALLIT =
            "@name('s0') select callIt() as val0, callIt().getTheString() as val1"
                    + " from SupportBean as sb";

    private static final String EPL_EXPR_ONE =
            "@name('expr-one') @public create expression MyFilter {sb => intPrimitive = 1}";
    private static final String EPL_EXPR_TWO =
            "@name('expr-two') @public create expression MyFilter {sb => intPrimitive = 2}";
    private static final String EPL_S1 =
            "@name('s1') select * from SupportBean(MyFilter(sb)) as sb";
    private static final String EPL_S2 =
            "@name('s2') select * from SupportBean(MyFilter(sb)) as sb";
    private static final String EPL_SCRIPT_ONE =
            "@name('expr-one') @public create expression boolean js:MyFilter(intPrimitive)"
                    + " [intPrimitive==1]";
    private static final String EPL_SCRIPT_TWO =
            "@name('expr-two') @public create expression boolean js:MyFilter(intPrimitive)"
                    + " [intPrimitive==2]";
    private static final String EPL_SCRIPT_S1 =
            "@name('s1') select * from SupportBean(MyFilter(intPrimitive)) as sb";
    private static final String EPL_SCRIPT_S2 =
            "@name('s2') select * from SupportBean(MyFilter(intPrimitive)) as sb";

    private static final String EPL_ABC_TWO_TEN =
            "@public create expression int js:abc(p1, p2) [p1*p2*10]";
    private static final String EPL_ABC_ONE_TEN =
            "@public create expression int js:abc(p1) [p1*10]";
    private static final String EPL_SELECT_ABC =
            "@name('s0') select abc(intPrimitive, doublePrimitive) as c0,"
                    + " abc(intPrimitive) as c1 from SupportBean";
    private static final String EPL_SOMESCRIPT =
            "@name('expr') @public create expression somescript(i1) ['a']";
    private static final String EPL_SELECT_SOMESCRIPT =
            "@name('select') select somescript(1) from SupportBean";

    private static final String EPL_TWOPI =
            "@public create expression TwoPi {Math.PI * 2}";
    private static final String EPL_FACTORPI =
            "@public create expression factorPi {sb => Math.PI * intPrimitive}";
    private static final String EPL_SELECT_TWOPI =
            "@name('s0') select TwoPi() as c0,(select TwoPi() from SupportBean_S0#lastevent)"
                    + " as c1,factorPi(sb) as c2 from SupportBean sb";
    private static final String EPL_SELECT_LOCAL =
            "@name('s0') expression TwoPi {Math.PI * 10} select TwoPi() as c0 from SupportBean";
    private static final String EPL_JOIN_EXPR =
            "@name('expr') @public create expression JoinMultiplication"
                    + " {(s1,s2) => s1.intPrimitive*s2.id}";
    private static final String EPL_JOIN_SELECT =
            "@name('join') select JoinMultiplication(sb,s0) from SupportBean#lastevent as sb,"
                    + " SupportBean_S0#lastevent as s0";
    private static final String EPL_MYEXPR_INFRA =
            "@public create expression myexpr {(select intPrimitive from MyInfra)}";
    private static final String EPL_CREATE_NW =
            "@public create window MyInfra#keepall as SupportBean";
    private static final String EPL_CREATE_TBL =
            "@public create table MyInfra(theString string, intPrimitive int)";
    private static final String EPL_INSERT_INFRA =
            "insert into MyInfra select theString, intPrimitive from SupportBean";
    private static final String EPL_SELECT_INFRA =
            "@name('s0') select myexpr() as c0 from SupportBean_S0";

    private static final String[] CASE_EPLS = {
            EPL_E1 + ";\n" + EPL_E1_DUP + ";\n" + EPL_ABC_TWO + ";\n" + EPL_ABC_DUP + ";\n",
            EPL_MYSCRIPT + ";\n" + EPL_MYEXPR + ";\n" + EPL_SELECT_MYEXPR + ";\n"
                    + EPL_SCALARFILTER + ";\n" + EPL_SELECT_SCALAR + ";\n" + EPL_CALLIT
                    + ";\n" + EPL_SELECT_CALLIT + ";\n",
            EPL_EXPR_ONE + ";\n" + EPL_S1 + ";\n" + EPL_EXPR_TWO + ";\n" + EPL_S2 + ";\n"
                    + EPL_SCRIPT_ONE + ";\n" + EPL_SCRIPT_S1 + ";\n" + EPL_SCRIPT_TWO + ";\n"
                    + EPL_SCRIPT_S2 + ";\n",
            EPL_ABC_TWO_TEN + ";\n" + EPL_ABC_ONE_TEN + ";\n" + EPL_SELECT_ABC + ";\n"
                    + EPL_SOMESCRIPT + ";\n" + EPL_SELECT_SOMESCRIPT + ";\n",
            EPL_TWOPI + ";\n" + EPL_FACTORPI + ";\n" + EPL_SELECT_TWOPI + ";\n"
                    + EPL_SELECT_LOCAL + ";\n" + EPL_JOIN_EXPR + ";\n" + EPL_JOIN_SELECT
                    + ";\n" + EPL_MYEXPR_INFRA + ";\n" + EPL_CREATE_NW + ";\n"
                    + EPL_INSERT_INFRA + ";\n" + EPL_SELECT_INFRA + ";\n",
            EPL_TWOPI + ";\n" + EPL_FACTORPI + ";\n" + EPL_SELECT_TWOPI + ";\n"
                    + EPL_SELECT_LOCAL + ";\n" + EPL_JOIN_EXPR + ";\n" + EPL_JOIN_SELECT
                    + ";\n" + EPL_MYEXPR_INFRA + ";\n" + EPL_CREATE_TBL + ";\n"
                    + EPL_INSERT_INFRA + ";\n" + EPL_SELECT_INFRA + ";\n",
    };

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+compile-error+unrepresentable; @public create expression E1 {''}"
                    + " deploys, then tryInvalidCompile rejects the redeclared E1 with"
                    + " 'Expression 'E1' has already been declared'; the js:abc script"
                    + " deploy and its same-arity duplicate rejection (name+parameter-count"
                    + " identity) have no Go script boundary",
            "deployed+listener+unrepresentable; myexpr {sb => '--'||theString||'--'} feeds"
                    + " select myexpr(sb) as c1 emitting {c1='--E1--'} on SupportBean(E1,1)"
                    + " (the Java select also projects myscript('x') as c0,"
                    + " unrepresentable); scalarfilter {s => strvals.where(y => y != 'E1')}"
                    + " chained .where(x => x != 'E2') over SupportCollection('E1,E2,E3,E4')"
                    + " emits val1=[E3,E4] and Java asserts s0 stateless; the js:callIt"
                    + " bean-factory select is unrepresentable",
            "deployed+listener+unrepresentable; MyFilter {sb => intPrimitive = 1} filters"
                    + " select * from SupportBean(MyFilter(sb)): E1/0 silent, E2/1 fires"
                    + " the full bean row; undeploy-all then expr-two redeploys MyFilter = 2"
                    + " and s2 rebinds: E3/0 and E4/1 silent, E4/2 fires; the js:MyFilter"
                    + " script pass is unrepresentable",
            "unrepresentable; Java overloads js:abc by arity (p1*p2*10 vs p1*10), coerces"
                    + " the intPrimitive/doublePrimitive arguments, and SODA round-trips"
                    + " the somescript create-expression and select statements; no Go"
                    + " script or SODA boundary exists",
            "deployed+listener+unrepresentable; TwoPi/factorPi feed select TwoPi(),"
                    + " (select TwoPi() from SupportBean_S0#lastevent), factorPi(sb)"
                    + " emitting [2pi,2pi,3pi] on SupportBean(E1,3) after S0(10);"
                    + " statement-local TwoPi {Math.PI * 10} shadows the env registration"
                    + " emitting c0=10pi; JoinMultiplication deploys over two #lastevent"
                    + " streams; myexpr {(select intPrimitive from MyInfra)} binds the"
                    + " keepall window deferred and emits c0=100 on S0(1,'E1') with s0"
                    + " asserted stateful",
            "deployed+listener+unrepresentable; TwoPi/factorPi feed select TwoPi(),"
                    + " (select TwoPi() from SupportBean_S0#lastevent), factorPi(sb)"
                    + " emitting [2pi,2pi,3pi] on SupportBean(E1,3) after S0(10);"
                    + " statement-local TwoPi {Math.PI * 10} shadows the env registration"
                    + " emitting c0=10pi; JoinMultiplication deploys over two #lastevent"
                    + " streams; myexpr {(select intPrimitive from MyInfra)} binds the"
                    + " no-key table deferred and emits c0=100 on S0(1,'E1') with s0"
                    + " asserted stateful",
    };

    private static final String NOTE_SCRIPT_DUP =
            "Java deploys @public create expression int js:abc(p1, p2) [p1*p2] then rejects"
                    + " the same-arity redeclare with 'Script 'abc' that takes the same"
                    + " number of parameters has already been declared'; no Go script"
                    + " boundary";
    private static final String NOTE_STATELESS_S0 =
            "Java asserts s0 isStatelessSelect=true for the chained scalarfilter select;"
                    + " the Go engine exposes no stateless-statement introspection";
    private static final String NOTE_CALLIT =
            "Java deploys js:callIt() returning a new SupportBean('E1',10) and selects"
                    + " callIt()/callIt().getTheString() emitting"
                    + " {val0.theString='E1',val0.intPrimitive=10,val1='E1'}; no Go script"
                    + " boundary";
    private static final String NOTE_SCRIPT_FILTER =
            "Java repeats the lifecycle for boolean js:MyFilter(intPrimitive) scripts"
                    + " (deploy =1, filtered select, undeploy-all, redeploy =2, rebind"
                    + " proof); no Go script boundary";
    private static final String NOTE_SCRIPT_USE =
            "Java overloads js:abc by arity (p1*p2*10 vs p1*10) emitting {c0=350,c1=100}"
                    + " on makeBean(10,3.5), then SODA round-trips @name('expr') create"
                    + " expression somescript(i1) ['a'] and @name('select') select"
                    + " somescript(1) from SupportBean; no Go script or SODA boundary";
    private static final String NOTE_STATEFUL_NW =
            "Java asserts s0 isStatelessSelect=false for the myexpr subquery select over"
                    + " named window MyInfra; the Go engine exposes no stateless-statement"
                    + " introspection";
    private static final String NOTE_STATEFUL_TBL =
            "Java asserts s0 isStatelessSelect=false for the myexpr subquery select over"
                    + " table MyInfra; the Go engine exposes no stateless-statement"
                    + " introspection";

    private static final int[] EXPECTED_CASE_RECORDS = {3, 8, 7, 1, 14, 14};
    private static final int EXPECTED_RECORDS = 47;
    private static final int EXPECTED_STEPS = 101;

    private EPLOtherCreateExpressionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherCreateExpressionScenarioOracle <scenario.json>");
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
        for (int index = 0; index < CASES.length; index++) {
            int before = records.size();
            runCase(index, allSteps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASES[index] + " emitted "
                        + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
            }
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

    /** Replays one case's steps on a fresh runtime (one runtime per Java execution). */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        Map<String, Object> collectionSchema = new LinkedHashMap<>();
        collectionSchema.put("strvals", String[].class);
        configuration.getCommon().addEventType("SupportCollection", collectionSchema);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        Map<String, EPDeployment> deploymentsByName = new HashMap<>();
        try {
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
                    case "deploy": {
                        String statementName = string(step, "statement");
                        CompilerArguments compilerArgs =
                                new CompilerArguments(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("case " + caseName + " deploy "
                                    + statementName + " produced " + deployed.length
                                    + " statements");
                        }
                        statementsByName.put(statementName, deployed[0]);
                        deploymentsByName.put(statementName, deployment);
                        if (listened(caseName, statementName)) {
                            String label = statementName;
                            deployed[0].addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statementsByName.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
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
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "build-error": {
                        String statementName = string(step, "statement");
                        String epl = string(step, "epl");
                        String prefix = string(step, "expectError");
                        // tryInvalidCompile: compile the pinned probe text
                        // against the runtime path and require the failure
                        // message to start with the pinned prefix.
                        try {
                            CompilerArguments compilerArgs =
                                    new CompilerArguments(runtime.getRuntimePath());
                            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                            throw new IllegalStateException("build-error probe "
                                    + statementName + " unexpectedly compiled in case "
                                    + caseName);
                        } catch (Exception ex) {
                            if (ex instanceof IllegalStateException) {
                                throw ex;
                            }
                            String message = ex.getMessage() == null ? "" : ex.getMessage();
                            if (!message.startsWith(prefix)) {
                                throw new IllegalStateException("build-error probe "
                                        + statementName + " message " + message
                                        + " does not start with the pinned prefix " + prefix);
                            }
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "compile-error");
                        record.add("statement", statementName);
                        record.add("sequence", 0);
                        record.add("value", prefix);
                        records.add(record);
                        break;
                    }
                    case "undeploy": {
                        String label = string(step, "statement");
                        EPDeployment deployment = deploymentsByName.remove(label);
                        if (deployment == null) {
                            throw new IllegalStateException("undeploy of unknown statement "
                                    + label + " in case " + caseName);
                        }
                        runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                        statementsByName.remove(label);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        deploymentsByName.clear();
                        break;
                    case "unrepresentable":
                        unrepresentableStep(caseName, step, runtime, sequences, records);
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
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

    /** Statements that carry the recording listener per case. */
    private static boolean listened(String caseName, String statement) {
        switch (caseName) {
            case "parse-mixed-expr":
                return "s0".equals(statement);
            case "lifecycle-filter":
                return "s1".equals(statement) || "s2".equals(statement);
            case "expression-use-nw":
            case "expression-use-table":
                return "s0".equals(statement);
            default:
                return false;
        }
    }

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                if (payload.get("theString") != null) {
                    bean.setTheString(payload.get("theString").asString());
                }
                if (payload.get("intPrimitive") != null) {
                    bean.setIntPrimitive(integer(payload, "intPrimitive"));
                }
                if (payload.get("doublePrimitive") != null) {
                    bean.setDoublePrimitive(payload.get("doublePrimitive").asDouble());
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                int id = integer(payload, "id");
                SupportBean_S0 bean = payload.get("p00") != null
                        ? new SupportBean_S0(id, payload.get("p00").asString())
                        : new SupportBean_S0(id);
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportCollection": {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("strvals", string(payload, "strvals").split(","));
                runtime.getEventService().sendEventMap(event, type);

                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /**
     * Verifies the Java surface the unrepresentable step documents, then
     * emits the pinned note record. Each label replays the Java execution's
     * unrepresentable half in-process: the script deploy plus same-arity
     * duplicate rejection, the isStatelessSelect flags, the callIt select
     * values, the script lifecycle pass, and the abc overloads plus SODA
     * EPL round-trip.
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            EPRuntime runtime, Map<String, Integer> sequences,
                                            JsonArray records) throws Exception {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        switch (label) {
            case "script-duplicate": {
                if (!NOTE_SCRIPT_DUP.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                compileDeploy(runtime, EPL_ABC_TWO);
                try {
                    CompilerArguments args = new CompilerArguments(runtime.getRuntimePath());
                    EPCompilerProvider.getCompiler().compile(EPL_ABC_DUP, args);
                    throw new IllegalStateException(
                            "same-arity script redeclare unexpectedly compiled");
                } catch (Exception ex) {
                    if (ex instanceof IllegalStateException) {
                        throw ex;
                    }
                    String message = ex.getMessage() == null ? "" : ex.getMessage();
                    if (!message.startsWith("Script 'abc' that takes the same number of"
                            + " parameters has already been declared")) {
                        throw new IllegalStateException(
                                "script duplicate message drift: " + message);
                    }
                }
                break;
            }
            case "stateless-s0": {
                if (!NOTE_STATELESS_S0.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                assertStateless(runtime, "s0", true);
                break;
            }
            case "stateful-s0": {
                String want = "expression-use-table".equals(caseName)
                        ? NOTE_STATEFUL_TBL : NOTE_STATEFUL_NW;
                if (!want.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                assertStateless(runtime, "s0", false);
                break;
            }
            case "script-callit": {
                if (!NOTE_CALLIT.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                compileDeploy(runtime, EPL_CALLIT);
                EPStatement select = compileDeploy(runtime, EPL_SELECT_CALLIT);
                List<EventBean> seen = new ArrayList<>();
                select.addListener((newEvents, oldEvents, statement, epRuntime) -> {
                    if (newEvents != null) {
                        seen.addAll(Arrays.asList(newEvents));
                    }
                });
                runtime.getEventService().sendEventBean(new SupportBean(), "SupportBean");
                if (seen.size() != 1) {
                    throw new IllegalStateException("callIt select emitted " + seen.size()
                            + " rows, expected 1");
                }
                EventBean row = seen.get(0);
                SupportBean val0 = (SupportBean) row.get("val0");
                if (val0 == null || !"E1".equals(val0.getTheString())
                        || val0.getIntPrimitive() != 10
                        || !"E1".equals(row.get("val1"))) {

                    throw new IllegalStateException("callIt select values drifted");
                }
                break;
            }
            case "script-filter": {
                if (!NOTE_SCRIPT_FILTER.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                compileDeploy(runtime, EPL_SCRIPT_ONE);
                EPStatement s1 = compileDeploy(runtime, EPL_SCRIPT_S1);
                List<EventBean> s1Rows = new ArrayList<>();
                s1.addListener((newEvents, oldEvents, statement, epRuntime) -> {
                    if (newEvents != null) {
                        s1Rows.addAll(Arrays.asList(newEvents));
                    }
                });
                sendBean(runtime, "E1", 0);
                sendBean(runtime, "E2", 1);
                if (s1Rows.size() != 1) {
                    throw new IllegalStateException("script s1 emitted " + s1Rows.size()
                            + " rows, expected 1");
                }
                runtime.getDeploymentService().undeployAll();
                compileDeploy(runtime, EPL_SCRIPT_TWO);
                EPStatement s2 = compileDeploy(runtime, EPL_SCRIPT_S2);
                List<EventBean> s2Rows = new ArrayList<>();
                s2.addListener((newEvents, oldEvents, statement, epRuntime) -> {
                    if (newEvents != null) {
                        s2Rows.addAll(Arrays.asList(newEvents));
                    }
                });
                sendBean(runtime, "E3", 0);
                sendBean(runtime, "E4", 1);
                sendBean(runtime, "E4", 2);
                if (s1Rows.size() != 1 || s2Rows.size() != 1) {
                    throw new IllegalStateException("script lifecycle rebind drifted: s1="
                            + s1Rows.size() + " s2=" + s2Rows.size());
                }
                break;
            }
            case "script-overload-soda": {
                if (!NOTE_SCRIPT_USE.equals(note)) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
                compileDeploy(runtime, EPL_ABC_TWO_TEN);
                compileDeploy(runtime, EPL_ABC_ONE_TEN);
                EPStatement s0 = compileDeploy(runtime, EPL_SELECT_ABC);
                List<EventBean> rows = new ArrayList<>();
                s0.addListener((newEvents, oldEvents, statement, epRuntime) -> {
                    if (newEvents != null) {
                        rows.addAll(Arrays.asList(newEvents));
                    }
                });
                SupportBean bean = new SupportBean();
                bean.setIntPrimitive(10);
                bean.setDoublePrimitive(3.5);
                runtime.getEventService().sendEventBean(bean, "SupportBean");
                if (rows.size() != 1 || !Integer.valueOf(350).equals(rows.get(0).get("c0"))
                        || !Integer.valueOf(100).equals(rows.get(0).get("c1"))) {
                    throw new IllegalStateException("abc overload values drifted");
                }
                runtime.getDeploymentService().undeployAll();
                EPStatement expr = compileDeploy(runtime, EPL_SOMESCRIPT);
                EPStatement select = compileDeploy(runtime, EPL_SELECT_SOMESCRIPT);
                if (!EPL_SOMESCRIPT.equals(expr.getProperty(
                        com.espertech.esper.common.client.util.StatementProperty.EPL))
                        || !EPL_SELECT_SOMESCRIPT.equals(select.getProperty(
                        com.espertech.esper.common.client.util.StatementProperty.EPL))) {
                    throw new IllegalStateException("SODA EPL round-trip drifted");
                }
                break;
            }
            default:
                throw new IllegalStateException("unknown unrepresentable step " + label);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    private static EPStatement compileDeploy(EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        EPStatement[] statements = deployment.getStatements();
        if (statements.length != 1) {
            throw new IllegalStateException("expected one statement, got "
                    + statements.length);
        }
        return statements[0];
    }

    private static void sendBean(EPRuntime runtime, String theString, int intPrimitive) {
        SupportBean bean = new SupportBean();
        bean.setTheString(theString);
        bean.setIntPrimitive(intPrimitive);
        runtime.getEventService().sendEventBean(bean, "SupportBean");
    }

    private static void assertStateless(EPRuntime runtime, String statementName,
                                        boolean expected) {
        EPStatement statement = null;
        for (String deploymentId : runtime.getDeploymentService().getDeployments()) {
            statement = runtime.getDeploymentService()
                    .getStatement(deploymentId, statementName);
            if (statement != null) {
                break;
            }
        }
        if (statement == null) {
            throw new IllegalStateException("statement " + statementName + " not deployed");
        }
        boolean stateless = ((EPStatementSPI) statement).getStatementContext()
                .isStatelessSelect();
        if (stateless != expected) {
            throw new IllegalStateException("statement " + statementName
                    + " isStatelessSelect=" + stateless + ", expected " + expected);
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty, and a listener
     * invocation that carries neither stream is a contract violation.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, EventBean fragments as nested row objects,
     * Java arrays as JSON arrays and collections as JSON arrays — the same
     * shapes the Go normalizer emits.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object item : (Collection<?>) value) {
                array.add(normalize(item));
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
        return Json.value(String.valueOf(value));
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
        validateStringArray(scenario.get("javaFlags"), new String[]{"OBSERVEROPS"},
                "javaFlags");

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
        offset = validateInvalidCase(steps, offset);
        offset = validateParseMixedCase(steps, offset);
        offset = validateLifecycleCase(steps, offset);
        offset = validateScriptUseCase(steps, offset);
        offset = validateExpressionUseCase(steps, offset, CASES[4], EPL_CREATE_NW,
                NOTE_STATEFUL_NW);
        offset = validateExpressionUseCase(steps, offset, CASES[5], EPL_CREATE_TBL,
                NOTE_STATEFUL_TBL);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact invalid step sequence mirroring EPLOtherInvalid lines 46-60:
     * the E1 deploy, the duplicate-expression compile-error, the
     * unrepresentable script duplicate half and undeploy-all.
     */
    private static int validateInvalidCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[0]);
        validateDeploy(steps.get(offset++), CASES[0], "s0", EPL_E1);
        validateDeployed(steps.get(offset++), CASES[0], "s0");
        validateBuildError(steps.get(offset++), CASES[0], "duplicate-expression",
                EPL_E1_DUP, "Expression 'E1' has already been declared");
        validateUnrepresentable(steps.get(offset++), CASES[0], "script-duplicate",
                NOTE_SCRIPT_DUP);
        validateUndeployAll(steps.get(offset++), CASES[0]);
        return offset;
    }

    /**
     * Exact parse-mixed step sequence mirroring
     * EPLOtherParseSpecialAndMixedExprAndScript lines 67-99: the myexpr
     * deploy, the narrowed s0 select, the E1 send, the s1 undeploy, the
     * scalarfilter deploy and s0 select, the stateless flag, the
     * SupportCollection send, undeploy-all, the unrepresentable callIt
     * half and undeploy-all.
     */
    private static int validateParseMixedCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[1]);
        validateDeploy(steps.get(offset++), CASES[1], "myexpr", EPL_MYEXPR);
        validateDeployed(steps.get(offset++), CASES[1], "myexpr");
        validateDeploy(steps.get(offset++), CASES[1], "s0", EPL_SELECT_MYEXPR);
        validateDeployed(steps.get(offset++), CASES[1], "s0");
        validateBeanSend(steps.get(offset++), CASES[1], "E1", 1);
        validateUndeploy(steps.get(offset++), CASES[1], "s0");
        validateDeploy(steps.get(offset++), CASES[1], "scalarfilter", EPL_SCALARFILTER);
        validateDeployed(steps.get(offset++), CASES[1], "scalarfilter");
        validateDeploy(steps.get(offset++), CASES[1], "s0", EPL_SELECT_SCALAR);
        validateDeployed(steps.get(offset++), CASES[1], "s0");
        validateUnrepresentable(steps.get(offset++), CASES[1], "stateless-s0",
                NOTE_STATELESS_S0);
        validateCollectionSend(steps.get(offset++), CASES[1], "E1,E2,E3,E4");
        validateUndeployAll(steps.get(offset++), CASES[1]);
        validateUnrepresentable(steps.get(offset++), CASES[1], "script-callit", NOTE_CALLIT);
        validateUndeployAll(steps.get(offset++), CASES[1]);
        return offset;
    }

    /**
     * Exact lifecycle step sequence mirroring tryAssertionLifecycleAndFilter
     * lines 225-253: the expr-one/s1 deploys, the E1/0 and E2/1 sends,
     * undeploy-all, the expr-two/s2 redeploys, the E3/0, E4/1 and E4/2
     * sends, undeploy-all and the unrepresentable script pass.
     */
    private static int validateLifecycleCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[2]);
        validateDeploy(steps.get(offset++), CASES[2], "expr-one", EPL_EXPR_ONE);
        validateDeployed(steps.get(offset++), CASES[2], "expr-one");
        validateDeploy(steps.get(offset++), CASES[2], "s1", EPL_S1);
        validateDeployed(steps.get(offset++), CASES[2], "s1");
        validateBeanSend(steps.get(offset++), CASES[2], "E1", 0);
        validateBeanSend(steps.get(offset++), CASES[2], "E2", 1);
        validateUndeployAll(steps.get(offset++), CASES[2]);
        validateDeploy(steps.get(offset++), CASES[2], "expr-two", EPL_EXPR_TWO);
        validateDeployed(steps.get(offset++), CASES[2], "expr-two");
        validateDeploy(steps.get(offset++), CASES[2], "s2", EPL_S2);
        validateDeployed(steps.get(offset++), CASES[2], "s2");
        validateBeanSend(steps.get(offset++), CASES[2], "E3", 0);
        validateBeanSend(steps.get(offset++), CASES[2], "E4", 1);
        validateBeanSend(steps.get(offset++), CASES[2], "E4", 2);
        validateUndeployAll(steps.get(offset++), CASES[2]);
        validateUnrepresentable(steps.get(offset++), CASES[2], "script-filter",
                NOTE_SCRIPT_FILTER);
        return offset;
    }

    /**
     * Exact script-use step sequence mirroring EPLOtherScriptUse lines
     * 105-130: the single unrepresentable marker whose oracle verification
     * replays the abc overloads and the SODA round-trips.
     */
    private static int validateScriptUseCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASES[3]);
        validateUnrepresentable(steps.get(offset++), CASES[3], "script-overload-soda",
                NOTE_SCRIPT_USE);
        return offset;
    }

    /**
     * Exact expression-use step sequence mirroring EPLOtherExpressionUse
     * lines 137-200: the TwoPi/factorPi deploys, the s0 select, the S0(10)
     * and E1/3 sends, the s0 undeploy, the statement-local TwoPi select,
     * the E1/0 send, the expr/join deploys, undeploy-all, the myexpr
     * deferred-subquery deploy, the infra create/insert/s0 deploys, the
     * stateful flag, the E1/100 and S0(1,'E1') sends and undeploy-all.
     */
    private static int validateExpressionUseCase(JsonArray steps, int offset,
                                                 String caseName, String createEpl,
                                                 String statefulNote) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "TwoPi", EPL_TWOPI);
        validateDeployed(steps.get(offset++), caseName, "TwoPi");
        validateDeploy(steps.get(offset++), caseName, "factorPi", EPL_FACTORPI);
        validateDeployed(steps.get(offset++), caseName, "factorPi");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELECT_TWOPI);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateS0Send(steps.get(offset++), caseName, 10, null);
        validateBeanSend(steps.get(offset++), caseName, "E1", 3);
        validateUndeploy(steps.get(offset++), caseName, "s0");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELECT_LOCAL);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateBeanSend(steps.get(offset++), caseName, "E1", 0);
        validateDeploy(steps.get(offset++), caseName, "expr", EPL_JOIN_EXPR);
        validateDeployed(steps.get(offset++), caseName, "expr");
        validateDeploy(steps.get(offset++), caseName, "join", EPL_JOIN_SELECT);
        validateDeployed(steps.get(offset++), caseName, "join");
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "myexpr", EPL_MYEXPR_INFRA);
        validateDeployed(steps.get(offset++), caseName, "myexpr");
        validateDeploy(steps.get(offset++), caseName, "create", createEpl);
        validateDeployed(steps.get(offset++), caseName, "create");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_INFRA);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "s0", EPL_SELECT_INFRA);
        validateDeployed(steps.get(offset++), caseName, "s0");
        validateUnrepresentable(steps.get(offset++), caseName, "stateful-s0", statefulNote);
        validateBeanSend(steps.get(offset++), caseName, "E1", 100);
        validateS0Send(steps.get(offset++), caseName, 1, "E1");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName,
                                       String expectedStatement, String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateBuildError(JsonValue value, String caseName,
                                           String expectedStatement, String expectedEpl,
                                           String expectedError) {
        JsonObject step = object(value, "build-error step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"build-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("build-error step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
    }

    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedStatement, String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
        }
    }

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String theString, int intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(payload.get("theString").asString())
                || payload.get("intPrimitive").asInt() != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, int id, String p00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        if (p00 == null) {
            requireFields(payload, "id");
        } else {
            requireFields(payload, "id", "p00");
            if (!p00.equals(payload.get("p00").asString())) {
                throw new IllegalArgumentException("SupportBean_S0 p00 is not pinned for "
                        + caseName);
            }
        }
        if (payload.get("id").asInt() != id) {
            throw new IllegalArgumentException("SupportBean_S0 id is not pinned for " + caseName);
        }
    }

    private static void validateCollectionSend(JsonValue value, String caseName, String csv) {
        JsonObject step = object(value, "SupportCollection step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportCollection".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportCollection step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportCollection payload");
        requireFields(payload, "strvals");
        if (!csv.equals(payload.get("strvals").asString())) {
            throw new IllegalArgumentException("SupportCollection payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateUndeploy(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "undeploy step");
        requireFields(step, "op", "case", "statement");
        if (!"undeploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("undeploy step is not pinned for " + caseName
                    + "/" + expectedStatement);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
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
                || !new HashSet<>(object.names()).equals(
                        new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields "
                    + (object == null ? "<null>" : object.names()) + ", expected "
                    + Arrays.toString(expectedNames));
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
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
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
